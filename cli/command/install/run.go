package install

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"wpm/cli/command"
	"wpm/pkg/output"
	"wpm/pkg/pm/installer"
	"wpm/pkg/pm/resolution"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmlock"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
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

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read lockfile")
	}
	if lock == nil {
		lock = wpmlock.New()
	}

	// Set lockfile indentation based on wpm.json formatting
	lock.SetIndentation(wpmCfg.GetIndentation())

	client, err := wpmCli.RegistryClient()
	if err != nil {
		return errors.Wrap(err, "failed to create registry client")
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
				return errors.Wrap(err, "failed to save wpm.json")
			}
		}

		wpmCli.Out().WriteString("Already up-to-date!\n")
		return nil
	}

	// -- Dry Run --
	if opts.DryRun {
		for _, action := range plan {
			installerProgress(wpmCli.Output())(action)
		}
		totalPackages := len(plan)

		wpmCli.Output().Prettyln(output.Text{
			Plain: fmt.Sprintf("\n%d %s can be installed", totalPackages, command.Pluralize("package", "s", totalPackages)),
			Fancy: fmt.Sprintf("\n%s %s can be installed", aec.GreenF.Apply(strconv.Itoa(totalPackages)), command.Pluralize("package", "s", totalPackages)),
		})

		return nil
	}

	// -- Actual Install --
	inst := installer.New(absContentDir, opts.NetworkConcurrency, client)
	if err := inst.InstallAll(ctx, plan, installerProgress(wpmCli.Output())); err != nil {
		return errors.Wrap(err, "installation failed")
	}

	// @todo: binary linking

	// @todo: dependencies lifecycle scripts

	// -- Update Lockfile --
	lock.Packages = make(map[string]wpmlock.LockPackage, len(resolved))
	for _, name := range slices.Sorted(maps.Keys(resolved)) {
		node := resolved[name]
		lock.Packages[name] = wpmlock.LockPackage{
			Version:      node.Version,
			Resolved:     node.Resolved,
			Digest:       node.Digest,
			Type:         node.Type,
			Bin:          node.Bin,
			Dependencies: node.Dependencies,
		}
	}

	if err := lock.Write(cwd); err != nil {
		return errors.Wrap(err, "failed to save lockfile")
	}

	// @todo: run root lifecycle scripts

	// -- Save wpm.json --
	if opts.SaveConfig {
		if err := wpmCfg.Write(cwd); err != nil {
			return errors.Wrap(err, "failed to save wpm.json")
		}
	}

	// -- Print Summary --
	var action string
	switch opts.Trigger {
	case TriggerInstall:
		action = "installed"
	case TriggerUpdate:
		action = "updated"
	case TriggerUninstall:
		action = "uninstalled"
	}

	if action != "" {
		wpmCli.Output().Prettyln(output.Text{
			Plain: fmt.Sprintf("\n%d %s %s", len(plan), command.Pluralize("package", "s", len(plan)), action),
			Fancy: fmt.Sprintf("\n%s %s %s", aec.GreenF.Apply(strconv.Itoa(len(plan))), command.Pluralize("package", "s", len(plan)), action),
		})
	}

	return nil
}
