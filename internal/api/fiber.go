package api

import (
	"slices"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/rs/zerolog"

	fiberZerolog "github.com/gofiber/contrib/v3/zerolog"
	fiberRecover "github.com/gofiber/fiber/v3/middleware/recover"
)

const (
	// bodyLimit defines the maximum request body size (1 MiB).
	bodyLimit = 1 << 20

	// readTimeout defines the maximum duration for reading the entire request,
	// including the body.
	readTimeout = 10 * time.Second

	// idleTimeout defines the maximum amount of time to wait for the next
	// request when keep-alives are enabled.
	idleTimeout = 30 * time.Second

	// defaultRateLimitPerMinute is the fallback rate limit when a
	// non-positive value is provided.
	defaultRateLimitPerMinute = 60

	// corsMaxAge is the maximum duration (in seconds) that browsers may cache
	// CORS preflight results.
	corsMaxAge = 3600
)

// ShutdownGracePeriod defines the maximum duration allowed for the server to
// shut down gracefully.
const ShutdownGracePeriod = 5 * time.Second

// ProxyConfig holds configuration for reverse proxy support.
type ProxyConfig struct {
	// TrustedProxies is a list of trusted proxy IPs/CIDRs.
	TrustedProxies []string
	// ProxyHeader is the header for the real client IP (e.g. X-Forwarded-For).
	ProxyHeader string
}

// CORSConfig holds configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins allowed to make cross-origin requests.
	// Use ["*"] to allow all origins.
	AllowedOrigins []string
	// AllowedMethods is a list of HTTP methods allowed for cross-origin requests.
	AllowedMethods []string
	// AllowedHeaders is a list of headers allowed in cross-origin requests.
	AllowedHeaders []string
}

// New creates a new Fiber-based API application with the configured middleware
// stack and lifecycle hooks.
//
// Parameters:
//   - log: Logger for Fiber middleware and server lifecycle messages.
//   - rateLimitPerMinute: Maximum requests per minute per IP. Values <= 0 fall
//     back to defaultRateLimitPerMinute (60).
//   - proxyCfg: Reverse proxy configuration.
//   - corsCfg: CORS middleware configuration.
//   - noStartupMessage: When true, suppresses the HTTP API startup log entries.
//
// Returns:
//   - *fiber.App: Configured Fiber application.
func New(
	log *zerolog.Logger,
	rateLimitPerMinute int,
	proxyCfg ProxyConfig,
	corsCfg CORSConfig,
	noStartupMessage bool,
) *fiber.App {
	rateLimit := rateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = defaultRateLimitPerMinute
	}

	fiberCfg := fiber.Config{
		BodyLimit:     bodyLimit,
		ReadTimeout:   readTimeout,
		IdleTimeout:   idleTimeout,
		StrictRouting: true,
		CaseSensitive: true,
	}

	if len(proxyCfg.TrustedProxies) > 0 {
		fiberCfg.TrustProxy = true

		fiberCfg.TrustProxyConfig = fiber.TrustProxyConfig{
			Proxies: proxyCfg.TrustedProxies,
		}
		if proxyCfg.ProxyHeader != "" {
			fiberCfg.ProxyHeader = proxyCfg.ProxyHeader
		} else {
			fiberCfg.ProxyHeader = fiber.HeaderXForwardedFor
		}
	}

	apiServer := fiber.New(fiberCfg)

	corsOrigins := corsCfg.AllowedOrigins
	enableCORS := len(corsOrigins) > 0

	corsMethods := corsCfg.AllowedMethods
	if len(corsMethods) == 0 {
		corsMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}

	corsHeaders := corsCfg.AllowedHeaders
	if len(corsHeaders) == 0 {
		corsHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}

	middlewares := []any{
		fiberRecover.New(),
		helmet.New(),
	}

	if enableCORS {
		allowCredentials := true

		allowedOrigins := corsOrigins
		if slices.Contains(allowedOrigins, "*") {
			allowedOrigins = nil
			allowCredentials = false
		}

		middlewares = append(middlewares, cors.New(cors.Config{
			AllowOrigins:     allowedOrigins,
			AllowMethods:     corsMethods,
			AllowHeaders:     corsHeaders,
			AllowCredentials: allowCredentials,
			MaxAge:           corsMaxAge,
		}))
	}

	// High-volume request and rate-limit logs must not trigger notification hooks.
	// Lifecycle hooks (listen/shutdown) use log as given from the composition root.
	requestLog := log.With().Str("notify", "no").Logger()

	middlewares = append(middlewares,
		requestid.New(),
		fiberZerolog.New(fiberZerolog.Config{
			Logger: &requestLog,
			// Structured access fields: status/method/path (prior stream shape) plus
			// ip/latency/error and request ID for correlation with requestid middleware.
			// Messages/Levels omit so contrib defaults apply (5xx Error, 4xx Warn, else Info).
			Fields: []string{
				fiberZerolog.FieldStatus,
				fiberZerolog.FieldMethod,
				fiberZerolog.FieldPath,
				fiberZerolog.FieldIP,
				fiberZerolog.FieldLatency,
				fiberZerolog.FieldError,
				fiberZerolog.FieldRequestID,
			},
		}),
		compress.New(compress.Config{
			Level: compress.LevelBestSpeed,
		}),
		limiter.New(limiter.Config{
			Max:               rateLimit,
			Expiration:        time.Minute,
			LimiterMiddleware: limiter.SlidingWindow{},
			KeyGenerator:      func(c fiber.Ctx) string { return c.IP() },
			LimitReached: func(c fiber.Ctx) error {
				requestLog.Warn().Str("ip", c.IP()).Msg("Rate limit exceeded")

				return c.SendStatus(fiber.StatusTooManyRequests)
			},
		}),
	)

	apiServer.Use(middlewares...)

	apiServer.Hooks().OnListen(func(data fiber.ListenData) error {
		if !noStartupMessage {
			log.Debug().
				Str("host", data.Host).
				Str("port", data.Port).
				Bool("tls", data.TLS).
				Msg("Starting HTTP API server")

			log.Info().
				Str("host", data.Host).
				Str("port", data.Port).
				Bool("tls", data.TLS).
				Msg("HTTP API server is enabled")
		}

		return nil
	})

	apiServer.Hooks().OnPreShutdown(func() error {
		log.Info().Msg("Initiating HTTP API shutdown")

		return nil
	})

	apiServer.Hooks().OnPostShutdown(func(err error) error {
		if err != nil {
			log.Warn().Err(err).Msg("HTTP server shut down with error")
		} else {
			log.Info().Msg("HTTP server shut down successfully")
		}

		return nil
	})

	return apiServer
}
