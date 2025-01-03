package validator

import (
	"github.com/go-playground/validator/v10"
)

// Dist struct to define the dist field
type Dist struct {
	Size      int    `json:"size" validate:"gte=0"`
	FileCount int    `json:"fileCount" validate:"gte=0"`
	Digest    string `json:"digest" validate:"required,sha256"`
}

// Platform struct to define the platform field
type Platform struct {
	WP  string `json:"wp" validate:"required"`
	PHP string `json:"php" validate:"required"`
}

// Package struct to define the wpm.json schema
type Package struct {
	Name            string            `json:"name" validate:"required,regex=^[a-zA-Z0-9-_]+$,min=3,max=164"`
	Description     string            `json:"description,omitempty"`
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

func NewValidator() *validator.Validate {
	validator := validator.New()
	return validator
}

func ValidatePackage(pkg Package, v *validator.Validate) error {
	return v.Struct(pkg)
}
