package outdated

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/version"
	"wpm/pkg/output"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmlock"

	"github.com/Masterminds/semver/v3"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func NewOutdatedCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Check for outdated dependencies",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOutdated(cmd.Context(), wpmCli)
		},
	}
	return cmd
}

type depCheck struct {
	name    string
	version string
	isDev   bool
}

func runOutdated(ctx context.Context, wpmCli command.Cli) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	config, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if config == nil {
		return errors.New("no wpm.json found, so nothing to check")
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read lockfile")
	}
	if lock == nil {
		return errors.New("no wpm.lock found. Run 'wpm install' first to generate a lockfile.")
	}

	wpmCli.Output().Prettyln(output.Text{
		Plain: "wpm outdated v" + version.Version,
		Fancy: aec.Bold.Apply("wpm outdated") + " " + aec.LightBlackF.Apply("v"+version.Version),
	})

	var checks []depCheck

	if config.Dependencies != nil {
		for name := range *config.Dependencies {
			if pkg, ok := lock.Packages[name]; ok {
				checks = append(checks, depCheck{name, pkg.Version, false})
			}
		}
	}
	if config.DevDependencies != nil {
		for name := range *config.DevDependencies {
			if pkg, ok := lock.Packages[name]; ok {
				checks = append(checks, depCheck{name, pkg.Version, true})
			}
		}
	}

	if len(checks) == 0 {
		return nil
	}

	results, err := findOutdatedPackages(ctx, config, wpmCli, checks)
	if err != nil {
		return err
	}

	wpmCli.Out().WriteString("\n")

	if len(results) == 0 {
		wpmCli.Out().WriteString("Already up-to-date!\n")
		return nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	printOutdatedList(wpmCli.Out(), wpmCli.Out().IsColorEnabled(), results)

	return nil
}

type outdatedInfo struct {
	name     string
	current  string
	latest   string
	pkgType  string
	isDev    bool
	diffType string // major, minor, patch, or unknown
}

func findOutdatedPackages(ctx context.Context, config *wpmjson.Config, wpmCli command.Cli, checks []depCheck) ([]outdatedInfo, error) {
	client, err := wpmCli.RegistryClient()
	if err != nil {
		return nil, err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(16) // Limit concurrency

	progress := wpmCli.Progress()
	progress.StartProgressIndicator(wpmCli.Err())
	defer func() {
		progress.Stream(wpmCli.Err(), "")
		progress.StopProgressIndicator()
	}()

	var (
		mu      sync.Mutex
		results []outdatedInfo
	)

	for i, check := range checks {
		progress.Stream(wpmCli.Err(), fmt.Sprintf("  Resolving %s@%s [%d/%d]", check.name, "latest", i+1, len(checks)))

		g.Go(func() error {
			manifest, err := client.GetPackageManifest(ctx, check.name, "latest", true)
			if err != nil {
				return errors.Wrapf(err, "failed to fetch package %s@%s", check.name, "latest")
			}

			currentVer, err1 := semver.NewVersion(check.version)
			latestVer, err2 := semver.NewVersion(manifest.Version)

			if err1 == nil && err2 == nil && latestVer.GreaterThan(currentVer) {
				diff := getDiffType(check.version, manifest.Version)

				info := outdatedInfo{
					name:     check.name,
					current:  check.version,
					latest:   manifest.Version,
					pkgType:  string(manifest.Type),
					isDev:    check.isDev,
					diffType: diff,
				}

				mu.Lock()
				results = append(results, info)
				mu.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

func getDiffType(current, latest string) string {
	currV, err1 := semver.NewVersion(current)
	latestV, err2 := semver.NewVersion(latest)

	if err1 != nil || err2 != nil {
		return "unknown"
	}

	if latestV.Major() > currV.Major() {
		return "major"
	}
	if latestV.Minor() > currV.Minor() {
		return "minor"
	}
	if latestV.Patch() > currV.Patch() {
		return "patch"
	}

	return "unknown"
}

func printOutdatedList(out io.Writer, colorize bool, results []outdatedInfo) {
	c := func(a aec.ANSI, s string) string {
		if !colorize {
			return s
		}
		return a.Apply(s)
	}

	for i, r := range results {
		nameStr := c(aec.Bold, r.name)
		typeStr := c(aec.CyanF, fmt.Sprintf("[%s]", r.pkgType))

		devStr := ""
		if r.isDev {
			devStr = c(aec.Faint, "(dev)")
		}

		fmt.Fprintf(out, "%s %s %s\n", nameStr, typeStr, devStr)

		var diffLabel string
		var severityColor aec.ANSI

		switch r.diffType {
		case "major":
			severityColor = aec.RedF
			diffLabel = "(major update)"
		case "minor":
			severityColor = aec.YellowF
			diffLabel = "(minor update)"
		case "patch":
			severityColor = aec.GreenF
			diffLabel = "(patch update)"
		default:
			severityColor = aec.DefaultF
			diffLabel = "(unknown update)"
		}

		treeEnd := c(aec.LightBlackF, "└──")
		treeBranch := c(aec.LightBlackF, "├──")

		fmt.Fprintf(out, "%s current: %s\n",
			treeBranch,
			r.current,
		)

		fmt.Fprintf(out, "%s latest:  %s %s\n",
			treeEnd,
			c(severityColor, r.latest),  // Colorized Version
			c(severityColor, diffLabel), // Colorized Label
		)

		if i < len(results)-1 {
			fmt.Fprintln(out, "")
		}
	}
}
