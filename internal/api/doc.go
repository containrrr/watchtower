// Package api provides Watchtower's HTTP API server, built on Fiber v3.
//
// It handles token-authenticated and session-authenticated requests for
// triggering container updates, checking update availability, serving Prometheus
// metrics, listing watched container image identities, and streaming real-time
// events via SSE.
//
// Endpoints:
//
//	GET  /livez                  Liveness probe
//	GET  /readyz                 Readiness probe — checks Docker client connectivity
//	GET  /startupz               Startup probe
//	POST /v1/update              Trigger container update scan (requires auth)
//	POST /v1/check               Check for available updates (requires auth)
//	GET  /v1/metrics             Prometheus metrics (requires auth)
//	GET  /v1/status              Last scan results (requires auth)
//	GET  /v1/containers          List watched container statuses (requires auth)
//	GET  /v1/containers/details  Detailed container info (requires auth)
//	GET  /v1/history             Historical scan results (requires auth)
//	GET  /v1/images              Tracked images with digests (requires auth)
//	GET  /v1/config              Active configuration settings (requires auth)
//	GET  /v1/events              Real-time events via SSE (requires events token)
//	GET  /swagger/*              Swagger UI (requires http-api-swagger)
//
// Health probes (/livez, /readyz, /startupz) are enabled via
// EnableHealthAPI and require no authentication.
// All /v1/* endpoints except /v1/events require Bearer token authentication.
// /v1/events requires a separate events token (via http-api-events-token).
//
// Key components:
//   - newApp: Creates a Fiber application with the configured middleware stack (fiber.go).
//   - newAPIAuthMiddleware: Bearer token authentication (auth.go).
//   - routes.ValidateAndRegister: Validates options and registers enabled endpoints.
//   - SetupAndStartAPI: Orchestrates endpoint registration and server lifecycle (lifecycle.go).
//   - config.ValidateUpdateOptions: Validates required update API dependencies (config/).
//   - config.TimeoutMiddleware: Per-request timeout enforcement (config/).
//
// File organization:
//   - config/: Shared configuration types, validation, and sentinel errors.
//   - lifecycle.go: Server startup, shutdown, and address formatting.
//   - fiber.go: Fiber app factory, configuration types, and middleware stack.
//   - auth.go: Token authentication middleware.
//   - routes/: Per-endpoint registration including health checks.
//
// Security features:
//   - Token hashing: Tokens are hashed with SHA-256 at initialization.
//   - Constant-time comparison: Uses crypto/subtle to prevent timing attacks.
//   - Per-IP rate limiting: Sliding window via Fiber's limiter middleware.
//   - Panic recovery: Catches handler panics and returns 500.
//   - Security headers: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection.
//   - Request ID: Unique ID per request for log correlation.
//   - Response compression: gzip, deflate, brotli, zstd.
//   - CORS: Configured for cross-origin requests.
//
// Middleware stack (outermost to innermost):
//  1. recover     — panic recovery
//  2. helmet      — security headers
//  3. cors        — cross-origin headers (only when AllowedOrigins is configured)
//  4. requestid   — request ID propagation
//  5. zerolog     — structured request logging via gofiber/contrib/v3/zerolog
//  6. compress    — response compression
//  7. limiter     — per-IP rate limiting (sliding window)
//  8. auth        — Bearer token authentication (per-route)
//
// API server timeout behavior:
//
//   - ReadTimeout is set to 10s. It bounds reading the full request including
//     body. Required for clean Fiber shutdown.
//   - IdleTimeout is set to 30s. It bounds the wait for the next request on
//     keep-alive connections. SSE sends stream comments every 5 seconds, so
//     idle timeout does not fire on quiet event streams.
//   - WriteTimeout is left at zero. A global write deadline conflicts with
//     long-lived routes. SSE idles between events, and the update handler
//     can run up to 10 minutes under its own per-route timeout middleware.
//     Route-specific timeouts bound each handler instead.
package api
