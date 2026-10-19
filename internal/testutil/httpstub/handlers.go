package httpstub

import (
	"encoding/json"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonContentType is the media type of JSON request and response bodies.
const jsonContentType = "application/json"

// PathMatcher matches a request path for VerifyRequest and RouteToHandler.
type PathMatcher struct {
	// desc describes the expected path in failure messages, and identifies the
	// matcher when a route replaces another.
	desc string
	// match reports whether a request path matches.
	match func(path string) bool
}

// Path matches a request path equal to path.
//
// Parameters:
//   - path: Expected request path.
//
// Returns:
//   - PathMatcher: Matcher for the exact path.
func Path(path string) PathMatcher {
	return PathMatcher{
		desc:  strconv.Quote(path),
		match: func(got string) bool { return got == path },
	}
}

// PathRegexp matches a request path that contains a match of the regular
// expression expr. It panics when expr does not compile.
//
// Parameters:
//   - expr: Regular expression in RE2 syntax.
//
// Returns:
//   - PathMatcher: Matcher for the expression.
func PathRegexp(expr string) PathMatcher {
	compiled := regexp.MustCompile(expr)

	return PathMatcher{
		desc:  "matching " + strconv.Quote(expr),
		match: compiled.MatchString,
	}
}

// PathSuffix matches a request path that ends with suffix.
//
// Parameters:
//   - suffix: Expected end of the request path.
//
// Returns:
//   - PathMatcher: Matcher for the suffix.
func PathSuffix(suffix string) PathMatcher {
	return PathMatcher{
		desc:  "ending in " + strconv.Quote(suffix),
		match: func(got string) bool { return strings.HasSuffix(got, suffix) },
	}
}

// String describes the expected path.
//
// Returns:
//   - string: Description used in failure messages.
func (m PathMatcher) String() string {
	return m.desc
}

// CombineHandlers returns a handler that calls each handler in order with the
// same request and response writer.
//
// Parameters:
//   - handlers: Handlers to call.
//
// Returns:
//   - http.HandlerFunc: Combined handler.
func CombineHandlers(handlers ...http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, handler := range handlers {
			handler(w, r)
		}
	}
}

// VerifyRequest returns a handler that fails the test unless the request has
// the given method and a matching path.
//
// Parameters:
//   - tb: Test that receives the failures.
//   - method: Expected request method.
//   - path: Expected request path.
//
// Returns:
//   - http.HandlerFunc: Verifying handler that writes no response.
func VerifyRequest(tb testing.TB, method string, path PathMatcher) http.HandlerFunc {
	tb.Helper()

	return func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(tb, method, r.Method, "httpstub: method mismatch for %s", r.URL)
		assert.True(tb, path.match(r.URL.Path), "httpstub: path %q is not %s", r.URL.Path, path)
	}
}

// VerifyQuery returns a handler that fails the test unless the request's query
// parameters equal those of rawQuery, in any order.
//
// Parameters:
//   - tb: Test that receives the failures.
//   - rawQuery: Expected query string without the leading "?".
//
// Returns:
//   - http.HandlerFunc: Verifying handler that writes no response.
func VerifyQuery(tb testing.TB, rawQuery string) http.HandlerFunc {
	tb.Helper()

	want, err := url.ParseQuery(rawQuery)
	require.NoError(tb, err, "httpstub: expected query is malformed")

	return func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(tb, want, r.URL.Query(), "httpstub: query mismatch for %s", r.URL)
	}
}

// VerifyHeader returns a handler that fails the test unless the request
// carries exactly the given values for each header in header. Headers that
// header does not name are not checked.
//
// Parameters:
//   - tb: Test that receives the failures.
//   - header: Expected header values.
//
// Returns:
//   - http.HandlerFunc: Verifying handler that writes no response.
func VerifyHeader(tb testing.TB, header http.Header) http.HandlerFunc {
	tb.Helper()

	return func(_ http.ResponseWriter, r *http.Request) {
		for key, values := range header {
			key = http.CanonicalHeaderKey(key)
			assert.Equal(tb, values, r.Header[key], "httpstub: header mismatch for %s", key)
		}
	}
}

// VerifyJSONRepresenting returns a handler that fails the test unless the
// request has a JSON body equivalent to the JSON encoding of value.
//
// Parameters:
//   - tb: Test that receives the failures.
//   - value: Value whose JSON encoding the body must match.
//
// Returns:
//   - http.HandlerFunc: Verifying handler that writes no response.
func VerifyJSONRepresenting(tb testing.TB, value any) http.HandlerFunc {
	tb.Helper()

	want, err := json.Marshal(value)
	require.NoError(tb, err, "httpstub: expected value cannot be encoded as JSON")

	return func(_ http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		assert.NoError(tb, err, "httpstub: request content type is malformed")
		assert.Equal(tb, "application/json", mediaType, "httpstub: request body is not JSON")

		body, err := io.ReadAll(r.Body)
		if assert.NoError(tb, err, "httpstub: reading the request body failed") {
			assert.JSONEq(tb, string(want), string(body), "httpstub: request body mismatch")
		}
	}
}

// RespondWith returns a handler that writes the status code, the headers, and
// the body.
//
// Parameters:
//   - status: Response status code.
//   - body: Response body.
//   - header: Response headers, or nil for none.
//
// Returns:
//   - http.HandlerFunc: Responding handler.
func RespondWith(status int, body string, header http.Header) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		maps.Copy(w.Header(), header)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// RespondWithJSONEncoded returns a handler that writes the status code, the
// headers, and the JSON encoding of value. The Content-Type is
// application/json unless header sets it.
//
// Parameters:
//   - tb: Test that receives the failures.
//   - status: Response status code.
//   - value: Value to encode as the response body.
//   - header: Response headers, or nil for none.
//
// Returns:
//   - http.HandlerFunc: Responding handler.
func RespondWithJSONEncoded(tb testing.TB, status int, value any, header http.Header) http.HandlerFunc {
	tb.Helper()

	body, err := json.Marshal(value)
	require.NoError(tb, err, "httpstub: response value cannot be encoded as JSON")

	responseHeader := http.Header{"Content-Type": {jsonContentType}}
	maps.Copy(responseHeader, header)

	return RespondWith(status, string(body), responseHeader)
}
