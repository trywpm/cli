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
	GetPackage(ctx context.Context, name string) (PackageData, error)
	PutPackage(ctx context.Context, data *NewPackageData) (string, error)
}

type PackageData struct{}

type NewPackageData struct {
	// fields from wpm.json
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Type            string            `json:"type"`
	Version         string            `json:"version"`
	License         string            `json:"license,omitempty"`
	Homepage        string            `json:"homepage,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Team            []string          `json:"team,omitempty"`
	Bin             map[string]string `json:"bin,omitempty"`
	Platform        wpm.Platform      `json:"platform"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`

	// package distribution fields
	Wpm        string `json:"_wpm"`
	Digest     string `json:"digest"`
	Access     string `json:"access"`
	Attachment string `json:"attachment"`
	Readme     string `json:"readme,omitempty"`
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

func (c *client) GetPackage(ctx context.Context, name string) (PackageData, error) {
	return PackageData{}, nil
}

func (c *client) PutPackage(ctx context.Context, data *NewPackageData) (string, error) {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	response := struct{ ID string }{}
	err = c.restClient.Put("/", bytes.NewReader(bodyBytes), &response)
	if err != nil {
		return "", err
	}

	return response.ID, nil
}
