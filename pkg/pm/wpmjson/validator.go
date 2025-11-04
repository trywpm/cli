package wpmjson

import (
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-playground/validator/v10"
)

var packageAndDistNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// IsValidSemver checks if the provided version string is a valid semantic version.
func IsValidSemver(version string) bool {
	if len(version) < 5 || len(version) > 64 {
		return false
	}

	if _, err := semver.StrictNewVersion(version); err != nil {
		return false
	}

	return true
}

// NewValidator creates a new validator instance.
func NewValidator() (*validator.Validate, error) {
	v := validator.New(validator.WithRequiredStructEnabled())

	err := v.RegisterValidation("wpm_name", validatePackageName)
	if err != nil {
		return nil, err
	}

	err = v.RegisterValidation("wpm_semver", validateSemver)
	if err != nil {
		return nil, err
	}

	err = v.RegisterValidation("wpm_semver_constraint", validateSemverConstraint)
	if err != nil {
		return nil, err
	}

	err = v.RegisterValidation("wpm_dependency_version", validateDependencyVersion)
	if err != nil {
		return nil, err
	}

	err = v.RegisterValidation("wpm_http_url", validateHttpURL)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// validatePackageName checks if the package name is valid.
func validatePackageName(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	if len(v) < 3 || len(v) > 164 {
		return false
	}
	return packageAndDistNameRegex.MatchString(v)
}

// validateSemver checks if the version string is a valid semantic version.
func validateSemver(fl validator.FieldLevel) bool {
	return IsValidSemver(fl.Field().String())
}

// validateSemverConstraint checks if the version constraint is valid.
func validateSemverConstraint(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	if v == "" {
		return false
	}

	if strings.HasPrefix(v, "v") {
		return false
	}

	if _, err := semver.NewConstraint(v); err != nil {
		return false
	}

	return true
}

// validateDependencyVersion checks if the dependency version is valid.
func validateDependencyVersion(fl validator.FieldLevel) bool {
	v := fl.Field().String()

	if v == "*" {
		return true
	}

	return IsValidSemver(v)
}

// validateHttpURL checks if a URL starts with http:// or https://.
func validateHttpURL(fl validator.FieldLevel) bool {
	v := fl.Field().String()
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://")
}
