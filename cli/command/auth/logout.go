package auth

import (
	"fmt"
	"wpm/cli/command"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewLogoutCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out from the wpm registry",
		RunE:  func(cmd *cobra.Command, args []string) error { return runLogout(wpmCli) },
	}

	return cmd
}

func runLogout(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()

	if cfg.AuthToken == "" {
		return errors.Errorf("user must be logged in to perform this action")
	}

	cfg.AuthToken = ""

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(wpmCli.Out(), "user logged out successfully\n")

	return nil
}
