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

// Runtime struct to define runtime requirements
//
// Unlike `Requires`, which specifies version constraints for the package itself,
// `Runtime` specifies the actual runtime environment versions required.
//
// Example:
//
//	"runtime": {
//	    "wp": "5.8",
//	    "php": "7.4"
//	}
type Runtime struct {
	WP  string `json:"wp,omitempty"`
	PHP string `json:"php,omitempty"`
}

// PackageConfig struct to define the package configuration
type PackageConfig struct {
	BinDir     string   `json:"bin-dir,omitempty"`
	ContentDir string   `json:"content-dir,omitempty"`
	Runtime    *Runtime `json:"runtime,omitempty"`
}

// Requires holds wp and php version constraints for a package
//
// Example:
//
//	"requires": {
//	    "wp": ">=5.7 <6.0",
//	    "php": ">=7.4 <8.1"
//	}
type Requires struct {
	WP  string `json:"wp,omitempty"`
	PHP string `json:"php,omitempty"`
}

type Bin map[string]string
type Scripts map[string]string
type Dependencies map[string]string
