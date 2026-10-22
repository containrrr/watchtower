package events

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/sse"
	"github.com/rs/zerolog"
)

// sseRetryInterval is the reconnect delay for SSE clients.
const sseRetryInterval = 5 * time.Second

// Handler serves the /v1/events endpoint with Server-Sent Events.
type Handler struct {
	log *zerolog.Logger

	Path           string
	Broadcaster    *Broadcaster
	AllowedOrigins []string
}

// NewHandler creates a new events handler backed by the given broadcaster.
//
// Parameters:
//   - b: The event broadcaster for distributing events to subscribers.
//   - allowedOrigins: CORS origins permitted to connect to the SSE endpoint.
//     Pass nil to allow all origins.
//
// Returns:
//   - *Handler: The initialized events handler.
func NewHandler(log *zerolog.Logger, b *Broadcaster, allowedOrigins []string) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:            log,
		Path:           "/v1/events",
		Broadcaster:    b,
		AllowedOrigins: allowedOrigins,
	}
}

// Handle streams Watchtower events to the client using Server-Sent Events.
//
// Returns:
//
//   - fiber.Handler: The registered route handler for SSE streaming.
//
//     @Summary		Real-time events stream
//     @Description	Streams Watchtower operational events (scan started/completed, update started/completed/failed) via Server-Sent Events (SSE).
//     @Description
//     @Description	**SSE is not supported by "Try it out"**.
//     @Tags			events
//     @Produce		text/event-stream
//     @Success		200	{string}	string	"Event stream (SSE)"
//     @Failure		401	{string}	string	"Missing or invalid events token"
//     @Security		EventsToken
//     @Router			/v1/events [get]
func (h *Handler) Handle() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !h.checkOrigin(c) {
			return fiber.ErrForbidden
		}

		logSubscriberConnected(h.log, c)

		subCh, done, err := h.subscribe()
		if err != nil {
			return err
		}

		return h.serveStream(c, subCh, done)
	}
}

// checkOrigin rejects the request when the Origin header is present but not
// permitted.
//
// Parameters:
//   - c: The Fiber request context.
//
// Returns:
//   - bool: True when the origin is permitted and the request should proceed.
func (h *Handler) checkOrigin(c fiber.Ctx) bool {
	origin := c.Get("Origin")
	if origin == "" {
		return true
	}

	host := c.Host()
	if isOriginAllowed(origin, host, h.AllowedOrigins) {
		return true
	}

	h.log.Warn().
		Str("origin", origin).
		Str("host", host).
		Str("ip", c.IP()).
		Str("notify", "no").
		Msg("Rejected SSE connection from disallowed origin")

	return false
}

// logSubscriberConnected logs a successful SSE subscriber connection.
//
// Parameters:
//   - c: The Fiber request context.
func logSubscriberConnected(log *zerolog.Logger, c fiber.Ctx) {
	log.Info().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Str("notify", "no").
		Msg("New SSE subscriber connected")
}

// subscribe allocates broadcaster subscription resources.
//
// Returns:
//   - <-chan Event: The event channel for receiving SSE events, or nil on failure.
//   - <-chan struct{}: The done channel closed when the subscriber is unsubscribed.
//   - error: Non-nil when the broadcaster is nil or the subscriber cap is reached.
func (h *Handler) subscribe() (<-chan Event, <-chan struct{}, error) {
	if h.Broadcaster == nil {
		return nil, nil, fiber.ErrServiceUnavailable
	}

	subCh, done := h.Broadcaster.SubscribeWithDone()
	if subCh == nil {
		return nil, nil, fiber.ErrServiceUnavailable
	}

	return subCh, done, nil
}

// serveStream runs the SSE middleware with the stream dispatch loop for a
// single subscriber.
//
// Parameters:
//   - c: The Fiber request context.
//   - subCh: The event channel for receiving SSE events.
//   - done: The done channel closed when the subscriber is unsubscribed.
//
// Returns:
//   - error: Non-nil if the SSE middleware fails to start or the stream errors.
func (h *Handler) serveStream(c fiber.Ctx, subCh <-chan Event, done <-chan struct{}) error {
	// The server closes this channel when it starts shutting down. The stream's
	// own context does not end then, and its Done channel closes only after a
	// failed write, so without it a stream would hold the shutdown open until
	// the next heartbeat.
	shutdown := c.RequestCtx().Done()
	conn := c.RequestCtx().Conn()

	return sse.New(sse.Config{
		Retry: sseRetryInterval,
		OnClose: func(_ fiber.Ctx, _ error) {
			h.Broadcaster.Unsubscribe(subCh)
		},
		Handler: func(c fiber.Ctx, stream *sse.Stream) error {
			go failWritesOnShutdown(conn, shutdown, stream.Done())

			return h.dispatchEvents(stream, subCh, done, shutdown)
		},
	})(c)
}

// dispatchEvents reads from the subscriber channel and writes each event to
// the SSE stream until the subscriber is unsubscribed, the stream ends, or the
// server shuts down.
//
// Parameters:
//   - stream: The SSE stream for writing events.
//   - subCh: The event channel for receiving SSE events.
//   - done: The done channel closed when the subscriber is unsubscribed.
//   - shutdown: The channel the server closes when it starts shutting down.
//
// Returns:
//   - error: Non-nil if a stream write fails. Nil on normal unsubscribe,
//     stream close, or server shutdown.
func (h *Handler) dispatchEvents(stream *sse.Stream, subCh <-chan Event, done, shutdown <-chan struct{}) error {
	for {
		select {
		case event, ok := <-subCh:
			if !ok {
				return nil
			}

			data, err := json.Marshal(event)
			if err != nil {
				h.log.Warn().
					Err(err).
					Str("notify", "no").
					Msg("Failed to marshal event")

				continue
			}

			err = stream.Event(sse.Event{
				Name: event.Type,
				Data: string(data),
			})
			if err != nil {
				return fmt.Errorf("failed to send SSE event: %w", err)
			}
		case <-done:
			return nil
		case <-stream.Done():
			return fmt.Errorf("stream error: %w", stream.Err())
		case <-shutdown:
			return nil
		}
	}
}

// failWritesOnShutdown makes writes to conn fail at once when the server starts
// shutting down. The server sets no write timeout, so a write to a client that
// has stopped reading would otherwise block and hold the shutdown open until
// its grace period ends.
//
// Parameters:
//   - conn: The stream's connection.
//   - shutdown: The channel the server closes when it starts shutting down.
//   - streamDone: The channel the stream closes when it ends.
func failWritesOnShutdown(conn net.Conn, shutdown, streamDone <-chan struct{}) {
	select {
	case <-shutdown:
		_ = conn.SetWriteDeadline(time.Now())
	case <-streamDone:
	}
}

// isOriginAllowed reports whether origin is allowed for the SSE endpoint.
//
// Same-origin (with/without scheme), "null", and origins in allowedOrigins (or "*")
// are permitted.
//
// Parameters:
//   - origin: The Origin header value.
//   - host: The request host (from c.Host()).
//   - allowedOrigins: CORS allowed origins from config.
//
// Returns:
//   - bool: True if the origin is allowed.
func isOriginAllowed(origin, host string, allowedOrigins []string) bool {
	if origin == "" || origin == "null" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err == nil && originURL.Host == host {
		return true
	}

	// Fallback for origins that url.Parse handles incorrectly (e.g. bare
	// host:port without a scheme).
	if origin == host ||
		origin == "http://"+host ||
		origin == "https://"+host {
		return true
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}
