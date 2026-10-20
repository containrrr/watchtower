package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Errors for auth operations.
var (
	errFailedCreateChallengeRequest  = errors.New("failed to create challenge request")
	errFailedExecuteChallengeRequest = errors.New("failed to execute challenge request")
	errNoCredentials                 = errors.New("no credentials available")
	errUnsupportedChallenge          = errors.New("unsupported challenge type from registry")
	errUnexpectedStatus              = errors.New("unexpected status with empty WWW-Authenticate header")
	errCrossOriginRedirect           = errors.New("cross-origin redirect not allowed for basic auth")
	errInvalidEndpoint               = errors.New("invalid registry endpoint")
)

// TokenResult holds the result of a token acquisition operation.
type TokenResult struct {
	// Token is the authentication token (for example "Basic ..." or "Bearer ...").
	Token string
	// ChallengeHost is the challenge host (for example "ghcr.io"), empty if not applicable.
	ChallengeHost string
	// Redirected is true if the challenge request was redirected.
	Redirected bool
	// RedirectHost is the final host after redirects, empty if not redirected.
	RedirectHost string
}

// challengeResponse holds parsed data from a challenge HTTP response.
type challengeResponse struct {
	// statusCode is the HTTP status code from the challenge response.
	statusCode int
	// wwwAuthHeader is the WWW-Authenticate header value.
	wwwAuthHeader string
	// redirected reports whether the request was redirected.
	redirected bool
	// redirectHost is the final host after redirects, empty if not redirected.
	redirectHost string
}

// resolveChallengeScheme determines the HTTP scheme for registry requests.
//
// Parameters:
//   - log: Logger for scheme selection diagnostics.
//   - host: The registry host.
//   - settings: Registry request settings.
//
// Returns:
//   - string: The resolved scheme ("http" or "https").
func resolveChallengeScheme(log *zerolog.Logger, host string, settings registryConfig.Registry) string {
	scheme := Scheme(settings)
	if settings.TLSSkip {
		log.Debug().
			Str("host", host).
			Msg("Using HTTP scheme due to WATCHTOWER_REGISTRY_TLS_SKIP")
	}

	return scheme
}

// ResolveEndpoint returns the host and scheme of a registry endpoint override,
// such as a registry mirror.
//
// An endpoint is either an http or https URL with a host, such as
// "https://mirror.example.com:5000", or a bare host with an optional port, such
// as "mirror.example.com:5000", "127.0.0.1:5000", or "[::1]:5000". A bare host
// or a scheme-relative URL keeps the given scheme. An empty endpoint selects the
// canonical host over https.
//
// Parameters:
//   - endpoint: Registry endpoint override, or empty for the canonical host.
//   - canonical: Canonical registry host, used when endpoint is empty.
//   - scheme: Scheme for an endpoint that does not name one.
//
// Returns:
//   - string: The resolved host.
//   - string: The resolved scheme.
//   - error: Wraps errInvalidEndpoint when endpoint is neither an http or https
//     URL with a host nor a bare host, nil otherwise.
func ResolveEndpoint(endpoint, canonical, scheme string) (string, string, error) {
	if endpoint == "" {
		return canonical, "https", nil
	}

	// A bare host[:port] does not parse as a URL with a host, so it is the
	// whole endpoint, with the given scheme.
	host, hostScheme := endpoint, scheme

	endpointURL, err := url.Parse(endpoint)
	if err == nil && endpointURL.Host != "" {
		host = endpointURL.Host

		switch endpointURL.Scheme {
		case "":
		case "http", "https":
			hostScheme = endpointURL.Scheme
		default:
			return "", "", fmt.Errorf("%w: unsupported scheme %q", errInvalidEndpoint, endpointURL.Scheme)
		}
	}

	// The errors leave out the endpoint, which can carry credentials.
	if !isRequestHost(host, hostScheme) {
		return "", "", fmt.Errorf("%w: not a URL with a host or a bare host", errInvalidEndpoint)
	}

	return host, hostScheme, nil
}

// isRequestHost reports whether host, with an optional port, is the whole
// authority of a request URL with the given scheme. A URL built with it then
// parses back to the same host.
//
// Parameters:
//   - host: Host with an optional port, such as "mirror.example.com:5000".
//   - scheme: Request scheme.
//
// Returns:
//   - bool: True when the URL built with host parses back to it.
func isRequestHost(host, scheme string) bool {
	parsed, err := url.Parse(scheme + "://" + host + "/v2/")

	return err == nil && parsed.Host == host && parsed.Hostname() != ""
}

// getChallengeURL generates a challenge URL for accessing an image's registry.
//
// When endpoint is non-empty, it is used as the registry host for the challenge URL instead
// of the canonical registry host. The endpoint may include a scheme (for example
// "https://mirror.example.com") or be a bare hostname.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - imageRef: Normalized image reference.
//   - settings: Registry request settings.
//   - endpoint: Optional registry host override (for example a mirror address). Empty uses the canonical host.
//
// Returns:
//   - url.URL: Generated challenge URL.
//   - error: Wraps errInvalidEndpoint when endpoint is not a valid registry endpoint.
func getChallengeURL(
	log *zerolog.Logger,
	imageRef reference.Named,
	settings registryConfig.Registry,
	endpoint string,
) (url.URL, error) {
	host, _ := GetRegistryAddress(log, imageRef.Name())

	scheme := resolveChallengeScheme(log, host, settings)

	if endpoint != "" {
		endpointHost, endpointScheme, err := ResolveEndpoint(endpoint, host, scheme)
		if err != nil {
			return url.URL{}, err
		}

		host = endpointHost
		scheme = endpointScheme
	}

	challengeURL := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   "/v2/",
	}
	log.Debug().
		Str("image", imageRef.Name()).
		Str("url", challengeURL.String()).
		Msg("Generated challenge URL")

	return challengeURL, nil
}

// getChallengeRequest creates a request for retrieving challenge instructions.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - ctx: Context for request lifecycle control.
//   - challengeURL: URL for the challenge request.
//
// Returns:
//   - *http.Request: Constructed request if successful.
//   - error: Non-nil if creation fails, nil on success.
func getChallengeRequest(log *zerolog.Logger, ctx context.Context, challengeURL url.URL) (*http.Request, error) {
	// Create a GET request with context for cancellation and timeout.
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		challengeURL.String(),
		nil,
	)
	if err != nil {
		log.Debug().
			Err(err).
			Str("url", challengeURL.String()).
			Msg("Failed to create challenge request")

		return nil, fmt.Errorf("%w: %w", errFailedCreateChallengeRequest, err)
	}

	// Set headers for compatibility and identification.
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", meta.UserAgent)

	log.Debug().
		Str("url", challengeURL.String()).
		Msg("Created challenge request")

	return request, nil
}

// handleSuccessfulChallenge returns a TokenResult indicating no authentication is required.
//
// Parameters:
//   - redirected: Whether the challenge request was redirected.
//   - redirectHost: The final host after redirects.
//
// Returns:
//   - TokenResult: Result with empty token and no challenge host.
func handleSuccessfulChallenge(redirected bool, redirectHost string) TokenResult {
	return TokenResult{
		Redirected:    redirected,
		RedirectHost:  redirectHost,
		ChallengeHost: "",
		Token:         "",
	}
}

// handleBasicAuthChallenge returns a TokenResult with Basic authentication.
//
// Parameters:
//   - log: Contextual logger (typically scoped with image and related fields).
//   - registryAuth: Base64-encoded auth string.
//   - redirected: Whether the challenge request was redirected.
//   - redirectHost: The final host after redirects.
//   - originalHost: The original registry host the challenge request was sent to.
//   - originalScheme: Scheme of the original challenge request.
//   - redirectScheme: Scheme of the final redirect destination.
//
// Returns:
//   - TokenResult: Result with Basic auth token.
//   - error: Non-nil if no credentials are provided or a cross-origin redirect is detected.
func handleBasicAuthChallenge(
	log *zerolog.Logger,
	registryAuth string,
	redirected bool,
	redirectHost string,
	originalHost string,
	originalScheme string,
	redirectScheme string,
) (TokenResult, error) {
	if registryAuth == "" {
		log.Debug().Msg("No credentials provided for Basic auth")

		return TokenResult{}, fmt.Errorf("%w: basic auth required", errNoCredentials)
	}

	// Reject Basic auth when the challenge follows a cross-origin redirect to prevent
	// credential forwarding to an unintended host.
	if redirected &&
		redirectHost != "" &&
		originalHost != "" &&
		!strings.EqualFold(redirectHost, originalHost) {
		log.Debug().
			Str("original_host", originalHost).
			Str("redirect_host", redirectHost).
			Msg("Cross-origin redirect detected. Rejecting Basic auth challenge")

		redirectURL := (&url.URL{Scheme: redirectScheme, Host: redirectHost}).String()
		originalURL := (&url.URL{Scheme: originalScheme, Host: originalHost}).String()

		return TokenResult{}, fmt.Errorf("%w: %s -> %s", errCrossOriginRedirect, originalURL, redirectURL)
	}

	// Reject Basic auth when the challenge redirects from HTTPS to HTTP to prevent
	// credential exposure over an unencrypted connection.
	if redirected && originalScheme == "https" && redirectScheme == "http" {
		log.Debug().
			Str("original_scheme", originalScheme).
			Str("redirect_scheme", redirectScheme).
			Msg("HTTPS to HTTP downgrade detected. Rejecting Basic auth challenge")

		redirectURL := (&url.URL{Scheme: redirectScheme, Host: redirectHost}).String()
		originalURL := (&url.URL{Scheme: originalScheme, Host: originalHost}).String()

		return TokenResult{}, fmt.Errorf("%w: %s -> %s", errCrossOriginRedirect, originalURL, redirectURL)
	}

	log.Debug().Msg("Using Basic auth")

	return TokenResult{
		Token:         "Basic " + registryAuth,
		ChallengeHost: "",
		Redirected:    redirected,
		RedirectHost:  redirectHost,
	}, nil
}

// handleUnsupportedChallenge returns an error for unsupported challenge types.
//
// Parameters:
//   - log: Contextual logger.
//   - challenge: The unsupported challenge type.
//
// Returns:
//   - TokenResult: Empty result.
//   - error: Non-nil describing the unsupported challenge.
func handleUnsupportedChallenge(log *zerolog.Logger, challenge string) (TokenResult, error) {
	log.Error().
		Str("challenge", challenge).
		Msg("Unsupported challenge type from registry")

	return TokenResult{}, fmt.Errorf("%w: %s", errUnsupportedChallenge, challenge)
}

// processChallengeResponse interprets the challenge HTTP response and routes to the appropriate auth handler.
//
// Parameters:
//   - log: Contextual logger.
//   - ctx: Context for request lifecycle control.
//   - container: Container with image info.
//   - registryAuth: Base64-encoded auth string.
//   - client: Client for HTTP requests.
//   - settings: Registry request settings.
//   - redirected: Whether the challenge request was redirected.
//   - redirectHost: The final host after redirects.
//   - originalHost: The original registry host the challenge request was sent to.
//   - originalScheme: The scheme used for the original challenge request.
//   - redirectScheme: The scheme used for the final redirect destination.
//   - response: Parsed challenge response data.
//
// Returns:
//   - TokenResult: Authentication result containing token and redirect info.
//   - error: Non-nil if operation fails, nil on success.
func processChallengeResponse(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
	redirected bool,
	redirectHost string,
	originalHost string,
	originalScheme string,
	redirectScheme string,
	response challengeResponse,
) (TokenResult, error) {
	// Handle 200 OK response (no auth required).
	if response.statusCode == http.StatusOK {
		log.Debug().
			Str("url", response.wwwAuthHeader).
			Msg("No authentication required (200 OK)")

		return handleSuccessfulChallenge(redirected, redirectHost), nil
	}

	// If the header is empty, return an authentication error for 401 responses.
	if response.wwwAuthHeader == "" {
		if response.statusCode == http.StatusUnauthorized {
			return TokenResult{}, fmt.Errorf("%w: no credentials available", errNoCredentials)
		}

		return TokenResult{}, fmt.Errorf(
			"%w: status %d",
			errUnexpectedStatus,
			response.statusCode,
		)
	}

	// Normalize challenge for comparison.
	challenge := strings.ToLower(strings.TrimSpace(response.wwwAuthHeader))
	log.Debug().
		Str("challenge", challenge).
		Msg("Processing challenge type")

	// Handle Basic auth if specified.
	if strings.HasPrefix(challenge, "basic") {
		return handleBasicAuthChallenge(
			log,
			registryAuth,
			redirected,
			redirectHost,
			originalHost,
			originalScheme,
			redirectScheme,
		)
	}

	// Handle Bearer auth.
	if strings.HasPrefix(challenge, "bearer") {
		return handleBearerAuth(
			log,
			ctx,
			response.wwwAuthHeader,
			container,
			registryAuth,
			client,
			settings,
			redirected,
			redirectHost,
		)
	}

	return handleUnsupportedChallenge(log, challenge)
}

// handleBearerAuth handles the Bearer authentication challenge.
//
// Parameters:
//   - log: Contextual logger.
//   - ctx: Context for request lifecycle control.
//   - wwwAuthHeader: The WWW-Authenticate header value.
//   - container: Container with image info.
//   - registryAuth: Base64-encoded auth string.
//   - client: Client for HTTP requests.
//   - settings: Registry request settings.
//   - redirected: Whether the challenge request was redirected.
//   - redirectHost: The final host after redirects.
//
// Returns:
//   - TokenResult: Authentication result containing token and redirect info.
//   - error: Non-nil if operation fails, nil on success.
func handleBearerAuth(
	log *zerolog.Logger,
	ctx context.Context,
	wwwAuthHeader string,
	container types.Container,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
	redirected bool,
	redirectHost string,
) (TokenResult, error) {
	log.Debug().Msg("Entering Bearer auth path")

	normalizedRef, err := reference.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to parse image name")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedParseImageName, err)
	}

	authURL, err := getAuthURL(
		log,
		strings.ToLower(wwwAuthHeader),
		normalizedRef,
		registryAuth,
		settings,
	)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to construct bearer auth URL")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedConstructBearerAuthURL, err)
	}

	challengeHost := authURL.Host
	if challengeHost != "" {
		log.Debug().
			Str("challenge_host", challengeHost).
			Msg("Extracted challenge host")
	}

	token, err := getBearerToken(
		log,
		ctx,
		wwwAuthHeader,
		normalizedRef,
		registryAuth,
		client,
		settings,
	)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to get bearer token")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedDecodeResponse, err)
	}

	if token == "" {
		log.Debug().Msg("Empty bearer token received")

		return TokenResult{}, fmt.Errorf(
			"%w: empty token in response",
			errFailedDecodeResponse,
		)
	}

	log.Debug().
		Bool("token_present", token != "").
		Str("challenge_host", challengeHost).
		Msg("Returning Bearer token and challenge host")

	return TokenResult{
		Token:         token,
		ChallengeHost: challengeHost,
		Redirected:    redirected,
		RedirectHost:  redirectHost,
	}, nil
}

// GetToken fetches a token and the challenge host for the registry hosting the provided image.
//
// When endpoint is non-empty, it is used as the registry host for the challenge URL
// instead of the canonical registry host. This enables digest checks against configured
// Docker registry mirrors.
//
// Anonymous GHCR requests skip the /v2/ challenge when a shared, unexpired
// bearer token is already cached.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for request lifecycle control.
//   - container: Container with image info.
//   - registryAuth: Base64-encoded auth string.
//   - client: Client for HTTP requests.
//   - settings: Registry request settings.
//   - endpoint: Optional registry host override (for example a mirror address). Empty uses the canonical host.
//
// Returns:
//   - TokenResult: Authentication result containing token, challenge host, and redirect info.
//   - error: Non-nil if operation fails, nil on success.
func GetToken(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
	endpoint string,
) (TokenResult, error) {
	// Scope all subsequent logs in this call with the image name.
	clog := log.With().Str("image", container.ImageName()).Logger()

	// Parse image name into a normalized reference.
	normalizedRef, err := reference.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to parse image name")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedParseImageName, err)
	}

	result, ok := cachedAnonymousGHCRToken(
		&clog,
		normalizedRef,
		registryAuth,
		endpoint,
	)
	if ok {
		clog.Debug().
			Str("challenge_host", result.ChallengeHost).
			Msg("Using cached anonymous GHCR token, skipping challenge")

		return result, nil
	}

	// Generate the challenge URL, using the endpoint override if provided.
	challengeURL, err := getChallengeURL(&clog, normalizedRef, settings, endpoint)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to build challenge URL")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedCreateChallengeRequest, err)
	}

	clog.Debug().
		Str("url", challengeURL.String()).
		Msg("Constructed challenge URL")

	// Build and execute the challenge request.
	request, err := getChallengeRequest(&clog, ctx, challengeURL)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to create challenge request")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedCreateChallengeRequest, err)
	}

	response, err := client.Do(request)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("url", challengeURL.String()).
			Msg("Failed to execute challenge request")

		return TokenResult{}, fmt.Errorf("%w: %w", errFailedExecuteChallengeRequest, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusTooManyRequests {
		body := ratelimit.ReadBody(response, ratelimit.DefaultBodyLimit)

		return TokenResult{}, ratelimit.FromResponse(response, body)
	}

	// Detect if the request was redirected by comparing the complete final URL.
	redirected := response.Request.URL.String() != challengeURL.String()

	// Capture the final host after redirects for use by callers.
	var (
		redirectHost   string
		redirectScheme string
	)

	if redirected {
		redirectHost = response.Request.URL.Host
		redirectScheme = response.Request.URL.Scheme

		clog.Debug().
			Str("redirect_host", redirectHost).
			Msg("Challenge request was redirected to different URL")
	}

	// Extract the challenge header.
	wwwAuthHeader := response.Header.Get(challengeHeaderName)
	// Log endpoint in sanitized form (host only) to avoid leaking credentials.
	sanitizedEndpoint := endpoint
	if endpoint != "" {
		sanitizedEndpoint = "<redacted>"

		u, parseErr := url.Parse(endpoint)
		if parseErr == nil && u.Host != "" {
			sanitizedEndpoint = u.Host
		}
	}

	clog.Debug().
		Str("status", response.Status).
		Str("header", wwwAuthHeader).
		Str("mirrors", sanitizedEndpoint).
		Msg("Received challenge response")

	parsedResponse := challengeResponse{
		statusCode:    response.StatusCode,
		wwwAuthHeader: wwwAuthHeader,
		redirected:    redirected,
		redirectHost:  redirectHost,
	}

	// Route the challenge response to the appropriate auth handler.
	return processChallengeResponse(
		&clog,
		ctx,
		container,
		registryAuth,
		client,
		settings,
		redirected,
		redirectHost,
		challengeURL.Host,
		challengeURL.Scheme,
		redirectScheme,
		parsedResponse,
	)
}
