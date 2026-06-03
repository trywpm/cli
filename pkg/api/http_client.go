package api

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/henvic/httpretty"
	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog"
	"golang.org/x/text/transform"

	"go.wpm.so/cli/pkg/asciisanitizer"
)

const (
	jsonContentType = "application/json; charset=utf-8"

	// headers
	HeaderEtag            = "ETag"
	HeaderSaveCache       = "X-Save-Cache"
	HeaderLocalCache      = "X-Local-Cache"
	HeaderIfNoneMatch     = "If-None-Match"
	HeaderLastModified    = "Last-Modified"
	HeaderIfModifiedSince = "If-Modified-Since"
	HeaderCacheRevalidate = "X-Cache-Revalidate"
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLength   = "Content-Length"
	HeaderContentType     = "Content-Type"
	HeaderCacheControl    = "Cache-Control"
	HeaderAccept          = "Accept"
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderAuthorization   = "Authorization"
	HeaderUserAgent       = "User-Agent"
	HeaderExpect          = "Expect"

	// header values
	CacheHit  = "HIT"
	CacheMiss = "MISS"

	encodingZstd = "zstd"
)

var (
	jsonTypeRE      = regexp.MustCompile(`[/+]json($|;)`)
	zstdDecoderPool = sync.Pool{
		New: func() any {
			d, err := zstd.NewReader(nil)
			if err != nil {
				panic(fmt.Sprintf("failed to create zstd reader: %v", err))
			}
			return d
		},
	}
)

func NewHTTPClient(opts ClientOptions) (*http.Client, error) {
	base, err := normalizeBaseURL(opts.Host)
	if err != nil {
		return nil, err
	}
	host := base.Hostname()

	allowToken := base.Scheme == "https" || isLoopbackHost(host)
	if opts.AuthToken != "" && !allowToken {
		return nil, fmt.Errorf("refusing to send auth token over http to %q: use https", base.Host)
	}

	// Sweep stale cache tmp files left behind by aborted requests. Runs in
	// the background so a slow filesystem doesn't delay the first request;
	// it's idempotent across concurrent invocations and safe to outlive the
	// process (the goroutine dies with the CLI either way).
	if opts.CacheDir != "" {
		go func() { _ = CleanupStale(opts.CacheDir) }()
	}

	transport := &Transport{
		Base: &http.Transport{
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
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
			DisableCompression:    true,
		},
		cacheDir: opts.CacheDir,
	}

	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}

	if !opts.SkipDefaultHeaders {
		resolveHeaders(opts.Headers)
	}

	var rt http.RoundTripper = transport

	rt = newHeaderRoundTripper(host, allowToken, opts.AuthToken, opts.Headers, rt)
	rt = newDecompressingRoundTripper(rt)
	rt = newSanitizerRoundTripper(rt)

	if opts.Log != nil && zerolog.GlobalLevel() == zerolog.DebugLevel {
		opts.LogVerboseHTTP = true
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
			return !inspectableMIMEType(h.Get(HeaderContentType)), nil
		})
		rt = logger.RoundTripper(rt)
	}

	return &http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if !isSameDomain(req.URL.Hostname(), host) {
				return fmt.Errorf("refusing redirect to %q outside registry host %q", req.URL.Host, host)
			}
			return nil
		},
	}, nil
}

func inspectableMIMEType(t string) bool {
	return jsonTypeRE.MatchString(t)
}

// normalizeBaseURL canonicalizes a configured registry host into a base URL
// with an explicit scheme. A bare host (no scheme) defaults to https so we
// never silently fall back to cleartext.
func normalizeBaseURL(host string) (*url.URL, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, errors.New("registry host not configured")
	}

	lc := strings.ToLower(host)
	if !strings.HasPrefix(lc, "http://") && !strings.HasPrefix(lc, "https://") {
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

type headerRoundTripper struct {
	headers    map[string]string
	host       string
	allowToken bool
	rt         http.RoundTripper
}

func resolveHeaders(headers map[string]string) {
	if _, ok := headers[HeaderContentType]; !ok {
		headers[HeaderContentType] = jsonContentType
	}
	if _, ok := headers[HeaderUserAgent]; !ok {
		headers[HeaderUserAgent] = "wpm-cli"
	}
	if _, ok := headers[HeaderAccept]; !ok {
		headers[HeaderAccept] = "application/json"
	}
}

func newHeaderRoundTripper(host string, allowToken bool, authToken string, headers map[string]string, rt http.RoundTripper) http.RoundTripper {
	if _, ok := headers[HeaderAuthorization]; !ok && authToken != "" {
		headers[HeaderAuthorization] = "Bearer " + authToken
	}
	if len(headers) == 0 {
		headers = nil
	}
	return headerRoundTripper{host: host, allowToken: allowToken, headers: headers, rt: rt}
}

func (hrt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqCopy := *req
	reqCopy.Header = req.Header.Clone()
	reqCopy.Header.Set(HeaderAcceptEncoding, encodingZstd)

	for k, v := range hrt.headers {
		if k == HeaderAuthorization {
			continue // handled separately below
		}
		if reqCopy.Header.Get(k) == "" {
			reqCopy.Header.Set(k, v)
		}
	}

	// The auth token only travels to the configured registry host over an
	// allowed transport. On anything else (cross-host redirect, plaintext to a
	// remote host) strip it, including any token a caller set per-request, so
	// the credential cannot leak.
	if hrt.allowToken && isSameDomain(reqCopy.URL.Hostname(), hrt.host) {
		if reqCopy.Header.Get(HeaderAuthorization) == "" {
			if token := hrt.headers[HeaderAuthorization]; token != "" {
				reqCopy.Header.Set(HeaderAuthorization, token)
			}
		}
	} else {
		reqCopy.Header.Del(HeaderAuthorization)
	}

	return hrt.rt.RoundTrip(&reqCopy)
}

type sanitizerRoundTripper struct {
	rt http.RoundTripper
}

func newSanitizerRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return sanitizerRoundTripper{rt: rt}
}

func (srt sanitizerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := srt.rt.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if !inspectableMIMEType(resp.Header.Get(HeaderContentType)) {
		return resp, nil
	}
	resp.Body = &wrappedBody{
		Reader: transform.NewReader(resp.Body, &asciisanitizer.Sanitizer{JSON: true}),
		Closer: resp.Body,
	}
	return resp, nil
}

type wrappedBody struct {
	io.Reader
	io.Closer
}

type decompressingRoundTripper struct {
	rt http.RoundTripper
}

func newDecompressingRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return &decompressingRoundTripper{rt: rt}
}

func (d decompressingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := d.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.Header.Get(HeaderContentEncoding) == encodingZstd {
		decoder := zstdDecoderPool.Get().(*zstd.Decoder)
		if err := decoder.Reset(resp.Body); err != nil {
			_ = resp.Body.Close()
			// Reset left the decoder in an unknown state hence we discard it instead of putting it back in the pool.
			// The next request needing a decoder will allocate a new one to replace it, so we don't leak resources by doing this.
			decoder.Close()
			return nil, fmt.Errorf("failed to reset zstd reader: %w", err)
		}

		resp.Body = &zstdReadCloser{
			Decoder:      decoder,
			OriginalBody: resp.Body,
		}
		resp.Header.Del(HeaderContentLength)
		resp.Header.Del(HeaderContentEncoding)
		resp.ContentLength = -1
	}

	return resp, nil
}

type zstdReadCloser struct {
	Decoder      *zstd.Decoder
	OriginalBody io.ReadCloser
}

func (z *zstdReadCloser) Read(p []byte) (n int, err error) {
	return z.Decoder.Read(p)
}

func (z *zstdReadCloser) Close() error {
	err := z.OriginalBody.Close()
	_ = z.Decoder.Reset(nil)
	zstdDecoderPool.Put(z.Decoder)
	return err
}
