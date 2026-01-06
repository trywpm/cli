package init

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"wpm/cli/command"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/wp/parser"

	"github.com/Masterminds/semver/v3"
	"github.com/go-playground/validator/v10"
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
	existing    bool
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

type existingProjectFiles struct {
	readmeTxt string
	readmeMd  string
	wpmJson   string
}

func NewInitCommand(wpmCli command.Cli) *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new WordPress package or init wpm in existing project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.existing {
				return runExistingInit(wpmCli, &opts)
			}
			return runNewInit(cmd.Context(), wpmCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.yes, "yes", "y", false, "Skip prompts and use default values")
	flags.BoolVar(&opts.existing, "existing", false, "Init wpm.json for an existing project")
	flags.StringVar(&opts.name, "name", "", "Package name")
	flags.StringVar(&opts.version, "version", "", "Semver-compliant version")
	flags.StringVar(&opts.license, "license", "", "Package license")
	flags.StringVar(&opts.packageType, "type", "", "Package type (plugin, theme, mu-plugin)")

	return cmd
}

func runNewInit(ctx context.Context, wpmCli command.Cli, opts *initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	wpmConfigFilePath := filepath.Join(cwd, wpmjson.ConfigFile)
	if _, err := os.Stat(wpmConfigFilePath); err == nil {
		return errors.Errorf("%s already exists in %s", wpmjson.ConfigFile, cwd)
	} else if !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to check for existing %s", wpmjson.ConfigFile)
	}

	defaultName := filepath.Base(cwd)
	wpmJsonInitData := wpmjson.NewConfig()

	ve, err := wpmjson.NewValidator()
	if err != nil {
		return errors.Wrap(err, "failed to initialize validator")
	}

	if opts.yes {
		if err := setDefaults(opts, defaultName); err != nil {
			return err
		}

		if err := validateOptions(opts, ve); err != nil {
			return err
		}

		setConfigFromOptions(wpmJsonInitData, opts)
	} else {
		if err := promptForConfig(ctx, wpmCli, wpmJsonInitData, ve, defaultName); err != nil {
			return err
		}
	}

	if err := wpmjson.WriteWpmJson(wpmJsonInitData, cwd); err != nil {
		return errors.Wrap(err, "failed to write wpm.json")
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "config created at %s\n", wpmConfigFilePath)

	return nil
}

func runExistingInit(wpmCli command.Cli, opts *initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get current working directory")
	}

	baseFiles, err := findExistingProjectFiles(cwd)
	if err != nil {
		return err
	}

	if baseFiles.wpmJson != "" {
		_, _ = fmt.Fprintf(wpmCli.Out(), "%s already exists at %s, skipping init.\n", wpmjson.ConfigFile, baseFiles.wpmJson)
		return nil
	}

	if opts.packageType == "" {
		opts.packageType = detectPackageType(cwd)
		_, _ = fmt.Fprintf(wpmCli.Out(), "using package type: %s\n", opts.packageType)
	}

	// Parse readme.txt if it exists
	readmeParser := &parser.ReadmeParser{}
	if baseFiles.readmeTxt != "" {
		readmeTxtContent, err := os.ReadFile(baseFiles.readmeTxt)
		if err != nil {
			return errors.Wrap(err, "failed to read readme.txt")
		}
		readmeParser = parser.NewReadmeParser()
		readmeParser.Parse(string(readmeTxtContent))
	}

	// Extract package info based on type
	var extractedVersion string
	var mainFileHeaders any

	switch opts.packageType {
	case "theme":
		mainFilePath := filepath.Join(cwd, "style.css")
		if _, err := os.Stat(mainFilePath); err != nil {
			if os.IsNotExist(err) && opts.version == "" {
				return errors.Errorf("style.css not found in %s", cwd)
			}
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "failed to stat style.css")
			}
		} else {
			headers, err := parser.GetThemeHeaders(mainFilePath)
			if err != nil {
				if opts.version == "" {
					return errors.Wrapf(err, "failed to parse theme headers from style.css")
				}
			} else {
				mainFileHeaders = headers
				extractedVersion = headers.Version
			}
		}

	case "plugin":
		dirEntries, err := os.ReadDir(cwd)
		if err != nil {
			return errors.Wrap(err, "failed to read current directory for plugin files")
		}

		foundPath, headers, err := findMainPluginFile(cwd, dirEntries)
		if err != nil {
			if opts.version == "" {
				return errors.Wrap(err, "failed to identify main plugin file")
			}
		} else {
			_, _ = fmt.Fprintf(wpmCli.Out(), "main plugin file found: %s\n", foundPath)
			mainFileHeaders = headers
			extractedVersion = headers.Version
		}

	default:
		return errors.Errorf("unsupported package type for existing project init: %s", opts.packageType)
	}

	// Build wpm.json config
	wpmJsonData := buildWPMConfig(*opts, opts.packageType, mainFileHeaders, readmeParser.GetMetadata())

	// Set name
	if opts.name != "" {
		wpmJsonData.Name = opts.name
	} else {
		wpmJsonData.Name = filepath.Base(cwd)
	}

	if opts.version != "" {
		wpmJsonData.Version = opts.version

		if extractedVersion != "" && extractedVersion != opts.version {
			_, _ = fmt.Fprintf(wpmCli.Err(), aec.YellowF.Apply("provided version (%s) differs from version in headers (%s)\n"), opts.version, extractedVersion)
		}
	} else {
		if extractedVersion == "" {
			return errors.New("unable to determine version; please specify it with --version")
		}
		wpmJsonData.Version = extractedVersion
	}

	// Normalize and validate version
	v, err := normalizeVersion(wpmJsonData.Version)
	if err != nil {
		return errors.New("invalid version format: " + err.Error())
	}
	wpmJsonData.Version = v

	ve, err := wpmjson.NewValidator()
	if err != nil {
		return errors.Wrap(err, "failed to initialize validator")
	}

	if err = wpmjson.ValidateWpmJson(ve, wpmJsonData); err != nil {
		return err
	}

	// create wpm.json file
	if err := wpmjson.WriteWpmJson(wpmJsonData, cwd); err != nil {
		return errors.Wrapf(err, "failed to write %s", wpmjson.ConfigFile)
	}

	// create readme.md from readme.txt if it exists
	if baseFiles.readmeTxt != "" && baseFiles.readmeMd == "" {
		markdownContent := readmeParser.ToMarkdown()
		readmeMdPath := strings.TrimSuffix(baseFiles.readmeTxt, filepath.Ext(baseFiles.readmeTxt)) + ".md"
		if err := os.WriteFile(readmeMdPath, []byte(markdownContent), 0644); err != nil {
			_, _ = fmt.Fprintf(wpmCli.Err(), "failed to write %s: %v\n", readmeMdPath, err)
		} else {
			_, _ = fmt.Fprintf(wpmCli.Out(), "%s created from readme.txt\n", filepath.Base(readmeMdPath))
		}
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "%s created at %s\n", wpmjson.ConfigFile, filepath.Join(cwd, wpmjson.ConfigFile))

	return nil
}

func setDefaults(opts *initOptions, defaultName string) error {
	if opts.name == "" {
		opts.name = defaultName
	}
	if opts.version == "" {
		opts.version = defaultVersion
	}
	if opts.packageType == "" {
		opts.packageType = defaultType
	}
	if opts.license == "" {
		opts.license = defaultLicense
	}
	return nil
}

func validateOptions(opts *initOptions, ve *validator.Validate) error {
	if errs := ve.Var(opts.name, "required,wpm_name"); errs != nil {
		return errors.Errorf("invalid package name: \"%s\"", aec.Bold.Apply(opts.name))
	}
	if errs := ve.Var(opts.version, "required,wpm_semver"); errs != nil {
		return errors.Errorf("invalid version: \"%s\"", aec.Bold.Apply(opts.version))
	}
	if errs := ve.Var(opts.packageType, "required,oneof=plugin theme mu-plugin"); errs != nil {
		return errors.Errorf("invalid type: \"%s\"", aec.Bold.Apply(opts.packageType))
	}
	if errs := ve.Var(opts.license, "required,min=3,max=100"); errs != nil {
		return errors.Errorf("invalid license: \"%s\"", aec.Bold.Apply(opts.license))
	}
	return nil
}

func setConfigFromOptions(config *wpmjson.Config, opts *initOptions) {
	config.Name = opts.name
	config.License = opts.license
	config.Version = opts.version
	config.Type = wpmjson.PackageType(opts.packageType)
}

func promptForConfig(ctx context.Context, wpmCli command.Cli, config *wpmjson.Config, ve *validator.Validate, defaultName string) error {
	prompts := []promptField{
		{
			"name",
			prompt{
				"package name",
				defaultName,
				func(val string) error {
					if val == "" {
						val = defaultName
					}
					if errs := ve.Var(val, "required,wpm_name"); errs != nil {
						return errors.Errorf("invalid package name: \"%s\"", aec.Bold.Apply(val))
					}
					config.Name = val
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
					if errs := ve.Var(val, "required,wpm_semver"); errs != nil {
						return errors.Errorf("invalid version: \"%s\"", aec.Bold.Apply(val))
					}
					config.Version = val
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
					config.License = val
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
					if errs := ve.Var(val, "required,oneof=plugin theme mu-plugin"); errs != nil {
						return errors.Errorf("invalid type: \"%s\"", aec.Bold.Apply(val))
					}
					config.Type = wpmjson.PackageType(val)
					return nil
				},
			},
		},
	}

	for _, pf := range prompts {
		for {
			val, err := command.PromptForInput(ctx, wpmCli.In(), wpmCli.Out(), fmt.Sprintf("%s (%s): ", pf.Prompt.Msg, pf.Prompt.Default))
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
	return nil
}

func findExistingProjectFiles(cwd string) (existingProjectFiles, error) {
	var paths existingProjectFiles
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
		case strings.ToLower(wpmjson.ConfigFile):
			paths.wpmJson = fullPath
		}
	}

	return paths, nil
}

func findMainPluginFile(cwd string, files []os.DirEntry) (string, parser.PluginFileHeaders, error) {
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".php") {
			continue
		}

		filePath := filepath.Join(cwd, file.Name())
		headers, err := parser.GetPluginHeaders(filePath)
		if err != nil {
			continue
		}

		if headers.Name != "" {
			return filePath, headers, nil
		}
	}

	return "", parser.PluginFileHeaders{}, errors.New("no main plugin file with valid plugin headers found")
}

func getMetaString(meta map[string]any, key string, defaultValue string) string {
	if val, ok := meta[key]; ok {
		if strVal, ok := val.(string); ok && strVal != "" {
			return strVal
		}
	}
	return defaultValue
}

func getMetaStringSlice(meta map[string]any, key string) []string {
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

	// Check if all characters are symbols (no letters)
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

func trimMeaningfully(s string, limit int) string {
	if s == "" {
		return s
	}

	if limit <= 0 {
		return ""
	}

	// String must be at least 3 characters
	if len(s) < 3 {
		return ""
	}

	if len(s) <= limit {
		return s
	}

	truncated := s[:limit]
	lastDotIndex := strings.LastIndex(truncated, ".")
	if lastDotIndex != -1 {
		return truncated[:lastDotIndex+1]
	}
	return truncated
}

func buildWPMConfig(opts initOptions, pkgType string, mainFileHeaders any, readmeMeta map[string]any) *wpmjson.Config {
	cfg := wpmjson.NewConfig()
	ve, err := wpmjson.NewValidator()
	if err != nil {
		return cfg
	}

	cfg.Type = wpmjson.PackageType(pkgType)
	cfg.License = getMetaString(readmeMeta, "license", opts.license)
	cfg.Description = getMetaString(readmeMeta, "meta_description", "")

	tags := getMetaStringSlice(readmeMeta, "tags")
	if len(tags) > 0 {
		cfg.Tags = tags
	}

	platform := &wpmjson.Platform{}
	dependencies := make(wpmjson.Dependencies)
	cfg.Team = getMetaStringSlice(readmeMeta, "contributors")
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
			cfg.Tags = h.Tags
		}

		if h.ThemeURI != "" {
			if err := ve.Var(h.ThemeURI, "url,wpm_http_url,min=10,max=200"); err == nil {
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
			cfg.Tags = h.Tags
		}

		if h.PluginURI != "" {
			if err := ve.Var(h.PluginURI, "url,wpm_http_url,min=10,max=200"); err == nil {
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
				if err := ve.Var(reqPlugin, "wpm_name"); err != nil {
					continue
				}

				// Add "*" as version since requires plugins only specify the plugin slug, not a version.
				dependencies[reqPlugin] = "*"
			}
		}
	}

	// Trim tags to max 5
	if len(cfg.Tags) > 5 {
		cfg.Tags = cfg.Tags[:5]
	}

	if len(cfg.Tags) > 0 {
		// pop tags having minimum 2 and maximum 64 characters
		validTags := []string{}
		for _, tag := range cfg.Tags {
			if len(tag) >= 2 && len(tag) <= 64 {
				validTags = append(validTags, tag)
			}
		}

		slices.Sort(validTags)

		cfg.Tags = slices.Compact(validTags)
	}

	// Trim team to max 100 members
	if len(cfg.Team) > 100 {
		cfg.Team = cfg.Team[:100]
	}

	if len(cfg.Team) > 0 {
		// pop team members having minimum 2 and maximum 100 characters
		validTeam := []string{}
		for _, member := range cfg.Team {
			if len(member) >= 2 && len(member) <= 100 {
				validTeam = append(validTeam, member)
			}
		}

		slices.Sort(validTeam)

		cfg.Team = slices.Compact(validTeam)
	}

	// Trim description to max 512 characters
	if len(cfg.Description) > 512 {
		cfg.Description = trimMeaningfully(cfg.Description, 512)
	}

	// Validate license length, and set to empty if invalid
	if len(cfg.License) < 3 || len(cfg.License) > 100 {
		cfg.License = ""
	}

	if wpRequires != "" {
		_, err := semver.NewConstraint(wpRequires)
		if err == nil {
			platform.WP = "^" + wpRequires
		}
	}
	if phpRequires != "" {
		_, err := semver.NewConstraint(phpRequires)
		if err == nil {
			platform.PHP = "^" + phpRequires
		}
	}

	if len(dependencies) > 0 {
		cfg.Dependencies = &dependencies
	}

	if platform.PHP != "" || platform.WP != "" {
		cfg.Platform = platform
	}

	return cfg
}

func normalizeVersion(version string) (string, error) {
	if version == "" {
		return "", errors.New("version cannot be empty")
	}

	v, err := semver.NewVersion(version)
	if err == nil {
		return v.String(), nil
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

	// If version part start with 0, we remove it
	// Example:
	// 01.0.0 -> 1.0.0
	// 1.01.0 -> 1.1.0
	// 1.0.01 -> 1.0.1
	// 1.0.01-beta -> 1.0.1-beta
	// Split version into parts
	parts = strings.Split(version, ".")
	for i, part := range parts {
		// Check if part starts with '0' and has more characters
		if len(part) > 1 && part[0] == '0' {
			// Split part into numeric and non-numeric (e.g., "01-beta" -> "01" and "-beta")
			numericPart := part
			nonNumericPart := ""
			if hyphenIndex := strings.Index(part, "-"); hyphenIndex != -1 {
				numericPart = part[:hyphenIndex]
				nonNumericPart = part[hyphenIndex:]
			}

			// Check if numeric part is all digits and starts with '0'
			isNumeric := true
			for _, r := range numericPart {
				if !unicode.IsDigit(r) {
					isNumeric = false
					break
				}
			}

			if isNumeric && len(numericPart) > 1 && numericPart[0] == '0' {
				// Remove leading zeros from numeric part
				trimmed := strings.TrimLeft(numericPart, "0")
				if trimmed == "" {
					trimmed = "0"
				}
				// Reconstruct the part
				parts[i] = trimmed + nonNumericPart
			}
		}
	}
	version = strings.Join(parts, ".")

	v, err = semver.NewVersion(version)
	if err != nil {
		return "", err
	}

	return v.String(), nil
}

func detectPackageType(cwd string) string {
	if _, err := os.Stat(filepath.Join(cwd, "style.css")); err == nil {
		return "theme"
	}
	return "plugin"
}
