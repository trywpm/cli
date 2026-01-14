package validator

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"wpm/pkg/pm/wpmjson/types"

	"github.com/Masterminds/semver/v3"
)

var (
	packageNameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// IsValidPackageName checks if the package name adheres to naming conventions.
func IsValidPackageName(name string) error {
	if len(name) < 3 {
		return fmt.Errorf("must be at least 3 characters")
	}
	if len(name) > 164 {
		return fmt.Errorf("must be at most 164 characters")
	}
	if !packageNameRegex.MatchString(name) {
		return fmt.Errorf("must consist of lowercase alphanumeric characters separated by hyphens")
	}
	return nil
}

// IsValidPackageType checks if the package type is valid.
func IsValidPackageType(t types.PackageType) error {
	if !t.Valid() {
		return fmt.Errorf("must be one of: theme, plugin, or mu-plugin")
	}
	return nil
}

// IsValidVersion checks if the version string is a valid semantic version.
func IsValidVersion(v string) error {
	if v == "" {
		return fmt.Errorf("cannot be empty")
	}
	if strings.HasPrefix(v, "v") {
		return fmt.Errorf("cannot start with 'v'")
	}
	if _, err := semver.StrictNewVersion(v); err != nil {
		return fmt.Errorf("must be a valid semantic version (X.Y.Z)")
	}
	return nil
}

// IsValidDescription checks if the description meets length requirements.
func IsValidDescription(desc string) error {
	if len(desc) < 3 || len(desc) > 512 {
		return fmt.Errorf("must be between 3 and 512 characters")
	}

	return IsSafeString(desc)
}

// IsValidLicense checks if the license string meets length requirements.
func IsValidLicense(license string) error {
	if len(license) < 3 || len(license) > 100 {
		return fmt.Errorf("must be between 3 and 100 characters")
	}
	return IsSafeString(license)
}

// IsValidHomepage checks if the homepage string is a valid URL.
func IsValidHomepage(homepage string) error {
	if len(homepage) < 10 || len(homepage) > 200 {
		return fmt.Errorf("must be between 10 and 200 characters")
	}

	url, err := url.Parse(homepage)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	return nil
}

// IsValidConstraint checks if the version constraint string is valid.
func IsValidConstraint(v string) error {
	if v == "" {
		return fmt.Errorf("constraint cannot be empty")
	}
	if v == "*" {
		return nil
	}
	if strings.HasPrefix(v, "v") {
		return fmt.Errorf("constraint cannot start with 'v'")
	}
	if _, err := semver.NewConstraint(v); err != nil {
		return fmt.Errorf("invalid version constraint")
	}
	return nil
}

// IsSafeString checks for disallowed control and special unicode characters.
func IsSafeString(s string) error {
	for _, r := range s {
		if r == '\u2028' || r == '\u2029' || r == '\uFFFD' || r == '\uFFFC' || r == '\u3164' {
			return fmt.Errorf("contains invalid unicode characters")
		}
		if unicode.IsControl(r) {
			if r == '\t' || r == '\n' || r == '\r' || r == '\u200D' || (r >= 0xE0020 && r <= 0xE007F) {
				continue
			}
			return fmt.Errorf("contains invalid control characters")
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

// ValidateTeam checks limits, formatting, and uniqueness for team members.
func ValidateTeam(team []string) error {
	var errs ErrorList
	if len(team) > 100 {
		errs.AddMsg("team", "cannot have more than 100 members")
	}

	seen := make(map[string]bool)
	for i, member := range team {
		field := fmt.Sprintf("team[%d]", i)

		if len(member) < 2 || len(member) > 100 {
			errs.AddMsg(field, "must be between 2 and 100 characters")
		}

		if err := IsSafeString(member); err != nil {
			errs.Add(field, err)
		}

		if seen[member] {
			errs.AddMsg("team", fmt.Sprintf("duplicate member '%s'", member))
		}
		seen[member] = true
	}
	return errs.Err()
}

// ValidateDependencies checks the validity of dependency names and constraints.
func ValidateDependencies(deps map[string]string, fieldName string) error {
	if deps == nil {
		return nil
	}
	var errs ErrorList

	if len(deps) > 16 {
		errs.AddMsg(fieldName, "cannot have more than 16 dependencies")
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
