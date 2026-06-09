package install

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/morikuni/aec"

	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/output"
	"go.wpm.so/cli/pkg/pm/installer"
	"go.wpm.so/cli/pkg/pm/resolution"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmlock"
)

type Trigger int

const (
	TriggerUpdate Trigger = iota
	TriggerInstall
	TriggerUninstall
)

type RunOptions struct {
	NoDev              bool
	IgnoreScripts      bool
	DryRun             bool
	Config             *wpmjson.Config
	SaveConfig         bool
	NetworkConcurrency int
	Trigger            Trigger
}

func installerProgress(out *output.Output) func(action installer.Action) {
	return func(action installer.Action) {
		actionStr := "+"
		color := aec.GreenF
		switch action.Type {
		case installer.ActionRemove:
			actionStr = "-"
			color = aec.RedF
		case installer.ActionUpdate:
			actionStr = "+" // we use "+" for updates as well to indicate addition of new version
			color = aec.YellowF
		case installer.ActionInstall:
			// keep defaults
		}

		out.Prettyln(output.Text{
			Plain: fmt.Sprintf("%s %s@%s", actionStr, action.Name, action.Version),
			Fancy: fmt.Sprintf("%s %s %s", color.Apply(actionStr), aec.Bold.Apply(action.Name), action.Version),
		})
	}
}

func Run(ctx context.Context, cwd string, wpmCli command.Cli, opts RunOptions) error {
	var err error

	wpmCfg := opts.Config
	if wpmCfg == nil {
		return errors.New("wpm.json config is required")
	}
	if err := wpmCfg.ValidateDependencyNames(); err != nil {
		return fmt.Errorf("invalid dependency name in wpm.json: %w", err)
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return fmt.Errorf("failed to read lockfile: %w", err)
	}
	if lock == nil {
		lock = wpmlock.New()
	}

	// Set lockfile indentation based on wpm.json formatting
	lock.SetIndentation(wpmCfg.GetIndentation())

	client, err := wpmCli.RegistryClient()
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	resolver := resolution.New(wpmCfg, lock, client)
	resolved, err := resolver.Resolve(ctx, wpmCli.Progress(), wpmCli.Err())
	if err != nil {
		return err
	}

	// Add empty line after resolution output for better readability
	wpmCli.Out().WriteString("\n")

	// absBinDir := filepath.Join(cwd, wpmCfg.BinDir())
	absContentDir := filepath.Join(cwd, wpmCfg.ContentDir())

	plan := installer.CalculatePlan(lock, resolved, absContentDir, wpmCfg, opts.NoDev)
	if len(plan) == 0 {
		if opts.SaveConfig {
			if err := wpmCfg.Write(cwd); err != nil {
				return fmt.Errorf("failed to save wpm.json: %w", err)
			}
		}

		wpmCli.Out().WriteString("Already up-to-date!\n")
		return nil
	}

	if opts.DryRun {
		printDryRunPlan(wpmCli, plan)
		return nil
	}

	// -- Actual Install --
	inst, err := installer.New(ctx, absContentDir, opts.NetworkConcurrency, client, func(format string, args ...any) {
		wpmCli.Output().ErrorWrite(fmt.Sprintf(format+"\n", args...))
	})
	if err != nil {
		return fmt.Errorf("failed to initialize installer: %w", err)
	}
	defer func() { _ = inst.Close() }()

	if err := inst.InstallAll(ctx, plan, installerProgress(wpmCli.Output())); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// @todo: binary linking

	// @todo: dependencies lifecycle scripts

	updateLockPackages(lock, resolved)
	if err := lock.Write(cwd); err != nil {
		return fmt.Errorf("failed to save lockfile: %w", err)
	}

	// @todo: run root lifecycle scripts

	if opts.SaveConfig {
		if err := wpmCfg.Write(cwd); err != nil {
			return fmt.Errorf("failed to save wpm.json: %w", err)
		}
	}

	printRunSummary(wpmCli, opts.Trigger, len(plan))
	return nil
}

func printDryRunPlan(wpmCli command.Cli, plan []installer.Action) {
	for _, action := range plan {
		installerProgress(wpmCli.Output())(action)
	}
	totalPackages := len(plan)
	wpmCli.Output().Prettyln(output.Text{
		Plain: fmt.Sprintf("\n%d %s can be installed", totalPackages, command.Pluralize("package", "s", totalPackages)),
		Fancy: fmt.Sprintf("\n%s %s can be installed", aec.GreenF.Apply(strconv.Itoa(totalPackages)), command.Pluralize("package", "s", totalPackages)),
	})
}

func updateLockPackages(lock *wpmlock.Lockfile, resolved map[string]resolution.Node) {
	lock.Packages = make(map[string]wpmlock.LockPackage, len(resolved))
	for _, name := range slices.Sorted(maps.Keys(resolved)) {
		node := resolved[name]
		lock.Packages[name] = wpmlock.LockPackage{
			Version:      node.Version,
			Signatures:   node.Signatures,
			Digest:       node.Digest,
			Type:         node.Type,
			Bin:          node.Bin,
			Dependencies: node.Dependencies,
		}
	}
}

func printRunSummary(wpmCli command.Cli, trigger Trigger, count int) {
	var action string
	switch trigger {
	case TriggerInstall:
		action = "installed"
	case TriggerUpdate:
		action = "updated"
	case TriggerUninstall:
		action = "uninstalled"
	}

	if action == "" {
		return
	}
	wpmCli.Output().Prettyln(output.Text{
		Plain: fmt.Sprintf("\n%d %s %s", count, command.Pluralize("package", "s", count), action),
		Fancy: fmt.Sprintf("\n%s %s %s", aec.GreenF.Apply(strconv.Itoa(count)), command.Pluralize("package", "s", count), action),
	})
}
