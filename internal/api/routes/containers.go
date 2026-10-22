package routes

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/containers"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/containers/details"
)

func registerContainersRoute(app *fiber.App, auth fiber.Handler, opts config.Options) {
	if opts.Client == nil {
		return
	}

	handler := containers.New(opts.Logger, func(ctx context.Context) ([]containers.Status, error) {
		return containers.ListContainerStatuses(ctx, opts.Client, opts.Filter)
	})
	app.Get(handler.Path, auth, config.TimeoutMiddleware(), handler.Handle)
}

func registerContainersDetailsRoute(app *fiber.App, auth fiber.Handler, opts config.Options) {
	if opts.Client == nil {
		return
	}

	detailsParams := config.BuildUpdateParams(opts)
	handler := details.New(opts.Logger, func(ctx context.Context, name, image string) ([]details.ContainerDetails, error) {
		return details.GetContainerDetails(ctx, opts.Client, opts.Filter, name, image, detailsParams)
	})
	app.Get(handler.Path, auth, config.TimeoutMiddleware(), handler.Handle)
}
