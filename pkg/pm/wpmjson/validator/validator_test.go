package validator

import (
	"strings"
	"testing"
)

func TestIsValidPackageName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 165), true},
		{"valid name", "my-plugin", false},
		{"uppercase rejected", "My-Plugin", true},
		{"underscore rejected", "my_plugin", true},
		{"leading hyphen rejected", "-plugin", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IsValidPackageName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsValidPackageName(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestIsValidDistTag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"too long", strings.Repeat("a", 65), true},
		{"too short", "ab", true},
		{"plain tag", "latest", false},
		{"hyphenated tag", "next-major", false},
		{"uppercase rejected", "Latest", true},
		{"resembles range two digits", "12", true},
		{"resembles range bare number", "100", true},
		{"resembles range hyphen version", "1-0-0", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IsValidDistTag(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsValidDistTag(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestIsValidVersionWhitespace(t *testing.T) {
	if err := IsValidVersion(" 1.0.0"); err == nil {
		t.Fatal("expected error for leading whitespace")
	}
	if err := IsValidVersion("1.0.0 "); err == nil {
		t.Fatal("expected error for trailing whitespace")
	}
	if err := IsValidVersion("1.0.0"); err != nil {
		t.Fatalf("unexpected error for valid version: %v", err)
	}
}

func TestIsValidConstraint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"wildcard", "*", false},
		{"v prefix", "v1.0.0", true},
		{"range", ">=1.0.0 <2.0.0", false},
		{"leading whitespace", " >=1.0.0", true},
		{"too long", strings.Repeat("1.0.0 ||", 12), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IsValidConstraint(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsValidConstraint(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// withRune is a helper to create test strings containing specific runes.
func withRune(cp rune) string {
	return "a" + string(cp) + "b"
}

func TestIsSafeString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"plain ascii", "hello world", false},
		{"tab and newline allowed", "a\tb\nc", false},
		{"zwj allowed", withRune(0x200D), false},              // ZWJ
		{"zwnj allowed", withRune(0x200C), false},             // ZWNJ
		{"zero width space rejected", withRune(0x200B), true}, // Cf format char
		{"soft hyphen rejected", withRune(0x00AD), true},      // Cf format char
		{"line separator rejected", withRune(0x2028), true},
		{"replacement char rejected", withRune(0xFFFD), true},
		{"hangul filler rejected", withRune(0x3164), true},
		{"null byte rejected", "a\x00b", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IsSafeString(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsSafeString(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestIsValidAuthor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "Jane Doe", false},
		{"too short", "a", true},
		{"too long", strings.Repeat("a", 165), true},
		{"max length ok", strings.Repeat("a", 164), false},
		{"unsafe char", "Jane" + string(rune(0x2028)) + "Doe", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := IsValidAuthor(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsValidAuthor(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestValidateDependencyIntegrity(t *testing.T) {
	t.Run("self dependency in deps", func(t *testing.T) {
		err := ValidateDependencyIntegrity("my-pkg", map[string]string{"my-pkg": "*"}, nil)
		if err == nil || !strings.Contains(err.Error(), "depend on itself") {
			t.Fatalf("expected self-dependency error, got %v", err)
		}
	})

	t.Run("self dependency in devDeps", func(t *testing.T) {
		err := ValidateDependencyIntegrity("my-pkg", nil, map[string]string{"my-pkg": "*"})
		if err == nil || !strings.Contains(err.Error(), "depend on itself") {
			t.Fatalf("expected self-dependency error, got %v", err)
		}
	})

	t.Run("listed in both", func(t *testing.T) {
		err := ValidateDependencyIntegrity(
			"my-pkg",
			map[string]string{"shared-dep": "1.0.0"},
			map[string]string{"shared-dep": "1.0.0"},
		)
		if err == nil || !strings.Contains(err.Error(), "both dependencies and devDependencies") {
			t.Fatalf("expected both-lists error, got %v", err)
		}
	})

	t.Run("no conflict", func(t *testing.T) {
		err := ValidateDependencyIntegrity(
			"my-pkg",
			map[string]string{"dep-a": "1.0.0"},
			map[string]string{"dep-b": "2.0.0"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
