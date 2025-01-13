package wpm

import (
	"fmt"
	"strings"

	goValidator "github.com/go-playground/validator/v10"
)

// validatorError represents an error response from the wpm schema validator.
type validatorError struct {
	Errors []validatorErrorItem
}

// validatorErrorItem represents an error item from the wpm schema validator.
type validatorErrorItem struct {
	Message     string
	FailedField string
}

// Allow ValidatorError to satisfy error interface.
func (err *validatorError) Error() string {
	// Add all error messages to a string.
	message := fmt.Sprintf("%s\n", "config validation failed")

	for _, e := range err.Errors {
		if e.FailedField == "DevDependencies" {
			e.FailedField = "dev_dependencies"
		}

		message += fmt.Sprintf("  - \"%s\" %s", strings.ToLower(e.FailedField), e.Message)
		if e != err.Errors[len(err.Errors)-1] {
			message += "\n"
		}
	}

	return message
}

// HandleValidatorError parses validation error into a ValidatorError.
func handleValidatorError(errs error) error {
	validationErrors := &validatorError{}

	for _, err := range errs.(goValidator.ValidationErrors) {
		ve := &validatorErrorItem{}

		ve.FailedField = err.Field()
		ve.Message = PackageFieldDescriptions[err.Field()]

		validationErrors.Errors = append(validationErrors.Errors, *ve)
	}

	return validationErrors
}
