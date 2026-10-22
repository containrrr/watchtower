package routes

import (
	"github.com/gofiber/fiber/v3"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/metrics"
)

func registerMetricsRoute(app *fiber.App, auth fiber.Handler, opts config.Options) {
	if opts.DefaultMetrics == nil {
		return
	}

	handler := metrics.New(opts.Logger)
	app.Get(handler.Path, auth, handler.Handle)

	statusHandler := metrics.NewStatusHandler(opts.Logger, opts.DefaultMetrics().GetLastScan)
	app.Get(statusHandler.Path, auth, config.TimeoutMiddleware(), statusHandler.Handle)
}
