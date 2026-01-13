package wpmjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"wpm/pkg/pm"
	"wpm/pkg/pm/wpmjson/types"

	"github.com/pkg/errors"
)

const ConfigFile = "wpm.json"

// Config struct to define the wpm.json schema
type Config struct {
	Name            string               `json:"name"`
	Version         string               `json:"version"`
	Type            types.PackageType    `json:"type"`
	Description     string               `json:"description,omitempty"`
	Private         bool                 `json:"private,omitempty"`
	Bin             *types.Bin           `json:"bin,omitempty"`
	Requires        *types.Requires      `json:"requires,omitempty"`
	License         string               `json:"license,omitempty"`
	Homepage        string               `json:"homepage,omitempty"`
	Tags            []string             `json:"tags,omitempty"`
	Team            []string             `json:"team,omitempty"`
	Dependencies    *types.Dependencies  `json:"dependencies,omitempty"`
	DevDependencies *types.Dependencies  `json:"devDependencies,omitempty"`
	Config          *types.PackageConfig `json:"config,omitempty"`
	Scripts         *types.Scripts       `json:"scripts,omitempty"`

	// Internal fields.
	Indentation string `json:"-"`
}

var defaultRuntimeStrict = true

// New returns a new instance of wpm.json config
func New() *Config {
	return &Config{
		Bin:             &types.Bin{},
		Requires:        &types.Requires{},
		Dependencies:    &types.Dependencies{},
		DevDependencies: &types.Dependencies{},
		Config: &types.PackageConfig{
			RuntimeStrict: &defaultRuntimeStrict,
			BinDir:        "wp-bin",
			ContentDir:    "wp-content",
		},
		Scripts: &types.Scripts{},
	}
}

// GetIndentation returns the indentation used in the wpm.json file
func (c *Config) GetIndentation() string {
	if c.Indentation != "" {
		return c.Indentation
	}

	return "  " // Default to 2 spaces if not set
}

// Read loads the wpm.json file from the specified directory.
func Read(cwd string) (*Config, error) {
	path := filepath.Join(cwd, ConfigFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read wpm.json")
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse wpm.json")
	}

	config.Indentation = pm.DetectIndentation(data)

	return &config, nil
}

// Write saves the wpm.json to disk in the specified directory.
func (c *Config) Write(cwd string) error {
	path := filepath.Join(cwd, ConfigFile)

	data, err := json.MarshalIndent(c, "", c.GetIndentation())
	if err != nil {
		return errors.Wrap(err, "failed to marshal wpm.json")
	}

	// Write with 0644 permissions (rw-r--r--)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write wpm.json to disk")
	}

	return nil
}
