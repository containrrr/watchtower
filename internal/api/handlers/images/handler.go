package images

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// Handler serves the /v1/images endpoint.
type Handler struct {
	log *zerolog.Logger

	list ListFunc
	Path string
}

// New creates a new images handler backed by the given list function.
//
// Parameters:
//   - list: Function that returns the current status of all tracked images.
func New(log *zerolog.Logger, list ListFunc) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:  log,
		list: list,
		Path: "/v1/images",
	}
}

// Handle responds with the JSON status of every tracked image.
//
//	@Summary		List tracked images
//	@Description	Returns the current image identity and digest for every image tracked by Watchtower. Optionally filter by image name or image ID.
//	@Tags			images
//	@Accept			json
//	@Produce		json
//	@Param			name	query		string					false	"Filter by image name (exact match)"
//	@Param			id		query		string					false	"Filter by image ID (sha256:...)"
//	@Success		200		{object}	map[string]interface{}	"Image statuses with count and timestamp"
//	@Failure		500		{string}	string					"Failed to list images"
//	@Failure		401		{string}	string					"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/images [get]
func (h *Handler) Handle(c fiber.Ctx) error {
	h.log.Debug().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("notify", "no").
		Msg("Received HTTP API images request")

	statuses, err := h.list(c.Context())
	if err != nil {
		h.log.Error().
			Err(err).
			Str("notify", "no").
			Msg("Failed to list images for API")

		sendErr := c.Status(fiber.StatusInternalServerError).
			SendString("failed to list images")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	nameFilter := c.Query("name")
	idFilter := c.Query("id")

	if nameFilter != "" || idFilter != "" {
		statuses = filterImages(statuses, nameFilter, idFilter)
	}

	err = c.Status(fiber.StatusOK).JSON(fiber.Map{
		"images":      statuses,
		"count":       len(statuses),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"api_version": "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}
