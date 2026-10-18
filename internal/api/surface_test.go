package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/routes"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// probeTimeout bounds each route probe so a handler that never responds, such as
// a streaming endpoint, fails the test instead of hanging it.
const probeTimeout = 5 * time.Second

// routeParamPattern matches named route parameters such as ":id".
var routeParamPattern = regexp.MustCompile(`:[A-Za-z0-9_]+`)

// newFullSurfaceApp builds the HTTP API with every endpoint enabled, wired the
// same way as SetupAndStartAPI but without starting a listener.
func newFullSurfaceApp(t *testing.T) *fiber.App {
	t.Helper()

	client := mockContainer.NewMockClient(t)
	client.EXPECT().Ping(mock.Anything).Return(nil).Maybe()
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil).Maybe()
	client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{}, nil).Maybe()

	opts := withTestLogger(t, config.Options{
		Token:               "api-token",
		EventsToken:         "events-token",
		RateLimit:           1000,
		EnableUpdateAPI:     true,
		UpdateLock:          scheduling.NewUpdateLock(),
		EnableMetricsAPI:    true,
		EnableContainersAPI: true,
		EnableCheckAPI:      true,
		EnableSwaggerAPI:    true,
		EnableHealthAPI:     true,
		EnableHistoryAPI:    true,
		EnableImagesAPI:     true,
		EnableConfigAPI:     true,
		EnableEventsAPI:     true,
		UnblockHTTPAPI:      true,
		NoStartupMessage:    true,
		Client:              client,
		Filter:              makeFilter(t),
		RunUpdatesWithNotifications: func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
			return &metrics.Metric{}
		},
		FilterByImage:    func(_ []string, filter types.Filter) types.Filter { return filter },
		DefaultMetrics:   func() *metrics.Metrics { return helperMetrics },
		EventBroadcaster: NewEventsBroadcasterHelper(),
	})

	app := newApp(opts.Logger, opts.RateLimit, proxyConfig{}, corsConfig{}, opts.NoStartupMessage)

	err := routes.ValidateAndRegister(t.Context(), app, newAPIAuthMiddleware(opts.Logger, opts.Token), opts)
	require.NoError(t, err)

	return app
}

// TestHTTPAPIRouteTable snapshots every HTTP API route with all endpoints
// enabled, along with whether it rejects unauthenticated requests.
//
// Authentication is observed by sending each route a request without
// credentials, so the snapshot reflects the middleware actually applied rather
// than how the routes are declared.
func TestHTTPAPIRouteTable(t *testing.T) {
	app := newFullSurfaceApp(t)

	seen := map[string]struct{}{}

	var lines []string

	for _, route := range app.GetRoutes(true) {
		key := route.Method + " " + route.Path
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		status := probeStatus(t, app, route.Method, route.Path)

		var access string

		switch {
		case status == http.StatusUnauthorized:
			access = "requires-auth"
		case status >= http.StatusOK && status < http.StatusMultipleChoices:
			access = "public"
		default:
			t.Errorf("%s %s: unauthenticated request returned %d; expected 401 or a 2xx status",
				route.Method, route.Path, status)

			continue
		}

		lines = append(lines, fmt.Sprintf("%-7s %-24s %s", route.Method, route.Path, access))
	}

	slices.Sort(lines)

	golden.Assert(t, "http-api-routes", []byte(strings.Join(lines, "\n")+"\n"))
}

// probeStatus sends an unauthenticated request to a concrete URL for the route
// pattern and returns the response status code.
func probeStatus(t *testing.T, app *fiber.App, method, pattern string) int {
	t.Helper()

	target := routeParamPattern.ReplaceAllString(pattern, "probe")
	target = strings.ReplaceAll(target, "*", "index.html")

	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: probeTimeout, FailOnTimeout: true})
	require.NoError(t, err, "%s %s", method, target)

	defer resp.Body.Close()

	return resp.StatusCode
}
