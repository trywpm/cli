package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"wpm/pkg/api"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/streams"
)

type client struct {
	restClient *api.RESTClient
}

// RegistryClient is a client used to communicate with a wpm distribution
// registry
type RegistryClient interface {
	PutPackage(ctx context.Context, data *wpmjson.PackageManifest, tarball io.Reader) error
	GetPackageManifest(ctx context.Context, packageName string, versionOrTag string) (*wpmjson.PackageManifest, error)
}

var _ RegistryClient = &client{}

// NewRegistryClient returns a new REST client for the wpm registry
func NewRegistryClient(host string, authToken string, userAgent string, out *streams.Out) (RegistryClient, error) {
	opts := api.ClientOptions{
		Log:         out,
		Host:        host,
		AuthToken:   authToken,
		LogColorize: out.IsColorEnabled(),
		Headers:     map[string]string{"User-Agent": userAgent},
	}

	_client, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, err
	}

	return &client{
		restClient: _client,
	}, nil
}

// PutPackage uploads a package to the registry
func (c *client) PutPackage(ctx context.Context, data *wpmjson.PackageManifest, tarball io.Reader) error {
	manifest, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.restClient.Put(
		"/",
		tarball,
		nil,
		api.WithHeader("Content-Type", "application/octet-stream"),
		api.WithHeader("x-wpm-manifest", base64.StdEncoding.EncodeToString(manifest)),
	)
}

// GetPackageManifest retrieves a package manifest from the registry
func (c *client) GetPackageManifest(ctx context.Context, packageName string, versionOrTag string) (*wpmjson.PackageManifest, error) {
	var manifest wpmjson.PackageManifest

	err := c.restClient.Get(
		"/"+packageName+"/"+versionOrTag,
		&manifest,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}
