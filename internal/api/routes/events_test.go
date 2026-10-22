package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
)

func TestExtractEventsToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authHeader string
		query      string
		wantToken  string
		wantOK     bool
	}{
		{
			name:   "missing",
			wantOK: false,
		},
		{
			name:       "bearer",
			authHeader: "Bearer events-secret",
			wantToken:  "events-secret",
			wantOK:     true,
		},
		{
			name:       "raw authorization swagger style",
			authHeader: "events-secret",
			wantToken:  "events-secret",
			wantOK:     true,
		},
		{
			name:      "query access_token",
			query:     "access_token=events-secret",
			wantToken: "events-secret",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()

			var got string

			var ok bool

			app.Get("/v1/events", func(c fiber.Ctx) error {
				got, ok = extractEventsToken(c)

				return c.SendStatus(fiber.StatusOK)
			})

			path := "/v1/events"
			if tt.query != "" {
				path += "?" + tt.query
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)

			_ = resp.Body.Close()

			assert.Equal(t, tt.wantOK, ok)

			if tt.wantOK {
				assert.Equal(t, tt.wantToken, got)
			}
		})
	}
}

func TestRegisterEventsRoute_RejectsMissingToken(t *testing.T) {
	app := testApp()
	opts := config.Options{
		EnableEventsAPI:  true,
		EventsToken:      "events-secret",
		EventBroadcaster: events.NewBroadcaster(),
	}
	registerEventsRoute(app, opts)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/events", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRegisterEventsRoute_HashCompareRejectsWrongLength(t *testing.T) {
	app := testApp()
	opts := config.Options{
		EnableEventsAPI:  true,
		EventsToken:      "events-secret",
		EventBroadcaster: events.NewBroadcaster(),
	}
	registerEventsRoute(app, opts)

	// Different length must not panic and must not authenticate.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/events", nil)
	req.Header.Set("Authorization", "x")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRegisterEventsRoute_RejectsWrongToken(t *testing.T) {
	app := testApp()
	opts := config.Options{
		EnableEventsAPI:  true,
		EventsToken:      "events-secret",
		EventBroadcaster: events.NewBroadcaster(),
	}
	registerEventsRoute(app, opts)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/events", nil)
	req.Header.Set("Authorization", "wrong")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()
}
