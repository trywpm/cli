package auth

import (
	"wpm/cli"
	"wpm/cli/command"

	"github.com/spf13/cobra"
)

func NewAuthCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with the wpm registry",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetOut(wpmCli.Out())
			cmd.HelpFunc()(cmd, args)
			return nil
		},
	}

	cmd.AddCommand(NewLoginCommand(wpmCli))
	cmd.AddCommand(NewLogoutCommand(wpmCli))

	return cmd
}
