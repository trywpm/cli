package commands

import (
	"wpm/cli/command"
	"wpm/cli/command/auth"
	pmInit "wpm/cli/command/init"

	"github.com/spf13/cobra"
)

func AddCommands(cmd *cobra.Command, wpmCli command.Cli) {
	cmd.AddCommand(
		auth.NewAuthCommand(wpmCli),
		pmInit.NewInitCommand(wpmCli),
	)
}
