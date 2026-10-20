package auth

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/maypok86/otter/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// testAnonymousGHCRToken is the shared anonymous GHCR token that
// seedAnonymousGHCRToken caches.
const testAnonymousGHCRToken = "Bearer cached-ghcr-token"

// mustParseURL parses rawURL and panics if it is not a URL, for fixed URLs in
// test tables.
//
// Parameters:
//   - rawURL: URL to parse.
//
// Returns:
//   - *url.URL: Parsed URL.
func mustParseURL(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	return parsed
}

// mustParseRef parses a normalized image reference and fails the test if it
// does not parse.
//
// Parameters:
//   - t: Test that receives the failure.
//   - image: Image reference to parse.
//
// Returns:
//   - reference.Named: Parsed reference.
func mustParseRef(t *testing.T, image string) reference.Named {
	t.Helper()

	ref, err := reference.ParseNormalizedNamed(image)
	require.NoError(t, err)

	return ref
}

// resetTokenCache empties the shared token cache, so a test starts without
// tokens cached by earlier tests.
//
// Parameters:
//   - t: Test whose logger initializes the cache.
func resetTokenCache(t *testing.T) {
	t.Helper()
	initTokenCache(newTestLogger(t))
	tokenCache.InvalidateAll()
}

// seedAnonymousGHCRToken empties the shared token cache and caches
// testAnonymousGHCRToken as the unexpired shared anonymous GHCR token.
//
// Parameters:
//   - t: Test whose logger initializes the cache.
func seedAnonymousGHCRToken(t *testing.T) {
	t.Helper()
	resetTokenCache(t)

	authURL := mustParseURL("https://ghcr.io/token?service=ghcr.io")
	tokenCache.SetIfAbsent(bearerCacheKey(authURL, ""), tokenCacheEntry{
		token:     testAnonymousGHCRToken,
		expiresAt: time.Now().Add(time.Hour),
	})
}

// Test_tokenExpiryCalculator_ExpireAfterCreate verifies that a new cache entry
// lives until its token expires.
func Test_tokenExpiryCalculator_ExpireAfterCreate(t *testing.T) {
	t.Parallel()

	calc := &tokenExpiryCalculator{}
	entry := otter.Entry[string, tokenCacheEntry]{
		Key:   "test",
		Value: tokenCacheEntry{expiresAt: time.Now().Add(5 * time.Minute)},
	}

	got := calc.ExpireAfterCreate(entry)
	assert.InDelta(t, 5*time.Minute.Seconds(), got.Seconds(), 1)
}

// Test_tokenExpiryCalculator_ExpireAfterUpdate verifies that an updated cache
// entry lives until its token expires.
func Test_tokenExpiryCalculator_ExpireAfterUpdate(t *testing.T) {
	t.Parallel()

	calc := &tokenExpiryCalculator{}
	entry := otter.Entry[string, tokenCacheEntry]{
		Key:   "test",
		Value: tokenCacheEntry{expiresAt: time.Now().Add(10 * time.Minute)},
	}

	got := calc.ExpireAfterUpdate(entry, tokenCacheEntry{})
	assert.InDelta(t, 10*time.Minute.Seconds(), got.Seconds(), 1)
}

// Test_tokenExpiryCalculator_ExpireAfterRead verifies that reading a cache
// entry keeps its remaining lifetime.
func Test_tokenExpiryCalculator_ExpireAfterRead(t *testing.T) {
	t.Parallel()

	calc := &tokenExpiryCalculator{}
	entry := otter.Entry[string, tokenCacheEntry]{
		Key:               "test",
		ExpiresAtNano:     time.Now().Add(3 * time.Minute).UnixNano(),
		RefreshableAtNano: time.Now().Add(24 * time.Hour).UnixNano(),
		SnapshotAtNano:    time.Now().UnixNano(),
	}

	got := calc.ExpireAfterRead(entry)
	assert.Equal(t, entry.ExpiresAfter(), got)
}

// Test_initTokenCache verifies that the token cache can be initialized more
// than once.
func Test_initTokenCache(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		initTokenCache(newTestLogger(t))
		initTokenCache(newTestLogger(t))
	})
	assert.NotNil(t, tokenCache)
}

// Test_computeTokenExpiry verifies that a token expires expires_in seconds
// after issued_at, falling back to the current time for a missing or invalid
// issued_at and to the default TTL for a missing or negative expires_in.
func Test_computeTokenExpiry(t *testing.T) {
	t.Parallel()

	now := time.Now()
	truncatedToSeconds := now.Truncate(time.Second)

	tests := []struct {
		name string
		args *types.TokenResponse
		want time.Time
	}{
		{
			name: "uses expires_in when provided",
			args: &types.TokenResponse{ExpiresIn: 3600},
			want: truncatedToSeconds.Add(3600 * time.Second),
		},
		{
			name: "falls back to default TTL when expires_in is negative",
			args: &types.TokenResponse{ExpiresIn: -1},
			want: truncatedToSeconds.Add(defaultTokenTTL),
		},
		{
			name: "uses issued_at with default TTL when expires_in is zero",
			args: &types.TokenResponse{IssuedAt: now.Add(-5 * time.Minute).Truncate(time.Second).Format(time.RFC3339)},
			want: now.Add(-5 * time.Minute).Truncate(time.Second).Add(defaultTokenTTL),
		},
		{
			name: "uses issued_at with expires_in when both are provided",
			args: &types.TokenResponse{
				ExpiresIn: 3600,
				IssuedAt:  now.Add(-10 * time.Minute).Truncate(time.Second).Format(time.RFC3339),
			},
			want: now.Add(-10 * time.Minute).Truncate(time.Second).Add(3600 * time.Second),
		},
		{
			name: "falls back to default TTL when both are missing",
			args: &types.TokenResponse{},
			want: truncatedToSeconds.Add(defaultTokenTTL),
		},
		{
			name: "falls back to default TTL when issued_at is invalid",
			args: &types.TokenResponse{IssuedAt: "not-a-date"},
			want: truncatedToSeconds.Add(defaultTokenTTL),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := computeTokenExpiry(newTestLogger(t), tt.args)
			assert.InDelta(t, tt.want.Unix(), got.Unix(), 2)
		})
	}
}

// Test_readBearerTokenWithExpiry verifies that the token is read from the token
// field, or from access_token when token is empty, and that a body that is not
// JSON is an error.
func Test_readBearerTokenWithExpiry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "valid token response", body: `{"token":"test-token","expires_in":3600,"issued_at":"2024-01-01T00:00:00Z"}`, want: "test-token"},
		{name: "invalid JSON returns error", body: `{invalid json}`, wantErr: true},
		{name: "empty token returns empty string", body: `{"token":""}`, want: ""},
		{name: "missing token field returns empty string", body: `{"expires_in":3600}`, want: ""},
		{name: "access_token used when token is empty", body: `{"access_token":"fallback-token","expires_in":3600}`, want: "fallback-token"},
		{name: "token takes precedence over access_token", body: `{"token":"primary-token","access_token":"fallback-token","expires_in":3600}`, want: "primary-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _, err := readBearerTokenWithExpiry(newTestLogger(t), strings.NewReader(tt.body), "test/image")
			if tt.wantErr {
				require.ErrorIs(t, err, errFailedUnmarshalBearerResponse)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_addBasicAuth verifies that credentials are sent with a token request
// over HTTPS, and over HTTP only when TLS verification is skipped.
func Test_addBasicAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		url          string
		registryAuth string
		tlsSkip      bool
		wantHeader   string
	}{
		{
			name:         "adds basic auth header for HTTPS when credentials provided",
			url:          "https://example.com",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			wantHeader:   "Basic dGVzdHVzZXI6dGVzdHBhc3M=",
		},
		{
			name:       "does not add header when no credentials",
			url:        "https://example.com",
			wantHeader: "",
		},
		{
			name:         "does not add header for HTTP without TLS skip",
			url:          "http://example.com",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			wantHeader:   "",
		},
		{
			name:         "adds basic auth header for HTTP with TLS skip",
			url:          "http://example.com",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			tlsSkip:      true,
			wantHeader:   "Basic dGVzdHVzZXI6dGVzdHBhc3M=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := mustNewRequest(t, tt.url)

			addBasicAuth(newTestLogger(t), request, "test/image", tt.registryAuth, registryConfig.Registry{TLSSkip: tt.tlsSkip})
			assert.Equal(t, tt.wantHeader, request.Header.Get("Authorization"))
		})
	}
}

// Test_newBearerRequest verifies that a token request is a GET for the auth URL,
// and that a request without a context is an error.
func Test_newBearerRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// nilContext builds the request without a context.
		nilContext bool
		wantErr    bool
	}{
		{name: "creates GET request with context"},
		{name: "nil context returns error", nilContext: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			if tt.nilContext {
				ctx = nil
			}

			got, err := newBearerRequest(newTestLogger(t), ctx, mustParseURL("https://example.com/token"), "test/image")
			if tt.wantErr {
				require.ErrorIs(t, err, errFailedCreateBearerRequest)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, got.Method)
			assert.Equal(t, "https://example.com/token", got.URL.String())
		})
	}
}

// Test_resolveService verifies that the token service is the challenge's
// service, or the realm's host when the challenge names none.
func Test_resolveService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values challengeValues
		want   string
	}{
		{name: "returns service when present", values: challengeValues{service: "ghcr.io"}, want: "ghcr.io"},
		{name: "derives service from realm when service is empty", values: challengeValues{realm: "https://ghcr.io/token"}, want: "ghcr.io"},
		{name: "returns empty when both service and realm are empty", values: challengeValues{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, resolveService(newTestLogger(t), tt.values, "test/image"))
		})
	}
}

// Test_validateRequiredChallengeValues verifies that a challenge needs both a
// realm and a service.
func Test_validateRequiredChallengeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  challengeValues
		wantErr bool
	}{
		{name: "valid realm and service returns no error", values: challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io"}},
		{name: "missing realm returns error", values: challengeValues{service: "ghcr.io"}, wantErr: true},
		{name: "missing service returns error", values: challengeValues{realm: "https://ghcr.io/token"}, wantErr: true},
		{name: "missing both returns error", values: challengeValues{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRequiredChallengeValues(newTestLogger(t), tt.values, "test/image", "bearer")
			if tt.wantErr {
				require.ErrorIs(t, err, errInvalidChallengeHeader)

				return
			}

			require.NoError(t, err)
		})
	}
}

// Test_buildAuthQuery verifies that the token query names the service and a
// pull scope for the image's repository path, replacing any service and scope
// already in the realm, with Docker Hub references scoped to their library/
// path and without their vanity host.
func Test_buildAuthQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		authURL string
		image   string
		want    string
	}{
		{
			name:    "adds service and scope query params",
			authURL: "https://ghcr.io/token",
			image:   "ghcr.io/user/repo:latest",
			want:    "scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
		{
			name:    "replaces existing service and scope query params",
			authURL: "https://ghcr.io/token?service=old&scope=old-scope",
			image:   "ghcr.io/user/repo:latest",
			want:    "scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
		{
			name:    "official Docker Hub image is scoped to library",
			authURL: "https://ghcr.io/token",
			image:   "registry",
			want:    "scope=repository%3Alibrary%2Fregistry%3Apull&service=ghcr.io",
		},
		{
			name:    "official Docker Hub image with docker.io is scoped to library",
			authURL: "https://ghcr.io/token",
			image:   "docker.io/registry",
			want:    "scope=repository%3Alibrary%2Fregistry%3Apull&service=ghcr.io",
		},
		{
			name:    "docker.io vanity host is left out of the scope",
			authURL: "https://ghcr.io/token",
			image:   "docker.io/nickfedor/watchtower",
			want:    "scope=repository%3Anickfedor%2Fwatchtower%3Apull&service=ghcr.io",
		},
		{
			name:    "index.docker.io vanity host is left out of the scope",
			authURL: "https://ghcr.io/token",
			image:   "index.docker.io/nickfedor/watchtower",
			want:    "scope=repository%3Anickfedor%2Fwatchtower%3Apull&service=ghcr.io",
		},
		{
			name:    "single-segment path on another registry",
			authURL: "https://ghcr.io/token",
			image:   "ghcr.io/watchtower",
			want:    "scope=repository%3Awatchtower%3Apull&service=ghcr.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildAuthQuery(newTestLogger(t), mustParseURL(tt.authURL), challengeValues{service: "ghcr.io"}, mustParseRef(t, tt.image))
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.RawQuery)
		})
	}
}

// TestGetAuthURL verifies the token URL built from a bearer challenge: the
// realm with the service, from the challenge or the realm's host, and the
// image's pull scope, and that a challenge without a realm, or with a realm
// that is not an HTTP or HTTPS URL with a host, is an error, as is an HTTP
// realm with credentials unless TLS verification is skipped.
func TestGetAuthURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		challenge    string
		image        string
		registryAuth string
		want         string
		wantErr      error
	}{
		{
			name:      "valid challenge constructs auth URL",
			challenge: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/repo:pull"`,
			image:     "ghcr.io/user/repo:latest",
			want:      "https://ghcr.io/token?scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
		{
			name:      "scope comes from the image, not the challenge",
			challenge: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`,
			image:     "nicholas-fedor/watchtower",
			want:      "https://ghcr.io/token?scope=repository%3Anicholas-fedor%2Fwatchtower%3Apull&service=ghcr.io",
		},
		{
			name:      "service derived from realm when challenge omits service",
			challenge: `bearer realm="https://registry.example.com/token"`,
			image:     "registry.example.com/test/image:latest",
			want:      "https://registry.example.com/token?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com",
		},
		{
			name:      "service derived from realm when service is empty",
			challenge: `bearer realm="https://registry.example.com/token",service=""`,
			image:     "registry.example.com/test/image:latest",
			want:      "https://registry.example.com/token?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com",
		},
		{
			name:      "service derived from realm keeps the realm's port",
			challenge: `bearer realm="http://localhost:5000/token"`,
			image:     "localhost:5000/test/image:latest",
			want:      "http://localhost:5000/token?scope=repository%3Atest%2Fimage%3Apull&service=localhost%3A5000",
		},
		{
			name:      "realm with a trailing slash",
			challenge: `bearer realm="https://registry.example.com/token/"`,
			image:     "registry.example.com/test/image:latest",
			want:      "https://registry.example.com/token/?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com",
		},
		{
			name:      "trailing empty field is ignored",
			challenge: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",`,
			image:     "nicholas-fedor/watchtower",
			want:      "https://ghcr.io/token?scope=repository%3Anicholas-fedor%2Fwatchtower%3Apull&service=ghcr.io",
		},
		{
			name:      "field without a value is ignored",
			challenge: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",valuelesskey`,
			image:     "nicholas-fedor/watchtower",
			want:      "https://ghcr.io/token?scope=repository%3Anicholas-fedor%2Fwatchtower%3Apull&service=ghcr.io",
		},
		{
			name:      "http realm without registry auth is allowed",
			challenge: `bearer realm="http://insecure-registry.local/token",service="insecure-registry.local"`,
			image:     "ghcr.io/user/repo:latest",
			want:      "http://insecure-registry.local/token?scope=repository%3Auser%2Frepo%3Apull&service=insecure-registry.local",
		},
		{
			name:      "missing realm returns error",
			challenge: `bearer service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidChallengeHeader,
		},
		{
			name:      "realm without scheme returns error",
			challenge: `bearer realm="example.com/token",service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidRealmURL,
		},
		{
			name:      "realm without scheme and service returns error",
			challenge: `bearer realm="registry.example.com/token"`,
			image:     "registry.example.com/test/image:latest",
			wantErr:   errInvalidChallengeHeader,
		},
		{
			name:      "realm with non-HTTP(S) scheme returns error",
			challenge: `bearer realm="ftp://example.com/token",service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidRealmURL,
		},
		{
			name:      "realm with empty host returns error",
			challenge: `bearer realm="http:///token",service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidRealmURL,
		},
		{
			name:      "realm with a port but no host name returns error",
			challenge: `bearer realm="https://:5000/token",service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidRealmURL,
		},
		{
			name:      "realm with an empty authority returns error",
			challenge: `bearer realm="http://:",service="ghcr.io"`,
			image:     "ghcr.io/user/repo:latest",
			wantErr:   errInvalidRealmURL,
		},
		{
			name:      "realm with an IPv6 host is allowed",
			challenge: `bearer realm="https://[::1]:5000/token",service="registry"`,
			image:     "ghcr.io/user/repo:latest",
			want:      "https://[::1]:5000/token?scope=repository%3Auser%2Frepo%3Apull&service=registry",
		},
		{
			name:         "http realm with registry auth is rejected without TLS skip",
			challenge:    `bearer realm="http://insecure-registry.local/token",service="insecure-registry.local"`,
			image:        "ghcr.io/user/repo:latest",
			registryAuth: "dGVzdA==",
			wantErr:      errInvalidRealmURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getAuthURL(newTestLogger(t), tt.challenge, mustParseRef(t, tt.image), tt.registryAuth, registryConfig.Registry{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

// TestGetBearerToken verifies that a bearer token is fetched from the
// challenge's realm, and that a failed request or a response that is not JSON
// is an error.
//
// The test resets the shared token cache, so it does not run in parallel.
func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		challenge string
		response  *http.Response
		doErr     error
		want      string
		wantErr   error
	}{
		{
			name:      "successful token fetch",
			challenge: `bearer realm="https://test.com/token",service="test.com"`,
			response:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"test-token","expires_in":3600}`))},
			want:      "Bearer test-token",
		},
		{
			name:      "client Do returns error",
			challenge: `bearer realm="https://error.test.com/token",service="error.test.com"`,
			doErr:     assert.AnError,
			wantErr:   errFailedExecuteBearerRequest,
		},
		{
			name:      "invalid JSON response returns error",
			challenge: `bearer realm="https://invalid.test.com/token",service="invalid.test.com"`,
			response:  &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"invalid": "json"`))},
			wantErr:   errFailedUnmarshalBearerResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTokenCache(t)

			client := mockAuth.NewMockClient(t)
			client.EXPECT().Do(mock.Anything).Return(tt.response, tt.doErr).Once()

			got, err := getBearerToken(newTestLogger(t), t.Context(), tt.challenge, mustParseRef(t, "test/image:latest"), "", client, registryConfig.Registry{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetBearerToken_tokenServer verifies that a bearer token is fetched from a
// stub token server, with the credentials sent as Basic auth when there are
// any.
//
// The test resets the shared token cache, so it does not run in parallel.
func TestGetBearerToken_tokenServer(t *testing.T) {
	tests := []struct {
		name string
		// dockerAuth is the credential in the form Docker supplies it.
		dockerAuth string
		// wantAuthorization is the Authorization header the token server expects.
		wantAuthorization string
	}{
		{name: "without credentials"},
		{name: "with credentials", dockerAuth: testDockerAuth, wantAuthorization: "Basic " + testBasicAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTokenCache(t)

			server := httpstub.NewTLS(t)
			server.AppendHandlers(httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/")),
				func(_ http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tt.wantAuthorization, r.Header.Get("Authorization"))
				},
				httpstub.RespondWith(http.StatusOK, `{"token": "test-token"}`, http.Header{"Content-Type": {"application/json"}}),
			))

			challenge := `bearer realm="` + server.URL() + `",service="test-service",scope="repository:test/image:pull"`

			got, err := getBearerToken(newTestLogger(t), t.Context(), challenge, mustParseRef(t, "test/image"),
				TransformAuth(newTestLogger(t), tt.dockerAuth), server.Client(), registryConfig.Registry{})
			require.NoError(t, err)
			assert.Equal(t, "Bearer test-token", got)
			assert.Len(t, server.ReceivedRequests(), 1)
		})
	}
}

// Test_executeBearerTokenRequest_cacheMiss verifies that a token that is not
// cached is fetched.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_executeBearerTokenRequest_cacheMiss(t *testing.T) {
	resetTokenCache(t)

	client := mockAuth.NewMockClient(t)
	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"cached-token","expires_in":3600}`))}, nil).Once()

	got, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), mustParseURL("https://test.com/token?service=test.com"), "test/image", "", client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer cached-token", got)
}

// Test_executeBearerTokenRequest_cacheHit verifies that a cached, unexpired
// token is returned without a request.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_executeBearerTokenRequest_cacheHit(t *testing.T) {
	resetTokenCache(t)

	authURL := mustParseURL("https://test.com/token?service=test.com")
	tokenCache.SetIfAbsent(authURL.String()+"|", tokenCacheEntry{
		token:     "Bearer cached-token",
		expiresAt: time.Now().Add(time.Hour),
	})

	got, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), authURL, "test/image", "", mockAuth.NewMockClient(t), registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer cached-token", got)
}

// Test_bearerCacheKey verifies that anonymous GHCR tokens, including those for
// lscr.io images, share a key without the repository scope, while
// authenticated GHCR tokens and other registries' tokens keep it.
func Test_bearerCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rawURL       string
		registryAuth string
		want         string
	}{
		{
			name:   "anonymous GHCR drops repository scope",
			rawURL: "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io",
			want:   "https://ghcr.io/token?service=ghcr.io|",
		},
		{
			name:   "anonymous GHCR shares key across images",
			rawURL: "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io",
			want:   "https://ghcr.io/token?service=ghcr.io|",
		},
		{
			name:   "anonymous lscr.io token host shares with GHCR mapping",
			rawURL: "https://lscr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io",
			want:   "https://lscr.io/token?service=ghcr.io|",
		},
		{
			name:         "authenticated GHCR keeps per-image scope",
			rawURL:       "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io",
			registryAuth: testBasicAuth,
			want:         "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io|" + testBasicAuth,
		},
		{
			name:   "anonymous Docker Hub keeps per-image scope",
			rawURL: "https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io",
			want:   "https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, bearerCacheKey(mustParseURL(tt.rawURL), tt.registryAuth))
		})
	}
}

// Test_executeBearerTokenRequest_anonymousGHCRSharesToken verifies that an
// anonymous GHCR token fetched for one repository is reused for another.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_executeBearerTokenRequest_anonymousGHCRSharesToken(t *testing.T) {
	resetTokenCache(t)

	client := mockAuth.NewMockClient(t)
	prowlarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io")
	sonarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io")

	client.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.Host == "ghcr.io" &&
			req.URL.Query().Get("scope") == "repository:linuxserver/prowlarr:pull"
	})).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"shared-anon-token","expires_in":3600}`))}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), prowlarr, "ghcr.io/linuxserver/prowlarr", "", client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer shared-anon-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), sonarr, "ghcr.io/linuxserver/sonarr", "", client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer shared-anon-token", second)
}

// Test_executeBearerTokenRequest_authenticatedGHCRDoesNotShare verifies that an
// authenticated GHCR token is fetched for each repository.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_executeBearerTokenRequest_authenticatedGHCRDoesNotShare(t *testing.T) {
	resetTokenCache(t)

	client := mockAuth.NewMockClient(t)
	prowlarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io")
	sonarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io")

	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":3600}`))}, nil).Once()
	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":3600}`))}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), prowlarr, "ghcr.io/linuxserver/prowlarr", testBasicAuth, client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer scoped-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), sonarr, "ghcr.io/linuxserver/sonarr", testBasicAuth, client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer scoped-token", second)
}

// Test_executeBearerTokenRequest_anonymousNonGHCRDoesNotShare verifies that an
// anonymous token from another registry is fetched for each repository.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_executeBearerTokenRequest_anonymousNonGHCRDoesNotShare(t *testing.T) {
	resetTokenCache(t)

	client := mockAuth.NewMockClient(t)
	alpine := mustParseURL("https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io")
	nginx := mustParseURL("https://auth.docker.io/token?scope=repository%3Alibrary%2Fnginx%3Apull&service=registry.docker.io")

	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"hub-token","expires_in":3600}`))}, nil).Once()
	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"hub-token","expires_in":3600}`))}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), alpine, "library/alpine", "", client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer hub-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), t.Context(), nginx, "library/nginx", "", client, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer hub-token", second)
}

// TestGetBearerToken_anonymousGHCRSharesAcrossImages verifies that one
// anonymous GHCR token serves every GHCR repository.
//
// The test resets the shared token cache, so it does not run in parallel.
func TestGetBearerToken_anonymousGHCRSharesAcrossImages(t *testing.T) {
	resetTokenCache(t)

	client := mockAuth.NewMockClient(t)
	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"shared-anon-token","expires_in":3600}`))}, nil).Once()

	challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:linuxserver/prowlarr:pull"`

	for _, image := range []string{
		"ghcr.io/linuxserver/prowlarr:latest",
		"ghcr.io/linuxserver/sonarr:latest",
		"ghcr.io/nicholas-fedor/watchtower:latest",
	} {
		got, err := getBearerToken(newTestLogger(t), t.Context(), challenge, mustParseRef(t, image), "", client, registryConfig.Registry{})
		require.NoError(t, err)
		assert.Equal(t, "Bearer shared-anon-token", got)
	}
}

// Test_performBearerTokenFetch_sendsBasicAuth verifies that the credentials are
// sent as Basic auth with the token request.
func Test_performBearerTokenFetch_sendsBasicAuth(t *testing.T) {
	t.Parallel()

	client := mockAuth.NewMockClient(t)
	client.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req != nil && req.Header.Get("Authorization") == "Basic "+testBasicAuth
	})).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":60}`))}, nil).Once()

	token, _, err := performBearerTokenFetch(
		newTestLogger(t),
		t.Context(),
		mustParseURL("https://ghcr.io/token?service=ghcr.io&scope=repository:linuxserver/radarr:pull"),
		"lscr.io/linuxserver/radarr:latest",
		testBasicAuth,
		client,
		registryConfig.Registry{},
	)
	require.NoError(t, err)
	assert.Equal(t, "scoped-token", token)
}

// Test_performBearerTokenFetch verifies that a token is read from a successful
// token response, and that a failed request or an error status is an error, a
// 429 a rate-limit error.
func Test_performBearerTokenFetch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response *http.Response
		doErr    error
		want     string
		wantErr  error
	}{
		{
			name:     "successful token fetch",
			response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"test-token","expires_in":1800,"issued_at":"2024-01-01T00:00:00Z"}`))},
			want:     "test-token",
		},
		{
			name:    "client error returns error",
			doErr:   assert.AnError,
			wantErr: errFailedExecuteBearerRequest,
		},
		{
			name: "401 Unauthorized returns error",
			response: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid credentials"}`)),
			},
			wantErr: errFailedExecuteBearerRequest,
		},
		{
			name: "429 Too Many Requests returns error",
			response: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"error":"rate limit"}`)),
			},
			wantErr: ratelimit.ErrRateLimited,
		},
		{
			name: "500 Internal Server Error returns error",
			response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(`{"error":"server error"}`)),
			},
			wantErr: errFailedExecuteBearerRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := mockAuth.NewMockClient(t)
			client.EXPECT().Do(mock.Anything).Return(tt.response, tt.doErr).Once()

			got, _, err := performBearerTokenFetch(newTestLogger(t), t.Context(), mustParseURL("https://test.com/token?service=test.com"), "test/image", "", client, registryConfig.Registry{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_lookupAnonymousGHCRToken verifies that the shared anonymous GHCR token
// is found only while it is cached and unexpired.
//
// The test resets the shared token cache, so it does not run in parallel.
func Test_lookupAnonymousGHCRToken(t *testing.T) {
	t.Run("miss when cache is empty", func(t *testing.T) {
		resetTokenCache(t)

		got, ok := lookupAnonymousGHCRToken(newTestLogger(t))
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("hit returns unexpired shared token", func(t *testing.T) {
		seedAnonymousGHCRToken(t)

		got, ok := lookupAnonymousGHCRToken(newTestLogger(t))
		assert.True(t, ok)
		assert.Equal(t, testAnonymousGHCRToken, got)
	})

	t.Run("expired entry is not returned", func(t *testing.T) {
		resetTokenCache(t)

		tokenCache.SetIfAbsent(bearerCacheKey(mustParseURL("https://ghcr.io/token?service=ghcr.io"), ""), tokenCacheEntry{
			token:     "Bearer stale-token",
			expiresAt: time.Now().Add(-time.Minute),
		})

		got, ok := lookupAnonymousGHCRToken(newTestLogger(t))
		assert.False(t, ok)
		assert.Empty(t, got)
	})
}

// Test_cachedAnonymousGHCRToken verifies that the cached anonymous GHCR token
// is used for anonymous GHCR and lscr.io requests, and not for requests with
// credentials, to a mirror, or to another registry.
//
// The test seeds the shared token cache, so it does not run in parallel.
func Test_cachedAnonymousGHCRToken(t *testing.T) {
	seedAnonymousGHCRToken(t)

	tests := []struct {
		name         string
		image        string
		registryAuth string
		endpoint     string
		wantOK       bool
	}{
		{name: "anonymous GHCR cache hit", image: "ghcr.io/linuxserver/sonarr:latest", wantOK: true},
		{name: "anonymous lscr.io cache hit", image: "lscr.io/linuxserver/sonarr:latest", wantOK: true},
		{name: "authenticated GHCR does not skip", image: "ghcr.io/linuxserver/sonarr:latest", registryAuth: testBasicAuth},
		{name: "mirror endpoint does not skip", image: "ghcr.io/linuxserver/sonarr:latest", endpoint: "https://mirror.example.com"},
		{name: "Docker Hub does not skip", image: "library/nginx:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cachedAnonymousGHCRToken(newTestLogger(t), mustParseRef(t, tt.image), tt.registryAuth, tt.endpoint)
			assert.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				assert.Equal(t, TokenResult{}, got)

				return
			}

			assert.Equal(t, TokenResult{Token: testAnonymousGHCRToken, ChallengeHost: "ghcr.io"}, got)
		})
	}
}
