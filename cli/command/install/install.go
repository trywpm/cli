package install

import (
	"context"
	"os"

	"wpm/cli/command"
	"wpm/pkg/wpm"

	"github.com/spf13/cobra"
)

type installOptions struct {
	packages []string

	saveDev       bool // save as dev dependency
	noDev         bool // do not install dev dependencies
	noLockfile    bool // do not write to lockfile
	ignoreScripts bool // do not run lifecycle scripts
	noProgress    bool // do not show progress bar -- maybe make it global
	dryRun        bool // do not write anything to disk
}

func NewInstallCommand(wpmCli command.Cli) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [OPTIONS] [PACKAGE[@VERSION]]",
		Short: "Install project dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.packages = args
			}

			return runInstall(cmd.Context(), wpmCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.saveDev, "save-dev", "D", false, "Save as dev dependency")
	flags.BoolVar(&opts.noDev, "no-dev", false, "Do not install dev dependencies")
	flags.BoolVar(&opts.noLockfile, "no-lockfile", false, "Do not use lockfile or write to it")
	flags.BoolVar(&opts.ignoreScripts, "ignore-scripts", false, "Do not run lifecycle scripts")
	flags.BoolVar(&opts.noProgress, "no-progress", false, "Do not show progress bar")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Do not write anything to disk")

	return cmd
}

// runInstallWithPackages installs the packages passed by the user
func runInstallWithPackages(ctx context.Context, wpmCli command.Cli, opts installOptions) error {
	return nil
}

// runInstallFromWpmJson installs the packages from the wpm.json file
func runInstallFromWpmJson(ctx context.Context, wpmCli command.Cli, opts installOptions) error {
	return nil
}

func runInstall(ctx context.Context, wpmCli command.Cli, opts installOptions) error {
	var err error
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wpmJson, err := wpm.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	var lj *wpm.LockJson
	var newLockfile bool = false
	var hasLockfile bool = !opts.noLockfile
	if hasLockfile {
		lj, err = wpm.ReadAndValidateWpmLock(cwd)
		if err != nil {
			switch err.(type) {
			case *wpm.LockfileNotFound:
				hasLockfile = false
			case *wpm.CorruptLockfile:
				newLockfile = true
			default:
				return err
			}
		} else {
			if lj.LockfileVersion != 1 {
				newLockfile = true
			}

			if lj.Platform != wpmJson.Platform {
				newLockfile = true
			}

			if !equalMaps(lj.Dependencies, wpmJson.Dependencies) {
				newLockfile = true
			}

			if !equalMaps(lj.DevDependencies, wpmJson.DevDependencies) {
				newLockfile = true
			}
		}
	}

	if newLockfile || !hasLockfile {
		if lj == nil {
			lj = &wpm.LockJson{
				LockfileVersion: 1, // maybe get this from a constant?
				Dependencies:    make(map[string]string),
				DevDependencies: make(map[string]string),
				Packages:        make(wpm.PackagesMap),
				Platform:        wpm.Platform{},
			}
		}
	}

	// todo: pass required information to install, like wpmJson, lj, opts, hasLockfile, newLockfile etc.
	// see you tomorrow :)
	if len(opts.packages) > 0 {
		return runInstallWithPackages(ctx, wpmCli, opts)
	} else {
		return runInstallFromWpmJson(ctx, wpmCli, opts)
	}
}

// equalMaps checks if two maps are equal
func equalMaps(x, y map[string]string) bool {
	if len(x) != len(y) {
		return false
	}

	for k, xv := range x {
		yv, ok := y[k]
		if !ok || xv != yv {
			return false
		}
	}

	return true
}
