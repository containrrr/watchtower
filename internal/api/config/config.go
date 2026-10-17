// Package config defines the shared configuration types, validation functions,
// and sentinel errors used across the API packages. It exists to break the
// import cycle between the top-level api package and the routes subpackage.
package config

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	mt "github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var (
	// errMissingRunUpdatesWithNotifications indicates RunUpdatesWithNotifications was not provided.
	errMissingRunUpdatesWithNotifications = errors.New("RunUpdatesWithNotifications must be provided when EnableUpdateAPI is set")
	// ErrMissingFilterByImage indicates FilterByImage was not provided when an
	// endpoint that builds image-scoped filters is enabled.
	ErrMissingFilterByImage = errors.New("FilterByImage must be provided when update or check API is enabled")
	// ErrMissingDefaultMetrics indicates DefaultMetrics was not provided when
	// an endpoint that requires the metrics store is enabled.
	ErrMissingDefaultMetrics = errors.New("DefaultMetrics must be provided when update, metrics, or history API is enabled")
	// ErrMissingAPIToken indicates the API token is empty or unset.
	ErrMissingAPIToken = errors.New("API token is empty or unset")
	// ErrMissingEventsAPIToken indicates events token is not set when events API is enabled.
	ErrMissingEventsAPIToken = errors.New("events API token is required when events API is enabled")
	// ErrMissingEventBroadcaster indicates EventBroadcaster was not provided when events API is enabled.
	ErrMissingEventBroadcaster = errors.New("EventBroadcaster must be provided when events API is enabled")
	// errMissingUpdateLock indicates UpdateLock was not provided when the
	// update API is enabled.
	errMissingUpdateLock = errors.New("UpdateLock must be provided when EnableUpdateAPI is set")
	// ErrMissingTLSConfig indicates only one of TLS cert/key was provided.
	ErrMissingTLSConfig = errors.New("TLS requires both TLS Cert Path and TLS Key Path to be set")
	// ErrMissingLogger indicates Options.Logger was nil when an API endpoint is enabled.
	ErrMissingLogger = errors.New("API Logger must be provided when any HTTP API endpoint is enabled")
)

const (
	// handlerTimeout defines the maximum duration for non-update handlers to
	// complete. This prevents slow Docker API calls from blocking connections
	// indefinitely.
	handlerTimeout = 30 * time.Second
	// DefaultCheckTimeout defines the default maximum duration for the /v1/check
	// API endpoint.
	DefaultCheckTimeout = 5 * time.Minute
	// DefaultUpdateTimeout defines the default maximum duration for the /v1/update
	// API endpoint.
	DefaultUpdateTimeout = 10 * time.Minute
)

// Options holds transport and runtime configuration for SetupAndStartAPI.
type Options struct {
	// Logger is the zerolog logger for the HTTP API server and middleware.
	// Required when any HTTP API endpoint is enabled.
	// SetupAndStartAPI returns ErrMissingLogger if it is nil.
	// The composition root should pass a child with notify=no when high-volume
	// request/auth logs must not trigger notification hooks.
	// New also attaches notify=no on request and rate-limit log paths.
	Logger *zerolog.Logger
	// Host is the HTTP bind host (empty means all interfaces).
	Host string
	// Port is the HTTP bind port.
	Port string
	// Token authenticates HTTP API requests.
	Token string
	// EventsToken authenticates the events SSE endpoint.
	EventsToken string
	// RateLimit is the maximum authentication requests per minute per IP.
	RateLimit int
	// EnableUpdateAPI enables the /v1/update endpoint.
	EnableUpdateAPI bool
	// EnableMetricsAPI enables the metrics endpoint.
	EnableMetricsAPI bool
	// EnableContainersAPI enables the containers listing endpoint.
	EnableContainersAPI bool
	// EnableCheckAPI enables the /v1/check endpoint.
	EnableCheckAPI bool
	// EnableSwaggerAPI enables the Swagger UI endpoint.
	EnableSwaggerAPI bool
	// EnableHealthAPI enables health probe endpoints.
	EnableHealthAPI bool
	// EnableHistoryAPI enables the history endpoint.
	EnableHistoryAPI bool
	// EnableImagesAPI enables the images endpoint.
	EnableImagesAPI bool
	// EnableConfigAPI enables the config inspection endpoint.
	EnableConfigAPI bool
	// EnableEventsAPI enables the events SSE endpoint.
	EnableEventsAPI bool
	// UnblockHTTPAPI keeps scheduled polls running when the HTTP API is enabled.
	UnblockHTTPAPI bool
	// NoStartupMessage suppresses startup logs and notifications.
	NoStartupMessage bool
	// TLSCertPath is the path to the TLS certificate file.
	TLSCertPath string
	// TLSKeyPath is the path to the TLS key file.
	TLSKeyPath string
	// CORSAllowedOrigins lists allowed CORS origins.
	CORSAllowedOrigins []string
	// TrustedProxies lists trusted proxy IPs or CIDRs.
	TrustedProxies []string
	// ProxyHeader is the header used for the real client IP behind a reverse proxy.
	ProxyHeader string
	// Filter is the process-wide container filter predicate.
	Filter types.Filter
	// FilterDesc is a human-readable description of the filter for startup messaging.
	FilterDesc string
	// UpdateLock serializes update sessions. It must be the lock the scheduler
	// uses, so API and scheduled updates never run at the same time.
	UpdateLock *scheduling.UpdateLock
	// BaseParams is the complete update policy snapshot from config.UpdateParams.
	BaseParams types.UpdateParams
	// IncludeStopped is exposed on the config API (client list option).
	IncludeStopped bool
	// IncludeRestarting is exposed on the config API (client list option).
	IncludeRestarting bool
	// LabelEnable is exposed on the config API (filter option).
	LabelEnable bool
	// Client is the Docker client used by API handlers.
	Client container.Client
	// GitClient inspects remotes for Git-watched containers on /v1/check.
	GitClient *git.Client
	// Notifier sends update and check status messages.
	Notifier types.Notifier
	// NotificationSplitByContainer sends one notification per updated container when true.
	NotificationSplitByContainer bool
	// Scope limits operations to containers matching this Watchtower scope.
	Scope string
	// Version is the Watchtower version string used in startup messaging.
	Version string
	// Startup holds resolved values for blocking-mode startup messaging.
	Startup logging.StartupParams
	// RunUpdatesWithNotifications runs the scan-and-update pipeline for HTTP update requests.
	RunUpdatesWithNotifications func(context.Context, types.Filter, types.UpdateParams) *mt.Metric
	// FilterByImage builds an image-scoped filter for update and check requests.
	FilterByImage func([]string, types.Filter) types.Filter
	// DefaultMetrics returns the process metrics store.
	DefaultMetrics func() *mt.Metrics
	// WriteStartupMessage writes the blocking-mode startup message when the update API starts.
	WriteStartupMessage func(logging.StartupParams)
	// EventBroadcaster publishes action events to SSE subscribers.
	EventBroadcaster *events.Broadcaster
	// OnUnexpectedServerStop is invoked when the HTTP server exits with an
	// unexpected error while running in non-blocking mode. Callers typically
	// cancel the process context so scheduling shuts down with the API.
	OnUnexpectedServerStop func(error)
	// CheckTimeout is the maximum duration for the /v1/check API endpoint.
	// If zero, DefaultCheckTimeout is used.
	CheckTimeout time.Duration
	// UpdateTimeout is the maximum duration for the /v1/update API endpoint.
	// If zero, DefaultUpdateTimeout is used.
	UpdateTimeout time.Duration
}

// BuildUpdateParams returns the complete UpdateParams snapshot for HTTP-triggered updates.
//
// Policy fields come only from BaseParams so HTTP, schedule, and run-once paths share the
// same config.UpdateParams construction. RunOnce is forced false for HTTP sessions.
//
// Parameters:
//   - opts: API configuration options.
//
// Returns:
//   - types.UpdateParams: Parameters for the update pipeline.
func BuildUpdateParams(opts Options) types.UpdateParams {
	params := opts.BaseParams
	params.RunOnce = false

	// Fall back to the process-wide filter when BaseParams did not carry one.
	if params.Filter == nil {
		params.Filter = opts.Filter
	}

	return params
}

// ValidateUpdateOptions validates that all required update options are set.
//
// Parameters:
//   - opts: API configuration options to validate.
//
// Returns:
//   - error: Non-nil if any required option is missing.
func ValidateUpdateOptions(opts Options) error {
	// RunUpdatesWithNotifications executes the scan-and-update pipeline,
	// which is the core operation of the update endpoint.
	if opts.RunUpdatesWithNotifications == nil {
		return errMissingRunUpdatesWithNotifications
	}

	// FilterByImage builds an image-level predicate that the update endpoint
	// combines with container-level filters to scope which containers are
	// scanned.
	if opts.FilterByImage == nil {
		return ErrMissingFilterByImage
	}

	// DefaultMetrics provides the metrics store where the update endpoint
	// records scan results after each run.
	if opts.DefaultMetrics == nil {
		return ErrMissingDefaultMetrics
	}

	// UpdateLock keeps API updates from running alongside scheduled updates.
	// A lock of the endpoint's own would not exclude them.
	if opts.UpdateLock == nil {
		return errMissingUpdateLock
	}

	return nil
}

// TimeoutMiddleware returns a Fiber middleware that enforces a per-request
// timeout for all wrapped handlers. This prevents slow Docker API calls from
// blocking connections indefinitely.
func TimeoutMiddleware() fiber.Handler {
	return timeout.New(func(c fiber.Ctx) error {
		return c.Next()
	}, timeout.Config{
		Timeout: handlerTimeout,
	})
}
