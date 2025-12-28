package client

import (
	"context"
	"encoding/json"
	"io"

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
	PutPackage(ctx context.Context, data *wpmjson.Package, tarball io.Reader) (string, error)
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

// PutPackage uploads a package to the registry
func (c *client) PutPackage(ctx context.Context, data *wpmjson.Package, tarball io.Reader) (string, error) {
	manifest, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	response := struct{ Message string }{}
	if err := c.restClient.Put(
		"/",
		tarball,
		&response,
		api.WithHeader("x-wpm-manifest", string(manifest)),
	); err != nil {
		return "", err
	}

	return response.Message, nil
}
