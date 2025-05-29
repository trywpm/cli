package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultDownloaderWorkers      = 5
	downloaderRequestTimeout      = 30 * time.Second
	defaultDownloaderManifestFile = "svn-download-manifest.json"
	globalLogFileName             = "svn-download-activity.log"
)

var (
	nameReg              = regexp.MustCompile(`^[\w-]{3,164}$`)
	downloaderHttpClient = &http.Client{Timeout: downloaderRequestTimeout}
	log                  = logrus.New()
)

type DownloaderConfig struct {
	PackageType  string
	OutputDir    string
	NumWorkers   int
	ManifestPath string
	SvnRepoURL   string
	Limit        int
	Verbose      bool
	LogPath      string
}

type APIResponse struct {
	Version string `json:"version"`
}

type PackageDownloadState struct {
	Status          string    `json:"status"`
	LatestVersion   string    `json:"latest_version,omitempty"`
	SVNCheckedOutAt time.Time `json:"svn_checked_out_at,omitempty"`
	LocalPath       string    `json:"local_path,omitempty"`
	Error           string    `json:"error,omitempty"`
	Retryable       bool      `json:"retryable,omitempty"`
	Retries         int       `json:"retries,omitempty"`
	LastExitCode    int       `json:"last_exit_code,omitempty"`
}

type DownloaderManifest struct {
	LastUpdated         time.Time                        `json:"last_updated"`
	RepoType            string                           `json:"repo_type"`
	SvnBaseURL          string                           `json:"svn_base_url"`
	LastAPIVersionsSync time.Time                        `json:"last_api_versions_sync,omitempty"`
	Packages            map[string]*PackageDownloadState `json:"packages"`
	manifestPath        string
	mu                  sync.Mutex
}

// FileHook enables dual logging: structured JSON to file, readable text to stdout
type FileHook struct {
	file      *os.File
	formatter logrus.Formatter
	levels    []logrus.Level
}

func NewFileHook(filePath string, formatter logrus.Formatter, levels []logrus.Level) (*FileHook, error) {
	logDir := filepath.Dir(filePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create log directory: %s", logDir)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open log file: %s", filePath)
	}

	return &FileHook{file: file, formatter: formatter, levels: levels}, nil
}

func (hook *FileHook) Fire(entry *logrus.Entry) error {
	lineBytes, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = hook.file.Write(lineBytes)
	return err
}

func (hook *FileHook) Levels() []logrus.Level {
	if len(hook.levels) == 0 {
		return logrus.AllLevels
	}
	return hook.levels
}

func setupLogger(logLevel logrus.Level, jsonLogFilePath string, verbose bool) error {
	log.SetOutput(os.Stdout)
	log.SetLevel(logLevel)

	if verbose {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			ForceColors:     true,
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "15:04:05",
			ForceColors:     true,
		})
	}

	if jsonLogFilePath != "" {
		fileHook, err := NewFileHook(jsonLogFilePath, &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		}, logrus.AllLevels)

		if err != nil {
			log.WithError(err).Errorf("❌ failed to initialize file logging: %s", jsonLogFilePath)
		} else {
			log.AddHook(fileHook)
			log.Infof("📝 logging: text to stdout, json to %s", jsonLogFilePath)
		}
	} else {
		log.Info("📝 logging: text to stdout only")
	}

	return nil
}

func loadDownloaderManifest(path string) (*DownloaderManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("📄 downloader manifest %s not found, creating new one", path)
			return &DownloaderManifest{
				Packages:     make(map[string]*PackageDownloadState),
				manifestPath: path,
			}, nil
		}
		return nil, errors.Wrapf(err, "failed to read downloader manifest %s", path)
	}

	var manifest DownloaderManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal downloader manifest %s", path)
	}

	if manifest.Packages == nil {
		manifest.Packages = make(map[string]*PackageDownloadState)
	}
	manifest.manifestPath = path
	log.Infof("✅ loaded downloader manifest from %s", path)
	return &manifest, nil
}

func (m *DownloaderManifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LastUpdated = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal downloader manifest")
	}

	if err := os.MkdirAll(filepath.Dir(m.manifestPath), 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory for manifest %s", m.manifestPath)
	}

	return os.WriteFile(m.manifestPath, data, 0644)
}

func (m *DownloaderManifest) GetPackageState(slug string) (*PackageDownloadState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.Packages[slug]
	if exists {
		sCopy := *state
		return &sCopy, true
	}
	return nil, false
}

func (m *DownloaderManifest) UpdatePackageState(slug string, state *PackageDownloadState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Packages[slug] = state
}

func checkSVNInstalled() error {
	_, err := exec.LookPath("svn")
	if err != nil {
		return errors.New("svn command-line tool is not installed or not in PATH")
	}
	return nil
}

func listSVNPackages(ctx context.Context, svnRepoURL string, limit int) ([]string, error) {
	l := log.WithFields(logrus.Fields{
		"svn_repo_url": svnRepoURL,
		"action":       "list_packages",
	})

	l.Info("📋 listing packages from svn repository...")

	cmd := exec.CommandContext(ctx, "svn", "list", svnRepoURL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stdout pipe for svn list")
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get stderr pipe for svn list")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start svn list command")
	}

	var packageNames []string
	scanner := bufio.NewScanner(stdout)
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			name := strings.TrimRight(line, "/")
			if name != "" {
				packageNames = append(packageNames, name)
				count++
				if limit > 0 && count >= limit {
					l.Infof("📊 reached limit of %d packages from svn list", limit)
					break
				}
			}
		}
	}

	var svnErrOutput strings.Builder
	if _, err := io.Copy(&svnErrOutput, stderr); err != nil {
		l.WithError(err).Warn("could not read from svn list stderr pipe")
	}

	if err := cmd.Wait(); err != nil {
		l.Errorf("❌ svn list command failed. stderr: %s", svnErrOutput.String())
		return nil, errors.Wrapf(err, "svn list command failed for %s. stderr: %s", svnRepoURL, svnErrOutput.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading svn list output")
	}

	l.Infof("✅ found %d package names from svn", len(packageNames))
	return packageNames, nil
}

func fetchLatestVersion(ctx context.Context, packageName, packageType string) (version string, statusCode int, err error) {
	l := log.WithFields(logrus.Fields{
		"package": packageName,
		"action":  "api_lookup",
	})

	var apiURL string
	if packageType == "theme" {
		apiURL = fmt.Sprintf("https://api.wordpress.org/themes/info/1.2/?action=theme_information&slug=%s", packageName)
	} else {
		apiURL = fmt.Sprintf("https://api.wordpress.org/plugins/info/1.2/?action=plugin_information&slug=%s", packageName)
	}

	l.Debugf("fetching from: %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to create api request")
	}

	resp, err := downloaderHttpClient.Do(req)
	if err != nil {
		return "", 0, errors.Wrap(err, "api request failed")
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		l.WithError(readErr).Warn("could not read api response body")
	}

	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, fmt.Errorf("api returned status %d. body: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return "", resp.StatusCode, errors.Wrapf(err, "failed to decode api response. body: %s", string(bodyBytes))
	}

	if apiResp.Version == "" {
		return "", resp.StatusCode, fmt.Errorf("api response has no version. body: %s", string(bodyBytes))
	}

	l.Debugf("✅ fetched version: %s", apiResp.Version)
	return apiResp.Version, resp.StatusCode, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isRetryableExitCode determines if an SVN exit code indicates a retryable error
func isRetryableExitCode(exitCode int) bool {
	switch exitCode {
	case 0:
		return false // Success, no retry needed
	case 1:
		return true // General error, could be temporary
	case 2:
		return false // System error, usually not retryable
	case 3:
		return false // Authentication error, not retryable without config change
	case 4:
		return false // Authorization error, not retryable
	case 5:
		return true // Network error, retryable
	case 6:
		return false // File/path error, usually not retryable
	case 125:
		return true // Temporary failure, retryable
	default:
		// For unknown exit codes, default to non-retryable to be safe
		return false
	}
}

func checkoutSVNPackage(ctx context.Context, svnBaseRepoURL, packageName, packageType, outputDir string) (string, int, error) {
	l := log.WithFields(logrus.Fields{
		"package": packageName,
		"type":    packageType,
		"action":  "svn_checkout",
	})

	var packageSvnURL, localCheckoutPath string

	if packageType == "theme" {
		packageSvnURL = fmt.Sprintf("%s/%s", strings.TrimRight(svnBaseRepoURL, "/"), packageName)
		localCheckoutPath = filepath.Join(outputDir, packageName)
	} else {
		packageSvnURL = fmt.Sprintf("%s/%s/tags", strings.TrimRight(svnBaseRepoURL, "/"), packageName)
		localCheckoutPath = filepath.Join(outputDir, packageName, "tags")
	}

	parentDirForCheckout := filepath.Dir(localCheckoutPath)
	if err := os.MkdirAll(parentDirForCheckout, 0755); err != nil {
		l.WithError(err).Errorf("❌ failed to create parent directory: %s", parentDirForCheckout)
		return "", -1, errors.Wrapf(err, "failed to create parent dir for %s", localCheckoutPath)
	}

	// Remove existing directory for clean checkout
	if _, err := os.Stat(localCheckoutPath); err == nil {
		l.Infof("🗑️ removing existing directory %s for clean checkout", localCheckoutPath)
		if err := os.RemoveAll(localCheckoutPath); err != nil {
			l.WithError(err).Errorf("⚠️ failed to remove existing directory %s", localCheckoutPath)
		} else {
			l.Debug("✅ successfully removed existing directory")
		}
	}

	l.Infof("📥 checking out %s to %s", packageSvnURL, localCheckoutPath)

	cmdArgs := []string{
		"co",
		"--non-interactive",
		"--trust-server-cert-failures=unknown-ca,cn-mismatch,expired,not-yet-valid,other",
		packageSvnURL,
		localCheckoutPath,
	}

	cmd := exec.CommandContext(ctx, "svn", cmdArgs...)
	outputBytes, err := cmd.CombinedOutput()
	outputStr := string(outputBytes)

	l.WithField("output", outputStr).Debug("svn command output")

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	l.WithFields(logrus.Fields{
		"exit_code": exitCode,
		"output":    outputStr[:min(200, len(outputStr))],
	}).Debug("svn checkout command completed")

	if err != nil {
		l.WithError(err).WithFields(logrus.Fields{
			"exit_code":   exitCode,
			"full_output": outputStr,
		}).Error("❌ svn checkout command failed")

		return localCheckoutPath, exitCode, errors.Wrapf(err, "svn checkout failed for %s (exit code %d). output: %s", packageSvnURL, exitCode, outputStr)
	}

	l.Info("✅ svn checkout successful")
	return localCheckoutPath, exitCode, nil
}

func downloaderWorker(ctx context.Context, cfg *DownloaderConfig, manifest *DownloaderManifest, packageNames <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for packageName := range packageNames {
		select {
		case <-ctx.Done():
			log.WithField("package", packageName).Info("⚠️ worker context cancelled, stopping")
			return
		default:
		}

		l := log.WithField("package", packageName)
		l.Info("👷 worker processing package")

		currentState, exists := manifest.GetPackageState(packageName)

		if exists {
			if currentState.Status == "downloaded" || currentState.Status == "api_not_found" || currentState.Status == "skipped_invalid_name" {
				l.Infof("⏭️ package status is '%s', skipping", currentState.Status)
				continue
			}
			if currentState.Status == "download_failed" && !currentState.Retryable {
				l.Infof("⏭️ package download failed previously (non-retryable: %s), skipping", currentState.Error)
				continue
			}
		}

		if !exists {
			currentState = &PackageDownloadState{Status: "pending_api_check"}
		} else if currentState.Status == "download_failed" && currentState.Retryable {
			l.Info("🔄 retrying previously failed (but retryable) download")
			currentState.Status = "api_qualified_pending_download"
		} else {
			currentState.Status = "pending_api_check"
		}

		currentState.Retryable = false
		manifest.UpdatePackageState(packageName, currentState)

		if !nameReg.MatchString(packageName) {
			l.Warnf("⚠️ package name '%s' is not a valid slug", packageName)
			currentState.Status = "skipped_invalid_name"
			currentState.Error = "invalid slug format"
			manifest.UpdatePackageState(packageName, currentState)
			continue
		}

		if currentState.Status == "pending_api_check" {
			l.Info("🔍 checking package against wordpress api...")

			select {
			case <-ctx.Done():
				l.Info("⚠️ context cancelled during api check")
				return
			default:
			}

			currentAPIVersion, statusCode, err := fetchLatestVersion(ctx, packageName, cfg.PackageType)

			if err != nil {
				currentState.Error = err.Error()
				if statusCode == http.StatusNotFound || strings.Contains(err.Error(), "no version") {
					l.Info("❌ package not found on wordpress api (404 or no version)")
					currentState.Status = "api_not_found"
					currentState.LatestVersion = ""
				} else {
					l.WithError(err).Warn("⚠️ error checking package with wordpress api")
					currentState.Status = "api_error"
				}
			} else {
				l.Infof("✅ package found on wordpress api, version: %s", currentAPIVersion)
				if currentState.LatestVersion != "" && currentState.LatestVersion != currentAPIVersion {
					l.Infof("🔄 api version changed: old '%s', new '%s', will re-sync", currentState.LatestVersion, currentAPIVersion)
				}
				currentState.Status = "api_qualified_pending_download"
				currentState.LatestVersion = currentAPIVersion
				currentState.Error = ""
			}

			manifest.UpdatePackageState(packageName, currentState)
		}

		if currentState.Status == "api_qualified_pending_download" {
			select {
			case <-ctx.Done():
				l.Info("⚠️ context cancelled during download")
				return
			default:
			}

			l.Info("📥 attempting svn checkout...")
			localPath, exitCode, err := checkoutSVNPackage(ctx, cfg.SvnRepoURL, packageName, cfg.PackageType, cfg.OutputDir)

			currentState.LocalPath = localPath
			currentState.LastExitCode = exitCode

			if err != nil {
				l.WithError(err).WithField("exit_code", exitCode).Error("❌ failed to checkout package from svn")
				currentState.Status = "download_failed"
				currentState.Error = err.Error()

				if isRetryableExitCode(exitCode) {
					currentState.Retryable = true
					l.Infof("🔄 marking download as retryable based on exit code %d", exitCode)
				} else {
					currentState.Retryable = false
					l.Infof("❌ marking download as non-retryable based on exit code %d", exitCode)
				}
			} else {
				l.Info("🎉 package downloaded/updated successfully")
				currentState.Status = "downloaded"
				currentState.SVNCheckedOutAt = time.Now()
				currentState.Error = ""
				currentState.Retryable = false
			}

			manifest.UpdatePackageState(packageName, currentState)
		}

		// Save manifest after each package
		select {
		case <-ctx.Done():
			l.Info("⚠️ context cancelled before saving manifest")
			return
		default:
		}

		if err := manifest.Save(); err != nil {
			l.WithError(err).Error("❌ failed to save manifest after processing package")
		}
	}
}

var rootCmd = &cobra.Command{
	Use:           "svn-download",
	Short:         "wordpress theme/plugin svn downloader",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runDownload,
}

func runDownload(cmd *cobra.Command, args []string) error {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{ForceColors: true, FullTimestamp: true})

	cfg := DownloaderConfig{}
	cfg.PackageType, _ = cmd.Flags().GetString("type")
	cfg.OutputDir, _ = cmd.Flags().GetString("output-dir")
	cfg.NumWorkers, _ = cmd.Flags().GetInt("workers")
	cfg.ManifestPath, _ = cmd.Flags().GetString("manifest-path")
	cfg.Limit, _ = cmd.Flags().GetInt("limit")
	cfg.Verbose, _ = cmd.Flags().GetBool("verbose")
	cfg.LogPath, _ = cmd.Flags().GetString("log-path")

	logLevel := logrus.InfoLevel
	if cfg.Verbose {
		logLevel = logrus.DebugLevel
	}

	if err := setupLogger(logLevel, cfg.LogPath, cfg.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "critical: failed to setup logger: %v\n", err)
		return err
	}

	mainLogger := log.WithField("component", "main")
	mainLogger.Info("🚀 wordpress svn downloader started")

	if cfg.PackageType != "theme" && cfg.PackageType != "plugin" {
		return errors.New("--type must be 'theme' or 'plugin'")
	}

	if cfg.PackageType == "theme" {
		cfg.SvnRepoURL = "https://themes.svn.wordpress.org"
	} else {
		cfg.SvnRepoURL = "https://plugins.svn.wordpress.org"
	}

	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(".", "wp-content", cfg.PackageType+"s")
	}

	absOutputDir, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return errors.Wrapf(err, "failed to get absolute path for output directory: %s", cfg.OutputDir)
	}
	cfg.OutputDir = absOutputDir

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create output directory: %s", cfg.OutputDir)
	}
	mainLogger.Infof("📁 output directory: %s", cfg.OutputDir)

	if err := checkSVNInstalled(); err != nil {
		mainLogger.WithError(err).Error("❌ svn installation check failed")
		return err
	}
	mainLogger.Info("✅ svn command-line tool found")

	mainLogger.Infof("📄 using manifest file: %s", cfg.ManifestPath)
	manifest, err := loadDownloaderManifest(cfg.ManifestPath)
	if err != nil {
		mainLogger.WithError(err).Error("❌ failed to load or initialize downloader manifest")
		return err
	}

	if manifest.RepoType != "" && manifest.RepoType != cfg.PackageType {
		mainLogger.Warnf("⚠️ manifest was for '%s', current run is for '%s', package states might be irrelevant", manifest.RepoType, cfg.PackageType)
	}
	manifest.RepoType = cfg.PackageType
	manifest.SvnBaseURL = cfg.SvnRepoURL

	// Setup signal handling with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		mainLogger.Warnf("🛑 received signal %s, cancelling all operations immediately", sig)
		cancel()

		time.Sleep(2 * time.Second)
		mainLogger.Warn("🛑 force exiting after cleanup timeout")
		os.Exit(1)
	}()

	mainLogger.Info("📋 listing packages from svn, this might take a while...")
	allPackageNames, err := listSVNPackages(ctx, cfg.SvnRepoURL, cfg.Limit)
	if err != nil {
		if ctx.Err() == context.Canceled {
			mainLogger.Info("⚠️ svn listing cancelled by user")
			return nil
		}
		mainLogger.WithError(err).Error("❌ failed to list packages from svn")
		if saveErr := manifest.Save(); saveErr != nil {
			mainLogger.WithError(saveErr).Error("❌ failed to save manifest after svn list failure")
		}
		return err
	}

	if len(allPackageNames) == 0 {
		mainLogger.Info("ℹ️ no packages found in svn repository list")
		if saveErr := manifest.Save(); saveErr != nil {
			mainLogger.WithError(saveErr).Error("❌ failed to save manifest (no packages listed)")
		}
		return nil
	}

	if cfg.Limit > 0 && len(allPackageNames) > cfg.Limit {
		allPackageNames = allPackageNames[:cfg.Limit]
	}
	mainLogger.Infof("📊 total packages to consider: %d", len(allPackageNames))

	jobs := make(chan string, len(allPackageNames))
	var wg sync.WaitGroup

	numEffectiveWorkers := cfg.NumWorkers
	if len(allPackageNames) < numEffectiveWorkers {
		numEffectiveWorkers = len(allPackageNames)
	}
	if numEffectiveWorkers == 0 && len(allPackageNames) > 0 {
		numEffectiveWorkers = 1
	}

	mainLogger.Infof("👷 starting download process: %d packages with %d workers", len(allPackageNames), numEffectiveWorkers)

	// Start workers
	for i := 0; i < numEffectiveWorkers; i++ {
		wg.Add(1)
		go downloaderWorker(ctx, &cfg, manifest, jobs, &wg)
	}

	// Job feeder
	go func() {
		defer close(jobs)
		for _, pkgName := range allPackageNames {
			select {
			case jobs <- pkgName:
			case <-ctx.Done():
				mainLogger.Info("⚠️ context cancelled, stopping job feeding")
				return
			}
		}
	}()

	// Wait for completion or cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mainLogger.Info("✅ all workers finished")
	case <-ctx.Done():
		mainLogger.Warn("⚠️ operation cancelled, waiting for workers to finish current tasks...")
		<-done
		mainLogger.Info("✅ workers finished after cancellation")
	}

	mainLogger.Info("💾 saving final manifest...")
	if err := manifest.Save(); err != nil {
		mainLogger.WithError(err).Error("❌ failed to save final manifest")
		return err
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📄 detailed logs: stdout (text) + log file (json)")
	fmt.Println("📋 manifest files: svn-download-manifest.json")
	fmt.Println(strings.Repeat("=", 80))

	mainLogger.Info("🎉 download process completed")
	return nil
}

func validateConfig(config *DownloaderConfig) error {
	if config.LogPath != "" {
		absLogPath, err := filepath.Abs(config.LogPath)
		if err != nil {
			return fmt.Errorf("invalid log path: %w", err)
		}
		config.LogPath = absLogPath

		info, err := os.Stat(config.LogPath)
		if err == nil && info.IsDir() {
			config.LogPath = filepath.Join(config.LogPath, globalLogFileName)
		} else if os.IsNotExist(err) {
			parentDir := filepath.Dir(config.LogPath)
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				if mkErr := os.MkdirAll(parentDir, 0755); mkErr != nil {
					return fmt.Errorf("cannot create log directory %s: %w", parentDir, mkErr)
				}
			}
		}
	} else {
		config.LogPath = globalLogFileName
	}

	return nil
}

func init() {
	rootCmd.Flags().StringP("type", "t", "", "repository type: 'plugin' or 'theme' (required)")
	rootCmd.Flags().StringP("output-dir", "o", "", "directory to download packages into (default: ./wp-content/{type}s)")
	rootCmd.Flags().IntP("workers", "w", defaultDownloaderWorkers, "number of parallel download workers")
	rootCmd.Flags().StringP("manifest-path", "m", defaultDownloaderManifestFile, "path to the downloader manifest json file")
	rootCmd.Flags().IntP("limit", "l", 0, "limit the number of packages to process (0 for no limit)")
	rootCmd.Flags().BoolP("verbose", "v", false, "enable verbose (debug) logging")
	rootCmd.Flags().String("log-path", "", fmt.Sprintf("path to json log file (default: ./%s)", globalLogFileName))

	if err := rootCmd.MarkFlagRequired("type"); err != nil {
		fmt.Fprintf(os.Stderr, "error marking 'type' flag as required: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ failed to run command: %v\n", err)
		os.Exit(1)
	}
}
