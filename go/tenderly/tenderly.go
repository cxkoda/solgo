// Package tenderly provides convenience wrappers to work with Tenderly's REST API.
package tenderly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cxkoda/solgo/go/secrets"
)

// Config contains the configuration for the Tenderly wrapper.
type Config struct {
	APIKey string
	APIURL string
}

const apiURL = "https://api.tenderly.co"

// NewFromSecret creates a new Tenderly config from a secret API key.
func NewFromSecret(ctx context.Context, apiKey *secrets.Secret) (*Config, error) {
	key, err := apiKey.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiKey.Fetch(ctx): %v", err)
	}

	return &Config{
		APIKey: string(key),
		APIURL: apiURL,
	}, nil
}

// unmarshalResponse unmarshals the JSON response body into the given type.
func unmarshalResponse[RespT any](body io.Reader) (*RespT, error) {
	var result RespT
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("json.NewDecoder([resp.Body]).Decode(%T): %v", result, err)
	}

	return &result, nil
}

// sendRequest sends a request to the Tenderly API and unmarshals the response into the given type.
func sendRequest[ReqT, RespT any](ctx context.Context, cfg *Config, method, path string, request ReqT) (*RespT, error) {
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(request); err != nil {
		return nil, fmt.Errorf("json.NewEncoder(w).Encode(%T): %v", request, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequest(%q, [apiURL], [json]): %v`, method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do(%+v): %v", req, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %d: io.ReadAll([resp.Body]): %v", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("HTTP %d: %v", resp.StatusCode, string(body))
	}

	return unmarshalResponse[RespT](resp.Body)
}

// NewForkResponse is the response from the Tenderly API when a new fork is created.
type NewForkResponse struct {
	Fork Fork `json:"fork"`
}

// Fork is the fork created by the Tenderly API.
type Fork struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	NetworkID   string      `json:"network_id"`
	BlockNumber string      `json:"block_number"`
	Details     ForkDetails `json:"details"`
	// hex string of private keys for pre-funded accounts
	Accounts map[common.Address]string `json:"accounts"`
	NodeURL  string                    `json:"json_rpc_url"`
}

// ForkDetails is the details of the fork created by the Tenderly API.
type ForkDetails struct {
	ChainConfig ForkDetailsChainConfig `json:"chain_config"`
}

// ForkDetailsChainConfig is the chain config of the fork created by the Tenderly API.
type ForkDetailsChainConfig struct {
	ChainID string `json:"chain_id"`
}

// NewForkParams are the parameters to create a new Tenderly fork using `Config.NewFork`.
type NewForkParams struct {
	// ProjectSlug is the slug of the project to create the fork in.
	ProjectSlug string `json:"-"`
	// Name is the name of the fork.
	Name string `json:"name"`
	// Description is the description of the fork.
	Description string `json:"description"`
	// NetworkID identifies the network to be forked.
	NetworkID uint64 `json:"network_id"`
	// BlockNumber is the block number at which the network is forked.
	BlockNumber uint64 `json:"block_number"`
}

// NewFork creates a new fork on Tenderly.
func (cfg *Config) NewFork(ctx context.Context, params NewForkParams) (*Fork, error) {
	ps := []string{"/api/v2/project", params.ProjectSlug, "forks"}
	path, err := url.JoinPath(cfg.APIURL, ps...)
	if err != nil {
		return nil, fmt.Errorf("url.JoinPath(%q, %v): %v", cfg.APIURL, ps, err)
	}

	resp, err := sendRequest[NewForkParams, NewForkResponse](ctx, cfg, http.MethodPost, path, params)
	if err != nil {
		return nil, fmt.Errorf("sendRequest(..., %s, %s, %+v): %v", path, http.MethodPost, params, err)
	}

	return &resp.Fork, nil
}

// DeleteFork deletes a fork on Tenderly.
func (cfg *Config) DeleteFork(ctx context.Context, projectSlug, forkID string) error {
	ps := []string{"/api/v2/project", projectSlug, "forks", forkID}
	path, err := url.JoinPath(cfg.APIURL, ps...)
	if err != nil {
		return fmt.Errorf("url.JoinPath(%q, %v): %v", cfg.APIURL, ps, err)
	}

	if _, err := sendRequest[struct{}, struct{}](ctx, cfg, http.MethodDelete, path, struct{}{}); err != nil {
		return fmt.Errorf("sendRequest(..., %s, %s): %v", path, http.MethodDelete, err)
	}

	return nil
}

// NewForkClientWithCleanup is a convenience wrapper that creates a new fork on Tenderly and
// returns an ethclient.Client connected to it together with a cleanup function to remove the fork again.
func (cfg *Config) NewForkClient(ctx context.Context, params NewForkParams) (*ethclient.Client, func() error, *Fork, error) {
	fork, err := cfg.NewFork(ctx, params)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%T.NewFork(ctx, [config]): %v", cfg, err)
	}

	client, err := ethclient.DialContext(ctx, fork.NodeURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(" ethclient.DialContext(ctx, [forkURL]): %v", err)
	}

	cleanup := func() error {
		if err := cfg.DeleteFork(ctx, params.ProjectSlug, fork.ID); err != nil {
			return fmt.Errorf("%T.DeleteFork(ctx, %q, %q): %v", cfg, params.ProjectSlug, fork.ID, err)
		}
		return nil
	}

	return client, cleanup, fork, nil
}
