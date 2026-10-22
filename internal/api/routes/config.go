package routes

import (
	"context"

	"github.com/gofiber/fiber/v3"

	apiconfig "github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/config"
)

func registerConfigRoute(app *fiber.App, auth fiber.Handler, opts apiconfig.Options) {
	handler := config.New(opts.Logger, func(_ context.Context) (config.ConfigData, error) {
		return config.ConfigData{
			MonitorOnly:       opts.BaseParams.MonitorOnly,
			Cleanup:           opts.BaseParams.Cleanup,
			NoPull:            opts.BaseParams.NoPull,
			NoRestart:         opts.BaseParams.NoRestart,
			RollingRestart:    opts.BaseParams.RollingRestart,
			IncludeStopped:    opts.IncludeStopped,
			IncludeRestarting: opts.IncludeRestarting,
			LifecycleHooks:    opts.BaseParams.LifecycleHooks,
			LabelEnable:       opts.LabelEnable,
			FilterDesc:        opts.FilterDesc,
			Scope:             opts.Scope,
			GitEnable:         opts.BaseParams.EnableGitMonitoring,
		}, nil
	})
	app.Get(handler.Path, auth, apiconfig.TimeoutMiddleware(), handler.Handle)
}
