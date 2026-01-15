package whoami

import (
	"context"
	"wpm/cli"
	"wpm/cli/command"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewWhoamiCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current user",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runWhoami(cmd.Context(), wpmCli) },
	}

	return cmd
}

func runWhoami(ctx context.Context, wpmCli command.Cli) error {
	client, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	var username string
	if err = wpmCli.Progress().RunWithProgress("", func() error {
		var err error
		username, err = client.Whoami(ctx, "")
		return err
	}, wpmCli.Err()); err != nil {
		return err
	}

	if username == "" {
		return errors.New("failed to retrieve username")
	}

	wpmCli.Out().WriteString(username + "\n")

	return nil
}
