// Command docgen renders reference documentation for the wpm CLI by
// walking the cobra command tree with docker/cli-docs-tool.
//
// It can emit three formats:
//
//   - md   — GitHub-flavored Markdown, committed to docs/reference/.
//     The source and target directories are the same: each per-command
//     file is rewritten in place, but only the section between
//     "<!---MARKER_GEN_START--->" and "<!---MARKER_GEN_END--->" is
//     touched. Any prose authored outside the markers (typically the
//     "## Description" and "## Examples" H2 sections) is preserved
//     across regenerations.
//
//   - man  — roff(7) man pages. Build artefact, NOT committed.
//
//   - yaml — structured per-command data, consumed by the docs site.
//     Each command becomes one YAML file with fields for short, long,
//     usage, options[], examples, cname[]/clink[], plus annotations
//     such as experimental, deprecated, hidden. Build artefact, NOT
//     committed. The docs site converts these to JSON at site-build
//     time before rendering with MDX.

package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/commands"
	"go.wpm.so/cli/cli/version"

	clidocstool "github.com/docker/cli-docs-tool"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

const (
	defaultMdDir   = "docs/reference"
	defaultManDir  = "man/man1"
	defaultYamlDir = "docs/yaml"
)

type options struct {
	format string
	source string
	target string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

func run() error {
	log.SetFlags(0)

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}

	root, err := newWpmRoot()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(opts.target, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", opts.target, err)
	}

	toolOpts := clidocstool.Options{
		Root:      root,
		SourceDir: opts.source,
		TargetDir: opts.target,
	}
	if opts.format == "man" {
		toolOpts.ManHeader = &doc.GenManHeader{
			Title:   "wpm",
			Section: "1",
			Source:  "wpm " + version.Version,
			Manual:  "wpm Manual",
		}
	}

	tool, err := clidocstool.New(toolOpts)
	if err != nil {
		return err
	}

	switch opts.format {
	case "md":
		return tool.GenMarkdownTree(root)
	case "man":
		return tool.GenManTree(root)
	case "yaml":
		return tool.GenYamlTree(root)
	default:
		return fmt.Errorf("unsupported format %q (want md, man, or yaml)", opts.format)
	}
}

func parseArgs(args []string) (options, error) {
	var opts options

	fs := pflag.NewFlagSet("docgen", pflag.ContinueOnError)
	fs.StringVar(&opts.format, "format", "md", "Output format: md, man, or yaml")
	fs.StringVar(&opts.source, "source", "", "Source directory (defaults to "+defaultMdDir+")")
	fs.StringVar(&opts.target, "target", "", "Target directory (defaults: "+defaultMdDir+" for md, "+defaultManDir+" for man, "+defaultYamlDir+" for yaml)")

	if err := fs.Parse(args); err != nil {
		return opts, err
	}

	if opts.source == "" {
		opts.source = defaultMdDir
	}
	if opts.target == "" {
		switch opts.format {
		case "md":
			opts.target = opts.source
		case "man":
			opts.target = defaultManDir
		case "yaml":
			opts.target = defaultYamlDir
		}
	}
	if opts.format == "" {
		return opts, errors.New("--format is required")
	}
	return opts, nil
}

// newWpmRoot creates the root cobra.Command for the wpm CLI, with all
// subcommands registered. This is used by docgen to generate documentation
// for the entire command tree.
func newWpmRoot() (*cobra.Command, error) {
	wpmCli, err := command.NewWpmCli()
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:                   "wpm [OPTIONS] COMMAND [ARG...]",
		Short:                 "Package Manager for WordPress ecosystem",
		DisableAutoGenTag:     true,
		DisableFlagsInUseLine: true,
	}

	cli.SetupRootCommand(cmd)
	commands.AddCommands(cmd, wpmCli)
	cli.DisableFlagsInUseLine(cmd)

	return cmd, nil
}
