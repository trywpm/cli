package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"wpm/cli/command"
	"wpm/pkg/wp/parser"
	"wpm/pkg/wpm"

	"github.com/Masterminds/semver/v3"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	defaultVersion = "1.0.0"
	defaultType    = "plugin"
	defaultLicense = "GPL-2.0-or-later"
)

var findLetters = regexp.MustCompile(`\b[a-zA-Z]{2,}\b`)

type initOptions struct {
	yes         bool
	name        string
	version     string
	migrate     bool
	license     string
	packageType string
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
		Short: "Initialize a new WordPress package or migrate an existing one",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.migrate {
				return runMigrate(cmd.Context(), wpmCli, &opts)
			}
			return runInit(cmd.Context(), wpmCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.yes, "yes", "y", false, "Skip prompts and use default values")
	flags.StringVar(&opts.name, "name", "", "Name of the package (for -y, or --migrate)")
	flags.BoolVarP(&opts.migrate, "migrate", "m", false, "Migrate existing theme or plugin to wpm.json format")
	flags.StringVar(&opts.license, "license", defaultLicense, "License of the package (default: GPL-2.0-or-later)")
	flags.StringVar(&opts.version, "version", "", "Version of the package (default: 1.0.0 for init; required for --migrate)")
	flags.StringVar(&opts.packageType, "type", "", "Type of the package (plugin, theme, mu-plugin). Auto-detected for --migrate if not set.")

	return cmd
}

func runInit(ctx context.Context, wpmCli command.Cli, opts initOptions) error {
	cwd := wpmCli.Options().Cwd

	wpmConfigFilePath := filepath.Join(cwd, wpm.ConfigFile)
	if _, err := os.Stat(wpmConfigFilePath); err == nil {
		return errors.Errorf("%s already exists in %s", wpm.ConfigFile, cwd)
	} else if !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to check for existing %s", wpm.ConfigFile)
	}

	basecwd := filepath.Base(cwd)
	wpmJsonInitData := &wpm.Config{
		Name:    basecwd,
		Version: defaultVersion,
		License: defaultLicense,
		Type:    defaultType,
		Tags:    []string{},
	}

	ve, err := wpm.NewValidator()
	if err != nil {
		return errors.Wrap(err, "failed to initialize validator")
	}

	if opts.yes {
		if opts.name == "" {
			opts.name = basecwd
		}
		if errs := ve.Var(opts.name, "required,min=3,max=164,package_name_regex"); errs != nil {
			return errors.Errorf("invalid package name: \"%s\"", aec.Bold.Apply(opts.name))
		}

		if opts.version == "" {
			opts.version = defaultVersion
		}
		if errs := ve.Var(opts.version, "required,package_semver,max=64"); errs != nil {
			return errors.Errorf("invalid version: \"%s\"", aec.Bold.Apply(opts.version))
		}

		if opts.packageType == "" {
			opts.packageType = defaultType
		}
		if errs := ve.Var(opts.packageType, "required,oneof=plugin theme mu-plugin drop-in"); errs != nil {
			return errors.Errorf("invalid type: \"%s\"", aec.Bold.Apply(opts.packageType))
		}

		if opts.license == "" {
			opts.license = defaultLicense
		}
		if errs := ve.Var(opts.license, "required,min=1,max=128"); errs != nil {
			return errors.Errorf("invalid license: \"%s\"", aec.Bold.Apply(opts.license))
		}

		wpmJsonInitData.Name = opts.name
		wpmJsonInitData.License = opts.license
		wpmJsonInitData.Version = opts.version
		wpmJsonInitData.Type = opts.packageType
	} else {
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
					return errors.Wrap(err, "failed to get prompt input")
				}

				if err := pf.Prompt.Validate(val); err != nil {
					fmt.Fprintf(wpmCli.Err(), "%s\n", err)
					continue
				}

				break
			}
		}
	}

	if err := wpm.WriteWpmJson(wpmJsonInitData, cwd); err != nil {
		return errors.Wrap(err, "failed to write wpm.json")
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "Config created at %s\n", wpmConfigFilePath)

	return nil
}

type migrationBaseFiles struct {
	readmeTxt string
	readmeMd  string
	wpmJson   string
}

func findMigrationBaseFiles(cwd string) (migrationBaseFiles, error) {
	var paths migrationBaseFiles
	files, err := os.ReadDir(cwd)
	if err != nil {
		return paths, errors.Wrap(err, "failed to read current directory")
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		lowerName := strings.ToLower(file.Name())
		fullPath := filepath.Join(cwd, file.Name())

		switch lowerName {
		case "readme.txt":
			paths.readmeTxt = fullPath
		case "readme.md":
			paths.readmeMd = fullPath
		case strings.ToLower(wpm.ConfigFile):
			paths.wpmJson = fullPath
		}
	}

	return paths, nil
}

func generateReadme(wpmCli command.Cli, readmeTxtPath, readmeMdPath string) (*parser.ReadmeParser, error) {
	if readmeTxtPath == "" {
		return &parser.ReadmeParser{}, nil
	}

	readmeTxtContent, err := os.ReadFile(readmeTxtPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read readme.txt")
	}

	p := parser.NewReadmeParser()
	p.Parse(string(readmeTxtContent))

	if readmeMdPath == "" {
		markdownContent := p.ToMarkdown()
		readmeMdPath := strings.TrimSuffix(readmeTxtPath, filepath.Ext(readmeTxtPath)) + ".md"
		if err := os.WriteFile(readmeMdPath, []byte(markdownContent), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write %s", readmeMdPath)
		}

		_, _ = fmt.Fprintf(wpmCli.Out(), "%s created from readme.txt\n", filepath.Base(readmeMdPath))
	}
	return p, nil
}

func findMainPluginFile(cwd string, targetVersion string, files []os.DirEntry) (string, parser.PluginFileHeaders, error) {
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".php") {
			continue
		}

		filePath := filepath.Join(cwd, file.Name())
		headers, err := parser.GetPluginHeaders(filePath)
		if err != nil {
			continue
		}

		if headers.Version == targetVersion {
			return filePath, headers, nil
		}
	}

	return "", parser.PluginFileHeaders{}, errors.New("no main plugin file with valid plugin headers found")
}

func getMetaString(meta map[string]interface{}, key string, defaultValue string) string {
	if val, ok := meta[key]; ok {
		if strVal, ok := val.(string); ok && strVal != "" {
			return strVal
		}
	}
	return defaultValue
}

func getMetaStringSlice(meta map[string]interface{}, key string) []string {
	if val, ok := meta[key]; ok {
		if sliceVal, ok := val.([]string); ok {
			return sliceVal
		}
	}
	return []string{}
}

func isMeaningfulText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	allSymbols := true
	for _, r := range s {
		if unicode.IsLetter(r) {
			allSymbols = false
			break
		}
	}
	if allSymbols {
		return false
	}

	// Find at least one word with 2 or more letters
	words := findLetters.FindAllString(s, -1)

	return len(words) > 0
}

func buildWPMConfig(
	opts initOptions,
	pkgType string,
	mainFileHeaders interface{},
	readmeMeta map[string]interface{},
) *wpm.Config {
	ve, err := wpm.NewValidator()
	if err != nil {
		return &wpm.Config{}
	}

	cfg := &wpm.Config{
		Name:        opts.name,
		Version:     opts.version,
		Type:        pkgType,
		License:     getMetaString(readmeMeta, "license", ""),
		Description: getMetaString(readmeMeta, "meta_description", ""),
	}

	tags := getMetaStringSlice(readmeMeta, "tags")
	if len(tags) > 5 {
		cfg.Tags = tags[:5]
	} else {
		cfg.Tags = tags
	}

	cfg.Team = getMetaStringSlice(readmeMeta, "contributors")

	dependencies := make(map[string]string)
	wpRequires := getMetaString(readmeMeta, "requires", "")
	phpRequires := getMetaString(readmeMeta, "requires_php", "")

	switch h := mainFileHeaders.(type) {
	case parser.ThemeFileHeaders:
		if cfg.License == "" {
			cfg.License = h.License
		}

		if cfg.Description == "" || !isMeaningfulText(cfg.Description) {
			cfg.Description = h.Description
		}

		if len(cfg.Team) == 0 && h.Author != "" {
			cfg.Team = []string{h.Author}
		}

		if len(tags) == 0 && len(h.Tags) > 0 {
			if len(h.Tags) > 5 {
				cfg.Tags = h.Tags[:5]
			} else {
				cfg.Tags = h.Tags
			}
		}

		if h.ThemeURI != "" {
			if err := ve.Var(h.ThemeURI, "http_url"); err == nil {
				cfg.Homepage = h.ThemeURI
			}
		}

		if wpRequires == "" && h.RequiresWP != "" {
			wpRequires = h.RequiresWP
		}

		if phpRequires == "" && h.RequiresPHP != "" {
			phpRequires = h.RequiresPHP
		}
	case parser.PluginFileHeaders:
		if cfg.License == "" {
			cfg.License = h.License
		}

		if cfg.Description == "" || !isMeaningfulText(cfg.Description) {
			cfg.Description = h.Description
		}

		if len(cfg.Team) == 0 && h.Author != "" {
			cfg.Team = []string{h.Author}
		}

		if len(tags) == 0 && len(h.Tags) > 0 {
			if len(h.Tags) > 5 {
				cfg.Tags = h.Tags[:5]
			} else {
				cfg.Tags = h.Tags
			}
		}

		if h.PluginURI != "" {
			if err := ve.Var(h.PluginURI, "http_url"); err == nil {
				cfg.Homepage = h.PluginURI
			}
		}

		if wpRequires == "" && h.RequiresWP != "" {
			wpRequires = h.RequiresWP
		}

		if phpRequires == "" && h.RequiresPHP != "" {
			phpRequires = h.RequiresPHP
		}

		if len(h.RequiresPlugins) > 0 {
			for _, reqPlugin := range h.RequiresPlugins {
				// Add "*" as version since requires plugins only specify the plugin slug, not a version.
				dependencies[reqPlugin] = "*"
			}
		}
	}

	if wpRequires != "" {
		v, err := semver.NewVersion(wpRequires)
		if err == nil {
			dependencies["wp"] = v.String()
		}
	}
	if phpRequires != "" {
		v, err := semver.NewVersion(phpRequires)
		if err == nil {
			dependencies["php"] = v.String()
		}
	}
	if len(dependencies) > 0 {
		cfg.Dependencies = dependencies
	}

	return cfg
}

func normalizeVersion(version string) (string, error) {
	if version == "" {
		return "", errors.New("version cannot be empty")
	}

	// Attempt to normalize the version format to be compatible with semver.
	// If version has more than 2 dots, we replace the last dot with a hyphen
	// Example:
	// 1.0.0.0 -> 1.0.0-0
	// 1.0.0.alpha.1+build -> 1.0.0-alpha.1+build
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		major := parts[0]
		minor := parts[1]
		patch := parts[2]
		prerelease := strings.Join(parts[3:], ".")

		version = fmt.Sprintf("%s.%s.%s-%s", major, minor, patch, prerelease)
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return "", errors.Wrapf(err, "invalid version format: %s", version)
	}

	return v.String(), nil
}

func runMigrationProcess(wpmCli command.Cli, opts initOptions, cwd string) error {
	baseFiles, err := findMigrationBaseFiles(cwd)
	if err != nil {
		return err
	}

	if baseFiles.wpmJson != "" {
		_, _ = fmt.Fprintf(wpmCli.Out(), "%s already exists at %s, skipping migration.\n", wpm.ConfigFile, baseFiles.wpmJson)
		return nil
	}

	readmeParser, err := generateReadme(wpmCli, baseFiles.readmeTxt, baseFiles.readmeMd)
	if err != nil {
		return err
	}

	readmeMetadata := readmeParser.GetMetadata()

	var mainFilePath string
	var mainFileHeaders interface{}
	var mainFileNameForMsg string // style.css for themes, main plugin file for plugins

	switch opts.packageType {
	case "theme":
		mainFileNameForMsg = "style.css"
		mainFilePath = filepath.Join(cwd, mainFileNameForMsg)
		if _, statErr := os.Stat(mainFilePath); statErr != nil {
			if os.IsNotExist(statErr) {
				return errors.Errorf("%s not found in %s", mainFileNameForMsg, cwd)
			}
			return errors.Wrapf(statErr, "failed to stat %s", mainFileNameForMsg)
		}

		headers, parseErr := parser.GetThemeHeaders(mainFilePath)
		if parseErr != nil {
			return errors.Wrapf(parseErr, "failed to parse theme headers from %s", mainFileNameForMsg)
		}
		if headers.Version != opts.version {
			return errors.Errorf("version in %s (%s) does not match specified version %s", mainFileNameForMsg, headers.Version, opts.version)
		}

		mainFileHeaders = headers
	case "plugin":
		dirEntries, readDirErr := os.ReadDir(cwd)
		if readDirErr != nil {
			return errors.Wrap(readDirErr, "failed to read current directory for plugin files")
		}

		foundPath, parsedHeaders, findErr := findMainPluginFile(cwd, opts.version, dirEntries)
		if findErr != nil {
			return errors.Wrap(findErr, "failed to identify main plugin file")
		}

		mainFilePath = foundPath
		mainFileNameForMsg = filepath.Base(mainFilePath)
		if parsedHeaders.Version != opts.version { // findMainPluginFile might return a non-matching version if no exact match found
			return errors.Errorf("version in %s (%s) does not match specified version --version %s", mainFileNameForMsg, parsedHeaders.Version, opts.version)
		}

		mainFileHeaders = parsedHeaders
	default:
		return errors.Errorf("unsupported package type for migration: %s", opts.packageType)
	}

	wpmJsonData := buildWPMConfig(opts, opts.packageType, mainFileHeaders, readmeMetadata)

	if wpmJsonData.Name == "" || wpmJsonData.Version == "" || wpmJsonData.Type == "" {
		return errors.New("failed to build wpm.json data; name, version, and type are required")
	}

	v, err := normalizeVersion(wpmJsonData.Version)
	if err != nil {
		return errors.Wrapf(err, "invalid version %s; must be a valid semantic version", wpmJsonData.Version)
	}

	wpmJsonData.Version = v

	if err := wpm.WriteWpmJson(wpmJsonData, cwd); err != nil {
		return errors.Wrapf(err, "failed to write %s", wpm.ConfigFile)
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "%s created at %s\n", wpm.ConfigFile, filepath.Join(cwd, wpm.ConfigFile))

	return nil
}

func getMigrateType(cwd string) string {
	if _, err := os.Stat(filepath.Join(cwd, "style.css")); err == nil {
		return "theme"
	}

	return "plugin"
}

func runMigrate(ctx context.Context, wpmCli command.Cli, opts *initOptions) error {
	cwd := wpmCli.Options().Cwd

	if opts.name == "" {
		return errors.Errorf("package --name is required for migration (e.g., --name=amp)")
	}

	if opts.version == "" {
		return errors.New("package --version is required for migration (e.g., --version=1.0.0)")
	}

	if opts.packageType == "" {
		opts.packageType = getMigrateType(cwd)

		_, _ = fmt.Fprintf(wpmCli.Out(), "auto-detected package type: %s\n", opts.packageType)
	}

	if opts.packageType != "theme" && opts.packageType != "plugin" {
		return errors.Errorf("invalid package type '%s' specified for migration; must be 'theme' or 'plugin'", opts.packageType)
	}

	ve, err := wpm.NewValidator()
	if err != nil {
		return errors.Wrap(err, "failed to initialize validator for migration")
	}
	if errs := ve.Var(opts.name, "required,min=3,max=164,package_name_regex"); errs != nil {
		return errors.Errorf("invalid --name for migration: \"%s\"", aec.Bold.Apply(opts.name))
	}
	if errs := ve.Var(opts.version, "required,max=64"); errs != nil {
		return errors.Errorf("invalid --version for migration: \"%s\"", aec.Bold.Apply(opts.version))
	}

	return runMigrationProcess(wpmCli, *opts, cwd)
}
