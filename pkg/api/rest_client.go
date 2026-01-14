package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"wpm/pkg/unsafeconv"
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

func (c *RESTClient) DoWithContext(ctx context.Context, method string, path string, body io.Reader, response any, opts ...RequestOption) error {
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
	defer resp.Body.Close()

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

func (c *RESTClient) RequestStream(ctx context.Context, method string, path string, body io.Reader, opts ...RequestOption) (io.ReadCloser, error) {
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
		defer resp.Body.Close()
		return nil, HandleHTTPError(resp)
	}

	return resp.Body, nil
}

func (c *RESTClient) Do(method string, path string, body io.Reader, response any, opts ...RequestOption) error {
	return c.DoWithContext(context.Background(), method, path, body, response, opts...)
}

func (c *RESTClient) Delete(path string, resp any, opts ...RequestOption) error {
	return c.Do(http.MethodDelete, path, nil, resp, opts...)
}

func (c *RESTClient) Get(path string, resp any, opts ...RequestOption) error {
	return c.Do(http.MethodGet, path, nil, resp, opts...)
}

func (c *RESTClient) Patch(path string, body io.Reader, resp any, opts ...RequestOption) error {
	return c.Do(http.MethodPatch, path, body, resp, opts...)
}

func (c *RESTClient) Post(path string, body io.Reader, resp any, opts ...RequestOption) error {
	return c.Do(http.MethodPost, path, body, resp, opts...)
}

func (c *RESTClient) Put(path string, body io.Reader, resp any, opts ...RequestOption) error {
	return c.Do(http.MethodPut, path, body, resp, opts...)
}

func restURL(hostname, pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "https://") || strings.HasPrefix(pathOrURL, "http://") {
		return pathOrURL
	}
	if !strings.HasPrefix(pathOrURL, "/") {
		pathOrURL = "/" + pathOrURL
	}
	return restPrefix(hostname) + pathOrURL
}

func restPrefix(hostname string) string {
	return fmt.Sprintf("https://%s", hostname)
}
