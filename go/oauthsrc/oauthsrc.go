// Package oauthsrc implements sourcing of OAuth2 tokens.
package oauthsrc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"golang.org/x/oauth2"
)

// A Tokener can be converted into an oauth2.Token. This allows for sources that
// need to be unmarshalled into arbitrary types.
type Tokener interface {
	AsToken() *oauth2.Token
}

// NewHTTP returns an HTTP token source that fetches tokens from the tokenURL
// with the specified HTTP method and request body. The request body will be
// unmarshalled into a new instance of emptyJSONToken, created via reflection.
func NewHTTP(method, tokenURL, body string, emptyJSONToken Tokener) oauth2.TokenSource {
	return &httpSource{
		method:   method,
		tokenURL: tokenURL,
		body:     body,
		empty:    emptyJSONToken,
	}
}

type httpSource struct {
	method, tokenURL string
	body             string
	empty            Tokener
}

func (src *httpSource) Token() (_ *oauth2.Token, retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("obtaining OAuth2 token from %q: %w", src.tokenURL, retErr)
		}
	}()

	req, err := http.NewRequest(src.method, src.tokenURL, strings.NewReader(src.body))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest(%q, …): %v", src.method, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do(…): %v", err)
	}
	if resp.StatusCode != 200 {
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			buf = []byte(fmt.Sprintf("[error reading response body: %v]", err))
		}
		return nil, fmt.Errorf("%s %q", resp.Status, buf)
	}

	raw := reflect.ValueOf(src.empty).Interface().(Tokener)
	if err := json.NewDecoder(resp.Body).Decode(raw); err != nil {
		return nil, fmt.Errorf("decode JSON into %T: %v", raw, err)
	}
	return raw.AsToken(), nil
}
