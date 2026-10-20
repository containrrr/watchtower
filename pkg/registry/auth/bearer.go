package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/distribution/reference"
	"github.com/maypok86/otter/v2"
	"github.com/maypok86/otter/v2/stats"
	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/hosts"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Errors for authentication operations.
var (
	errFailedCreateBearerRequest     = errors.New("failed to create bearer token request")
	errFailedConstructBearerAuthURL  = errors.New("failed to construct bearer auth url")
	errFailedExecuteBearerRequest    = errors.New("failed to execute bearer token request")
	errFailedUnmarshalBearerResponse = errors.New("failed to unmarshal bearer token response")
	errFailedDecodeResponse          = errors.New("failed to decode response")
	errFailedParseImageName          = errors.New("failed to parse image name")
	errInvalidRealmURL               = errors.New("invalid realm URL in challenge header")
)

// Default token TTL when the registry does not provide expires_in.
const defaultTokenTTL = 60 * time.Second

// Cache configuration constants.
const (
	tokenCacheMaximumSize     = 100_000
	tokenCacheInitialCapacity = 16
)

// Package-level token cache variables.
var (
	tokenCache     *otter.Cache[string, tokenCacheEntry]
	tokenCacheOnce sync.Once
)

// tokenCacheEntry stores a bearer token and its absolute expiry time.
type tokenCacheEntry struct {
	token     string
	expiresAt time.Time
}

// tokenExpiryCalculator computes per-entry TTL based on the token's expires_in field.
type tokenExpiryCalculator struct{}

// ExpireAfterCreate returns the duration until the token expires.
func (e *tokenExpiryCalculator) ExpireAfterCreate(entry otter.Entry[string, tokenCacheEntry]) time.Duration {
	return time.Until(entry.Value.expiresAt)
}

// ExpireAfterUpdate returns the duration until the updated token expires.
func (e *tokenExpiryCalculator) ExpireAfterUpdate(entry otter.Entry[string, tokenCacheEntry], _ tokenCacheEntry) time.Duration {
	return time.Until(entry.Value.expiresAt)
}

// ExpireAfterRead preserves the existing expiry on reads.
func (e *tokenExpiryCalculator) ExpireAfterRead(entry otter.Entry[string, tokenCacheEntry]) time.Duration {
	return entry.ExpiresAfter()
}

// initTokenCache initializes the bearer token cache with Otter.
func initTokenCache(log *zerolog.Logger) {
	tokenCacheOnce.Do(func() {
		counter := stats.NewCounter()

		cache, err := otter.New(
			&otter.Options[string, tokenCacheEntry]{
				MaximumSize:      tokenCacheMaximumSize,
				InitialCapacity:  tokenCacheInitialCapacity,
				ExpiryCalculator: &tokenExpiryCalculator{},
				StatsRecorder:    counter,
			},
		)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize token cache: %v", err))
		}

		tokenCache = cache

		log.Debug().Msg("Initialized bearer token cache")
	})
}

// getBearerToken fetches a bearer token based on challenge instructions.
//
// It parses the challenge header, constructs the auth URL, and retrieves a bearer token.
// The token is cached to avoid redundant HTTP requests for the same challenge.
// Anonymous GHCR tokens are reused across public images because GHCR does not
// bind those tokens to the requested repository scope.
//
// Parameters:
//   - ctx: Context for request lifecycle control, enabling cancellation or timeouts.
//   - challenge: Challenge string from the registry's WWW-Authenticate header.
//   - imageRef: Normalized image reference for scoping the token request.
//   - registryAuth: Base64-encoded auth string (e.g., "username:password").
//   - client: Client instance for executing HTTP requests.
//   - settings: Registry request settings.
//
// Returns:
//   - string: Bearer token header (e.g., "Bearer ...") if successful.
//   - error: Non-nil if the operation fails, nil on success.
func getBearerToken(log *zerolog.Logger,
	ctx context.Context,
	challenge string,
	imageRef reference.Named,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
) (string, error) {
	clog := log.With().Str("image", imageRef.Name()).Logger()
	clog.Debug().Msg("Fetching bearer token from challenge")

	// Construct the auth URL from the challenge details.
	authURL, err := getAuthURL(&clog,
		challenge,
		imageRef,
		registryAuth,
		settings,
	)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errFailedConstructBearerAuthURL, err)
	}

	token, err := executeBearerTokenRequest(&clog,
		ctx,
		authURL,
		imageRef.Name(),
		registryAuth,
		client,
		settings,
	)
	if err != nil {
		return "", err
	}

	clog.Debug().
		Bool("token_present", token != "").
		Msg("Fetched bearer token")

	return token, nil
}

// executeBearerTokenRequest retrieves a bearer token, using the cache when possible.
//
// It checks the token cache for a valid, unexpired entry. On a cache miss or expiry,
// it executes the HTTP request, parses the response, and populates the cache.
// Anonymous GHCR entries share a cache key that omits repository scope.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - authURL: Pre-parsed authentication URL.
//   - imageName: Image name for logging context.
//   - registryAuth: Base64-encoded auth string.
//   - client: Client for HTTP requests.
//   - settings: Registry request settings.
//
// Returns:
//   - string: Bearer token header (e.g., "Bearer ...").
//   - error: Non-nil if the operation fails, nil on success.
func executeBearerTokenRequest(log *zerolog.Logger,
	ctx context.Context,
	authURL *url.URL,
	imageName string,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
) (string, error) {
	initTokenCache(log)

	// Anonymous GHCR requests share a cache key that omits repository scope.
	cacheKeyStr := bearerCacheKey(authURL, registryAuth)

	// Attempt cache lookup.
	entry, ok := tokenCache.GetIfPresent(cacheKeyStr)
	if ok {
		if time.Now().Before(entry.expiresAt) {
			log.Debug().
				Str("image", imageName).
				Str("url", authURL.String()).
				Msg("Using cached bearer token")

			return entry.token, nil
		}

		log.Debug().
			Str("image", imageName).
			Str("url", authURL.String()).
			Msg("Cached bearer token expired - fetching new token")
	} else {
		log.Debug().
			Str("image", imageName).
			Str("url", authURL.String()).
			Msg("No cached bearer token - fetching new token")
	}

	// Cache miss or expired: fetch new token.
	token, expiresAt, err := performBearerTokenFetch(log,
		ctx,
		authURL,
		imageName,
		registryAuth,
		client,
		settings,
	)
	if err != nil {
		return "", err
	}

	fullToken := "Bearer " + token

	// Populate cache.
	tokenCache.SetIfAbsent(cacheKeyStr, tokenCacheEntry{
		token:     fullToken,
		expiresAt: expiresAt,
	})

	log.Debug().
		Str("image", imageName).
		Str("expires_at", expiresAt.Format(time.RFC3339)).
		Msg("Retrieved and cached bearer token")

	return fullToken, nil
}

// bearerCacheKey returns the cache key for a bearer token request.
//
// Anonymous GHCR tokens are reusable across public repositories, so the
// repository scope is omitted from the key. Authenticated GHCR tokens and
// tokens for other registries remain scoped to the full token URL.
//
// Parameters:
//   - authURL: Token endpoint URL, including service and scope query parameters.
//   - registryAuth: Base64-encoded credentials. Empty for anonymous pulls.
//
// Returns:
//   - string: Cache key used by the bearer token cache.
func bearerCacheKey(authURL *url.URL, registryAuth string) string {
	if sharedAnonymousGHCRToken(authURL, registryAuth) {
		shared := *authURL
		query := shared.Query()
		query.Del("scope")
		shared.RawQuery = query.Encode()

		return shared.String() + "|"
	}

	return authURL.String() + "|" + registryAuth
}

// sharedAnonymousGHCRToken reports whether authURL and registryAuth identify
// an anonymous GitHub Container Registry token that can be reused across
// public images.
//
// Parameters:
//   - authURL: Token endpoint URL.
//   - registryAuth: Base64-encoded credentials. Empty for anonymous pulls.
//
// Returns:
//   - bool: True when the token should use the shared anonymous GHCR cache key.
func sharedAnonymousGHCRToken(authURL *url.URL, registryAuth string) bool {
	return registryAuth == "" && hosts.IsGitHubRegistry(authURL.Hostname())
}

// anonymousGHCRTokenURL returns the GHCR token endpoint used for the shared
// anonymous cache key.
//
// Returns:
//   - *url.URL: https://ghcr.io/token?service=ghcr.io
func anonymousGHCRTokenURL() *url.URL {
	query := url.Values{}
	query.Set("service", hosts.GitHubRegistryDomain)

	return &url.URL{
		Scheme:   "https",
		Host:     hosts.GitHubRegistryDomain,
		Path:     "/token",
		RawQuery: query.Encode(),
	}
}

// lookupAnonymousGHCRToken returns a cached anonymous GHCR bearer token when
// one is present and unexpired.
//
// Parameters:
//   - log: Logger used to initialize the token cache on first use.
//
// Returns:
//   - string: Cached bearer token header (for example "Bearer ...").
//   - bool: True when a valid cached token was found.
func lookupAnonymousGHCRToken(log *zerolog.Logger) (string, bool) {
	initTokenCache(log)

	entry, ok := tokenCache.GetIfPresent(
		bearerCacheKey(anonymousGHCRTokenURL(), ""),
	)
	if !ok || !time.Now().Before(entry.expiresAt) {
		return "", false
	}

	return entry.token, true
}

// cachedAnonymousGHCRToken returns a TokenResult for a cached anonymous GHCR
// token when the request can skip the registry challenge.
//
// Credentials, registry mirrors, and non-GHCR hosts still require a live
// challenge. A cache miss or expired token also falls through.
//
// Parameters:
//   - log: Logger used for cache initialization.
//   - imageRef: Normalized image reference whose registry host is inspected.
//   - registryAuth: Base64-encoded credentials. Empty for anonymous pulls.
//   - endpoint: Optional registry mirror override. Empty uses the canonical host.
//
// Returns:
//   - TokenResult: Cached token and ghcr.io challenge host when skipping.
//   - bool: True when the caller may skip the /v2/ challenge.
func cachedAnonymousGHCRToken(
	log *zerolog.Logger,
	imageRef reference.Named,
	registryAuth string,
	endpoint string,
) (TokenResult, bool) {
	if endpoint != "" || registryAuth != "" {
		return TokenResult{}, false
	}

	if !hosts.IsGitHubRegistry(reference.Domain(imageRef)) {
		return TokenResult{}, false
	}

	token, ok := lookupAnonymousGHCRToken(log)
	if !ok {
		return TokenResult{}, false
	}

	return TokenResult{
		Token:         token,
		ChallengeHost: hosts.GitHubRegistryDomain,
	}, true
}

// performBearerTokenFetch executes the HTTP request to fetch a bearer token.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - authURL: URL for the bearer token request.
//   - imageName: Image name for logging context.
//   - registryAuth: Base64-encoded auth string.
//   - client: Client for HTTP requests.
//   - settings: Registry request settings.
//
// Returns:
//   - string: Bearer token value (without "Bearer " prefix).
//   - time.Time: Absolute expiry time for the token.
//   - error: Non-nil if the operation fails, nil on success.
func performBearerTokenFetch(log *zerolog.Logger,
	ctx context.Context,
	authURL *url.URL,
	imageName string,
	registryAuth string,
	client Client,
	settings registryConfig.Registry,
) (string, time.Time, error) {
	r, err := newBearerRequest(log, ctx, authURL, imageName)
	if err != nil {
		return "", time.Time{}, err
	}

	addBasicAuth(log, r, imageName, registryAuth, settings)
	log.Debug().
		Str("url", r.URL.String()).
		Msg("Sending bearer token request")

	authResponse, err := client.Do(r)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image", imageName).
			Str("url", authURL.String()).
			Msg("Failed to execute bearer token request")

		return "", time.Time{}, fmt.Errorf("%w: %w", errFailedExecuteBearerRequest, err)
	}
	defer authResponse.Body.Close()

	// Reject non-success responses before reading the body to avoid
	// attempting to parse error pages as bearer token JSON.
	if authResponse.StatusCode == http.StatusTooManyRequests {
		body := ratelimit.ReadBody(authResponse, ratelimit.DefaultBodyLimit)

		return "", time.Time{}, ratelimit.FromResponse(authResponse, body)
	}

	if authResponse.StatusCode < http.StatusOK || authResponse.StatusCode >= http.StatusMultipleChoices {
		return "", time.Time{}, fmt.Errorf(
			"%w: status %s",
			errFailedExecuteBearerRequest,
			authResponse.Status,
		)
	}

	token, expiresAt, err := readBearerTokenWithExpiry(log,
		authResponse.Body,
		imageName,
	)
	if err != nil {
		return "", time.Time{}, err
	}

	if token == "" {
		log.Debug().
			Str("image", imageName).
			Msg("Empty bearer token received")

		return "", time.Time{}, fmt.Errorf("%w: empty token in response", errFailedDecodeResponse)
	}

	return token, expiresAt, nil
}

// newBearerRequest creates an HTTP request for fetching a bearer token.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - authURL: URL for the bearer token request.
//   - imageName: Image name for logging context.
//
// Returns:
//   - *http.Request: Constructed request if successful.
//   - error: Non-nil if creation fails, nil on success.
func newBearerRequest(log *zerolog.Logger, ctx context.Context, authURL *url.URL, imageName string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		authURL.String(),
		nil,
	)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image", imageName).
			Str("url", authURL.String()).
			Msg("Failed to create bearer token request")

		return nil, fmt.Errorf("%w: %w", errFailedCreateBearerRequest, err)
	}

	return request, nil
}

// addBasicAuth adds Basic authentication to the request if credentials are provided.
//
// It only attaches credentials for HTTPS requests or when the insecure-registry
// opt-in is explicitly enabled to prevent plaintext credential transmission.
//
// Parameters:
//   - request: HTTP request to modify.
//   - imageName: Image name for logging context.
//   - registryAuth: Base64-encoded auth string.
//   - settings: Registry request settings.
func addBasicAuth(
	log *zerolog.Logger,
	request *http.Request,
	imageName, registryAuth string,
	settings registryConfig.Registry,
) {
	if registryAuth == "" {
		log.Debug().
			Str("image", imageName).
			Msg("No credentials found")

		return
	}

	if request.URL.Scheme != "https" && !settings.TLSSkip {
		log.Debug().
			Str("image", imageName).
			Str("scheme", request.URL.Scheme).
			Str("host", request.URL.Host).
			Msg("Skipping Basic auth for non-HTTPS request without TLS skip opt-in")

		return
	}

	log.Debug().
		Str("image", imageName).
		Msg("Found credentials")

	if log.GetLevel() == zerolog.TraceLevel {
		log.Trace().
			Str("image", imageName).
			Msg("Using credentials")
	}

	request.Header.Add("Authorization", "Basic "+registryAuth)
}

// readBearerTokenWithExpiry reads and parses a bearer token with its expiry time.
//
// Parameters:
//   - body: Response body containing JSON token response.
//   - imageName: Image name for logging context.
//
// Returns:
//   - string: Bearer token value.
//   - time.Time: Absolute expiry time for the token.
//   - error: Non-nil if parsing fails, nil on success.
func readBearerTokenWithExpiry(log *zerolog.Logger, body io.Reader, imageName string) (string, time.Time, error) {
	const maxBearerTokenResponseSize = 1 << 20

	limitedBody := io.LimitReader(body, maxBearerTokenResponseSize+1)

	b, err := io.ReadAll(limitedBody)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image", imageName).
			Msg("Failed to read bearer token response body")

		return "", time.Time{}, fmt.Errorf("%w: %w", errFailedDecodeResponse, err)
	}

	if len(b) > maxBearerTokenResponseSize {
		return "", time.Time{}, fmt.Errorf("%w: bearer token response exceeds %d bytes", errFailedDecodeResponse, maxBearerTokenResponseSize)
	}

	tokenResponse := &types.TokenResponse{}

	err = json.Unmarshal(b, tokenResponse)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image", imageName).
			Msg("Failed to unmarshal bearer token response")

		return "", time.Time{}, fmt.Errorf("%w: %w", errFailedUnmarshalBearerResponse, err)
	}

	expiresAt := computeTokenExpiry(log, tokenResponse)
	effectiveTTL := int64(time.Until(expiresAt).Round(time.Second).Seconds())
	log.Debug().
		Str("image", imageName).
		Int("expires_in", tokenResponse.ExpiresIn).
		Str("issued_at", tokenResponse.IssuedAt).
		Int64("effective_ttl_secs", effectiveTTL).
		Str("expires_at", expiresAt.Format(time.RFC3339)).
		Msg("Parsed bearer token expiry")

	token := tokenResponse.Token
	if token == "" {
		token = tokenResponse.AccessToken
	}

	return token, expiresAt, nil
}

// computeTokenExpiry calculates the absolute expiry time from a token response.
//
// It prefers the registry-provided expires_in combined with issued_at when both
// are available. It falls back to current time when issued_at is absent or invalid,
// and finally defaults to defaultTokenTTL.
//
// Parameters:
//   - tokenResponse: Parsed token response from the registry.
//
// Returns:
//   - time.Time: The absolute expiry time for the token.
func computeTokenExpiry(log *zerolog.Logger, tokenResponse *types.TokenResponse) time.Time {
	now := time.Now()

	if tokenResponse.IssuedAt != "" {
		issuedAt, parseErr := time.Parse(
			time.RFC3339,
			tokenResponse.IssuedAt,
		)
		if parseErr == nil {
			if tokenResponse.ExpiresIn > 0 {
				return issuedAt.Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
			}

			return issuedAt.Add(defaultTokenTTL)
		}

		log.Debug().
			Err(parseErr).
			Str("issued_at", tokenResponse.IssuedAt).
			Msg("Failed to parse issued_at - using current time as base")
	}

	if tokenResponse.ExpiresIn > 0 {
		return now.Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	}

	return now.Add(defaultTokenTTL)
}

// resolveService resolves the service identifier from challenge values,
// deriving it from the realm host when omitted.
//
// Parameters:
//   - log: Contextual logger.
//   - values: Parsed challenge values.
//   - imageName: Image name for logging context.
//
// Returns:
//   - string: The resolved service identifier.
func resolveService(log *zerolog.Logger, values challengeValues, imageName string) string {
	if values.service != "" || values.realm == "" {
		return values.service
	}

	service := extractChallengeHost(log, values.realm)
	if service != "" {
		log.Debug().
			Str("image", imageName).
			Str("service", service).
			Msg("Derived challenge service from realm")
	}

	return service
}

// validateRequiredChallengeValues ensures realm and service are both present.
//
// Parameters:
//   - values: Parsed challenge values.
//   - imageName: Image name for logging context.
//   - challenge: Raw challenge string for error messages.
//
// Returns:
//   - error: Non-nil if either realm or service is missing.
func validateRequiredChallengeValues(log *zerolog.Logger, values challengeValues, imageName, challenge string) error {
	if values.realm != "" && values.service != "" {
		return nil
	}

	log.Warn().
		Str("image", imageName).
		Msg("Missing required challenge header values: realm or service")

	return fmt.Errorf("%w: missing realm or service in header: %s", errInvalidChallengeHeader, challenge)
}

// buildAuthQuery constructs the query parameters for the auth URL.
//
// It always derives the token scope from the image reference rather than
// using the scope from the WWW-Authenticate challenge. Some registries
// return a placeholder scope in the challenge that their backing token
// service rejects with 403, so the scope is built as:
//
//	repository:<image-path>:pull
//
// Parameters:
//   - authURL: The base auth URL.
//   - values: Parsed challenge values.
//   - imageRef: Normalized image reference.
//
// Returns:
//   - *url.URL: The auth URL with query parameters applied.
func buildAuthQuery(log *zerolog.Logger, authURL *url.URL, values challengeValues, imageRef reference.Named) *url.URL {
	query := authURL.Query()
	query.Set("service", values.service)

	scopeImage := reference.Path(imageRef)

	scopeRepositoryPrefix := "repository:"
	scopeActionPull := "pull"
	scope := scopeRepositoryPrefix + scopeImage + ":" + scopeActionPull

	log.Debug().
		Str("image", imageRef.Name()).
		Str("scope", scope).
		Msg("Set auth token scope")

	query.Set("scope", scope)
	authURL.RawQuery = query.Encode()

	return authURL
}

// getAuthURL constructs an authentication URL from challenge instructions.
//
// Parameters:
//   - challenge: Challenge string from the registry.
//   - imageRef: Normalized image reference.
//   - registryAuth: Base64-encoded auth string, empty if not provided.
//   - settings: Registry request settings.
//
// Returns:
//   - *url.URL: Constructed auth URL if successful.
//   - error: Non-nil if parsing fails, nil on success.
func getAuthURL(
	log *zerolog.Logger,
	challenge string,
	imageRef reference.Named,
	registryAuth string,
	settings registryConfig.Registry,
) (*url.URL, error) {
	values := parseChallenge(challenge)

	values.service = resolveService(log, values, imageRef.Name())

	log.Debug().
		Str("image", imageRef.Name()).
		Str("realm", values.realm).
		Str("service", values.service).
		Str("scope", values.scope).
		Msg("Parsed challenge header")

	err := validateRequiredChallengeValues(log,
		values,
		imageRef.Name(),
		challenge,
	)
	if err != nil {
		return nil, err
	}

	// Parse the realm into a URL.
	authURL, err := url.Parse(values.realm)
	if err != nil || authURL == nil {
		if err != nil {
			log.Debug().
				Err(err).
				Str("image", imageRef.Name()).
				Str("realm", values.realm).
				Msg("Failed to parse realm URL")
		} else {
			log.Debug().
				Str("image", imageRef.Name()).
				Str("realm", values.realm).
				Msg("Invalid realm URL (nil after parsing)")
		}

		return nil, fmt.Errorf("%w: %s", errInvalidRealmURL, values.realm)
	}

	// Reject realms without a host name or with a non-HTTP(S) scheme. A realm
	// with only a port names no server, and its token request would go to the
	// local machine.
	if authURL.Hostname() == "" || (authURL.Scheme != "http" && authURL.Scheme != "https") {
		log.Debug().
			Str("image", imageRef.Name()).
			Str("realm", values.realm).
			Str("scheme", authURL.Scheme).
			Str("host", authURL.Host).
			Msg("Invalid realm URL (missing host or unsupported scheme)")

		return nil, fmt.Errorf("%w: %s", errInvalidRealmURL, values.realm)
	}

	// Reject HTTP realms when Basic authentication is attached unless the
	// insecure-registry opt-in explicitly allows plaintext token endpoints.
	if authURL.Scheme == "http" && registryAuth != "" && !settings.TLSSkip {
		log.Debug().
			Str("image", imageRef.Name()).
			Str("realm", values.realm).
			Str("scheme", authURL.Scheme).
			Msg("Invalid realm URL (HTTP realm with Basic auth requires TLS skip opt-in)")

		return nil, fmt.Errorf("%w: %s", errInvalidRealmURL, values.realm)
	}

	authURL = buildAuthQuery(log, authURL, values, imageRef)

	return authURL, nil
}
