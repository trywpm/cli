package command

import (
	"io"

	"wpm/cli/config"
	"wpm/cli/config/configfile"
	"wpm/cli/debug"
	cliflags "wpm/cli/flags"
	"wpm/cli/streams"

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
	SetIn(in *streams.In)
	Apply(ops ...CLIOption) error
}

// WpmCli is an instance the wpm command line client.
// Instances of the client can be returned from NewWpmCli.
type WpmCli struct {
	in         *streams.In
	out        *streams.Out
	err        *streams.Out
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

// Initialize the dockerCli runs initialization that must happen after command
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

	// cli.options = opts
	cli.configFile = config.LoadDefaultConfigFile(cli.err)

	return nil
}
