package installer

import (
	"os"
	"path/filepath"
	"wpm/pkg/pm/resolution"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmjson/types"
	"wpm/pkg/pm/wpmlock"
)

type ActionType int

const (
	ActionInstall ActionType = iota
	ActionUpdate
	ActionRemove
)

// Action represents a single operation to be performed on the filesystem
type Action struct {
	Type     ActionType
	Name     string
	Version  string
	Resolved string // Tarball URL
	Digest   string // Sha256 digest
	PkgType  types.PackageType
}

// CalculatePlan determines filesystem operations based on lockfile, resolved tree, and flags.
func CalculatePlan(
	lock *wpmlock.Lockfile,
	resolved map[string]resolution.Node,
	contentDir string,
	wpmCfg *wpmjson.Config,
	noDev bool,
) []Action {
	var actions []Action
	seen := make(map[string]bool)

	var prodSet map[string]bool
	if noDev {
		prodSet = getProdDependencies(wpmCfg, resolved)
	}

	for name, node := range resolved {
		seen[name] = true

		shouldInstall := true
		if noDev && !prodSet[name] {
			shouldInstall = false
		}

		// Calculate target path to check filesystem state
		subDir := "plugins"
		switch node.Type {
		case types.TypeTheme:
			subDir = "themes"
		case types.TypeMuPlugin:
			subDir = "mu-plugins"
		}
		targetPath := filepath.Join(contentDir, subDir, name)

		exists := false
		if _, err := os.Stat(targetPath); err == nil {
			exists = true
		}

		// If we shouldn't install it (dev dep in --no-dev mode)
		if !shouldInstall {
			if exists {
				// It exists on disk but shouldn't be there -> Remove
				actions = append(actions, Action{
					Type:    ActionRemove,
					Name:    name,
					PkgType: node.Type,
				})
			}
			continue
		}

		// Check if package exists in lockfile
		if oldPkg, ok := lock.Packages[name]; ok {
			// Update if version or digest has changed
			if oldPkg.Version != node.Version || oldPkg.Digest != node.Digest {
				actions = append(actions, Action{
					Type:     ActionUpdate,
					Name:     name,
					Version:  node.Version,
					Resolved: node.Resolved,
					Digest:   node.Digest,
					PkgType:  node.Type,
				})
				continue
			}

			if !exists {
				actions = append(actions, Action{
					Type:     ActionInstall,
					Name:     name,
					Version:  node.Version,
					Resolved: node.Resolved,
					Digest:   node.Digest,
					PkgType:  node.Type,
				})
			}

			// If it exists and matches lockfile, do nothing (NoOp)

		} else {
			// New package -> Install
			actions = append(actions, Action{
				Type:     ActionInstall,
				Name:     name,
				Version:  node.Version,
				Resolved: node.Resolved,
				Digest:   node.Digest,
				PkgType:  node.Type,
			})
		}
	}

	// Any package in lockfile that is not in resolved map
	for name, lockPkg := range lock.Packages {
		if !seen[name] {
			actions = append(actions, Action{
				Type:    ActionRemove,
				Name:    name,
				Version: lockPkg.Version,
				PkgType: lockPkg.Type,
			})
		}
	}

	return actions
}

// getProdDependencies returns a set of all production dependencies and their transitive dependencies.
func getProdDependencies(root *wpmjson.Config, resolved map[string]resolution.Node) map[string]bool {
	prodSet := make(map[string]bool)
	queue := make([]string, 0)

	if root.Dependencies != nil {
		for name := range *root.Dependencies {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if prodSet[name] {
			continue
		}
		prodSet[name] = true

		// Add children to queue
		if node, ok := resolved[name]; ok {
			if node.Dependencies == nil {
				continue
			}

			for depName := range *node.Dependencies {
				queue = append(queue, depName)
			}
		}
	}

	return prodSet
}
