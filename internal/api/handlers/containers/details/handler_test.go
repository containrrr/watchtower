package details

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
	h := New(newTestLogger(t), func(_ context.Context, _, _ string) ([]ContainerDetails, error) {
		return nil, nil
	})
	require.NotNil(t, h)
	assert.Equal(t, "/v1/containers/details", h.Path)
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		getDetails func(ctx context.Context, name, image string) ([]ContainerDetails, error)
		wantStatus int
	}{
		{
			name: "successful list returns 200",
			getDetails: func(_ context.Context, _, _ string) ([]ContainerDetails, error) {
				return []ContainerDetails{
					{Name: "container1", Image: "nginx:latest", Running: true},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty list returns 200",
			getDetails: func(_ context.Context, _, _ string) ([]ContainerDetails, error) {
				return []ContainerDetails{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "error returns 500",
			getDetails: func(_ context.Context, _, _ string) ([]ContainerDetails, error) {
				return nil, errors.New("docker error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(newTestLogger(t), tt.getDetails)
			app := fiber.New(fiber.Config{})
			app.Get("/v1/containers/details", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers/details", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}
