package wpmlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.wpm.so/cli/pkg/atomicwriter"
	"go.wpm.so/cli/pkg/pm"
	"go.wpm.so/cli/pkg/pm/wpmjson/manifest"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
)

const (
	LockfileName   = "wpm.lock"
	CurrentVersion = 1
)

// LockPackage represents a specific version of a package locked in the lockfile.
type LockPackage struct {
	Version      string               `json:"version"`
	Signatures   []manifest.Signature `json:"signatures"`
	Digest       string               `json:"digest"`
	Type         types.PackageType    `json:"type"`
	Bin          *types.Bin           `json:"bin,omitempty"`
	Dependencies *types.Dependencies  `json:"dependencies,omitempty"`
}

// Lockfile represents the state of the dependency tree.
// Since wpm does not support nesting, this is a flat map of package names to their locked details.
type Lockfile struct {
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]LockPackage `json:"packages"`
	Indentation     string                 `json:"-"`
}

// New creates a new empty Lockfile instance with the current version.
func New() *Lockfile {
	return &Lockfile{
		LockfileVersion: CurrentVersion,
		Packages:        make(map[string]LockPackage),
	}
}

// Read loads the wpm.lock file from the specified directory.
func Read(cwd string) (*Lockfile, error) {
	path := filepath.Join(cwd, LockfileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is cwd + LockfileName constant
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	var lockfile Lockfile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	if lockfile.LockfileVersion > CurrentVersion {
		return nil, errors.New("wpm upgrade required: lockfile version is newer than this version of wpm")
	}

	if lockfile.Packages == nil {
		lockfile.Packages = make(map[string]LockPackage)
	}

	for name := range lockfile.Packages {
		if err := validator.IsValidPackageName(name); err != nil {
			return nil, fmt.Errorf("invalid package name %q in lockfile: %w", name, err)
		}
	}

	lockfile.Indentation = pm.DetectIndentation(data)

	return &lockfile, nil
}

// SetIndentation sets the indentation style for the lockfile when written to disk.
func (l *Lockfile) SetIndentation(indentation string) {
	l.Indentation = indentation
}

// Write saves the Lockfile to disk in the specified directory.
func (l *Lockfile) Write(cwd string) error {
	l.LockfileVersion = CurrentVersion

	path := filepath.Join(cwd, LockfileName)

	data, err := json.MarshalIndent(l, "", l.Indentation)
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}

	// Write with 0644 permissions (rw-r--r--)
	if err := atomicwriter.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write lockfile to disk: %w", err)
	}

	return nil
}
