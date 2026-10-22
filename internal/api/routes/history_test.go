package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
)

func TestRegisterHistoryRoute(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	opts := config.Options{
		EnableHistoryAPI: true,
		DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
	}

	registerHistoryRoute(app, auth, opts)

	routes := app.GetRoutes()
	found := false

	for _, r := range routes {
		if r.Path == "/v1/history" && r.Method == http.MethodGet {
			found = true

			break
		}
	}

	assert.True(t, found, "GET /v1/history should be registered")
}
