package routes

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestRegisterCheckRoute(t *testing.T) {
	t.Run("nil client does not register route", func(t *testing.T) {
		app := testApp()
		auth := testAuthMiddleware()

		opts := config.Options{
			EnableCheckAPI: true,
			Client:         nil,
		}

		registerCheckRoute(app, auth, opts)

		routes := app.GetRoutes()
		for _, r := range routes {
			if r.Path == "/v1/check" {
				t.Error("POST /v1/check should not be registered when client is nil")
			}
		}
	})

	t.Run("with client registers route", func(t *testing.T) {
		app := testApp()
		auth := testAuthMiddleware()
		mockClient := mockContainer.NewMockClient(t)

		opts := config.Options{
			EnableCheckAPI:   true,
			Client:           mockClient,
			FilterByImage:    func(_ []string, f types.Filter) types.Filter { return f },
			EventBroadcaster: events.NewBroadcaster(),
		}

		registerCheckRoute(app, auth, opts)

		routes := app.GetRoutes()
		found := false

		for _, r := range routes {
			if r.Path == "/v1/check" && r.Method == http.MethodPost {
				found = true

				break
			}
		}

		assert.True(t, found, "POST /v1/check should be registered")
	})
}

func TestRegisterEventsRoute(t *testing.T) {
	app := testApp()

	opts := config.Options{
		EnableEventsAPI:    true,
		EventsToken:        "events-token",
		CORSAllowedOrigins: []string{"https://example.com"},
		EventBroadcaster:   events.NewBroadcaster(),
	}

	registerEventsRoute(app, opts)

	routes := app.GetRoutes()
	found := false

	for _, r := range routes {
		if r.Path == "/v1/events" && r.Method == http.MethodGet {
			found = true

			break
		}
	}

	assert.True(t, found, "GET /v1/events should be registered")
}

func TestRegisterUpdateRoute_WriteStartupMessage(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	opts := config.Options{
		EnableUpdateAPI: true,
		UpdateLock:      scheduling.NewUpdateLock(),
		UnblockHTTPAPI:  false,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		WriteStartupMessage: func(_ logging.StartupParams) {
		},
	}

	registerUpdateRoute(context.Background(), app, auth, opts)

	routes := app.GetRoutes()
	found := false

	for _, r := range routes {
		if r.Path == "/v1/update" && r.Method == http.MethodPost {
			found = true

			break
		}
	}

	assert.True(t, found, "POST /v1/update should be registered")
}
