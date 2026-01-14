package publish

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/version"
	"wpm/pkg/archive"
	"wpm/pkg/output"
	"wpm/pkg/pm/wpmignore"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmjson/manifest"
	"wpm/pkg/pm/wpmjson/types"

	"github.com/docker/go-units"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const maxReadmeSize = 50 * 1024 // 50KB

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

func pack(path string, opts publishOptions, out *output.Output) (*archive.Tarballer, error) {
	ignorePatterns, err := wpmignore.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tar, err := archive.Tar(path, &archive.TarOptions{
		ExcludePatterns: ignorePatterns,
	}, func(fileInfo os.FileInfo) {
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

			f, err := os.Open(fullPath)
			if err != nil {
				return "", err
			}
			defer f.Close()

			// Limit readme size to maxReadmeSize i.e. 50KB
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

	visibility := types.PackageVisibility(opts.access)
	if !visibility.Valid() {
		return errors.New("access must be either public or private")
	}

	wpmJson, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if err := wpmJson.Validate(); err != nil {
		return err
	}

	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}

	fmt.Fprintf(wpmCli.Err(), "📦 %s@%s\n\n", wpmJson.Name, wpmJson.Version)

	tempFile, err := os.CreateTemp("", "wpm-tarball-*.tar.zst")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary tarball")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	tarballer, err := pack(cwd, opts, wpmCli.Output())
	if err != nil {
		return errors.Wrap(err, "failed to pack the package into a tarball")
	}

	hasher := sha256.New()
	counter := &tarballSizeCounter{}
	multiWriter := io.MultiWriter(tempFile, hasher, counter)

	packTarball := func() error {
		if _, err := io.Copy(multiWriter, tarballer.Reader()); err != nil {
			return errors.Wrap(err, "failed to process tarball")
		}
		return nil
	}

	if opts.verbose {
		if err = packTarball(); err != nil {
			return err
		}
	} else {
		if err = wpmCli.Progress().RunWithProgress("packing tarball", packTarball, wpmCli.Err()); err != nil {
			return err
		}
	}

	digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	if opts.verbose {
		fmt.Fprint(wpmCli.Err(), "\n")
	}

	// bail if tarball size is zero or greater than 128mb
	if counter.total == 0 {
		return errors.New("tarball size is zero, cannot publish empty package")
	}

	if counter.total > 128*1024*1024 {
		return errors.New("tarball size exceeds 128mb, cannot publish package")
	}

	dim := aec.Faint.Apply
	blue := aec.LightBlueF.Apply
	w := tabwriter.NewWriter(wpmCli.Err(), 0, 0, 2, ' ', 0)

	packedSize := units.HumanSize(float64(counter.total))
	unpackedSize := units.HumanSize(float64(tarballer.UnpackedSize()))

	fmt.Fprintf(w, "├─ %s:\t%s\n", blue("Tag"), opts.tag)
	fmt.Fprintf(w, "├─ %s:\t%s\n", blue("Access"), opts.access)
	fmt.Fprintf(w, "├─ %s:\t%d\n", blue("Files"), tarballer.FileCount())
	fmt.Fprintf(w, "├─ %s:\t%s %s\n", blue("Size"), packedSize, dim(fmt.Sprintf("(%s unpacked)", unpackedSize)))
	fmt.Fprintf(w, "└─ %s:\t%s\n", blue("Digest"), digest)

	w.Flush()
	fmt.Fprint(wpmCli.Err(), "\n")

	if opts.dryRun {
		fmt.Fprintf(wpmCli.Err(), "dry run complete, %s@%s is ready to be published\n", wpmJson.Name, wpmJson.Version)
		return nil
	}

	cfg := wpmCli.ConfigFile()
	if cfg.DefaultUser == "" || cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	readme, err := getReadme(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read readme file")
	}

	manifest := &manifest.Package{
		Name:            wpmJson.Name,
		Description:     wpmJson.Description,
		Type:            wpmJson.Type,
		Version:         wpmJson.Version,
		Requires:        wpmJson.Requires,
		License:         wpmJson.License,
		Homepage:        wpmJson.Homepage,
		Tags:            wpmJson.Tags,
		Team:            wpmJson.Team,
		Dependencies:    wpmJson.Dependencies,
		DevDependencies: wpmJson.DevDependencies,
		Tag:             opts.tag,
		Dist: manifest.Dist{
			Digest:       "sha256:" + digest,
			PackedSize:   counter.total,
			TotalFiles:   tarballer.FileCount(),
			UnpackedSize: tarballer.UnpackedSize(),
		},
		Wpm:        version.Version,
		Visibility: visibility,
		Readme:     readme,
	}

	if err = wpmCli.Progress().RunWithProgress(
		"publishing package",
		func() error {
			if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
				return errors.Wrap(err, "failed to seek to beginning of tarball")
			}

			return registryClient.PutPackage(ctx, manifest, tempFile)
		},
		wpmCli.Err(),
	); err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Err(), "%s %s\n", aec.GreenF.Apply("✔"), "published "+wpmJson.Name+"@"+wpmJson.Version)

	return nil
}
