package cache

import (
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
)

func NewCacheCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the wpm cache",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetOut(wpmCli.Out())
			cmd.HelpFunc()(cmd, args)
			return nil
		},
	}

	cmd.AddCommand(newDirCommand(wpmCli))
	cmd.AddCommand(newLsCommand(wpmCli))
	cmd.AddCommand(newCleanCommand(wpmCli))
	cmd.AddCommand(newVerifyCommand(wpmCli))

	return cmd
}

func dirStats(dir string) (int, int64) {
	var files int
	var bytes int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes
}
