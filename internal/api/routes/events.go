package routes

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/keyauth"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
)

func registerEventsRoute(app *fiber.App, opts config.Options) {
	handler := events.NewHandler(opts.Logger, opts.EventBroadcaster, opts.CORSAllowedOrigins)

	eventsToken := opts.EventsToken
	expectedHash := sha256.Sum256([]byte(eventsToken))

	eventsAuth := func(c fiber.Ctx) error {
		if eventsToken == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("events API token not configured")
		}

		provided, ok := extractEventsToken(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).SendString(keyauth.ErrMissingOrMalformedAPIKey.Error())
		}

		providedHash := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
			return c.Status(fiber.StatusUnauthorized).SendString(keyauth.ErrMissingOrMalformedAPIKey.Error())
		}

		return c.Next()
	}

	app.Get(handler.Path, eventsAuth, handler.Handle())
}

// extractEventsToken returns the events token from the request.
//
// Order:
//  1. Authorization Bearer scheme
//  2. Raw Authorization header (Swagger UI apiKey style and optional Bearer prefix)
//  3. access_token query parameter (browser EventSource)
func extractEventsToken(c fiber.Ctx) (string, bool) {
	token, err := extractors.FromAuthHeader("Bearer").Extract(c)
	if err == nil && token != "" {
		return token, true
	}

	raw := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if raw != "" {
		const bearerPrefix = "bearer "

		if len(raw) > len(bearerPrefix) && strings.EqualFold(raw[:len(bearerPrefix)], bearerPrefix) {
			raw = strings.TrimSpace(raw[len(bearerPrefix):])
		}

		if raw != "" {
			return raw, true
		}
	}

	token, err = extractors.FromQuery("access_token").Extract(c)
	if err == nil && token != "" {
		return token, true
	}

	return "", false
}
