package auth

import (
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
)

func NewAuthCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the wpm registry",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetOut(wpmCli.Out())
			cmd.HelpFunc()(cmd, args)
			return nil
		},
		Annotations: map[string]string{
			"category-top": "1",
		},
	}

	cmd.AddCommand(NewLoginCommand(wpmCli))
	cmd.AddCommand(NewLogoutCommand(wpmCli))

	return cmd
}
