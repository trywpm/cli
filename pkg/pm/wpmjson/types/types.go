package types

// PackageType defines the type of the package
type PackageType string

func (pt PackageType) String() string {
	return string(pt)
}

func (pt PackageType) Valid() bool {
	switch pt {
	case TypeTheme, TypePlugin, TypeMuPlugin:
		return true
	default:
		return false
	}
}

// PackageVisibility defines the visibility of the package
type PackageVisibility string

func (pv PackageVisibility) String() string {
	return string(pv)
}

func (pv PackageVisibility) Valid() bool {
	switch pv {
	case VisibilityPublic, VisibilityPrivate:
		return true
	default:
		return false
	}
}

const (
	TypeTheme    PackageType = "theme"
	TypePlugin   PackageType = "plugin"
	TypeMuPlugin PackageType = "mu-plugin"

	VisibilityPublic  PackageVisibility = "public"
	VisibilityPrivate PackageVisibility = "private"
)

// PackageConfig struct to define the package configuration
type PackageConfig struct {
	BinDir        string `json:"bin-dir,omitempty"`
	ContentDir    string `json:"content-dir,omitempty"`
	RuntimeStrict bool   `json:"runtime-strict,omitempty"`
	RuntimeWP     string `json:"runtime-wp,omitempty"`
	RuntimePHP    string `json:"runtime-php,omitempty"`
}

// Requires holds wp and php version constraints for a package
type Requires struct {
	WP  string `json:"wp,omitempty" validate:"omitempty,wpm_semver_constraint"`
	PHP string `json:"php,omitempty" validate:"omitempty,wpm_semver_constraint"`
}

type Bin map[string]string
type Scripts map[string]string
type Dependencies map[string]string
