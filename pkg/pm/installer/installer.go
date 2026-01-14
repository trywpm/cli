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
	"time"

	"wpm/pkg/archive"
	"wpm/pkg/pm/registry"
	"wpm/pkg/pm/wpmjson/types"

	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Installer struct {
	concurrency int
	contentDir  string
	cacheDir    string
	client      registry.Client
	extractSem  chan struct{}
}

func New(contentDir, cacheDir string, concurrency int, client registry.Client) *Installer {
	if concurrency <= 0 {
		concurrency = 16
	}

	return &Installer{
		client:      client,
		contentDir:  contentDir,
		cacheDir:    cacheDir,
		concurrency: concurrency,
		extractSem:  make(chan struct{}, max(runtime.NumCPU(), 1)),
	}
}

func (i *Installer) InstallAll(ctx context.Context, plan []Action, progressFn func(Action)) error {
	if err := os.MkdirAll(i.cacheDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create cache directory")
	}

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

	return g.Wait()
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
	tarPath, err := i.ensureCached(ctx, action.Resolved, action.Digest)
	if err != nil {
		return err
	}

	// Limit concurrent extraction operations
	select {
	case i.extractSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-i.extractSem }()

	contentPath, err := i.unpackToStaging(tarPath)
	if err != nil {
		return err
	}
	defer i.removeAll(contentPath)

	return i.replaceDir(contentPath, targetDir)
}

func verifyDigest(tarPath, expectedDigest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return errors.Wrap(err, "failed to open tarball for digest verification")
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return errors.Wrap(err, "failed to read tarball for digest verification")
	}

	calculated := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if calculated != expectedDigest {
		return errors.Errorf("digest mismatch: expected %s, got %s", expectedDigest, calculated)
	}

	return nil
}

func (i *Installer) ensureCached(ctx context.Context, url, digest string) (string, error) {
	cleanDigest := strings.TrimPrefix(digest, "sha256:")
	safeFilename := strings.ReplaceAll(cleanDigest, "/", "_")
	tarPath := filepath.Join(i.cacheDir, safeFilename+".tar.zst")

	if _, err := os.Stat(tarPath); err == nil {
		if err := verifyDigest(tarPath, cleanDigest); err == nil {
			return tarPath, nil
		}

		if err := i.removeAll(tarPath); err != nil {
			return "", errors.Wrap(err, "failed to remove corrupted cache file; cannot proceed")
		}
	}

	f, err := os.CreateTemp(i.cacheDir, safeFilename+".tmp.*")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary tarball file")
	}

	tmpPath := f.Name()
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpPath)
	}()

	resp, err := i.client.DownloadTarball(ctx, url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to download %s", url)
	}
	defer resp.Close()

	hasher := sha256.New()
	tee := io.TeeReader(resp, hasher)

	if _, err := io.Copy(f, tee); err != nil {
		return "", errors.Wrap(err, "failed to write tarball to disk")
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	calculated := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if calculated != cleanDigest {
		return "", errors.Errorf("digest mismatch: expected %s, got %s", cleanDigest, calculated)
	}

	if err := os.Rename(tmpPath, tarPath); err != nil {
		return "", errors.Wrap(err, "failed to move tarball to cache")
	}

	return tarPath, nil
}

// unpackToStaging extracts the tarball to a temporary staging directory.
func (i *Installer) unpackToStaging(tarPath string) (contentDir string, err error) {
	rootTemp, err := os.MkdirTemp("", "wpm-staging-*")
	if err != nil {
		return "", err
	}

	file, err := os.Open(tarPath)
	if err != nil {
		_ = os.RemoveAll(rootTemp)
		return "", err
	}
	defer file.Close()

	if err := archive.Untar(file, rootTemp, nil); err != nil {
		_ = os.RemoveAll(rootTemp)
		return "", errors.Wrap(err, "failed to extract tarball")
	}

	entries, err := os.ReadDir(rootTemp)
	if err != nil {
		_ = os.RemoveAll(rootTemp)
		return "", err
	}

	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(rootTemp, entries[0].Name()), nil
	}

	return "", errors.New("unexpected tarball structure: expected single root directory")
}

// replaceDir atomically replaces targetDir with sourceDir.
func (i *Installer) replaceDir(sourceDir, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}

	// Simply move since target does not exist
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return i.moveDir(sourceDir, targetDir)
	}

	// Create backup path before replacing
	backupPath := targetDir + ".bak." + fmt.Sprint(time.Now().UnixNano())

	// Backup the existing package first
	if err := os.Rename(targetDir, backupPath); err != nil {
		return errors.Wrap(err, "failed to move existing package version to backup")
	}

	// Move in the new version
	if err := i.moveDir(sourceDir, targetDir); err != nil {
		_ = os.Rename(backupPath, targetDir)
		return errors.Wrap(err, "failed to install new version; rollback attempted")
	}

	if err := i.removeAll(backupPath); err != nil {
		// @todo: log warning about failed backup removal once telemetry/logging is in place
	}

	return nil
}

// moveDir attempts an atomic rename, falling back to copy+delete if across devices.
func (i *Installer) moveDir(source, dest string) error {
	err := os.Rename(source, dest)
	if err == nil {
		return nil
	}

	// cross-device can happen in case when same system has different volumes/mounts
	isCrossDevice := strings.Contains(err.Error(), "cross-device") ||
		strings.Contains(err.Error(), "different device") ||
		strings.Contains(err.Error(), "different disk")

	if isCrossDevice {
		if err := copy.Copy(source, dest); err != nil {
			_ = os.RemoveAll(dest)
			return errors.Wrap(err, "failed to copy package files across devices")
		}
		return os.RemoveAll(source)
	}

	return err
}

// removeAll retries deletion to handle transient file locks (Windows specific mostly).
func (i *Installer) removeAll(path string) error {
	var err error
	for range 3 {
		err = os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
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
