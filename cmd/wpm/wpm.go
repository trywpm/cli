package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/morikuni/aec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/commands"
	cliflags "go.wpm.so/cli/cli/flags"
	"go.wpm.so/cli/cli/version"
	platformsignals "go.wpm.so/cli/cmd/wpm/internal/signals"
)

// exitCodeInterrupted is the exit code used when the process is interrupted by a signal (e.g., Ctrl-C).
const exitCodeInterrupted = 130

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), platformsignals.TerminationSignals...)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	err := wpmMain(ctx)
	if err == nil {
		return 0
	}

	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return exitCodeInterrupted
	}

	if err.Error() != "" {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}

	return getExitCode(err)
}

func wpmMain(ctx context.Context) error {
	wpmCli, err := command.NewWpmCli()
	if err != nil {
		return err
	}

	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        wpmCli.Err(),
		NoColor:    !wpmCli.Err().IsColorEnabled(),
		TimeFormat: time.Kitchen,
	}).With().Timestamp().Logger()

	return runWpm(ctx, wpmCli)
}

// getExitCode returns the exit-code to use for the given error.
// If err is a [cli.StatusError] and has a StatusCode set, it uses the
// status-code from it, otherwise it returns "1" for any error.
func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	if stErr, ok := errors.AsType[cli.StatusError](err); ok && stErr.StatusCode != 0 {
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

	var ver string
	if wpmCli.Out().IsColorEnabled() {
		ver = "v" + version.Version + aec.LightBlackF.Apply(" ("+version.GitCommit+")")
	} else {
		ver = "v" + version.Version + " (" + version.GitCommit + ")"
	}

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

			_, _ = fmt.Fprintf(wpmCli.Err(), "wpm: unknown command: wpm %s\n", args[0])

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
				_, _ = fmt.Fprint(wpmCli.Err(), "\nDid you mean this?\n")
				for _, c := range candidates {
					_, _ = fmt.Fprintf(wpmCli.Err(), "\t%s\n", c)
				}
			}

			return errors.New("\nRun 'wpm --help' for more information")
		},
		Version:               ver,
		DisableFlagsInUseLine: true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd:    true,
			DisableDefaultCmd:   false,
			DisableDescriptions: os.Getenv("WPM_CLI_DISABLE_COMPLETION_DESCRIPTION") != "",
		},
	}

	// Disable file-completion by default. Most commands and flags should not
	// complete with filenames.
	cmd.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)

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

	// We've parsed global args already, so reset args to those
	// which remain.
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}
