package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"wpm/cli/command"

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

type initOptions struct {
	yes bool
}

type PackageInfo struct {
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

type prompt struct {
	Msg     string
	Default string
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
	wpmJsonInitData := PackageInfo{
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

	// If not auto-confirmed, prompt the user for values
	if !opts.yes {
		prompts := []promptField{
			{"name", prompt{"package name", basecwd}},
			{"version", prompt{"version", defaultVersion}},
			{"license", prompt{"license", defaultLicense}},
			{"type", prompt{"type", defaultType}},
			{"php", prompt{"requires php", defaultPHP}},
			{"wp", prompt{"requires wp", defaultWP}},
		}

		for _, pf := range prompts {
			val, err := command.PromptForInput(ctx, wpmCli.In(), wpmCli.Out(), fmt.Sprintf("%s (%s): ", pf.Prompt.Msg, pf.Prompt.Default))
			if err != nil {
				return err
			}
			if val == "" {
				val = pf.Prompt.Default
			}

			switch pf.Key {
			case "name":
				wpmJsonInitData.Name = val
			case "version":
				wpmJsonInitData.Version = val
			case "license":
				wpmJsonInitData.License = val
			case "type":
				wpmJsonInitData.Type = val
			case "php":
				wpmJsonInitData.Platform.PHP = val
			case "wp":
				wpmJsonInitData.Platform.WP = val
			}
		}
	}

	if err := writeWpmJson(wpmJsonPath, wpmJsonInitData); err != nil {
		return err
	}

	return nil
}

func writeWpmJson(path string, data PackageInfo) error {
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

	return nil
}
