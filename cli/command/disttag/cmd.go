package disttag

import (
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
)

func NewDistTagCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dist-tag",
		Short:   "Manage package distribution tags",
		Aliases: []string{"dist-tags"},
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetOut(wpmCli.Out())
			cmd.HelpFunc()(cmd, args)
			return nil
		},
	}

	cmd.AddCommand(newAddCommand(wpmCli))

	return cmd
}
