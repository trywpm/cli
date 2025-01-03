package validator

import (
	goValidator "github.com/go-playground/validator/v10"
)

// ValidatorError represents an error response from the wpm schema validator.
type ValidatorError struct {
	Errors []ValidatorErrorItem
}

type ValidatorErrorItem struct {
	Message     string
	FailedField string
}

// Allow ValidatorError to satisfy error interface.
func (err *ValidatorError) Error() string {
	// Add all error messages to a string.
	var messages string

	for _, e := range err.Errors {
		messages += e.FailedField + "\n"
	}

	return messages
}

// HandleValidatorError parses validation error into a ValidatorError.
func HandleValidatorError(errs error) error {
	validationErrors := &ValidatorError{}

	for _, err := range errs.(goValidator.ValidationErrors) {
		ve := &ValidatorErrorItem{}

		ve.Message = err.Error()
		ve.FailedField = err.Field()

		validationErrors.Errors = append(validationErrors.Errors, *ve)
	}

	return validationErrors
}
