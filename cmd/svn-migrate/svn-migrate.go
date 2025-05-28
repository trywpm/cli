package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	defaultMaxWorkers = 5
	defaultTagTimeout = 5 * time.Minute // Timeout per individual tag
	requestTimeout    = 30 * time.Second
)

var (
	nameReg    = regexp.MustCompile(`^[\w-]{3,164}$`)
	httpClient = &http.Client{Timeout: requestTimeout}
)

// CLI configuration
type Config struct {
	RepoPath   string
	RepoType   string
	MaxWorkers int
	TagTimeout time.Duration
	DryRun     bool
	Verbose    bool
	WpmPath    string
}

// Package information
type PackageInfo struct {
	Name          string
	Type          string
	Path          string
	TagsPath      string
	LatestVersion string
	Tags          []string
}

// Migration result
type MigrationResult struct {
	PackageName string
	Success     bool
	Error       error
	Duration    time.Duration
	TagsCount   int
}

// WordPress API response
type APIResponse struct {
	Version string `json:"version"`
}

func normalizeVersion(version string) (string, error) {
	if version == "" {
		return "", errors.New("version cannot be empty")
	}

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

func fetchLatestVersion(ctx context.Context, packageName, repoType string) (string, error) {
	var apiURL string
	if repoType == "theme" {
		apiURL = fmt.Sprintf("https://api.wordpress.org/themes/info/1.2/?action=theme_information&slug=%s", packageName)
	} else {
		apiURL = fmt.Sprintf("https://api.wordpress.org/plugins/info/1.2/?action=plugin_information&slug=%s", packageName)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status code %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}

	return apiResp.Version, nil
}

func getPackageTags(tagsPath string) ([]string, error) {
	entries, err := os.ReadDir(tagsPath)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := normalizeVersion(entry.Name()); err == nil {
				tags = append(tags, entry.Name())
			}
		}
	}

	return tags, nil
}

func runWpmCommand(ctx context.Context, wpmPath string, args []string, workDir string, logFile *os.File) error {
	cmd := exec.CommandContext(ctx, wpmPath, args...)
	cmd.Dir = workDir

	// Log the command being executed
	fmt.Fprintf(logFile, "[%s] Executing: %s %s\n", time.Now().Format("2006-01-02 15:04:05"), wpmPath, strings.Join(args, " "))
	fmt.Fprintf(logFile, "Working directory: %s\n", workDir)

	// Capture output
	output, err := cmd.CombinedOutput()
	fmt.Fprintf(logFile, "Command output:\n%s\n", string(output))

	if err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(logFile, "Command timed out after timeout period\n")
			return fmt.Errorf("wpm command timed out")
		}

		fmt.Fprintf(logFile, "Command failed with error: %v\n", err)
		if cmd.ProcessState != nil {
			fmt.Fprintf(logFile, "Exit code: %d\n", cmd.ProcessState.ExitCode())
		}
		return fmt.Errorf("wpm command failed: %w", err)
	}

	fmt.Fprintf(logFile, "Command completed successfully\n")
	return nil
}

func migratePackage(ctx context.Context, pkg *PackageInfo, config *Config) *MigrationResult {
	start := time.Now()
	result := &MigrationResult{
		PackageName: pkg.Name,
		TagsCount:   len(pkg.Tags),
	}

	// Create log file
	logPath := filepath.Join(pkg.Path, "migrate.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		result.Error = fmt.Errorf("failed to create log file: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	defer logFile.Close()

	// Write log header
	fmt.Fprintf(logFile, "========================================\n")
	fmt.Fprintf(logFile, "WordPress Package Migration Log\n")
	fmt.Fprintf(logFile, "========================================\n")
	fmt.Fprintf(logFile, "Package: %s\n", pkg.Name)
	fmt.Fprintf(logFile, "Type: %s\n", pkg.Type)
	fmt.Fprintf(logFile, "Path: %s\n", pkg.Path)
	fmt.Fprintf(logFile, "Tags Path: %s\n", pkg.TagsPath)
	fmt.Fprintf(logFile, "Latest Version: %s\n", pkg.LatestVersion)
	fmt.Fprintf(logFile, "Total Tags: %d\n", len(pkg.Tags))
	fmt.Fprintf(logFile, "Tag Timeout: %v\n", config.TagTimeout)
	fmt.Fprintf(logFile, "Started at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "========================================\n\n")

	// Process each tag/version
	var failedTags []string
	var successfulTags []string

	for i, version := range pkg.Tags {
		fmt.Fprintf(logFile, "--- Processing Version %s (%d/%d) ---\n", version, i+1, len(pkg.Tags))

		// we only run migration for 5 minutes per tag
		// because there are thousand of tags in whole svn repository
		tagCtx, cancel := context.WithTimeout(ctx, config.TagTimeout)

		tagPath := filepath.Join(pkg.TagsPath, version)

		// Check if tag directory exists
		if _, err := os.Stat(tagPath); os.IsNotExist(err) {
			fmt.Fprintf(logFile, "ERROR: Tag directory not found: %s\n", tagPath)
			fmt.Fprintf(logFile, "Skipping version %s\n\n", version)
			failedTags = append(failedTags, fmt.Sprintf("%s (directory not found)", version))
			cancel()
			continue
		}

		// Run wpm init --migrate
		initArgs := []string{
			"--cwd", tagPath,
			"init",
			"--migrate",
			"--name", pkg.Name,
			"--version", version,
		}

		if err := runWpmCommand(tagCtx, config.WpmPath, initArgs, tagPath, logFile); err != nil {
			fmt.Fprintf(logFile, "ERROR: Init failed for version %s: %v\n", version, err)
			fmt.Fprintf(logFile, "Skipping version %s\n\n", version)
			failedTags = append(failedTags, fmt.Sprintf("%s (init failed)", version))
			cancel()
			continue
		}

		// Determine tag for publish
		var publishTag string
		if version == pkg.LatestVersion {
			publishTag = "latest"
		} else {
			publishTag = "untagged"
		}

		// Run wpm publish
		publishArgs := []string{
			"--cwd", tagPath,
			"publish",
			"--access", "public",
			"--tag", publishTag,
		}

		if err := runWpmCommand(tagCtx, config.WpmPath, publishArgs, tagPath, logFile); err != nil {
			fmt.Fprintf(logFile, "ERROR: Publish failed for version %s: %v\n", version, err)
			fmt.Fprintf(logFile, "Skipping version %s\n\n", version)
			failedTags = append(failedTags, fmt.Sprintf("%s (publish failed)", version))
			cancel()
			continue
		}

		cancel()
		fmt.Fprintf(logFile, "✓ Successfully migrated version %s with tag '%s'\n\n", version, publishTag)
		successfulTags = append(successfulTags, version)
	}

	// Summary in log
	fmt.Fprintf(logFile, "========================================\n")
	fmt.Fprintf(logFile, "PACKAGE MIGRATION SUMMARY\n")
	fmt.Fprintf(logFile, "========================================\n")
	fmt.Fprintf(logFile, "Total versions: %d\n", len(pkg.Tags))
	fmt.Fprintf(logFile, "Successful: %d\n", len(successfulTags))
	fmt.Fprintf(logFile, "Failed: %d\n", len(failedTags))

	if len(successfulTags) > 0 {
		fmt.Fprintf(logFile, "Successful versions: %s\n", strings.Join(successfulTags, ", "))
	}

	if len(failedTags) > 0 {
		fmt.Fprintf(logFile, "Failed versions: %s\n", strings.Join(failedTags, ", "))
	}

	fmt.Fprintf(logFile, "Completed at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Duration: %v\n", time.Since(start))

	if len(successfulTags) == 0 {
		result.Error = fmt.Errorf("all %d tags failed migration", len(pkg.Tags))
	} else if len(failedTags) > 0 {
		result.Success = true
		fmt.Fprintf(logFile, "PARTIAL SUCCESS: %d/%d tags migrated successfully\n", len(successfulTags), len(pkg.Tags))
	} else {
		result.Success = true
		fmt.Fprintf(logFile, "COMPLETE SUCCESS: All tags migrated successfully\n")
	}

	result.Duration = time.Since(start)
	return result
}

func processPackage(ctx context.Context, svnRepoPath, repoType, packageName string, config *Config) (*PackageInfo, error) {
	if !nameReg.MatchString(packageName) {
		return nil, fmt.Errorf("invalid package name: %s", packageName)
	}

	var tagsPath string
	packagePath := filepath.Join(svnRepoPath, packageName)

	if repoType == "theme" {
		tagsPath = packagePath
	} else {
		tagsPath = filepath.Join(packagePath, "tags")
	}

	// Check if tags directory exists
	if _, err := os.Stat(tagsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tags directory not found: %s", tagsPath)
	}

	// Fetch latest version from WordPress API
	// It will also validate if package qualifies as a valid theme/plugin to be migrated
	// to the wpm registry
	latestVersion, err := fetchLatestVersion(ctx, packageName, repoType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest version: %w", err)
	}

	// Get package tags. In theme its themeName/..tags and in plugin its themeName/tags/...tags
	tags, err := getPackageTags(tagsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags: %w", err)
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no valid tags found")
	}

	return &PackageInfo{
		Name:          packageName,
		Type:          repoType,
		Path:          packagePath,
		TagsPath:      tagsPath,
		LatestVersion: latestVersion,
		Tags:          tags,
	}, nil
}

func worker(ctx context.Context, jobs <-chan string, results chan<- *MigrationResult, svnRepoPath string, config *Config, wg *sync.WaitGroup) {
	defer wg.Done()

	for packageName := range jobs {
		select {
		case <-ctx.Done():
			results <- &MigrationResult{
				PackageName: packageName,
				Error:       ctx.Err(),
			}
			return
		default:
			pkg, err := processPackage(ctx, svnRepoPath, config.RepoType, packageName, config)
			if err != nil {
				results <- &MigrationResult{
					PackageName: packageName,
					Error:       err,
				}
				continue
			}

			// Migrate package
			result := migratePackage(ctx, pkg, config)
			results <- result
		}
	}
}

// Cobra command setup
var rootCmd = &cobra.Command{
	Use:           "wp-migrate",
	Short:         "svn to wpm migration tool",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate [repository-path]",
	Short: "Migrate svn to wpm registry",
	Args:  cobra.ExactArgs(1),
	RunE:  runMigrate,
}

func runMigrate(cmd *cobra.Command, args []string) error {
	config := &Config{}

	// Get flags
	config.RepoPath = args[0]
	config.RepoType, _ = cmd.Flags().GetString("type")
	config.MaxWorkers, _ = cmd.Flags().GetInt("workers")
	config.TagTimeout, _ = cmd.Flags().GetDuration("tag-timeout")
	config.DryRun, _ = cmd.Flags().GetBool("dry-run")
	config.Verbose, _ = cmd.Flags().GetBool("verbose")
	config.WpmPath, _ = cmd.Flags().GetString("wpm-path")

	// Validate inputs
	if err := validateConfig(config); err != nil {
		return err
	}

	// Read packages
	packages, err := os.ReadDir(config.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	// Filter valid packages
	var packageNames []string
	for _, pkg := range packages {
		if pkg.IsDir() {
			packageNames = append(packageNames, pkg.Name())
		}
	}

	if len(packageNames) == 0 {
		fmt.Println("No packages found in repository")
		return nil
	}

	fmt.Printf("Found %d package(s) in %s repository\n", len(packageNames), config.RepoType)

	if config.DryRun {
		fmt.Println("\nDry run - packages that would be migrated:")
		for _, name := range packageNames {
			fmt.Printf("  - %s\n", name)
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup worker pool
	jobs := make(chan string, len(packageNames))
	results := make(chan *MigrationResult, len(packageNames))

	var wg sync.WaitGroup

	numWorkers := config.MaxWorkers
	if len(packageNames) < numWorkers {
		numWorkers = len(packageNames)
	}

	fmt.Printf("Starting migration with %d worker(s)...\n", numWorkers)
	fmt.Printf("Timeout per tag: %v\n", config.TagTimeout)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, jobs, results, config.RepoPath, config, &wg)
	}

	go func() {
		defer close(jobs)
		for _, name := range packageNames {
			select {
			case jobs <- name:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var successCount, errorCount int
	var totalDuration time.Duration
	var failedPackages []string
	var totalTags int

	for result := range results {
		if result.Success {
			if result.Error != nil {
				fmt.Printf("⚠ %s (%d tags, %.2fs) - some tags failed\n", result.PackageName, result.TagsCount, result.Duration.Seconds())
			} else {
				fmt.Printf("✓ %s (%d tags, %.2fs)\n", result.PackageName, result.TagsCount, result.Duration.Seconds())
			}

			successCount++
			totalTags += result.TagsCount
		} else {
			fmt.Printf("✗ %s: %v\n", result.PackageName, result.Error)
			errorCount++
			failedPackages = append(failedPackages, result.PackageName)
		}
		totalDuration += result.Duration
	}

	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Total packages: %d\n", len(packageNames))
	fmt.Printf("Successfully migrated: %d\n", successCount)
	fmt.Printf("Failed: %d\n", errorCount)
	fmt.Printf("Total tags/versions processed: %d\n", totalTags)
	if len(packageNames) > 0 {
		fmt.Printf("Success rate: %d%%\n", (successCount*100)/len(packageNames))
	}
	fmt.Printf("Each package has detailed logs in migrate.log files\n")

	if errorCount > 0 {
		fmt.Printf("\nFailed packages:\n")
		for _, name := range failedPackages {
			fmt.Printf("  - %s\n", name)
		}
		return fmt.Errorf("%d packages failed migration", errorCount)
	}

	fmt.Printf("\n✓ All packages migrated successfully!\n")
	return nil
}

func validateConfig(config *Config) error {
	absPath, err := filepath.Abs(config.RepoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	config.RepoPath = absPath

	if info, err := os.Stat(config.RepoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository path is not a valid directory: %s", config.RepoPath)
	}

	if config.RepoType != "plugin" && config.RepoType != "theme" {
		return fmt.Errorf("repository type must be 'plugin' or 'theme'")
	}

	// Validate wpm path
	if config.WpmPath == "" {
		if _, err := exec.LookPath("wpm"); err != nil {
			return fmt.Errorf("wpm command not found in PATH, please specify --wpm-path")
		}
		config.WpmPath = "wpm"
	} else {
		if _, err := os.Stat(config.WpmPath); err != nil {
			return fmt.Errorf("wpm binary not found at path: %s", config.WpmPath)
		}
	}

	if config.MaxWorkers <= 0 {
		config.MaxWorkers = defaultMaxWorkers
	}
	if config.TagTimeout <= 0 {
		config.TagTimeout = defaultTagTimeout
	}

	return nil
}

func init() {
	migrateCmd.Flags().StringP("type", "t", "", "Repository type: 'plugin' or 'theme' (required)")
	migrateCmd.Flags().IntP("workers", "w", defaultMaxWorkers, "Number of parallel workers")
	migrateCmd.Flags().Duration("tag-timeout", defaultTagTimeout, "Timeout per individual tag migration")
	migrateCmd.Flags().Bool("dry-run", false, "Show what would be migrated without executing")
	migrateCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")
	migrateCmd.Flags().String("wpm-path", "", "Path to wpm binary (default: search in PATH)")

	err := migrateCmd.MarkFlagRequired("type")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marking 'type' flag as required: %v\n", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(migrateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
