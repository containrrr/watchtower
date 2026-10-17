package images

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

// testLogger returns a logger that discards every event, for tests that do not
// assert on logs.
func testLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

func TestNew(t *testing.T) {
	h := New(testLogger(), func(_ context.Context) ([]ImageStatus, error) { return nil, nil })
	require.NotNil(t, h)
	assert.Equal(t, "/v1/images", h.Path)
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		listFunc   func(ctx context.Context) ([]ImageStatus, error)
		wantStatus int
	}{
		{
			name: "successful list returns 200",
			listFunc: func(_ context.Context) ([]ImageStatus, error) {
				return []ImageStatus{
					{Name: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def", Containers: 2},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty list returns 200",
			listFunc: func(_ context.Context) ([]ImageStatus, error) {
				return []ImageStatus{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "list error returns 500",
			listFunc: func(_ context.Context) ([]ImageStatus, error) {
				return nil, errors.New("docker error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(testLogger(), tt.listFunc)
			app := fiber.New(fiber.Config{})
			app.Get("/v1/images", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/images", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}
