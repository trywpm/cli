package installer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"wpm/pkg/archive"
	"wpm/pkg/pm/registry"
	"wpm/pkg/pm/wpmjson"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Installer struct {
	concurrency int
	contentDir  string
	client      registry.Client
}

func New(contentDir string, concurrency int, client registry.Client) *Installer {
	if concurrency <= 0 {
		concurrency = 16
	}

	return &Installer{
		client:      client,
		contentDir:  contentDir,
		concurrency: concurrency,
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

	return g.Wait()
}

func (i *Installer) Install(ctx context.Context, action Action) error {
	targetDir := i.getTargetDir(action.PkgType, action.Name)

	switch action.Type {
	case ActionRemove:
		return os.RemoveAll(targetDir)
	case ActionInstall, ActionUpdate:
		if err := os.RemoveAll(targetDir); err != nil {
			return err
		}

		return i.downloadAndExtract(ctx, action.Resolved, action.Digest, targetDir)
	default:
		return nil
	}
}

func (i *Installer) getTargetDir(pkgType wpmjson.PackageType, name string) string {
	subDir := "plugins"
	switch pkgType {
	case wpmjson.TypeTheme:
		subDir = "themes"
	case wpmjson.TypeMuPlugin:
		subDir = "mu-plugins"
	}
	return filepath.Join(i.contentDir, subDir, name)
}

func (i *Installer) downloadAndExtract(ctx context.Context, url, digest, targetDir string) error {
	resp, err := i.client.DownloadTarball(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to download tarball from %s", url)
	}
	defer resp.Close()

	tempDir, err := os.MkdirTemp("", "wpm-install-*")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(tempDir)

	hasher := sha256.New()
	teeReader := io.TeeReader(resp, hasher)

	if err := archive.Untar(teeReader, tempDir, nil); err != nil {
		return errors.Wrap(err, "failed to extract tarball")
	}

	expectedHash := strings.TrimPrefix(digest, "sha256:")
	calculatedHash := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	if expectedHash != calculatedHash {
		return errors.Errorf("Integrity check failed for %s", url)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return errors.Wrap(err, "failed to read extracted files")
	}

	sourcePath := tempDir
	if len(entries) == 1 && entries[0].IsDir() {
		sourcePath = filepath.Join(tempDir, entries[0].Name())
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}

	if err := os.Rename(sourcePath, targetDir); err != nil {
		return errors.Wrap(err, "failed to move extracted package to target dir")
	}

	return nil
}
