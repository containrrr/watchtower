package routes

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/check"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/update"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func registerCheckRoute(app *fiber.App, auth fiber.Handler, opts config.Options) {
	if opts.Client == nil {
		return
	}

	checkTimeout := opts.CheckTimeout
	if checkTimeout <= 0 {
		checkTimeout = config.DefaultCheckTimeout
	}

	params := config.BuildUpdateParams(opts)
	scanStartedData := events.NewScanStartedData(params)

	handler := check.New(opts.Logger,
		func(ctx context.Context, images, names []string) ([]check.ContainerCheck, error) {
			imageFilter := opts.FilterByImage(images, opts.Filter)
			containerFilter := update.ContainerFilter(names)
			combinedFilter := func(c types.FilterableContainer) bool {
				return imageFilter(c) && containerFilter(c.Name(), true)
			}

			return check.CheckForUpdates(
				opts.Logger,
				ctx,
				opts.Client,
				combinedFilter,
				params,
				opts.GitClient,
			)
		},
		checkTimeout,
		opts.Notifier,
		opts.NotificationSplitByContainer,
		opts.EventBroadcaster,
		scanStartedData,
	)

	app.Post(
		handler.Path,
		auth,
		timeout.New(
			handler.Handle,
			timeout.Config{
				Timeout: checkTimeout,
			},
		),
	)
}
