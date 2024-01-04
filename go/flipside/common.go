// Package flipside provides convenience wrappers to work with flipside.xyz's REST API.
// https://api-docs.flipsidecrypto.xyz/
package flipside

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cxkoda/solgo/go/secrets"
)

// Config configures the flipside wrapper.
type Config struct {
	APIKey string
	APIURL string
}

const apiURL = "https://api-v2.flipsidecrypto.xyz/json-rpc"

// NewFromSecret creates a new Flipside config from a secret API key.
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

type request[T any] struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int    `json:"id"`
	Params  []T    `json:"params"`
}

func (cfg *Config) addHeaders(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("x-api-key", cfg.APIKey)
}

// submitRequest submits a request to the flipside API.
func submitRequest[T any](ctx context.Context, cfg *Config, request request[T]) (io.Reader, error) {
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(request); err != nil {
		return nil, fmt.Errorf("json.NewEncoder(w).Encode(%T): %v", request, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL, body)
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequestWithContext(ctx, "POST", [apiURL], [json]): %v`, err)
	}
	cfg.addHeaders(req)

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

	return resp.Body, nil
}

// submitParams submits a template request with given parameters to the flipside API.
func submitParams[T any](ctx context.Context, cfg *Config, method string, params []T) (io.Reader, error) {
	return submitRequest(ctx, cfg, request[T]{JSONRPC: "2.0", Method: method, ID: 1, Params: params})
}

type response[U any] struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  U           `json:"result"`
	Error   interface{} `json:"error"`
}

// parseResults decodes a flipside API response and returns the result.
// Any flipside API error is returned as error.
func parseResults[U any](r io.Reader) (*U, error) {
	var resp response[U]
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("json.NewDecoder(resp.Body).Decode(%T): %v", resp, err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("flipside API error: %+v", resp.Error)
	}

	return &resp.Result, nil
}

// submitParamsAndParseResults submits a template request with given parameters to the flipside API and parses the results from the APU response.
func submitParamsAndParseResults[T any, U any](ctx context.Context, cfg *Config, method string, params []T) (*U, error) {
	raw, err := submitParams(ctx, cfg, method, params)
	if err != nil {
		return nil, fmt.Errorf("submitParams(ctx, cfg, %q, %+v): %v", method, params, err)
	}

	return parseResults[U](raw)
}

// QueryRunID identifies a flipside query run.
type QueryRunID string
