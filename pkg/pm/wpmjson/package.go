package wpmjson

type (
	PackageType       string
	PackageVisibility string
)

const (
	TypeTheme    PackageType = "theme"
	TypePlugin   PackageType = "plugin"
	TypeMuPlugin PackageType = "mu-plugin"

	VisibilityPublic  PackageVisibility = "public"
	VisibilityPrivate PackageVisibility = "private"
)

// Dist struct to define the distribution metadata
type Dist struct {
	Digest       string `json:"digest"`
	TotalFiles   int    `json:"totalFiles"`
	PackedSize   int64  `json:"packedSize"`
	UnpackedSize int64  `json:"unpackedSize"`
}

// PackageConfig struct to define the package configuration
type PackageConfig struct {
	BinDir         *string `json:"bin-dir,omitempty"`
	ContentDir     *string `json:"content-dir,omitempty"`
	PlatformStrict *bool   `json:"platform-strict,omitempty"` // If set to true, wpm will refuse to install the package if the platform requirements are not met.
}

// Platform holds the php and wp version constraints for a package.
type Platform struct {
	WP  string `json:"wp,omitempty" validate:"omitempty,wpm_semver_constraint"`
	PHP string `json:"php,omitempty" validate:"omitempty,wpm_semver_constraint"`
}

type Dependencies map[string]string

// Config struct to define the wpm.json schema
type Config struct {
	Name            string             `json:"name" validate:"required,wpm_name"`
	Description     string             `json:"description,omitempty" validate:"omitempty,min=3,max=512"`
	Private         bool               `json:"private,omitempty"`
	Type            string             `json:"type" validate:"required,oneof=theme plugin mu-plugin"`
	Version         string             `json:"version" validate:"required,wpm_semver"`
	Platform        *Platform          `json:"platform,omitempty" validate:"omitempty"`
	License         string             `json:"license,omitempty" validate:"omitempty,min=3,max=100"`
	Homepage        string             `json:"homepage,omitempty" validate:"omitempty,url,wpm_http_url,min=10,max=200"`
	Tags            []string           `json:"tags,omitempty" validate:"omitempty,max=5,dive,min=3,max=64"`
	Team            []string           `json:"team,omitempty" validate:"omitempty,max=10,dive,min=3,max=100"`
	Bin             *map[string]string `json:"bin,omitempty"`
	Dependencies    *Dependencies      `json:"dependencies,omitempty" validate:"omitempty,max=16,dive,keys,wpm_name,endkeys,wpm_dependency_version"`
	DevDependencies *Dependencies      `json:"devDependencies,omitempty" validate:"omitempty,max=16,dive,keys,wpm_name,endkeys,wpm_dependency_version"`
	Scripts         *map[string]string `json:"scripts,omitempty"`
	Config          *PackageConfig     `json:"config,omitempty"`
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
	"Name":            "must contain only lowercase letters and hyphens with a length between 3 and 164 characters. (required)",
	"Description":     "should be a string. (optional)",
	"Private":         "must be a boolean. (optional)",
	"Type":            "must be one of: 'plugin', 'theme', or 'mu-plugin'. (required)",
	"Version":         "must be a valid semantic version (semver) and less than 64 characters. (required)",
	"Platform":        "must be an object with 'php' and 'wp' fields, both of which must be valid semantic version constraints. (optional)",
	"License":         "must be a string. (optional)",
	"Homepage":        "must be a valid http url. (optional)",
	"Tags":            "must be an array of strings with a maximum of 5 tags. (optional)",
	"Team":            "must be an array of strings with a maximum of 10 team members. (optional)",
	"Bin":             "must be an object with string values. (optional)",
	"Dependencies":    "must be an object with string values. (optional)",
	"DevDependencies": "must be an object with string values. (optional)",
	"Scripts":         "must be an object with string values. (optional)",
}
