package httperr

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/julienschmidt/httprouter"
)

// errMsg returns the error string written to http.Error().
func errMsg(msg string) string {
	_, msg = obfuscate(msg)
	return msg
}

func TestHandlerFunc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://target", nil)

	// ResponseWriters add a bunch of headers that we don't really care about
	// testing. If the body and code return as expected then we know that the
	// plumbing works.
	ignore := cmpopts.IgnoreFields(httptest.ResponseRecorder{}, "HeaderMap")

	// resp builds an expected ResponseRecorder state for use as `want` values
	// in tests.
	resp := func(code int, body string) *httptest.ResponseRecorder {
		return &httptest.ResponseRecorder{
			Code: code,
			Body: bytes.NewBuffer([]byte(body)),
		}
	}

	tests := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) error
		want *httptest.ResponseRecorder
	}{
		{
			name: "empty response",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return nil
			},
			want: resp(200, ""),
		},
		{
			name: "hello world",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				_, err := w.Write([]byte("hello world"))
				return err
			},
			want: resp(200, "hello world"),
		},
		{
			name: "Request propagation",
			fn: func(w http.ResponseWriter, got *http.Request) error {
				want := req
				if got.Method != want.Method || got.URL.String() != want.URL.String() {
					return Formatf(http.StatusBadRequest, "got %T{Method = %q, URL = %q}; want {Method = %q, URL = %q}", got, got.Method, got.URL, want.Method, want.URL)
				}
				return nil
			},
			want: resp(200, ""),
		},
		{
			name: "Formatf(200) identical to nil",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return Formatf(200, "this should be ignored")
			},
			want: resp(200, ""),
		},
		{
			name: "Formatf(non-200/4xx)",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return Formatf(502, "%s", "whoops")
			},
			want: resp(502, errMsg("whoops")),
		},
		{
			name: "WithStatus(200) identical to nil",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return WithStatus(200, errors.New("really bad error"))
			},
			want: resp(200, ""),
		},
		{
			name: "WithStatus(non-200/4xx)",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return WithStatus(502, errors.New("really bad error"))
			},
			want: resp(502, errMsg("really bad error")),
		},
		{
			name: "vanilla error is 500",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return errors.New("uh oh")
			},
			want: resp(500, errMsg("uh oh")),
		},
		{
			name: "teapot (i.e. 400-level shown)",
			fn: func(w http.ResponseWriter, r *http.Request) error {
				return Formatf(418, "I'm a teapot")
			},
			want: resp(418, "I'm a teapot"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := httptest.NewRecorder()
			HandlerFunc(tt.fn)(got, req)

			opts := cmp.Options{
				ignore,
				cmpopts.IgnoreUnexported(
					httptest.ResponseRecorder{},
					bytes.Buffer{},
				),
				cmp.Comparer(func(a, b *bytes.Buffer) bool {
					return strings.TrimSpace(a.String()) == strings.TrimSpace(b.String())
				}),
			}

			if diff := cmp.Diff(tt.want, got, opts); diff != "" {
				t.Errorf("%T passed to HandlerFunc(); diff (-want +got):\n%s", got, diff)
			}
		})
	}
}

func TestRouterHandle(t *testing.T) {
	// Most of the functionality is already tested with the regular
	// net/http.HandlerFunc() so this test only checks that the plumbing is OK.

	method := http.MethodPatch
	req := httptest.NewRequest(method, "http://foo", nil)

	const (
		parKey = "key"
		parVal = "val"
		body   = "good"
	)

	handle := RouterHandle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		if got, want := r.Method, method; got != want {
			return Formatf(http.StatusBadRequest, "got %T.Method = %q; want %q", r, got, want)
		}
		if got, want := p.ByName(parKey), parVal; got != want {
			return Formatf(http.StatusBadRequest, "got %T.ByName(%q) = %q; want %q", p, parKey, got, want)
		}

		_, err := w.Write([]byte(body))
		return err
	})

	got := httptest.NewRecorder()
	handle(got, req, httprouter.Params{{Key: parKey, Value: parVal}})

	if got.Code != 200 || got.Body.String() != body {
		t.Fatalf("%T passed to RouterHandle() got {Code = %d, Body = %q}; want {Code = 200, Body = %q}", got, got.Code, got.Body.String(), body)
	}
}
