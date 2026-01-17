package uninstall

import (
	"context"
	"fmt"
	"os"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/command/install"
	"wpm/cli/version"
	"wpm/pkg/output"
	"wpm/pkg/pm/wpmjson"

	"github.com/morikuni/aec"
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

	cfg, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if cfg == nil {
		return fmt.Errorf("no wpm.json found, nothing to uninstall")
	}

	wpmCli.Output().Prettyln(output.Text{
		Plain: "wpm uninstall v" + version.Version,
		Fancy: aec.Bold.Apply("wpm uninstall") + " " + aec.LightBlackF.Apply("v"+version.Version),
	})

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
