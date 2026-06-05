package init

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"
	"github.com/morikuni/aec"
	"github.com/spf13/cobra"

	"go.wpm.so/cli/cli/command"
	"go.wpm.so/cli/cli/command/completion"
	"go.wpm.so/cli/pkg/output"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmjson/validator"
	"go.wpm.so/cli/pkg/version"
	"go.wpm.so/cli/pkg/wp/parser"
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
	flags.StringVar(&opts.packageType, "type", "", "Package type (plugin, theme)")

	_ = cmd.RegisterFlagCompletionFunc("type", completion.PackageTypes())
	_ = cmd.RegisterFlagCompletionFunc("license", completion.PackageLicenses())

	return cmd
}

func runNewInit(ctx context.Context, wpmCli command.Cli, opts *initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	wpmConfigFilePath := filepath.Join(cwd, wpmjson.ConfigFile)
	if _, err := os.Stat(wpmConfigFilePath); err == nil {
		return fmt.Errorf("%s already exists in %s", wpmjson.ConfigFile, cwd)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check for existing %s: %w", wpmjson.ConfigFile, err)
	}

	wpmCfg := wpmjson.New()
	defaultName := filepath.Base(cwd)

	if !wpmCli.Out().IsTerminal() {
		opts.yes = true
	}

	if opts.yes {
		if err := setDefaults(opts, defaultName); err != nil {
			return err
		}

		if err := validateOptions(opts, true); err != nil {
			return err
		}

		setConfigFromOptions(wpmCfg, opts)
	} else {
		if err := promptForConfig(ctx, wpmCli, wpmCfg, defaultName); err != nil {
			return err
		}
	}

	if err := wpmCfg.Write(cwd); err != nil {
		return fmt.Errorf("failed to write wpm.json: %w", err)
	}

	_, _ = fmt.Fprintf(wpmCli.Out(), "config created at %s\n", wpmConfigFilePath)

	return nil
}

// extractPackageHeaders inspects the working directory for the package's main
// file (style.css for themes, main plugin .php for plugins) and returns the
// parsed headers plus the version string it found. Returns (nil, "", nil) when
// the main file is missing but --version was supplied.
func extractPackageHeaders(wpmCli command.Cli, cwd string, opts *initOptions) (mainFileHeaders any, extractedVersion string, err error) {
	switch opts.packageType {
	case "theme":
		mainFilePath := filepath.Join(cwd, "style.css")
		if _, err := os.Stat(mainFilePath); err != nil {
			if os.IsNotExist(err) && opts.version == "" {
				return nil, "", fmt.Errorf("style.css not found in %s", cwd)
			}
			if !os.IsNotExist(err) {
				return nil, "", fmt.Errorf("failed to stat style.css: %w", err)
			}
			return nil, "", nil
		}
		headers, hErr := parser.GetThemeHeaders(mainFilePath)
		if hErr != nil {
			if opts.version == "" {
				return nil, "", fmt.Errorf("failed to parse theme headers from style.css: %w", hErr)
			}
			return nil, "", nil
		}
		return headers, headers.Version, nil

	case "plugin":
		dirEntries, dErr := os.ReadDir(cwd)
		if dErr != nil {
			return nil, "", fmt.Errorf("failed to read current directory for plugin files: %w", dErr)
		}

		foundPath, headers, fErr := findMainPluginFile(cwd, dirEntries)
		if fErr != nil {
			if opts.version == "" {
				return nil, "", fmt.Errorf("failed to identify main plugin file: %w", fErr)
			}
			return nil, "", nil
		}
		_, _ = fmt.Fprintf(wpmCli.Out(), "main plugin file found: %s\n", foundPath)
		return headers, headers.Version, nil

	default:
		return nil, "", fmt.Errorf("unsupported package type for existing project init: %s", opts.packageType)
	}
}

// resolveConfigVersion fills wpmCfg.Version from --version or the extracted
// header, warns on mismatch, and normalizes to strict semver.
func resolveConfigVersion(wpmCli command.Cli, wpmCfg *wpmjson.Config, opts *initOptions, extractedVersion string) error {
	switch {
	case opts.version != "":
		wpmCfg.Version = opts.version
		if extractedVersion != "" && extractedVersion != opts.version {
			wpmCli.Output().PrettyErrorln(output.Text{
				Plain: fmt.Sprintf("warn: provided version (%s) differs from version in parsed headers (%s)", opts.version, extractedVersion),
				Fancy: fmt.Sprintf(
					"%s provided version (%s) differs from version in parsed headers (%s)",
					aec.YellowF.Apply("warn:"),
					aec.LightBlueF.Apply(opts.version), aec.LightBlueF.Apply(extractedVersion),
				),
			})
		}
	case extractedVersion == "":
		return errors.New("unable to determine version; please specify it with --version")
	default:
		wpmCfg.Version = extractedVersion
	}

	v, err := version.Normalize(wpmCfg.Version)
	if err != nil {
		return errors.New("invalid version format: " + err.Error())
	}
	wpmCfg.Version = v
	return nil
}

func runExistingInit(wpmCli command.Cli, opts *initOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
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
			return fmt.Errorf("failed to read readme.txt: %w", err)
		}
		readmeParser = parser.NewReadmeParser()
		readmeParser.Parse(string(readmeTxtContent))
	}

	mainFileHeaders, extractedVersion, err := extractPackageHeaders(wpmCli, cwd, opts)
	if err != nil {
		return err
	}

	wpmCfg := buildWpmConfig(*opts, opts.packageType, mainFileHeaders, readmeParser.GetMetadata())

	if opts.name != "" {
		wpmCfg.Name = opts.name
	} else {
		wpmCfg.Name = filepath.Base(cwd)
	}

	removeSelfDependency(wpmCfg)

	if err := resolveConfigVersion(wpmCli, wpmCfg, opts, extractedVersion); err != nil {
		return err
	}
	if err := wpmCfg.Validate(); err != nil {
		return err
	}

	// create wpm.json file
	if err := wpmCfg.Write(cwd); err != nil {
		return err
	}

	// create readme.md from readme.txt if it exists
	if baseFiles.readmeTxt != "" && baseFiles.readmeMd == "" {
		markdownContent := readmeParser.ToMarkdown()
		readmeMdPath := strings.TrimSuffix(baseFiles.readmeTxt, filepath.Ext(baseFiles.readmeTxt)) + ".md"
		if err := os.WriteFile(readmeMdPath, []byte(markdownContent), 0o644); err != nil {
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
	if opts.license == "" {
		opts.license = defaultLicense
	}
	return nil
}

func validateOptions(opts *initOptions, omitType bool) error {
	if err := validator.IsValidPackageName(opts.name); err != nil {
		return fmt.Errorf("name %w", err)
	}
	if err := validator.IsValidVersion(opts.version); err != nil {
		return fmt.Errorf("version %w", err)
	}
	if !omitType {
		if err := validator.IsValidPackageType(types.PackageType(opts.packageType)); err != nil {
			return fmt.Errorf("type %w", err)
		}
	}
	if err := validator.IsValidLicense(opts.license); err != nil {
		return fmt.Errorf("license %w", err)
	}

	return nil
}

func setConfigFromOptions(config *wpmjson.Config, opts *initOptions) {
	config.Name = opts.name
	config.License = opts.license
	config.Version = opts.version

	pkgType := types.PackageType(opts.packageType)
	if pkgType.Valid() {
		config.Type = pkgType
	}
}

func promptForConfig(ctx context.Context, wpmCli command.Cli, config *wpmjson.Config, defaultName string) error {
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
					if err := validator.IsValidPackageName(val); err != nil {
						return fmt.Errorf("name %w", err)
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
					if err := validator.IsValidVersion(val); err != nil {
						return fmt.Errorf("version %w", err)
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
					if err := validator.IsValidPackageType(types.PackageType(val)); err != nil {
						return fmt.Errorf("type %w", err)
					}
					config.Type = types.PackageType(val)
					return nil
				},
			},
		},
	}

	for _, pf := range prompts {
		for {
			val, err := command.PromptForInput(ctx, wpmCli.In(), wpmCli.Out(), fmt.Sprintf("%s (%s): ", pf.Prompt.Msg, pf.Prompt.Default))
			if err != nil {
				return fmt.Errorf("failed to get prompt input: %w", err)
			}

			if err := pf.Prompt.Validate(val); err != nil {
				_, _ = fmt.Fprintf(wpmCli.Err(), "%s\n", err)
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
		return paths, fmt.Errorf("failed to read current directory: %w", err)
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

func getMetaString(meta map[string]any, key, defaultValue string) string {
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

func buildWpmConfig(opts initOptions, pkgType string, mainFileHeaders any, readmeMeta map[string]any) *wpmjson.Config {
	cfg := wpmjson.New()
	cfg.Type = types.PackageType(pkgType)
	cfg.License = getMetaString(readmeMeta, "license", opts.license)
	cfg.Description = getMetaString(readmeMeta, "meta_description", "")

	tags := getMetaStringSlice(readmeMeta, "tags")
	if len(tags) > 0 {
		cfg.Tags = tags
	}

	dependencies := &types.Dependencies{}
	wpRequires := getMetaString(readmeMeta, "requires", "")
	phpRequires := getMetaString(readmeMeta, "requires_php", "")
	testedUpTo := getMetaString(readmeMeta, "tested", "")

	switch h := mainFileHeaders.(type) {
	case parser.ThemeFileHeaders:
		applyThemeHeaders(cfg, h, tags, &wpRequires, &phpRequires)
	case parser.PluginFileHeaders:
		applyPluginHeaders(cfg, h, tags, dependencies, &wpRequires, &phpRequires)
	}

	cfg.Tags = sanitizeStringList(cfg.Tags, 5, 2, 64)
	if len(cfg.Description) > 512 {
		cfg.Description = trimMeaningfully(cfg.Description, 512)
	}

	requires := buildRequires(wpRequires, phpRequires, testedUpTo)
	if len(*dependencies) > 0 {
		cfg.Dependencies = dependencies
	}
	if requires.PHP != "" || requires.WP != "" {
		cfg.Requires = requires
	}

	dropInvalidMetadata(cfg)
	return cfg
}

// removeSelfDependency drops a dependency whose name matches the package itself.
func removeSelfDependency(cfg *wpmjson.Config) {
	if cfg.Dependencies == nil {
		return
	}
	delete(*cfg.Dependencies, cfg.Name)
	if len(*cfg.Dependencies) == 0 {
		cfg.Dependencies = nil
	}
}

// dropInvalidMetadata clears optional fields that fail their own validator,
// keeping init's best-effort extraction from emitting an invalid wpm.json.
func dropInvalidMetadata(cfg *wpmjson.Config) {
	cfg.Tags = slices.DeleteFunc(cfg.Tags, func(tag string) bool {
		return validator.IsSafeString(tag) != nil
	})
	if cfg.Description != "" && validator.IsValidDescription(cfg.Description) != nil {
		cfg.Description = ""
	}
	if cfg.License != "" && validator.IsValidLicense(cfg.License) != nil {
		cfg.License = ""
	}
	if cfg.Author != "" && validator.IsValidAuthor(cfg.Author) != nil {
		cfg.Author = ""
	}
	if cfg.Requires == nil {
		return
	}
	if cfg.Requires.WP != "" && validator.IsValidConstraint(cfg.Requires.WP) != nil {
		cfg.Requires.WP = ""
	}
	if cfg.Requires.PHP != "" && validator.IsValidConstraint(cfg.Requires.PHP) != nil {
		cfg.Requires.PHP = ""
	}
	if cfg.Requires.WP == "" && cfg.Requires.PHP == "" {
		cfg.Requires = nil
	}
}

// applyThemeHeaders fills config fields from a theme's style.css headers,
// only overwriting empty values and only forwarding WP/PHP requires when
// they weren't already set from readme.txt metadata.
func applyThemeHeaders(cfg *wpmjson.Config, h parser.ThemeFileHeaders, tags []string, wpRequires, phpRequires *string) {
	if cfg.License == "" {
		cfg.License = h.License
	}
	if cfg.Description == "" || !isMeaningfulText(cfg.Description) {
		cfg.Description = h.Description
	}
	if h.Author != "" {
		cfg.Author = h.Author
	}
	if len(tags) == 0 && len(h.Tags) > 0 {
		cfg.Tags = h.Tags
	}
	if h.ThemeURI != "" {
		if err := validator.IsValidHomepage(h.ThemeURI); err == nil {
			cfg.Homepage = h.ThemeURI
		}
	}
	if *wpRequires == "" && h.RequiresWP != "" {
		*wpRequires = h.RequiresWP
	}
	if *phpRequires == "" && h.RequiresPHP != "" {
		*phpRequires = h.RequiresPHP
	}
}

// applyPluginHeaders fills config fields from a plugin's main file headers
// and, additionally, populates the dependencies map from "Requires Plugins".
func applyPluginHeaders(cfg *wpmjson.Config, h parser.PluginFileHeaders, tags []string, dependencies *types.Dependencies, wpRequires, phpRequires *string) {
	if cfg.License == "" {
		cfg.License = h.License
	}
	if cfg.Description == "" || !isMeaningfulText(cfg.Description) {
		cfg.Description = h.Description
	}
	if h.Author != "" {
		cfg.Author = h.Author
	}
	if len(tags) == 0 && len(h.Tags) > 0 {
		cfg.Tags = h.Tags
	}
	if h.PluginURI != "" {
		if err := validator.IsValidHomepage(h.PluginURI); err == nil {
			cfg.Homepage = h.PluginURI
		}
	}
	if *wpRequires == "" && h.RequiresWP != "" {
		*wpRequires = h.RequiresWP
	}
	if *phpRequires == "" && h.RequiresPHP != "" {
		*phpRequires = h.RequiresPHP
	}
	for _, reqPlugin := range h.RequiresPlugins {
		if len(*dependencies) >= validator.MaxDependencies {
			break
		}
		if err := validator.IsValidPackageName(reqPlugin); err != nil {
			continue
		}
		// "Requires Plugins" header carries only slugs, so pin to "*".
		(*dependencies)[reqPlugin] = "*"
	}
}

// sanitizeStringList trims the list to maxItems, drops entries outside
// [minLen, maxLen], sorts, and compacts duplicates.
func sanitizeStringList(items []string, maxItems, minLen, maxLen int) []string {
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	if len(items) == 0 {
		return items
	}
	valid := []string{}
	for _, it := range items {
		if len(it) >= minLen && len(it) <= maxLen {
			valid = append(valid, it)
		}
	}
	slices.Sort(valid)
	return slices.Compact(valid)
}

// buildRequires constructs the runtime requires from raw WP/PHP constraint
// strings plus an optional "tested up to" upper bound on the WP range.
func buildRequires(wpRequires, phpRequires, testedUpTo string) *types.Requires {
	requires := &types.Requires{}
	if wpRequires != "" {
		if _, err := semver.NewConstraint(wpRequires); err == nil {
			requires.WP = ">=" + wpRequires
		}
		if _, err := semver.NewVersion(testedUpTo); err == nil && wpRequires != testedUpTo {
			requires.WP += " <=" + testedUpTo
			requires.WP = strings.TrimSpace(requires.WP)
		}
	}
	if phpRequires != "" {
		if _, err := semver.NewConstraint(phpRequires); err == nil {
			requires.PHP = ">=" + phpRequires
		}
	}
	return requires
}

func detectPackageType(cwd string) string {
	if _, err := os.Stat(filepath.Join(cwd, "style.css")); err == nil {
		return "theme"
	}
	return "plugin"
}
