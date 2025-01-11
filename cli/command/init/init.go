package init

import (
	"context"
	"encoding/json"
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
	wpmJsonFile    = "wpm.json"
	defaultVersion = "1.0.0"
	defaultLicense = "GPL-2.0-or-later"
	defaultType    = "plugin"
	defaultPHP     = "7.2"
	defaultWP      = "6.7"
)

type packageInit struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	License  string   `json:"license"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags"`
	Platform struct {
		PHP string `json:"php"`
		WP  string `json:"wp"`
	} `json:"platform"`
}

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

	wpmJsonPath := filepath.Join(cwd, wpmJsonFile)
	if _, err := os.Stat(wpmJsonPath); err == nil {
		return errors.Errorf("wpm.json already exists in %s", cwd)
	}

	basecwd := filepath.Base(cwd)
	wpmJsonInitData := packageInit{
		Name:    basecwd,
		Version: defaultVersion,
		License: defaultLicense,
		Type:    defaultType,
		Tags:    []string{},
		Platform: struct {
			PHP string `json:"php"`
			WP  string `json:"wp"`
		}{
			PHP: defaultPHP,
			WP:  defaultWP,
		},
	}
	wpm, err := wpm.NewWpm(false)
	if err != nil {
		return err
	}

	ve := wpm.Validator()

	// If not auto-confirmed, prompt the user for values
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

						errs := ve.Var(val, "required,semver,max=64")
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
			{
				"php",
				prompt{
					"requires php",
					defaultPHP,
					func(val string) error {
						if val == "" {
							val = defaultPHP
						}

						var semverVal string

						semverVal, err = formatSemver(val)
						if err != nil {
							return errors.Errorf("invalid php version: \"%s\"", aec.Bold.Apply(val))
						}

						errs := ve.Var(semverVal, "required,semver")
						if errs != nil {
							return errors.Errorf("invalid php version: \"%s\"", aec.Bold.Apply(semverVal))
						}

						wpmJsonInitData.Platform.PHP = val

						return nil
					},
				},
			},
			{
				"wp",
				prompt{
					"requires wp",
					defaultWP,
					func(val string) error {
						if val == "" {
							val = defaultWP
						}

						var semverVal string

						semverVal, err = formatSemver(val)
						if err != nil {
							return errors.Errorf("invalid wp version: \"%s\"", aec.Bold.Apply(val))
						}

						errs := ve.Var(semverVal, "required,semver")
						if errs != nil {
							return errors.Errorf("invalid wp version: \"%s\"", aec.Bold.Apply(semverVal))
						}

						wpmJsonInitData.Platform.WP = val

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

	if err := writeWpmJson(wpmCli, wpmJsonPath, wpmJsonInitData); err != nil {
		return err
	}

	return nil
}

func writeWpmJson(wpmCli command.Cli, path string, data packageInit) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return err
	}

	fmt.Fprint(wpmCli.Out(), "config created at ", path, "\n")

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
