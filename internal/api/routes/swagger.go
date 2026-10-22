package routes

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/swaggo/swag"

	swaggo "github.com/gofiber/contrib/v3/swaggo"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
)

// registerSwaggerRoute mounts Swagger UI under /swagger/* without API auth.
// Interactive Try it out against protected /v1/* routes still requires the
// API token via Swagger UI Authorize (BearerAuth / EventsToken schemes).
//
// Parameters:
//   - app: Fiber application.
//   - opts: API configuration options (Reserved for future config options).
func registerSwaggerRoute(app *fiber.App, _ config.Options) {
	swaggerUI := swaggo.New(swaggo.Config{
		Title:                "Watchtower HTTP API",
		PersistAuthorization: true,
		ValidatorUrl:         "none",
		TryItOutEnabled:      true,
	})

	app.Get("/swagger/*", swaggerDocMiddleware(), swaggerUI)
}

// swaggerDocMiddleware rewrites host and schemes on doc.json responses so
// Swagger UI Try It Out targets the request host instead of the static
// generated localhost:8080 value.
func swaggerDocMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Params("*") is the wildcard segment for /swagger/* routes.
		if c.Params("*") != "doc.json" {
			return c.Next()
		}

		doc, err := swag.ReadDoc()
		if err != nil {
			return fmt.Errorf("read swagger doc: %w", err)
		}

		rewritten, err := rewriteSwaggerDocHost(doc, c)
		if err != nil {
			return err
		}

		return c.Type("json").SendString(rewritten)
	}
}

// rewriteSwaggerDocHost sets host and schemes on an OpenAPI 2.0 JSON document
// from the incoming request.
//
// Scheme comes from Fiber's Protocol, which honors X-Forwarded-Proto only
// when TrustedProxies is configured on the app.
// When behind a TLS-terminating reverse proxy and TrustedProxies is unset,
// c.Protocol() reports "http" and the rewritten spec will use http schemes.
//
// Parameters:
//   - doc: Raw OpenAPI JSON string.
//   - c: Request context.
//
// Returns:
//   - string: Modified OpenAPI JSON.
//   - error: Non-nil if JSON is invalid.
func rewriteSwaggerDocHost(doc string, c fiber.Ctx) (string, error) {
	var spec map[string]any

	err := json.Unmarshal([]byte(doc), &spec)
	if err != nil {
		return "", fmt.Errorf("parse swagger doc: %w", err)
	}

	host := c.Host()
	if host != "" {
		spec["host"] = host
	}

	scheme := "http"
	if c.Protocol() == "https" {
		scheme = "https"
	}

	spec["schemes"] = []string{scheme}

	out, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal swagger doc: %w", err)
	}

	return string(out), nil
}
