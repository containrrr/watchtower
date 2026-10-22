package httpstub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathMatchers verifies which request paths each path matcher accepts.
func TestPathMatchers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		matcher PathMatcher
		path    string
		want    bool
	}{
		{"exact match", Path("/v2/"), "/v2/", true},
		{"exact mismatch", Path("/v2/"), "/v2", false},
		{"regexp match", PathRegexp(`^/v[0-9.]+/info$`), "/v1.45/info", true},
		{"unanchored regexp match", PathRegexp(`/images/create`), "/v1.45/images/create", true},
		{"regexp mismatch", PathRegexp(`^/v[0-9.]+/info$`), "/v1.45/info/x", false},
		{"suffix match", PathSuffix("/start"), "/v1.45/containers/abc/start", true},
		{"suffix mismatch", PathSuffix("/start"), "/v1.45/containers/abc/stop", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.matcher.match(tt.path))
		})
	}
}

// TestPathMatcher_String verifies the descriptions used in failure messages.
func TestPathMatcher_String(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `"/v2/"`, Path("/v2/").String())
	assert.Equal(t, `matching "^/v2/$"`, PathRegexp(`^/v2/$`).String())
	assert.Equal(t, `ending in "/start"`, PathSuffix("/start").String())
}

// TestCombineHandlers verifies that combined handlers run in order on the
// same response.
func TestCombineHandlers(t *testing.T) {
	t.Parallel()

	var order []string

	handler := CombineHandlers(
		func(http.ResponseWriter, *http.Request) { order = append(order, "verify") },
		RespondWith(http.StatusTeapot, "body", nil),
		func(http.ResponseWriter, *http.Request) { order = append(order, "after") },
	)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	assert.Equal(t, []string{"verify", "after"}, order)
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "body", rec.Body.String())
}

// TestVerifyRequest verifies that a request passes only with the expected
// method and a matching path, and that each mismatch is reported.
func TestVerifyRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		target   string
		failures int
	}{
		{"matching request", http.MethodGet, "/v2/test/manifests/latest", 0},
		{"wrong method", http.MethodPost, "/v2/test/manifests/latest", 1},
		{"wrong path", http.MethodGet, "/v2/other/manifests/latest", 1},
		{"wrong method and path", http.MethodPost, "/v2/other", 2},
		{"query ignored", http.MethodGet, "/v2/test/manifests/latest?x=1", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := newFailureRecorder(t)
			handler := VerifyRequest(recorder, http.MethodGet, Path("/v2/test/manifests/latest"))

			handler(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), tt.method, tt.target, nil))

			assert.Len(t, recorder.reported(), tt.failures)
		})
	}
}

// TestVerifyQuery verifies that query parameters must match in any order.
func TestVerifyQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   string
		failures int
	}{
		{"same order", "/token?scope=repo&service=hub", 0},
		{"other order", "/token?service=hub&scope=repo", 0},
		{"missing parameter", "/token?scope=repo", 1},
		{"extra parameter", "/token?scope=repo&service=hub&x=1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := newFailureRecorder(t)
			handler := VerifyQuery(recorder, "scope=repo&service=hub")

			handler(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil))

			assert.Len(t, recorder.reported(), tt.failures)
		})
	}
}

// TestVerifyHeader verifies that each named header must carry exactly the
// expected values, whatever the case of its name, and that other headers are
// ignored.
func TestVerifyHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   http.Header
		failures int
	}{
		{"matching header", http.Header{"Authorization": {"Bearer token"}, "X-Other": {"1"}}, 0},
		{"missing header", http.Header{"X-Other": {"1"}}, 1},
		{"different value", http.Header{"Authorization": {"Basic abc"}}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := newFailureRecorder(t)
			handler := VerifyHeader(recorder, http.Header{"authorization": {"Bearer token"}})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.Header = tt.header
			handler(httptest.NewRecorder(), req)

			assert.Len(t, recorder.reported(), tt.failures)
		})
	}
}

// TestVerifyJSONRepresenting verifies that the request must have a JSON
// content type and a body equivalent to the expected value's encoding.
func TestVerifyJSONRepresenting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		failures    int
	}{
		{"equivalent body", "application/json", `{"b": [1, 2], "a": "x"}`, 0},
		{"content type with parameters", "application/json; charset=utf-8", `{"a":"x","b":[1,2]}`, 0},
		{"different body", "application/json", `{"a":"y","b":[1,2]}`, 1},
		{"wrong content type", "text/plain", `{"a":"x","b":[1,2]}`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := newFailureRecorder(t)
			handler := VerifyJSONRepresenting(recorder, map[string]any{"a": "x", "b": []int{1, 2}})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			handler(httptest.NewRecorder(), req)

			assert.Len(t, recorder.reported(), tt.failures)
		})
	}
}

// TestRespondWith verifies that the response carries the status, headers, and
// body.
func TestRespondWith(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	RespondWith(http.StatusNotFound, "missing", http.Header{"X-Reason": {"gone"}})(
		rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
	)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "missing", rec.Body.String())
	assert.Equal(t, "gone", rec.Header().Get("X-Reason"))
}

// TestRespondWithJSONEncoded verifies that the response body is the value's
// JSON encoding, with a JSON content type unless the headers set one.
func TestRespondWithJSONEncoded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		header          http.Header
		wantContentType string
	}{
		{"default content type", nil, "application/json"},
		{"content type from headers", http.Header{"Content-Type": {"application/vnd.docker+json"}}, "application/vnd.docker+json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			RespondWithJSONEncoded(t, http.StatusOK, map[string]string{"ID": "abc"}, tt.header)(
				rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
			)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantContentType, rec.Header().Get("Content-Type"))
			require.JSONEq(t, `{"ID":"abc"}`, rec.Body.String())
		})
	}
}
