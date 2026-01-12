package wpmlock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"wpm/pkg/pm/wpmjson"

	"github.com/pkg/errors"
)

const (
	LockfileName   = "wpm.lock"
	CurrentVersion = 1
)

// LockPackage represents a specific version of a package locked in the lockfile.
type LockPackage struct {
	Version      string                `json:"version"`
	Resolved     string                `json:"resolved"`
	Digest       string                `json:"digest"`
	Type         wpmjson.PackageType   `json:"type"`
	Bin          *wpmjson.Bin          `json:"bin,omitempty"`
	Dependencies *wpmjson.Dependencies `json:"dependencies,omitempty"`
}

// Lockfile represents the state of the dependency tree.
// Since wpm does not support nesting, this is a flat map of package names to their locked details.
type Lockfile struct {
	LockfileVersion int                    `json:"lockfileVersion"`
	Packages        map[string]LockPackage `json:"packages"`
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

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read lockfile")
	}

	var lockfile Lockfile
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, errors.Wrap(err, "failed to parse lockfile")
	}

	if lockfile.LockfileVersion > CurrentVersion {
		return nil, errors.New("wpm upgrade required: lockfile version is newer than this version of wpm")
	}

	if lockfile.Packages == nil {
		lockfile.Packages = make(map[string]LockPackage)
	}

	return &lockfile, nil
}

// Write saves the Lockfile to disk in the specified directory.
func (l *Lockfile) Write(cwd string) error {
	l.LockfileVersion = CurrentVersion

	path := filepath.Join(cwd, LockfileName)

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal lockfile")
	}

	// Write with 0644 permissions (rw-r--r--)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write lockfile to disk")
	}

	return nil
}
