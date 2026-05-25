package ls

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmlock"
)

type lsOptions struct {
	depth int
}

func NewLsCommand(wpmCli command.Cli) *cobra.Command {
	var opts lsOptions

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List installed dependencies",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(wpmCli, opts)
		},
	}

	cmd.Flags().IntVarP(&opts.depth, "depth", "d", -1, "Max display depth of the dependency tree")

	return cmd
}

func runLs(wpmCli command.Cli, opts lsOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	config, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}
	if config == nil {
		return errors.New("no wpm.json found, so nothing to list")
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read lockfile")
	}
	if lock == nil {
		return errors.New("no wpm.lock found, you need to run `wpm install` first")
	}

	root := config.Name
	if root == "" {
		root = filepath.Base(cwd)
	}

	rootDeps := make(map[string]string)
	if config.Dependencies != nil {
		maps.Copy(rootDeps, *config.Dependencies)
	}
	if config.DevDependencies != nil {
		maps.Copy(rootDeps, *config.DevDependencies)
	}

	if len(rootDeps) == 0 {
		return errors.New("no dependencies found in wpm.json")
	}

	wpmCli.Out().WriteString(root + "\n")

	printer := &treePrinter{
		out:      wpmCli.Out(),
		lock:     lock,
		maxDepth: opts.depth,
	}

	visited := make(map[string]bool)
	printer.printLevel(rootDeps, wpmCli.Out().IsColorEnabled(), "", 0, visited)

	return nil
}

type treePrinter struct {
	out      io.Writer
	lock     *wpmlock.Lockfile
	maxDepth int
}

// printLevel recursively prints dependencies.
func (p *treePrinter) printLevel(deps map[string]string, colorize bool, prefix string, currentDepth int, visited map[string]bool) {
	if p.maxDepth >= 0 && currentDepth > p.maxDepth {
		return
	}

	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, name := range keys {
		isLast := i == len(keys)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		isCycle := visited[name]
		info, subDeps, isMissing := p.formatNode(name, deps[name], colorize, isCycle)
		_, _ = fmt.Fprintf(p.out, "%s%s%s\n", prefix, connector, info)

		// Recurse only if:
		// 1. It's not a missing package
		// 2. It has dependencies
		// 3. We haven't seen this package in the current stack (Cycle detection)
		if !isMissing && len(subDeps) > 0 && !isCycle {
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}

			// Add current to stack, recurse, then remove (backtracking)
			visited[name] = true
			p.printLevel(subDeps, colorize, childPrefix, currentDepth+1, visited)
			delete(visited, name)
		}
	}
}

// formatNode renders the display string for a single tree node and returns its
// sub-dependencies along with whether the node is missing from the lockfile.
func (p *treePrinter) formatNode(name, requestedVersion string, colorize, isCycle bool) (info string, subDeps map[string]string, isMissing bool) {
	pkg, ok := p.lock.Packages[name]
	if !ok {
		if colorize {
			info = fmt.Sprintf("%s@%s %s", name, requestedVersion, aec.RedF.Apply("UNMET DEPENDENCY"))
		} else {
			info = fmt.Sprintf("%s@%s UNMET DEPENDENCY", name, requestedVersion)
		}
		return info, nil, true
	}

	info = fmt.Sprintf("%s@%s", name, pkg.Version)
	if colorize {
		info = fmt.Sprintf("%s%s%s", name, aec.LightBlackF.Apply("@"), aec.LightBlackF.Apply(pkg.Version))
	}

	if pkg.Version != requestedVersion && requestedVersion != "*" {
		invalidMsg := fmt.Sprintf("(invalid: \"%s\")", requestedVersion)
		if colorize {
			info += " " + aec.RedF.Apply(invalidMsg)
		} else {
			info += " " + invalidMsg
		}
	}

	if isCycle {
		cycleMsg := "(cycle)"
		if colorize {
			info += " " + aec.MagentaF.Apply(cycleMsg)
		} else {
			info += " " + cycleMsg
		}
	}

	if pkg.Dependencies != nil {
		subDeps = *pkg.Dependencies
	}
	return info, subDeps, false
}
