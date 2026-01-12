package install

import (
	"context"
	"fmt"
	"path/filepath"
	"wpm/cli/command"
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

	// absBinDir := filepath.Join(cwd, wpmCfg.Config.BinDir)
	absContentDir := filepath.Join(cwd, wpmCfg.Config.ContentDir)

	resolver := resolution.NewResolver(wpmCfg, lock, client, runtimeWP, runtimePHP)

	resolved, err := resolver.Resolve(ctx)
	if err != nil {
		return err
	}

	plan := installer.CalculatePlan(lock, resolved, absContentDir)
	if len(plan) == 0 {
		wpmCli.Out().With(aec.GreenF).Println("Already up to date.")
	} else {
		// -- Dry Run --
		if opts.DryRun {
			for _, action := range plan {
				actionStr := "install"
				switch action.Type {
				case installer.ActionUpdate:
					actionStr = "update"
				case installer.ActionRemove:
					actionStr = "remove"
				}

				wpmCli.Out().With(aec.GreenF).Printf("%s: %s\n", actionStr, action.Name)
			}
			return nil
		}

		// -- Actual Install --
		inst := installer.New(absContentDir, 16, client)
		wpmCli.Err().With(aec.GreenF).Printf("Installing %d package(s)...\n", len(plan))

		progressFunc := func(action installer.Action) {
			actionStr := "installed"
			switch action.Type {
			case installer.ActionRemove:
				actionStr = "removed"
			case installer.ActionUpdate:
				actionStr = "updated"
			}
			fmt.Fprintf(wpmCli.Out(), "  %s %s\n", actionStr, action.Name)
		}

		if err := inst.InstallAll(ctx, plan, progressFunc); err != nil {
			return errors.Wrap(err, "installation failed")
		}
	}

	// -- Save Changes --
	if !opts.DryRun {
		if opts.SaveConfig {
			if err := wpmjson.WriteWpmJson(wpmCfg, cwd); err != nil {
				return errors.Wrap(err, "failed to save wpm.json")
			}
		}

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
	}

	return nil
}
