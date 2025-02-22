package config

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"wpm/pkg/config/configfile"

	"github.com/pkg/errors"
)

const (
	// EnvOverrideConfigDir is the name of the environment variable that can be
	// used to override the location of the client configuration files (~/.wpm).
	//
	// It takes priority over the default, but can be overridden by the "--config"
	// command line option.
	EnvOverrideConfigDir = "WPM_CONFIG"

	// ConfigFileName is the name of the client configuration file inside the
	// config-directory.
	ConfigFileName = "config.json"
	configFileDir  = ".wpm"
)

var (
	initConfigDir = new(sync.Once)
	configDir     string
)

// getHomeDir returns the home directory of the current user with the help of
// environment variables depending on the target operating system.
// Returned path should be used with "path/filepath" to form new paths.
//
// On non-Windows platforms, it falls back to nss lookups, if the home
// directory cannot be obtained from environment-variables.
//
// If linking statically with cgo enabled against glibc, ensure the
// osusergo build tag is used.
//
// If needing to do nss lookups, do not disable cgo or set osusergo.
func getHomeDir() string {
	home, _ := os.UserHomeDir()
	if home == "" && runtime.GOOS != "windows" {
		if u, err := user.Current(); err == nil {
			return u.HomeDir
		}
	}
	return home
}

// Dir returns the directory the configuration file is stored in
func Dir() string {
	initConfigDir.Do(func() {
		configDir = os.Getenv(EnvOverrideConfigDir)
		if configDir == "" {
			configDir = filepath.Join(getHomeDir(), configFileDir)
		}
	})
	return configDir
}

// SetDir sets the directory the configuration file is stored in
func SetDir(dir string) {
	// trigger the sync.Once to synchronise with Dir()
	initConfigDir.Do(func() {})
	configDir = filepath.Clean(dir)
}

// Path returns the path to a file relative to the config dir
func Path(p ...string) (string, error) {
	path := filepath.Join(append([]string{Dir()}, p...)...)
	if !strings.HasPrefix(path, Dir()+string(filepath.Separator)) {
		return "", errors.Errorf("path %q is outside of root config directory %q", path, Dir())
	}
	return path, nil
}

// LoadFromReader is a convenience function that creates a ConfigFile object from
// a reader. It returns an error if configData is malformed.
func LoadFromReader(configData io.Reader) (*configfile.ConfigFile, error) {
	configFile := configfile.ConfigFile{
		AuthToken:        "",
		UsersAuthTokens:  make(map[string]configfile.UsersAuthConfig),
		PluginsAuthToken: make(map[string]configfile.PluginsAuthConfig),
	}
	err := configFile.LoadFromReader(configData)
	return &configFile, err
}

// Load reads the configuration file ([ConfigFileName]) from the given directory.
// If no directory is given, it uses the default [Dir]. A [*configfile.ConfigFile]
// is returned containing the contents of the configuration file, or a default
// struct if no configfile exists in the given location.
//
// Load returns an error if a configuration file exists in the given location,
// but cannot be read, or is malformed. Consumers must handle errors to prevent
// overwriting an existing configuration file.
func Load(configDir string) (*configfile.ConfigFile, error) {
	if configDir == "" {
		configDir = Dir()
	}
	return load(configDir)
}

func load(configDir string) (*configfile.ConfigFile, error) {
	filename := filepath.Join(configDir, ConfigFileName)
	configFile := configfile.New(filename)

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// It is OK for no configuration file to be present, in which
			// case we return a default struct.
			return configFile, nil
		}
		// Any other error happening when failing to read the file must be returned.
		return configFile, errors.Wrap(err, "loading config file")
	}
	defer file.Close()
	err = configFile.LoadFromReader(file)
	if err != nil {
		err = errors.Wrapf(err, "parsing config file (%s)", filename)
	}
	return configFile, err
}

// LoadDefaultConfigFile attempts to load the default config file and returns
// a reference to the ConfigFile struct. If none is found or when failing to load
// the configuration file, it initializes a default ConfigFile struct. If no
// credentials-store is set in the configuration file, it attempts to discover
// the default store to use for the current platform.
//
// Important: LoadDefaultConfigFile prints a warning to stderr when failing to
// load the configuration file, but otherwise ignores errors. Consumers should
// consider using [Load] (and [credentials.DetectDefaultStore]) to detect errors
// when updating the configuration file, to prevent discarding a (malformed)
// configuration file.
func LoadDefaultConfigFile(stderr io.Writer) *configfile.ConfigFile {
	configFile, err := load(Dir())
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "WARNING: Error", err)
	}

	return configFile
}

func CacheDir() string {
	return filepath.Join(Dir(), "cache")
}

func UserAuthCacheDir() string {
	return filepath.Join(Dir(), "cache", "auth")
}
