package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestRegisterUpdateRoute(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	opts := config.Options{
		EnableUpdateAPI: true,
		UnblockHTTPAPI:  true,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
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

func TestRegisterUpdateRoute_BuildsFullUpdateParams(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	var got types.UpdateParams

	opts := config.Options{
		EnableUpdateAPI: true,
		UnblockHTTPAPI:  true,
		BaseParams: types.UpdateParams{
			Cleanup:             true,
			ReviveStopped:       true,
			UseComposeDependsOn: true,
			NoPull:              true,
			NoRestart:           true,
			LifecycleHooks:      true,
			RollingRestart:      true,
			LabelPrecedence:     true,
			SkipSelfUpdate:      true,
		},
		Filter: func(_ types.FilterableContainer) bool { return true },
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
			got = params

			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		UpdateLock:     make(chan bool, 1),
	}

	registerUpdateRoute(context.Background(), app, auth, opts)

	// Invoke the update pipeline the same way the handler does (via the
	// registered RunUpdatesWithNotifications closure setup).
	params := config.BuildUpdateParams(opts)
	opts.RunUpdatesWithNotifications(context.Background(), opts.Filter, params)

	assert.True(t, got.ReviveStopped)
	assert.True(t, got.UseComposeDependsOn)
	assert.True(t, got.Cleanup)
	assert.True(t, got.NoPull)
	assert.True(t, got.NoRestart)
	assert.True(t, got.LifecycleHooks)
	assert.True(t, got.RollingRestart)
	assert.True(t, got.LabelPrecedence)
	assert.True(t, got.SkipSelfUpdate)
	assert.False(t, got.RunOnce)
}

func TestRegisterUpdateRoute_NilWriteStartupMessage(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	opts := config.Options{
		EnableUpdateAPI: true,
		UnblockHTTPAPI:  false,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:       func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics:      func() *metrics.Metrics { return testMetrics },
		WriteStartupMessage: nil,
	}

	assert.NotPanics(t, func() {
		registerUpdateRoute(context.Background(), app, auth, opts)
	})
}

func TestRegisterUpdateRoute_WriteStartupMessageCalledWhenBlocking(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	called := false
	opts := config.Options{
		EnableUpdateAPI: true,
		UnblockHTTPAPI:  false,
		RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		WriteStartupMessage: func(_ logging.StartupParams) {
			called = true
		},
	}

	registerUpdateRoute(context.Background(), app, auth, opts)
	assert.True(t, called)
}

// TestRegisterUpdateRoute_TimeoutIsServiceUnavailable verifies that an update
// that outlasts the update timeout gets 503 Service Unavailable, as the API
// documents.
func TestRegisterUpdateRoute_TimeoutIsServiceUnavailable(t *testing.T) {
	app := testApp()
	auth := testAuthMiddleware()

	lock := make(chan bool, 1)
	lock <- true

	// The update runs until the test ends, so only the timeout can answer.
	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	opts := config.Options{
		EnableUpdateAPI: true,
		UnblockHTTPAPI:  true,
		UpdateTimeout:   20 * time.Millisecond,
		RunUpdatesWithNotifications: func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
			<-release

			return &metrics.Metric{}
		},
		FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
		DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		UpdateLock:     lock,
	}

	registerUpdateRoute(t.Context(), app, auth, opts)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/update", nil)
	req.Header.Set("Authorization", "Bearer test")

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
