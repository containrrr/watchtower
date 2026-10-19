package httpstub

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failureRecorder is a test handle that records the failures reported to it
// instead of failing the test, so tests can check what a server or verifier
// reports. Every other method goes to the embedded test.
type failureRecorder struct {
	testing.TB

	// mu guards failures.
	mu sync.Mutex
	// failures holds the reported failure messages.
	failures []string
}

// Errorf records a failure message.
//
// Parameters:
//   - format: Message format.
//   - args: Format arguments.
func (f *failureRecorder) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failures = append(f.failures, fmt.Sprintf(format, args...))
}

// Helper does nothing, since the recorder reports no file and line.
func (f *failureRecorder) Helper() {}

// reported returns the recorded failure messages.
//
// Returns:
//   - []string: Copy of the recorded messages.
func (f *failureRecorder) reported() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.failures...)
}

// newFailureRecorder returns a recorder whose cleanups run with t's.
//
// Parameters:
//   - t: Test that owns the recorder.
//
// Returns:
//   - *failureRecorder: Recorder with no failures.
func newFailureRecorder(t *testing.T) *failureRecorder {
	t.Helper()

	return &failureRecorder{TB: t}
}

// get sends a GET request for path to the server and returns the response
// status code and body.
//
// Parameters:
//   - t: Test that owns the request.
//   - server: Server to request.
//   - path: Request path.
//
// Returns:
//   - int: Response status code.
//   - string: Response body.
func get(t *testing.T, server *Server, path string) (int, string) {
	t.Helper()

	return send(t, server, http.MethodGet, path)
}

// send sends a request with the given method for path to the server and
// returns the response status code and body.
//
// Parameters:
//   - t: Test that owns the request.
//   - server: Server to request.
//   - method: Request method.
//   - path: Request path.
//
// Returns:
//   - int: Response status code.
//   - string: Response body.
func send(t *testing.T, server *Server, method, path string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, server.URL()+path, nil)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

// TestServer_AppendHandlersAnswerInOrder verifies that each appended handler
// answers one request, in the order the handlers were appended.
func TestServer_AppendHandlersAnswerInOrder(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.AppendHandlers(
		RespondWith(http.StatusOK, "first", nil),
		RespondWith(http.StatusAccepted, "second", nil),
	)
	server.AppendHandlers(RespondWith(http.StatusCreated, "third", nil))

	for _, want := range []struct {
		status int
		body   string
	}{
		{http.StatusOK, "first"},
		{http.StatusAccepted, "second"},
		{http.StatusCreated, "third"},
	} {
		status, body := get(t, server, "/")
		assert.Equal(t, want.status, status)
		assert.Equal(t, want.body, body)
	}

	assert.Empty(t, recorder.reported())
}

// TestServer_UnhandledRequestFailsTest verifies that a request with no handler
// left fails the test with a dump of the request and gets a 500 response.
func TestServer_UnhandledRequestFailsTest(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.AppendHandlers(RespondWith(http.StatusOK, "", nil))

	status, _ := get(t, server, "/first")
	assert.Equal(t, http.StatusOK, status)

	status, _ = get(t, server, "/second")
	assert.Equal(t, http.StatusInternalServerError, status)

	failures := recorder.reported()
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "unhandled request")
	assert.Contains(t, failures[0], "GET /second")
}

// TestServer_AllowUnhandledRequests verifies that an allowed unhandled request
// gets a 500 response without failing the test.
func TestServer_AllowUnhandledRequests(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.AllowUnhandledRequests()

	status, _ := get(t, server, "/anything")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Empty(t, recorder.reported())
}

// TestServer_RouteToHandler verifies that routes answer every matching request
// before appended handlers are used, that the method must match, and that a
// route for the same method and matcher replaces the earlier one.
func TestServer_RouteToHandler(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.RouteToHandler(http.MethodGet, PathRegexp(`^/v[0-9.]+/info$`), RespondWith(http.StatusOK, "old", nil))
	server.RouteToHandler(http.MethodGet, PathRegexp(`^/v[0-9.]+/info$`), RespondWith(http.StatusOK, "info", nil))
	server.RouteToHandler(http.MethodPost, Path("/token"), RespondWith(http.StatusCreated, "token", nil))
	server.AppendHandlers(RespondWith(http.StatusAccepted, "appended", nil))

	for range 2 {
		status, body := get(t, server, "/v1.45/info")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "info", body)
	}

	status, body := send(t, server, http.MethodPost, "/token")
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "token", body)

	// A GET to the POST route falls through to the appended handler.
	status, body = get(t, server, "/token")
	assert.Equal(t, http.StatusAccepted, status)
	assert.Equal(t, "appended", body)

	assert.Empty(t, recorder.reported())
}

// TestServer_ReceivedRequests verifies that every request is recorded in
// arrival order, including routed and unhandled ones, and that the returned
// slice is a copy.
func TestServer_ReceivedRequests(t *testing.T) {
	t.Parallel()

	server := New(t)
	server.AllowUnhandledRequests()
	server.RouteToHandler(http.MethodGet, Path("/routed"), RespondWith(http.StatusOK, "", nil))
	server.AppendHandlers(RespondWith(http.StatusOK, "", nil))

	get(t, server, "/routed")
	get(t, server, "/appended")
	get(t, server, "/unhandled")

	requests := server.ReceivedRequests()
	require.Len(t, requests, 3)
	assert.Equal(t, "/routed", requests[0].URL.Path)
	assert.Equal(t, "/appended", requests[1].URL.Path)
	assert.Equal(t, "/unhandled", requests[2].URL.Path)

	requests[0] = nil

	assert.NotNil(t, server.ReceivedRequests()[0])
}

// TestServer_Reset verifies that Reset removes the handlers, routes, and
// received requests, so the next request is unhandled.
func TestServer_Reset(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.RouteToHandler(http.MethodGet, Path("/routed"), RespondWith(http.StatusOK, "", nil))
	server.AppendHandlers(RespondWith(http.StatusOK, "", nil), RespondWith(http.StatusOK, "", nil))

	get(t, server, "/appended")
	server.Reset()

	assert.Empty(t, server.ReceivedRequests())

	status, _ := get(t, server, "/routed")
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Len(t, recorder.reported(), 1)
	assert.Len(t, server.ReceivedRequests(), 1)
}

// TestServer_HandlerPanicFailsTest verifies that a panicking handler fails the
// test and gets a 500 response, instead of only dropping the connection.
func TestServer_HandlerPanicFailsTest(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.AppendHandlers(func(http.ResponseWriter, *http.Request) { panic("boom") })

	status, _ := get(t, server, "/panic")
	assert.Equal(t, http.StatusInternalServerError, status)

	failures := recorder.reported()
	require.Len(t, failures, 1)
	assert.Contains(t, failures[0], "panicked: boom")
}

// TestServer_AbortHandlerPanicAborts verifies that a handler that panics with
// http.ErrAbortHandler aborts the response without failing the test.
func TestServer_AbortHandlerPanicAborts(t *testing.T) {
	t.Parallel()

	recorder := newFailureRecorder(t)
	server := New(recorder)
	server.AppendHandlers(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL()+"/abort", nil)
	require.NoError(t, err)

	resp, err := server.Client().Do(req)
	if err == nil {
		resp.Body.Close()
	}

	require.Error(t, err)
	assert.Empty(t, recorder.reported())
}

// TestNewTLS verifies that a TLS server serves HTTPS and that its client
// trusts the server's certificate.
func TestNewTLS(t *testing.T) {
	t.Parallel()

	server := NewTLS(t)
	server.AppendHandlers(RespondWith(http.StatusOK, "secure", nil))

	assert.True(t, strings.HasPrefix(server.URL(), "https://"))

	status, body := get(t, server, "/")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "secure", body)
}

// TestNewUnstarted verifies that an unstarted server can be configured through
// TestServer before it starts, here with a maximum TLS version.
func TestNewUnstarted(t *testing.T) {
	t.Parallel()

	server := NewUnstarted(t)
	server.TestServer().TLS = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
	server.TestServer().StartTLS()
	server.AppendHandlers(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, uint16(tls.VersionTLS12), r.TLS.Version)
		w.WriteHeader(http.StatusOK)
	})

	status, _ := get(t, server, "/")
	assert.Equal(t, http.StatusOK, status)
}

// TestServer_Addr verifies that Addr is the host and port of the server's URL.
func TestServer_Addr(t *testing.T) {
	t.Parallel()

	server := New(t)

	assert.Equal(t, "http://"+server.Addr(), server.URL())
}

// TestServer_CloseTwice verifies that closing a closed server does nothing, so
// a test can close a server before its cleanup does.
func TestServer_CloseTwice(t *testing.T) {
	t.Parallel()

	server := New(t)
	server.Close()

	assert.NotPanics(t, server.Close)
}
