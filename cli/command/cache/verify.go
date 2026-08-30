package cache

import (
	"fmt"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/config"
	"go.wpm.so/cli/pkg/pm/cas"
)

func newVerifyCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the cached content",
		Args:  cli.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runVerify(cmd, wpmCli) },
	}

	return cmd
}

func runVerify(cmd *cobra.Command, wpmCli command.Cli) error {
	blobs := cas.Verify(cmd.Context(), config.ContentCacheDir())

	_, _ = fmt.Fprintf(wpmCli.Out(), "%d tarballs verified (%s), %d corrupt removed\n",
		blobs.Blobs, units.HumanSize(float64(blobs.Bytes)), blobs.Removed)

	return nil
}
