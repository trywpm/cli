package installer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"wpm/pkg/archive"
	"wpm/pkg/pm/registry"
	"wpm/pkg/pm/wpmjson/types"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Installer struct {
	concurrency int
	contentDir  string
	tmpDir      string
	client      registry.Client
	extractSem  chan struct{}
}

func New(contentDir string, concurrency int, client registry.Client) *Installer {
	if concurrency <= 0 {
		concurrency = 16
	}

	tmpDir := filepath.Join(contentDir, ".tmp")
	_ = os.MkdirAll(tmpDir, 0755)

	return &Installer{
		client:      client,
		contentDir:  contentDir,
		tmpDir:      tmpDir,
		concurrency: concurrency,
		extractSem:  make(chan struct{}, max(runtime.NumCPU(), 1)),
	}
}

func (i *Installer) InstallAll(ctx context.Context, plan []Action, progressFn func(Action)) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(i.concurrency)

	for _, action := range plan {
		g.Go(func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			err := i.Install(ctx, action)
			if err == nil && progressFn != nil {
				progressFn(action)
			}

			return err
		})
	}

	err := g.Wait()
	os.RemoveAll(i.tmpDir)
	return err
}

func (i *Installer) Install(ctx context.Context, action Action) error {
	targetDir := i.getTargetDir(action.PkgType, action.Name)

	switch action.Type {
	case ActionRemove:
		if err := i.removeAll(targetDir); err != nil {
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
		_ = i.removeAll(tempContainer)
	}()

	if err != nil {
		return errors.Wrap(err, "failed to unpack package")
	}

	cleanDigest := strings.TrimPrefix(action.Digest, "sha256:")
	calculated := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if calculated != cleanDigest {
		return errors.Errorf("digest mismatch: expected %s, got %s", cleanDigest, calculated)
	}

	return i.replaceDir(extractedPath, targetDir)
}

// unpackToStaging extracts to a temporary directory inside .tmp.
// Returns the path to the inner single-root folder, the path to the outer temp container, and error.
func (i *Installer) unpackToStaging(r io.Reader) (string, string, error) {
	rootTemp, err := os.MkdirTemp(i.tmpDir, "wpm-pkg-*")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create temporary directory")
	}

	if err := archive.Untar(r, rootTemp, nil); err != nil {
		return "", rootTemp, errors.Wrap(err, "failed to extract tarball")
	}

	entries, err := os.ReadDir(rootTemp)
	if err != nil {
		return "", rootTemp, err
	}

	// Strict Single Root Check
	if len(entries) != 1 || !entries[0].IsDir() {
		return "", rootTemp, errors.New("invalid package structure: expected exactly one root directory")
	}

	return filepath.Join(rootTemp, entries[0].Name()), rootTemp, nil
}

// replaceDir atomically replaces targetDir
func (i *Installer) replaceDir(sourceDir, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return i.rename(sourceDir, targetDir)
	}

	backupPath := targetDir + ".bak." + fmt.Sprint(time.Now().UnixNano())
	if err := i.rename(targetDir, backupPath); err != nil {
		return errors.Wrap(err, "failed to move existing package to backup")
	}

	if err := i.rename(sourceDir, targetDir); err != nil {
		// ROLLBACK: Try to restore backup
		_ = i.rename(backupPath, targetDir)
		return errors.Wrap(err, "failed to install new version, rolled back")
	}

	go i.removeAll(backupPath)

	return nil
}

// rename with retries for Windows file locking stability
func (i *Installer) rename(src, dst string) error {
	var err error
	for attempt := range 5 {
		err = os.Rename(src, dst)
		if err == nil {
			return nil
		}

		if isLinkError(err) {
			return err
		}

		if !isRetriableError(err) {
			return err
		}

		time.Sleep(50 * time.Millisecond * time.Duration(attempt+1))

		// On the 4th attempt, try to force GC to release file handles on Windows
		if attempt == 4 {
			runtime.GC()
		}
	}
	return errors.Wrapf(err, "failed to rename %s to %s after retries", filepath.Base(src), filepath.Base(dst))
}

func isRetriableError(err error) bool {
	if os.IsPermission(err) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		var errno syscall.Errno
		if errors.As(pathErr.Err, &errno) {
			// ERROR_ACCESS_DENIED (5) or ERROR_SHARING_VIOLATION (32)
			if errno == 5 || errno == 32 {
				return true
			}
		}
	}
	return false
}

func isLinkError(err error) bool {
	var linkErr *os.LinkError
	return errors.As(err, &linkErr)
}

// removeAll with retries for Windows file locking stability
func (i *Installer) removeAll(path string) error {
	var err error
	for j := range 5 {
		err = os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}

		time.Sleep(100 * time.Millisecond)

		if j == 2 {
			// attempt to force GC to release file handles on Windows
			runtime.GC()
		}
	}
	return err
}

func (i *Installer) getTargetDir(pkgType types.PackageType, name string) string {
	subDir := "plugins"
	switch pkgType {
	case types.TypeTheme:
		subDir = "themes"
	case types.TypeMuPlugin:
		subDir = "mu-plugins"
	}
	return filepath.Join(i.contentDir, subDir, name)
}
