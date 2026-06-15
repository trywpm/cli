package publish

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/completion"
	"go.wpm.so/cli/cli/version"
	"go.wpm.so/cli/pkg/archive"
	"go.wpm.so/cli/pkg/output"
	"go.wpm.so/cli/pkg/pm/registry"
	"go.wpm.so/cli/pkg/pm/wpmignore"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/manifest"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
)

const (
	maxReadmeSize       = 100 * 1024        // 100KB
	maxPackedSize int64 = 128 * 1024 * 1024 // 128MB
)

type publishOptions struct {
	dryRun  bool
	verbose bool
	tag     string
	access  string
}

func NewPublishCommand(wpmCli command.Cli) *cobra.Command {
	var opts publishOptions

	cmd := &cobra.Command{
		Use:   "publish [OPTIONS]",
		Short: "Publish a package to the wpm registry",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runPublish(cmd.Context(), wpmCli, opts) },
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.tag, "tag", "latest", "Set the package tag")
	flags.BoolVar(&opts.verbose, "verbose", false, "Enable verbose output")
	flags.StringVarP(&opts.access, "access", "a", "private", "Set the package access level to either public or private")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Perform a publish operation without actually publishing the package")

	_ = cmd.RegisterFlagCompletionFunc("tag", completion.DistTags())
	_ = cmd.RegisterFlagCompletionFunc("access", completion.PackageVisibility())

	return cmd
}

func pack(ctx context.Context, path string, opts publishOptions, out *output.Output) (*archive.Tarballer, error) {
	ignorePatterns, err := wpmignore.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tarOptions := &archive.TarOptions{
		ExcludePatterns: ignorePatterns,
		Logger: func(format string, args ...any) {
			out.ErrorWrite(fmt.Sprintf(format+"\n", args...))
		},
	}

	tar, err := archive.Tar(ctx, path, tarOptions, func(fileInfo os.FileInfo) {
		if opts.verbose {
			sizeString := units.HumanSize(float64(fileInfo.Size()))
			sizeString = fmt.Sprintf("%-7s", sizeString) // pad to 7 spaces since size string is capped to 4 numbers
			out.PrettyErrorln(output.Text{
				Plain: fmt.Sprintf("%s %s %s", "packed", sizeString, fileInfo.Name()),
				Fancy: fmt.Sprintf("%s %s %s", aec.CyanF.Apply("packed"), sizeString, fileInfo.Name()),
			})
		}
	})
	if err != nil {
		return nil, err
	}

	return tar, nil
}

func getReadme(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.EqualFold(entry.Name(), "readme.md") {
			fullPath := filepath.Join(dirPath, entry.Name())

			//nolint:gosec // This is a CLI tool safely reading from the local workspace
			f, err := os.Open(fullPath)
			if err != nil {
				return "", err
			}
			defer func() {
				_ = f.Close()
			}()

			// Limit readme size to maxReadmeSize.
			data, err := io.ReadAll(io.LimitReader(f, maxReadmeSize))
			if err != nil {
				return "", err
			}

			return string(data), nil
		}
	}

	return "", nil
}

type tarballSizeCounter struct {
	total int64
	limit int64
}

func (c *tarballSizeCounter) Write(p []byte) (n int, err error) {
	if c.limit > 0 && c.total+int64(len(p)) > c.limit {
		return 0, fmt.Errorf("tarball size exceeds %d bytes, refusing to continue", c.limit)
	}
	c.total += int64(len(p))
	return len(p), nil
}

func runPublish(ctx context.Context, wpmCli command.Cli, opts publishOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	visibility := types.PackageVisibility(opts.access)
	if !visibility.Valid() {
		return errors.New("access must be either public or private")
	}

	wpmJson, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}
	if err := validateWpmJson(wpmJson); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(wpmCli.Err(), "📦 %s@%s\n\n", wpmJson.Name, wpmJson.Version)

	tempFile, err := os.CreateTemp("", "wpm-tarball-*.tar.zst")
	if err != nil {
		return fmt.Errorf("failed to create temporary tarball: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	tarballer, err := pack(ctx, cwd, opts, wpmCli.Output())
	if err != nil {
		return fmt.Errorf("failed to pack the package into a tarball: %w", err)
	}
	defer func() { _ = tarballer.Close() }()

	hasher := sha256.New()
	counter := &tarballSizeCounter{limit: maxPackedSize}

	if err = packIntoTarball(wpmCli, opts, tarballer, tempFile, hasher, counter); err != nil {
		return err
	}

	if counter.total == 0 {
		return errors.New("tarball size is zero, cannot publish empty package")
	}

	digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	printPublishSummary(wpmCli, opts, counter.total, tarballer, digest)

	if opts.dryRun {
		_, _ = fmt.Fprintf(wpmCli.Err(), "dry run complete, %s@%s is ready to be published\n", wpmJson.Name, wpmJson.Version)
		return nil
	}

	if err := validateAuth(wpmCli); err != nil {
		return err
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	readme, err := getReadme(cwd)
	if err != nil {
		return fmt.Errorf("failed to read readme file: %w", err)
	}

	pkgManifest := buildManifest(wpmJson, opts, visibility, digest, counter.total, tarballer, readme)
	if err = uploadPackage(ctx, wpmCli, registryClient, pkgManifest, tempFile); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(wpmCli.Err(), "%s %s\n", aec.GreenF.Apply("✔"), "published "+wpmJson.Name+"@"+wpmJson.Version)

	return nil
}

func validateWpmJson(wpmJson *wpmjson.Config) error {
	if wpmJson == nil {
		return errors.New("no wpm.json found in the current directory")
	}
	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}
	if err := wpmJson.Validate(); err != nil {
		return err
	}
	return nil
}

func packIntoTarball(wpmCli command.Cli, opts publishOptions, tarballer *archive.Tarballer, tempFile io.Writer, hasher hash.Hash, counter *tarballSizeCounter) error {
	multiWriter := io.MultiWriter(tempFile, hasher, counter)

	packFn := func() error {
		if _, err := io.Copy(multiWriter, tarballer.Reader()); err != nil {
			return fmt.Errorf("failed to process tarball: %w", err)
		}
		return nil
	}

	if opts.verbose {
		if err := packFn(); err != nil {
			return err
		}
		_, _ = fmt.Fprint(wpmCli.Err(), "\n")
		return nil
	}

	return wpmCli.Progress().RunWithProgress("packing tarball", packFn, wpmCli.Err())
}

func printPublishSummary(wpmCli command.Cli, opts publishOptions, packedBytes int64, tarballer *archive.Tarballer, digest string) {
	c := func(a aec.ANSI, s string) string {
		if !wpmCli.Err().IsColorEnabled() {
			return s
		}
		return a.Apply(s)
	}
	w := tabwriter.NewWriter(wpmCli.Err(), 0, 0, 2, ' ', 0)

	packedSize := units.HumanSize(float64(packedBytes))
	unpackedSize := units.HumanSize(float64(tarballer.UnpackedSize()))

	_, _ = fmt.Fprintf(w, "├─ %s:\t%s\n", c(aec.LightBlueF, "Tag"), opts.tag)
	_, _ = fmt.Fprintf(w, "├─ %s:\t%s\n", c(aec.LightBlueF, "Access"), opts.access)
	_, _ = fmt.Fprintf(w, "├─ %s:\t%d\n", c(aec.LightBlueF, "Files"), tarballer.FileCount())
	_, _ = fmt.Fprintf(w, "├─ %s:\t%s %s\n", c(aec.LightBlueF, "Size"), packedSize, c(aec.Faint, fmt.Sprintf("(%s unpacked)", unpackedSize)))
	_, _ = fmt.Fprintf(w, "└─ %s:\t%s\n", c(aec.LightBlueF, "Digest"), digest)

	_ = w.Flush()
	_, _ = fmt.Fprint(wpmCli.Err(), "\n")
}

func validateAuth(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()
	if cfg.DefaultUser == "" || cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}
	return nil
}

func buildManifest(wpmJson *wpmjson.Config, opts publishOptions, visibility types.PackageVisibility, digest string, packedBytes int64, tarballer *archive.Tarballer, readme string) *manifest.Package {
	return &manifest.Package{
		Name:            wpmJson.Name,
		Description:     wpmJson.Description,
		Type:            wpmJson.Type,
		Version:         wpmJson.Version,
		Requires:        wpmJson.Requires,
		License:         wpmJson.License,
		Homepage:        wpmJson.Homepage,
		Tags:            wpmJson.Tags,
		Author:          wpmJson.Author,
		Dependencies:    wpmJson.Dependencies,
		DevDependencies: wpmJson.DevDependencies,
		Tag:             opts.tag,
		Dist: manifest.Dist{
			Digest:       "sha256:" + digest,
			PackedSize:   packedBytes,
			TotalFiles:   tarballer.FileCount(),
			UnpackedSize: tarballer.UnpackedSize(),
		},
		Wpm:        version.Version,
		Visibility: visibility,
		Readme:     readme,
	}
}

func uploadPackage(ctx context.Context, wpmCli command.Cli, registryClient registry.Client, pkgManifest *manifest.Package, tempFile *os.File) error {
	return wpmCli.Progress().RunWithProgress(
		"publishing package",
		func() error {
			if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("failed to seek to beginning of tarball: %w", err)
			}
			return registryClient.PutPackage(ctx, pkgManifest, tempFile)
		},
		wpmCli.Err(),
	)
}
