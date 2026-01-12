package wpmjson

type PackageType string

func (pt PackageType) String() string {
	return string(pt)
}

func (pt PackageType) Valid() bool {
	switch pt {
	case TypeTheme, TypePlugin, TypeMuPlugin:
		return true
	default:
		return false
	}
}

type PackageVisibility string

func (pv PackageVisibility) String() string {
	return string(pv)
}

func (pv PackageVisibility) Valid() bool {
	switch pv {
	case VisibilityPublic, VisibilityPrivate:
		return true
	default:
		return false
	}
}

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
	TotalFiles   int64  `json:"totalFiles"`
	PackedSize   int64  `json:"packedSize"`
	UnpackedSize int64  `json:"unpackedSize"`
}

// PackageConfig struct to define the package configuration
type PackageConfig struct {
	BinDir        string `json:"bin-dir,omitempty"`
	ContentDir    string `json:"content-dir,omitempty"`
	RuntimeStrict *bool  `json:"runtime-strict,omitempty"`
	RuntimeWp     string `json:"runtime-wp,omitempty"`
	RuntimePhp    string `json:"runtime-php,omitempty"`
}

// Requires holds wp and php version constraints for a package
type Requires struct {
	WP  string `json:"wp,omitempty" validate:"omitempty,wpm_semver_constraint"`
	PHP string `json:"php,omitempty" validate:"omitempty,wpm_semver_constraint"`
}

type Bin map[string]string
type Scripts map[string]string
type Dependencies map[string]string

// Config struct to define the wpm.json schema
type Config struct {
	Name            string         `json:"name" validate:"required,wpm_name"`
	Description     string         `json:"description,omitempty" validate:"omitempty,min=3,max=512"`
	Private         bool           `json:"private,omitempty"`
	Type            PackageType    `json:"type" validate:"required,oneof=theme plugin mu-plugin"`
	Version         string         `json:"version" validate:"required,wpm_semver"`
	Requires        *Requires      `json:"requires,omitempty" validate:"omitempty"`
	License         string         `json:"license,omitempty" validate:"omitempty,min=3,max=100"`
	Homepage        string         `json:"homepage,omitempty" validate:"omitempty,url,wpm_http_url,min=10,max=200"`
	Tags            []string       `json:"tags,omitempty" validate:"omitempty,max=5,dive,min=2,max=64"`
	Team            []string       `json:"team,omitempty" validate:"omitempty,max=100,dive,min=2,max=100"`
	Bin             *Bin           `json:"bin,omitempty"`
	Dependencies    *Dependencies  `json:"dependencies,omitempty" validate:"omitempty,max=16,dive,keys,wpm_name,endkeys,wpm_dependency_version"`
	DevDependencies *Dependencies  `json:"devDependencies,omitempty" validate:"omitempty,max=16,dive,keys,wpm_name,endkeys,wpm_dependency_version"`
	Scripts         *Scripts       `json:"scripts,omitempty"`
	Config          *PackageConfig `json:"config,omitempty"`
}

// Description of package fields.
var PackageFieldDescriptions = map[string]string{
	"Name":            "must contain only lowercase letters, numbers, and hyphens with a length between 3 and 164 characters. (required)",
	"Description":     "should be a string. (optional)",
	"Private":         "must be a boolean. (optional)",
	"Type":            "must be one of: 'plugin', 'theme', or 'mu-plugin'. (required)",
	"Version":         "must be a valid semantic version (semver) and less than 64 characters. (required)",
	"Requires":        "must be an object with 'php' and 'wp' fields, both of which must be valid semantic version constraints. (optional)",
	"License":         "must be a string. (optional)",
	"Homepage":        "must be a valid http url. (optional)",
	"Tags":            "must be an array of strings with a maximum of 5 tags. (optional)",
	"Team":            "must be an array of strings with a maximum of 100 team members. (optional)",
	"Bin":             "must be an object with string values. (optional)",
	"Dependencies":    "must be an object with string values. (optional)",
	"DevDependencies": "must be an object with string values. (optional)",
	"Scripts":         "must be an object with string values. (optional)",
}

// PackageManifest struct to define the package manifest in registry
//
// It will act as the source of truth for publishing and installing packages.
type PackageManifest struct {
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Type            PackageType       `json:"type"`
	Version         string            `json:"version"`
	Bin             *Bin              `json:"bin,omitempty"`
	Requires        *Requires         `json:"requires,omitempty"`
	License         string            `json:"license,omitempty"`
	Homepage        string            `json:"homepage,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Team            []string          `json:"team,omitempty"`
	Dependencies    *Dependencies     `json:"dependencies,omitempty"`
	DevDependencies *Dependencies     `json:"devDependencies,omitempty"`
	Tag             string            `json:"tag"`
	Dist            Dist              `json:"dist"`
	Wpm             string            `json:"_wpm"`
	Visibility      PackageVisibility `json:"visibility"`
	Readme          string            `json:"readme,omitempty"`
}
