package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestRegisterContainersRoute_NilClient(t *testing.T) {
	t.Run("nil client skips containers route", func(t *testing.T) {
		app := testApp()
		auth := testAuthMiddleware()

		opts := config.Options{
			EnableContainersAPI: true,
			Client:              nil,
			UnblockHTTPAPI:      true,
		}

		registerContainersRoute(app, auth, opts)

		routes := app.GetRoutes()

		for _, r := range routes {
			if r.Path == "/v1/containers" {
				t.Error("GET /v1/containers should not be registered when client is nil")
			}
		}
	})

	t.Run("nil client skips details route", func(t *testing.T) {
		app := testApp()
		auth := testAuthMiddleware()

		opts := config.Options{
			EnableContainersAPI: true,
			Client:              nil,
			UnblockHTTPAPI:      true,
		}

		registerContainersDetailsRoute(app, auth, opts)

		routes := app.GetRoutes()

		for _, r := range routes {
			if r.Path == "/v1/containers/details" {
				t.Error("GET /v1/containers/details should not be registered when client is nil")
			}
		}
	})

	t.Run("with client registers routes", func(t *testing.T) {
		app := testApp()
		auth := testAuthMiddleware()
		mockClient := mockContainer.NewMockClient(t)
		mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{}, nil).Maybe()

		opts := config.Options{
			EnableContainersAPI: true,
			Client:              mockClient,
			UnblockHTTPAPI:      true,
		}

		registerContainersRoute(app, auth, opts)
		registerContainersDetailsRoute(app, auth, opts)

		routes := app.GetRoutes()

		foundContainers := false
		foundDetails := false

		for _, r := range routes {
			if r.Path == "/v1/containers" && r.Method == http.MethodGet {
				foundContainers = true
			}

			if r.Path == "/v1/containers/details" && r.Method == http.MethodGet {
				foundDetails = true
			}
		}

		assert.True(t, foundContainers, "GET /v1/containers should be registered")
		assert.True(t, foundDetails, "GET /v1/containers/details should be registered")
	})
}
