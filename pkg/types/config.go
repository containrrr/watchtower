package types

import (
	"time"

	"github.com/spf13/cobra"
)

// RunConfig encapsulates the configuration parameters for the runMain function.
type RunConfig struct {
	// Command is the cobra.Command instance representing the executed command, providing access to parsed flags.
	Command *cobra.Command
	// Names is a slice of container names explicitly provided as positional arguments, used for filtering.
	Names []string
	// Filter is the types.Filter function determining which containers are processed during updates.
	Filter Filter
	// FilterDesc is a human-readable description of the applied filter, used in logging and notifications.
	FilterDesc string
	// RunOnce indicates whether to perform a single update and exit.
	RunOnce bool
	// UpdateOnStart enables an immediate update check on startup, then continues with periodic updates.
	UpdateOnStart bool
	// EnableCheckAPI enables the check API endpoint.
	EnableCheckAPI bool
	// EnableConfigAPI enables the config API endpoint.
	EnableConfigAPI bool
	// EnableContainersAPI enables the containers API endpoint.
	EnableContainersAPI bool
	// EnableEventsAPI enables the events API endpoint.
	EnableEventsAPI bool
	// EnableHealthAPI enables the HTTP API health probes.
	EnableHealthAPI bool
	// EnableHistoryAPI enables the history API endpoint.
	EnableHistoryAPI bool
	// EnableImagesAPI enables the images API endpoint.
	EnableImagesAPI bool
	// EnableMetricsAPI enables the metrics API endpoint.
	EnableMetricsAPI bool
	// EnableSwaggerAPI enables Swagger UI endpoint.
	EnableSwaggerAPI bool
	// EnableUpdateAPI enables the update API endpoint.
	EnableUpdateAPI bool
	// UnblockHTTPAPI allows periodic polling alongside the HTTP API.
	UnblockHTTPAPI bool
	// APIToken is the authentication token for HTTP API access.
	APIToken string
	// APIEventsToken is the authentication token for the events SSE endpoint.
	APIEventsToken string
	// APIHost is the host interface to bind the HTTP API to (default: empty string).
	APIHost string
	// APIPort is the port for the HTTP API server (defaults to "8080").
	APIPort string
	// APIRateLimit is the maximum authentication requests per minute per IP address (default: 60).
	APIRateLimit int
	// NoStartupMessage suppresses startup messages if true.
	NoStartupMessage bool
	// TLSCertPath is the path to the TLS certificate file.
	TLSCertPath string
	// TLSKeyPath is the path to the TLS key file.
	TLSKeyPath string
	// CORSAllowedOrigins is a list of allowed CORS origins for cross-origin requests.
	CORSAllowedOrigins []string
	// TrustedProxies is a list of trusted proxy IPs/CIDRs for reverse proxy support.
	TrustedProxies []string
	// ProxyHeader is the header to use for real client IP behind a reverse proxy.
	ProxyHeader string
	// APIHostChanged reports whether http-api-host was explicitly configured.
	APIHostChanged bool
	// APIPortChanged reports whether http-api-port was explicitly configured.
	APIPortChanged bool
	// APIRateLimitChanged reports whether http-api-rate-limit was explicitly configured.
	APIRateLimitChanged bool
	// CheckAPITimeout is the maximum duration for the /v1/check API endpoint.
	CheckAPITimeout time.Duration
	// CheckAPITimeoutChanged reports whether http-api-check-timeout was explicitly configured.
	CheckAPITimeoutChanged bool
	// UpdateAPITimeout is the maximum duration for the /v1/update API endpoint.
	UpdateAPITimeout time.Duration
	// UpdateAPITimeoutChanged reports whether http-api-update-timeout was explicitly configured.
	UpdateAPITimeoutChanged bool
}
