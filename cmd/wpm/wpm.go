package main

import (
	"fmt"
	"wpm/cli/command"
	pmInit "wpm/cli/command/init"

	"github.com/spf13/cobra"
)

func main() {
	wpmCli, err := command.NewWpmCli()

	if err != nil {
		fmt.Println(err)
		return
	}

	cmd := newWpmCommand(wpmCli)

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func newWpmCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "wpm [OPTIONS] COMMAND [ARG...]",
		Short:            "WordPress Package Manager",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("wpm: unknown command: wpm %s\n\nRun 'wpm --help' for more information on a command", args[0])
		},
		Version: "0.1.0",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   false,
			HiddenDefaultCmd:    true,
			DisableDescriptions: true,
		},
	}

	addCommands(cmd, wpmCli)

	return cmd
}

func addCommands(cmd *cobra.Command, wpmCli command.Cli) {
	cmd.AddCommand(pmInit.NewInitCommand(wpmCli))
}
