package command

import (
	"io"

	"wpm/cli/streams"

	"github.com/moby/term"
)

// CLIOption is a functional argument to apply options to a [WpmCli]. These
// options can be passed to [NewWpmCli] to initialize a new CLI, or
// applied with [WpmCli.Initialize] or [WpmCli.Apply].
type CLIOption func(cli *WpmCli) error

// WithStandardStreams sets a cli in, out and err streams with the standard streams.
func WithStandardStreams() CLIOption {
	return func(cli *WpmCli) error {
		// Set terminal emulation based on platform as required.
		stdin, stdout, stderr := term.StdStreams()
		cli.in = streams.NewIn(stdin)
		cli.out = streams.NewOut(stdout)
		cli.err = streams.NewOut(stderr)
		return nil
	}
}

// WithCombinedStreams uses the same stream for the output and error streams.
func WithCombinedStreams(combined io.Writer) CLIOption {
	return func(cli *WpmCli) error {
		s := streams.NewOut(combined)
		cli.out = s
		cli.err = s
		return nil
	}
}

// WithInputStream sets a cli input stream.
func WithInputStream(in io.ReadCloser) CLIOption {
	return func(cli *WpmCli) error {
		cli.in = streams.NewIn(in)
		return nil
	}
}

// WithOutputStream sets a cli output stream.
func WithOutputStream(out io.Writer) CLIOption {
	return func(cli *WpmCli) error {
		cli.out = streams.NewOut(out)
		return nil
	}
}

// WithErrorStream sets a cli error stream.
func WithErrorStream(err io.Writer) CLIOption {
	return func(cli *WpmCli) error {
		cli.err = streams.NewOut(err)
		return nil
	}
}
