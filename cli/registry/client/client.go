package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"wpm/pkg/api"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/streams"
	wpmTerm "wpm/pkg/term"

	"github.com/moby/term"
)

type client struct {
	restClient *api.RESTClient
}

// RegistryClient is a client used to communicate with a wpm distribution
// registry
type RegistryClient interface {
	PublishPackage(ctx context.Context, data *wpmjson.Package, opts PublishPackageOptions) (string, error)
	GetUploadTarballUrl(ctx context.Context, opts UploadTarballOptions) (UploadTarballResponse, error)
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
	Name           string // package name
	Acl            string // must be one of "public" or "private"
	Digest         string // base64 encoded digest of the package
	Version        string // package version
	Type           string // package type, e.g., "theme", "plugin" or "mu-plugin"
	ContentLength  int64  // length of the content being uploaded
	IdempotencyKey string // unique key to ensure idempotent uploads
}

// UploadTarballResponse defines the response structure for uploading a package.
type UploadTarballResponse struct {
	Id  string `json:"id"`
	Url string `json:"url"`
}

func (c *client) GetUploadTarballUrl(ctx context.Context, opts UploadTarballOptions) (UploadTarballResponse, error) {
	var response UploadTarballResponse
	err := c.restClient.Post(
		fmt.Sprintf("/%s/%s.tar.zst", opts.Name, opts.Version),
		nil,
		&response,
		api.WithHeader("x-wpm-acl", opts.Acl),
		api.WithHeader("x-wpm-package-type", opts.Type),
		api.WithHeader("x-wpm-checksum-sha256", opts.Digest),
		api.WithHeader("content-type", "application/octet-stream"),
		api.WithHeader("x-wpm-idempotency-key", opts.IdempotencyKey),
		api.WithHeader("x-wpm-content-length", fmt.Sprintf("%d", opts.ContentLength)),
	)
	if err != nil {
		return UploadTarballResponse{}, err
	}

	return response, nil
}

// PublishPackageOptions defines the options for publishing a package to the registry.
type PublishPackageOptions struct {
	Name           string // package name
	Version        string // package version
	RequestId      string // request ID from the upload tarball request
	IdempotencyKey string // unique key to ensure idempotent publishing
}

// PublishPackage publishes a package to the registry.
// It requires the package data and a request ID from the upload tarball request.
func (c *client) PublishPackage(ctx context.Context, data *wpmjson.Package, opts PublishPackageOptions) (string, error) {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	response := struct{ Message string }{}
	err = c.restClient.Put(
		"/"+opts.Name+"/"+opts.Version,
		bytes.NewReader(bodyBytes),
		&response,
		api.WithHeader("x-wpm-req-id", opts.RequestId),
		api.WithHeader("x-wpm-idempotency-key", opts.IdempotencyKey),
	)
	if err != nil {
		return "", err
	}

	return response.Message, nil
}
