package debug

import (
	"os"

	"github.com/rs/zerolog"
)

// Enable sets the WPM_DEBUG env var to true
// and makes the logger to log at debug level.
func Enable() {
	_ = os.Setenv("WPM_DEBUG", "1")
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// Disable sets the WPM_DEBUG env var to false
// and makes the logger to log at info level.
func Disable() {
	_ = os.Setenv("WPM_DEBUG", "")
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// IsEnabled checks whether the debug flag is set or not.
func IsEnabled() bool {
	return os.Getenv("WPM_DEBUG") != ""
}
