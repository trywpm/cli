package install

import (
	"context"
	"os"
	"strings"

	"wpm/cli/command"
	"wpm/pkg/pm/wpmjson"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type installOptions struct {
	noDev         bool
	ignoreScripts bool
	dryRun        bool
	saveDev       bool
}

func NewInstallCommand(wpmCli command.Cli) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [OPTIONS]",
		Short: "Install project dependencies",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd.Context(), wpmCli, opts, args)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noDev, "no-dev", false, "Do not install dev dependencies")
	flags.BoolVar(&opts.ignoreScripts, "ignore-scripts", false, "Do not run lifecycle scripts")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Do not write anything to disk")
	flags.BoolVarP(&opts.saveDev, "save-dev", "D", false, "Install package as a dev dependency")

	return cmd
}

func runInstall(ctx context.Context, wpmCli command.Cli, opts installOptions, packages []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	cfg, err := wpmjson.ReadAndValidateWpmJson(cwd)
	if err != nil {
		return err
	}

	setDefaultPackageConfig(cfg.Config)

	configModified := false

	if len(packages) > 0 {
		if err := addPackages(cfg, packages, opts.saveDev); err != nil {
			return err
		}

		configModified = true
	}

	return Run(ctx, cwd, wpmCli, RunOptions{
		NoDev:         opts.noDev,
		IgnoreScripts: opts.ignoreScripts,
		DryRun:        opts.dryRun,
		Config:        cfg,
		SaveConfig:    configModified,
	})
}

func addPackages(config *wpmjson.Config, packages []string, saveDev bool) error {
	for _, pkgArg := range packages {
		name, version := parsePackageArg(pkgArg)

		if saveDev {
			if config.DevDependencies == nil {
				config.DevDependencies = &wpmjson.Dependencies{}
			}

			(*config.DevDependencies)[name] = version

			if config.Dependencies != nil {
				delete(*config.Dependencies, name)
			}
		} else {
			if config.Dependencies == nil {
				config.Dependencies = &wpmjson.Dependencies{}
			}

			(*config.Dependencies)[name] = version

			if config.DevDependencies != nil {
				delete(*config.DevDependencies, name)
			}
		}
	}

	return nil
}

func setDefaultPackageConfig(pkgConfig *wpmjson.PackageConfig) {
	if pkgConfig.BinDir == "" {
		pkgConfig.BinDir = "wp-bin"
	}

	if pkgConfig.ContentDir == "" {
		pkgConfig.ContentDir = "wp-content"
	}

	if pkgConfig.RuntimeStrict == nil {
		defaultStrict := true
		pkgConfig.RuntimeStrict = &defaultStrict
	}
}

func parsePackageArg(arg string) (string, string) {
	lastAt := strings.LastIndex(arg, "@")
	if lastAt > 0 {
		return arg[:lastAt], arg[lastAt+1:]
	}
	return arg, "latest"
}
