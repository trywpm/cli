package auth

import (
	"wpm/cli/command"

	"github.com/spf13/cobra"
)

func NewLogoutCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from the wpm registry",
		RunE:  func(cmd *cobra.Command, args []string) error { return runLogout(wpmCli, cmd, args) },
	}

	return cmd
}

func runLogout(wpmCli command.Cli, cmd *cobra.Command, args []string) error {
	return nil
}
