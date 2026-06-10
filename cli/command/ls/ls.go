package ls

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"github.com/morikuni/aec"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/completion"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmlock"
)

const (
	unknownType = "unknown"

	connectorMid = "├── "
	connectorEnd = "└── "
	indentMid    = "│   "
	indentEnd    = "    "
)

// typeOrder is the display order for known package types; others follow sorted.
var typeOrder = []string{types.TypePlugin.String(), types.TypeTheme.String()}

type lsOptions struct {
	depth      int
	filterType string
}

func NewLsCommand(wpmCli command.Cli) *cobra.Command {
	var opts lsOptions

	cmd := &cobra.Command{
		Use:               "ls [OPTIONS] [plugin|theme]",
		Short:             "List installed dependencies",
		Args:              cli.RequiresMaxArgs(1),
		ValidArgsFunction: completion.PackageTypes(),
		Aliases:           []string{"list", "tree"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.filterType = args[0]
				if !types.PackageType(opts.filterType).Valid() {
					return fmt.Errorf("invalid type %q: must be theme or plugin", opts.filterType)
				}
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
			return runLs(wpmCli, cwd, opts)
		},
	}

	cmd.Flags().IntVarP(&opts.depth, "depth", "d", -1, "Max display depth of the dependency tree")

	return cmd
}

func runLs(wpmCli command.Cli, cwd string, opts lsOptions) (err error) {
	config, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}
	if config == nil {
		return errors.New("no wpm.json found, so nothing to list")
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return err
	}
	if lock == nil {
		return errors.New("no wpm.lock found, you need to run `wpm install` first")
	}

	root := config.Name
	if root == "" {
		root = filepath.Base(cwd)
	}

	capHint := depLen(config.Dependencies) + depLen(config.DevDependencies)
	if capHint == 0 {
		return errors.New("no dependencies found in wpm.json")
	}

	rootDeps := make(map[string]string, capHint)
	if config.Dependencies != nil {
		maps.Copy(rootDeps, *config.Dependencies)
	}
	if config.DevDependencies != nil {
		maps.Copy(rootDeps, *config.DevDependencies)
	}

	groups := groupByType(rootDeps, lock, opts.filterType)
	if len(groups) == 0 {
		return fmt.Errorf("no %s dependencies found", opts.filterType)
	}

	out := bufio.NewWriter(wpmCli.Out())
	defer func() {
		if flushErr := out.Flush(); flushErr != nil && err == nil {
			err = flushErr
		}
	}()

	printer := &treePrinter{
		out:      out,
		lock:     lock,
		maxDepth: opts.depth,
		colorize: wpmCli.Out().IsColorEnabled(),
	}

	_, _ = fmt.Fprintln(out, root)

	visited := make(map[string]bool)
	var prefix []byte
	for i, g := range groups {
		connector, indent := connectorMid, indentMid
		if i == len(groups)-1 {
			connector, indent = connectorEnd, indentEnd
		}

		label := g.label
		if printer.colorize {
			label = aec.Bold.Apply(label)
		}
		_, _ = fmt.Fprintf(out, "%s%s\n", connector, label)

		prefix = append(prefix[:0], indent...)
		printer.printLevel(g.deps, prefix, 0, visited)
	}

	return nil
}

func depLen(d *types.Dependencies) int {
	if d == nil {
		return 0
	}
	return len(*d)
}

// depGroup is a set of direct dependencies that share a package type.
type depGroup struct {
	label string
	deps  map[string]string
}

// groupByType partitions direct dependencies by their locked package type,
// known types first then any others sorted.
func groupByType(deps map[string]string, lock *wpmlock.Lockfile, filter string) []depGroup {
	buckets := make(map[string]map[string]string)
	for name, version := range deps {
		label := unknownType
		if pkg, ok := lock.Packages[name]; ok && pkg.Type != "" {
			label = pkg.Type.String()
		}
		if filter != "" && label != filter {
			continue
		}
		bucket := buckets[label]
		if bucket == nil {
			bucket = make(map[string]string)
			buckets[label] = bucket
		}
		bucket[name] = version
	}

	groups := make([]depGroup, 0, len(buckets))
	for _, label := range typeOrder {
		if bucket := buckets[label]; len(bucket) > 0 {
			groups = append(groups, depGroup{label, bucket})
			delete(buckets, label)
		}
	}

	rest := make([]string, 0, len(buckets))
	for label := range buckets {
		rest = append(rest, label)
	}
	sort.Strings(rest)
	for _, label := range rest {
		groups = append(groups, depGroup{label, buckets[label]})
	}

	return groups
}

type treePrinter struct {
	out      io.Writer
	lock     *wpmlock.Lockfile
	maxDepth int
	colorize bool
}

// printLevel renders one level of the tree.
func (p *treePrinter) printLevel(deps map[string]string, prefix []byte, depth int, visited map[string]bool) {
	if p.maxDepth >= 0 && depth > p.maxDepth {
		return
	}

	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, name := range keys {
		isLast := i == len(keys)-1
		connector := connectorMid
		if isLast {
			connector = connectorEnd
		}

		isCycle := visited[name]
		info, subDeps, isMissing := p.formatNode(name, deps[name], isCycle)
		_, _ = fmt.Fprintf(p.out, "%s%s%s\n", prefix, connector, info)

		if isMissing || isCycle || len(subDeps) == 0 {
			continue
		}

		indent := indentMid
		if isLast {
			indent = indentEnd
		}

		mark := len(prefix)
		prefix = append(prefix, indent...)
		visited[name] = true
		p.printLevel(subDeps, prefix, depth+1, visited)
		delete(visited, name)
		prefix = prefix[:mark]
	}
}

// formatNode renders a single node and returns its sub-dependencies and whether
// the package is missing from the lockfile.
func (p *treePrinter) formatNode(name, requestedVersion string, isCycle bool) (info string, subDeps map[string]string, isMissing bool) {
	pkg, ok := p.lock.Packages[name]
	if !ok {
		unmet := "UNMET DEPENDENCY"
		if p.colorize {
			unmet = aec.RedF.Apply(unmet)
		}
		return name + "@" + requestedVersion + " " + unmet, nil, true
	}

	if p.colorize {
		info = name + aec.LightBlackF.Apply("@"+pkg.Version)
	} else {
		info = name + "@" + pkg.Version
	}

	if pkg.Version != requestedVersion {
		invalid := "(invalid: \"" + requestedVersion + "\")"
		if p.colorize {
			invalid = aec.RedF.Apply(invalid)
		}
		info += " " + invalid
	}

	if isCycle {
		cycle := "(cycle)"
		if p.colorize {
			cycle = aec.MagentaF.Apply(cycle)
		}
		info += " " + cycle
	}

	if pkg.Dependencies != nil {
		subDeps = *pkg.Dependencies
	}
	return info, subDeps, false
}
