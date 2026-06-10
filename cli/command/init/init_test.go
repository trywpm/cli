package init

import (
	"context"
	"fmt"
	"testing"

	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
	"go.wpm.so/cli/pkg/wp/parser"
)

func lineSeparator() string { return string(rune(0x2028)) }

// resolveTo returns a latestVersionResolver that pins every package to version.
func resolveTo(version string) latestVersionResolver {
	return func(context.Context, string) (string, bool) { return version, true }
}

func TestBuildWpmConfigDropsInvalidMetadata(t *testing.T) {
	bad := lineSeparator()
	headers := parser.PluginFileHeaders{
		Author:      "Jane" + bad + "Doe",               // unsafe -> dropped
		Description: "valid description" + bad,          // unsafe -> dropped
		License:     "x",                                // too short -> dropped
		Tags:        []string{"good", "ba" + bad + "d"}, // one unsafe -> filtered
	}

	cfg := buildWpmConfig(context.Background(), initOptions{name: "my-plugin"}, "plugin", headers, map[string]any{}, resolveTo("1.0.0"))
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
	required = append(required, "my-plugin")
	for i := range 20 {
		required = append(required, fmt.Sprintf("dep-%02d", i))
	}

	headers := parser.PluginFileHeaders{RequiresPlugins: required}
	cfg := buildWpmConfig(context.Background(), initOptions{name: "my-plugin"}, "plugin", headers, map[string]any{}, resolveTo("1.0.0"))
	cfg.Version = "1.0.0"

	if cfg.Dependencies == nil {
		t.Fatal("expected dependencies to be populated")
	}
	if n := len(*cfg.Dependencies); n > 16 {
		t.Errorf("expected at most 16 dependencies, got %d", n)
	}
	if _, ok := (*cfg.Dependencies)["my-plugin"]; ok {
		t.Error("expected the cyclic self-dependency to be removed")
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config with capped dependencies should be valid, got: %v", err)
	}
}

func TestRemoveCyclicDependencyClearsEmptyMap(t *testing.T) {
	cfg := wpmjson.New()
	cfg.Name = "solo"
	deps := types.Dependencies{"solo": "1.0.0"}
	cfg.Dependencies = &deps

	removeCyclicDependency(cfg)

	if cfg.Dependencies != nil {
		t.Errorf("expected dependencies to be nil after removing the only entry, got %v", *cfg.Dependencies)
	}
}

func TestBuildWpmConfigResolvesDependencyVersions(t *testing.T) {
	headers := parser.PluginFileHeaders{
		RequiresPlugins: []string{"found-plugin", "missing-plugin"},
	}
	resolve := func(_ context.Context, name string) (string, bool) {
		if name == "found-plugin" {
			return "2.3.4", true
		}
		return "", false
	}

	cfg := buildWpmConfig(context.Background(), initOptions{name: "host-plugin"}, "plugin", headers, map[string]any{}, resolve)
	cfg.Version = "1.0.0"

	if cfg.Dependencies == nil {
		t.Fatal("expected dependencies to be populated")
	}
	deps := *cfg.Dependencies
	if got := deps["found-plugin"]; got != "2.3.4" {
		t.Errorf("expected found-plugin pinned to resolved version 2.3.4, got %q", got)
	}
	if _, ok := deps["missing-plugin"]; ok {
		t.Error("expected missing-plugin to be skipped when the registry has no latest version")
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("config with resolved dependencies should be valid, got: %v", err)
	}
}

func TestResolveRequiredPluginsLowercasesSlugs(t *testing.T) {
	resolve := func(_ context.Context, name string) (string, bool) {
		if name == "woocommerce" {
			return "8.5.0", true
		}
		return "", false
	}

	deps := types.Dependencies{}
	resolveRequiredPlugins(context.Background(), []string{" WooCommerce "}, &deps, resolve)

	if got, ok := deps["woocommerce"]; !ok || got != "8.5.0" {
		t.Fatalf("expected lowercased woocommerce@8.5.0, got %q (present=%v)", got, ok)
	}
	if _, ok := deps["WooCommerce"]; ok {
		t.Error("expected the original mixed-case key to be absent")
	}
}
