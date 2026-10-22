package config

import (
	"errors"
	"fmt"
	"net"

	"github.com/spf13/cobra"

	apiConfig "github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// errInvalidAPIHost indicates http-api-host is neither empty nor a valid IP.
var errInvalidAPIHost = errors.New(
	"invalid http-api-host: must be empty or a valid IP address (IPv4 or IPv6)",
)

// RunConfigInput supplies runtime values needed to project Config into types.RunConfig.
type RunConfigInput struct {
	// Command is the cobra command retained on types.RunConfig for callers that still
	// carry a command reference through orchestration (not used for policy flag reads).
	Command *cobra.Command
	// Names are normalized container names for filtering display.
	Names []string
}

// BuildRunConfig projects Config into types.RunConfig for process orchestration.
//
// It applies API port and rate-limit defaults, validates the API host, resolves
// endpoint enablement from cfg.API, and returns a complete RunConfig snapshot.
//
// Parameters:
//   - input: Runtime inputs (command pointer and normalized names).
//
// Returns:
//   - types.RunConfig: Orchestration snapshot for runMain.
//   - error: Non-nil when API host or endpoint configuration is invalid.
func (c Config) BuildRunConfig(input RunConfigInput) (types.RunConfig, error) {
	// Default port when unset so HTTP API bind always has a concrete value.
	apiPort := c.API.Port
	if apiPort == "" {
		apiPort = "8080"
	}

	// Default rate limit when unset or invalid.
	apiRateLimit := c.API.RateLimit
	if apiRateLimit <= 0 {
		apiRateLimit = 60
	}

	err := ValidateAPIHost(c.API.Host)
	if err != nil {
		return types.RunConfig{}, err
	}

	endpointSet, err := apiConfig.ResolveEndpoints(
		c.API.Endpoints,
		c.API.LegacyUpdate,
		c.API.LegacyMetrics,
		c.API.LegacyContainers,
	)
	if err != nil {
		return types.RunConfig{}, fmt.Errorf("invalid HTTP API endpoint configuration: %w", err)
	}

	cfg := types.RunConfig{
		Command:                 input.Command,
		Names:                   append([]string(nil), input.Names...),
		Filter:                  c.Filter.Predicate,
		FilterDesc:              c.Filter.Desc,
		RunOnce:                 c.Mode.RunOnce,
		UpdateOnStart:           c.Schedule.UpdateOnStart,
		TLSCertPath:             c.API.TLSCert,
		TLSKeyPath:              c.API.TLSKey,
		CORSAllowedOrigins:      append([]string(nil), c.API.CORSOrigins...),
		TrustedProxies:          append([]string(nil), c.API.TrustedProxies...),
		ProxyHeader:             c.API.ProxyHeader,
		UnblockHTTPAPI:          c.API.PeriodicPolls,
		NoStartupMessage:        c.Mode.NoStartupMessage,
		APIToken:                c.API.Token,
		APIEventsToken:          c.API.EventsToken,
		APIHost:                 c.API.Host,
		APIHostChanged:          c.API.HostChanged,
		APIPort:                 apiPort,
		APIPortChanged:          c.API.PortChanged,
		APIRateLimit:            apiRateLimit,
		APIRateLimitChanged:     c.API.RateLimitChanged,
		CheckAPITimeout:         c.API.CheckTimeout,
		CheckAPITimeoutChanged:  c.API.CheckTimeoutChanged,
		UpdateAPITimeout:        c.API.UpdateTimeout,
		UpdateAPITimeoutChanged: c.API.UpdateTimeoutChanged,
	}

	// Map resolved endpoint names onto Enable* fields on RunConfig.
	apiConfig.SetEndpointConfig(endpointSet, &cfg)

	return cfg, nil
}

// StartupParams builds logging.StartupParams from Config and RunConfig endpoint state.
//
// Runtime fields such as Sched, Filtering, Client, Notifier, and Version are left for
// callers to fill when invoking WriteStartupMessage.
//
// Parameters:
//   - run: RunConfig with resolved endpoint enablement.
//
// Returns:
//   - logging.StartupParams: Mode and HTTP API values for startup messaging.
func (c Config) StartupParams(run types.RunConfig) logging.StartupParams {
	return logging.StartupParams{
		NoStartupMessage:     c.Mode.NoStartupMessage,
		RunOnce:              c.Mode.RunOnce,
		HTTPAPIUpdate:        run.EnableUpdateAPI,
		HTTPAPIPeriodicPolls: run.UnblockHTTPAPI,
		DiskSpaceMaxBytes:    c.Update.DiskSpaceMaxBytes,
		DiskSpaceWarnBytes:   c.Update.DiskSpaceWarnBytes,
	}
}

// AnyHTTPAPIConfig reports whether any HTTP API-related settings are present
// without enabled endpoints, so operators can be warned about a no-op config.
func AnyHTTPAPIConfig(cfg types.RunConfig) bool {
	return cfg.APIToken != "" ||
		cfg.APIEventsToken != "" ||
		cfg.TLSCertPath != "" ||
		cfg.TLSKeyPath != "" ||
		len(cfg.CORSAllowedOrigins) > 0 ||
		len(cfg.TrustedProxies) > 0 ||
		cfg.ProxyHeader != "" ||
		cfg.APIHostChanged ||
		cfg.APIPortChanged ||
		cfg.APIRateLimitChanged ||
		cfg.CheckAPITimeoutChanged ||
		cfg.UpdateAPITimeoutChanged
}

// HTTPAPIEndpointsEnabled reports whether any HTTP API endpoint is enabled.
func HTTPAPIEndpointsEnabled(cfg types.RunConfig) bool {
	return cfg.EnableUpdateAPI ||
		cfg.EnableMetricsAPI ||
		cfg.EnableContainersAPI ||
		cfg.EnableCheckAPI ||
		cfg.EnableSwaggerAPI ||
		cfg.EnableHealthAPI ||
		cfg.EnableHistoryAPI ||
		cfg.EnableImagesAPI ||
		cfg.EnableConfigAPI ||
		cfg.EnableEventsAPI
}

// ValidateAPIHost ensures http-api-host is empty (all interfaces) or a valid IP.
//
// Parameters:
//   - host: Value of the http-api-host setting.
//
// Returns:
//   - error: Non-nil when host is a non-empty non-IP string (for example a hostname).
func ValidateAPIHost(host string) error {
	if host == "" {
		return nil
	}

	if net.ParseIP(host) == nil {
		return fmt.Errorf("%w: %q", errInvalidAPIHost, host)
	}

	return nil
}
