package command

import (
	"io"
	"runtime"

	"wpm/cli/debug"
	cliflags "wpm/cli/flags"
	"wpm/cli/registry/client"
	"wpm/cli/version"
	"wpm/pkg/config"
	"wpm/pkg/config/configfile"
	"wpm/pkg/progress"
	"wpm/pkg/streams"

	"github.com/spf13/cobra"
)

// Streams is an interface which exposes the standard input and output streams
type Streams interface {
	In() *streams.In
	Out() *streams.Out
	Err() *streams.Out
}

// Cli represents the wpm command line client.
type Cli interface {
	Streams
	Registry() string
	SetIn(in *streams.In)
	Apply(ops ...CLIOption) error
	Progress() *progress.Progress
	Options() *cliflags.ClientOptions
	ConfigFile() *configfile.ConfigFile
	RegistryClient() (client.RegistryClient, error)
}

// WpmCli is an instance the wpm command line client.
// Instances of the client can be returned from NewWpmCli.
type WpmCli struct {
	registry   string
	in         *streams.In
	out        *streams.Out
	err        *streams.Out
	options    *cliflags.ClientOptions
	configFile *configfile.ConfigFile
}

// NewWpmCli returns a WpmCli instance with all operators applied on it.
// It applies by default the standard streams, and the content trust from
// environment.
func NewWpmCli(ops ...CLIOption) (*WpmCli, error) {
	defaultOps := []CLIOption{
		WithStandardStreams(),
	}
	ops = append(defaultOps, ops...)

	cli := &WpmCli{}
	if err := cli.Apply(ops...); err != nil {
		return nil, err
	}
	return cli, nil
}

// Registry returns the registry URL
func (cli *WpmCli) Registry() string {
	return cli.registry
}

// Out returns the writer used for stdout
func (cli *WpmCli) Out() *streams.Out {
	return cli.out
}

// Err returns the writer used for stderr
func (cli *WpmCli) Err() *streams.Out {
	return cli.err
}

// SetIn sets the reader used for stdin
func (cli *WpmCli) SetIn(in *streams.In) {
	cli.in = in
}

// In returns the reader used for stdin
func (cli *WpmCli) In() *streams.In {
	return cli.in
}

// ShowHelp shows the command help.
func ShowHelp(err io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SetOut(err)
		cmd.HelpFunc()(cmd, args)
		return nil
	}
}

// Apply all the operation on the cli
func (cli *WpmCli) Apply(ops ...CLIOption) error {
	for _, op := range ops {
		if err := op(cli); err != nil {
			return err
		}
	}
	return nil
}

// ConfigFile returns the ConfigFile
func (cli *WpmCli) ConfigFile() *configfile.ConfigFile {
	// TODO(thelovekesh): when would this happen? Is this only in tests (where cli.Initialize() is not called first?)
	if cli.configFile == nil {
		cli.configFile = config.LoadDefaultConfigFile(cli.err)
	}
	return cli.configFile
}

// Options returns the options used to initialize the cli
func (cli *WpmCli) Options() *cliflags.ClientOptions {
	return cli.options
}

// Initialize the wpmCli runs initialization that must happen after command
// line flags are parsed.
func (cli *WpmCli) Initialize(opts *cliflags.ClientOptions, ops ...CLIOption) error {
	for _, o := range ops {
		if err := o(cli); err != nil {
			return err
		}
	}
	cliflags.SetLogLevel(opts.LogLevel)

	if opts.ConfigDir != "" {
		config.SetDir(opts.ConfigDir)
	}

	if opts.Debug {
		debug.Enable()
	}

	cli.options = opts
	cli.registry = opts.Registry
	cli.configFile = config.LoadDefaultConfigFile(cli.err)

	return nil
}

// RegistryClient returns a client for communicating with wpm registry
func (cli *WpmCli) RegistryClient() (client.RegistryClient, error) {
	_client, err := client.NewRegistryClient(cli.Registry(), cli.configFile.AuthToken, UserAgent(), cli.out)
	if err != nil {
		return nil, err
	}

	return _client, nil
}

// UserAgent returns the user agent string used for making API requests
func UserAgent() string {
	return "wpm-cli/" + version.Version + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
}

// Progress returns the progress indicator
func (cli *WpmCli) Progress() *progress.Progress {
	return &progress.Progress{
		ProgressIndicatorEnabled: cli.Out().IsTerminal() && cli.in.IsTerminal(),
	}
}
