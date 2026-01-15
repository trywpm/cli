package api

import (
	"fmt"
	"io"
	"time"
)

// ClientOptions holds available options to configure API clients.
type ClientOptions struct {
	// AuthToken is the authorization token that will be used
	// to authenticate against API endpoints.
	AuthToken string

	// Headers are the headers that will be sent with every API request.
	// Default headers set are Accept, Content-Type, Time-Zone, and User-Agent.
	// Default headers will be overridden by keys specified in Headers.
	Headers map[string]string

	// Host is the default host that API requests will be sent to.
	Host string

	// Log specifies a writer to write API request logs to.
	Log io.Writer

	// LogColorize enables colorized logging to Log for display in a terminal.
	// Default is no coloring.
	LogColorize bool

	// LogVerboseHTTP enables logging HTTP headers and bodies to Log.
	// Default is only logging request URLs and response statuses.
	// By default fallback to logrus log level.
	LogVerboseHTTP bool

	// SkipDefaultHeaders disables setting of the default headers.
	SkipDefaultHeaders bool

	// Timeout specifies a time limit for each API request.
	// Default is no timeout.
	Timeout time.Duration

	// CacheDir specifies a directory to use for caching GET requests.
	CacheDir string
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

func resolveOptions(opts ClientOptions) (ClientOptions, error) {
	if opts.Host == "" {
		return ClientOptions{}, fmt.Errorf("host not found")
	}

	return opts, nil
}
