package validator

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"

	"go.wpm.so/cli/pkg/pm/wpmjson/types"
)

var packageNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const unsafeStringMsg = "contains invalid control characters or invisible formatting"

// MaxDependencies is the limit on entries in dependencies or devDependencies.
const MaxDependencies = 16

// IsValidPackageName checks if the package name adheres to naming conventions.
func IsValidPackageName(name string) error {
	if len(name) < 3 {
		return errors.New("must be at least 3 characters")
	}
	if len(name) > 164 {
		return errors.New("must be at most 164 characters")
	}
	if !packageNameRegex.MatchString(name) {
		return errors.New("must consist of lowercase alphanumeric characters separated by hyphens")
	}
	return nil
}

// IsValidDistTag checks a dist tag follows package-name formatting and does not
// resemble a semantic version or range, so it can never collide with one.
func IsValidDistTag(tag string) error {
	if len(tag) < 3 {
		return errors.New("must be at least 3 characters")
	}
	if len(tag) > 64 {
		return errors.New("must be at most 64 characters")
	}
	if !packageNameRegex.MatchString(tag) {
		return errors.New("must consist of lowercase alphanumeric characters separated by hyphens")
	}
	if _, err := semver.NewConstraint(tag); err == nil {
		return errors.New("cannot resemble a valid semantic version or range")
	}
	return nil
}

// IsValidPackageType checks if the package type is valid.
func IsValidPackageType(t types.PackageType) error {
	if !t.Valid() {
		return errors.New("must be one of: theme, plugin, or mu-plugin")
	}
	return nil
}

// IsValidVersion checks if the version string is a valid semantic version.
func IsValidVersion(v string) error {
	if v == "" {
		return errors.New("cannot be empty")
	}
	if len(v) < 5 {
		return errors.New("must be at least 5 characters")
	}
	if len(v) > 64 {
		return errors.New("must be at most 64 characters")
	}
	if v != strings.TrimSpace(v) {
		return errors.New("cannot contain leading or trailing whitespace")
	}
	if strings.HasPrefix(v, "v") {
		return errors.New("cannot start with 'v'")
	}
	if _, err := semver.StrictNewVersion(v); err != nil {
		return errors.New("must be a valid semantic version (X.Y.Z)")
	}
	return nil
}

// IsValidDescription checks if the description meets length requirements.
func IsValidDescription(desc string) error {
	if len(desc) < 3 || len(desc) > 512 {
		return errors.New("must be between 3 and 512 characters")
	}

	return IsSafeString(desc)
}

// IsValidLicense checks if the license string meets length requirements.
func IsValidLicense(license string) error {
	if len(license) < 3 || len(license) > 100 {
		return errors.New("must be between 3 and 100 characters")
	}
	return IsSafeString(license)
}

// IsValidHomepage checks if the homepage string is a valid URL.
func IsValidHomepage(homepage string) error {
	if len(homepage) < 10 || len(homepage) > 200 {
		return errors.New("must be between 10 and 200 characters")
	}

	u, err := url.Parse(homepage)
	if err != nil {
		return errors.New("must be a valid URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}

	return nil
}

// IsValidConstraint checks if the version constraint string is valid.
func IsValidConstraint(v string) error {
	if v == "" {
		return errors.New("constraint cannot be empty")
	}
	if len(v) > 64 {
		return errors.New("constraint must be at most 64 characters")
	}
	if v != strings.TrimSpace(v) {
		return errors.New("constraint cannot contain leading or trailing whitespace")
	}
	if v == "*" {
		return nil
	}
	if strings.HasPrefix(v, "v") {
		return errors.New("constraint cannot start with 'v'")
	}
	if _, err := semver.NewConstraint(v); err != nil {
		return errors.New("invalid version constraint")
	}
	return nil
}

// IsSafeString rejects any character in the Unicode "Other" category (Cc, Cf,
// Co, Cs) outside the allowlist, plus separators and replacement characters
// that are not in that category but still break rendering or eval.
func IsSafeString(s string) error {
	for _, r := range s {
		switch r {
		case '\t', '\n', '\r', '\u200C', '\u200D': // whitespace and zero-width joiners
			continue
		case '\u2028', '\u2029', '\uFFFD', '\uFFFC', '\u3164':
			return errors.New(unsafeStringMsg)
		}
		if r >= 0xE0020 && r <= 0xE007F { // emoji tag characters
			continue
		}
		if unicode.In(r, unicode.C) {
			return errors.New(unsafeStringMsg)
		}
	}
	return nil
}

// ValidateTags checks limits, formatting, and uniqueness for tags.
func ValidateTags(tags []string) error {
	var errs ErrorList
	if len(tags) > 5 {
		errs.AddMsg("tags", "cannot have more than 5 tags")
	}

	seen := make(map[string]bool, len(tags))
	for i, tag := range tags {
		field := fmt.Sprintf("tags[%d]", i)

		if len(tag) < 2 || len(tag) > 64 {
			errs.AddMsg(field, "must be between 2 and 64 characters")
		}

		if err := IsSafeString(tag); err != nil {
			errs.Add(field, err)
		}

		if seen[tag] {
			errs.AddMsg("tags", fmt.Sprintf("duplicate tag '%s'", tag))
		}
		seen[tag] = true
	}
	return errs.Err()
}

// IsValidAuthor checks the author name length and character safety.
func IsValidAuthor(author string) error {
	if len(author) < 2 {
		return errors.New("must be at least 2 characters")
	}
	if len(author) > 164 {
		return errors.New("must be at most 164 characters")
	}
	return IsSafeString(author)
}

// ValidateDependencies checks the validity of dependency names and constraints.
func ValidateDependencies(deps map[string]string, fieldName string) error {
	if deps == nil {
		return nil
	}
	var errs ErrorList

	if len(deps) > MaxDependencies {
		errs.AddMsg(fieldName, fmt.Sprintf("cannot have more than %d dependencies", MaxDependencies))
	}

	for name, version := range deps {
		if err := IsValidPackageName(name); err != nil {
			errs.Add(fmt.Sprintf("%s[%s]", fieldName, name), err)
		}

		if version == "*" {
			continue
		}

		// Dependencies version should be strict semver
		if err := IsValidVersion(version); err != nil {
			errs.Add(fmt.Sprintf("%s[%s]", fieldName, name), err)
		}
	}
	return errs.Err()
}

// ValidateDependencyIntegrity checks that a package does not depend on itself
// and that no dependency is listed in both dependencies and devDependencies.
func ValidateDependencyIntegrity(name string, deps, devDeps map[string]string) error {
	var errs ErrorList

	if _, ok := deps[name]; ok {
		errs.AddMsg("dependencies", "package cannot depend on itself")
	}
	if _, ok := devDeps[name]; ok {
		errs.AddMsg("devDependencies", "package cannot depend on itself")
	}

	for dep := range deps {
		if _, ok := devDeps[dep]; ok {
			errs.AddMsg(
				fmt.Sprintf("devDependencies[%s]", dep),
				fmt.Sprintf("'%s' cannot be listed in both dependencies and devDependencies", dep),
			)
		}
	}
	return errs.Err()
}

// ValidateRequires checks the validity of the WP and PHP constraints.
func ValidateRequires(wp, php string) error {
	var errs ErrorList

	if wp != "" {
		if err := IsValidConstraint(wp); err != nil {
			errs.Add("requires.wp", err)
		}
	}
	if php != "" {
		if err := IsValidConstraint(php); err != nil {
			errs.Add("requires.php", err)
		}
	}
	return errs.Err()
}

// IsValidProjectRelPath checks that a path is relative, non-empty, and stays within the project root.
func IsValidProjectRelPath(p string) error {
	if p == "" {
		return errors.New("must not be empty")
	}
	if filepath.IsAbs(p) {
		return errors.New("must be a relative path")
	}
	cleaned := filepath.Clean(p)
	if !filepath.IsLocal(cleaned) {
		return errors.New("must not contain '..' or escape the project directory")
	}
	return nil
}
