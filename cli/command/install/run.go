package install

import (
	"context"
	"fmt"
	"path/filepath"
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

type RunOptions struct {
	NoDev         bool
	IgnoreScripts bool
	DryRun        bool
	Config        *wpmjson.Config
	SaveConfig    bool
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
		wpmCfg, err = wpmjson.ReadAndValidateWpmJson(cwd)
		if err != nil {
			return err
		}
	}

	var runtimeWP, runtimePHP string
	if wpmCfg.Config != nil {
		runtimeWP = wpmCfg.Config.RuntimeWp
		runtimePHP = wpmCfg.Config.RuntimePhp
	}

	if wpmCfg.Config.RuntimeStrict == nil || *wpmCfg.Config.RuntimeStrict {
		if runtimeWP == "" {
			return errors.New("runtime-wp must be specified in wpm.json")
		}

		if runtimePHP == "" {
			return errors.New("runtime-php must be specified in wpm.json")
		}
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read lockfile")
	}
	if lock == nil {
		lock = wpmlock.New()
	}

	client, err := wpmCli.RegistryClient()
	if err != nil {
		return errors.Wrap(err, "failed to create registry client")
	}

	resolver := resolution.New(wpmCfg, lock, client, runtimeWP, runtimePHP)
	resolved, err := resolver.Resolve(ctx)
	if err != nil {
		return err
	}

	// Add empty line after resolution output for better readability
	wpmCli.Out().WriteString("\n")

	// absBinDir := filepath.Join(cwd, wpmCfg.Config.BinDir)
	absContentDir := filepath.Join(cwd, wpmCfg.Config.ContentDir)

	plan := installer.CalculatePlan(lock, resolved, absContentDir, wpmCfg, opts.NoDev)
	if len(plan) == 0 {
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
	inst := installer.New(absContentDir, 16, client)
	if err := inst.InstallAll(ctx, plan, installerProgress(wpmCli.Output())); err != nil {
		return errors.Wrap(err, "installation failed")
	}

	// @todo: binary linking

	// @todo: dependencies lifecycle scripts

	// -- Update Lockfile --
	newLock := wpmlock.New()
	for name, node := range resolved {
		newLock.Packages[name] = wpmlock.LockPackage{
			Version:      node.Version,
			Resolved:     node.Resolved,
			Digest:       node.Digest,
			Type:         node.Type,
			Bin:          node.Bin,
			Dependencies: node.Dependencies,
		}
	}
	if err := newLock.Write(cwd); err != nil {
		return errors.Wrap(err, "failed to save lockfile")
	}

	// @todo: run root lifecycle scripts

	// -- Save wpm.json --
	if opts.SaveConfig {
		if err := wpmjson.WriteWpmJson(wpmCfg, cwd); err != nil {
			return errors.Wrap(err, "failed to save wpm.json")
		}
	}

	// -- Print Summary --
	wpmCli.Output().Prettyln(output.Text{
		Plain: fmt.Sprintf("\n%d %s installed", len(plan), command.Pluralize("package", "s", len(plan))),
		Fancy: fmt.Sprintf("\n%s %s installed", aec.GreenF.Apply(strconv.Itoa(len(plan))), command.Pluralize("package", "s", len(plan))),
	})

	return nil
}
