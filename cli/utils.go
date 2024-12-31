package cli

import "os"

// IsColorDisabled returns true if environment variables NO_COLOR or CLICOLOR prohibit usage of color codes
// in terminal output.
func IsColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0"
}
