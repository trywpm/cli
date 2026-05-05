package uninstall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/command/install"
	"wpm/cli/version"
	"wpm/pkg/output"
	"wpm/pkg/pm/workspace"
	"wpm/pkg/pm/wpmjson"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewUninstallCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "uninstall [PACKAGE]...",
		Short:   "Remove dependencies from the project",
		Aliases: []string{"remove", "rm"},
		Args:    cli.RequiresMinArgs(1),
		Example: `  wpm uninstall hello-dolly`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd.Context(), wpmCli, args)
		},
	}

	return cmd
}

func runUninstall(ctx context.Context, wpmCli command.Cli, packages []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wpmCli.Output().Prettyln(output.Text{
		Plain: "wpm uninstall v" + version.Version,
		Fancy: aec.Bold.Apply("wpm uninstall") + " " + aec.LightBlackF.Apply("v"+version.Version),
	})

	contentDir := wpmjson.New().ContentDir()
	if probe, _ := wpmjson.Read(cwd); probe != nil {
		contentDir = probe.ContentDir()
	}

	lock, err := workspace.AcquireLock(ctx, filepath.Join(cwd, contentDir), func() {
		wpmCli.Output().PrettyErrorln(output.Text{
			Plain: "waiting for another wpm process to finish in this workspace...",
			Fancy: aec.Faint.Apply("waiting for another wpm process to finish in this workspace..."),
		})
	})
	if err != nil {
		return errors.Wrap(err, "failed to acquire workspace lock")
	}
	defer func() {
		_ = lock.Release()
	}()

	cfg, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if cfg == nil {
		return fmt.Errorf("no wpm.json found, so nothing to uninstall")
	}

	changed := false
	for _, name := range packages {
		if cfg.Dependencies != nil {
			if _, ok := (*cfg.Dependencies)[name]; ok {
				delete(*cfg.Dependencies, name)
				changed = true
			}
		}
		if cfg.DevDependencies != nil {
			if _, ok := (*cfg.DevDependencies)[name]; ok {
				delete(*cfg.DevDependencies, name)
				changed = true
			}
		}
	}

	if !changed {
		wpmCli.Out().WriteString("\n")
		fmt.Fprintln(wpmCli.Out(), "No matching packages found to uninstall.")
		return nil
	}

	return install.Run(ctx, cwd, wpmCli, install.RunOptions{
		Config:     cfg,
		SaveConfig: true,
		Trigger:    install.TriggerUninstall,
	})
}
