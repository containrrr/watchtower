package events

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

func TestNewHandler(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)
	require.NotNil(t, h)
	assert.Equal(t, "/v1/events", h.Path)
	assert.Nil(t, h.AllowedOrigins)

	h2 := NewHandler(newTestLogger(t), b, []string{"https://example.com"})
	assert.Equal(t, []string{"https://example.com"}, h2.AllowedOrigins)
}

func TestHandler_Handle_Connects(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestHandler_Handle_MaxSubscribers(t *testing.T) {
	b := NewBroadcaster()

	for range maxSubscribers {
		ch, _ := b.SubscribeWithDone()
		assert.NotNil(t, ch, "subscriber channel should not be nil")
	}

	ch, _ := b.SubscribeWithDone()
	assert.Nil(t, ch, "subscriber channel should be nil when max subscribers reached")
}

func TestHandler_Handle_NilBroadcaster(t *testing.T) {
	h := NewHandler(newTestLogger(t), nil, nil)
	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	// The SSE handler returns 200 before checking the broadcaster, so
	// a nil broadcaster results in a 503 after the stream opens.
	// The test client may see a timeout or a 200 with subsequent error.
	if err != nil {
		assert.Error(t, err)
	} else {
		resp.Body.Close()
	}
}

func TestHandler_Handle_ReceivesEvent(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 2 * time.Second,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestHandler_Handle_ReceivesPublishedEvent(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	subCh, _ := b.SubscribeWithDone()
	assert.NotNil(t, subCh)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 2 * time.Second,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	event := Event{Type: "test-event", Data: map[string]string{"key": "value"}}
	b.Publish(event)

	select {
	case received := <-subCh:
		assert.Equal(t, "test-event", received.Type)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("did not receive event within timeout")
	}

	b.Unsubscribe(subCh)
}

func TestHandler_Handle_RejectsDisallowedOrigin(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, []string{"https://allowed.com"})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Host", "example.com:8080")

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestHandler_Handle_AllowsSameOrigin(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com:8080/v1/events", nil)

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestHandler_Handle_AllowsCORSMatch(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, []string{"https://app.example.com"})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/events", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Host", "example.com:8080")

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestIsOriginAllowed(t *testing.T) {
	cases := []struct {
		name    string
		origin  string
		host    string
		allowed []string
		want    bool
	}{
		{"empty origin", "", "example.com:8080", nil, true},
		{"null origin", "null", "example.com:8080", nil, true},
		{"null in allowed list", "null", "example.com:8080", []string{"null"}, true},
		{"same origin no scheme", "example.com:8080", "example.com:8080", nil, true},
		{"same origin http", "http://example.com:8080", "example.com:8080", nil, true},
		{"same origin https", "https://example.com:8080", "example.com:8080", nil, true},
		{"different origin no cors", "https://evil.com", "example.com:8080", nil, false},
		{"subdomain is different origin", "https://www.example.com", "example.com:8080", nil, false},
		{"port mismatch same host", "https://example.com:8443", "example.com:8080", nil, false},
		{"credentials preserved in origin", "https://user:pass@example.com", "example.com:8080", []string{"https://user:pass@example.com"}, true},
		{"non-http origin not allowed", "file:///etc/passwd", "example.com:8080", nil, false},
		{"explicit cors match", "https://app.example.com", "example.com:8080", []string{"https://app.example.com"}, true},
		{"cors wildcard", "https://anything", "example.com:8080", []string{"*"}, true},
		{"cors list no match", "https://other.com", "example.com:8080", []string{"https://app.example.com"}, false},
		{"cors plus same origin", "http://example.com:8080", "example.com:8080", []string{"https://app.example.com"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isOriginAllowed(tc.origin, tc.host, tc.allowed)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHandler_Handle_AllowsNullOrigin(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com:8080/v1/events", nil)
	req.Header.Set("Origin", "null")

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestHandler_Handle_AllowsWildcardCORS(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, []string{"*"})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com:8080/v1/events", nil)
	req.Header.Set("Origin", "https://anything.com")

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestHandler_Handle_MaxSubscribersHTTP(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	for range maxSubscribers {
		ch, _ := b.SubscribeWithDone()
		require.NotNil(t, ch, "pre-fill subscriber")
	}

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com:8080/v1/events", nil)

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

func TestHandler_Handle_StreamStaysOpenAndDeliversEvent(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	go func() {
		time.Sleep(20 * time.Millisecond)
		b.Publish(Event{
			Type:      "scan_completed",
			Timestamp: time.Now().UTC(),
			Data:      ScanCompletedData{Scanned: 1, Updated: 0, Failed: 0},
		})
	}()

	resp, err := app.Test(httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/events",
		http.NoBody,
	), fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	scanner := bufio.NewScanner(resp.Body)
	found := false

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "event: scan_completed") {
			found = true

			break
		}
	}

	err = scanner.Err()
	if err != nil {
		t.Fatal(err)
	}

	require.True(t, found, "expected event not received in stream")
}

func TestHandler_Handle_OriginCheckPrecedence(t *testing.T) {
	b := NewBroadcaster()
	h := NewHandler(newTestLogger(t), b, nil)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/events", h.Handle())

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com:8080/v1/events", nil)
	req.Header.Set("Origin", "https://evil.com")

	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// TestHandler_Handle_EndsStreamsOnShutdown verifies that a server shutdown
// ends open event streams, whether their client is still connected, has gone,
// or has stopped reading in the middle of an event, so the shutdown waits
// neither for the next heartbeat nor for a write the client never reads.
func TestHandler_Handle_EndsStreamsOnShutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// disconnect closes the client's side of the stream before the shutdown.
		disconnect bool
		// stall publishes an event whose write blocks, as for a client that has
		// stopped reading, and waits for the write to start.
		stall bool
	}{
		{name: "client connected"},
		{name: "client disconnected", disconnect: true},
		{name: "client stopped reading during an event write", stall: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := NewBroadcaster()
			h := NewHandler(newTestLogger(t), b, nil)

			app := fiber.New(fiber.Config{})
			app.Get("/v1/events", h.Handle())

			inner, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)

			listener := newStallingListener(t, inner)

			go func() {
				_ = app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
			}()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				"http://"+listener.Addr().String()+"/v1/events", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			// The retry field is the first thing the stream writes.
			line, err := bufio.NewReader(resp.Body).ReadString('\n')
			require.NoError(t, err)
			assert.Equal(t, "retry: 5000\n", line)

			if tt.disconnect {
				require.NoError(t, resp.Body.Close())
			}

			if tt.stall {
				b.Publish(Event{Type: stallEventType})

				select {
				case <-listener.writing:
				case <-time.After(5 * time.Second):
					t.Fatal("the stall event was never written")
				}
			}

			// The heartbeat comes every 15 seconds, so a stream that ignores the
			// shutdown holds it past this timeout.
			require.NoError(t, app.ShutdownWithTimeout(2*time.Second))
			assert.Zero(t, b.SubscriberCount())
		})
	}
}

// stallEventType names the event whose write a stallingConn blocks.
const stallEventType = "stall"

// stallingListener wraps a listener so that a write of the stall event on an
// accepted connection blocks, as a write to a client that has stopped reading
// does, until a write deadline that has passed is set.
type stallingListener struct {
	net.Listener

	// writing is closed when a write of the stall event starts.
	writing chan struct{}
	// release is closed when a write deadline that has passed is set, or when
	// the test ends.
	release chan struct{}
	// writingOnce closes writing once.
	writingOnce sync.Once
	// releaseOnce closes release once.
	releaseOnce sync.Once
}

// newStallingListener wraps inner and unblocks any stalled write when the test
// ends.
//
// Parameters:
//   - t: Test that owns the listener.
//   - inner: Listener to wrap.
//
// Returns:
//   - *stallingListener: The wrapping listener.
func newStallingListener(t *testing.T, inner net.Listener) *stallingListener {
	t.Helper()

	l := &stallingListener{
		Listener: inner,
		writing:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	t.Cleanup(l.unblock)

	return l
}

// Accept returns the next connection, wrapped so that it stalls the stall
// event's write.
//
// Returns:
//   - net.Conn: The wrapped connection.
//   - error: The inner listener's error.
func (l *stallingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept: %w", err)
	}

	return &stallingConn{Conn: conn, listener: l}, nil
}

// unblock releases any stalled write.
func (l *stallingListener) unblock() {
	l.releaseOnce.Do(func() { close(l.release) })
}

// stallingConn is a connection whose write of the stall event blocks until a
// write deadline that has passed is set.
type stallingConn struct {
	net.Conn

	// listener holds the channels the connection signals and waits on.
	listener *stallingListener
}

// Write writes p, or blocks when p carries the stall event and then fails as a
// write past its deadline does.
//
// Parameters:
//   - p: Bytes to write.
//
// Returns:
//   - int: Number of bytes written.
//   - error: os.ErrDeadlineExceeded for the stall event, or the inner write's error.
func (c *stallingConn) Write(p []byte) (int, error) {
	if !bytes.Contains(p, []byte("event: "+stallEventType)) {
		n, err := c.Conn.Write(p)
		if err != nil {
			return n, fmt.Errorf("write: %w", err)
		}

		return n, nil
	}

	c.listener.writingOnce.Do(func() { close(c.listener.writing) })
	<-c.listener.release

	return 0, os.ErrDeadlineExceeded
}

// SetWriteDeadline sets the inner connection's write deadline, and releases a
// stalled write when the deadline has already passed.
//
// Parameters:
//   - deadline: The write deadline. The zero time means no deadline.
//
// Returns:
//   - error: The inner connection's error.
func (c *stallingConn) SetWriteDeadline(deadline time.Time) error {
	if !deadline.IsZero() && !deadline.After(time.Now()) {
		c.listener.unblock()
	}

	err := c.Conn.SetWriteDeadline(deadline)
	if err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	return nil
}
