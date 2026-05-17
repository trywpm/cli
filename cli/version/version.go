package version

// Default build-time variable.
// These values are overridden via ldflags
var (
	PlatformName = ""
	Version      = "0.1.12"
	GitCommit    = "unknown-commit"
	BuildTime    = "unknown-buildtime"
)
