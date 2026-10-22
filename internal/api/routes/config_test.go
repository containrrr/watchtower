package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
)

func TestRegisterConfigRoute(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	opts := config.Options{
		EnableConfigAPI: true,
	}

	registerConfigRoute(app, auth, opts)

	routes := app.GetRoutes()
	found := false

	for _, r := range routes {
		if r.Path == "/v1/config" && r.Method == http.MethodGet {
			found = true

			break
		}
	}

	assert.True(t, found, "GET /v1/config should be registered")
}
