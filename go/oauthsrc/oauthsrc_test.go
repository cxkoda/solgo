package oauthsrc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/h-fam/errdiff"
	"golang.org/x/oauth2"
)

type weirdToken struct {
	TheToken string `json:"the_token"`
	Type     string `json:"type_of_token"`
	Expiry   int64  `json:"dont_accept_after"`
}

func (w *weirdToken) AsToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: w.TheToken,
		TokenType:   w.Type,
		Expiry:      time.Unix(w.Expiry, 0),
	}
}

const validRequestBody = "valid-body"

func tokenServer(token *weirdToken) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "only POST", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("reading request body: %v", err), http.StatusInternalServerError)
			return
		}
		if string(body) != validRequestBody {
			http.Error(w, fmt.Sprintf("body != %q", validRequestBody), http.StatusBadRequest)
			return
		}

		if err := json.NewEncoder(w).Encode(token); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func TestHTTP(t *testing.T) {
	const (
		token     = "something-very-secret"
		tokenType = "NotABearer"
		expiry    = 42
	)
	server := httptest.NewServer(tokenServer(&weirdToken{
		TheToken: token,
		Type:     tokenType,
		Expiry:   expiry,
	}))

	tests := []struct {
		name              string
		method, url, body string
		emptyJSONToken    Tokener
		errDiffAgainst    interface{}
		want              *oauth2.Token
	}{
		{
			name:           "success",
			method:         http.MethodPost,
			url:            server.URL,
			body:           validRequestBody,
			emptyJSONToken: &weirdToken{},
			want: &oauth2.Token{
				AccessToken: token,
				TokenType:   tokenType,
				Expiry:      time.Unix(expiry, 0),
			},
		},
		{
			name:           "bad method",
			method:         http.MethodGet,
			url:            server.URL,
			errDiffAgainst: strconv.Itoa(http.StatusMethodNotAllowed),
		},
		{
			name:           "bad url",
			method:         http.MethodPost,
			url:            "http://127.0.0.1:1",
			errDiffAgainst: "127.0.0.1:1",
		},
		{
			name:           "bad body",
			method:         http.MethodPost,
			url:            server.URL,
			body:           "BROKEN-" + validRequestBody,
			errDiffAgainst: strconv.Itoa(http.StatusBadRequest),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := NewHTTP(tt.method, tt.url, tt.body, tt.emptyJSONToken)
			for i := 0; i < 10; i++ { // is the src reusable?
				got, err := src.Token()
				if diff := errdiff.Check(err, tt.errDiffAgainst); diff != "" {
					t.Fatalf("attempt #%d: %T.Token() %s", i+1, src, diff)
				}

				ignore := cmpopts.IgnoreUnexported(oauth2.Token{})
				if diff := cmp.Diff(tt.want, got, ignore); diff != "" {
					t.Errorf("attempt #%d: %T.Token() diff (-want +got):\n%s", i+1, src, diff)
				}
			}
		})
	}
}
