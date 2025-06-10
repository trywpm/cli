package publish

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/registry/client"
	"wpm/cli/version"
	"wpm/pkg/archive"
	"wpm/pkg/wpm"

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
		RunE:  func(cmd *cobra.Command, args []string) error { return runPublish(wpmCli, opts) },
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.tag, "tag", "latest", "Set the package tag")
	flags.BoolVar(&opts.verbose, "verbose", false, "Enable verbose output")
	flags.StringVarP(&opts.access, "access", "a", "private", "Set the package access level to either public or private")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Perform a publish operation without actually publishing the package")

	return cmd
}

func pack(stdOut io.Writer, path string, opts publishOptions) (*archive.Tarballer, error) {
	ignorePatterns, err := wpm.ReadWpmIgnore(path)
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

func readme(path string) (string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.ToLower(file.Name()) == "readme.md" {
			readme, err := os.ReadFile(path + "/" + file.Name())
			if err != nil {
				return "", err
			}

			return string(readme), nil
		}
	}

	return "", nil
}

// tarballSizeCounter is an io.Writer that counts bytes.
type tarballSizeCounter struct {
	total int64
}

func (c *tarballSizeCounter) Write(p []byte) (n int, err error) {
	c.total += int64(len(p))
	return len(p), nil
}

func runPublish(wpmCli command.Cli, opts publishOptions) error {
	cwd := wpmCli.Options().Cwd

	if opts.access != "public" && opts.access != "private" {
		return errors.New("access must be either public or private")
	}

	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	wpmJson, err := wpm.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}

	fmt.Fprintf(wpmCli.Err(), aec.CyanF.Apply("📦 preparing %s@%s for publishing 📦\n\n"), wpmJson.Name, wpmJson.Version)

	tempFile, err := io.CreateTemp("", "wpm-tarball-*.tar.zst")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary tarball")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	tarballer, err := pack(wpmCli.Err(), cwd, opts)
	if err != nil {
		return errors.Wrap(err, "failed to pack the package into a tarball")
	}

	tarReader := tarballer.Reader()

	if _, err := io.Copy(tempFile, tarReader); err != nil {
		return errors.Wrap(err, "failed to write temporary tarball")
	}

	if err := tempFile.Close(); err != nil {
		return errors.Wrap(err, "failed to close temporary tarball")
	}

	tempTarball, err := io.Open(tempFile.Name())
	if err != nil {
		return errors.Wrap(err, "failed to open tarball for reading")
	}
	defer tempTarball.Close()

	hasher := sha256.New()
	counter := &tarballSizeCounter{}
	teeReader := io.TeeReader(tempTarball, io.MultiWriter(hasher, counter))

	if _, err := io.Copy(io.Discard, teeReader); err != nil {
		return errors.Wrap(err, "failed to process tarball")
	}

	digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	if opts.verbose {
		fmt.Fprint(wpmCli.Err(), "\n")
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

	if _, err := tempTarball.Seek(0, io.SeekStart); err != nil {
		return errors.Wrap(err, "failed to seek to the start of the tarball")
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	var reqId string
	err = wpmCli.Progress().RunWithProgress(
		"uploading tarball to registry",
		func() error {
			upload, err := registryClient.UploadTarball(context.TODO(), tempTarball, client.UploadTarballOptions{
				Acl:     opts.access,
				Name:    wpmJson.Name,
				Version: wpmJson.Version,
				Digest:  digest,
			})
			if err != nil {
				return err
			}
			if upload.ID == "" {
				return errors.New("failed to get request id from upload response")
			}
			reqId = upload.ID
			return nil
		},
		wpmCli.Err(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to upload tarball to registry")
	}

	readmeText, err := readme(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read readme file")
	}

	newPackageData := &wpm.Package{
		Config: wpm.Config{
			Name:            wpmJson.Name,
			Description:     wpmJson.Description,
			Type:            wpmJson.Type,
			Version:         wpmJson.Version,
			License:         wpmJson.License,
			Homepage:        wpmJson.Homepage,
			Tags:            wpmJson.Tags,
			Team:            wpmJson.Team,
			Bin:             wpmJson.Bin,
			Dependencies:    wpmJson.Dependencies,
			DevDependencies: wpmJson.DevDependencies,
			Scripts:         wpmJson.Scripts,
		},
		Meta: wpm.Meta{
			Dist: wpm.Dist{
				PackedSize:   int(counter.total),
				TotalFiles:   tarballer.FileCount(),
				UnpackedSize: tarballer.UnpackedSize(),
				Digest:       "sha256:" + digest,
			},
			Tag:        opts.tag,
			Visibility: opts.access,
			Readme:     readmeText,
			Wpm:        version.Version,
		},
	}

	var message string
	err = wpmCli.Progress().RunWithProgress(
		"publishing package to registry",
		func() error {
			message, err = registryClient.PublishPackage(context.TODO(), newPackageData, reqId)
			return err
		},
		wpmCli.Err(),
	)
	if err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Err(), "🚀 %s 🚀\n", aec.GreenF.Apply(message))

	return nil
}
