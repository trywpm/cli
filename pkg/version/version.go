package version

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const maxVersionLength = 64

var (
	numericIdentifier   = regexp.MustCompile(`^\d+$`)
	trailingAlphaSuffix = regexp.MustCompile(`^(\d+(?:\.\d+){0,2})([A-Za-z][A-Za-z0-9.]*)$`)
)

// Normalize converts a version string into strict semver format (X.Y.Z[-prerelease][+build]).
//
// It handles common PHP/WordPress patterns:
//   - leading 'v' or 'V' prefix (v1.2.3 -> 1.2.3)
//   - leading zeros in core segments (01.0.0 -> 1.0.0)
//   - leading zeros in numeric prerelease segments (1.0.0.01 -> 1.0.0-1)
//   - short forms (1, 1.2 -> 1.0.0, 1.2.0)
//   - alphabetic suffixes without separator (1.0.0beta -> 1.0.0-beta)
//   - 4+ dotted segments collapsed into prerelease (1.0.0.0 -> 1.0.0-0)
//
// The output is guaranteed to satisfy semver.StrictNewVersion and to be no
// longer than 64 characters. Inputs that cannot be normalized return an error.
func Normalize(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", errors.New("version cannot be empty")
	}
	if len(version) > maxVersionLength {
		return "", fmt.Errorf("input version length %d exceeds maximum of %d", len(version), maxVersionLength)
	}

	// Fast path for already-normalized versions.
	if v, err := semver.StrictNewVersion(version); err == nil {
		return v.String(), nil
	}

	// Strip a single leading 'v' or 'V'.
	if version[0] == 'v' || version[0] == 'V' {
		version = version[1:]
		if version == "" {
			return "", errors.New("version contains only a 'v' prefix")
		}
	}

	// Insert hyphen before an alphabetic qualifier with no separator.
	// 1.0.0beta1 -> 1.0.0-beta1,  2.1rc1 -> 2.1-rc1,  3.0a -> 3.0-a
	version = trailingAlphaSuffix.ReplaceAllString(version, "$1-$2")

	// Collapse 4+ dotted segments into a prerelease, stripping leading zeros
	// from any purely numeric segment (semver forbids them in numeric IDs).
	// 1.0.0.0       -> 1.0.0-0
	// 1.0.0.01      -> 1.0.0-1
	// 1.0.0.01.02   -> 1.0.0-1.2
	// 1.2.3.4.5     -> 1.2.3-4.5
	// 1.0.0.alpha.1 -> 1.0.0-alpha.1
	// 1.0.0.0beta   -> 1.0.0-0beta  (mixed; left alone)
	if parts := strings.Split(version, "."); len(parts) > 3 {
		pre := make([]string, len(parts)-3)
		for i, p := range parts[3:] {
			pre[i] = stripLeadingZeros(p)
		}
		version = fmt.Sprintf("%s.%s.%s-%s",
			parts[0], parts[1], parts[2],
			strings.Join(pre, "."))
	}

	// Coerce to semver, which will handle leading zeros and short forms.
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("cannot normalize %q to semver: %w", version, err)
	}

	result := v.String()

	if len(result) > maxVersionLength {
		return "", fmt.Errorf("normalized result %q has length %d, exceeds maximum of %d",
			result, len(result), maxVersionLength)
	}
	if _, err := semver.StrictNewVersion(result); err != nil {
		return "", fmt.Errorf("normalized result %q is not strict semver: %w", result, err)
	}

	return result, nil
}

func stripLeadingZeros(s string) string {
	if !numericIdentifier.MatchString(s) {
		return s
	}
	trimmed := strings.TrimLeft(s, "0")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}
