package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/rs/zerolog"
)

// newAPIAuthMiddleware returns a Fiber middleware that validates the HTTP API
// token using constant-time SHA-256 comparison.
//
// Accepted credentials (first match wins):
//   - Authorization: Bearer <token>
//   - Authorization: <token> (raw value. Swagger UI apiKey style)
//   - Cookie access_token=<token>
//
// Auth failure logs use notify=no so they never fan out through notification hooks.
func newAPIAuthMiddleware(log *zerolog.Logger, token string) fiber.Handler {
	expectedHash := sha256.Sum256([]byte(token))
	// Child logger for high-volume auth warnings. The composition root may already
	// pass a notify=no logger, but setting it here keeps auth safe either way.
	authLog := log.With().Str("notify", "no").Logger()

	return func(c fiber.Ctx) error {
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).SendString("API token not configured")
		}

		if !tokenMatches(&authLog, c, expectedHash) {
			return c.Status(fiber.StatusUnauthorized).SendString(keyauth.ErrMissingOrMalformedAPIKey.Error())
		}

		return c.Next()
	}
}

// tokenMatches reports whether the request carries a valid API token.
func tokenMatches(log *zerolog.Logger, c fiber.Ctx, expectedHash [sha256.Size]byte) bool {
	provided, ok := extractAPIToken(c)
	if !ok {
		log.Warn().Str("ip", c.IP()).Msg("Missing or malformed API key")

		return false
	}

	if !tokenHashMatches(expectedHash, provided) {
		log.Warn().Str("ip", c.IP()).Msg("Invalid token attempt")

		return false
	}

	return true
}

// extractAPIToken returns the API token from the request.
//
// Order:
//  1. Authorization Bearer scheme (RFC-style)
//  2. Raw Authorization header value (optional "Bearer " prefix stripped), for
//     Swagger UI apiKey security which places the typed value into Authorization
//  3. access_token cookie
func extractAPIToken(c fiber.Ctx) (string, bool) {
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

	token, err = extractors.FromCookie("access_token").Extract(c)
	if err == nil && token != "" {
		return token, true
	}

	return "", false
}

// tokenHashMatches compares a provided secret to the precomputed SHA-256 of the
// configured token using constant-time comparison.
func tokenHashMatches(expectedHash [sha256.Size]byte, provided string) bool {
	providedHash := sha256.Sum256([]byte(provided))

	return subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1
}
