package config

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestNew(t *testing.T) {
	h := New(newTestLogger(t), func(_ context.Context) (ConfigData, error) {
		return ConfigData{}, nil
	})
	require.NotNil(t, h)
	assert.Equal(t, "/v1/config", h.Path)
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		getConfig  func(ctx context.Context) (ConfigData, error)
		wantStatus int
	}{
		{
			name: "successful config returns 200",
			getConfig: func(_ context.Context) (ConfigData, error) {
				return ConfigData{
					MonitorOnly: true,
					Cleanup:     true,
					FilterDesc:  "all containers",
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "config error returns 500",
			getConfig: func(_ context.Context) (ConfigData, error) {
				return ConfigData{}, errors.New("config error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(newTestLogger(t), tt.getConfig)
			app := fiber.New(fiber.Config{})
			app.Get("/v1/config", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}
