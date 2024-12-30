package commands

import (
	"wpm/cli/command"
	pmInit "wpm/cli/command/init"

	"github.com/spf13/cobra"
)

func AddCommands(cmd *cobra.Command, wpmCli command.Cli) {
	cmd.AddCommand(pmInit.NewInitCommand(wpmCli))
}
