package publish

import (
	"bytes"
	"context"
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
	"github.com/opencontainers/go-digest"
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

func pack(dst io.Writer, path string, opts publishOptions) (*archive.Tarballer, error) {
	ignorePatterns, err := wpm.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tar, err := archive.Tar(path, &archive.TarOptions{
		ShowInfo:        opts.verbose,
		ExcludePatterns: ignorePatterns,
	}, dst)
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

func runPublish(wpmCli command.Cli, opts publishOptions) error {
	if opts.access != "public" && opts.access != "private" {
		return errors.New("access must be either public or private")
	}

	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wpmJson, err := wpm.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	if wpmJson.Private {
		return errors.New("package marked as private cannot be published")
	}

	fmt.Fprintf(wpmCli.Err(), aec.CyanF.Apply("📦 preparing %s@%s for publishing 📦\n\n"), wpmJson.Name, wpmJson.Version)

	tarball, err := pack(wpmCli.Err(), cwd, opts)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	tarReader := tarball.Reader()
	_, err = io.Copy(buf, tarReader)
	if err != nil {
		return err
	}
	tarReader.Close()

	digest := digest.FromBytes(buf.Bytes())

	if opts.verbose {
		fmt.Fprint(wpmCli.Err(), "\n") // add a newline after the tarball progress
	}

	fmt.Fprintf(wpmCli.Err(), "%s: %d\n", aec.LightBlueF.Apply("total files"), tarball.FileCount())
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("digest"), digest.String())
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("unpacked size"), units.HumanSize(float64(tarball.UnpackedSize())))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("packed size"), units.HumanSize(float64(buf.Len())))
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("tag"), opts.tag)
	fmt.Fprintf(wpmCli.Err(), "%s: %s\n", aec.LightBlueF.Apply("access"), opts.access)
	fmt.Fprint(wpmCli.Err(), "\n")

	if opts.dryRun {
		fmt.Fprintf(wpmCli.Err(), "dry run complete, %s@%s is ready to be published\n", wpmJson.Name, wpmJson.Version)
		return nil
	}

	readme, err := readme(cwd)
	if err != nil {
		return err
	}

	registryClient, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	ver := version.Version
	if ver == "unknown-version" {
		ver = "0.1.0-dev"
	}

	newPackageData := &client.NewPackageData{
		Name:            wpmJson.Name,
		Description:     wpmJson.Description,
		Type:            wpmJson.Type,
		Version:         wpmJson.Version,
		License:         wpmJson.License,
		Homepage:        wpmJson.Homepage,
		Tags:            wpmJson.Tags,
		Team:            wpmJson.Team,
		Bin:             wpmJson.Bin,
		Platform:        wpmJson.Platform,
		Dependencies:    wpmJson.Dependencies,
		DevDependencies: wpmJson.DevDependencies,
		Scripts:         wpmJson.Scripts,
		Wpm:             ver,
		Digest:          digest.String(),
		Access:          opts.access,
		Attachment:      base64.StdEncoding.EncodeToString(buf.Bytes()),
		Readme:          readme,
	}

	var message string
	err = wpmCli.Progress().RunWithProgress(
		"adding package to publish queue",
		func() error {
			message, err = registryClient.PutPackage(context.TODO(), newPackageData)
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
