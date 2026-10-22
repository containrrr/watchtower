package health

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"

	"github.com/nicholas-fedor/watchtower/pkg/container"
)

const readinessProbeTimeout = 5 * time.Second

// LivenessHandler serves the /livez endpoint.
type LivenessHandler struct {
	Path string
}

// NewLivenessHandler creates a new liveness handler.
func NewLivenessHandler() *LivenessHandler {
	return &LivenessHandler{
		Path: healthcheck.LivenessEndpoint,
	}
}

// Handle responds to liveness probe requests.
//
//	@Summary		Liveness probe
//	@Description	Returns 200 OK if the Watchtower HTTP server is running. Used by orchestrators (e.g., Kubernetes) to determine if the process is alive.
//	@Tags			health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Router			/livez [get]
func (h *LivenessHandler) Handle(c fiber.Ctx) error {
	return healthcheck.New()(c)
}

// ReadinessHandler serves the /readyz endpoint.
type ReadinessHandler struct {
	Path  string
	Probe func(ctx context.Context) bool
}

// NewReadinessHandler creates a new readiness handler that checks Docker client
// connectivity via Ping.
//
// Parameters:
//   - client: Docker client for the readiness probe. May be nil, in which case
//     the readiness probe will report unhealthy.
func NewReadinessHandler(client container.Client) *ReadinessHandler {
	return &ReadinessHandler{
		Path: healthcheck.ReadinessEndpoint,
		Probe: func(ctx context.Context) bool {
			if client == nil {
				return false
			}

			probeCtx, cancel := context.WithTimeout(ctx, readinessProbeTimeout)
			defer cancel()

			return client.Ping(probeCtx) == nil
		},
	}
}

// Handle responds to readiness probe requests.
//
//	@Summary		Readiness probe
//	@Description	Returns 200 OK if the Watchtower HTTP server is running and the Docker client is connected and responsive. Used by orchestrators to determine if the service is ready to accept
//
// traffic.
//
//	@Tags			health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Failure		503	{string}	string	"Docker client not connected"
//	@Router			/readyz [get]
func (h *ReadinessHandler) Handle(c fiber.Ctx) error {
	return healthcheck.New(healthcheck.Config{
		Probe: func(c fiber.Ctx) bool {
			return h.Probe(c.Context())
		},
	})(c)
}

// StartupHandler serves the /startupz endpoint.
type StartupHandler struct {
	Path string
}

// NewStartupHandler creates a new startup handler.
func NewStartupHandler() *StartupHandler {
	return &StartupHandler{
		Path: healthcheck.StartupEndpoint,
	}
}

// Handle responds to startup probe requests.
//
//	@Summary		Startup probe
//	@Description	Returns 200 OK if the Watchtower HTTP server has started. Used by orchestrators (e.g., Kubernetes) to determine if the process has finished startup.
//	@Tags			health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Router			/startupz [get]
func (h *StartupHandler) Handle(c fiber.Ctx) error {
	return healthcheck.New()(c)
}
