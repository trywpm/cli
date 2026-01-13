package install

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"wpm/cli/command"
	"wpm/cli/version"
	"wpm/pkg/output"
	"wpm/pkg/pm/wpmjson"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
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
	wpmCli.Output().Prettyln(output.Text{
		Plain: "wpm install v" + version.Version,
		Fancy: aec.Bold.Apply("wpm install") + " " + aec.LightBlackF.Apply("v"+version.Version),
	})

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
		if err := addPackages(ctx, cfg, wpmCli, packages, opts.saveDev); err != nil {
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

func addPackages(ctx context.Context, config *wpmjson.Config, wpmCli command.Cli, packages []string, saveDev bool) error {
	client, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(16)

	progress := wpmCli.Progress()
	progress.StartProgressIndicator(wpmCli.Err())

	if saveDev {
		if config.DevDependencies == nil {
			config.DevDependencies = &wpmjson.Dependencies{}
		}
	} else {
		if config.Dependencies == nil {
			config.Dependencies = &wpmjson.Dependencies{}
		}
	}

	var mu sync.Mutex

	for i, pkgArg := range packages {
		name, versionOrTag := parsePackageArg(pkgArg)

		progress.Stream(wpmCli.Err(), fmt.Sprintf("  Resolving %s@%s [%d/%d]", name, versionOrTag, i+1, len(packages)))

		g.Go(func() error {
			manifest, err := client.GetPackageManifest(ctx, name, versionOrTag)
			if err != nil {
				return errors.Wrapf(err, "failed to fetch package %s@%s", name, versionOrTag)
			}

			mu.Lock()
			defer mu.Unlock()

			if saveDev {
				(*config.DevDependencies)[name] = manifest.Version

				if config.Dependencies != nil {
					delete(*config.Dependencies, name)
				}
			} else {
				(*config.Dependencies)[name] = manifest.Version

				if config.DevDependencies != nil {
					delete(*config.DevDependencies, name)
				}
			}

			return nil
		})
	}

	progress.Stream(wpmCli.Err(), "")
	progress.StopProgressIndicator()

	return g.Wait()
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
