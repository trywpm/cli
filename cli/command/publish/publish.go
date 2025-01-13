package publish

import (
	"os"
	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/archive"
	"wpm/pkg/wpm"

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
		RunE:  func(cmd *cobra.Command, args []string) error { return runPublish(wpmCli) },
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

func tarballPackage(path string) (string, error) {
	ignorePatterns, err := wpm.ReadWpmIgnore(path)
	if err != nil {
		return "", err
	}

	tarball, err := archive.Tar(path, &archive.TarOptions{
		ExcludePatterns: ignorePatterns,
	})
	if err != nil {
		return "", err
	}

	return archive.ToBase64(tarball)
}

func runPublish(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	_, err = readAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	_, err = tarballPackage(cwd)
	if err != nil {
		return err
	}

	// TODO: Generate dist from tarball
	// - Extend archive package to report number of files in the tar with their sizes
	// - Use the tarball to generate the dist

	return nil
}
