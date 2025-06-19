package wpm

// Dist struct to define the distribution metadata
type Dist struct {
	Digest       string `json:"digest"`
	TotalFiles   int    `json:"totalFiles"`
	PackedSize   int    `json:"packedSize"`
	UnpackedSize int    `json:"unpackedSize"`
}

// PackageConfig struct to define the package configuration
type PackageConfig struct {
	BinDir         string `json:"bin-dir,omitempty"`
	ContentDir     string `json:"content-dir,omitempty"`
	PlatformStrict bool   `json:"platform-strict,omitempty"` // If set to true, wpm will refuse to install the package if the platform requirements are not met.
}

// Platform holds the php and wp version constraints for a package.
type Platform struct {
	PHP string `json:"php,omitempty" validate:"package_semver_constraint"`
	WP  string `json:"wp,omitempty" validate:"package_semver_constraint"`
}

// Config struct to define the wpm.json schema
type Config struct {
	Name            string            `json:"name" validate:"required,min=3,max=164,package_name_regex"`
	Description     string            `json:"description,omitempty"`
	Private         bool              `json:"private,omitempty" validate:"boolean"`
	Type            string            `json:"type" validate:"required,oneof=plugin theme mu-plugin"`
	Version         string            `json:"version" validate:"required,package_semver,max=64"`
	Platform        Platform          `json:"platform,omitempty"`
	License         string            `json:"license"`
	Homepage        string            `json:"homepage,omitempty" validate:"http_url"`
	Tags            []string          `json:"tags,omitempty" validate:"max=5"`
	Team            []string          `json:"team,omitempty"`
	Bin             map[string]string `json:"bin,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty" validate:"package_dependencies"`
	DevDependencies map[string]string `json:"devDependencies,omitempty" validate:"package_dependencies"`
	Scripts         map[string]string `json:"scripts,omitempty"`
	Config          PackageConfig     `json:"config,omitempty"`
}

// Meta struct to define the package metadata
type Meta struct {
	Tag        string `json:"tag"`
	Dist       Dist   `json:"dist"`
	Wpm        string `json:"_wpm"`
	Visibility string `json:"visibility"`
	Readme     string `json:"readme,omitempty"`
}

// Package struct to define the package schema
type Package struct {
	Meta   Meta   `json:"meta"`
	Config Config `json:"config"`
}

// Description of package fields.
var PackageFieldDescriptions = map[string]string{
	"Name":            "must contain only lowercase letters, numbers, and hyphens, and be between 3 and 164 characters. (required)",
	"Description":     "should be a string. (optional)",
	"Private":         "must be a boolean. (optional)",
	"Type":            "must be one of: 'plugin', or 'theme'. (required)",
	"Version":         "must be a valid semantic version (semver) and less than 64 characters. (required)",
	"License":         "must be a string. (optional)",
	"Homepage":        "must be a valid http url. (optional)",
	"Tags":            "must be an array of strings with a maximum of 5 tags. (optional)",
	"Team":            "must be an array of strings. (optional)",
	"Bin":             "must be an object with string values. (optional)",
	"Dependencies":    "must be an object with string values. (optional)",
	"DevDependencies": "must be an object with string values. (optional)",
	"Scripts":         "must be an object with string values. (optional)",
}
