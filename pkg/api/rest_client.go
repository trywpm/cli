package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go.wpm.so/cli/pkg/unsafeconv"
)

type RESTClient struct {
	client *http.Client
	host   string
}

func DefaultRESTClient() (*RESTClient, error) {
	return NewRESTClient(ClientOptions{})
}

func NewRESTClient(opts ClientOptions) (*RESTClient, error) {
	if optionsNeedResolution(opts) {
		var err error
		opts, err = resolveOptions(opts)
		if err != nil {
			return nil, err
		}
	}

	client, err := NewHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	host := strings.TrimRight(opts.Host, "/")

	return &RESTClient{
		client: client,
		host:   host,
	}, nil
}

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

func (c *RESTClient) DoWithContext(ctx context.Context, method, path string, body io.Reader, response any, opts ...RequestOption) error {
	url := restURL(c.host, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.client.Do(req)
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

	if resp.StatusCode == http.StatusNoContent || response == nil {
		return nil
	}

	switch v := response.(type) {
	case *string:
		var b []byte
		b, err = io.ReadAll(resp.Body)
		if err == nil {
			*v = unsafeconv.UnsafeBytesToString(b)
		}
		return err
	case *[]byte:
		var b []byte
		b, err = io.ReadAll(resp.Body)
		if err == nil {
			*v = b
		}
		return err
	default:
		return json.NewDecoder(resp.Body).Decode(v)
	}
}

func (c *RESTClient) RequestStream(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (io.ReadCloser, error) {
	url := restURL(c.host, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.client.Do(req)
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

// drainBody discards any unread bytes from body so the underlying TCP
// connection can be reused from the idle pool instead of being torn down.
//
// The drain is capped so a hostile or buggy server cannot keep us reading
// indefinitely.
func drainBody(body io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
}

func restURL(hostname, pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "https://") || strings.HasPrefix(pathOrURL, "http://") {
		return pathOrURL
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	if strings.HasPrefix(hostname, "https://") || strings.HasPrefix(hostname, "http://") {
		return strings.TrimRight(hostname, "/") + pathOrURL
	}
	return restPrefix(hostname) + pathOrURL
}

func restPrefix(hostname string) string {
	return "https://" + hostname
}
