package registry

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.wpm.so/cli/pkg/api"
	"go.wpm.so/cli/pkg/pm/signatures"
	"go.wpm.so/cli/pkg/pm/wpmjson/manifest"
)

const (
	maxManifestSize          = 256 * 1024 // 256KB
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
	GetKeysJson(ctx context.Context) (signatures.KeysJson, error)
	DownloadTarball(ctx context.Context, url string) (io.ReadCloser, error)
	PutPackage(ctx context.Context, data *manifest.Package, tarball io.Reader) error
	GetPackageManifest(ctx context.Context, packageName, versionOrTag string, force bool) (*manifest.Package, error)
	AddDistTag(ctx context.Context, packageName, tag, version string) error
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
	manifestBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	manifestLen := len(manifestBytes)
	if manifestLen > maxManifestSize {
		return fmt.Errorf("manifest size exceeds %d bytes, refusing to continue", maxManifestSize)
	}

	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(manifestLen))

	bodyReader := io.MultiReader(
		bytes.NewReader(lengthBytes),
		bytes.NewReader(manifestBytes),
		tarball,
	)

	totalContentLength := int64(4+manifestLen) + data.Dist.PackedSize

	return c.restClient.DoWithContext(
		ctx,
		http.MethodPut,
		"/"+data.Name+"/"+data.Version,
		bodyReader,
		nil,
		api.WithHeader(api.HeaderContentType, contentTypeOctetStream),
		api.WithContentLength(totalContentLength),
	)
}

type distTagRequest struct {
	Version string `json:"version"`
}

// AddDistTag sets a distribution tag to point at a specific package version in the registry.
func (c *client) AddDistTag(ctx context.Context, packageName, tag, version string) error {
	body, err := json.Marshal(distTagRequest{Version: version})
	if err != nil {
		return err
	}

	return c.restClient.DoWithContext(
		ctx,
		http.MethodPut,
		"/-/dist-tags/"+packageName+"/"+tag,
		bytes.NewReader(body),
		nil,
	)
}

// GetPackageManifest retrieves a package manifest from the registry
func (c *client) GetPackageManifest(ctx context.Context, packageName, versionOrTag string, force bool) (*manifest.Package, error) {
	var pkg *manifest.Package

	if versionOrTag == "" {
		versionOrTag = "latest"
	}

	header := api.HeaderSaveCache
	if force {
		header = api.HeaderCacheRevalidate
	}

	err := c.restClient.DoWithContext(
		ctx,
		http.MethodGet,
		"/"+packageName+"/"+versionOrTag,
		nil,
		&pkg,
		api.WithHeader(header, "true"), // Used by cache round tripper.
		api.WithHeader(api.HeaderAccept, wpmContentTypeManifestV1),
	)
	if err != nil {
		return nil, err
	}

	return pkg, nil
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

	if err := c.restClient.DoWithContext(ctx, http.MethodGet, "/-/whoami", nil, &response, opts...); err != nil {
		return "", err
	}

	return response, nil
}

// GetKeysJson retrieves the public keys from the registry
func (c *client) GetKeysJson(ctx context.Context) (signatures.KeysJson, error) {
	var keys signatures.KeysJson

	err := c.restClient.DoWithContext(
		ctx,
		http.MethodGet,
		"/keys.json",
		nil,
		&keys,
	)
	if err != nil {
		return nil, err
	}

	return keys, nil
}
