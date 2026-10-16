package update

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
)

// Handler triggers container update scans via HTTP.
type Handler struct {
	log *zerolog.Logger

	fn         func(ctx context.Context, images, containers []string) *metrics.Metric
	Path       string
	lock       *scheduling.UpdateLock
	ctx        context.Context //nolint:containedctx // application-lifetime context owned by Handler
	maxTimeout time.Duration
}

// lockResult holds the outcome of an acquireLock attempt.
type lockResult struct {
	Acquired   bool
	RequestErr bool
}

const (
	// retryAfterSeconds is the value for the Retry-After header in 429 responses.
	retryAfterSeconds = "30"
)

// New creates a new Handler.
//
// Parameters:
//   - updateFn: Function that executes container updates, accepting a context,
//     image names, container name patterns, and returning metrics.
//   - updateLock: Update lock shared with every other caller that starts
//     updates, such as the scheduler. Must be non-nil.
//   - ctx: Optional application-lifetime context for background work.
//     If nil, context.Background is used.
func New(
	log *zerolog.Logger,
	updateFn func(ctx context.Context, images, containers []string) *metrics.Metric,
	updateLock *scheduling.UpdateLock,
	ctx ...context.Context,
) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return NewWithTimeout(log, updateFn, updateLock, 0, ctx...)
}

// NewWithTimeout creates a new Handler with a maximum per-request timeout.
//
// Parameters:
//   - updateFn: Function that executes container updates, accepting a context,
//     image names, container name patterns, and returning metrics.
//   - updateLock: Update lock shared with every other caller that starts
//     updates, such as the scheduler. Must be non-nil.
//   - maxTimeout: Maximum allowed per-request timeout, used to bound the
//     ?timeout= query parameter for both sync and async updates. If zero,
//     no per-request timeout override is applied.
//   - ctx: Optional application-lifetime context for background work.
//     If nil, context.Background is used.
func NewWithTimeout(
	log *zerolog.Logger,
	updateFn func(ctx context.Context, images, containers []string) *metrics.Metric,
	updateLock *scheduling.UpdateLock,
	maxTimeout time.Duration,
	ctx ...context.Context,
) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	var bgCtx context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		bgCtx = ctx[0]
	} else {
		bgCtx = context.Background()
	}

	return &Handler{
		log:        log,
		fn:         updateFn,
		Path:       "/v1/update",
		lock:       updateLock,
		ctx:        bgCtx,
		maxTimeout: maxTimeout,
	}
}

// Handle processes an HTTP update request, extracting image and container
// targets from query parameters and dispatching to async or sync execution.
//
//	@Summary		Trigger container update scan
//	@Description	Scans watched containers for image updates and applies them. Supports both full scans and targeted updates filtered by image name or container name. Container patterns support Go
//
// regex syntax.
//
//	@Tags			update
//	@Accept			json
//	@Produce		json
//	@Param			image		query		string					false	"Comma-separated image names to update (repeatable)"
//	@Param			container	query		string					false	"Container name patterns to update (repeatable, supports Go regex)"
//	@Param			async		query		string					false	"When 'true', runs update asynchronously and returns 202 Accepted"
//	@Success		200			{object}	map[string]interface{}	"Synchronous update results with summary and timing"
//	@Success		202			{string}	string					"Asynchronous update accepted"
//	@Failure		429			{string}	string					"Another update is already running"
//	@Header			429			{string}	Retry-After				"Seconds to wait before retrying"
//	@Failure		503			{string}	string					"Request cancelled while waiting for lock"
//	@Failure		401			{string}	string					"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/update [post]
func (h *Handler) Handle(c fiber.Ctx) error {
	h.log.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("notify", "no").
		Msg("Received HTTP API update request")

	images := h.extractImages(c)
	containers := h.extractContainers(c)

	updateCtx := h.applyTimeout(c)

	result := h.acquireLock(c, images, containers)
	if result.RequestErr {
		return fiber.ErrServiceUnavailable
	}

	if !result.Acquired {
		return nil
	}

	if c.Query("async") == "true" {
		return h.handleAsync(c, images, containers, updateCtx)
	}

	return h.handleSync(c, images, containers, updateCtx)
}

// extractImages parses the "image" query parameters into a slice of image
// strings. It supports comma-separated values within a single query parameter
// and multiple "image" parameters (e.g., ?image=a&image=b or ?image=a,b).
// Empty values are filtered out.
func (h *Handler) extractImages(c fiber.Ctx) []string {
	var images []string

	queryArgs := c.Request().URI().QueryArgs()
	values := queryArgs.PeekMulti("image")

	for _, v := range values {
		parts := strings.SplitSeq(string(v), ",")
		for p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				images = append(images, trimmed)
			}
		}
	}

	if len(images) > 0 {
		h.log.Debug().
			Strs("images", images).
			Str("notify", "no").
			Msg("Extracted images from query parameters")
	} else {
		h.log.Debug().
			Str("notify", "no").
			Msg("No image query parameters provided")
	}

	return images
}

// extractContainers parses the "container" query parameters into a slice of
// container name patterns. Supports comma-separated values and repeated params.
// Empty values are filtered out.
func (h *Handler) extractContainers(c fiber.Ctx) []string {
	var containers []string

	queryArgs := c.Request().URI().QueryArgs()
	values := queryArgs.PeekMulti("container")

	for _, v := range values {
		parts := strings.SplitSeq(string(v), ",")
		for p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				containers = append(containers, trimmed)
			}
		}
	}

	if len(containers) > 0 {
		h.log.Debug().
			Strs("containers", containers).
			Str("notify", "no").
			Msg("Extracted container patterns from query parameters")
	} else {
		h.log.Debug().
			Str("notify", "no").
			Msg("No container query parameters provided")
	}

	return containers
}

// acquireLock attempts to acquire the update lock.
//
// For targeted updates (len(images) > 0 or len(containers) > 0), it blocks
// until the lock is available or the request is cancelled. For full updates,
// it attempts a non-blocking acquire and returns a 429 response if the lock
// is held.
func (h *Handler) acquireLock(c fiber.Ctx, images, containers []string) lockResult {
	h.log.Debug().
		Str("notify", "no").
		Msg("Handler: trying to acquire lock")

	if len(images) > 0 || len(containers) > 0 {
		err := h.lock.Lock(c.Context())
		if err != nil {
			h.log.Debug().
				Str("notify", "no").
				Msg("Handler: request cancelled while waiting for lock")

			return lockResult{RequestErr: true}
		}

		h.log.Debug().
			Str("notify", "no").
			Msg("Handler: acquired lock for targeted update")

		return lockResult{Acquired: true}
	}

	if !h.lock.TryLock() {
		h.log.Debug().
			Str("notify", "no").
			Msg("Skipped update, another update already in progress")
		h.send429Response(c)

		return lockResult{}
	}

	h.log.Debug().
		Str("notify", "no").
		Msg("Handler: acquired lock for full update")

	return lockResult{Acquired: true}
}

// send429Response writes a JSON error response indicating an update is
// already running.
func (h *Handler) send429Response(c fiber.Ctx) {
	c.Set("Retry-After", retryAfterSeconds)

	err := c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error":       "another update is already running",
		"api_version": "v1",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		h.log.Error().
			Err(err).
			Str("notify", "no").
			Msg("Failed to send 429 response")
	}
}

// applyTimeout returns a context derived from the request context, optionally
// wrapped with a per-request timeout from the ?timeout= query parameter.
// The timeout is clamped to h.maxTimeout.
func (h *Handler) applyTimeout(c fiber.Ctx) context.Context {
	ctx := c.Context()
	if timeoutStr := c.Query("timeout"); timeoutStr != "" {
		parsed, err := time.ParseDuration(timeoutStr)
		if err == nil && parsed > 0 {
			if parsed > h.maxTimeout {
				parsed = h.maxTimeout
			}

			var cancel func()

			ctx, cancel = context.WithTimeout(ctx, parsed) //nolint:gosec
			// cancel is a no-op here.
			// Fiber will cancel the parent context when the request ends
			_ = cancel
		}
	}

	return ctx
}

// handleAsync processes an asynchronous update request by spawning a
// goroutine and returning 202 Accepted.
//
// updateCtx is used only for deadline projection inside executeUpdateAsync.
// The goroutine does not run under the request context, which is canceled
// when this handler returns.
func (h *Handler) handleAsync(c fiber.Ctx, images, containers []string, updateCtx context.Context) error {
	h.log.Info().
		Str("notify", "no").
		Msg("Handling async update request - spawning async update")

	go h.executeUpdateAsync(updateCtx, images, containers)

	err := c.SendStatus(fiber.StatusAccepted)
	if err != nil {
		return fmt.Errorf("failed to send 202 response: %w", err)
	}

	return nil
}

// handleSync processes a synchronous update request, returning the update
// results as JSON.
func (h *Handler) handleSync(c fiber.Ctx, images, containers []string, updateCtx context.Context) error {
	defer h.releaseLock()

	metric, duration := h.executeUpdate(updateCtx, images, containers)
	if metric == nil {
		return fiber.ErrInternalServerError
	}

	summary := fiber.Map{
		"scanned":   metric.Scanned,
		"updated":   metric.Updated,
		"failed":    metric.Failed,
		"restarted": metric.Restarted,
		"skipped":   metric.Skipped,
	}

	// Report a failed scan, and leave the field out otherwise.
	if metric.ScanFailed {
		summary["scan_failed"] = true
	}

	err := c.Status(fiber.StatusOK).JSON(fiber.Map{
		"summary": summary,
		"timing": fiber.Map{
			"duration_ms": duration.Milliseconds(),
			"duration":    duration.String(),
		},
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"api_version": "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}

// executeUpdateAsync runs the update function in a goroutine, ensuring the
// lock is released when done.
//
// The execution context is always derived from the handler's application-lifetime
// context so the update survives request completion and Fiber timeout-middleware
// cancellation. Any deadline on updateCtx (route timeout middleware and optional
// ?timeout=) is projected onto that context without inheriting request cancellation.
func (h *Handler) executeUpdateAsync(updateCtx context.Context, images, containers []string) {
	defer func() {
		if rec := recover(); rec != nil {
			h.log.Error().
				Interface("panic", rec).
				Str("notify", "no").
				Msg("Update goroutine panicked")
		}

		h.releaseLock()
	}()

	ctx, cancel := h.contextForAsync(updateCtx)
	defer cancel()

	startTime := time.Now()

	h.fn(ctx, images, containers)

	duration := time.Since(startTime)
	h.log.Debug().
		Dur("duration", duration).
		Str("notify", "no").
		Msg("Handler (async): update function completed")
}

// contextForAsync returns a context rooted at h.ctx for background update work.
//
// If updateCtx has a deadline, it is applied so middleware and per-request
// timeouts still bound the async update. Request cancellation on updateCtx is
// intentionally not propagated.
//
// Parameters:
//   - updateCtx: Request-scoped context used only for deadline projection.
//
// Returns:
//   - context.Context: Context for the async update function.
//   - context.CancelFunc: Cancel function. Always non-nil and safe to defer.
func (h *Handler) contextForAsync(updateCtx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := updateCtx.Deadline(); ok {
		return context.WithDeadline(h.ctx, deadline)
	}

	return h.ctx, func() {}
}

// executeUpdate runs the update function and returns the metric along with
// duration.
func (h *Handler) executeUpdate(ctx context.Context, images, containers []string) (*metrics.Metric, time.Duration) {
	h.log.Debug().
		Str("notify", "no").
		Msg("Handler: executing update function")

	startTime := time.Now()
	metric := h.fn(ctx, images, containers)
	duration := time.Since(startTime)

	h.log.Debug().
		Str("notify", "no").
		Msg("Handler: update function completed")

	return metric, duration
}

// releaseLock releases the update lock, allowing another update to proceed.
func (h *Handler) releaseLock() {
	h.log.Debug().
		Str("notify", "no").
		Msg("Handler: releasing lock")

	h.lock.Unlock()
}
