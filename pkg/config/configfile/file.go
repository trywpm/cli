package configfile

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type UsersAuthConfig struct {
	AuthToken string `json:"authToken,omitempty"`
}

type PluginsAuthConfig struct {
	AuthToken string `json:"authToken,omitempty"`
}

// ConfigFile ~/.wpm/config.json file info
type ConfigFile struct {
	Filename         string                       `json:"-"` // Note: for internal use only
	AuthToken        string                       `json:"authToken,omitempty"`
	DefaultUser      string                       `json:"defaultUser,omitempty"`
	DefaultUId       string                       `json:"defaultUid,omitempty"`
	DefaultTId       string                       `json:"defaultTid,omitempty"`
	UsersAuthTokens  map[string]UsersAuthConfig   `json:"usersAuthTokens,omitempty"`
	PluginsAuthToken map[string]PluginsAuthConfig `json:"pluginsAuthToken,omitempty"`
}

// New initializes an empty configuration file for the given filename 'fn'
func New(fn string) *ConfigFile {
	return &ConfigFile{
		AuthToken:        "",
		DefaultUser:      "",
		Filename:         fn,
		UsersAuthTokens:  make(map[string]UsersAuthConfig),
		PluginsAuthToken: make(map[string]PluginsAuthConfig),
	}
}

// LoadFromReader reads the configuration data given and sets up the auth config
// information with given directory and populates the receiver object
func (configFile *ConfigFile) LoadFromReader(configData io.Reader) error {
	if err := json.NewDecoder(configData).Decode(configFile); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	var err error

	if configFile.AuthToken != "" {
		configFile.AuthToken, err = decodeToken(configFile.AuthToken)
		if err != nil {
			return err
		}
	}

	for user, userAuth := range configFile.UsersAuthTokens {
		if userAuth.AuthToken != "" {
			userAuth.AuthToken, err = decodeToken(userAuth.AuthToken)
			if err != nil {
				return err
			}
			configFile.UsersAuthTokens[user] = userAuth
		}
	}

	for plugin, pluginAuth := range configFile.PluginsAuthToken {
		if pluginAuth.AuthToken != "" {
			pluginAuth.AuthToken, err = decodeToken(pluginAuth.AuthToken)
			if err != nil {
				return err
			}
			configFile.PluginsAuthToken[plugin] = pluginAuth
		}
	}

	return nil
}

// ContainsAuth returns whether the AuthToken is set or not
func (configFile *ConfigFile) ContainsAuth() bool {
	return configFile.AuthToken != ""
}

// GetUsersAuthTokens returns the mapping of user to auth token
func (configFile *ConfigFile) GetUsersAuthTokens() map[string]UsersAuthConfig {
	if configFile.UsersAuthTokens == nil {
		configFile.UsersAuthTokens = make(map[string]UsersAuthConfig)
	}

	return configFile.UsersAuthTokens
}

// GetPluginsAuthTokens returns the mapping of plugin to auth token
func (configFile *ConfigFile) GetPluginsAuthTokens() map[string]PluginsAuthConfig {
	if configFile.PluginsAuthToken == nil {
		configFile.PluginsAuthToken = make(map[string]PluginsAuthConfig)
	}

	return configFile.PluginsAuthToken
}

// SaveToWriter encodes and writes out all the auth information to
// the given writer
func (configFile *ConfigFile) SaveToWriter(writer io.Writer) error {
	// Encode auth token
	if configFile.AuthToken != "" {
		configFile.AuthToken = encodeToken(configFile.AuthToken)
	}

	// Encode user auth tokens
	for user, userAuth := range configFile.UsersAuthTokens {
		if userAuth.AuthToken != "" {
			userAuth.AuthToken = encodeToken(userAuth.AuthToken)
			configFile.UsersAuthTokens[user] = userAuth
		}
	}

	// Encode plugin auth tokens
	for plugin, pluginAuth := range configFile.PluginsAuthToken {
		if pluginAuth.AuthToken != "" {
			pluginAuth.AuthToken = encodeToken(pluginAuth.AuthToken)
			configFile.PluginsAuthToken[plugin] = pluginAuth
		}
	}

	data, err := json.MarshalIndent(configFile, "", "\t")
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

// Save encodes and writes out all the authorization information
func (configFile *ConfigFile) Save() (retErr error) {
	if configFile.Filename == "" {
		return errors.Errorf("Can't save config with empty filename")
	}

	dir := filepath.Dir(configFile.Filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, filepath.Base(configFile.Filename))
	if err != nil {
		return err
	}
	defer func() {
		temp.Close()
		if retErr != nil {
			if err := os.Remove(temp.Name()); err != nil {
				logrus.WithError(err).WithField("file", temp.Name()).Debug("Error cleaning up temp file")
			}
		}
	}()

	err = configFile.SaveToWriter(temp)
	if err != nil {
		return err
	}

	if err := temp.Close(); err != nil {
		return errors.Wrap(err, "error closing temp file")
	}

	// Handle situation where the configfile is a symlink
	cfgFile := configFile.Filename
	if f, err := os.Readlink(cfgFile); err == nil {
		cfgFile = f
	}

	// Try copying the current config file (if any) ownership and permissions
	copyFilePermissions(cfgFile, temp.Name())
	return os.Rename(temp.Name(), cfgFile)
}

// encodeToken creates a base64 encoded string to containing auth token
func encodeToken(token string) string {
	if token == "" {
		return ""
	}

	return base64.StdEncoding.EncodeToString([]byte(token))
}

// decodeAuth decodes the base64 encoded string to get the auth token
func decodeToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetFilename returns the file name that this config file is based on.
func (configFile *ConfigFile) GetFilename() string {
	return configFile.Filename
}
