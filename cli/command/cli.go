package command

import (
	"io"

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
	in  *streams.In
	out *streams.Out
	err *streams.Out
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
