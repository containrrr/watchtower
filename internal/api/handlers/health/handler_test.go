package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
)

func TestNewLivenessHandler(t *testing.T) {
	h := NewLivenessHandler()
	require.NotNil(t, h)
	assert.Equal(t, "/livez", h.Path)
}

func TestLivenessHandler_Handle(t *testing.T) {
	h := NewLivenessHandler()
	app := fiber.New(fiber.Config{})
	app.Get("/livez", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/livez", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewReadinessHandler(t *testing.T) {
	tests := []struct {
		name   string
		client *mockContainer.MockClient
	}{
		{
			name:   "with mock client",
			client: mockContainer.NewMockClient(t),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewReadinessHandler(tt.client)
			require.NotNil(t, h)
			assert.Equal(t, "/readyz", h.Path)
		})
	}
}

func TestReadinessHandler_Handle_NilClient(t *testing.T) {
	h := NewReadinessHandler(nil)
	app := fiber.New(fiber.Config{})
	app.Get("/readyz", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestReadinessHandler_Handle_HealthyClient(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	client.EXPECT().Ping(mock.Anything).Return(nil)

	h := NewReadinessHandler(client)
	app := fiber.New(fiber.Config{})
	app.Get("/readyz", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestReadinessHandler_Handle_UnhealthyClient(t *testing.T) {
	client := mockContainer.NewMockClient(t)
	client.EXPECT().Ping(mock.Anything).Return(errors.New("connection refused"))

	h := NewReadinessHandler(client)
	app := fiber.New(fiber.Config{})
	app.Get("/readyz", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestNewStartupHandler(t *testing.T) {
	h := NewStartupHandler()
	require.NotNil(t, h)
	assert.Equal(t, "/startupz", h.Path)
}

func TestStartupHandler_Handle(t *testing.T) {
	h := NewStartupHandler()
	app := fiber.New(fiber.Config{})
	app.Get("/startupz", h.Handle)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/startupz", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
