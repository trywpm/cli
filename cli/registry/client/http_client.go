package client

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"wpm/cli"
	"wpm/cli/command"
	"wpm/cli/config"
	"wpm/pkg/asciisanitizer"

	"github.com/henvic/httpretty"
	"github.com/moby/term"
	"github.com/sirupsen/logrus"
	"github.com/thlib/go-timezone-local/tzlocal"
	"golang.org/x/text/transform"
)

const (
	accept          = "Accept"
	wpm             = "wpm.so"
	localhost       = "wpm.local"
	timeZone        = "Time-Zone"
	userAgent       = "User-Agent"
	contentType     = "Content-Type"
	authorization   = "Authorization"
	jsonContentType = "application/json; charset=utf-8"
)

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

func DefaultHTTPClient(wpmCli command.Cli) (*http.Client, error) {
	return NewHTTPClient(wpmCli, ClientOptions{})
}

// NewHTTPClient creates a new HTTP client with the provided options.
func NewHTTPClient(wpmCli command.Cli, opts ClientOptions) (*http.Client, error) {
	if optionsNeedResolution(opts) {
		var err error
		opts, err = resolveOptions(wpmCli, opts)
		if err != nil {
			return nil, err
		}
	}

	transport := http.DefaultTransport
	transport = newSanitizerRoundTripper(transport)

	if opts.CacheDir == "" {
		opts.CacheDir = config.CacheDir()
	}

	if opts.EnableCache && opts.CacheTTL == 0 {
		opts.CacheTTL = time.Hour * 24
	}

	c := cache{dir: opts.CacheDir, ttl: opts.CacheTTL}
	transport = c.RoundTripper(transport)

	if opts.Log == nil && !opts.LogIgnoreEnv {
		if logrus.GetLevel() == logrus.DebugLevel {
			opts.Log = wpmCli.Err()
			opts.LogColorize = !cli.IsColorDisabled() && term.IsTerminal(wpmCli.Err().FD())
			opts.LogVerboseHTTP = true
		}
	}

	if opts.Log != nil {
		logger := &httpretty.Logger{
			Time:            true,
			TLS:             false,
			Colors:          opts.LogColorize,
			RequestHeader:   opts.LogVerboseHTTP,
			RequestBody:     opts.LogVerboseHTTP,
			ResponseHeader:  opts.LogVerboseHTTP,
			ResponseBody:    opts.LogVerboseHTTP,
			Formatters:      []httpretty.Formatter{&jsonFormatter{colorize: opts.LogColorize}},
			MaxResponseBody: 100000,
		}
		logger.SetOutput(opts.Log)
		logger.SetBodyFilter(func(h http.Header) (skip bool, err error) {
			return !inspectableMIMEType(h.Get(contentType)), nil
		})
		transport = logger.RoundTripper(transport)
	}

	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}
	if !opts.SkipDefaultHeaders {
		resolveHeaders(opts.Headers)
	}
	transport = newHeaderRoundTripper(opts.Host, opts.AuthToken, opts.Headers, transport)

	return &http.Client{Transport: transport, Timeout: opts.Timeout}, nil
}

func inspectableMIMEType(t string) bool {
	return jsonTypeRE.MatchString(t)
}

func isSameDomain(requestHost, domain string) bool {
	requestHost = strings.ToLower(requestHost)
	domain = strings.ToLower(domain)
	return (requestHost == domain) || strings.HasSuffix(requestHost, "."+domain)
}

type headerRoundTripper struct {
	headers map[string]string
	host    string
	rt      http.RoundTripper
}

func resolveHeaders(headers map[string]string) {
	if _, ok := headers[contentType]; !ok {
		headers[contentType] = jsonContentType
	}

	if _, ok := headers[userAgent]; !ok {
		headers[userAgent] = command.UserAgent()
	}

	if _, ok := headers[timeZone]; !ok {
		tz := currentTimeZone()
		if tz != "" {
			headers[timeZone] = tz
		}
	}

	if _, ok := headers[accept]; !ok {
		headers[accept] = "application/json"
	}
}

func newHeaderRoundTripper(host string, authToken string, headers map[string]string, rt http.RoundTripper) http.RoundTripper {
	if _, ok := headers[authorization]; !ok && authToken != "" {
		headers[authorization] = fmt.Sprintf("token %s", authToken)
	}
	if len(headers) == 0 {
		return rt
	}
	return headerRoundTripper{host: host, headers: headers, rt: rt}
}

func (hrt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range hrt.headers {
		// If the authorization header has been set and the request
		// host is not in the same domain that was specified in the ClientOptions
		// then do not add the authorization header to the request.
		if k == authorization && !isSameDomain(req.URL.Hostname(), hrt.host) {
			continue
		}

		// If the header is already set in the request, don't overwrite it.
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}

	return hrt.rt.RoundTrip(req)
}

type sanitizerRoundTripper struct {
	rt http.RoundTripper
}

func newSanitizerRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return sanitizerRoundTripper{rt: rt}
}

func (srt sanitizerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := srt.rt.RoundTrip(req)
	if err != nil || !jsonTypeRE.MatchString(resp.Header.Get(contentType)) {
		return resp, err
	}
	sanitizedReadCloser := struct {
		io.Reader
		io.Closer
	}{
		Reader: transform.NewReader(resp.Body, &asciisanitizer.Sanitizer{JSON: true}),
		Closer: resp.Body,
	}
	resp.Body = sanitizedReadCloser
	return resp, err
}

func currentTimeZone() string {
	tz, err := tzlocal.RuntimeTZ()
	if err != nil {
		return ""
	}
	return tz
}
