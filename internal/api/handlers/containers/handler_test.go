package containers

import (
	"context"
	"encoding/json"
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
	tests := []struct {
		name string
		list ListFunc
	}{
		{
			name: "with list function",
			list: func(_ context.Context) ([]Status, error) { return nil, nil },
		},
		{
			name: "with nil list function",
			list: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(newTestLogger(t), tt.list)
			require.NotNil(t, h)
			assert.Equal(t, "/v1/containers", h.Path)
		})
	}
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		listFunc   ListFunc
		wantStatus int
	}{
		{
			name: "successful list returns 200",
			listFunc: func(_ context.Context) ([]Status, error) {
				return []Status{
					{Name: "container1", Image: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def"},
				}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "empty list returns 200",
			listFunc: func(_ context.Context) ([]Status, error) {
				return []Status{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "list error returns 500",
			listFunc: func(_ context.Context) ([]Status, error) {
				return nil, errors.New("docker error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(newTestLogger(t), tt.listFunc)
			app := fiber.New(fiber.Config{})
			app.Get("/v1/containers", h.Handle)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestHandler_Handle_FilterByName(t *testing.T) {
	statuses := []Status{
		{Name: "nginx-proxy", Image: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def"},
		{Name: "redis-cache", Image: "redis:7", ImageID: "sha256:123", Digest: "sha256:456"},
	}

	h := New(newTestLogger(t), func(_ context.Context) ([]Status, error) { return statuses, nil })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/containers", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers?name=nginx-proxy", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	containers := result["containers"].([]any)
	assert.Len(t, containers, 1)

	first := containers[0].(map[string]any)
	assert.Equal(t, "nginx-proxy", first["name"])
}

func TestHandler_Handle_FilterByImage(t *testing.T) {
	statuses := []Status{
		{Name: "nginx-proxy", Image: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def"},
		{Name: "redis-cache", Image: "redis:7", ImageID: "sha256:123", Digest: "sha256:456"},
	}

	h := New(newTestLogger(t), func(_ context.Context) ([]Status, error) { return statuses, nil })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/containers", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers?image=redis:7", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	containers := result["containers"].([]any)
	assert.Len(t, containers, 1)

	first := containers[0].(map[string]any)
	assert.Equal(t, "redis-cache", first["name"])
}

func TestHandler_Handle_FilterByNameAndImage(t *testing.T) {
	statuses := []Status{
		{Name: "nginx-proxy", Image: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def"},
		{Name: "redis-cache", Image: "redis:7", ImageID: "sha256:123", Digest: "sha256:456"},
		{Name: "mysql-db", Image: "mysql:8.0", ImageID: "sha256:789", Digest: "sha256:012"},
	}

	h := New(newTestLogger(t), func(_ context.Context) ([]Status, error) { return statuses, nil })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/containers", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers?name=redis-cache&image=mysql:8.0", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.InDelta(t, float64(0), result["count"], 0.001, "no container should match both name AND image simultaneously")
}

func TestHandler_Handle_NoMatchFilter(t *testing.T) {
	statuses := []Status{
		{Name: "nginx-proxy", Image: "nginx:latest", ImageID: "sha256:abc", Digest: "sha256:def"},
	}

	h := New(newTestLogger(t), func(_ context.Context) ([]Status, error) { return statuses, nil })
	app := fiber.New(fiber.Config{})
	app.Get("/v1/containers", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/containers?name=nonexistent", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.InDelta(t, float64(0), result["count"], 0.001)
}
