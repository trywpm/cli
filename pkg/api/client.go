package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/henvic/httpretty"
	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog"
	"golang.org/x/text/transform"

	"go.wpm.so/cli/pkg/asciisanitizer"
	"go.wpm.so/cli/pkg/jsonpretty"
)

const (
	HeaderAccept          = "Accept"
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderAuthorization   = "Authorization"
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLength   = "Content-Length"
	HeaderContentType     = "Content-Type"
	HeaderEtag            = "ETag"
	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderIfNoneMatch     = "If-None-Match"
	HeaderLastModified    = "Last-Modified"
	HeaderRetryAfter      = "Retry-After"
	HeaderUserAgent       = "User-Agent"

	jsonContentType = "application/json; charset=utf-8"
	encodingZstd    = "zstd"

	// maxResponseBodySize caps decoded response bodies so a hostile or buggy
	// server cannot exhaust memory. Streams are exempt because their
	// consumers enforce their own limits.
	maxResponseBodySize = 4 << 20

	retryAttempts = 3
	maxRetryAfter = 3 * time.Second
)

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

// errRedirectRefused marks the redirect policy refusal so the retry loop can
// tell a deliberate refusal from a transient network failure.
var errRedirectRefused = errors.New("refusing redirect")

// Options configures a Client.
type Options struct {
	// Host is the registry this client talks to. A bare host defaults to
	// https, and plain http is allowed only for loopback addresses.
	Host string

	// AuthToken, when set, is sent as a bearer token to Host and never
	// anywhere else.
	AuthToken string

	// UserAgent overrides the default User-Agent header.
	UserAgent string

	// Log enables request logging to the given writer when the global log
	// level is debug.
	Log io.Writer

	// LogColorize renders the debug log with colors.
	LogColorize bool

	// CacheDir enables the manifest cache under the given directory. Empty
	// disables caching.
	CacheDir string
}

// Client is an HTTP client bound to a single registry host.
type Client struct {
	http *http.Client
	base *url.URL
}

// New returns a Client for the given registry.
func New(opts Options) (*Client, error) {
	base, err := normalizeBaseURL(opts.Host)
	if err != nil {
		return nil, err
	}
	host := base.Hostname()

	allowToken := base.Scheme == "https" || isLoopbackHost(host)
	if opts.AuthToken != "" && !allowToken {
		return nil, fmt.Errorf("refusing to send auth token over http to %q: use https", base.Host)
	}

	var rt http.RoundTripper = &decodeTransport{base: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
	}}
	if opts.Log != nil && zerolog.GlobalLevel() == zerolog.DebugLevel {
		rt = debugLogger(opts).RoundTripper(rt)
	}
	rt = &cacheTransport{base: rt, dir: opts.CacheDir}
	rt = &headerTransport{
		base:       rt,
		host:       host,
		allowToken: allowToken,
		authToken:  opts.AuthToken,
		userAgent:  opts.UserAgent,
	}

	return &Client{
		base: base,
		http: &http.Client{
			Transport: rt,
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if !isSameDomain(req.URL.Hostname(), host) {
					return fmt.Errorf("%w to %q outside registry host %q", errRedirectRefused, req.URL.Host, host)
				}
				return nil
			},
		},
	}, nil
}

// RequestOption mutates a request before it is sent.
type RequestOption func(*http.Request)

func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

func WithContentLength(length int64) RequestOption {
	return func(req *http.Request) {
		req.ContentLength = length
	}
}

// Do executes a request and decodes a 2xx response into out. A *string
// receives the raw body, nil discards it, and anything else is decoded as
// JSON. Any other status becomes an *HTTPError.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, out any, opts ...RequestOption) error {
	resp, err := c.send(ctx, method, path, body, opts...)
	if err != nil {
		return err
	}
	defer func() {
		drainBody(resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HandleHTTPError(resp)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		return nil
	}

	switch v := out.(type) {
	case *string:
		b, err := readAtMost(resp.Body, maxResponseBodySize)
		if err != nil {
			return err
		}
		*v = string(b)
		return nil
	default:
		return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodySize)).Decode(v)
	}
}

// Stream executes a request and returns the raw 2xx response body. The caller
// owns the body and must close it.
func (c *Client) Stream(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (io.ReadCloser, error) {
	resp, err := c.send(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() {
			drainBody(resp.Body)
			_ = resp.Body.Close()
		}()
		return nil, HandleHTTPError(resp)
	}
	return resp.Body, nil
}

// send executes the request, retrying GETs on transport errors and on
// 429/502/503/504. Only GETs retry because they carry no body to replay and
// every registry GET is idempotent.
func (c *Client) send(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (*http.Response, error) {
	if strings.Contains(path, "://") {
		return nil, fmt.Errorf("path %q must be relative, not an absolute URL", path)
	}

	attempts := 1
	if method == http.MethodGet && body == nil {
		attempts = retryAttempts
	}

	var lastErr error
	for attempt := range attempts {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt, lastErr)); err != nil {
				return nil, err
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.base.JoinPath(path).String(), body)
		if err != nil {
			return nil, err
		}
		for _, opt := range opts {
			opt(req)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, errRedirectRefused) {
				return nil, err
			}
			lastErr = err
			continue
		}

		if retriableStatus(resp.StatusCode) && attempt < attempts-1 {
			lastErr = &retryAfterError{status: resp.StatusCode, after: parseRetryAfter(resp.Header)}
			drainBody(resp.Body)
			_ = resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d attempts: %w", attempts, lastErr)
}

func retriableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

type retryAfterError struct {
	status int
	after  time.Duration
}

func (e *retryAfterError) Error() string {
	return "wpm registry error: " + strings.ToLower(http.StatusText(e.status))
}

func backoff(attempt int, lastErr error) time.Duration {
	d := time.Duration(attempt) * 250 * time.Millisecond
	d += rand.N(250 * time.Millisecond) //nolint:gosec // jitter needs no cryptographic randomness
	if raErr, ok := errors.AsType[*retryAfterError](lastErr); ok && raErr.after > d {
		d = min(raErr.after, maxRetryAfter)
	}
	return d
}

func parseRetryAfter(h http.Header) time.Duration {
	secs, err := strconv.Atoi(h.Get(HeaderRetryAfter))
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// headerTransport applies default headers and confines the auth token to the
// registry host over an allowed transport. Caller-set headers always win.
type headerTransport struct {
	base       http.RoundTripper
	host       string
	allowToken bool
	authToken  string
	userAgent  string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	setDefault(req, HeaderAccept, "application/json")
	setDefault(req, HeaderAcceptEncoding, encodingZstd)
	setDefault(req, HeaderContentType, jsonContentType)
	if t.userAgent != "" {
		setDefault(req, HeaderUserAgent, t.userAgent)
	}

	// The token travels only to the registry host, and only when New allowed
	// it (https or loopback). Anything else, including any token a caller set
	// per-request, is stripped so the credential cannot leak on a redirect.
	if t.allowToken && isSameDomain(req.URL.Hostname(), t.host) {
		if t.authToken != "" {
			setDefault(req, HeaderAuthorization, "Bearer "+t.authToken)
		}
	} else {
		req.Header.Del(HeaderAuthorization)
	}

	return t.base.RoundTrip(req)
}

func setDefault(req *http.Request, key, value string) {
	if req.Header.Get(key) == "" {
		req.Header.Set(key, value)
	}
}

// decodeTransport decompresses zstd bodies transparently and escapes control
// characters in JSON bodies before any consumer parses them.
type decodeTransport struct {
	base http.RoundTripper
}

var zstdDecoderPool = sync.Pool{
	New: func() any {
		d, err := zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true),
			zstd.WithDecoderMaxWindow(8<<20),
		)
		if err != nil {
			panic(fmt.Sprintf("failed to create zstd reader: %v", err))
		}
		return d
	},
}

func (t *decodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get(HeaderContentEncoding) == encodingZstd {
		decoder := zstdDecoderPool.Get().(*zstd.Decoder)
		if err := decoder.Reset(resp.Body); err != nil {
			_ = resp.Body.Close()
			// The decoder is in an unknown state, so drop it instead of pooling.
			decoder.Close()
			return nil, fmt.Errorf("failed to reset zstd reader: %w", err)
		}
		resp.Body = &zstdBody{decoder: decoder, original: resp.Body}
		resp.Header.Del(HeaderContentEncoding)
		resp.Header.Del(HeaderContentLength)
		resp.ContentLength = -1
	}

	if jsonTypeRE.MatchString(resp.Header.Get(HeaderContentType)) {
		resp.Body = &wrappedBody{
			Reader: transform.NewReader(resp.Body, &asciisanitizer.Sanitizer{JSON: true}),
			Closer: resp.Body,
		}
	}
	return resp, nil
}

type zstdBody struct {
	decoder  *zstd.Decoder
	original io.ReadCloser
}

func (z *zstdBody) Read(p []byte) (int, error) {
	return z.decoder.Read(p)
}

// Close is safe to call twice. A second Put of the same decoder would hand
// one instance to two goroutines.
func (z *zstdBody) Close() error {
	if z.decoder == nil {
		return nil
	}
	err := z.original.Close()
	_ = z.decoder.Reset(nil)
	zstdDecoderPool.Put(z.decoder)
	z.decoder = nil
	return err
}

type wrappedBody struct {
	io.Reader
	io.Closer
}

func debugLogger(opts Options) *httpretty.Logger {
	logger := &httpretty.Logger{
		Time:            true,
		Colors:          opts.LogColorize,
		RequestHeader:   true,
		RequestBody:     true,
		ResponseHeader:  true,
		ResponseBody:    true,
		Formatters:      []httpretty.Formatter{&jsonFormatter{colorize: opts.LogColorize}},
		MaxResponseBody: 100000,
	}
	logger.SetOutput(opts.Log)
	logger.SetBodyFilter(func(h http.Header) (bool, error) {
		return !jsonTypeRE.MatchString(h.Get(HeaderContentType)), nil
	})
	return logger
}

// normalizeBaseURL canonicalizes a configured registry host into a base URL
// with an explicit scheme. A bare host defaults to https so we never silently
// fall back to cleartext.
func normalizeBaseURL(host string) (*url.URL, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("registry host not configured")
	}

	if !strings.Contains(host, "://") {
		host = "https://" + host
	}

	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid registry host %q: %w", host, err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("invalid registry host %q: missing hostname", host)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported registry scheme %q, only http and https are supported", u.Scheme)
	}

	u.Fragment = ""
	u.RawQuery = ""
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// isLoopbackHost reports whether host refers to the local machine, where
// cleartext traffic never touches the network.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func isSameDomain(requestHost, domain string) bool {
	requestHost = strings.ToLower(requestHost)
	domain = strings.ToLower(domain)
	return (requestHost == domain) || strings.HasSuffix(requestHost, "."+domain)
}

// drainBody discards any unread bytes, bounded, so the connection returns to
// the idle pool instead of being torn down.
func drainBody(body io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
}

func readAtMost(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("response body exceeds %d byte limit", limit)
	}
	return b, nil
}

// jsonFormatter prettifies JSON payloads in the debug log.
type jsonFormatter struct {
	colorize bool
}

func (*jsonFormatter) Match(mediatype string) bool {
	return jsonTypeRE.MatchString(mediatype)
}

func (f *jsonFormatter) Format(w io.Writer, src []byte) error {
	return jsonpretty.Format(w, bytes.NewReader(src), "  ", f.colorize)
}
