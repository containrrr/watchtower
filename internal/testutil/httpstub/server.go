package httpstub

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"slices"
	"sync"
	"testing"
)

// Server is an HTTP test server that answers each request with a handler the
// test registered.
//
// A request whose method and path match a route goes to that route's handler,
// which answers every matching request. Any other request goes to the next
// unused appended handler, so appended handlers answer requests in the order
// they were appended. A request with no handler fails the test.
type Server struct {
	// tb receives the failures for unhandled and panicking requests.
	tb testing.TB
	// testServer is the underlying server.
	testServer *httptest.Server
	// mu guards the fields below.
	mu sync.Mutex
	// handlers holds the appended handlers in the order they answer requests.
	handlers []http.HandlerFunc
	// calls counts the appended handlers that have answered a request.
	calls int
	// routes holds the handlers that answer every request matching their route.
	routes []route
	// requests holds every request the server received, in arrival order.
	requests []*http.Request
	// allowUnhandled answers unhandled requests with 500 instead of failing
	// the test.
	allowUnhandled bool
}

// route is a handler for every request with a given method and path.
type route struct {
	// method is the request method the route matches.
	method string
	// path matches the request path.
	path PathMatcher
	// handler answers the matching requests.
	handler http.HandlerFunc
}

// New starts a server that serves HTTP and closes it when the test ends.
//
// Parameters:
//   - tb: Test that receives the server's failures.
//
// Returns:
//   - *Server: Started server.
func New(tb testing.TB) *Server {
	tb.Helper()

	server := NewUnstarted(tb)
	server.testServer.Start()

	return server
}

// NewTLS starts a server that serves HTTPS with a self-signed certificate and
// closes it when the test ends. Its Client trusts that certificate.
//
// Parameters:
//   - tb: Test that receives the server's failures.
//
// Returns:
//   - *Server: Started server.
func NewTLS(tb testing.TB) *Server {
	tb.Helper()

	server := NewUnstarted(tb)
	server.testServer.StartTLS()

	return server
}

// NewUnstarted creates a server that the test configures through TestServer
// and then starts. The server closes when the test ends.
//
// Parameters:
//   - tb: Test that receives the server's failures.
//
// Returns:
//   - *Server: Server that is not yet listening for requests.
func NewUnstarted(tb testing.TB) *Server {
	tb.Helper()

	server := &Server{tb: tb}
	server.testServer = httptest.NewUnstartedServer(http.HandlerFunc(server.serveHTTP))
	tb.Cleanup(server.Close)

	return server
}

// URL returns the server's base URL, such as http://127.0.0.1:41234.
//
// Returns:
//   - string: Base URL with scheme and address.
func (s *Server) URL() string {
	return s.testServer.URL
}

// Addr returns the address the server listens on, such as 127.0.0.1:41234.
//
// Returns:
//   - string: Host and port.
func (s *Server) Addr() string {
	return s.testServer.Listener.Addr().String()
}

// Client returns an HTTP client for the server. For a TLS server, the client
// trusts the server's certificate.
//
// Returns:
//   - *http.Client: Client configured for the server.
func (s *Server) Client() *http.Client {
	return s.testServer.Client()
}

// TestServer returns the underlying server, for settings this package does not
// cover, such as the TLS configuration of an unstarted server.
//
// Returns:
//   - *httptest.Server: Underlying server.
func (s *Server) TestServer() *httptest.Server {
	return s.testServer
}

// Close shuts the server down and waits for its in-flight requests to finish.
// Closing a closed server does nothing.
func (s *Server) Close() {
	s.testServer.Close()
}

// AppendHandlers queues handlers that each answer one request, in order, after
// the handlers already queued.
//
// Parameters:
//   - handlers: Handlers to queue.
func (s *Server) AppendHandlers(handlers ...http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = append(s.handlers, handlers...)
}

// RouteToHandler makes handler answer every request with the given method and
// a matching path. Routes take precedence over appended handlers. A route with
// the same method and path matcher as an existing route replaces it.
//
// Parameters:
//   - method: Request method to match.
//   - path: Request path to match.
//   - handler: Handler that answers the matching requests.
func (s *Server) RouteToHandler(method string, path PathMatcher, handler http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newRoute := route{method: method, path: path, handler: handler}

	for i, existing := range s.routes {
		if existing.method == method && existing.path.desc == path.desc {
			s.routes[i] = newRoute

			return
		}
	}

	s.routes = append(s.routes, newRoute)
}

// ReceivedRequests returns every request the server received, including
// unhandled ones, in arrival order.
//
// Returns:
//   - []*http.Request: Copy of the received requests.
func (s *Server) ReceivedRequests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.requests)
}

// AllowUnhandledRequests makes the server answer a request with no handler
// with 500 Internal Server Error instead of failing the test.
func (s *Server) AllowUnhandledRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allowUnhandled = true
}

// Reset removes the appended handlers, the routes, and the received requests,
// and then closes every open client connection, including connections with a
// request in flight.
func (s *Server) Reset() {
	s.mu.Lock()
	s.handlers = nil
	s.calls = 0
	s.routes = nil
	s.requests = nil
	s.mu.Unlock()

	// Closing can wait for each connection, so it runs without the lock that
	// incoming requests need.
	s.testServer.CloseClientConnections()
}

// serveHTTP records a request and answers it with its route's handler, the next
// appended handler, or the unhandled response.
//
// A handler that panics fails the test and gets a 500 response, except for a
// panic with http.ErrAbortHandler, which aborts the response as net/http does.
//
// Parameters:
//   - w: Response writer.
//   - r: Received request.
func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		err, isError := recovered.(error)
		if isError && errors.Is(err, http.ErrAbortHandler) {
			panic(recovered)
		}

		s.tb.Errorf("httpstub: handler for %s %s panicked: %v", r.Method, r.URL, recovered)
		w.WriteHeader(http.StatusInternalServerError)
	}()

	handler, allowUnhandled := s.nextHandler(r)
	if handler != nil {
		handler(w, r)

		return
	}

	if !allowUnhandled {
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			s.tb.Errorf("httpstub: unhandled request %s %s (dump failed: %v)", r.Method, r.URL, err)
		} else {
			s.tb.Errorf("httpstub: unhandled request:\n%s", dump)
		}
	}

	_, _ = io.Copy(io.Discard, r.Body)

	w.WriteHeader(http.StatusInternalServerError)
}

// nextHandler records a request and selects the handler that answers it.
//
// Parameters:
//   - r: Received request.
//
// Returns:
//   - http.HandlerFunc: Handler for the request, nil when none is registered.
//   - bool: True when an unhandled request must not fail the test.
func (s *Server) nextHandler(r *http.Request) (http.HandlerFunc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, r)

	for _, candidate := range s.routes {
		if candidate.method == r.Method && candidate.path.match(r.URL.Path) {
			return candidate.handler, s.allowUnhandled
		}
	}

	if s.calls < len(s.handlers) {
		handler := s.handlers[s.calls]
		s.calls++

		return handler, s.allowUnhandled
	}

	return nil, s.allowUnhandled
}
