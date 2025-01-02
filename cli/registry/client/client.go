package client

import (
	"context"

	"wpm/cli/streams"
	"wpm/pkg/api"
	wpmTerm "wpm/pkg/term"

	"github.com/moby/term"
)

type client struct {
	restClient *api.RESTClient
}

// RegistryClient is a client used to communicate with a wpm distribution
// registry
type RegistryClient interface {
	GetTarball(ctx context.Context, name string) (string, error)
	GetPackage(ctx context.Context, name string) (PackageData, error)
	PutPackage(ctx context.Context, data PackageData) (NewPackageData, error)
}

type PackageData struct{}

type NewPackageData struct {
	PackageData
}

var _ RegistryClient = &client{}

// NewRegistryClient returns a new REST client for the wpm registry
func NewRegistryClient(authToken string, userAgent string, out *streams.Out) (RegistryClient, error) {
	opts := api.ClientOptions{
		Log:         out,
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

func (c *client) GetPackage(ctx context.Context, name string) (PackageData, error) {
	return PackageData{}, nil
}

func (c *client) PutPackage(ctx context.Context, data PackageData) (NewPackageData, error) {
	return NewPackageData{}, nil
}
