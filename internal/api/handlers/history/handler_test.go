package history

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
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

func TestNew(t *testing.T) {
	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return nil })
	require.NotNil(t, h)
	assert.Equal(t, "/v1/history", h.Path)
}

func TestHandler_Handle(t *testing.T) {
	entries := []metrics.HistoryEntry{
		{Scanned: 5, Updated: 2, Failed: 0, Restarted: 0, Skipped: 3},
		{Scanned: 5, Updated: 1, Failed: 0, Restarted: 0, Skipped: 4},
	}

	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return entries })

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_HandleWithInvalidSince(t *testing.T) {
	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return nil })

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?since=not-a-date", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_HandleWithInvalidLimit(t *testing.T) {
	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return nil })

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?limit=abc", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_HandleWithValidTimeRange(t *testing.T) {
	entries := []metrics.HistoryEntry{
		{Scanned: 3, Updated: 1, Failed: 0, Restarted: 0, Skipped: 2},
	}

	var (
		gotSince, gotUntil *time.Time
		gotLimit           int
	)

	h := New(newTestLogger(t), func(since, until *time.Time, limit int) []metrics.HistoryEntry {
		gotSince = since
		gotUntil = until
		gotLimit = limit

		return entries
	})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	sinceStr := "2024-01-01T00:00:00Z"
	untilStr := "2024-06-01T00:00:00Z"

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/v1/history?since="+sinceStr+"&until="+untilStr+"&limit=10",
		nil,
	)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, gotSince)
	assert.NotNil(t, gotUntil)
	assert.Equal(t, 10, gotLimit)
}

func TestHandler_HandleWithInvalidUntil(t *testing.T) {
	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return nil })

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?until=not-a-date", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_HandleWithNegativeLimit(t *testing.T) {
	h := New(newTestLogger(t), func(_, _ *time.Time, _ int) []metrics.HistoryEntry { return nil })

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?limit=-5", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandler_Handle_WithTimezoneOffset(t *testing.T) {
	var capturedSince *time.Time

	h := New(newTestLogger(t), func(since, _ *time.Time, _ int) []metrics.HistoryEntry {
		capturedSince = since

		return nil
	})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?since=2024-06-15T12:30:45%2B05:30", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, capturedSince)

	_, offset := capturedSince.Zone()
	assert.Equal(t, 5*3600+30*60, offset, "should preserve timezone offset (+05:30)")
}

func TestHandler_Handle_WithFractionalSeconds(t *testing.T) {
	var capturedUntil *time.Time

	h := New(newTestLogger(t), func(_, until *time.Time, _ int) []metrics.HistoryEntry {
		capturedUntil = until

		return nil
	})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?until=2024-06-15T12:30:45.123Z", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, capturedUntil)
	assert.Equal(t, 123000000, capturedUntil.Nanosecond(), "should preserve fractional seconds")
}

func TestHandler_Handle_NegativeTimezone(t *testing.T) {
	var capturedSince *time.Time

	h := New(newTestLogger(t), func(since, _ *time.Time, _ int) []metrics.HistoryEntry {
		capturedSince = since

		return nil
	})

	app := fiber.New(fiber.Config{})
	app.Get("/v1/history", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/history?since=2024-06-15T12:30:45-08:00", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, capturedSince)

	_, offset := capturedSince.Zone()
	assert.Equal(t, -8*3600, offset, "should preserve negative timezone offset")
}
