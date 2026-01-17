package ls

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmlock"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
	out      usersWriter
	lock     *wpmlock.Lockfile
	maxDepth int
}

type usersWriter interface {
	Write(p []byte) (n int, err error)
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
		requestedVersion := deps[name]
		isLast := i == len(keys)-1

		connector := "├── "
		if isLast {
			connector = "└── "
		}

		var info string
		var subDeps map[string]string
		var isMissing bool
		var isCycle bool

		// Check for cycles
		if visited[name] {
			isCycle = true
		}

		if pkg, ok := p.lock.Packages[name]; ok {
			info = fmt.Sprintf("%s@%s", name, pkg.Version)
			if colorize {
				info = fmt.Sprintf("%s%s%s", name, aec.LightBlackF.Apply("@"), aec.LightBlackF.Apply(pkg.Version))
			}

			if pkg.Version != requestedVersion && requestedVersion != "*" {
				if colorize {
					info += " " + aec.RedF.Apply(fmt.Sprintf("(invalid: \"%s\")", requestedVersion))
				} else {
					info += fmt.Sprintf(" (invalid: \"%s\")", requestedVersion)
				}
			}

			if isCycle {
				if colorize {
					info += aec.MagentaF.Apply(" (cycle)")
				} else {
					info += " (cycle)"
				}
			}

			if pkg.Dependencies != nil {
				subDeps = *pkg.Dependencies
			}
		} else {
			if colorize {
				info = fmt.Sprintf("%s@%s %s", name, requestedVersion, aec.RedF.Apply("UNMET DEPENDENCY"))
			} else {
				info = fmt.Sprintf("%s@%s UNMET DEPENDENCY", name, requestedVersion)
			}
			isMissing = true
		}

		fmt.Fprintf(p.out, "%s%s%s\n", prefix, connector, info)

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
