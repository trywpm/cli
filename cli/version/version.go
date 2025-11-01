package version

// Default build-time variable.
// These values are overridden via ldflags
var (
	PlatformName = ""
	Version      = "1.0.0-test" // Test version for local development
	GitCommit    = "unknown-commit"
	BuildTime    = "unknown-buildtime"
)
