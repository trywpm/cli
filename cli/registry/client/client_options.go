package client

import (
	"fmt"
	"io"
	"time"
	"wpm/cli/command"
)

// ClientOptions holds available options to configure API clients.
type ClientOptions struct {
	// AuthToken is the authorization token that will be used
	// to authenticate against API endpoints.
	AuthToken string

	// CacheDir is the directory to use for cached API requests.
	// Default is the same directory that gh uses for caching.
	CacheDir string

	// CacheTTL is the time that cached API requests are valid for.
	// Default is 24 hours.
	CacheTTL time.Duration

	// EnableCache specifies if API requests will be cached or not.
	// Default is no caching.
	EnableCache bool

	// Headers are the headers that will be sent with every API request.
	// Default headers set are Accept, Content-Type, Time-Zone, and User-Agent.
	// Default headers will be overridden by keys specified in Headers.
	Headers map[string]string

	// Host is the default host that API requests will be sent to.
	Host string

	// Log specifies a writer to write API request logs to. Default is to respect the GH_DEBUG environment
	// variable, and no logging otherwise.
	Log io.Writer

	// LogIgnoreEnv disables respecting the GH_DEBUG environment variable. This can be useful in test mode
	// or when the extension already offers its own controls for logging to the user.
	LogIgnoreEnv bool

	// LogColorize enables colorized logging to Log for display in a terminal.
	// Default is no coloring.
	LogColorize bool

	// LogVerboseHTTP enables logging HTTP headers and bodies to Log.
	// Default is only logging request URLs and response statuses.
	LogVerboseHTTP bool

	// SkipDefaultHeaders disables setting of the default headers.
	SkipDefaultHeaders bool

	// Timeout specifies a time limit for each API request.
	// Default is no timeout.
	Timeout time.Duration
}

func optionsNeedResolution(opts ClientOptions) bool {
	if opts.Host == "" {
		return true
	}

	if opts.AuthToken == "" {
		return true
	}

	return false
}

func resolveOptions(wpmCli command.Cli, opts ClientOptions) (ClientOptions, error) {
	cfg := wpmCli.ConfigFile()

	if opts.Host == "" {
		opts.Host = wpmCli.Registry()
	}

	if opts.AuthToken == "" {
		opts.AuthToken = cfg.AuthToken
		if opts.AuthToken == "" {
			return ClientOptions{}, fmt.Errorf("bearer authentication token not found for host %s", opts.Host)
		}
	}

	return opts, nil
}
