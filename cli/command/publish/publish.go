package publish

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/version"
	"wpm/pkg/archive"
	"wpm/pkg/pm/wpmignore"
	"wpm/pkg/pm/wpmjson"

	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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

	return cmd
}

func pack(stdOut io.Writer, path string, opts publishOptions) (*archive.Tarballer, error) {
	ignorePatterns, err := wpmignore.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tar, err := archive.Tar(path, &archive.TarOptions{
		ShowInfo:        opts.verbose,
		ExcludePatterns: ignorePatterns,
	}, stdOut)
	if err != nil {
		return nil, err
	}

	return tar, nil
}

// tarballSizeCounter is an io.Writer that counts bytes.
type tarballSizeCounter struct {
	total int64
}

type idempotencyRespError struct {
	message string
}

func (e *idempotencyRespError) Error() string {
	return e.message
}

func (c *tarballSizeCounter) Write(p []byte) (n int, err error) {
	c.total += int64(len(p))
	return len(p), nil
}

func runPublish(ctx context.Context, wpmCli command.Cli, opts publishOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	if opts.access != "public" && opts.access != "private" {
		return errors.New("access must be either public or private")
	}

	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" || cfg.DefaultTId == "" {
		return errors.New("user must be logged in to perform this action")
	}

	wpmJson, err := wpmjson.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}

	fmt.Fprintf(wpmCli.Err(), aec.CyanF.Apply("📦 preparing %s@%s for publishing 📦\n\n"), wpmJson.Name, wpmJson.Version)

	tempFile, err := os.CreateTemp("", "wpm-tarball-*.tar.zst")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary tarball")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	tarballer, err := pack(wpmCli.Err(), cwd, opts)
	if err != nil {
		return errors.Wrap(err, "failed to pack the package into a tarball")
	}

	hasher := sha256.New()
	counter := &tarballSizeCounter{}
	multiWriter := io.MultiWriter(tempFile, hasher, counter)

	if _, err := io.Copy(multiWriter, tarballer.Reader()); err != nil {
		tempFile.Close()
		return errors.Wrap(err, "failed to process tarball")
	}

	digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	if opts.verbose {
		fmt.Fprint(wpmCli.Err(), "\n")
	}

	// bail if tarball size is zero or greater than 128mb
	if counter.total == 0 {
		tempFile.Close()
		return errors.New("tarball size is zero, cannot publish empty package")
	}

	if counter.total > 128*1024*1024 {
		tempFile.Close()
		return errors.New("tarball size exceeds 128mb, cannot publish package")
	}

	fmt.Fprintf(wpmCli.Err(), "%s: %d\n", aec.LightBlueF.Apply("total files"), tarballer.FileCount())
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("digest"), digest)
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("unpacked size"), units.HumanSize(float64(tarballer.UnpackedSize())))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("packed size"), units.HumanSize(float64(counter.total)))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("tag"), opts.tag)
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("access"), opts.access)
	fmt.Fprint(wpmCli.Err(), "\n")

	if opts.dryRun {
		fmt.Fprintf(wpmCli.Err(), "dry run complete, %s@%s is ready to be published\n", wpmJson.Name, wpmJson.Version)
		return nil
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	manifest := &wpmjson.Package{
		Config: wpmjson.Config{
			Name:            wpmJson.Name,
			Description:     wpmJson.Description,
			Type:            wpmJson.Type,
			Version:         wpmJson.Version,
			Platform:        wpmJson.Platform,
			License:         wpmJson.License,
			Homepage:        wpmJson.Homepage,
			Tags:            wpmJson.Tags,
			Team:            wpmJson.Team,
			Dependencies:    wpmJson.Dependencies,
			DevDependencies: wpmJson.DevDependencies,
		},
		Meta: wpmjson.Meta{
			Dist: wpmjson.Dist{
				Digest:       "sha256:" + digest,
				PackedSize:   counter.total,
				TotalFiles:   tarballer.FileCount(),
				UnpackedSize: tarballer.UnpackedSize(),
			},
			Tag:        opts.tag,
			Visibility: opts.access,
			Wpm:        version.Version,
		},
	}

	err = wpmCli.Progress().RunWithProgress(
		"publishing package",
		func() error {
			if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
				return errors.Wrap(err, "failed to seek to beginning of tarball")
			}

			return registryClient.PutPackage(ctx, manifest, tempFile)
		},
		wpmCli.Err(),
	)
	if err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Err(), "%s %s\n", aec.GreenF.Apply("✔"), "package published successfully!")

	return nil
}
