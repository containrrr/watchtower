package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
)

func TestNew_ProxyConfig(t *testing.T) {
	app := New(
		testLogger(),
		60,
		ProxyConfig{
			TrustedProxies: []string{"127.0.0.1"},
			ProxyHeader:    "X-Real-IP",
		},
		CORSConfig{},
		false,
	)
	assert.NotNil(t, app)

	app2 := New(
		testLogger(),
		60,
		ProxyConfig{
			TrustedProxies: []string{"10.0.0.0/8"},
		},
		CORSConfig{},
		false,
	)
	assert.NotNil(t, app2)
}

func TestNew_CORSConfig(t *testing.T) {
	app := New(
		testLogger(),
		60,
		ProxyConfig{},
		CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
		},
		false,
	)
	assert.NotNil(t, app)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodOptions,
		"/test",
		nil,
	)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	assert.Contains(t, allowOrigin, "https://example.com")
}

func TestNew_CORSWildcard(t *testing.T) {
	app := New(
		testLogger(),
		60,
		ProxyConfig{},
		CORSConfig{
			AllowedOrigins: []string{"*"},
		},
		false,
	)
	assert.NotNil(t, app)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodOptions,
		"/test",
		nil,
	)
	req.Header.Set("Origin", "https://any-origin.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	assert.Equal(t, "*", allowOrigin)

	allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowMethods, "GET")
	assert.Contains(t, allowMethods, "POST")
}

func TestNew_CORSCustomMethodsHeaders(t *testing.T) {
	app := New(
		testLogger(),
		60,
		ProxyConfig{},
		CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type", "X-Custom-Header"},
		},
		false,
	)
	assert.NotNil(t, app)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodOptions,
		"/test",
		nil,
	)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header")

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowMethods, "GET")
	assert.Contains(t, allowMethods, "POST")

	allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowHeaders, "X-Custom-Header")
}

func TestNew_NoCORSWhenNoOrigins(t *testing.T) {
	app := New(
		testLogger(),
		60,
		ProxyConfig{},
		CORSConfig{},
		false,
	)
	assert.NotNil(t, app)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/test",
		nil,
	)
	req.Header.Set("Origin", "https://example.com")

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	assert.Empty(t, allowOrigin, "no CORS header should be set when no origins configured")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name               string
		log                *zerolog.Logger
		rateLimitPerMinute int
		wantNil            bool
	}{
		{
			name:               "default rate limit",
			log:                testLogger(),
			rateLimitPerMinute: 60,
			wantNil:            false,
		},
		{
			name:               "zero rate limit falls back to default",
			log:                testLogger(),
			rateLimitPerMinute: 0,
			wantNil:            false,
		},
		{
			name:               "negative rate limit falls back to default",
			log:                testLogger(),
			rateLimitPerMinute: -1,
			wantNil:            false,
		},
		{
			name:               "custom rate limit",
			log:                testLogger(),
			rateLimitPerMinute: 120,
			wantNil:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(
				tt.log,
				tt.rateLimitPerMinute,
				ProxyConfig{},
				CORSConfig{},
				false,
			)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.IsType(t, &fiber.App{}, got)
			}
		})
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	handler := config.TimeoutMiddleware()
	require.NotNil(t, handler)

	app := fiber.New(fiber.Config{})
	app.Get("/test", handler, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/test",
		nil,
	)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestTimeoutMiddleware_Timeout verifies that a request whose handler
// outlasts the timeout gets 503 Service Unavailable, which the API documents.
func TestTimeoutMiddleware_Timeout(t *testing.T) {
	const testTimeout = 10 * time.Millisecond

	handler := timeout.New(func(c fiber.Ctx) error {
		return c.Next()
	}, timeout.Config{
		Timeout: testTimeout,
	})
	require.NotNil(t, handler)

	app := fiber.New(fiber.Config{})
	app.Get("/test", handler, func(c fiber.Ctx) error {
		<-c.Context().Done()

		// The deadline error counts as a timeout even if the handler returns
		// before the middleware notices the deadline.
		return c.Context().Err()
	})

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/test",
		nil,
	)

	resp, err := app.Test(req)
	require.NoError(t, err)

	synctest.Test(t, func(t *testing.T) {
		defer resp.Body.Close()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
}

// lockedBuffer is a bytes.Buffer that is safe to write from a server goroutine
// while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends p to the buffer.
func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String returns the buffer contents.
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestNew_OnListenHook(t *testing.T) {
	// The listen hook logs from the server goroutine while the test polls.
	var buf lockedBuffer

	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	app := New(&logger, 60, ProxyConfig{}, CORSConfig{}, false)

	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	go func() {
		_ = app.Listen("127.0.0.1:0", fiber.ListenConfig{
			DisableStartupMessage: true,
		})
	}()

	// Wait for the second of the two listen messages.
	require.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "HTTP API server is enabled")
	}, 2*time.Second, 50*time.Millisecond)

	output := buf.String()
	assert.Contains(t, output, "Starting HTTP API server")
	assert.Contains(t, output, "HTTP API server is enabled")
	assert.Contains(t, output, `"host"`)
	assert.Contains(t, output, `"port"`)
	assert.Contains(t, output, `"tls"`)

	_ = app.Shutdown()
}
