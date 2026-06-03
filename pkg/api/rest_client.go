package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxResponseBodySize = 4 << 20 // 4 MiB

type RESTClient struct {
	client  *http.Client
	baseURL *url.URL
}

func NewRESTClient(opts ClientOptions) (*RESTClient, error) {
	base, err := normalizeBaseURL(opts.Host)
	if err != nil {
		return nil, err
	}

	client, err := NewHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	return &RESTClient{
		baseURL: base,
		client:  client,
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
	req, err := c.newRequest(ctx, method, path, body, opts...)
	if err != nil {
		return err
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
		b, rErr := readAtMost(resp.Body, maxResponseBodySize)
		if rErr != nil {
			return rErr
		}
		*v = string(b)
		return nil
	case *[]byte:
		b, rErr := readAtMost(resp.Body, maxResponseBodySize)
		if rErr != nil {
			return rErr
		}
		*v = b
		return nil
	default:
		return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodySize)).Decode(v)
	}
}

func (c *RESTClient) RequestStream(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
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

// readAtMost reads limited bytes and returns an error if the limit
// is exceeded, to prevent OOM from hostile or buggy servers.
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

func (c *RESTClient) newRequest(ctx context.Context, method, path string, body io.Reader, opts ...RequestOption) (*http.Request, error) {
	if strings.Contains(path, "://") {
		return nil, fmt.Errorf("path %q must be relative, not an absolute URL", path)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL.JoinPath(path).String(), body)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}
