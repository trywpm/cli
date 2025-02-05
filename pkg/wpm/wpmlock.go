package wpm

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

const Lockfile = "wpm.lock"

type PackagesMap map[string]Json

type LockJson struct {
	LockfileVersion int               `json:"lockfileVersion" validate:"required,min=1"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Packages        PackagesMap       `json:"packages,omitempty"`
	Platform        Platform          `json:"platform" validate:"required"`
}

// ReadWpmLock reads the wpm.lock file from the passed path
func ReadWpmLock(path string) (*LockJson, error) {
	f, err := os.Open(filepath.Join(path, "wpm.lock"))
	switch {
	case os.IsNotExist(err):
		return nil, &LockfileNotFound{}
	case err != nil:
		return nil, err
	}
	defer f.Close()

	var lj LockJson
	if err := json.NewDecoder(f).Decode(&lj); err != nil && !errors.Is(err, io.EOF) {
		return nil, &CorruptLockfile{}
	}

	return &lj, nil
}

// WriteWpmLock writes the wpm.lock file to the passed path
func WriteWpmLock(path string, lj *LockJson) error {
	file, err := os.Create(filepath.Join(path, "wpm.lock"))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")

	if err := encoder.Encode(lj); err != nil {
		return err
	}

	return nil
}

// ValidateWpmLock validates the wpm.lock file
func ValidateWpmLock(validator *validator.Validate, lj *LockJson) error {
	if err := validator.Struct(lj); err != nil {
		return &CorruptLockfile{}
	}

	return nil
}

// ReadAndValidateWpmLock reads and validates the wpm.lock file
func ReadAndValidateWpmLock(path string) (*LockJson, error) {
	wpmLock, err := ReadWpmLock(path)
	if err != nil {
		return nil, err
	}

	ve, err := NewValidator()
	if err != nil {
		return nil, err
	}

	if err := ValidateWpmLock(ve, wpmLock); err != nil {
		return nil, err
	}

	return wpmLock, nil
}
