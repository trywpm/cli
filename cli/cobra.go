package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"wpm/cli/command"
	cliflags "wpm/cli/flags"

	"github.com/fvbommel/sortorder"
	"github.com/moby/term"
	"github.com/morikuni/aec"
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
	cobra.AddTemplateFunc("hasAliases", hasAliases)
	cobra.AddTemplateFunc("hasSubCommands", hasSubCommands)
	cobra.AddTemplateFunc("hasTopCommands", hasTopCommands)
	cobra.AddTemplateFunc("topCommands", topCommands)
	cobra.AddTemplateFunc("commandAliases", commandAliases)
	cobra.AddTemplateFunc("operationSubCommands", operationSubCommands)
	cobra.AddTemplateFunc("hasAdditionalHelp", hasAdditionalHelp)
	cobra.AddTemplateFunc("wrappedFlagUsages", wrappedFlagUsages)
	cobra.AddTemplateFunc("additionalHelp", additionalHelp)
	cobra.AddTemplateFunc("decoratedName", decoratedName)

	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetFlagErrorFunc(FlagErrorFunc)
	rootCmd.SetHelpCommand(helpCommand)

	rootCmd.PersistentFlags().BoolP("help", "h", false, "Show help for command")
	rootCmd.PersistentFlags().Lookup("help").Hidden = true

	rootCmd.Annotations = map[string]string{
		"additionalHelp": "For more help on how to use wpm, see https://docs.wpm.so",
	}

	return opts, helpCommand
}

// SetupRootCommand sets default usage, help, and error handling for the
// root command.
func SetupRootCommand(rootCmd *cobra.Command) (opts *cliflags.ClientOptions, helpCmd *cobra.Command) {
	rootCmd.SetVersionTemplate("wpm version {{.Version}}\n")
	return setupCommonRootCommand(rootCmd)
}

// FlagErrorFunc prints an error message which matches the format of the
// wpm/cli/cli error messages
func FlagErrorFunc(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	return StatusError{
		Status:     fmt.Sprintf("%s\n\nUsage:  %s\n\nRun '%s --help' for more information", err, cmd.UseLine(), cmd.CommandPath()),
		StatusCode: 125,
	}
}

func operationSubCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if _, ok := sub.Annotations["category-top"]; ok {
			if cmd.Parent() == nil {
				// for now, only use top-commands for the root-command
				continue
			}
		}
		if sub.IsAvailableCommand() && !sub.HasSubCommands() {
			cmds = append(cmds, sub)
		}
	}
	return cmds
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

func hasAliases(cmd *cobra.Command) bool {
	return len(cmd.Aliases) > 0 || cmd.Annotations["aliases"] != ""
}

func hasSubCommands(cmd *cobra.Command) bool {
	return len(operationSubCommands(cmd)) > 0
}

func hasTopCommands(cmd *cobra.Command) bool {
	return len(topCommands(cmd)) > 0
}

func topCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	if cmd.Parent() != nil {
		// for now, only use top-commands for the root-command, and skip
		// for sub-commands
		return cmds
	}
	for _, sub := range cmd.Commands() {
		if _, ok := sub.Annotations["category-top"]; ok {
			cmds = append(cmds, sub)
		}
	}
	sort.SliceStable(cmds, func(i, j int) bool {
		return sortorder.NaturalLess(cmds[i].Annotations["category-top"], cmds[j].Annotations["category-top"])
	})
	return cmds
}

// commandAliases is a templating function to return aliases for the command,
// formatted as the full command as they're called (contrary to the default
// Aliases function, which only returns the subcommand).
func commandAliases(cmd *cobra.Command) string {
	if cmd.Annotations["aliases"] != "" {
		return cmd.Annotations["aliases"]
	}
	var parentPath string
	if cmd.HasParent() {
		parentPath = cmd.Parent().CommandPath() + " "
	}
	aliases := cmd.CommandPath()
	for _, alias := range cmd.Aliases {
		aliases += ", " + parentPath + alias
	}
	return aliases
}

func wrappedFlagUsages(cmd *cobra.Command) string {
	width := 80
	if ws, err := term.GetWinsize(0); err == nil {
		width = int(ws.Width)
	}
	return cmd.Flags().FlagUsagesWrapped(width - 1)
}

func additionalHelp(cmd *cobra.Command) string {
	if msg, ok := cmd.Annotations["additionalHelp"]; ok {
		out := cmd.OutOrStderr()
		if _, isTerminal := term.GetFdInfo(out); !isTerminal {
			return msg
		}
		style := aec.EmptyBuilder.Bold().ANSI
		return style.Apply(msg)
	}
	return ""
}

func hasAdditionalHelp(cmd *cobra.Command) bool {
	return additionalHelp(cmd) != ""
}

func decoratedName(cmd *cobra.Command) string {
	return cmd.Name() + " "
}

// TopLevelCommand encapsulates a top-level cobra command and global flag handling logic necessary
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

// Initialize finalises global option parsing and initializes the wpm client.
func (tcmd *TopLevelCommand) Initialize(ops ...command.CLIOption) error {
	tcmd.opts.SetDefaultOptions(tcmd.flags)
	return tcmd.wpmCli.Initialize(tcmd.opts, ops...)
}

const usageTemplate = `Usage:
{{- if not .HasSubCommands}} {{.UseLine}}{{end}}
{{- if .HasSubCommands}} {{ .CommandPath}}{{- if .HasAvailableFlags}} [OPTIONS]{{end}} COMMAND{{end}}

{{if ne .Long ""}}{{ .Long | trim }}{{ else }}{{ .Short | trim }}{{end}}
{{- if hasAliases . }}

Aliases:
  {{ commandAliases . }}

{{- end}}

{{- if .HasExample}}

Examples:
{{ .Example }}

{{- end}}

{{- if .HasParent}}
{{- if .HasAvailableFlags}}

Options:
{{ wrappedFlagUsages . | trimRightSpace}}

{{- end}}
{{- end}}

{{- if hasTopCommands .}}

Common Commands:
{{- range topCommands .}}
  {{rpad (decoratedName .) (add .NamePadding 1)}}{{.Short}}
{{- end}}
{{- end}}

{{- if hasSubCommands .}}

Commands:

{{- range operationSubCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}
{{- end}}

{{- if not .HasParent}}
{{- if .HasAvailableFlags}}

Global Options:
{{ wrappedFlagUsages . | trimRightSpace}}

{{- end}}
{{- end}}

{{- if .HasSubCommands }}

Run '{{.CommandPath}} COMMAND --help' for more information on a command.
{{- end}}

{{- if hasAdditionalHelp .}}

{{ additionalHelp . }}

{{- end}}
`

const helpTemplate = `
{{- if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
