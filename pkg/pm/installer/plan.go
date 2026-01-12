package installer

import (
	"os"
	"wpm/pkg/pm/resolution"
	"wpm/pkg/pm/wpmjson"
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
	PkgType  wpmjson.PackageType
}

// CalculatePlan diffs the old lockfile against the new resolved tree and the filesystem state.
func CalculatePlan(lock *wpmlock.Lockfile, resolved map[string]resolution.Node, contentDir string) []Action {
	var actions []Action
	seen := make(map[string]bool)

	for name, node := range resolved {
		seen[name] = true

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

			// Check fs if package is installed.
			//
			// Could be possible that the package exists in lockfile but is missing on disk.
			subDir := "plugins"
			switch node.Type {
			case wpmjson.TypeTheme:
				subDir = "themes"
			case wpmjson.TypeMuPlugin:
				subDir = "mu-plugins"
			}

			targetPath := contentDir + "/" + subDir + "/" + name

			// Install if not present on the filesystem
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
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
