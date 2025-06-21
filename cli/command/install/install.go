package install

import (
	"context"
	"os"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/pm/wpmjson"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type installOptions struct {
	noDev         bool // do not install dev dependencies
	ignoreScripts bool // do not run lifecycle scripts
	dryRun        bool // do not write anything to disk
}

func NewInstallCommand(wpmCli command.Cli) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [OPTIONS]",
		Short: "Install project dependencies",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd.Context(), wpmCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noDev, "no-dev", false, "Do not install dev dependencies")
	flags.BoolVar(&opts.ignoreScripts, "ignore-scripts", false, "Do not run lifecycle scripts")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Do not write anything to disk")

	return cmd
}

func runInstall(ctx context.Context, wpmCli command.Cli, opts installOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	// @todo: complete the install command
	_, err = wpmjson.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	return nil
}
