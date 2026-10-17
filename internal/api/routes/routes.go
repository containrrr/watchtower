package routes

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	_ "github.com/nicholas-fedor/watchtower/internal/api/swagger"
)

var (
	// errMissingClientForCheckAPI is returned when the check API is enabled
	// but no Docker client is configured.
	errMissingClientForCheckAPI = errors.New("check API is enabled but no Docker client is configured")

	// errMissingClientForContainersAPI is returned when the containers API
	// is enabled but no Docker client is configured.
	errMissingClientForContainersAPI = errors.New("containers API is enabled but no Docker client is configured")

	// errMissingClientForImagesAPI is returned when the images API is enabled
	// but no Docker client is configured.
	errMissingClientForImagesAPI = errors.New("images API is enabled but no Docker client is configured")
)

// ValidateAndRegister validates options and registers enabled routes.
//
// Parameters:
//   - ctx: Request context for long-lived handlers.
//   - app: Fiber application.
//   - auth: Middleware for protected /v1/* routes (Bearer/raw token/cookie).
//   - opts: API configuration options.
//
// Returns:
//   - error: Non-nil if validation fails.
func ValidateAndRegister(
	ctx context.Context,
	app *fiber.App,
	auth fiber.Handler,
	opts config.Options,
) error {
	if opts.EnableUpdateAPI {
		err := config.ValidateUpdateOptions(opts)
		if err != nil {
			return fmt.Errorf("update options validation failed: %w", err)
		}
	}

	if opts.EnableCheckAPI && opts.FilterByImage == nil {
		return config.ErrMissingFilterByImage
	}

	// Metrics and history read the in-memory metrics store.
	if (opts.EnableMetricsAPI || opts.EnableHistoryAPI) && opts.DefaultMetrics == nil {
		return config.ErrMissingDefaultMetrics
	}

	if opts.EnableCheckAPI && opts.Client == nil {
		return errMissingClientForCheckAPI
	}

	if opts.EnableContainersAPI && opts.Client == nil {
		return errMissingClientForContainersAPI
	}

	if opts.EnableImagesAPI && opts.Client == nil {
		return errMissingClientForImagesAPI
	}

	if opts.EnableEventsAPI && opts.EventBroadcaster == nil {
		return config.ErrMissingEventBroadcaster
	}

	registerRoutes(ctx, app, auth, opts)

	return nil
}

// registerRoutes mounts enabled HTTP API routes on the Fiber app.
//
// Parameters:
//   - ctx: Request context for long-lived handlers.
//   - app: Fiber application.
//   - auth: Middleware for protected /v1/* routes.
//   - opts: API configuration options.
func registerRoutes(
	ctx context.Context,
	app *fiber.App,
	auth fiber.Handler,
	opts config.Options,
) {
	if opts.EnableHealthAPI {
		registerHealthRoute(app, opts)
	}

	if opts.EnableUpdateAPI {
		registerUpdateRoute(ctx, app, auth, opts)
	}

	if opts.EnableMetricsAPI {
		registerMetricsRoute(app, auth, opts)
	}

	if opts.EnableContainersAPI {
		registerContainersRoute(app, auth, opts)
		registerContainersDetailsRoute(app, auth, opts)
	}

	if opts.EnableCheckAPI {
		registerCheckRoute(app, auth, opts)
	}

	if opts.EnableHistoryAPI {
		registerHistoryRoute(app, auth, opts)
	}

	if opts.EnableImagesAPI {
		registerImagesRoute(app, auth, opts)
	}

	if opts.EnableConfigAPI {
		registerConfigRoute(app, auth, opts)
	}

	if opts.EnableEventsAPI {
		registerEventsRoute(app, opts)
	}

	if opts.EnableSwaggerAPI {
		registerSwaggerRoute(app, opts)
	}
}
