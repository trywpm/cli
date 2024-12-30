package cli

import (
	"os"
	"strings"

	"wpm/cli/command"
	cliflags "wpm/cli/flags"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setupCommonRootCommand contains the setup common to
// SetupRootCommand and SetupPluginRootCommand.
func setupCommonRootCommand(rootCmd *cobra.Command) (*cliflags.ClientOptions, *cobra.Command) {
	opts := cliflags.NewClientOptions()
	opts.InstallFlags(rootCmd.Flags())

	cobra.AddTemplateFunc("add", func(a, b int) int { return a + b })

	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	rootCmd.PersistentFlags().MarkShorthandDeprecated("help", "use --help")
	rootCmd.PersistentFlags().Lookup("help").Hidden = true

	return opts, helpCommand
}

var helpCommand = &cobra.Command{
	Use:               "help [command]",
	Short:             "Help about the command",
	PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {},
	RunE: func(c *cobra.Command, args []string) error {
		cmd, args, e := c.Root().Find(args)
		if cmd == nil || e != nil || len(args) > 0 {
			return errors.Errorf("unknown help topic: %v", strings.Join(args, " "))
		}
		helpFunc := cmd.HelpFunc()
		helpFunc(cmd, args)
		return nil
	},
}

// TopLevelCommand encapsulates a top-level cobra command (either
// docker CLI or a plugin) and global flag handling logic necessary
// for plugins.
type TopLevelCommand struct {
	cmd    *cobra.Command
	wpmCli *command.WpmCli
	opts   *cliflags.ClientOptions
	flags  *pflag.FlagSet
	args   []string
}

// NewTopLevelCommand returns a new TopLevelCommand object
func NewTopLevelCommand(cmd *cobra.Command, wpmCli *command.WpmCli, opts *cliflags.ClientOptions, flags *pflag.FlagSet) *TopLevelCommand {
	return &TopLevelCommand{
		cmd:    cmd,
		wpmCli: wpmCli,
		opts:   opts,
		flags:  flags,
		args:   os.Args[1:],
	}
}

// SetupRootCommand sets default usage, help, and error handling for the
// root command.
func SetupRootCommand(rootCmd *cobra.Command) (opts *cliflags.ClientOptions, helpCmd *cobra.Command) {
	rootCmd.SetVersionTemplate("wpm version {{.Version}}\n")
	return setupCommonRootCommand(rootCmd)
}

// VisitAll will traverse all commands from the root.
// This is different from the VisitAll of cobra.Command where only parents
// are checked.
func VisitAll(root *cobra.Command, fn func(*cobra.Command)) {
	for _, cmd := range root.Commands() {
		VisitAll(cmd, fn)
	}
	fn(root)
}

// DisableFlagsInUseLine sets the DisableFlagsInUseLine flag on all
// commands within the tree rooted at cmd.
func DisableFlagsInUseLine(cmd *cobra.Command) {
	VisitAll(cmd, func(ccmd *cobra.Command) {
		// do not add a `[flags]` to the end of the usage line.
		ccmd.DisableFlagsInUseLine = true
	})
}

// HandleGlobalFlags takes care of parsing global flags defined on the
// command, it returns the underlying cobra command and the args it
// will be called with (or an error).
//
// On success the caller is responsible for calling Initialize()
// before calling `Execute` on the returned command.
func (tcmd *TopLevelCommand) HandleGlobalFlags() (*cobra.Command, []string, error) {
	cmd := tcmd.cmd

	// We manually parse the global arguments and find the
	// subcommand in order to properly deal with plugins. We rely
	// on the root command never having any non-flag arguments. We
	// create our own FlagSet so that we can configure it
	// (e.g. `SetInterspersed` below) in an idempotent way.
	flags := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)

	// We need !interspersed to ensure we stop at the first
	// potential command instead of accumulating it into
	// flags.Args() and then continuing on and finding other
	// arguments which we try and treat as globals (when they are
	// actually arguments to the subcommand).
	flags.SetInterspersed(false)

	// We need the single parse to see both sets of flags.
	flags.AddFlagSet(cmd.Flags())
	flags.AddFlagSet(cmd.PersistentFlags())
	// Now parse the global flags, up to (but not including) the
	// first command. The result will be that all the remaining
	// arguments are in `flags.Args()`.
	if err := flags.Parse(tcmd.args); err != nil {
		// Our FlagErrorFunc uses the cli, make sure it is initialized
		if err := tcmd.Initialize(); err != nil {
			return nil, nil, err
		}
		return nil, nil, cmd.FlagErrorFunc()(cmd, err)
	}

	return cmd, flags.Args(), nil
}

// Initialize finalises global option parsing and initializes the docker client.
func (tcmd *TopLevelCommand) Initialize(ops ...command.CLIOption) error {
	tcmd.opts.SetDefaultOptions(tcmd.flags)
	return tcmd.wpmCli.Initialize(tcmd.opts, ops...)
}
