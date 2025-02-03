package wpm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

const Config = "wpm.json"

// Platform struct to define the platform field
type Platform struct {
	WP  string `json:"wp" validate:"required"`
	PHP string `json:"php" validate:"required"`
}

// Json struct to define the wpm.json schema
type Json struct {
	Name            string            `json:"name" validate:"required,min=3,max=164"`
	Description     string            `json:"description,omitempty"`
	Private         bool              `json:"private,omitempty"`
	Type            string            `json:"type" validate:"required,oneof=plugin theme mu-plugin drop-in"`
	Version         string            `json:"version" validate:"required,semver,max=64"`
	License         string            `json:"license" validate:"omitempty"`
	Homepage        string            `json:"homepage,omitempty" validate:"omitempty,url"`
	Tags            []string          `json:"tags,omitempty" validate:"dive,max=5"`
	Team            []string          `json:"team,omitempty"`
	Bin             map[string]string `json:"bin,omitempty"`
	Platform        Platform          `json:"platform" validate:"required"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"dev_dependencies,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
}

// Description of package fields.
var PackageFieldDescriptions = map[string]string{
	"Name":            "must contain only lowercase letters, numbers, and hyphens, and be between 3 and 164 characters. (required)",
	"Description":     "should be a string. (optional)",
	"Private":         "must be a boolean. (optional)",
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

// ReadWpmJson reads the wpm.json file from the passed path and
// returns the list of paths to exclude
func ReadWpmJson(path string) (*Json, error) {
	f, err := os.Open(filepath.Join(path, "wpm.json"))
	switch {
	case os.IsNotExist(err):
		return nil, fmt.Errorf("wpm.json file not found")
	case err != nil:
		return nil, err
	}
	defer f.Close()

	var j Json
	if err := json.NewDecoder(f).Decode(&j); err != nil && !errors.Is(err, io.EOF) {
		var typeError *json.UnmarshalTypeError
		if errors.As(err, &typeError) {
			return nil, errors.Errorf("wpm.json has an invalid value for field %s, expected %s but got %s", typeError.Field, typeError.Type.Name(), typeError.Value)
		}

		return nil, errors.New("error parsing wpm.json")
	}

	return &j, nil
}

func WriteWpmJson(pkg *Json, path string) error {
	file, err := os.Create(filepath.Join(path, "wpm.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")

	if err := encoder.Encode(pkg); err != nil {
		return err
	}

	return nil
}

func ValidateWpmJson(validator *validator.Validate, pkg *Json) error {
	if err := validator.Struct(pkg); err != nil {
		return handleValidatorError(err)
	}

	return nil
}

func ReadAndValidateWpmJson(path string) (*Json, error) {
	wpmJson, err := ReadWpmJson(path)
	if err != nil {
		return nil, err
	}

	ve, err := NewValidator()
	if err != nil {
		return nil, err
	}

	if err = ValidateWpmJson(ve, wpmJson); err != nil {
		return nil, err
	}

	return wpmJson, nil
}
