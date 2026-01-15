package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"wpm/pkg/api"
	"wpm/pkg/pm/wpmjson/manifest"
)

const (
	wpmManifestHeader        = "X-WPM-Manifest"
	contentTypeOctetStream   = "application/octet-stream"
	wpmContentTypeManifestV1 = "application/vnd.wpm.install-v1+json"
)

type client struct {
	restClient *api.RESTClient
}

// Client is a client used to communicate with a wpm distribution
// registry
type Client interface {
	Whoami(ctx context.Context, token string) (string, error)
	DownloadTarball(ctx context.Context, url string) (io.ReadCloser, error)
	PutPackage(ctx context.Context, data *manifest.Package, tarball io.Reader) error
	GetPackageManifest(ctx context.Context, packageName, versionOrTag string, force bool) (*manifest.Package, error)
}

var _ Client = &client{}

// New returns a new REST client for the wpm registry
func New(host, authToken, userAgent, cacheDir string, colorize bool, out io.Writer) (Client, error) {
	opts := api.ClientOptions{
		Log:         out,
		Host:        host,
		AuthToken:   authToken,
		LogColorize: colorize,
		CacheDir:    cacheDir,
		Headers:     map[string]string{api.HeaderUserAgent: userAgent},
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
func (c *client) PutPackage(ctx context.Context, data *manifest.Package, tarball io.Reader) error {
	manifest, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.restClient.Put(
		"/",
		tarball,
		nil,
		api.WithHeader(api.HeaderContentType, contentTypeOctetStream),
		api.WithHeader(wpmManifestHeader, base64.StdEncoding.EncodeToString(manifest)),
	)
}

// GetPackageManifest retrieves a package manifest from the registry
func (c *client) GetPackageManifest(ctx context.Context, packageName, versionOrTag string, force bool) (*manifest.Package, error) {
	var manifest *manifest.Package

	if versionOrTag == "" || versionOrTag == "*" {
		versionOrTag = "latest"
	}

	header := api.HeaderSaveCache
	if force {
		header = api.HeaderCacheRevalidate
	}

	err := c.restClient.Get(
		"/"+packageName+"/"+versionOrTag,
		&manifest,
		api.WithHeader(header, "true"), // Used by cache round tripper.
		api.WithHeader(api.HeaderAccept, wpmContentTypeManifestV1),
	)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

// DownloadTarball downloads a package tarball from the registry
func (c *client) DownloadTarball(ctx context.Context, url string) (io.ReadCloser, error) {
	return c.restClient.RequestStream(
		ctx,
		http.MethodGet,
		url,
		nil,
		api.WithHeader(api.HeaderAccept, contentTypeOctetStream),
		api.WithHeader(api.HeaderSaveCache, "true"), // Used by cache round tripper.
	)
}

// Whoami validates the provided token and returns the associated username
func (c *client) Whoami(ctx context.Context, token string) (string, error) {
	var response string

	opts := []api.RequestOption{}
	if token != "" {
		opts = append(opts, api.WithHeader(api.HeaderAuthorization, "Bearer "+token))
	}

	if err := c.restClient.Get("/-/whoami", &response, opts...); err != nil {
		return "", err
	}

	return response, nil
}
