package auth

import (
	"context"
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
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func Test_tokenExpiryCalculator_ExpireAfterCreate(t *testing.T) {
	calc := &tokenExpiryCalculator{}
	entry := otter.Entry[string, tokenCacheEntry]{
		Key:   "test",
		Value: tokenCacheEntry{expiresAt: time.Now().Add(5 * time.Minute)},
	}

	got := calc.ExpireAfterCreate(entry)
	assert.InDelta(t, 5*time.Minute.Seconds(), got.Seconds(), 1)
}

func Test_tokenExpiryCalculator_ExpireAfterUpdate(t *testing.T) {
	calc := &tokenExpiryCalculator{}
	entry := otter.Entry[string, tokenCacheEntry]{
		Key:   "test",
		Value: tokenCacheEntry{expiresAt: time.Now().Add(10 * time.Minute)},
	}

	got := calc.ExpireAfterUpdate(entry, tokenCacheEntry{})
	assert.InDelta(t, 10*time.Minute.Seconds(), got.Seconds(), 1)
}

func Test_tokenExpiryCalculator_ExpireAfterRead(t *testing.T) {
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

func Test_initTokenCache(t *testing.T) {
	assert.NotPanics(t, func() {
		initTokenCache(newTestLogger(t))
	})
}

func Test_computeTokenExpiry(t *testing.T) {
	now := time.Now()
	truncatedToSeconds := now.Truncate(time.Second)

	tests := []struct {
		name string
		args *types.TokenResponse
		want time.Time
	}{
		{
			name: "uses expires_in when provided",
			args: &types.TokenResponse{
				ExpiresIn: 3600,
			},
			want: truncatedToSeconds.Add(3600 * time.Second),
		},
		{
			name: "falls back to default TTL when expires_in is negative",
			args: &types.TokenResponse{
				ExpiresIn: -1,
			},
			want: truncatedToSeconds.Add(defaultTokenTTL),
		},
		{
			name: "uses issued_at with default TTL when expires_in is zero",
			args: &types.TokenResponse{
				IssuedAt: now.Add(-5 * time.Minute).Truncate(time.Second).Format(time.RFC3339),
			},
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
			args: &types.TokenResponse{
				IssuedAt: "not-a-date",
			},
			want: truncatedToSeconds.Add(defaultTokenTTL),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeTokenExpiry(newTestLogger(t), tt.args)
			assert.InDelta(t, tt.want.Unix(), got.Unix(), 2)
		})
	}
}

func Test_readBearerTokenWithExpiry(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		image   string
		want    string
		wantErr bool
	}{
		{
			name:    "valid token response",
			body:    `{"token":"test-token","expires_in":3600,"issued_at":"2024-01-01T00:00:00Z"}`,
			image:   "test/image",
			want:    "test-token",
			wantErr: false,
		},
		{
			name:    "invalid JSON returns error",
			body:    `{invalid json}`,
			image:   "test/image",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty token returns empty string",
			body:    `{"token":""}`,
			image:   "test/image",
			want:    "",
			wantErr: false,
		},
		{
			name:    "missing token field returns empty string",
			body:    `{"expires_in":3600}`,
			image:   "test/image",
			want:    "",
			wantErr: false,
		},
		{
			name:    "access_token used when token is empty",
			body:    `{"access_token":"fallback-token","expires_in":3600}`,
			image:   "test/image",
			want:    "fallback-token",
			wantErr: false,
		},
		{
			name:    "token takes precedence over access_token",
			body:    `{"token":"primary-token","access_token":"fallback-token","expires_in":3600}`,
			image:   "test/image",
			want:    "primary-token",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := readBearerTokenWithExpiry(newTestLogger(t), strings.NewReader(tt.body), tt.image)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_addBasicAuth(t *testing.T) {
	tests := []struct {
		name         string
		request      *http.Request
		imageName    string
		registryAuth string
		tlsSkip      bool
		wantHeader   string
	}{
		{
			name:         "adds basic auth header for HTTPS when credentials provided",
			request:      mustNewRequest(t, "https://example.com"),
			imageName:    "test/image",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			tlsSkip:      false,
			wantHeader:   "Basic dGVzdHVzZXI6dGVzdHBhc3M=",
		},
		{
			name:         "does not add header when no credentials",
			request:      mustNewRequest(t, "https://example.com"),
			imageName:    "test/image",
			registryAuth: "",
			tlsSkip:      false,
			wantHeader:   "",
		},
		{
			name:         "does not add header for HTTP without TLS skip",
			request:      mustNewRequest(t, "http://example.com"),
			imageName:    "test/image",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			tlsSkip:      false,
			wantHeader:   "",
		},
		{
			name:         "adds basic auth header for HTTP with TLS skip",
			request:      mustNewRequest(t, "http://example.com"),
			imageName:    "test/image",
			registryAuth: "dGVzdHVzZXI6dGVzdHBhc3M=",
			tlsSkip:      true,
			wantHeader:   "Basic dGVzdHVzZXI6dGVzdHBhc3M=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addBasicAuth(newTestLogger(t), tt.request, tt.imageName, tt.registryAuth, registryConfig.Registry{TLSSkip: tt.tlsSkip})
			assert.Equal(t, tt.wantHeader, tt.request.Header.Get("Authorization"))
		})
	}
}

func backgroundContext() context.Context {
	return context.Background()
}

func emptyContext() context.Context {
	return nil
}

func Test_newBearerRequest(t *testing.T) {
	authURL, _ := url.Parse("https://example.com/token")

	tests := []struct {
		name      string
		authURL   *url.URL
		imageName string
		ctx       func() context.Context
		wantErr   bool
	}{
		{
			name:      "creates GET request with context",
			authURL:   authURL,
			imageName: "test/image",
			ctx:       backgroundContext,
			wantErr:   false,
		},
		{
			name:      "nil context returns error",
			authURL:   authURL,
			imageName: "test/image",
			ctx:       emptyContext,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newBearerRequest(newTestLogger(t), tt.ctx(), tt.authURL, tt.imageName)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, got.Method)
			assert.Equal(t, "https://example.com/token", got.URL.String())
		})
	}
}

func Test_resolveService(t *testing.T) {
	tests := []struct {
		name   string
		values challengeValues
		image  string
		want   string
	}{
		{
			name:   "returns service when present",
			values: challengeValues{service: "ghcr.io"},
			image:  "test/image",
			want:   "ghcr.io",
		},
		{
			name:   "derives service from realm when service is empty",
			values: challengeValues{realm: "https://ghcr.io/token"},
			image:  "test/image",
			want:   "ghcr.io",
		},
		{
			name:   "returns empty when both service and realm are empty",
			values: challengeValues{},
			image:  "test/image",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveService(newTestLogger(t), tt.values, tt.image)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_validateRequiredChallengeValues(t *testing.T) {
	tests := []struct {
		name      string
		values    challengeValues
		image     string
		challenge string
		wantErr   bool
	}{
		{
			name:      "valid realm and service returns no error",
			values:    challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io"},
			image:     "test/image",
			challenge: "bearer",
			wantErr:   false,
		},
		{
			name:      "missing realm returns error",
			values:    challengeValues{service: "ghcr.io"},
			image:     "test/image",
			challenge: "bearer",
			wantErr:   true,
		},
		{
			name:      "missing service returns error",
			values:    challengeValues{realm: "https://ghcr.io/token"},
			image:     "test/image",
			challenge: "bearer",
			wantErr:   true,
		},
		{
			name:      "missing both returns error",
			values:    challengeValues{},
			image:     "test/image",
			challenge: "bearer",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredChallengeValues(newTestLogger(t), tt.values, tt.image, tt.challenge)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_buildAuthQuery(t *testing.T) {
	imageRef, _ := reference.ParseNormalizedNamed("ghcr.io/user/repo:latest")

	tests := []struct {
		name     string
		authURL  *url.URL
		values   challengeValues
		imageRef reference.Named
		want     string
	}{
		{
			name:     "adds service and scope query params",
			authURL:  mustParseURL("https://ghcr.io/token"),
			values:   challengeValues{service: "ghcr.io", scope: ""},
			imageRef: imageRef,
			want:     "scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
		{
			name:     "replaces existing service and scope query params",
			authURL:  mustParseURL("https://ghcr.io/token?service=old&scope=old-scope"),
			values:   challengeValues{service: "ghcr.io", scope: ""},
			imageRef: imageRef,
			want:     "scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAuthQuery(newTestLogger(t), tt.authURL, tt.values, tt.imageRef)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.RawQuery)
		})
	}
}

func mustParseURL(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	return parsed
}

func TestGetAuthURL(t *testing.T) {
	imageRef, _ := reference.ParseNormalizedNamed("ghcr.io/user/repo:latest")

	tests := []struct {
		name         string
		challenge    string
		imageRef     reference.Named
		registryAuth string
		wantErr      bool
		wantHost     string
		wantQuery    string
	}{
		{
			name:         "valid challenge constructs auth URL",
			challenge:    `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/repo:pull"`,
			imageRef:     imageRef,
			registryAuth: "",
			wantErr:      false,
			wantHost:     "ghcr.io",
			wantQuery:    "scope=repository%3Auser%2Frepo%3Apull&service=ghcr.io",
		},
		{
			name:      "missing realm returns error",
			challenge: `bearer service="ghcr.io"`,
			imageRef:  imageRef,
			wantErr:   true,
		},
		{
			name:      "realm without scheme returns error",
			challenge: `bearer realm="example.com/token",service="ghcr.io"`,
			imageRef:  imageRef,
			wantErr:   true,
		},
		{
			name:      "realm with non-HTTP(S) scheme returns error",
			challenge: `bearer realm="ftp://example.com/token",service="ghcr.io"`,
			imageRef:  imageRef,
			wantErr:   true,
		},
		{
			name:      "realm with empty host returns error",
			challenge: `bearer realm="http:///token",service="ghcr.io"`,
			imageRef:  imageRef,
			wantErr:   true,
		},
		{
			name:         "http realm without registry auth is allowed",
			challenge:    `bearer realm="http://insecure-registry.local/token",service="insecure-registry.local"`,
			imageRef:     imageRef,
			registryAuth: "",
			wantErr:      false,
			wantHost:     "insecure-registry.local",
			wantQuery:    "scope=repository%3Auser%2Frepo%3Apull&service=insecure-registry.local",
		},
		{
			name:         "http realm with registry auth is rejected without TLS skip",
			challenge:    `bearer realm="http://insecure-registry.local/token",service="insecure-registry.local"`,
			imageRef:     imageRef,
			registryAuth: "dGVzdA==",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAuthURL(newTestLogger(t), tt.challenge, tt.imageRef, tt.registryAuth, registryConfig.Registry{})
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, got.Host)
			assert.Equal(t, tt.wantQuery, got.RawQuery)
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	ctx := context.Background()
	imageRef, _ := reference.ParseNormalizedNamed("test/image:latest")
	mockClient := mockAuth.NewMockClient(t)

	tests := []struct {
		name         string
		challenge    string
		imageRef     reference.Named
		registryAuth string
		client       Client
		onDo         *http.Response
		doErr        error
		want         string
		wantErr      bool
	}{
		{
			name:      "successful token fetch",
			challenge: `bearer realm="https://test.com/token",service="test.com"`,
			imageRef:  imageRef,
			onDo: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"token":"test-token","expires_in":3600}`)),
			},
			want:    "Bearer test-token",
			wantErr: false,
		},
		{
			name:      "client Do returns error",
			challenge: `bearer realm="https://error.test.com/token",service="error.test.com"`,
			imageRef:  imageRef,
			doErr:     assert.AnError,
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.onDo != nil || tt.doErr != nil {
				mockClient.On("Do", mock.Anything).Return(tt.onDo, tt.doErr).Once()
			}

			got, err := getBearerToken(newTestLogger(t), ctx, tt.challenge, tt.imageRef, tt.registryAuth, mockClient, registryConfig.Registry{})
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_executeBearerTokenRequest_cacheMiss(t *testing.T) {
	ctx := context.Background()
	authURL, _ := url.Parse("https://test.com/token?service=test.com")
	imageName := "test/image"
	mockClient := mockAuth.NewMockClient(t)

	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"cached-token","expires_in":3600}`)),
	}, nil).Once()

	got, err := executeBearerTokenRequest(newTestLogger(t), ctx, authURL, imageName, "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer cached-token", got)
}

func Test_executeBearerTokenRequest_cacheHit(t *testing.T) {
	ctx := context.Background()
	authURL, _ := url.Parse("https://test.com/token?service=test.com")
	imageName := "test/image"
	mockClient := mockAuth.NewMockClient(t)

	resetTokenCache(t)

	cacheKey := authURL.String() + "|"
	tokenCache.SetIfAbsent(cacheKey, tokenCacheEntry{
		token:     "Bearer cached-token",
		expiresAt: time.Now().Add(time.Hour),
	})

	got, err := executeBearerTokenRequest(newTestLogger(t), ctx, authURL, imageName, "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer cached-token", got)
}

func resetTokenCache(t *testing.T) {
	t.Helper()
	initTokenCache(newTestLogger(t))
	tokenCache.InvalidateAll()
}

const testAnonymousGHCRToken = "Bearer cached-ghcr-token"

func seedAnonymousGHCRToken(t *testing.T) {
	t.Helper()
	resetTokenCache(t)

	authURL := mustParseURL("https://ghcr.io/token?service=ghcr.io")
	tokenCache.SetIfAbsent(bearerCacheKey(authURL, ""), tokenCacheEntry{
		token:     testAnonymousGHCRToken,
		expiresAt: time.Now().Add(time.Hour),
	})
}

func Test_bearerCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		registryAuth string
		want         string
	}{
		{
			name:         "anonymous GHCR drops repository scope",
			rawURL:       "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io",
			registryAuth: "",
			want:         "https://ghcr.io/token?service=ghcr.io|",
		},
		{
			name:         "anonymous GHCR shares key across images",
			rawURL:       "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io",
			registryAuth: "",
			want:         "https://ghcr.io/token?service=ghcr.io|",
		},
		{
			name:         "anonymous lscr.io token host shares with GHCR mapping",
			rawURL:       "https://lscr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io",
			registryAuth: "",
			want:         "https://lscr.io/token?service=ghcr.io|",
		},
		{
			name:         "authenticated GHCR keeps per-image scope",
			rawURL:       "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io",
			registryAuth: "dXNlcjpwYXNz",
			want:         "https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io|dXNlcjpwYXNz",
		},
		{
			name:         "anonymous Docker Hub keeps per-image scope",
			rawURL:       "https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io",
			registryAuth: "",
			want:         "https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authURL, err := url.Parse(tt.rawURL)
			require.NoError(t, err)
			assert.Equal(t, tt.want, bearerCacheKey(authURL, tt.registryAuth))
		})
	}
}

func Test_executeBearerTokenRequest_anonymousGHCRSharesToken(t *testing.T) {
	resetTokenCache(t)

	ctx := context.Background()
	mockClient := mockAuth.NewMockClient(t)
	prowlarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io")
	sonarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io")

	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.Host == "ghcr.io" &&
			req.URL.Query().Get("scope") == "repository:linuxserver/prowlarr:pull"
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"shared-anon-token","expires_in":3600}`)),
	}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), ctx, prowlarr, "ghcr.io/linuxserver/prowlarr", "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer shared-anon-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), ctx, sonarr, "ghcr.io/linuxserver/sonarr", "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer shared-anon-token", second)
}

func Test_executeBearerTokenRequest_authenticatedGHCRDoesNotShare(t *testing.T) {
	resetTokenCache(t)

	ctx := context.Background()
	mockClient := mockAuth.NewMockClient(t)
	registryAuth := "dXNlcjpwYXNz"
	prowlarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fprowlarr%3Apull&service=ghcr.io")
	sonarr := mustParseURL("https://ghcr.io/token?scope=repository%3Alinuxserver%2Fsonarr%3Apull&service=ghcr.io")

	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":3600}`)),
	}, nil).Once()
	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":3600}`)),
	}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), ctx, prowlarr, "ghcr.io/linuxserver/prowlarr", registryAuth, mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer scoped-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), ctx, sonarr, "ghcr.io/linuxserver/sonarr", registryAuth, mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer scoped-token", second)
}

func Test_executeBearerTokenRequest_anonymousNonGHCRDoesNotShare(t *testing.T) {
	resetTokenCache(t)

	ctx := context.Background()
	mockClient := mockAuth.NewMockClient(t)
	alpine := mustParseURL("https://auth.docker.io/token?scope=repository%3Alibrary%2Falpine%3Apull&service=registry.docker.io")
	nginx := mustParseURL("https://auth.docker.io/token?scope=repository%3Alibrary%2Fnginx%3Apull&service=registry.docker.io")

	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"hub-token","expires_in":3600}`)),
	}, nil).Once()
	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"hub-token","expires_in":3600}`)),
	}, nil).Once()

	first, err := executeBearerTokenRequest(newTestLogger(t), ctx, alpine, "library/alpine", "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer hub-token", first)

	second, err := executeBearerTokenRequest(newTestLogger(t), ctx, nginx, "library/nginx", "", mockClient, registryConfig.Registry{})
	require.NoError(t, err)
	assert.Equal(t, "Bearer hub-token", second)
}

func TestGetBearerToken_anonymousGHCRSharesAcrossImages(t *testing.T) {
	resetTokenCache(t)

	ctx := context.Background()
	mockClient := mockAuth.NewMockClient(t)
	prowlarr, err := reference.ParseNormalizedNamed("ghcr.io/linuxserver/prowlarr:latest")
	require.NoError(t, err)
	sonarr, err := reference.ParseNormalizedNamed("ghcr.io/linuxserver/sonarr:latest")
	require.NoError(t, err)
	watchtower, err := reference.ParseNormalizedNamed("ghcr.io/nicholas-fedor/watchtower:latest")
	require.NoError(t, err)

	challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:linuxserver/prowlarr:pull"`

	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"shared-anon-token","expires_in":3600}`)),
	}, nil).Once()

	for _, image := range []reference.Named{prowlarr, sonarr, watchtower} {
		got, getErr := getBearerToken(newTestLogger(t), ctx, challenge, image, "", mockClient, registryConfig.Registry{})
		require.NoError(t, getErr)
		assert.Equal(t, "Bearer shared-anon-token", got)
	}
}

func Test_performBearerTokenFetch_sendsBasicAuth(t *testing.T) {
	ctx := context.Background()
	authURL, err := url.Parse("https://ghcr.io/token?service=ghcr.io&scope=repository:linuxserver/radarr:pull")
	require.NoError(t, err)

	const registryAuth = "dXNlcjpwYXNz"

	mockClient := mockAuth.NewMockClient(t)
	mockClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req != nil && req.Header.Get("Authorization") == "Basic "+registryAuth
	})).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"scoped-token","expires_in":60}`)),
	}, nil).Once()

	token, _, err := performBearerTokenFetch(
		newTestLogger(t),
		ctx,
		authURL,
		"lscr.io/linuxserver/radarr:latest",
		registryAuth,
		mockClient,
		registryConfig.Registry{},
	)
	require.NoError(t, err)
	assert.Equal(t, "scoped-token", token)
}

func Test_performBearerTokenFetch(t *testing.T) {
	ctx := context.Background()
	authURL, _ := url.Parse("https://test.com/token?service=test.com")
	mockClient := mockAuth.NewMockClient(t)

	tests := []struct {
		name      string
		authURL   *url.URL
		imageName string
		auth      string
		onDo      *http.Response
		doErr     error
		want      string
		wantErr   bool
	}{
		{
			name:      "successful token fetch",
			authURL:   authURL,
			imageName: "test/image",
			auth:      "",
			onDo: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"token":"test-token","expires_in":1800,"issued_at":"2024-01-01T00:00:00Z"}`)),
			},
			want:    "test-token",
			wantErr: false,
		},
		{
			name:      "client error returns error",
			authURL:   authURL,
			imageName: "test/image",
			auth:      "",
			doErr:     assert.AnError,
			want:      "",
			wantErr:   true,
		},
		{
			name:      "401 Unauthorized returns error",
			authURL:   authURL,
			imageName: "test/image",
			auth:      "",
			onDo: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid credentials"}`)),
			},
			want:    "",
			wantErr: true,
		},
		{
			name:      "429 Too Many Requests returns error",
			authURL:   authURL,
			imageName: "test/image",
			auth:      "",
			onDo: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"error":"rate limit"}`)),
			},
			want:    "",
			wantErr: true,
		},
		{
			name:      "500 Internal Server Error returns error",
			authURL:   authURL,
			imageName: "test/image",
			auth:      "",
			onDo: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(`{"error":"server error"}`)),
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.onDo != nil || tt.doErr != nil {
				mockClient.On("Do", mock.Anything).Return(tt.onDo, tt.doErr).Once()
			}

			got, _, err := performBearerTokenFetch(newTestLogger(t), ctx, tt.authURL, tt.imageName, tt.auth, mockClient, registryConfig.Registry{})
			if tt.wantErr {
				assert.Error(t, err)

				if tt.onDo != nil && tt.onDo.StatusCode == http.StatusTooManyRequests {
					require.ErrorIs(t, err, ratelimit.ErrRateLimited)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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

		authURL := mustParseURL("https://ghcr.io/token?service=ghcr.io")
		tokenCache.SetIfAbsent(bearerCacheKey(authURL, ""), tokenCacheEntry{
			token:     "Bearer stale-token",
			expiresAt: time.Now().Add(-time.Minute),
		})

		got, ok := lookupAnonymousGHCRToken(newTestLogger(t))
		assert.False(t, ok)
		assert.Empty(t, got)
	})
}

func Test_cachedAnonymousGHCRToken(t *testing.T) {
	ghcrRef, err := reference.ParseNormalizedNamed("ghcr.io/linuxserver/sonarr:latest")
	require.NoError(t, err)
	lscrRef, err := reference.ParseNormalizedNamed("lscr.io/linuxserver/sonarr:latest")
	require.NoError(t, err)
	hubRef, err := reference.ParseNormalizedNamed("library/nginx:latest")
	require.NoError(t, err)

	seedAnonymousGHCRToken(t)

	tests := []struct {
		name         string
		imageRef     reference.Named
		registryAuth string
		endpoint     string
		wantOK       bool
	}{
		{
			name:     "anonymous GHCR cache hit",
			imageRef: ghcrRef,
			wantOK:   true,
		},
		{
			name:     "anonymous lscr.io cache hit",
			imageRef: lscrRef,
			wantOK:   true,
		},
		{
			name:         "authenticated GHCR does not skip",
			imageRef:     ghcrRef,
			registryAuth: "dXNlcjpwYXNz",
			wantOK:       false,
		},
		{
			name:     "mirror endpoint does not skip",
			imageRef: ghcrRef,
			endpoint: "https://mirror.example.com",
			wantOK:   false,
		},
		{
			name:     "Docker Hub does not skip",
			imageRef: hubRef,
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cachedAnonymousGHCRToken(newTestLogger(t), tt.imageRef, tt.registryAuth, tt.endpoint)
			assert.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				assert.Equal(t, TokenResult{}, got)

				return
			}

			assert.Equal(t, TokenResult{
				Token:         testAnonymousGHCRToken,
				ChallengeHost: "ghcr.io",
			}, got)
		})
	}
}
