// @title						Watchtower HTTP API
// @version					1.0
// @description				Watchtower HTTP API for container update management, metrics, and health probes.
// @contact.name				Nicholas Fedor
// @contact.url				https://github.com/nicholas-fedor/watchtower
// @license.name				Apache-2.0
// @license.url				https://www.apache.org/licenses/LICENSE-2.0
// @host						localhost:8080
// @BasePath					/
// @schemes					http https
// @accept						json
// @produce					json
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Paste the HTTP API token only (WATCHTOWER_HTTP_API_TOKEN). Swagger UI sends it as the Authorization header value. curl clients should use "Authorization: Bearer <token>".
// @securityDefinitions.apikey	EventsToken
// @in							header
// @name						Authorization
// @description				Paste the events API token only (WATCHTOWER_HTTP_API_EVENTS_TOKEN). Swagger UI sends it as the Authorization header value.
// @tag.name					health
// @tag.description			Standardized liveness, readiness, and startup probes
// @tag.name					update
// @tag.description			Trigger and manage container image updates
// @tag.name					metrics
// @tag.description			Prometheus metrics and scan status
// @tag.name					containers
// @tag.description			Watched container image identity and update availability
// @tag.name					check
// @tag.description			Check for available container updates without applying them
// @tag.name					history
// @tag.description			Historical scan results from the in-memory ring buffer
// @tag.name					images
// @tag.description			Tracked images with digests and container counts
// @tag.name					config
// @tag.description			Active Watchtower configuration settings
// @tag.name					events
// @tag.description			Real-time operational events via Server-Sent Events
package main

import (
	"os"

	"github.com/nicholas-fedor/watchtower/cmd"
	"github.com/nicholas-fedor/watchtower/internal/logging"
)

// main serves as the entry point for the Watchtower application.
//
// It constructs the process logger and delegates execution to the cmd package,
// which handles CLI setup, flag parsing, and core logic for container updates
// and notifications.
func main() {
	log := logging.New(os.Stderr, logging.InfoLevel)
	cmd.Execute(log)
}
