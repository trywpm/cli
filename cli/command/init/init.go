package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"wpm/cli/command"
	"wpm/pkg/validator"

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

var packageType = []string{"plugin", "theme", "mu-plugin", "drop-in", "private"}

type initOptions struct {
	Type string
}

func NewInitCommand(wpmCli command.Cli) *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new WordPress package",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(wpmCli, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Type, "type", "t", defaultType, "Type of package (plugin, theme, mu-plugin, drop-in or private)")

	return cmd
}

func runInit(wpmCli command.Cli, opts initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wpmJsonPath := filepath.Join(cwd, wpmJsonFile)
	if _, err := os.Stat(wpmJsonPath); err == nil {
		return errors.Errorf("wpm.json already exists in %s", cwd)
	}

	if !slices.Contains(packageType, opts.Type) {
		return errors.Errorf("Invalid package type: %s.\nValid package types are: %s", opts.Type, strings.Join(packageType, ", "))
	}

	basecwd := filepath.Base(cwd)
	wpmJsonInitData := validator.Package{
		Name:        basecwd,
		Description: "",
		Type:        defaultType,
		Private:     false,
		Version:     defaultVersion,
		License:     defaultLicense,
		Homepage:    "https://wpm.so/packages/" + basecwd,
		Tags:        []string{},
		Team:        []string{},
		Bin:         map[string]string{},
		Platform: validator.PackagePlatform{
			PHP: defaultPHP,
			WP:  defaultWP,
		},
		Dependencies:    map[string]string{},
		DevDependencies: map[string]string{},
		Scripts:         map[string]string{},
	}

	if opts.Type == "private" {
		wpmJsonInitData.Type = ""
		wpmJsonInitData.Private = true
	} else if opts.Type != defaultType {
		wpmJsonInitData.Type = opts.Type
	}

	if err := writeWpmJson(wpmCli, wpmJsonPath, wpmJsonInitData); err != nil {
		return err
	}

	return nil
}

func writeWpmJson(wpmCli command.Cli, path string, data validator.Package) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "\t")

	if err := encoder.Encode(data); err != nil {
		return err
	}

	fmt.Fprint(wpmCli.Err(), "config created at ", path, "\n")

	return nil
}
