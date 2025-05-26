package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"wpm/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// ClientOptions are the options used to configure the client cli.
type ClientOptions struct {
	Debug     bool
	LogLevel  string
	ConfigDir string
	Cwd       string
}

// NewClientOptions returns a new ClientOptions.
func NewClientOptions() *ClientOptions {
	return &ClientOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (o *ClientOptions) InstallFlags(flags *pflag.FlagSet) {
	configDir := config.Dir()

	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
	flags.StringVar(&o.Cwd, "cwd", "", "Set the current working directory")
	flags.StringVar(&o.ConfigDir, "config", configDir, "Location of client config files")
	flags.StringVarP(&o.LogLevel, "log-level", "l", "info", `Set the logging level ("debug", "info", "warn", "error", "fatal")`)
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (o *ClientOptions) SetDefaultOptions(flags *pflag.FlagSet) {}

// SetLogLevel sets the logrus logging level
func SetLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse logging level: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

// GetWorkingDir returns the working directory for the client.
// By default recurse to root, find wpm.json and return the directory
// containing it. If cwd is provided, it returns that directory.
// If none, fallback to the current working directory.
func GetWorkingDir(cwd string) string {
	if cwd != "" {
		if _, err := os.Stat(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid working directory: %s\n", cwd)
			os.Exit(1)
		}

		cwd, err := filepath.Abs(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to resolve absolute path for cwd: %s\n", cwd)
			os.Exit(1)
		}

		return cwd
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get current working directory: %v\n", err)
		os.Exit(1)
	}

	return cwd
}
