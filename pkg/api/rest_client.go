package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RESTClient wraps methods for the different types of
// API requests that are supported by the server.
type RESTClient struct {
	client *http.Client
	host   string
}

func DefaultRESTClient() (*RESTClient, error) {
	return NewRESTClient(ClientOptions{})
}

// RESTClient builds a client to send requests to wpm REST API endpoints.
// As part of the configuration a hostname, auth token, default set of headers,
// and unix domain socket are resolved from the gh environment configuration.
// These behaviors can be overridden using the opts argument.
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

	return &RESTClient{
		client: client,
		host:   opts.Host,
	}, nil
}

// RequestOption is a function that can modify an http.Request.
type RequestOption func(*http.Request)

// WithHeader returns a RequestOption that adds a header to the request.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}

// DoWithContext issues a request with type specified by method to the
// specified path with the specified body.
// The response is populated into the response argument.
func (c *RESTClient) DoWithContext(ctx context.Context, method string, path string, body io.Reader, response any, opts ...RequestOption) error {
	url := restURL(c.host, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	// Set any additional headers from options
	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		defer resp.Body.Close()
		return HandleHTTPError(resp)
	}

	if resp.StatusCode == http.StatusNoContent || response == nil {
		return nil
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if s, ok := response.(*string); ok {
		*s = string(b)
		return nil
	}

	if bs, ok := response.(*[]byte); ok {
		*bs = b
		return nil
	}

	err = json.Unmarshal(b, &response)
	if err != nil {
		return err
	}

	return nil
}

// Do wraps DoWithContext with context.Background.
func (c *RESTClient) Do(method string, path string, body io.Reader, response interface{}, opts ...RequestOption) error {
	return c.DoWithContext(context.Background(), method, path, body, response, opts...)
}

// Delete issues a DELETE request to the specified path.
// The response is populated into the response argument.
func (c *RESTClient) Delete(path string, resp interface{}, opts ...RequestOption) error {
	return c.Do(http.MethodDelete, path, nil, resp, opts...)
}

// Get issues a GET request to the specified path.
// The response is populated into the response argument.
func (c *RESTClient) Get(path string, resp interface{}, opts ...RequestOption) error {
	return c.Do(http.MethodGet, path, nil, resp, opts...)
}

// Patch issues a PATCH request to the specified path with the specified body.
// The response is populated into the response argument.
func (c *RESTClient) Patch(path string, body io.Reader, resp interface{}, opts ...RequestOption) error {
	return c.Do(http.MethodPatch, path, body, resp, opts...)
}

// Post issues a POST request to the specified path with the specified body.
// The response is populated into the response argument.
func (c *RESTClient) Post(path string, body io.Reader, resp interface{}, opts ...RequestOption) error {
	return c.Do(http.MethodPost, path, body, resp, opts...)
}

// Put issues a PUT request to the specified path with the specified body.
// The response is populated into the response argument.
func (c *RESTClient) Put(path string, body io.Reader, resp interface{}, opts ...RequestOption) error {
	return c.Do(http.MethodPut, path, body, resp, opts...)
}

func restURL(hostname string, pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "https://") || strings.HasPrefix(pathOrURL, "http://") {
		return pathOrURL
	}
	return restPrefix(hostname) + pathOrURL
}

func restPrefix(hostname string) string {
	return fmt.Sprintf("https://%s", hostname)
}
