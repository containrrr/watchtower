package config

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// ConfigData represents the active Watchtower configuration exposed via the API.
type ConfigData struct {
	// MonitorOnly indicates whether Watchtower is in monitor-only mode.
	MonitorOnly bool `json:"monitor_only"`
	// Cleanup indicates whether old images are removed after updating.
	Cleanup bool `json:"cleanup"`
	// NoPull indicates whether image pulling is disabled.
	NoPull bool `json:"no_pull"`
	// NoRestart indicates whether container restarting is disabled.
	NoRestart bool `json:"no_restart"`
	// RollingRestart indicates whether containers are restarted one at a time.
	RollingRestart bool `json:"rolling_restart"`
	// IncludeStopped indicates whether stopped containers are included.
	IncludeStopped bool `json:"include_stopped"`
	// IncludeRestarting indicates whether restarting containers are included.
	IncludeRestarting bool `json:"include_restarting"`
	// LifecycleHooks indicates whether lifecycle hooks are enabled.
	LifecycleHooks bool `json:"lifecycle_hooks"`
	// LabelEnable indicates whether label-based enabling is active.
	LabelEnable bool `json:"label_enable"`
	// FilterDesc is a human-readable description of the applied filter.
	FilterDesc string `json:"filter_desc"`
	// Scope is the monitoring scope.
	Scope string `json:"scope"`
	// GitEnable is the process-wide Git watcher default.
	GitEnable bool `json:"git_enable"`
}

// GetFunc returns the current configuration.
type GetFunc func(ctx context.Context) (ConfigData, error)

// Handler serves the /v1/config endpoint.
type Handler struct {
	log *zerolog.Logger

	getConfig GetFunc
	Path      string
}

// New creates a new config handler backed by the given function.
//
// Parameters:
//   - getConfig: Function that returns the current Watchtower configuration.
func New(log *zerolog.Logger, getConfig GetFunc) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:       log,
		getConfig: getConfig,
		Path:      "/v1/config",
	}
}

// Handle responds with the current Watchtower configuration as JSON.
//
//	@Summary		Get current configuration
//	@Description	Returns the active Watchtower configuration settings. Sensitive values (notification URLs, tokens) are redacted.
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Config object with timestamp and api_version"
//	@Failure		500	{string}	string					"Failed to get configuration"
//	@Failure		401	{string}	string					"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/config [get]
func (h *Handler) Handle(c fiber.Ctx) error {
	h.log.Debug().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("notify", "no").
		Msg("Received HTTP API config request")

	cfg, err := h.getConfig(c.Context())
	if err != nil {
		h.log.Error().
			Err(err).
			Str("notify", "no").
			Msg("Failed to get config for API")

		sendErr := c.Status(fiber.StatusInternalServerError).
			SendString("failed to get configuration")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	err = c.Status(fiber.StatusOK).JSON(fiber.Map{
		"config":      cfg,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"api_version": "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}
