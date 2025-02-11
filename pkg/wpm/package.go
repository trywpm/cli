package wpm

// Dist struct to define the distribution metadata
type Dist struct {
	Digest       string `json:"digest"`
	TotalFiles   int    `json:"totalFiles"`
	PackedSize   int    `json:"packedSize"`
	UnpackedSize int    `json:"unpackedSize"`
}

// Config struct to define the wpm.json schema
type Config struct {
	Name            string            `json:"name" validate:"required,min=3,max=164,package_name_regex"`
	Description     string            `json:"description,omitempty"`
	Private         bool              `json:"private,omitempty" validate:"boolean,omitempty"`
	Type            string            `json:"type" validate:"required,oneof=plugin theme"`
	Version         string            `json:"version" validate:"required,package_semver,max=64"`
	License         string            `json:"license" validate:"omitempty"`
	Homepage        string            `json:"homepage,omitempty" validate:"omitempty,url"`
	Tags            []string          `json:"tags,omitempty" validate:"dive,max=5"`
	Team            []string          `json:"team,omitempty"`
	Bin             map[string]string `json:"bin,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty" validate:"omitempty,package_dependencies"`
	DevDependencies map[string]string `json:"devDependencies,omitempty" validate:"omitempty,package_dependencies"`
	Scripts         map[string]string `json:"scripts,omitempty"`
}

// Meta struct to define the package metadata
type Meta struct {
	Dist       Dist   `json:"dist"`
	Wpm        string `json:"_wpm"`
	Access     string `json:"access"`
	Attachment string `json:"attachment"`
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
	"Homepage":        "must be a valid url. (optional)",
	"Tags":            "must be an array of strings with a maximum of 5 tags. (optional)",
	"Team":            "must be an array of strings. (optional)",
	"Bin":             "must be an object with string values. (optional)",
	"Dependencies":    "must be an object with string values. (optional)",
	"DevDependencies": "must be an object with string values. (optional)",
	"Scripts":         "must be an object with string values. (optional)",
}
