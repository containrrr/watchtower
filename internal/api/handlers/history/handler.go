package history

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
)

// Handler serves the /v1/history endpoint.
type Handler struct {
	log *zerolog.Logger

	Path    string
	getHist func(*time.Time, *time.Time, int) []metrics.HistoryEntry
}

// New creates a new history handler backed by the given function.
//
// Parameters:
//   - getHist: Function that returns scan history entries, optionally filtered
//     by time range and limited to N entries.
func New(log *zerolog.Logger, getHist func(*time.Time, *time.Time, int) []metrics.HistoryEntry) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:     log,
		Path:    "/v1/history",
		getHist: getHist,
	}
}

// Handle responds with the scan history as JSON, optionally filtered by
// time range and limited to the most recent N entries.
//
//	@Summary		Scan history
//	@Description	Returns historical scan results from the in-memory ring buffer (up to 500 entries). Optionally filter by time range and limit the number of results.
//	@Tags			history
//	@Accept			json
//	@Produce		json
//	@Param			since	query		string					false	"Include entries at or after this RFC3339 timestamp"
//	@Param			until	query		string					false	"Include entries at or before this RFC3339 timestamp"
//	@Param			limit	query		int						false	"Maximum number of entries to return (default: all)"
//	@Success		200		{object}	map[string]interface{}	"History entries with count and timestamp"
//	@Failure		400		{string}	string					"Invalid query parameter"
//	@Failure		401		{string}	string					"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/history [get]
func (h *Handler) Handle(c fiber.Ctx) error {
	h.log.Debug().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("notify", "no").
		Msg("Received HTTP API history request")

	sinceRaw := c.Query("since")

	if sinceRaw == "" && c.Request().URI().QueryArgs().Has("since") {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid 'since' parameter: empty value")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	since, sinceErr := parseTimeParam(sinceRaw)
	if sinceErr != nil && !errors.Is(sinceErr, errNoTimeParameter) {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid 'since' parameter: " + sinceErr.Error())
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	untilRaw := c.Query("until")
	if untilRaw == "" && c.Request().URI().QueryArgs().Has("until") {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid 'until' parameter: empty value")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	until, untilErr := parseTimeParam(untilRaw)
	if untilErr != nil && !errors.Is(untilErr, errNoTimeParameter) {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid 'until' parameter: " + untilErr.Error())
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	if since != nil && until != nil && since.After(*until) {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid time range: 'since' must not be after 'until'")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		sendErr := c.Status(fiber.StatusBadRequest).SendString("invalid 'limit' parameter: " + err.Error())
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	entries := h.getHist(since, until, limit)

	err = c.Status(fiber.StatusOK).JSON(fiber.Map{
		"entries":     entries,
		"count":       len(entries),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"api_version": "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}
