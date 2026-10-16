package routes

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/update"
	mt "github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func registerUpdateRoute(ctx context.Context, app *fiber.App, auth fiber.Handler, opts config.Options) {
	updateTimeout := opts.UpdateTimeout
	if updateTimeout <= 0 {
		updateTimeout = config.DefaultUpdateTimeout
	}

	handler := update.New(opts.Logger, func(updateCtx context.Context, images, containers []string) *mt.Metric {
		params := config.BuildUpdateParams(opts)

		imageFilter := opts.FilterByImage(images, opts.Filter)

		containerFilter := update.ContainerFilter(containers)
		combinedFilter := func(c types.FilterableContainer) bool {
			return imageFilter(c) && containerFilter(c.Name(), true)
		}

		metric := opts.RunUpdatesWithNotifications(updateCtx, combinedFilter, params)
		opts.DefaultMetrics().RegisterScan(metric)

		return metric
	}, opts.UpdateLock, updateTimeout, ctx)

	app.Post(handler.Path, auth, timeout.New(handler.Handle, timeout.Config{
		Timeout: updateTimeout,
	}))
}
