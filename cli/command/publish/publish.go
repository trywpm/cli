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
	"wpm/pkg/archive"
	"wpm/pkg/wpm"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type publishOptions struct {
	dryRun bool
	access string
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

	flags.BoolVar(&opts.dryRun, "dry-run", false, "Perform a publish operation without actually publishing the package")
	flags.StringVarP(&opts.access, "access", "a", "public", "Set the package access level to either public or private")

	return cmd
}

func readAndValidateWpmJson(cwd string) (*wpm.Json, error) {
	wpmJson, err := wpm.ReadWpmJson(cwd)
	if err != nil {
		return nil, err
	}

	ve, err := wpm.NewValidator()
	if err != nil {
		return nil, err
	}

	if err = wpm.ValidateWpmJson(ve, wpmJson); err != nil {
		return nil, err
	}

	return wpmJson, nil
}

func pack(path string) (io.ReadCloser, error) {
	ignorePatterns, err := wpm.ReadWpmIgnore(path)
	if err != nil {
		return nil, err
	}

	tar, err := archive.Tar(path, &archive.TarOptions{
		ShowInfo:        true,
		ExcludePatterns: ignorePatterns,
	})
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

	wpmJson, err := readAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	tarball, err := pack(cwd)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, tarball)
	if err != nil {
		return err
	}
	tarball.Close()

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

	jobId, err := registryClient.PutPackage(context.TODO(), &client.NewPackageData{
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
		Wpm:             "1.0.0",
		Digest:          (digest.FromBytes(buf.Bytes())).String(),
		Access:          opts.access,
		Attachment:      base64.StdEncoding.EncodeToString(buf.Bytes()),
		Readme:          readme,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Out(), "publishing package %s to the registry\n", jobId)

	return nil
}
