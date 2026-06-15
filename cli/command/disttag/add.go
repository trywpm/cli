package disttag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli"
	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/completion"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
)

const defaultDistTag = "latest"

func newAddCommand(wpmCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add PACKAGE@VERSION [TAG]",
		Short: "Point a dist tag at a package version",
		Args:  cli.RequiresRangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd.Context(), wpmCli, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 1 {
				return completion.DistTags()(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	return cmd
}

func runAdd(ctx context.Context, wpmCli command.Cli, args []string) error {
	name, version, err := parsePackageVersion(args[0])
	if err != nil {
		return err
	}

	tag := defaultDistTag
	if len(args) == 2 {
		tag = args[1]
	}

	if err := validator.IsValidDistTag(tag); err != nil {
		return fmt.Errorf("invalid dist tag %q: %w", tag, err)
	}

	if err := validateAuth(wpmCli); err != nil {
		return err
	}

	client, err := wpmCli.RegistryClient()
	if err != nil {
		return err
	}

	if err := wpmCli.Progress().RunWithProgress(
		"",
		func() error { return client.AddDistTag(ctx, name, tag, version) },
		wpmCli.Err(),
	); err != nil {
		return err
	}

	wpmCli.Out().WriteString(fmt.Sprintf("+%s: %s@%s\n", tag, name, version))

	return nil
}

func validateAuth(wpmCli command.Cli) error {
	cfg := wpmCli.ConfigFile()
	if cfg.DefaultUser == "" || cfg.AuthToken == "" {
		return errors.New("user must be logged in to perform this action")
	}
	return nil
}

func parsePackageVersion(arg string) (name, version string, err error) {
	lastAt := strings.LastIndex(arg, "@")
	if lastAt <= 0 {
		return "", "", fmt.Errorf("invalid package spec %q: expected <pkg>@<version>", arg)
	}

	name = arg[:lastAt]
	version = arg[lastAt+1:]

	if err := validator.IsValidPackageName(name); err != nil {
		return "", "", fmt.Errorf("invalid package name %q: %w", name, err)
	}

	if err := validator.IsValidVersion(version); err != nil {
		return "", "", fmt.Errorf("invalid version %q: %w", version, err)
	}

	return name, version, nil
}
