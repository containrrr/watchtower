package routes

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/images"
)

func registerImagesRoute(app *fiber.App, auth fiber.Handler, opts config.Options) {
	if opts.Client == nil {
		return
	}

	handler := images.New(opts.Logger, func(ctx context.Context) ([]images.ImageStatus, error) {
		return images.ListImageStatuses(ctx, opts.Client, opts.Filter)
	})
	app.Get(handler.Path, auth, config.TimeoutMiddleware(), handler.Handle)
}
