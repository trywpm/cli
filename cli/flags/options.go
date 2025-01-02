package flags

import (
	"fmt"
	"os"

	"wpm/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// ClientOptions are the options used to configure the client cli.
type ClientOptions struct {
	Debug     bool
	LogLevel  string
	ConfigDir string
}

// NewClientOptions returns a new ClientOptions.
func NewClientOptions() *ClientOptions {
	return &ClientOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (o *ClientOptions) InstallFlags(flags *pflag.FlagSet) {
	configDir := config.Dir()

	flags.StringVar(&o.ConfigDir, "config", configDir, "Location of client config files")
	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
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
