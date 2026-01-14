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
	"wpm/pkg/pm/wpmjson/types"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type installOptions struct {
	noDev              bool
	ignoreScripts      bool
	dryRun             bool
	saveDev            bool
	saveProd           bool
	networkConcurrency int
}

func NewInstallCommand(wpmCli command.Cli) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [OPTIONS]",
		Short: "Install project dependencies",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runInstall(cmd.Context(), wpmCli, opts, args)
			if err != nil {
				suffix := "error:"
				if wpmCli.Out().IsColorEnabled() {
					suffix = aec.RedF.Apply("error:")
				}

				err = fmt.Errorf("%s %w", suffix, err)
			}
			return err
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noDev, "no-dev", false, "Do not install dev dependencies")
	flags.BoolVar(&opts.ignoreScripts, "ignore-scripts", false, "Do not run lifecycle scripts")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "Do not write anything to disk")
	flags.BoolVarP(&opts.saveDev, "save-dev", "D", false, "Install package as a dev dependency")
	flags.BoolVarP(&opts.saveProd, "save-prod", "P", false, "Install package as a production dependency (default behavior)")
	flags.IntVar(&opts.networkConcurrency, "network-concurrency", 16, "Number of concurrent network requests when installing packages (default 16)")

	cmd.MarkFlagsMutuallyExclusive("no-dev", "save-dev")
	cmd.MarkFlagsMutuallyExclusive("no-dev", "save-prod")
	cmd.MarkFlagsMutuallyExclusive("save-dev", "save-prod")

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

	cfg, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if cfg == nil {
		cfg = wpmjson.New()
	}

	configModified := false

	if len(packages) > 0 {
		if err := addPackages(ctx, cfg, wpmCli, packages, opts); err != nil {
			return err
		}

		// If dependencies or devDependencies still have zero entries, set them to nil
		if cfg.Dependencies != nil && len(*cfg.Dependencies) == 0 {
			cfg.Dependencies = nil
		}
		if cfg.DevDependencies != nil && len(*cfg.DevDependencies) == 0 {
			cfg.DevDependencies = nil
		}

		configModified = true
	}

	return Run(ctx, cwd, wpmCli, RunOptions{
		NoDev:              opts.noDev,
		IgnoreScripts:      opts.ignoreScripts,
		DryRun:             opts.dryRun,
		Config:             cfg,
		SaveConfig:         configModified,
		NetworkConcurrency: opts.networkConcurrency,
	})
}

func addPackages(ctx context.Context, config *wpmjson.Config, wpmCli command.Cli, packages []string, opts installOptions) error {
	client, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.networkConcurrency)

	progress := wpmCli.Progress()
	progress.StartProgressIndicator(wpmCli.Err())
	defer func() {
		progress.Stream(wpmCli.Err(), "")
		progress.StopProgressIndicator()
	}()

	if config.DevDependencies == nil {
		config.DevDependencies = &types.Dependencies{}
	}

	if config.Dependencies == nil {
		config.Dependencies = &types.Dependencies{}
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

			if opts.saveDev {
				(*config.DevDependencies)[name] = manifest.Version
				delete(*config.Dependencies, name)
			} else if opts.saveProd {
				(*config.Dependencies)[name] = manifest.Version
				delete(*config.DevDependencies, name)
			} else {
				if _, exists := (*config.DevDependencies)[name]; exists {
					(*config.DevDependencies)[name] = manifest.Version
				} else {
					(*config.Dependencies)[name] = manifest.Version
				}
			}

			return nil
		})
	}

	return g.Wait()
}

func parsePackageArg(arg string) (string, string) {
	lastAt := strings.LastIndex(arg, "@")
	if lastAt > 0 {
		return arg[:lastAt], arg[lastAt+1:]
	}
	return arg, "latest"
}
