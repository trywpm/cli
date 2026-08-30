package cache

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/config"
)

func newCleanCommand(wpmCli command.Cli) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "clean",
		Short:   "Remove all cached data",
		Args:    cli.NoArgs,
		Aliases: []string{"clear"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, wpmCli, force)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	return cmd
}

func runClean(cmd *cobra.Command, wpmCli command.Cli, force bool) error {
	out := wpmCli.Out()

	if !force {
		_, _ = fmt.Fprintln(wpmCli.Err(), "WARNING! This will remove all cached package data.")
		ok, err := command.PromptForConfirmation(cmd.Context(), wpmCli.In(), wpmCli.Err(), "Are you sure you want to continue?")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		_, _ = fmt.Fprintln(wpmCli.Err())
	}

	var reclaimed int64
	blobRoot := filepath.Join(config.ContentCacheDir(), "sha256")
	_ = filepath.WalkDir(blobRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if os.Remove(path) == nil { //nolint:gosec // the path comes from walking our own store
			reclaimed += info.Size()
			_, _ = fmt.Fprintln(out, "deleted: sha256:"+d.Name())
		}
		return nil
	})

	for _, dir := range []string{config.ContentCacheDir(), config.ManifestCacheDir()} {
		_, bytes := dirStats(dir)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("failed to remove %s: %w", dir, err)
		}
		reclaimed += bytes
	}

	_, _ = fmt.Fprintf(out, "\nTotal reclaimed space: %s\n", units.HumanSize(float64(reclaimed)))

	return nil
}
