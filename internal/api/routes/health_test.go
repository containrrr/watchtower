package routes

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
)

func TestRegisterHealthRoute(t *testing.T) {
	tests := []struct {
		name             string
		clientSetup      func(t *testing.T) container.Client
		checkReadiness   bool
		readinessHealthy bool
	}{
		{
			name: "nil client",
			clientSetup: func(t *testing.T) container.Client {
				t.Helper()

				return nil
			},
		},
		{
			name: "working client",
			clientSetup: func(t *testing.T) container.Client {
				t.Helper()

				mc := mockContainer.NewMockClient(t)
				mc.EXPECT().Ping(mock.Anything).Return(nil)

				return mc
			},
			checkReadiness:   true,
			readinessHealthy: true,
		},
		{
			name: "failing client",
			clientSetup: func(t *testing.T) container.Client {
				t.Helper()

				mc := mockContainer.NewMockClient(t)
				mc.EXPECT().Ping(mock.Anything).Return(errors.New("fail"))

				return mc
			},
			checkReadiness:   true,
			readinessHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp()
			client := tt.clientSetup(t)

			opts := config.Options{
				EnableHealthAPI: true,
				Client:          client,
			}

			registerHealthRoute(app, opts)

			routes := app.GetRoutes()
			healthCount := 0

			for _, r := range routes {
				if r.Path == "/livez" || r.Path == "/readyz" || r.Path == "/startupz" {
					healthCount++
				}
			}

			assert.Equal(t, 3, healthCount)

			if tt.checkReadiness && client != nil {
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
				resp, err := app.Test(req)
				require.NoError(t, err)

				defer resp.Body.Close()

				if tt.readinessHealthy {
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				} else {
					assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
				}
			}
		})
	}
}
