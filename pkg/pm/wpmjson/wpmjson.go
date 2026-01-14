package wpmjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"wpm/pkg/pm"
	"wpm/pkg/pm/wpmjson/types"
	"wpm/pkg/pm/wpmjson/validator"

	"github.com/pkg/errors"
)

const ConfigFile = "wpm.json"

// Config struct to define the wpm.json schema
type Config struct {
	Name            string               `json:"name,omitempty"`
	Version         string               `json:"version,omitempty"`
	Type            types.PackageType    `json:"type,omitempty"`
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
	Indentation   string              `json:"-"`
	packageConfig types.PackageConfig `json:"-"`
}

// New returns a new instance of wpm.json config
func New() *Config {
	return &Config{
		packageConfig: types.PackageConfig{
			BinDir:        "wp-bin",
			ContentDir:    "wp-content",
			RuntimeStrict: true, // Runtime strict mode enabled by default
		},
	}
}

// BinDir returns the bin directory from the config or the default if not set
func (c *Config) BinDir() string {
	if c.Config != nil && c.Config.BinDir != "" {
		return c.Config.BinDir
	}
	return c.packageConfig.BinDir
}

// ContentDir returns the content directory from the config or the default if not set
func (c *Config) ContentDir() string {
	if c.Config != nil && c.Config.ContentDir != "" {
		return c.Config.ContentDir
	}
	return c.packageConfig.ContentDir
}

// RuntimeStrict returns the runtime strict mode from the config or the default if not set
func (c *Config) RuntimeStrict() bool {
	if c.Config != nil {
		return c.Config.RuntimeStrict
	}
	return c.packageConfig.RuntimeStrict
}

// Validate checks the configuration struct for logical and schema errors.
func (c *Config) Validate() error {
	var errs validator.ErrorList

	// Required fields
	errs.Add("name", validator.IsValidPackageName(c.Name))
	errs.Add("version", validator.IsValidVersion(c.Version))
	errs.Add("type", validator.IsValidPackageType(c.Type))

	// Metadata fields
	if c.Description != "" {
		errs.Add("description", validator.IsValidDescription(c.Description))
	}
	if c.License != "" {
		errs.Add("license", validator.IsValidLicense(c.License))
	}
	if c.Homepage != "" {
		errs.Add("homepage", validator.IsValidHomepage(c.Homepage))
	}
	if len(c.Tags) > 0 {
		errs.MustMerge(validator.ValidateTags(c.Tags))
	}
	if len(c.Team) > 0 {
		errs.MustMerge(validator.ValidateTeam(c.Team))
	}

	// Core fields
	if c.Requires != nil {
		errs.MustMerge(validator.ValidateRequires(c.Requires.WP, c.Requires.PHP))
	}
	if c.Dependencies != nil {
		errs.MustMerge(validator.ValidateDependencies(*c.Dependencies, "dependencies"))
	}
	if c.DevDependencies != nil {
		errs.MustMerge(validator.ValidateDependencies(*c.DevDependencies, "devDependencies"))
	}

	return errs.Err()
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
