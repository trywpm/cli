package validator

import (
	"regexp"

	goValidator "github.com/go-playground/validator/v10"
)

// Platform struct to define the platform field
type PackagePlatform struct {
	WP  string `json:"wp" validate:"required"`
	PHP string `json:"php" validate:"required"`
}

// Package struct to define the wpm.json schema
type Package struct {
	Name            string            `json:"name,omitempty" validate:"required,min=3,max=164"`
	Description     string            `json:"description,omitempty"`
	Private         bool              `json:"private,omitempty"`
	Type            string            `json:"type,omitempty" validate:"required,oneof=plugin theme mu-plugin drop-in"`
	Version         string            `json:"version,omitempty" validate:"required,semver,max=64"`
	License         string            `json:"license,omitempty"`
	Homepage        string            `json:"homepage,omitempty" validate:"url"`
	Tags            []string          `json:"tags,omitempty" validate:"dive,max=5"`
	Team            []string          `json:"team,omitempty"`
	Bin             map[string]string `json:"bin,omitempty"`
	Platform        PackagePlatform   `json:"platform,omitempty" validate:"required"`
	Config          map[string]string `json:"config,omitempty"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
}

// Description of package fields.
var PackageFieldDescriptions = map[string]string{
	"Name":            "must contain only lowercase letters, numbers, and hyphens, and be between 3 and 164 characters. (required)",
	"Description":     "should be a string. (optional)",
	"Type":            "must be one of: 'plugin', 'theme', 'mu-plugin', or 'drop-in'. (required)",
	"Version":         "must be a valid semantic version (semver) and less than 64 characters. (required)",
	"License":         "must be a string. (optional)",
	"Homepage":        "must be a valid url. (optional)",
	"Tags":            "must be an array of strings with a maximum of 5 tags. (optional)",
	"Team":            "must be an array of strings. (optional)",
	"Bin":             "must be an object with string values. (optional)",
	"Platform":        "must contain wp and php versions. (required)",
	"Dependencies":    "must be an object with string values. (optional)",
	"DevDependencies": "must be an object with string values. (optional)",
	"Scripts":         "must be an object with string values. (optional)",
}

// Dist struct to define the dist field
type PackageDist struct {
	Size      int    `json:"size" validate:"gte=0"`
	FileCount int    `json:"fileCount" validate:"gte=0"`
	Digest    string `json:"digest" validate:"required,sha256"`
}

// NewValidator creates a new validator instance.
func NewValidator() (*goValidator.Validate, error) {
	validator := goValidator.New()
	err := validator.RegisterValidation("package_name_regex", packageNameRegex)
	if err != nil {
		return nil, err
	}

	return validator, nil
}

// ValidatePackage validates the package struct.
func ValidatePackage(pkg Package, v *goValidator.Validate) error {
	errs := v.Struct(pkg)
	if errs != nil {
		return HandleValidatorError(errs)
	}

	return nil
}

// packageNameRegex validates the package name field with a regex.
// Only lowercase letters, numbers, and hyphens are allowed.
func packageNameRegex(fl goValidator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(value)
}
