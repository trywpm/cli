package publish

import (
	"fmt"
	"wpm/cli"
	"wpm/cli/command"
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

func runPublish(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()
	if cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}

	wpm, err := wpm.NewWpm(true)
	if err != nil {
		return err
	}

	err = wpm.Validate()
	if err != nil {
		_, _ = fmt.Fprintf(wpmCli.Err(), "error validating wpm.json\n")
		return err
	}

	// TODO: Bail if package is private
	// update validator.Package to have Private field in it with type bool
	// do something about the empty line before the config validation error messages. It's weird.

	return nil
}
