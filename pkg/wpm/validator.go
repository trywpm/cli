package wpm

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// NewValidator creates a new validator instance.
func NewValidator() (*validator.Validate, error) {
	validator := validator.New()
	err := validator.RegisterValidation("package_name_regex", packageNameRegex)
	if err != nil {
		return nil, err
	}

	return validator, nil
}

// packageNameRegex validates the package name field with a regex.
// Only lowercase letters, numbers, and hyphens are allowed.
//
// FIXME: Fix the regex.
func packageNameRegex(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}

	return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(value)
}
