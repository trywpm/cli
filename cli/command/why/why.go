package why

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/pkg/output"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmlock"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewWhyCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why [PACKAGE]",
		Short: "Show why a package is installed",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhy(wpmCli, args[0])
		},
	}
	return cmd
}

func runWhy(wpmCli command.Cli, targetPkg string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	config, err := wpmjson.Read(cwd)
	if err != nil {
		return err
	}

	if config == nil {
		return errors.New("no wpm.json found in the current directory")
	}

	lock, err := wpmlock.Read(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to read lockfile")
	}
	if lock == nil {
		return errors.New("no wpm.lock found. Run 'wpm install' first to generate a lockfile.")
	}

	_, exists := lock.Packages[targetPkg]
	if !exists {
		return fmt.Errorf("package '%s' is not found in wpm.lock", targetPkg)
	}

	rootNode := config.Name
	if rootNode == "" {
		rootNode = filepath.Base(cwd)
	}

	colorize := wpmCli.Out().IsColorEnabled()

	rootNodeIdDeps := rootNode + " (dependencies)"
	if colorize {
		rootNodeIdDeps = fmt.Sprintf("%s %s", aec.Bold.Apply(rootNode), aec.Faint.Apply("(dependencies)"))
	}

	rootNodeIdDevDeps := rootNode + " (devDependencies)"
	if colorize {
		rootNodeIdDevDeps = fmt.Sprintf("%s %s", aec.Bold.Apply(rootNode), aec.Faint.Apply("(devDependencies)"))
	}

	dependents := make(map[string][]string)

	if config.Dependencies != nil {
		for name := range *config.Dependencies {
			dependents[name] = append(dependents[name], rootNodeIdDeps)
		}
	}
	if config.DevDependencies != nil {
		for name := range *config.DevDependencies {
			dependents[name] = append(dependents[name], rootNodeIdDevDeps)
		}
	}

	for parentName, parentPkg := range lock.Packages {
		if parentPkg.Dependencies == nil {
			continue
		}

		for depName := range *parentPkg.Dependencies {
			dependents[depName] = append(dependents[depName], parentName)
		}
	}

	paths := findPathsToRoot(targetPkg, dependents)

	if len(paths) == 0 {
		wpmCli.Output().Prettyln(output.Text{
			Plain: fmt.Sprintf("%s is present in lockfile but has no apparent dependents (orphaned?).", targetPkg),
			Fancy: fmt.Sprintf("%s is present in lockfile but has no apparent dependents (orphaned?).", aec.Bold.Apply(targetPkg)),
		})
		return nil
	}

	for _, path := range paths {
		indent := ""
		for i := len(path) - 1; i >= 0; i-- {
			name := path[i]

			info := ""
			if !stringsContainsRoot(name) {
				if pkg, ok := lock.Packages[name]; ok {
					info = fmt.Sprintf("@%s", pkg.Version)
				}
			}

			// If it's the last item, don't print the branch line
			if i == len(path)-1 {
				wpmCli.Out().WriteString(fmt.Sprintf("%s%s\n", indent, name))
			} else {
				wpmCli.Out().WriteString(fmt.Sprintf("%s└─ %s%s\n", indent, name, info))
			}

			// Increase indent for the next level
			if i < len(path)-1 {
				indent += "   "
			}
		}

		wpmCli.Out().WriteString("\n")
	}

	return nil
}

// findPathsToRoot performs a BFS traversal backwards to find chains to the root
func findPathsToRoot(start string, dependents map[string][]string) [][]string {
	var results [][]string

	// Queue holds paths: ["target", "parent", "grandparent", "root"]
	queue := [][]string{{start}}

	seen := make(map[string]bool)

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]

		// Check if we hit the root
		if stringsContainsRoot(current) {
			results = append(results, path)
			continue
		}

		seen[current] = true

		parents, hasParents := dependents[current]
		if !hasParents {
			// Dead end (orphaned package)
			continue
		}

		sort.Strings(parents)

		for _, parent := range parents {
			// Create new path: target -> ... -> current -> parent
			newPath := make([]string, len(path))
			copy(newPath, path)
			newPath = append(newPath, parent)
			queue = append(queue, newPath)
		}
	}

	return results
}

func stringsContainsRoot(s string) bool {
	return strings.Contains(s, "(dependencies)") || strings.Contains(s, "(devDependencies)")
}
