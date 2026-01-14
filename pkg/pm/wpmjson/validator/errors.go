package validator

import (
	"fmt"
	"strings"
)

// ValidationError holds a specific field error.
type ValidationError struct {
	Field   string
	Message string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Field, v.Message)
}

// ErrorList collects multiple validation errors.
type ErrorList []ValidationError

// Add appends a single error to the list.
func (e *ErrorList) Add(field string, err error) {
	if err != nil {
		*e = append(*e, ValidationError{Field: field, Message: err.Error()})
	}
}

// AddMsg allows adding a string message directly.
func (e *ErrorList) AddMsg(field string, msg string) {
	*e = append(*e, ValidationError{Field: field, Message: msg})
}

// Merge combines another error (single or ErrorList) into this list.
func (e *ErrorList) MustMerge(err error) {
	if err == nil {
		return
	}

	if list, ok := err.(ErrorList); ok {
		*e = append(*e, list...)
	} else {
		panic("Merge called with non-ErrorList error")
	}
}

// Err returns nil if no errors, or the list itself if errors exist.
func (e ErrorList) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

func (e ErrorList) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("error: wpm.json validation failed (%d errors):\n", len(e)))
	for _, err := range e {
		b.WriteString(fmt.Sprintf("error:    %s\n", err.Error()))
	}

	errStr := b.String()
	return strings.TrimRight(errStr, "\n")
}
