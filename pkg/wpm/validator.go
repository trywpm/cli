package wpm

import (
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/go-playground/validator/v10"
)

// NewValidator creates a new validator instance.
func NewValidator() (*validator.Validate, error) {
	validator := validator.New()

	// Package name regex validation.
	err := validator.RegisterValidation("package_name_regex", packageNameRegex)
	if err != nil {
		return nil, err
	}

	// Semver validation.
	err = validator.RegisterValidation("package_semver", validateSemver)
	if err != nil {
		return nil, err
	}

	// Semver constraint validation.
	err = validator.RegisterValidation("package_semver_constraint", validateSemverConstraint)
	if err != nil {
		return nil, err
	}

	// Constraint validation.
	err = validator.RegisterValidation("package_constraint", validateConstraint)
	if err != nil {
		return nil, err
	}

	// Dependencies validation.
	err = validator.RegisterValidation("package_dependencies", validateDependencies)
	if err != nil {
		return nil, err
	}

	return validator, nil
}

var regex *regexp.Regexp

func init() {
	regex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
}

// nameRegex validates the name field with a regex.
func nameRegex(n string) bool {
	return regex.MatchString(n)
}

// packageNameRegex validates the package name field with a regex.
func packageNameRegex(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	return nameRegex(value)
}

// validateSemver validates the semver field.
func validateSemver(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	_, err := semver.StrictNewVersion(value)

	return err == nil
}

// validateSemverConstraint validates the semver constraint field.
func validateSemverConstraint(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	_, err := semver.NewConstraint(value)

	return err == nil
}

// validateConstraint validates the version constraint field.
func validateConstraint(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	_, err := semver.NewConstraint(value)

	return err == nil
}

// validateDependencies validates the dependencies field.
func validateDependencies(fl validator.FieldLevel) bool {
	value := fl.Field().Interface().(map[string]string)
	if value == nil {
		return false
	}

	for k, v := range value {
		if k == "" || v == "" {
			return false
		}

		if !nameRegex(k) {
			return false
		}

		// if version is wildcard, it is valid
		if v == "*" {
			continue
		}

		_, err := semver.NewVersion(v)
		if err != nil {
			return false
		}
	}

	return true
}
