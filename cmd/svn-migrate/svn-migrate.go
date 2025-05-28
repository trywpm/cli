package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultMaxWorkers = 5
	defaultTagTimeout = 5 * time.Minute
	requestTimeout    = 30 * time.Second
	manifestFileName  = "manifest.json"
	globalLogFileName = "wp-migration-activity.log"
	statusSuccess     = "success"
	statusFailed      = "failed"
	statusPending     = "pending"

	wpmOutputAlreadyExists       = "already exists"
	wpmErrorInitNoPluginMainFile = "no main plugin file with valid plugin headers"
	wpmErrorInitVersionMismatch  = "does not match specified version"
)

var (
	nameReg    = regexp.MustCompile(`^[\w-]{3,164}$`)
	httpClient = &http.Client{Timeout: requestTimeout}
	log        = logrus.New()
)

type Config struct {
	RepoPath   string
	RepoType   string
	MaxWorkers int
	TagTimeout time.Duration
	DryRun     bool
	Verbose    bool
	WpmPath    string
	LogPath    string
}

type PackageInfo struct {
	Name          string
	Type          string
	Path          string
	TagsPath      string
	SvnTags       []string
	LatestVersion string
}

type TagManifest struct {
	Status        string    `json:"status"`
	PublishedAt   time.Time `json:"published_at,omitempty"`
	PublishTag    string    `json:"publish_tag,omitempty"`
	Error         string    `json:"error,omitempty"`
	Retryable     bool      `json:"retryable"`
	LastAttempt   time.Time `json:"last_attempt,omitempty"`
	WpmInitLog    string    `json:"wpm_init_log,omitempty"`
	WpmPublishLog string    `json:"wpm_publish_log,omitempty"`
}

type PackageManifest struct {
	PackageName     string                 `json:"package_name"`
	Type            string                 `json:"type"`
	Qualified       bool                   `json:"qualified"`
	ApiLookupDone   bool                   `json:"api_lookup_done"`
	ApiError        string                 `json:"api_error,omitempty"`
	LatestWpVersion string                 `json:"latest_wp_version"`
	TotalSvnTags    int                    `json:"total_svn_tags"`
	LastSvnSync     time.Time              `json:"last_svn_sync"`
	LastUpdated     time.Time              `json:"last_updated"`
	Tags            map[string]TagManifest `json:"tags"`
	path            string                 `json:"-"`
}

type MigrationResult struct {
	PackageName             string
	IsQualified             bool
	QualificationReason     string
	SvnTagsCount            int
	TagsProcessedThisRun    int
	TagsSucceededThisRun    int
	TagsFailedThisRun       int
	TagsSkippedSuccess      int
	TagsSkippedNonRetryable int
	Error                   error
	Duration                time.Duration
}

type APIResponse struct {
	Version string `json:"version"`
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

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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

// normalizeVersion handles version strings with more than 3 dot-separated parts
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

func getManifestPath(packageRootPath string) string {
	return filepath.Join(packageRootPath, manifestFileName)
}

func loadManifest(manifestPath string) (*PackageManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, errors.Wrapf(err, "failed to read manifest: %s", manifestPath)
	}

	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal manifest: %s", manifestPath)
	}

	manifest.path = manifestPath
	if manifest.Tags == nil {
		manifest.Tags = make(map[string]TagManifest)
	}

	return &manifest, nil
}

func saveManifest(manifest *PackageManifest) error {
	manifest.LastUpdated = time.Now()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}

	if manifest.path == "" {
		return errors.New("manifest path is not set")
	}

	if err := os.MkdirAll(filepath.Dir(manifest.path), 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory for manifest: %s", manifest.path)
	}

	return os.WriteFile(manifest.path, data, 0644)
}

func fetchLatestVersion(ctx context.Context, packageName, repoType string) (version string, statusCode int, err error) {
	l := log.WithFields(logrus.Fields{"package": packageName, "action": "api_lookup"})

	var apiURL string
	if repoType == "theme" {
		apiURL = fmt.Sprintf("https://api.wordpress.org/themes/info/1.2/?action=theme_information&slug=%s", packageName)
	} else {
		apiURL = fmt.Sprintf("https://api.wordpress.org/plugins/info/1.2/?action=plugin_information&slug=%s", packageName)
	}

	l.Debugf("fetching from: %s", apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to create API request")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, errors.Wrap(err, "API request failed")
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

func getPackageSvnTags(tagsPath string) ([]string, error) {
	l := log.WithField("tags_path", tagsPath)
	entries, err := os.ReadDir(tagsPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read tags directory: %s", tagsPath)
	}

	var validRawTags []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			if strings.HasPrefix(dirName, ".") {
				l.Debugf("skipping dot-directory: %s", dirName)
				continue
			}

			if _, normErr := normalizeVersion(dirName); normErr == nil {
				validRawTags = append(validRawTags, dirName)
			} else {
				l.Debugf("skipping invalid version '%s': %v", dirName, normErr)
			}
		}
	}

	l.Debugf("found %d valid tag directories", len(validRawTags))
	return validRawTags, nil
}

func runWpmCommand(ctx context.Context, l *logrus.Entry, wpmPath string, args []string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, wpmPath, args...)
	cmd.Dir = workDir

	commandStr := fmt.Sprintf("%s %s", wpmPath, strings.Join(args, " "))
	l.WithFields(logrus.Fields{
		"command": commandStr,
		"workdir": workDir,
	}).Info("🔧 executing wpm command")

	outputBytes, err := cmd.CombinedOutput()
	outputStr := string(outputBytes)

	l.WithField("output", outputStr).Debug("wpm command output")

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}

		l.WithFields(logrus.Fields{
			"exit_code": exitCode,
			"output":    outputStr,
		}).WithError(err).Error("❌ wpm command failed")

		if ctx.Err() == context.DeadlineExceeded {
			return outputStr, errors.New("wpm command timed out")
		}
		return outputStr, errors.Wrapf(err, "wpm command failed. output: %s", outputStr)
	}

	l.Info("✅ wpm command completed")
	return outputStr, nil
}

func migratePackage(ctx context.Context, pkgInfo *PackageInfo, manifest *PackageManifest, config *Config) *MigrationResult {
	start := time.Now()
	l := log.WithFields(logrus.Fields{
		"package": pkgInfo.Name,
		"type":    pkgInfo.Type,
		"action":  "migrate",
	})

	result := &MigrationResult{
		PackageName:         pkgInfo.Name,
		IsQualified:         manifest.Qualified,
		QualificationReason: manifest.ApiError,
		SvnTagsCount:        len(pkgInfo.SvnTags),
	}

	if manifest.Qualified {
		result.QualificationReason = "OK"
	}

	l.WithFields(logrus.Fields{
		"tags_path":      pkgInfo.TagsPath,
		"tags_count":     len(pkgInfo.SvnTags),
		"qualified":      manifest.Qualified,
		"api_done":       manifest.ApiLookupDone,
		"api_error":      manifest.ApiError,
		"latest_version": manifest.LatestWpVersion,
		"tag_timeout":    config.TagTimeout.Seconds(),
		"dry_run":        config.DryRun,
	}).Info("🚀 starting package migration")

	if !manifest.Qualified && manifest.ApiLookupDone {
		l.Infof("⏭️  package not qualified (%s). skipping tags.", manifest.ApiError)
		if err := saveManifest(manifest); err != nil {
			l.WithError(err).Error("failed to save manifest for non-qualified package")
			result.Error = errors.Wrapf(err, "failed to save manifest for %s", pkgInfo.Name)
		}
		result.Duration = time.Since(start)
		return result
	}

	for i, rawVersionName := range pkgInfo.SvnTags {
		tagLogger := l.WithFields(logrus.Fields{
			"tag":   rawVersionName,
			"index": fmt.Sprintf("%d/%d", i+1, len(pkgInfo.SvnTags)),
		})

		tagLogger.Info("🏷️  processing tag")

		tagManifestEntry, entryExists := manifest.Tags[rawVersionName]

		if entryExists {
			if tagManifestEntry.Status == statusSuccess {
				tagLogger.Info("✅ already migrated successfully")
				result.TagsSkippedSuccess++
				continue
			}
			if tagManifestEntry.Status == statusFailed && !tagManifestEntry.Retryable {
				tagLogger.WithField("error", tagManifestEntry.Error).Info("⏭️  failed previously (non-retryable)")
				result.TagsSkippedNonRetryable++
				continue
			}
		} else {
			tagManifestEntry = TagManifest{Status: statusPending}
		}

		result.TagsProcessedThisRun++
		tagManifestEntry.LastAttempt = time.Now()
		tagPath := filepath.Join(pkgInfo.TagsPath, rawVersionName)

		if _, statErr := os.Stat(tagPath); os.IsNotExist(statErr) {
			errMsg := fmt.Sprintf("tag directory not found: %s", tagPath)
			tagLogger.WithField("tag_path", tagPath).Error("❌ " + errMsg)
			tagManifestEntry.Status = statusFailed
			tagManifestEntry.Error = errMsg
			tagManifestEntry.Retryable = false
			manifest.Tags[rawVersionName] = tagManifestEntry
			result.TagsFailedThisRun++

			if !config.DryRun {
				if err := saveManifest(manifest); err != nil {
					tagLogger.WithError(err).Error("failed to save manifest")
				}
			}
			continue
		}

		versionForWpmCommand := rawVersionName

		if config.DryRun {
			tagLogger.WithField("wpm_version", versionForWpmCommand).Info("🔍 dry run: would migrate")
			continue
		}

		tagCtx, cancelTag := context.WithTimeout(ctx, config.TagTimeout)

		// wpm init
		initArgs := []string{"--cwd", tagPath, "init", "--migrate", "--name", pkgInfo.Name, "--version", versionForWpmCommand}
		initOutput, initErr := runWpmCommand(tagCtx, tagLogger.WithField("cmd", "init"), config.WpmPath, initArgs, tagPath)
		tagManifestEntry.WpmInitLog = initOutput

		if initErr != nil {
			errStr := initErr.Error()
			if strings.Contains(initOutput, wpmOutputAlreadyExists) || strings.Contains(errStr, wpmOutputAlreadyExists) {
				tagLogger.Info("✅ wpm init: already exists, continuing")
				tagManifestEntry.Error = ""
			} else {
				tagManifestEntry.Error = fmt.Sprintf("init failed: %s", errStr)
				if strings.Contains(errStr, wpmErrorInitNoPluginMainFile) || strings.Contains(errStr, wpmErrorInitVersionMismatch) {
					tagLogger.WithError(initErr).Error("❌ wpm init failed (non-retryable)")
					tagManifestEntry.Status = statusFailed
					tagManifestEntry.Retryable = false
				} else {
					tagLogger.WithError(initErr).Error("❌ wpm init failed (retryable)")
					tagManifestEntry.Status = statusFailed
					tagManifestEntry.Retryable = true
				}
				cancelTag()
				manifest.Tags[rawVersionName] = tagManifestEntry
				result.TagsFailedThisRun++
				if err := saveManifest(manifest); err != nil {
					tagLogger.WithError(err).Error("failed to save manifest after init fail")
				}
				continue
			}
		} else {
			tagLogger.Info("✅ wpm init successful")
			tagManifestEntry.Error = ""
		}

		// wpm publish
		publishCmdTag := "untagged"
		if rawVersionName == manifest.LatestWpVersion {
			publishCmdTag = "latest"
		}
		tagManifestEntry.PublishTag = publishCmdTag

		publishArgs := []string{"--cwd", tagPath, "publish", "--access", "public", "--tag", publishCmdTag}
		publishOutput, publishErr := runWpmCommand(tagCtx, tagLogger.WithField("cmd", "publish"), config.WpmPath, publishArgs, tagPath)
		tagManifestEntry.WpmPublishLog = publishOutput

		if publishErr != nil {
			errStr := publishErr.Error()
			tagManifestEntry.Error = fmt.Sprintf("publish failed: %s", errStr)
			if strings.Contains(publishOutput, wpmOutputAlreadyExists) || strings.Contains(errStr, wpmOutputAlreadyExists) {
				tagLogger.Info("✅ wpm publish: already published")
				tagManifestEntry.Status = statusSuccess
				tagManifestEntry.PublishedAt = time.Now()
				tagManifestEntry.Error = ""
				tagManifestEntry.Retryable = false
			} else {
				tagLogger.WithError(publishErr).Error("❌ wpm publish failed (retryable)")
				tagManifestEntry.Status = statusFailed
				tagManifestEntry.Retryable = true
			}
		} else {
			tagLogger.WithField("publish_tag", publishCmdTag).Info("🎉 successfully migrated")
			tagManifestEntry.Status = statusSuccess
			tagManifestEntry.PublishedAt = time.Now()
			tagManifestEntry.Error = ""
			tagManifestEntry.Retryable = false
		}

		cancelTag()

		manifest.Tags[rawVersionName] = tagManifestEntry
		if tagManifestEntry.Status == statusSuccess {
			result.TagsSucceededThisRun++
		} else if tagManifestEntry.Status == statusFailed {
			result.TagsFailedThisRun++
		}

		if err := saveManifest(manifest); err != nil {
			errMsg := fmt.Sprintf("critical: failed to save manifest after processing %s", rawVersionName)
			tagLogger.WithError(err).Error(errMsg)
			result.Error = errors.New(errMsg)
			result.Duration = time.Since(start)
			return result
		}

		tagLogger.Info("✅ finished processing tag")
	}

	l.WithFields(logrus.Fields{
		"total_tags":     result.SvnTagsCount,
		"attempted":      result.TagsProcessedThisRun,
		"succeeded":      result.TagsSucceededThisRun,
		"failed":         result.TagsFailedThisRun,
		"skipped_ok":     result.TagsSkippedSuccess,
		"skipped_nonret": result.TagsSkippedNonRetryable,
		"duration":       time.Since(start).Seconds(),
	}).Info("📊 package migration summary")

	if err := saveManifest(manifest); err != nil {
		l.WithError(err).Error("failed to save manifest at end")
		if result.Error == nil {
			result.Error = errors.Wrap(err, "final manifest save failed")
		}
	}

	result.Duration = time.Since(start)
	return result
}

func processPackage(ctx context.Context, svnRepoPath, repoType, packageName string, config *Config) (*PackageInfo, *PackageManifest, error) {
	l := log.WithFields(logrus.Fields{
		"package": packageName,
		"type":    repoType,
		"action":  "process",
	})

	l.Info("📦 starting package processing")

	packageRootPath := filepath.Join(svnRepoPath, packageName)
	var svnTagsDir string
	if repoType == "theme" {
		svnTagsDir = packageRootPath
	} else {
		svnTagsDir = filepath.Join(packageRootPath, "tags")
	}

	if _, err := os.Stat(packageRootPath); os.IsNotExist(err) {
		l.WithError(err).Error("❌ package directory not found")
		return nil, nil, fmt.Errorf("package directory not found: %s", packageRootPath)
	}

	if _, err := os.Stat(svnTagsDir); os.IsNotExist(err) {
		if repoType == "plugin" {
			l.WithField("tags_dir", svnTagsDir).Error("❌ plugin tags directory missing")
			manifestPath := getManifestPath(packageRootPath)
			manifest := &PackageManifest{
				PackageName:   packageName,
				Type:          repoType,
				Qualified:     false,
				ApiLookupDone: true,
				ApiError:      fmt.Sprintf("plugin tags directory missing: %s", svnTagsDir),
				Tags:          make(map[string]TagManifest),
				path:          manifestPath,
			}
			if errSave := saveManifest(manifest); errSave != nil {
				l.WithError(errSave).Error("failed to save manifest")
				return nil, nil, errors.Wrapf(errSave, "failed to save manifest for %s", packageName)
			}
			pkgInfo := &PackageInfo{Name: packageName, Type: repoType, Path: packageRootPath, TagsPath: svnTagsDir, SvnTags: []string{}}
			return pkgInfo, manifest, nil
		}
	}

	manifestPath := getManifestPath(packageRootPath)
	manifest, err := loadManifest(manifestPath)
	previousSvnTagCount := 0

	if err != nil {
		if !os.IsNotExist(err) {
			l.WithError(err).Error("❌ error loading manifest")
			return nil, nil, errors.Wrapf(err, "error loading manifest for %s", packageName)
		}

		l.Info("📝 creating new manifest")
		manifest = &PackageManifest{
			PackageName: packageName,
			Type:        repoType,
			Tags:        make(map[string]TagManifest),
			path:        manifestPath,
		}
	} else {
		l.Debug("✅ manifest loaded")
		previousSvnTagCount = manifest.TotalSvnTags
		manifest.Type = repoType
	}

	currentSvnTags, err := getPackageSvnTags(svnTagsDir)
	if err != nil {
		l.WithError(err).WithField("tags_dir", svnTagsDir).Error("❌ failed to get svn tags")
		return nil, nil, errors.Wrapf(err, "failed to get svn tags for %s from %s", packageName, svnTagsDir)
	}

	manifest.TotalSvnTags = len(currentSvnTags)
	manifest.LastSvnSync = time.Now()
	l.Debugf("found %d valid svn tags (previous: %d)", manifest.TotalSvnTags, previousSvnTagCount)

	shouldFetchApi := false
	if !manifest.ApiLookupDone {
		shouldFetchApi = true
		l.Debug("api lookup needed")
	} else if manifest.Qualified && previousSvnTagCount != manifest.TotalSvnTags {
		shouldFetchApi = true
		l.Debug("refreshing api due to tag count change")
	} else if !manifest.Qualified && manifest.ApiLookupDone &&
		!(strings.Contains(manifest.ApiError, "(404)") || strings.Contains(manifest.ApiError, "plugin tags directory missing")) {
		l.Debugf("retrying api lookup: %s", manifest.ApiError)
		shouldFetchApi = true
	}

	if shouldFetchApi && !config.DryRun {
		l.Info("🔍 fetching latest version from wordpress api")
		latestVersion, statusCode, apiErr := fetchLatestVersion(ctx, packageName, repoType)
		manifest.ApiLookupDone = true

		if apiErr != nil {
			errMsg := apiErr.Error()
			manifest.ApiError = errMsg
			manifest.Qualified = false
			manifest.LatestWpVersion = ""

			if statusCode == http.StatusNotFound {
				manifest.ApiError = fmt.Sprintf("package not found on wordpress.org (%d)", statusCode)
				l.Info("❌ package not found on wordpress.org (404)")
			} else {
				l.WithField("status", statusCode).WithError(apiErr).Warn("⚠️  api error - marking as not qualified")
			}
		} else {
			manifest.LatestWpVersion = latestVersion
			manifest.Qualified = true
			manifest.ApiError = ""
			l.Infof("✅ wordpress api latest version: %s", latestVersion)
		}

		if errSave := saveManifest(manifest); errSave != nil {
			l.WithError(errSave).Error("failed to save manifest after api call")
			return nil, nil, errors.Wrapf(errSave, "failed to save manifest for %s after api call", packageName)
		}
	} else if config.DryRun && shouldFetchApi {
		l.Info("🔍 dry run: would fetch from wordpress api")
	}

	pkgInfo := &PackageInfo{
		Name:          packageName,
		Type:          repoType,
		Path:          packageRootPath,
		TagsPath:      svnTagsDir,
		SvnTags:       currentSvnTags,
		LatestVersion: manifest.LatestWpVersion,
	}

	if !shouldFetchApi || config.DryRun {
		if errSave := saveManifest(manifest); errSave != nil {
			l.WithError(errSave).Error("failed to save manifest")
			return nil, nil, errors.Wrapf(errSave, "failed to save manifest for %s", packageName)
		}
	}

	l.Info("✅ finished package processing")
	return pkgInfo, manifest, nil
}

func worker(ctx context.Context, jobs <-chan string, results chan<- *MigrationResult, svnRepoPath string, config *Config, wg *sync.WaitGroup) {
	defer wg.Done()

	for packageName := range jobs {
		workerLogger := log.WithField("worker_package", packageName)

		select {
		case <-ctx.Done():
			workerLogger.WithError(ctx.Err()).Info("⚠️  worker context cancelled")
			results <- &MigrationResult{PackageName: packageName, Error: ctx.Err()}
			return
		default:
			workerLogger.Info("👷 worker processing package")

			pkgInfo, manifest, err := processPackage(ctx, svnRepoPath, config.RepoType, packageName, config)
			if err != nil {
				workerLogger.WithError(err).Error("❌ critical error during package processing")
				results <- &MigrationResult{PackageName: packageName, Error: err}
				continue
			}

			if config.DryRun {
				workerLogger.WithFields(logrus.Fields{
					"qualified":      manifest.Qualified,
					"api_done":       manifest.ApiLookupDone,
					"api_error":      manifest.ApiError,
					"valid_tags":     len(pkgInfo.SvnTags),
					"latest_version": manifest.LatestWpVersion,
				}).Info("🔍 dry run: package processed")

				results <- &MigrationResult{
					PackageName:         packageName,
					IsQualified:         manifest.Qualified,
					QualificationReason: manifest.ApiError,
					SvnTagsCount:        len(pkgInfo.SvnTags),
				}
				continue
			}

			migrationResult := migratePackage(ctx, pkgInfo, manifest, config)
			results <- migrationResult
			workerLogger.Info("✅ worker finished processing package")
		}
	}
}

var rootCmd = &cobra.Command{
	Use:           "svn-to-wpm",
	Short:         "svn to wpm migration tool",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var migrateCmd = &cobra.Command{
	Use:   "migrate [repository-path]",
	Short: "migrate from svn to wpm",
	Args:  cobra.ExactArgs(1),
	RunE:  runMigrate,
}

func runMigrate(cmd *cobra.Command, args []string) error {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{ForceColors: true, FullTimestamp: true})

	config := &Config{}
	config.RepoPath = args[0]
	config.RepoType, _ = cmd.Flags().GetString("type")
	config.MaxWorkers, _ = cmd.Flags().GetInt("workers")
	config.TagTimeout, _ = cmd.Flags().GetDuration("tag-timeout")
	config.DryRun, _ = cmd.Flags().GetBool("dry-run")
	config.Verbose, _ = cmd.Flags().GetBool("verbose")
	config.WpmPath, _ = cmd.Flags().GetString("wpm-path")
	config.LogPath, _ = cmd.Flags().GetString("log-path")

	logLevel := logrus.InfoLevel
	if config.Verbose {
		logLevel = logrus.DebugLevel
	}

	if err := setupLogger(logLevel, config.LogPath, config.Verbose); err != nil {
		fmt.Fprintf(os.Stderr, "critical: failed to setup logger: %v\n", err)
		return err
	}

	mainLogger := log.WithField("component", "main")
	mainLogger.Info("🚀 wordpress migration tool started")

	mainLogger.WithFields(logrus.Fields{
		"repo_path": config.RepoPath,
		"repo_type": config.RepoType,
		"workers":   config.MaxWorkers,
		"timeout":   config.TagTimeout,
		"dry_run":   config.DryRun,
		"verbose":   config.Verbose,
		"wpm_path":  config.WpmPath,
		"log_path":  config.LogPath,
	}).Debug("configuration loaded")

	if err := validateConfig(config); err != nil {
		mainLogger.WithError(err).Error("❌ invalid configuration")
		return err
	}

	repoDirEntries, err := os.ReadDir(config.RepoPath)
	if err != nil {
		mainLogger.WithError(err).Errorf("❌ failed to read repository directory: %s", config.RepoPath)
		return fmt.Errorf("failed to read repository directory %s: %w", config.RepoPath, err)
	}

	var validPackageNames []string
	var skippedNames []string

	for _, entry := range repoDirEntries {
		if entry.IsDir() {
			name := entry.Name()
			if !nameReg.MatchString(name) {
				skippedNames = append(skippedNames, name)
				continue
			}
			validPackageNames = append(validPackageNames, name)
		}
	}

	if len(skippedNames) > 0 {
		mainLogger.WithFields(logrus.Fields{
			"count":        len(skippedNames),
			"skipped_dirs": strings.Join(skippedNames, ", "),
			"name_regex":   nameReg.String(),
		}).Warn("⚠️  skipped directories with invalid package names")
	}

	if len(validPackageNames) == 0 {
		mainLogger.Warn("⚠️  no valid package directories found")
		return nil
	}

	mainLogger.Infof("📁 found %d valid %s package(s)", len(validPackageNames), config.RepoType)

	if config.DryRun {
		mainLogger.Info("🔍 dry run mode enabled - no changes will be made")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan string, len(validPackageNames))
	resultsChan := make(chan *MigrationResult, len(validPackageNames))
	var wg sync.WaitGroup

	numWorkers := config.MaxWorkers
	if len(validPackageNames) < numWorkers {
		numWorkers = len(validPackageNames)
	}

	mainLogger.Infof("👷 starting migration: %d packages, %d workers, %v timeout per tag",
		len(validPackageNames), numWorkers, config.TagTimeout)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, jobs, resultsChan, config.RepoPath, config, &wg)
	}

	// Feed jobs
	go func() {
		defer close(jobs)
		for _, name := range validPackageNames {
			select {
			case jobs <- name:
			case <-ctx.Done():
				mainLogger.Info("⚠️  job feeding cancelled")
				return
			}
		}
	}()

	// Close results when all workers done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var (
		totalPackagesProcessed, pkgsFullSuccess, pkgsPartialSuccess,
		pkgsWithTagFails, pkgsNotQualified, pkgsCriticalError int
		totalSvnTagsScanned, totalTagsAttemptedRun,
		totalTagsSucceededRun, totalTagsFailedRun int
		problematicPackages []string
	)

	overallStart := time.Now()

	for res := range resultsChan {
		totalPackagesProcessed++
		resLogger := mainLogger.WithField("result_package", res.PackageName)

		if res.Error != nil {
			resLogger.WithError(res.Error).Errorf("💥 critical error (%.2fs)", res.Duration.Seconds())
			pkgsCriticalError++
			problematicPackages = append(problematicPackages,
				fmt.Sprintf("%s (critical error: %v)", res.PackageName, res.Error))
			continue
		}

		if config.DryRun {
			if !res.IsQualified && res.QualificationReason != "OK" && res.QualificationReason != "" {
				pkgsNotQualified++
			}
			continue
		}

		totalSvnTagsScanned += res.SvnTagsCount
		totalTagsAttemptedRun += res.TagsProcessedThisRun
		totalTagsSucceededRun += res.TagsSucceededThisRun
		totalTagsFailedRun += res.TagsFailedThisRun

		baseFields := logrus.Fields{
			"duration":     fmt.Sprintf("%.2fs", res.Duration.Seconds()),
			"tags_total":   res.SvnTagsCount,
			"tags_tried":   res.TagsProcessedThisRun,
			"tags_success": res.TagsSucceededThisRun,
			"tags_failed":  res.TagsFailedThisRun,
			"tags_skip_ok": res.TagsSkippedSuccess,
			"tags_skip_nr": res.TagsSkippedNonRetryable,
		}

		if !res.IsQualified {
			resLogger.WithFields(baseFields).WithField("reason", res.QualificationReason).
				Warn("⚠️  package not qualified")
			pkgsNotQualified++
		} else if res.SvnTagsCount == 0 {
			resLogger.WithFields(baseFields).Info("✅ package qualified, no svn tags")
			pkgsFullSuccess++
		} else if res.TagsProcessedThisRun == 0 {
			resLogger.WithFields(baseFields).Info("✅ all tags already processed")
			pkgsFullSuccess++
		} else if res.TagsSucceededThisRun > 0 && res.TagsFailedThisRun == 0 {
			resLogger.WithFields(baseFields).Info("🎉 all attempted tags migrated successfully")
			pkgsFullSuccess++
		} else if res.TagsSucceededThisRun > 0 && res.TagsFailedThisRun > 0 {
			resLogger.WithFields(baseFields).Warn("⚠️  partial success")
			pkgsPartialSuccess++
			problematicPackages = append(problematicPackages,
				fmt.Sprintf("%s (partial: %d✅ %d❌)", res.PackageName, res.TagsSucceededThisRun, res.TagsFailedThisRun))
		} else if res.TagsFailedThisRun > 0 && res.TagsSucceededThisRun == 0 {
			resLogger.WithFields(baseFields).Error("❌ all attempted tags failed")
			pkgsWithTagFails++
			problematicPackages = append(problematicPackages,
				fmt.Sprintf("%s (all %d tags failed)", res.PackageName, res.TagsProcessedThisRun))
		} else {
			resLogger.WithFields(baseFields).Error("❓ unhandled status")
			pkgsWithTagFails++
			problematicPackages = append(problematicPackages,
				fmt.Sprintf("%s (unhandled status)", res.PackageName))
		}
	}

	// Final Summary
	summaryFields := logrus.Fields{
		"total_duration":          fmt.Sprintf("%.2fs", time.Since(overallStart).Seconds()),
		"packages_found":          len(validPackageNames),
		"packages_processed":      totalPackagesProcessed,
		"packages_success":        pkgsFullSuccess,
		"packages_partial":        pkgsPartialSuccess,
		"packages_tag_fails":      pkgsWithTagFails,
		"packages_not_qualified":  pkgsNotQualified,
		"packages_critical_error": pkgsCriticalError,
		"tags_scanned":            totalSvnTagsScanned,
		"tags_attempted":          totalTagsAttemptedRun,
		"tags_succeeded":          totalTagsSucceededRun,
		"tags_failed":             totalTagsFailedRun,
	}

	if config.DryRun {
		mainLogger.WithFields(logrus.Fields{
			"duration":               fmt.Sprintf("%.2fs", time.Since(overallStart).Seconds()),
			"packages_found":         len(validPackageNames),
			"packages_processed":     totalPackagesProcessed,
			"packages_not_qualified": pkgsNotQualified,
		}).Info("🔍 dry run summary")
	} else {
		mainLogger.WithFields(summaryFields).Info("📊 migration summary")
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📄 detailed logs: stdout (text) + log file (json)")
	fmt.Println("📋 manifest files: [package-dir]/manifest.json")
	fmt.Println(strings.Repeat("=", 80))

	errorCount := pkgsPartialSuccess + pkgsWithTagFails + pkgsCriticalError
	if errorCount > 0 {
		mainLogger.WithField("error_count", errorCount).Error("❌ migration completed with issues")

		if len(problematicPackages) > 0 {
			fmt.Println("\n🚨 packages requiring attention:")
			for _, detail := range problematicPackages {
				fmt.Printf("   • %s\n", detail)
			}
		}

		return fmt.Errorf("%d package(s) had issues requiring attention", errorCount)
	}

	if config.DryRun {
		mainLogger.Info("🔍 dry run completed - no changes made")
	} else {
		mainLogger.Info("🎉 migration completed successfully")
	}

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

	if config.WpmPath == "" {
		foundPath, err := exec.LookPath("wpm")
		if err != nil {
			return errors.New("wpm command not found in PATH and --wpm-path not specified")
		}
		config.WpmPath = foundPath
	} else {
		absWpmPath, err := filepath.Abs(config.WpmPath)
		if err != nil {
			return fmt.Errorf("could not get absolute path for wpm binary %s: %w", config.WpmPath, err)
		}
		if _, err := os.Stat(absWpmPath); err != nil {
			return fmt.Errorf("wpm binary not found: %s", absWpmPath)
		}
		config.WpmPath = absWpmPath
	}

	if config.MaxWorkers <= 0 {
		config.MaxWorkers = defaultMaxWorkers
	}

	if config.TagTimeout <= 0 {
		config.TagTimeout = defaultTagTimeout
	}

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
	migrateCmd.Flags().StringP("type", "t", "", "repository type: 'plugin' or 'theme' (required)")
	migrateCmd.Flags().IntP("workers", "w", defaultMaxWorkers, "number of parallel workers")
	migrateCmd.Flags().Duration("tag-timeout", defaultTagTimeout, "timeout per tag migration")
	migrateCmd.Flags().Bool("dry-run", false, "simulate migration without making changes")
	migrateCmd.Flags().BoolP("verbose", "v", false, "enable verbose (debug) logging")
	migrateCmd.Flags().String("wpm-path", "", "path to wpm binary (if not in PATH)")
	migrateCmd.Flags().String("log-path", "", fmt.Sprintf("path to json log file (default: ./%s)", globalLogFileName))

	if err := migrateCmd.MarkFlagRequired("type"); err != nil {
		logrus.WithError(err).Fatal("internal error marking 'type' flag required")
	}

	rootCmd.AddCommand(migrateCmd)
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true, FullTimestamp: true})
	logrus.SetOutput(os.Stderr)

	if err := rootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("failed to execute command")
	}
}
