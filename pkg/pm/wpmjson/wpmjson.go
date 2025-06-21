package wpmjson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

const ConfigFile = "wpm.json"

// ReadWpmJson reads the wpm.json file from the passed path and
// returns the list of paths to exclude
func ReadWpmJson(path string) (*Config, error) {
	f, err := os.Open(filepath.Join(path, "wpm.json"))
	switch {
	case os.IsNotExist(err):
		return nil, fmt.Errorf("wpm.json file not found")
	case err != nil:
		return nil, err
	}
	defer f.Close()

	var j Config
	if err := json.NewDecoder(f).Decode(&j); err != nil && !errors.Is(err, io.EOF) {
		var typeError *json.UnmarshalTypeError
		if errors.As(err, &typeError) {
			return nil, errors.Errorf("wpm.json has an invalid value for field %s, expected %s but got %s", typeError.Field, typeError.Type.Name(), typeError.Value)
		}

		return nil, errors.New("error parsing wpm.json")
	}

	return &j, nil
}

func WriteWpmJson(pkg *Config, path string) error {
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

func ValidateWpmJson(validator *validator.Validate, pkg *Config) error {
	if err := validator.Struct(pkg); err != nil {
		return handleValidatorError(err)
	}

	return nil
}

func ReadAndValidateWpmJson(path string) (*Config, error) {
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
