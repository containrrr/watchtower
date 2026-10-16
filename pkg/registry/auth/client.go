package auth

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
)

// Constants for HTTP client configuration.
// These values define timeouts and connection limits for registry requests.
const (
	DefaultTimeoutSeconds             = 30  // Default timeout for HTTP requests in seconds
	DefaultMaxIdleConns               = 100 // Maximum number of idle connections in the pool
	DefaultIdleConnTimeoutSeconds     = 90  // Timeout for idle connections in seconds
	DefaultTLSHandshakeTimeoutSeconds = 10  // Timeout for TLS handshake in seconds
	DefaultExpectContinueTimeout      = 1   // Timeout for expecting continue response in seconds
	DefaultDialTimeoutSeconds         = 30  // Timeout for establishing TCP connections in seconds
	DefaultDialKeepAliveSeconds       = 30  // Keep-alive probes for persistent connections in seconds
	DefaultMaxRedirects               = 3   // Maximum number of redirects to follow (reduced from Go's default of 10)
)

// TLSVersionMap maps string names to TLS version constants.
// It provides a lookup for configuring the minimum TLS version based on user settings.
// Only TLS 1.2 and TLS 1.3 are accepted as valid minimum versions.
// Older versions are rejected by the lookup and fall back to the default.
var TLSVersionMap = map[string]uint16{
	"TLS1.2": tls.VersionTLS12,
	"TLS1.3": tls.VersionTLS13,
}

// Cached clients, one per TLS configuration, so registry requests made with
// the same settings share one connection pool.
var (
	// clientsMu guards clients.
	clientsMu sync.Mutex
	// clients maps a TLS configuration to the Client built for it.
	clients = map[clientKey]Client{}
)

// Client defines the interface for executing HTTP requests to container registries.
//
// This interface abstracts the HTTP client used for authentication operations, enabling
// dependency injection and facilitating unit testing with mock implementations.
type Client interface {
	// Do executes the provided HTTP request and returns the response or an error.
	//
	// Parameters:
	//   - req: The HTTP request to execute.
	//
	// Returns:
	//   - *http.Response: The HTTP response from the registry, if successful.
	//   - error: Non-nil if the request fails, nil otherwise.
	Do(req *http.Request) (*http.Response, error)
}

// clientKey identifies the TLS configuration a cached Client was built with.
type clientKey struct {
	// minVersion is the resolved minimum TLS version.
	minVersion uint16
	// skipVerify reports whether certificate verification is disabled.
	skipVerify bool
}

// registryClient is a concrete implementation of the Client interface.
//
// It encapsulates an HTTP client configured for registry interactions, providing a
// mechanism to execute authenticated requests with customizable TLS settings.
type registryClient struct {
	client *http.Client // The underlying HTTP client for making requests.
}

// Do executes an HTTP request using the underlying HTTP client.
//
// This method satisfies the Client interface, delegating the request execution
// to the embedded HTTP client.
//
// Parameters:
//   - request: The HTTP request to execute.
//
// Returns:
//   - *http.Response: The HTTP response from the registry, if successful.
//   - error: Non-nil if the request fails, nil otherwise.
func (r *registryClient) Do(request *http.Request) (*http.Response, error) {
	response, err := r.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}

	return response, nil
}

// ConfigureTLS applies the registry TLS settings to a TLS configuration.
//
// Parameters:
//   - log: Logger for TLS diagnostics. May be nil (events are skipped).
//   - tlsConfig: The base TLS configuration to modify.
//   - settings: Registry request settings.
func ConfigureTLS(log *zerolog.Logger, tlsConfig *tls.Config, settings registryConfig.Registry) {
	// InsecureSkipVerify is intentional when the registry-tls-skip
	// configuration option is set.
	if settings.TLSSkip {
		tlsConfig.InsecureSkipVerify = true

		if log != nil {
			log.Debug().
				Msg("TLS certificate verification disabled via WATCHTOWER_REGISTRY_TLS_SKIP")
		}
	}

	minVersion := settings.TLSMinVersion
	if minVersion != "" {
		version, ok := minTLSVersion(minVersion)
		if !ok && log != nil {
			log.Warn().
				Str("configured_version", minVersion).
				Str("fallback_version", "TLS1.2").
				Msg("Invalid WATCHTOWER_REGISTRY_TLS_MIN_VERSION. Falling back to TLS 1.2")
		}

		tlsConfig.MinVersion = version
	}
}

// minTLSVersion resolves a configured minimum TLS version name.
//
// Names are matched case-insensitively. An empty name means TLS 1.2, and a
// name that TLSVersionMap does not accept falls back to TLS 1.2.
//
// Parameters:
//   - name: Configured minimum TLS version, such as "TLS1.3".
//
// Returns:
//   - uint16: The resolved TLS version.
//   - bool: False when a non-empty name was not accepted.
func minTLSVersion(name string) (uint16, bool) {
	if name == "" {
		return tls.VersionTLS12, true
	}

	version, ok := TLSVersionMap[strings.ToUpper(name)]
	if !ok {
		return tls.VersionTLS12, false
	}

	return version, true
}

// buildRegistryTransport creates an HTTP transport with the given TLS configuration.
//
// Parameters:
//   - tlsConfig: The TLS configuration to use.
//
// Returns:
//   - *http.Transport: Configured transport for registry requests.
func buildRegistryTransport(tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		TLSClientConfig:       tlsConfig,                                       // TLS configuration for secure registry connections.
		Proxy:                 http.ProxyFromEnvironment,                       // Respect proxy environment variables (e.g., HTTP_PROXY, HTTPS_PROXY).
		MaxIdleConns:          DefaultMaxIdleConns,                             // Maximum number of idle connections to keep open.
		IdleConnTimeout:       DefaultIdleConnTimeoutSeconds * time.Second,     // Timeout for closing idle connections.
		TLSHandshakeTimeout:   DefaultTLSHandshakeTimeoutSeconds * time.Second, // Timeout for completing TLS handshakes.
		ExpectContinueTimeout: DefaultExpectContinueTimeout * time.Second,      // Timeout for receiving HTTP 100-Continue responses.
		DialContext: (&net.Dialer{
			Timeout:   DefaultDialTimeoutSeconds * time.Second,   // Timeout for establishing TCP connections.
			KeepAlive: DefaultDialKeepAliveSeconds * time.Second, // Keep-alive probes for persistent connections.
		}).DialContext,
	}
}

// buildRegistryClient creates an HTTP client with the configured transport and timeouts.
//
// Parameters:
//   - tlsConfig: The TLS configuration to use.
//
// Returns:
//   - *http.Client: Configured HTTP client for registry requests.
func buildRegistryClient(tlsConfig *tls.Config) *http.Client {
	return &http.Client{
		Transport: buildRegistryTransport(tlsConfig),
		Timeout:   DefaultTimeoutSeconds * time.Second, // Overall timeout for HTTP requests.
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= DefaultMaxRedirects { // Limit redirects to prevent excessive loops or attacks.
				return http.ErrUseLastResponse
			}

			return nil
		},
	}
}

// NewAuthClient returns the Client for registry requests made with settings.
//
// Clients are cached by their resolved TLS configuration, so calls whose
// settings resolve to the same configuration share one client and its
// connection pool, and calls with different settings get clients configured
// for them. The client is configured with default
// timeouts and connection limits for registry access.
//
// Parameters:
//   - log: Logger for TLS configuration diagnostics when a client is built.
//     May be nil (invalid-version warning is skipped).
//   - settings: Registry request settings.
//
// Returns:
//   - Client: Ready for registry authentication requests.
func NewAuthClient(log *zerolog.Logger, settings registryConfig.Registry) Client {
	minVersion, _ := minTLSVersion(settings.TLSMinVersion)
	key := clientKey{minVersion: minVersion, skipVerify: settings.TLSSkip}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	cached, ok := clients[key]
	if ok {
		return cached
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Default to TLS 1.2 for secure communication.
	}

	ConfigureTLS(log, tlsConfig, settings)

	client := &registryClient{
		client: buildRegistryClient(tlsConfig),
	}
	clients[key] = client

	return client
}

// Scheme returns the URL scheme for registry requests made with settings.
//
// Parameters:
//   - settings: Registry request settings.
//
// Returns:
//   - string: "http" when the registry-tls-skip configuration option is set,
//     otherwise "https".
func Scheme(settings registryConfig.Registry) string {
	if settings.TLSSkip {
		return "http"
	}

	return "https"
}
