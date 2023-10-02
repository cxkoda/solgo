// Package httperr adds idiomatic error handling to net/http and httprouter
// Handle(r)s.
package httperr

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"
)

// Formatf returns an error that induces this package's Handle(r)s to send the
// specified HTTP code. If the code is 200, the returned error is nil. Only
// 400-level codes have their messages propagated. All other codes result in the
// error message being hashed and logged, with only a portion of the hash
// returned to the end user for reporting.
func Formatf(code int, format string, a ...interface{}) error {
	if code == 200 {
		return nil
	}
	return &httpError{
		code: code,
		msg:  fmt.Sprintf(format, a...),
	}
}

// WithStatus converts err into an error with the same behaviour as those returned
// by Formatf().
func WithStatus(code int, err error) error {
	if code == 200 {
		return nil
	}
	return &httpError{code: code, msg: err.Error()}
}

// httpError is an error that carries an HTTP response code and a message.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string {
	return e.msg
}

// HandlerFunc allows http.HandlerFunc-like functions to return errors. If the
// returned error is one returned by Formatf(), it is treated as described in
// that function's documentation. All other errors are treated as 500.
func HandlerFunc(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleErr(w, fn(w, r))
	}
}

// RouterHandle is equivalent to HandlerFunc, but also supports propagation of
// httprouter.Params.
func RouterHandle(fn func(http.ResponseWriter, *http.Request, httprouter.Params) error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		handleErr(w, fn(w, r, p))
	}
}

func handleErr(w http.ResponseWriter, err error) {
	writeObfuscated := func(code int, msg string) {
		id, errMsg := obfuscate(msg)
		glog.Errorf("%x: %s", id, msg)
		http.Error(w, errMsg, code)
	}

	switch err := err.(type) {
	case nil:
	case *httpError:
		// TODO(arran) revisit which codes are propagated.
		if err.code/100 == 4 {
			http.Error(w, err.Error(), err.code)
		} else {
			writeObfuscated(err.code, err.Error())
		}
	default:
		writeObfuscated(500, err.Error())
	}
}

func obfuscate(msg string) ([]byte, string) {
	x := sha256.Sum256([]byte(msg))
	return x[:8], fmt.Sprintf("see log: %x", x[:8])
}
