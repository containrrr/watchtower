// Package api holds HTTP API transport and endpoint settings.
package api

import "time"

// API holds HTTP API server configuration.
type API struct {
	// Endpoints lists enabled endpoint names, or contains "all".
	Endpoints []string
	// LegacyUpdate enables the legacy http-api-update flag behavior.
	LegacyUpdate bool
	// LegacyMetrics enables the legacy http-api-metrics flag behavior.
	LegacyMetrics bool
	// LegacyContainers enables the legacy http-api-containers flag behavior.
	LegacyContainers bool
	// Host is the bind host.
	Host string
	// HostChanged is true when the host flag was explicitly set.
	HostChanged bool
	// Port is the bind port.
	Port string
	// PortChanged is true when the port flag was explicitly set.
	PortChanged bool
	// Token is the API authentication token.
	Token string
	// EventsToken is the events SSE authentication token.
	EventsToken string
	// PeriodicPolls keeps scheduled polls when the HTTP API is enabled.
	PeriodicPolls bool
	// RateLimit is max auth requests per minute per IP.
	RateLimit int
	// RateLimitChanged is true when the rate-limit flag was explicitly set.
	RateLimitChanged bool
	// TLSCert is the path to the TLS certificate file.
	TLSCert string
	// TLSKey is the path to the TLS key file.
	TLSKey string
	// TrustedProxies lists trusted proxy CIDRs or IPs.
	TrustedProxies []string
	// ProxyHeader is the header used for the real client IP.
	ProxyHeader string
	// CORSOrigins lists allowed CORS origins.
	CORSOrigins []string
	// CheckTimeout is the max duration for /v1/check.
	CheckTimeout time.Duration
	// CheckTimeoutChanged is true when the check-timeout flag was explicitly set.
	CheckTimeoutChanged bool
	// UpdateTimeout is the max duration for /v1/update.
	UpdateTimeout time.Duration
	// UpdateTimeoutChanged is true when the update-timeout flag was explicitly set.
	UpdateTimeoutChanged bool
}
