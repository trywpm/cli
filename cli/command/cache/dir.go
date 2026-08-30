package cache

import (
	"fmt"

	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/config"
)

func newDirCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dir",
		Short: "Print the cache directory",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runDir(wpmCli) },
	}

	return cmd
}

func runDir(wpmCli command.Cli) error {
	_, _ = fmt.Fprintln(wpmCli.Out(), config.CacheDir())
	return nil
}
