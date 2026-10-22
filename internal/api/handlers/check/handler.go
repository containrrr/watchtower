package check

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/pkg/notifications"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Handler serves the /v1/check endpoint.
type Handler struct {
	log *zerolog.Logger

	// check is the function that checks container update availability.
	check CheckFunc
	// Path is the HTTP route path for the check endpoint.
	Path string
	// maxTimeout bounds the optional per-request ?timeout= query parameter.
	maxTimeout time.Duration
	// notifier batches log entries during the check to suppress immediate notifications.
	notifier types.Notifier
	// splitByContainer sends per-container notifications when true.
	splitByContainer bool
	// eventBroadcaster publishes SSE events for check runs.
	eventBroadcaster *events.Broadcaster
	// scanStartedData carries redacted policy flags reused for every scan_started event.
	scanStartedData events.ScanStartedData
}

// New creates a new check handler backed by the given check function.
//
// Parameters:
//   - check: Function that checks container update availability.
//   - maxTimeout: Maximum allowed per-request timeout, used to bound the ?timeout= query parameter.
//   - notifier: Optional notification system instance. When provided, notification batching is enabled
//     during the check to prevent log entries from triggering immediate notifications.
//   - splitByContainer: When true, notifications are split by container instead of being grouped.
//   - eventBroadcaster: Optional SSE broadcaster. When provided, scan_started, scan_completed,
//     and scan_failed events are emitted during check runs.
//   - scanStartedData: Pre-built redacted policy flags for the scan_started event.
func New(
	log *zerolog.Logger,
	check CheckFunc,
	maxTimeout time.Duration,
	notifier types.Notifier,
	splitByContainer bool,
	eventBroadcaster *events.Broadcaster,
	scanStartedData events.ScanStartedData,
) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:              log,
		check:            check,
		Path:             "/v1/check",
		maxTimeout:       maxTimeout,
		notifier:         notifier,
		splitByContainer: splitByContainer,
		eventBroadcaster: eventBroadcaster,
		scanStartedData:  scanStartedData,
	}
}

// Handle processes HTTP check requests. It extracts filter parameters, runs
// the check function, and returns JSON results. If no check function is
// configured, it returns a 500 response.
//
//	@Summary		Check for available container updates
//	@Description	Checks each watched container for available updates. Associated Git-watched containers compare the hosted Git ref (no clone or build). Other containers query the registry for the
//
// latest digest without pulling image layers.
//
//	@Tags			check
//	@Accept			json
//	@Produce		json
//	@Param			image		query		string			false	"Image names to check (comma-separated, repeatable). When combined with container, only containers matching both are checked."
//	@Param			container	query		string			false	"Container names to check (comma-separated, repeatable). When combined with image, only containers matching both are checked."
//	@Param			timeout		query		string			false	"Per-request timeout override (e.g. 30s, 2m). Bounded by the configured check API timeout."
//	@Success		200			{object}	CheckResponse	"Container update availability results"
//	@Failure		500			{string}	string			"Failed to check for updates"
//	@Failure		401			{string}	string			"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/check [post]
func (h *Handler) Handle(c fiber.Ctx) error {
	// API check path must not re-enter notification hooks (notify=no child of h.log).
	apiLog := h.log.With().Str("notify", "no").Logger()

	if h.check == nil {
		apiLog.Warn().Msg("Received HTTP API check request but no check function is configured")

		sendErr := c.Status(fiber.StatusInternalServerError).
			SendString("check function is not configured")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	apiLog.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Msg("Received HTTP API check request")

	images := extractFilterParams(c, "image")
	containers := extractFilterParams(c, "container")

	ctx := c.Context()
	if timeoutStr := c.Query("timeout"); timeoutStr != "" {
		parsed, err := time.ParseDuration(timeoutStr)
		if err == nil && parsed > 0 {
			if parsed > h.maxTimeout {
				parsed = h.maxTimeout
			}

			var cancel func()

			ctx, cancel = context.WithTimeout(ctx, parsed)
			defer cancel()
		}
	}

	// Suppress notifications during the check to prevent log entries from
	// triggering immediate notifications. The notifier is reset after the check.
	if h.notifier != nil {
		h.notifier.StartNotification(true)

		defer func() {
			if h.splitByContainer {
				notifications.FlushSplitByContainer(h.notifier)
				h.notifier.StartNotification(true)
			} else {
				h.notifier.SendNotification(nil)
			}

			h.notifier.StartNotification(false)
		}()
	}

	// Notify SSE subscribers that the check scan is starting.
	if h.eventBroadcaster != nil {
		h.eventBroadcaster.Publish(events.Event{
			Type:      "scan_started",
			Timestamp: time.Now().UTC(),
			Data:      h.scanStartedData,
		})
	}

	results, err := h.check(ctx, images, containers)
	if err != nil {
		apiLog.Error().Err(err).Msg("Failed to check for updates")

		// Notify SSE subscribers that the check scan failed.
		if h.eventBroadcaster != nil {
			h.eventBroadcaster.Publish(events.Event{
				Type:      "scan_failed",
				Timestamp: time.Now().UTC(),
				Data: events.ScanFailedData{
					Error: "failed to check for updates",
				},
			})
		}

		sendErr := c.Status(fiber.StatusInternalServerError).
			SendString("failed to check for updates")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	// Count containers whose per-container check returned an error.
	failedCount := 0

	for _, r := range results {
		if r.Error != "" {
			failedCount++
		}
	}

	// Notify SSE subscribers that the check scan completed successfully.
	if h.eventBroadcaster != nil {
		h.eventBroadcaster.Publish(events.Event{
			Type:      "scan_completed",
			Timestamp: time.Now().UTC(),
			Data: events.ScanCompletedData{
				Scanned: len(results),
				Updated: 0,
				Failed:  failedCount,
			},
		})
	}

	err = c.Status(fiber.StatusOK).JSON(CheckResponse{
		Containers: results,
		Count:      len(results),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		APIVersion: "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}
