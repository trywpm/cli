package installer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"wpm/pkg/archive"
	"wpm/pkg/pm/registry"
	"wpm/pkg/pm/signatures"
	"wpm/pkg/pm/wpmjson/types"
	"wpm/pkg/pm/wpmjson/validator"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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

	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create content directory")
	}

	tmpDir := filepath.Join(contentDir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create tmp directory")
	}

	sweepStaleRunDirs(tmpDir)

	runDir, err := os.MkdirTemp(tmpDir, "run-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create staging directory")
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
		return errors.Wrap(err, "failed to fetch public keys for signature verification")
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
			return errors.Wrapf(err, "failed to delete %s", targetDir)
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
		return errors.Wrapf(err, "failed to fetch manifest for %s@%s", action.Name, action.Version)
	}

	sigs := manifest.Dist.Signatures
	if len(sigs) == 0 {
		return errors.Errorf("no signatures found for package %s@%s", action.Name, action.Version)
	}

	err = signatures.Verify(
		i.keysJson,
		sigs[0].KeyID,
		sigs[0].Sig,
		fmt.Appendf(nil, "%s:%s:%s", action.Name, action.Version, action.Digest),
	)
	if err != nil {
		return errors.Wrapf(err, "signature verification failed for package %s@%s", action.Name, action.Version)
	}

	resp, err := i.client.DownloadTarball(ctx, action.Resolved)
	if err != nil {
		return errors.Wrapf(err, "failed to download %s", action.Resolved)
	}
	defer resp.Close()

	hasher := sha256.New()
	stream := io.TeeReader(resp, hasher)

	select {
	case i.extractSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-i.extractSem }()

	extractedPath, tempContainer, err := i.unpackToStaging(stream)
	defer func() {
		_ = i.removeAll(context.Background(), tempContainer)
	}()
	if err != nil {
		return errors.Wrap(err, "failed to unpack package")
	}

	if _, err := io.Copy(io.Discard, stream); err != nil {
		return errors.Wrap(err, "failed to drain download stream")
	}

	cleanDigest := strings.TrimPrefix(action.Digest, "sha256:")
	calculated := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if calculated != cleanDigest {
		return errors.Errorf("digest mismatch: expected %s, got %s", cleanDigest, calculated)
	}

	return i.replaceDir(ctx, extractedPath, targetDir)
}

func (i *Installer) unpackToStaging(r io.Reader) (string, string, error) {
	rootTemp, err := os.MkdirTemp(i.runDir, "pkg-*")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create staging directory")
	}

	opts := &archive.TarOptions{Logger: i.logger}
	if err := archive.Untar(r, rootTemp, opts); err != nil {
		return "", rootTemp, errors.Wrap(err, "failed to extract tarball")
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
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return errors.Wrap(err, "failed to create parent directory")
	}

	if _, err := os.Lstat(targetDir); errors.Is(err, fs.ErrNotExist) {
		return i.rename(ctx, sourceDir, targetDir)
	}

	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return errors.Wrap(err, "failed to generate backup nonce")
	}
	backupDir := filepath.Join(i.runDir, filepath.Base(targetDir)+".bak-"+hex.EncodeToString(nonce[:]))

	if err := i.rename(ctx, targetDir, backupDir); err != nil {
		return errors.Wrap(err, "failed to move existing package to backup")
	}

	if err := i.rename(ctx, sourceDir, targetDir); err != nil {
		if rbErr := i.rename(context.Background(), backupDir, targetDir); rbErr != nil {
			return errors.Wrapf(
				err,
				"failed to install new version AND failed to roll back: previous version preserved at %q (rollback error: %v)",
				backupDir, rbErr,
			)
		}
		return errors.Wrap(err, "failed to install new version, rolled back")
	}

	_ = i.removeAll(ctx, backupDir)
	return nil
}

func (i *Installer) rename(ctx context.Context, src, dst string) error {
	return retryFS(ctx, func() error {
		err := os.Rename(src, dst)
		if err != nil && isCrossDeviceError(err) {
			return errors.Errorf(
				"cannot move %q to %q: source and destination are on different filesystems. "+
					"wpm requires the staging area (%s) and the install target (%s) to live on the same volume. "+
					"This typically affects Docker setups where individual plugin/theme directories are bind-mounted",
				src, dst, i.tmpDir, dst,
			)
		}
		return err
	}, isRetriableError)
}

func (i *Installer) removeAll(ctx context.Context, path string) error {
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

	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		var errno syscall.Errno
		if errors.As(linkErr.Err, &errno) {
			return isRetriableErrno(errno)
		}
	}

	if os.IsPermission(err) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		var errno syscall.Errno
		if errors.As(pathErr.Err, &errno) {
			return isRetriableErrno(errno)
		}
	}

	return false
}

func isRetriableErrno(errno syscall.Errno) bool {
	return errno == errorAccessDenied || errno == errorSharingViolation
}

func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		var errno syscall.Errno
		if errors.As(linkErr.Err, &errno) {
			return errno == syscall.EXDEV || errno == errorNotSameDevice
		}
	}
	return false
}

func (i *Installer) getTargetDir(pkgType types.PackageType, name string) (string, error) {
	if err := validator.IsValidPackageName(name); err != nil {
		return "", errors.Wrapf(err, "refusing to operate on package with invalid name %q", name)
	}

	subDir := "plugins"
	switch pkgType {
	case types.TypeTheme:
		subDir = "themes"
	case types.TypeMuPlugin:
		subDir = "mu-plugins"
	}

	target := filepath.Join(i.contentDir, subDir, name)

	// IsValidPackageName already prevents escape, but verify the resolved
	// path stays inside contentDir in case of symlinks or other weird filesystem setups.
	rel, err := filepath.Rel(i.contentDir, target)
	if err != nil || !filepath.IsLocal(rel) {
		return "", errors.Errorf("package %q resolves outside content directory", name)
	}

	return target, nil
}
