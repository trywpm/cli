package client

import (
	"bytes"
	"context"
	"encoding/json"

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
	GetTarball(ctx context.Context, name string) (string, error)
	GetPackage(ctx context.Context, name string) (wpm.Config, error)
	PutPackage(ctx context.Context, data *wpm.Package) (string, error)
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

func (c *client) GetTarball(ctx context.Context, name string) (string, error) {
	return "", nil
}

func (c *client) GetPackage(ctx context.Context, name string) (wpm.Config, error) {
	return wpm.Config{}, nil
}

func (c *client) PutPackage(ctx context.Context, data *wpm.Package) (string, error) {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	response := struct{ Message string }{}
	err = c.restClient.Put("/", bytes.NewReader(bodyBytes), &response)
	if err != nil {
		return "", err
	}

	return response.Message, nil
}
