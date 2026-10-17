// Package api registers HTTP API transport and endpoint flags.
package api

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// defaultRateLimit is the static default HTTP API rate limit per minute per IP.
const defaultRateLimit = 60

// Specs returns API domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: API flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "http-api-endpoints",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_HTTP_API_ENDPOINTS"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "HTTP API endpoints to enable (health, update, metrics, containers, check, history, images, config, events, swagger), or \"all\". Comma- or space-separated. Empty disables the HTTP API.",
		},

		{
			Name:       "http-api-update",
			Kind:       spec.KindBool,
			Default:    false,
			EnvKeys:    []string{"WATCHTOWER_HTTP_API_UPDATE"},
			Deprecated: "Use the http-api-endpoints configuration option instead.",
			Help:       "Runs Watchtower in HTTP API mode, so that image updates must be triggered by a request",
		},
		{
			Name:       "http-api-metrics",
			Kind:       spec.KindBool,
			Default:    false,
			EnvKeys:    []string{"WATCHTOWER_HTTP_API_METRICS"},
			Deprecated: "Use the http-api-endpoints configuration option instead.",
			Help:       "Runs Watchtower with the Prometheus metrics API enabled",
		},
		{
			Name:       "http-api-containers",
			Kind:       spec.KindBool,
			Default:    false,
			EnvKeys:    []string{"WATCHTOWER_HTTP_API_CONTAINERS"},
			Deprecated: "Use the http-api-endpoints configuration option instead.",
			Help:       "Runs Watchtower with the read-only containers API enabled, exposing each watched container's current image digest",
		},
		{
			Name:    "http-api-host",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_HOST"},
			Help:    "Host to bind the HTTP API to (default: empty, binds to all IPv4 interfaces; allows empty or valid IP address)",
		},
		{
			Name:    "http-api-port",
			Kind:    spec.KindString,
			Default: "8080",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_PORT"},
			Help:    "Port to bind the HTTP API to (default: 8080)",
		},
		{
			Name:    "http-api-token",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_TOKEN"},
			Help:    "Sets an authentication token for HTTP API requests (required when any non-health endpoint is enabled)",
		},
		{
			Name:    "http-api-events-token",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_EVENTS_TOKEN"},
			Help:    "Sets an authentication token for the events SSE endpoint. Required when the events endpoint is enabled. Supports Bearer header and query parameter access_token (for browser EventSource)",
		},
		{
			Name:    "http-api-periodic-polls",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_HTTP_API_PERIODIC_POLLS"},
			Help:    "Also run periodic updates (specified with --interval and --schedule) if HTTP API is enabled",
		},
		{
			Name:    "http-api-rate-limit",
			Kind:    spec.KindInt,
			Default: defaultRateLimit,
			EnvKeys: []string{"WATCHTOWER_HTTP_API_RATE_LIMIT"},
			Help:    "Maximum authentication requests per minute per IP address for the HTTP API (default: 60)",
		},
		{
			Name:    "http-api-tls-cert",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_TLS_CERT"},
			Help:    "Path to TLS certificate file for the HTTP API",
		},
		{
			Name:    "http-api-tls-key",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_TLS_KEY"},
			Help:    "Path to TLS key file for the HTTP API",
		},
		{
			Name:      "http-api-trusted-proxies",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_HTTP_API_TRUSTED_PROXIES"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "Comma-separated list of trusted proxy IP addresses or CIDR ranges for reverse proxy support (e.g. 10.0.0.0/8,172.16.0.0/12). When set, enables proxy header processing for client IP and scheme detection",
		},
		{
			Name:    "http-api-proxy-header",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_HTTP_API_PROXY_HEADER"},
			Help:    "Header to use for real client IP when behind a reverse proxy (default: X-Forwarded-For). Only used when http-api-trusted-proxies is set",
		},
		{
			Name:      "http-api-cors-origins",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_HTTP_API_CORS_ORIGINS"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "Comma-separated list of allowed CORS origins for cross-origin requests (e.g. https://app.example.com). If unset, CORS is disabled and only same-origin requests are allowed.",
		},
		{
			Name:    "http-api-check-timeout",
			Kind:    spec.KindDuration,
			Default: time.Duration(0),
			EnvKeys: []string{"WATCHTOWER_HTTP_API_CHECK_TIMEOUT"},
			Help:    "Maximum duration for the /v1/check API endpoint (e.g. 30s, 2m, 5m). Default: 5m",
		},
		{
			Name:    "http-api-update-timeout",
			Kind:    spec.KindDuration,
			Default: time.Duration(0),
			EnvKeys: []string{"WATCHTOWER_HTTP_API_UPDATE_TIMEOUT"},
			Help:    "Maximum duration for the /v1/update API endpoint (e.g. 1m, 10m, 30m). Default: 10m",
		},
	}
}

// Register adds HTTP API domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
