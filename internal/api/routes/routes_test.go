package routes

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var testMetrics = metrics.Default()

func makeFilter(_ *testing.T) types.Filter {
	return func(_ types.FilterableContainer) bool { return true }
}

// testApp creates a minimal Fiber app for route registration tests.
func testApp() *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit:     1 << 20,
		ReadTimeout:   10 * time.Second,
		IdleTimeout:   30 * time.Second,
		StrictRouting: true,
		CaseSensitive: true,
	})
	app.Use(
		recover.New(),
		helmet.New(),
		requestid.New(),
		logger.New(logger.Config{
			Stream: io.Discard,
			Format: "${status} - ${method} ${path}\n",
		}),
		compress.New(),
		limiter.New(limiter.Config{
			Max:               60,
			Expiration:        time.Minute,
			LimiterMiddleware: limiter.SlidingWindow{},
			KeyGenerator:      func(c fiber.Ctx) string { return c.IP() },
		}),
	)

	return app
}

// testAuthMiddleware creates a Bearer token auth middleware for testing.
func testAuthMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		expectedHash := sha256.Sum256([]byte("test"))

		extracted, err := extractors.FromAuthHeader("Bearer").Extract(c)
		if err != nil {
			extracted, err = extractors.FromCookie("access_token").Extract(c)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).SendString(keyauth.ErrMissingOrMalformedAPIKey.Error())
			}
		}

		providedHash := sha256.Sum256([]byte(extracted))
		if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
			return c.Status(fiber.StatusUnauthorized).SendString(keyauth.ErrMissingOrMalformedAPIKey.Error())
		}

		return c.Next()
	}
}

func TestValidateAndRegister(t *testing.T) {
	baseOpts := func() config.Options {
		return config.Options{
			EnableUpdateAPI: true,
			UpdateLock:      scheduling.NewUpdateLock(),
			UnblockHTTPAPI:  true,
			RunUpdatesWithNotifications: func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
				return &metrics.Metric{}
			},
			FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
			DefaultMetrics: func() *metrics.Metrics { return testMetrics },
		}
	}

	tests := []struct {
		name    string
		opts    config.Options
		wantErr bool
		errMsg  string
	}{
		{name: "all valid", opts: baseOpts(), wantErr: false},
		{
			name: "missing RunUpdatesWithNotifications",
			opts: func() config.Options {
				o := baseOpts()
				o.RunUpdatesWithNotifications = nil

				return o
			}(),
			wantErr: true,
			errMsg:  "RunUpdatesWithNotifications must be provided",
		},
		{
			name: "missing FilterByImage",
			opts: func() config.Options {
				o := baseOpts()
				o.FilterByImage = nil

				return o
			}(),
			wantErr: true,
			errMsg:  "FilterByImage must be provided",
		},
		{
			name: "missing DefaultMetrics for update",
			opts: func() config.Options {
				o := baseOpts()
				o.DefaultMetrics = nil

				return o
			}(),
			wantErr: true,
			errMsg:  "DefaultMetrics must be provided",
		},
		{
			name: "history without client is valid",
			opts: config.Options{
				EnableHistoryAPI: true,
				DefaultMetrics:   func() *metrics.Metrics { return testMetrics },
			},
			wantErr: false,
		},
		{
			name: "history without DefaultMetrics fails",
			opts: config.Options{
				EnableHistoryAPI: true,
				DefaultMetrics:   nil,
			},
			wantErr: true,
			errMsg:  "DefaultMetrics must be provided",
		},
		{
			name: "metrics without DefaultMetrics fails",
			opts: config.Options{
				EnableMetricsAPI: true,
				DefaultMetrics:   nil,
			},
			wantErr: true,
			errMsg:  "DefaultMetrics must be provided",
		},
		{
			name: "check without FilterByImage fails",
			opts: config.Options{
				EnableCheckAPI: true,
				FilterByImage:  nil,
				Client:         mockContainer.NewMockClient(t),
			},
			wantErr: true,
			errMsg:  "FilterByImage must be provided",
		},
		{
			name: "check with FilterByImage is valid",
			opts: config.Options{
				EnableCheckAPI: true,
				FilterByImage:  func(_ []string, f types.Filter) types.Filter { return f },
				Client:         mockContainer.NewMockClient(t),
			},
			wantErr: false,
		},
		{
			name: "events without EventBroadcaster fails",
			opts: config.Options{
				EnableEventsAPI:  true,
				EventBroadcaster: nil,
			},
			wantErr: true,
			errMsg:  "EventBroadcaster must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp()
			auth := testAuthMiddleware()

			err := ValidateAndRegister(context.Background(), app, auth, tt.opts)

			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name                string
		enableUpdateAPI     bool
		enableMetricsAPI    bool
		enableContainersAPI bool
		enableHistoryAPI    bool
		enableImagesAPI     bool
		enableConfigAPI     bool
		enableEventsAPI     bool
		wantCount           int
	}{
		{name: "only update", enableUpdateAPI: true, wantCount: 1},
		{name: "only metrics", enableMetricsAPI: true, wantCount: 2},
		{name: "only containers", enableContainersAPI: true, wantCount: 2},
		{name: "only history", enableHistoryAPI: true, wantCount: 1},
		{name: "only images", enableImagesAPI: true, wantCount: 1},
		{name: "only config", enableConfigAPI: true, wantCount: 1},
		{name: "only events", enableEventsAPI: true, wantCount: 1},
		{
			name:                "all APIs",
			enableUpdateAPI:     true,
			enableMetricsAPI:    true,
			enableContainersAPI: true,
			enableHistoryAPI:    true,
			enableImagesAPI:     true,
			enableConfigAPI:     true,
			enableEventsAPI:     true,
			wantCount:           9,
		},
		{name: "none", wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp()
			auth := testAuthMiddleware()

			var updateFn func(ctx context.Context, f types.Filter, p types.UpdateParams) *metrics.Metric
			if tt.enableUpdateAPI {
				updateFn = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
					return &metrics.Metric{}
				}
			}

			opts := config.Options{
				EnableUpdateAPI:             tt.enableUpdateAPI,
				UnblockHTTPAPI:              true,
				RunUpdatesWithNotifications: updateFn,
				FilterByImage:               func(_ []string, f types.Filter) types.Filter { return f },
				DefaultMetrics:              func() *metrics.Metrics { return testMetrics },
				EnableMetricsAPI:            tt.enableMetricsAPI,
				EnableContainersAPI:         tt.enableContainersAPI,
				EnableHistoryAPI:            tt.enableHistoryAPI,
				EnableImagesAPI:             tt.enableImagesAPI,
				EnableConfigAPI:             tt.enableConfigAPI,
				EnableEventsAPI:             tt.enableEventsAPI,
				Client:                      mockContainer.NewMockClient(t),
				Filter:                      makeFilter(t),
			}
			if tt.enableEventsAPI {
				opts.EventBroadcaster = events.NewBroadcaster()
			}

			Register(context.Background(), app, auth, opts)

			routes := app.GetRoutes()
			apiCount := 0

			for _, r := range routes {
				if r.Path == "/v1/update" || r.Path == "/v1/metrics" || r.Path == "/v1/status" || r.Path == "/v1/containers" || r.Path == "/v1/containers/details" || r.Path == "/v1/history" ||
					r.Path == "/v1/images" ||
					r.Path == "/v1/config" ||
					r.Path == "/v1/events" {
					apiCount++
				}
			}

			assert.Equal(t, tt.wantCount, apiCount)
		})
	}
}
