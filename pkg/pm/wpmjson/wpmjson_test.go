package wpmjson

import (
	"strings"
	"testing"

	"go.wpm.so/cli/pkg/pm/wpmjson/types"
)

func baseConfig() *Config {
	return &Config{
		Name:    "my-plugin",
		Version: "1.0.0",
		Type:    types.TypePlugin,
	}
}

func TestConfigValidateAuthor(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		wantErr bool
	}{
		{"empty author skipped", "", false},
		{"valid author", "Jane Doe", false},
		{"too short author", "a", true},
		{"too long author", strings.Repeat("a", 165), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.Author = tc.author
			err := cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() with author %q error = %v, wantErr %v", tc.author, err, tc.wantErr)
			}
		})
	}
}

func TestConfigValidateDependencyIntegrity(t *testing.T) {
	t.Run("cannot depend on itself", func(t *testing.T) {
		cfg := baseConfig()
		deps := types.Dependencies{"my-plugin": "*"}
		cfg.Dependencies = &deps

		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "depend on itself") {
			t.Fatalf("expected self-dependency error, got %v", err)
		}
	})

	t.Run("cannot be listed in both", func(t *testing.T) {
		cfg := baseConfig()
		deps := types.Dependencies{"shared-dep": "1.0.0"}
		devDeps := types.Dependencies{"shared-dep": "1.0.0"}
		cfg.Dependencies = &deps
		cfg.DevDependencies = &devDeps

		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "both dependencies and devDependencies") {
			t.Fatalf("expected both-lists error, got %v", err)
		}
	})

	t.Run("distinct dependencies are valid", func(t *testing.T) {
		cfg := baseConfig()
		deps := types.Dependencies{"dep-a": "1.0.0"}
		devDeps := types.Dependencies{"dep-b": "2.0.0"}
		cfg.Dependencies = &deps
		cfg.DevDependencies = &devDeps

		if err := cfg.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
