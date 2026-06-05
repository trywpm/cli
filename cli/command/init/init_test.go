package init

import (
	"fmt"
	"testing"

	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
	"go.wpm.so/cli/pkg/wp/parser"
)

func lineSeparator() string { return string(rune(0x2028)) }

func TestBuildWpmConfigDropsInvalidMetadata(t *testing.T) {
	bad := lineSeparator()
	headers := parser.PluginFileHeaders{
		Author:      "Jane" + bad + "Doe",               // unsafe -> dropped
		Description: "valid description" + bad,          // unsafe -> dropped
		License:     "x",                                // too short -> dropped
		Tags:        []string{"good", "ba" + bad + "d"}, // one unsafe -> filtered
	}

	cfg := buildWpmConfig(initOptions{}, "plugin", headers, map[string]any{})
	cfg.Name = "my-plugin"
	cfg.Version = "1.0.0"

	if cfg.Author != "" {
		t.Errorf("expected unsafe author to be cleared, got %q", cfg.Author)
	}
	if cfg.Description != "" {
		t.Errorf("expected unsafe description to be cleared, got %q", cfg.Description)
	}
	if cfg.License != "" {
		t.Errorf("expected invalid license to be cleared, got %q", cfg.License)
	}
	for _, tag := range cfg.Tags {
		if validator.IsSafeString(tag) != nil {
			t.Errorf("unsafe tag survived sanitization: %q", tag)
		}
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config built from adversarial headers should be valid, got: %v", err)
	}
}

func TestBuildWpmConfigCapsAndSanitizesDependencies(t *testing.T) {
	required := make([]string, 0, 21)
	required = append(required, "my-plugin") // self-reference, must be dropped
	for i := range 20 {
		required = append(required, fmt.Sprintf("dep-%02d", i))
	}

	headers := parser.PluginFileHeaders{RequiresPlugins: required}
	cfg := buildWpmConfig(initOptions{}, "plugin", headers, map[string]any{})
	cfg.Name = "my-plugin"
	cfg.Version = "1.0.0"
	removeSelfDependency(cfg)

	if cfg.Dependencies == nil {
		t.Fatal("expected dependencies to be populated")
	}
	if n := len(*cfg.Dependencies); n > 16 {
		t.Errorf("expected at most 16 dependencies, got %d", n)
	}
	if _, ok := (*cfg.Dependencies)["my-plugin"]; ok {
		t.Error("expected self-dependency to be removed")
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config with capped dependencies should be valid, got: %v", err)
	}
}

func TestRemoveSelfDependencyClearsEmptyMap(t *testing.T) {
	cfg := wpmjson.New()
	cfg.Name = "solo"
	deps := types.Dependencies{"solo": "*"}
	cfg.Dependencies = &deps

	removeSelfDependency(cfg)

	if cfg.Dependencies != nil {
		t.Errorf("expected dependencies to be nil after removing the only entry, got %v", *cfg.Dependencies)
	}
}
