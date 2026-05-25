package commands

import (
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/auth"
	pmInit "go.wpm.so/cli/cli/command/init"
	"go.wpm.so/cli/cli/command/install"
	"go.wpm.so/cli/cli/command/ls"
	"go.wpm.so/cli/cli/command/outdated"
	"go.wpm.so/cli/cli/command/publish"
	"go.wpm.so/cli/cli/command/uninstall"
	"go.wpm.so/cli/cli/command/whoami"
	"go.wpm.so/cli/cli/command/why"
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
		outdated.NewOutdatedCommand(wpmCli),
		uninstall.NewUninstallCommand(wpmCli),
	)
}
