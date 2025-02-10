package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wpm/cli/command"
	"wpm/pkg/wpm"

	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	defaultVersion = "1.0.0"
	defaultLicense = "GPL-2.0-or-later"
	defaultType    = "plugin"
	defaultPHP     = "7.2"
	defaultWP      = "6.7"
)

type initOptions struct {
	yes bool
}

type prompt struct {
	Msg      string
	Default  string
	Validate func(string) error
}

type promptField struct {
	Key    string
	Prompt prompt
}

func NewInitCommand(wpmCli command.Cli) *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new WordPress package",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), wpmCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.yes, "yes", "y", false, "Skip prompts and use default values")

	return cmd
}

func runInit(ctx context.Context, wpmCli command.Cli, opts initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if _, err = os.Stat(filepath.Join(cwd, wpm.Config)); err == nil {
		return errors.New("wpm.json already exists")
	}

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	basecwd := filepath.Base(cwd)
	wpmJsonInitData := &wpm.Json{
		Name:    basecwd,
		Version: defaultVersion,
		License: defaultLicense,
		Type:    defaultType,
		Tags:    []string{},
	}

	ve, err := wpm.NewValidator()
	if err != nil {
		return err
	}

	if !opts.yes {
		prompts := []promptField{
			{
				"name",
				prompt{
					"package name",
					basecwd,
					func(val string) error {
						if val == "" {
							val = basecwd
						}

						errs := ve.Var(val, "required,min=3,max=164,package_name_regex")
						if errs != nil {
							return errors.Errorf("invalid package name: \"%s\"", aec.Bold.Apply(val))
						}

						wpmJsonInitData.Name = val

						return nil
					},
				},
			},
			{
				"version",
				prompt{
					"version",
					defaultVersion,
					func(val string) error {
						if val == "" {
							val = defaultVersion
						}

						errs := ve.Var(val, "required,package_semver,max=64")
						if errs != nil {
							return errors.Errorf("invalid version: \"%s\"", aec.Bold.Apply(val))
						}

						wpmJsonInitData.Version = val

						return nil
					},
				},
			},
			{
				"license",
				prompt{
					"license",
					defaultLicense,
					func(val string) error {
						if val == "" {
							val = defaultLicense
						}

						wpmJsonInitData.License = val

						return nil
					},
				},
			},
			{
				"type",
				prompt{
					"type",
					defaultType,
					func(val string) error {
						if val == "" {
							val = defaultType
						}

						errs := ve.Var(val, "required,oneof=plugin theme mu-plugin drop-in")
						if errs != nil {
							return errors.Errorf("invalid type: \"%s\"", aec.Bold.Apply(val))
						}

						wpmJsonInitData.Type = val

						return nil
					},
				},
			},
		}

		for _, pf := range prompts {
			var val string
			var err error

			for {
				val, err = command.PromptForInput(ctx, wpmCli.In(), wpmCli.Out(), fmt.Sprintf("%s (%s): ", pf.Prompt.Msg, pf.Prompt.Default))
				if err != nil {
					return err
				}

				if err := pf.Prompt.Validate(val); err != nil {
					fmt.Fprintf(wpmCli.Err(), "%s\n", err)
					continue
				}

				break
			}
		}
	}

	if err := wpm.WriteWpmJson(wpmJsonInitData, ""); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "config created at %s\n", filepath.Join(cwd, wpm.Config))

	return nil
}

func formatSemver(version string) (string, error) {
	parts := strings.Split(version, ".")

	for _, part := range parts {
		if part == "" {
			return "", errors.New("empty part")
		}

		if _, err := fmt.Sscanf(part, "%d", new(int)); err != nil {
			return "", err
		}
	}

	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	return strings.Join(parts, "."), nil
}
