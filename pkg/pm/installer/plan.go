package installer

import (
	"os"
	"path/filepath"

	"go.wpm.so/cli/pkg/pm/resolution"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmlock"
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

		subDir, ok := subDirForType(node.Type)
		if !ok {
			continue
		}
		exists := pathExists(filepath.Join(contentDir, subDir, name))

		if noDev && !prodSet[name] {
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

		if action, hasAction := resolveAction(name, node, lock, exists); hasAction {
			actions = append(actions, action)
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

// subDirForType maps a package type to its content sub-directory. Returns false for unknown types.
func subDirForType(t types.PackageType) (string, bool) {
	switch t {
	case types.TypeTheme:
		return "themes", true
	case types.TypePlugin:
		return "plugins", true
	default:
		return "", false
	}
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// resolveAction picks the Action (if any) for a resolved package by comparing it
// against lockfile state and on-disk presence. Returns (zero, false) for NoOp.
func resolveAction(name string, node resolution.Node, lock *wpmlock.Lockfile, exists bool) (Action, bool) {
	oldPkg, inLock := lock.Packages[name]
	if !inLock {
		return Action{
			Type:     ActionInstall,
			Name:     name,
			Version:  node.Version,
			Resolved: node.Resolved,
			Digest:   node.Digest,
			PkgType:  node.Type,
		}, true
	}

	if oldPkg.Version != node.Version || oldPkg.Digest != node.Digest {
		return Action{
			Type:     ActionUpdate,
			Name:     name,
			Version:  node.Version,
			Resolved: node.Resolved,
			Digest:   node.Digest,
			PkgType:  node.Type,
		}, true
	}

	if !exists {
		return Action{
			Type:     ActionInstall,
			Name:     name,
			Version:  node.Version,
			Resolved: node.Resolved,
			Digest:   node.Digest,
			PkgType:  node.Type,
		}, true
	}

	// Exists on disk and matches lockfile -> NoOp
	return Action{}, false
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
