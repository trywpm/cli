package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/command/commands"
	cliflags "wpm/cli/flags"
	"wpm/cli/version"
	platformsignals "wpm/cmd/wpm/internal/signals"

	"github.com/containerd/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type errCtxSignalTerminated struct {
	signal os.Signal
}

func (errCtxSignalTerminated) Error() string {
	return ""
}

func main() {
	err := wpmMain(context.Background())
	if errors.As(err, &errCtxSignalTerminated{}) {
		os.Exit(getExitCode(err))
	}

	if err != nil && !errdefs.IsCanceled(err) {
		if err.Error() != "" {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(getExitCode(err))
	}
}

func notifyContext(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	ctxCause, cancel := context.WithCancelCause(ctx)

	go func() {
		select {
		case <-ctx.Done():
			signal.Stop(ch)
			return
		case sig := <-ch:
			cancel(errCtxSignalTerminated{
				signal: sig,
			})
			signal.Stop(ch)
			return
		}
	}()

	return ctxCause, func() {
		signal.Stop(ch)
		cancel(nil)
	}
}

func wpmMain(ctx context.Context) error {
	ctx, cancelNotify := notifyContext(ctx, platformsignals.TerminationSignals...)
	defer cancelNotify()

	wpmCli, err := command.NewWpmCli()
	if err != nil {
		return err
	}
	logrus.SetOutput(wpmCli.Err())

	return runWpm(ctx, wpmCli)
}

// getExitCode returns the exit-code to use for the given error.
// If err is a [cli.StatusError] and has a StatusCode set, it uses the
// status-code from it, otherwise it returns "1" for any error.
func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	var userTerminatedErr errCtxSignalTerminated
	if errors.As(err, &userTerminatedErr) {
		s, ok := userTerminatedErr.signal.(syscall.Signal)
		if !ok {
			return 1
		}
		return 128 + int(s)
	}

	var stErr cli.StatusError
	if errors.As(err, &stErr) && stErr.StatusCode != 0 {
		return stErr.StatusCode
	}

	// No status-code provided; all errors should have a non-zero exit code.
	return 1
}

func newWpmCommand(wpmCli *command.WpmCli) *cli.TopLevelCommand {
	var (
		opts    *cliflags.ClientOptions
		helpCmd *cobra.Command
	)

	cmd := &cobra.Command{
		Use:              "wpm [OPTIONS] COMMAND [ARG...]",
		Short:            "Package Manager for WordPress ecosystem",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.ShowHelp(wpmCli.Err())(cmd, args)
			}

			fmt.Fprintf(wpmCli.Err(), "wpm: unknown command: wpm %s\n", args[0])

			var candidates []string
			if args[0] == "help" {
				candidates = []string{"--help"}
			} else {
				if cmd.SuggestionsMinimumDistance <= 0 {
					cmd.SuggestionsMinimumDistance = 2
				}
				candidates = cmd.SuggestionsFor(args[0])
			}

			if len(candidates) > 0 {
				fmt.Fprint(wpmCli.Err(), "\nDid you mean this?\n")
				for _, c := range candidates {
					fmt.Fprintf(wpmCli.Err(), "\t%s\n", c)
				}
			}

			return fmt.Errorf("\nRun 'wpm --help' for more information")
		},
		Version:               fmt.Sprintf("%s, build %s", version.Version, version.GitCommit),
		DisableFlagsInUseLine: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   false,
			HiddenDefaultCmd:    true,
			DisableDescriptions: true,
		},
	}
	cmd.SetIn(wpmCli.In())
	cmd.SetOut(wpmCli.Out())
	cmd.SetErr(wpmCli.Err())

	opts, helpCmd = cli.SetupRootCommand(cmd)

	cmd.Flags().BoolP("version", "v", false, "Print version information and quit")

	setupHelpCommand(helpCmd)

	cmd.SetOut(wpmCli.Out())
	commands.AddCommands(cmd, wpmCli)

	cli.DisableFlagsInUseLine(cmd)

	// flags must be the top-level command flags, not cmd.Flags()
	return cli.NewTopLevelCommand(cmd, wpmCli, opts, cmd.Flags())
}

// forceExitAfter3TerminationSignals waits for the first termination signal
// to be caught and the context to be marked as done, then registers a new
// signal handler for subsequent signals. It forces the process to exit
// after 3 SIGTERM/SIGINT signals.
func forceExitAfter3TerminationSignals(ctx context.Context, w io.Writer) {
	// wait for the first signal to be caught and the context to be marked as done
	<-ctx.Done()
	// register a new signal handler for subsequent signals
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, platformsignals.TerminationSignals...)

	// once we have received a total of 3 signals we force exit the cli
	for i := 0; i < 2; i++ {
		<-sig
	}
	_, _ = fmt.Fprint(w, "\ngot 3 SIGTERM/SIGINTs, forcefully exiting\n")
	os.Exit(1)
}

func setupHelpCommand(helpCmd *cobra.Command) {
	origRun := helpCmd.Run
	origRunE := helpCmd.RunE

	helpCmd.Run = nil
	helpCmd.RunE = func(c *cobra.Command, args []string) error {
		if origRunE != nil {
			return origRunE(c, args)
		}
		origRun(c, args)
		return nil
	}
}

func runWpm(ctx context.Context, wpmCli *command.WpmCli) error {
	tcmd := newWpmCommand(wpmCli)

	cmd, args, err := tcmd.HandleGlobalFlags()
	if err != nil {
		return err
	}

	if err := tcmd.Initialize(); err != nil {
		return err
	}

	// This is a fallback for the case where the command does not exit
	// based on context cancellation.
	go forceExitAfter3TerminationSignals(ctx, wpmCli.Err())

	// We've parsed global args already, so reset args to those
	// which remain.
	cmd.SetArgs(args)
	err = cmd.ExecuteContext(ctx)

	return err
}
