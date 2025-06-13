package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"wpm/pkg/api"
	"wpm/pkg/streams"
	wpmTerm "wpm/pkg/term"
	"wpm/pkg/wpm"

	"github.com/moby/term"
)

type client struct {
	restClient *api.RESTClient
}

// RegistryClient is a client used to communicate with a wpm distribution
// registry
type RegistryClient interface {
	PublishPackage(ctx context.Context, data *wpm.Package, requestId string) (string, error)
	UploadTarball(ctx context.Context, body io.Reader, opts UploadTarballOptions) (UploadTarballResponse, error)
}

var _ RegistryClient = &client{}

// NewRegistryClient returns a new REST client for the wpm registry
func NewRegistryClient(host string, authToken string, userAgent string, out *streams.Out) (RegistryClient, error) {
	opts := api.ClientOptions{
		Log:         out,
		Host:        host,
		AuthToken:   authToken,
		Headers:     map[string]string{"User-Agent": userAgent},
		LogColorize: !wpmTerm.IsColorDisabled() && term.IsTerminal(out.FD()),
	}

	_client, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, err
	}

	return &client{
		restClient: _client,
	}, nil
}

// UploadTarballOptions defines the options for uploading a tarball to the registry.
type UploadTarballOptions struct {
	Name    string // package name
	Acl     string // must be one of "public" or "private"
	Digest  string // base64 encoded digest of the package
	Version string // package version
	Type    string // package type, e.g., "theme", "plugin" or "mu-plugin"
}

// UploadTarballResponse defines the response structure for uploading a package.
type UploadTarballResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (c *client) UploadTarball(ctx context.Context, body io.Reader, opts UploadTarballOptions) (UploadTarballResponse, error) {
	response := UploadTarballResponse{}
	err := c.restClient.Put(
		fmt.Sprintf("/-/upload/%s/%s", opts.Name, opts.Version),
		body,
		&response,
		api.WithHeader("x-wpm-acl", opts.Acl),
		api.WithHeader("x-wpm-package-type", opts.Type),
		api.WithHeader("x-wpm-checksum-sha256", opts.Digest),
		api.WithHeader("Content-Type", "application/octet-stream"),
	)
	if err != nil {
		return UploadTarballResponse{}, err
	}

	return response, nil
}

// PublishPackage publishes a package to the registry.
// It requires the package data and a request ID from the upload tarball request.
func (c *client) PublishPackage(ctx context.Context, data *wpm.Package, requestId string) (string, error) {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	response := struct{ Message string }{}
	err = c.restClient.Put("/", bytes.NewReader(bodyBytes), &response, api.WithHeader("x-wpm-req-id", requestId))
	if err != nil {
		return "", err
	}

	return response.Message, nil
}
