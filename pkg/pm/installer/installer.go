package installer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"go.wpm.so/cli/pkg/archive"
	"go.wpm.so/cli/pkg/pm/registry"
	"go.wpm.so/cli/pkg/pm/signatures"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
)

const (
	errorAccessDenied     = syscall.Errno(5)  // Windows ERROR_ACCESS_DENIED
	errorNotSameDevice    = syscall.Errno(17) // Windows ERROR_NOT_SAME_DEVICE
	errorSharingViolation = syscall.Errno(32) // Windows ERROR_SHARING_VIOLATION
)

type Installer struct {
	concurrency int
	contentDir  string
	tmpDir      string
	runDir      string

	client     registry.Client
	extractSem chan struct{}
	keysJson   signatures.KeysJson
	logger     func(format string, args ...any)
}

func New(
	ctx context.Context,
	contentDir string,
	concurrency int,
	client registry.Client,
	logger func(format string, args ...any),
) (*Installer, error) {
	if concurrency <= 0 {
		concurrency = 16
	}

	//nolint:gosec // Dir perms are intentionally permissive here.
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create content directory: %w", err)
	}

	tmpDir := filepath.Join(contentDir, ".tmp")
	//nolint:gosec // Dir perms are intentionally permissive here.
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp directory: %w", err)
	}

	sweepStaleRunDirs(tmpDir)

	runDir, err := os.MkdirTemp(tmpDir, "run-")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory: %w", err)
	}

	return &Installer{
		client:      client,
		contentDir:  contentDir,
		tmpDir:      tmpDir,
		runDir:      runDir,
		concurrency: concurrency,
		extractSem:  make(chan struct{}, max(runtime.NumCPU(), 1)),
		logger:      logger,
	}, nil
}

func (i *Installer) Close() error {
	if i == nil {
		return nil
	}
	return os.RemoveAll(i.tmpDir)
}

// Caller must hold the project lock.
func sweepStaleRunDirs(tmpDir string) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "run-") {
			continue
		}
		_ = os.RemoveAll(filepath.Join(tmpDir, e.Name()))
	}
}

func (i *Installer) InstallAll(ctx context.Context, plan []Action, progressFn func(Action)) error {
	keys, err := i.client.GetKeysJson(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch public keys for signature verification: %w", err)
	}
	i.keysJson = keys

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(i.concurrency)

	for _, action := range plan {
		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}

			err := i.install(ctx, action)
			if err == nil && progressFn != nil {
				progressFn(action)
			}
			return err
		})
	}
	return g.Wait()
}

func (i *Installer) install(ctx context.Context, action Action) error {
	targetDir, err := i.getTargetDir(action.PkgType, action.Name)
	if err != nil {
		return err
	}

	switch action.Type {
	case ActionRemove:
		if err := i.removeAll(ctx, targetDir); err != nil {
			return fmt.Errorf("failed to delete %s: %w", targetDir, err)
		}
		return nil
	case ActionInstall, ActionUpdate:
		return i.installOrUpdate(ctx, action, targetDir)
	default:
		return nil
	}
}

func (i *Installer) installOrUpdate(ctx context.Context, action Action, targetDir string) error {
	manifest, err := i.client.GetPackageManifest(ctx, action.Name, action.Version, false)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest for %s@%s: %w", action.Name, action.Version, err)
	}

	sigs := manifest.Dist.Signatures
	if len(sigs) == 0 {
		return fmt.Errorf("no signatures found for package %s@%s", action.Name, action.Version)
	}

	err = signatures.Verify(
		i.keysJson,
		sigs[0].KeyID,
		sigs[0].Sig,
		fmt.Appendf(nil, "%s:%s:%s", action.Name, action.Version, action.Digest),
	)
	if err != nil {
		return fmt.Errorf("signature verification failed for package %s@%s: %w", action.Name, action.Version, err)
	}

	resp, err := i.client.DownloadTarball(ctx, action.Resolved)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", action.Resolved, err)
	}
	defer func() {
		_ = resp.Close()
	}()

	hasher := sha256.New()
	stream := io.TeeReader(resp, hasher)

	select {
	case i.extractSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-i.extractSem }()

	extractedPath, tempContainer, err := i.unpackToStaging(ctx, stream)
	defer func() {
		_ = i.removeAll(context.Background(), tempContainer)
	}()
	if err != nil {
		return fmt.Errorf("failed to unpack package: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := io.Copy(io.Discard, stream); err != nil {
		return fmt.Errorf("failed to drain download stream: %w", err)
	}

	cleanDigest := strings.TrimPrefix(action.Digest, "sha256:")
	calculated := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if calculated != cleanDigest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", cleanDigest, calculated)
	}

	return i.replaceDir(ctx, extractedPath, targetDir)
}

func (i *Installer) unpackToStaging(ctx context.Context, r io.Reader) (string, string, error) {
	rootTemp, err := os.MkdirTemp(i.runDir, "pkg-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create staging directory: %w", err)
	}

	opts := &archive.TarOptions{Logger: i.logger}
	if err := archive.Untar(ctx, r, rootTemp, opts); err != nil {
		return "", rootTemp, fmt.Errorf("failed to extract tarball: %w", err)
	}

	entries, err := os.ReadDir(rootTemp)
	if err != nil {
		return "", rootTemp, err
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return "", rootTemp, errors.New("invalid package structure: expected exactly one root directory")
	}

	return filepath.Join(rootTemp, entries[0].Name()), rootTemp, nil
}

func (i *Installer) replaceDir(ctx context.Context, sourceDir, targetDir string) error {
	//nolint:gosec // Dir perms are intentionally permissive here.
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if _, err := os.Lstat(targetDir); errors.Is(err, fs.ErrNotExist) {
		return i.rename(ctx, sourceDir, targetDir)
	}

	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("failed to generate backup nonce: %w", err)
	}
	backupDir := filepath.Join(i.runDir, filepath.Base(targetDir)+".bak-"+hex.EncodeToString(nonce[:]))

	if err := i.rename(ctx, targetDir, backupDir); err != nil {
		return fmt.Errorf("failed to move existing package to backup: %w", err)
	}

	if err := i.rename(ctx, sourceDir, targetDir); err != nil {
		if rbErr := i.rename(context.Background(), backupDir, targetDir); rbErr != nil {
			return fmt.Errorf(
				"failed to install new version AND failed to roll back: previous version preserved at %q (rollback error: %v): %w",
				backupDir, rbErr, err,
			)
		}
		return fmt.Errorf("failed to install new version, rolled back: %w", err)
	}

	_ = i.removeAll(ctx, backupDir)
	return nil
}

func (i *Installer) rename(ctx context.Context, src, dst string) error {
	return retryFS(ctx, func() error {
		err := os.Rename(src, dst)
		if err != nil && isCrossDeviceError(err) {
			return fmt.Errorf(
				"cannot move %q to %q: source and destination are on different filesystems. "+
					"wpm requires the staging area (%s) and the install target (%s) to live on the same volume. "+
					"This typically affects Docker setups where individual plugin/theme directories are bind-mounted",
				src, dst, i.tmpDir, dst,
			)
		}
		return err
	}, isRetriableError)
}

func (*Installer) removeAll(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	return retryFS(ctx, func() error {
		err := os.RemoveAll(path)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}, isRetriableError)
}

// retryFS handles transient Windows file-locking errors (AV scanners, the
// indexer, web-server workers holding plugin files open during a swap).
// Exponential backoff up to ~5s; the budget is sized to outlast typical
// AV scan windows without making clean failures feel slow.
func retryFS(ctx context.Context, op func() error, retriable func(error) bool) error {
	const maxAttempts = 8
	backoff := 25 * time.Millisecond

	var err error
	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < time.Second {
				backoff *= 2
			}
		}

		err = op()
		if err == nil || !retriable(err) {
			return err
		}
	}
	return err
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		if errno, ok := errors.AsType[syscall.Errno](linkErr.Err); ok {
			return isRetriableErrno(errno)
		}
	}

	if os.IsPermission(err) {
		return true
	}

	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
		if errno, ok := errors.AsType[syscall.Errno](pathErr.Err); ok {
			return isRetriableErrno(errno)
		}
	}

	return false
}

func isRetriableErrno(errno syscall.Errno) bool {
	return errno == errorAccessDenied || errno == errorSharingViolation
}

func isCrossDeviceError(err error) bool {
	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		if errno, ok := errors.AsType[syscall.Errno](linkErr.Err); ok {
			return errno == syscall.EXDEV || errno == errorNotSameDevice
		}
	}
	return false
}

func (i *Installer) getTargetDir(pkgType types.PackageType, name string) (string, error) {
	if err := validator.IsValidPackageName(name); err != nil {
		return "", fmt.Errorf("refusing to operate on package with invalid name %q: %w", name, err)
	}

	var subDir string
	switch pkgType {
	case types.TypeTheme:
		subDir = "themes"
	case types.TypeMuPlugin:
		subDir = "mu-plugins"
	case types.TypePlugin:
		subDir = "plugins"
	default:
		return "", fmt.Errorf("unknown package type %q for package %q", pkgType, name)
	}

	target := filepath.Join(i.contentDir, subDir, name)

	// IsValidPackageName already prevents escape, but verify the resolved
	// path stays inside contentDir in case of symlinks or other weird filesystem setups.
	rel, err := filepath.Rel(i.contentDir, target)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("package %q resolves outside content directory", name)
	}

	return target, nil
}
