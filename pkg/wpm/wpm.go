package wpm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"wpm/pkg/wpm/validator"

	goValidator "github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

type Wpm struct {
	config    *validator.Package
	validator *goValidator.Validate
}

const wpmJson = "wpm.json"

func readWpmJson(path string) (*validator.Package, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		path = filepath.Join(cwd, wpmJson)
	}

	_, err := os.Stat(path)
	if err != nil {
		return nil, errors.New("wpm.json file not found")
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg validator.Package
	err = json.Unmarshal(file, &pkg)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func NewWpm(withConfig bool) (*Wpm, error) {
	validator, err := validator.NewValidator()
	if err != nil {
		return nil, err
	}

	if !withConfig {
		return &Wpm{
			validator: validator,
		}, nil
	}

	config, err := readWpmJson("")
	if err != nil {
		return nil, err
	}

	return &Wpm{
		config:    config,
		validator: validator,
	}, nil
}

func (w *Wpm) WpmJson() *validator.Package {
	return w.config
}

func (w *Wpm) Validate() error {
	return w.validator.Struct(w.config)
}

func (w *Wpm) Validator() *goValidator.Validate {
	return w.validator
}
