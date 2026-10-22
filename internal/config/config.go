package config

import (
	"github.com/nicholas-fedor/watchtower/internal/config/api"
	"github.com/nicholas-fedor/watchtower/internal/config/client"
	"github.com/nicholas-fedor/watchtower/internal/config/compatibility"
	"github.com/nicholas-fedor/watchtower/internal/config/docker"
	"github.com/nicholas-fedor/watchtower/internal/config/filter"
	"github.com/nicholas-fedor/watchtower/internal/config/git"
	"github.com/nicholas-fedor/watchtower/internal/config/lifecycle"
	"github.com/nicholas-fedor/watchtower/internal/config/logging"
	"github.com/nicholas-fedor/watchtower/internal/config/mode"
	"github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/config/schedule"
	"github.com/nicholas-fedor/watchtower/internal/config/update"
)

// Config is the immutable process configuration snapshot produced by Load.
//
// Each field is a domain group aligned with internal/flags/<domain> registration.
// Use UpdateParams and ClientOptions to project this snapshot into the DTOs
// consumed by the update pipeline and Docker client construction.
type Config struct {
	// Docker holds Docker daemon connection settings (host, TLS, API version, cert path).
	Docker docker.Docker
	// Client holds options passed to container.NewClient (include-stopped, revive-stopped, etc.).
	Client client.Client
	// Schedule holds poll interval, cron spec, and update-on-start.
	Schedule schedule.Schedule
	// Mode holds process entry shape (run-once, health-check, porcelain, orchestrator).
	Mode mode.Mode
	// Update holds container update policy (cleanup, no-pull, compose depends-on, timeouts).
	Update update.Update
	// Lifecycle holds pre/post update lifecycle hook defaults (enable, UID, GID).
	Lifecycle lifecycle.Lifecycle
	// Filter holds container selection inputs and the resolved predicate.
	Filter filter.Filter
	// Registry holds registry TLS settings.
	Registry registry.Registry
	// Compatibility holds runtime compatibility options (for example Podman CPU/memory).
	Compatibility compatibility.Compatibility
	// API holds HTTP API transport, auth, and endpoint settings.
	API api.API
	// Notify holds notification URLs and related options for notifications.NewNotifier.
	Notify notify.Notify
	// Logging holds console log format and level settings.
	Logging logging.Logging
	// Git holds Git association mappings and watcher credentials.
	Git git.Git
}
