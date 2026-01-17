package commands

import (
	"wpm/cli/command"
	"wpm/cli/command/auth"
	pmInit "wpm/cli/command/init"
	"wpm/cli/command/install"
	"wpm/cli/command/ls"
	"wpm/cli/command/publish"
	"wpm/cli/command/uninstall"
	"wpm/cli/command/whoami"
	"wpm/cli/command/why"

	"github.com/spf13/cobra"
)

func AddCommands(cmd *cobra.Command, wpmCli command.Cli) {
	cmd.AddCommand(
		ls.NewLsCommand(wpmCli),
		why.NewWhyCommand(wpmCli),
		auth.NewAuthCommand(wpmCli),
		pmInit.NewInitCommand(wpmCli),
		whoami.NewWhoamiCommand(wpmCli),
		publish.NewPublishCommand(wpmCli),
		install.NewInstallCommand(wpmCli),
		uninstall.NewUninstallCommand(wpmCli),
	)
}
