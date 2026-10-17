package metrics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
)

// testLogger returns a logger that discards every event, for tests that do not
// assert on logs.
func testLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

func TestNew(t *testing.T) {
	h := New(testLogger())
	require.NotNil(t, h)
	assert.Equal(t, "/v1/metrics", h.Path)
	assert.NotNil(t, h.Handle)
}

func TestNew_ServesPrometheusMetrics(t *testing.T) {
	h := New(testLogger())
	require.NotNil(t, h)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/metrics", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/metrics", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify Prometheus format response
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Should contain Go runtime metrics (from default Prometheus registry)
	assert.NotEmpty(t, body, "response body should not be empty")
	// Prometheus text format uses # for comments and has metric names
	assert.Contains(t, body, "# HELP")
	assert.Contains(t, body, "# TYPE")
}

func TestNew_ReturnsTextContentType(t *testing.T) {
	h := New(testLogger())
	require.NotNil(t, h)

	app := fiber.New(fiber.Config{})
	app.Get("/v1/metrics", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/metrics", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	assert.True(t, strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/html"),
		"expected text content type, got: %s", contentType)
}

func TestNewStatusHandler(t *testing.T) {
	tests := []struct {
		name    string
		getLast func() *metrics.Metric
	}{
		{
			name: "returns non-nil metric",
			getLast: func() *metrics.Metric {
				return &metrics.Metric{Scanned: 5, Updated: 2, Failed: 1, Restarted: 0, Skipped: 2}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewStatusHandler(testLogger(), tt.getLast)
			require.NotNil(t, h)
			assert.Equal(t, "/v1/status", h.Path)
		})
	}
}

func TestStatusHandler_Handle_WithMetric(t *testing.T) {
	expected := &metrics.Metric{Scanned: 10, Updated: 3, Failed: 1, Restarted: 2, Skipped: 0}

	h := NewStatusHandler(testLogger(), func() *metrics.Metric { return expected })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/status", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestStatusHandler_Handle_NilMetric(t *testing.T) {
	h := NewStatusHandler(testLogger(), func() *metrics.Metric { return nil })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/status", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestStatusHandler_Handle_ScanFailed verifies that the status summary reports
// a failed scan, and leaves the field out for a successful one.
func TestStatusHandler_Handle_ScanFailed(t *testing.T) {
	tests := []struct {
		name   string
		metric *metrics.Metric
		// want is the scan_failed value, or nil when the field is absent.
		want any
	}{
		{name: "failed scan", metric: &metrics.Metric{Scanned: 2, ScanFailed: true}, want: true},
		{name: "successful scan", metric: &metrics.Metric{Scanned: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewStatusHandler(testLogger(), func() *metrics.Metric { return tt.metric })
			app := fiber.New(fiber.Config{})
			app.Get("/v1/status", h.Handle)

			resp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/status", nil))
			require.NoError(t, err)

			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var got struct {
				Summary map[string]any `json:"summary"`
			}

			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, tt.want, got.Summary["scan_failed"])
		})
	}
}
