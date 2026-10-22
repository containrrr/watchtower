package routes

import (
	"github.com/gofiber/fiber/v3"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/health"
)

func registerHealthRoute(app *fiber.App, opts config.Options) {
	liveness := health.NewLivenessHandler()
	readiness := health.NewReadinessHandler(opts.Client)
	startup := health.NewStartupHandler()

	app.Get(liveness.Path, liveness.Handle)
	app.Get(readiness.Path, readiness.Handle)
	app.Get(startup.Path, startup.Handle)
}
