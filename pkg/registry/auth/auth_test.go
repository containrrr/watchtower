package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

// newNopLogger returns a logger that discards every event, for Ginkgo specs,
// benchmarks, and fuzz targets.
//
// Returns:
//   - *zerolog.Logger: Logger that discards all events.
func newNopLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

func Test_resolveChallengeScheme(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{
			name: "returns https for empty host",
			host: "",
			want: "https",
		},
		{
			name: "preserves https scheme by default",
			host: "https://example.com",
			want: "https",
		},
		{
			name: "returns https for non-empty host without explicit scheme",
			host: "example.com",
			want: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveChallengeScheme(newTestLogger(t), tt.host, registryConfig.Registry{})
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestResolveEndpoint verifies the host and scheme taken from registry
// endpoint overrides, as http and https URLs, scheme-relative URLs, and bare
// hosts with and without ports, and that any other endpoint is rejected.
func TestResolveEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		endpoint   string
		canonical  string
		scheme     string
		wantHost   string
		wantScheme string
		wantErr    bool
	}{
		{
			name:       "returns endpoint and http when endpoint is provided",
			endpoint:   "http://custom.example.com",
			canonical:  "index.docker.io",
			scheme:     "https",
			wantHost:   "custom.example.com",
			wantScheme: "http",
		},
		{
			name:       "returns canonical host and https when endpoint is empty",
			endpoint:   "",
			canonical:  "index.docker.io",
			scheme:     "https",
			wantHost:   "index.docker.io",
			wantScheme: "https",
		},
		{
			name:       "returns empty host and https when both are empty",
			endpoint:   "",
			canonical:  "",
			scheme:     "https",
			wantHost:   "",
			wantScheme: "https",
		},
		{
			name:       "bare endpoint preserves provided scheme",
			endpoint:   "custom.example.com",
			canonical:  "index.docker.io",
			scheme:     "http",
			wantHost:   "custom.example.com",
			wantScheme: "http",
		},
		{
			name:       "bare endpoint falls back to https when scheme is https",
			endpoint:   "custom.example.com",
			canonical:  "index.docker.io",
			scheme:     "https",
			wantHost:   "custom.example.com",
			wantScheme: "https",
		},
		{
			name:       "URL endpoint keeps its port",
			endpoint:   "https://mirror.example.com:5000",
			scheme:     "http",
			wantHost:   "mirror.example.com:5000",
			wantScheme: "https",
		},
		{
			name:       "scheme-relative endpoint keeps the provided scheme",
			endpoint:   "//mirror.example.com",
			scheme:     "http",
			wantHost:   "mirror.example.com",
			wantScheme: "http",
		},
		{
			name:       "bare host with port",
			endpoint:   "mirror.example.com:5000",
			scheme:     "https",
			wantHost:   "mirror.example.com:5000",
			wantScheme: "https",
		},
		{
			name:       "bare IPv4 address with port",
			endpoint:   "127.0.0.1:5000",
			scheme:     "http",
			wantHost:   "127.0.0.1:5000",
			wantScheme: "http",
		},
		{
			name:       "bare IPv6 address with port",
			endpoint:   "[::1]:5000",
			scheme:     "http",
			wantHost:   "[::1]:5000",
			wantScheme: "http",
		},
		{name: "rejects a host with spaces", endpoint: "bad host", scheme: "https", wantErr: true},
		{name: "rejects a URL whose host has spaces", endpoint: "https://bad host", scheme: "https", wantErr: true},
		{name: "rejects a bare host with a path", endpoint: "mirror.example.com/v2", scheme: "https", wantErr: true},
		{name: "rejects a bare host with a query", endpoint: "mirror.example.com?x=1", scheme: "https", wantErr: true},
		{name: "rejects a bare host with user info", endpoint: "user@mirror.example.com", scheme: "https", wantErr: true},
		{name: "rejects a bare host with a non-numeric port", endpoint: "mirror.example.com:abc", scheme: "https", wantErr: true},
		{name: "rejects a URL with an unsupported scheme", endpoint: "ftp://mirror.example.com", scheme: "https", wantErr: true},
		{name: "rejects a port without a host", endpoint: ":5000", scheme: "https", wantErr: true},
		{name: "rejects a bare IPv6 address without brackets", endpoint: "::", scheme: "http", wantErr: true},
		{name: "rejects a host made of colons and digits", endpoint: ":0:0", scheme: "https", wantErr: true},
		{name: "rejects a URL whose host has no name", endpoint: "http://:5000", scheme: "https", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotHost, gotScheme, err := ResolveEndpoint(tt.endpoint, tt.canonical, tt.scheme)
			if tt.wantErr {
				require.ErrorIs(t, err, errInvalidEndpoint)
				assert.Empty(t, gotHost)
				assert.Empty(t, gotScheme)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, gotHost)
			assert.Equal(t, tt.wantScheme, gotScheme)
		})
	}
}

func TestGetChallengeURL(t *testing.T) {
	tests := []struct {
		name       string
		imageRef   string
		endpoint   string
		wantHost   string
		wantScheme string
		wantErr    bool
		// errIs is the sentinel the error must wrap, nil to skip the check.
		errIs error
	}{
		{
			name:       "docker.io maps to index.docker.io",
			imageRef:   "docker.io/library/nginx:latest",
			endpoint:   "",
			wantHost:   "index.docker.io",
			wantScheme: "https",
			wantErr:    false,
		},
		{
			name:       "custom endpoint overrides host",
			imageRef:   "docker.io/library/nginx:latest",
			endpoint:   "mirror.example.com",
			wantHost:   "mirror.example.com",
			wantScheme: "https",
			wantErr:    false,
		},
		{
			name:       "lscr.io maps to ghcr.io",
			imageRef:   "lscr.io/linuxserver/nginx:latest",
			endpoint:   "",
			wantHost:   "ghcr.io",
			wantScheme: "https",
			wantErr:    false,
		},
		{
			name:       "non-docker host passes through",
			imageRef:   "ghcr.io/example/app:latest",
			endpoint:   "",
			wantHost:   "ghcr.io",
			wantScheme: "https",
			wantErr:    false,
		},
		{
			name:       "localhost passes through",
			imageRef:   "localhost:5000/myimage:latest",
			endpoint:   "",
			wantHost:   "localhost:5000",
			wantScheme: "https",
			wantErr:    false,
		},
		{
			name:     "invalid image ref returns error",
			imageRef: ":::invalid:::",
			endpoint: "",
			wantErr:  true,
		},
		{
			name:     "invalid endpoint returns error",
			imageRef: "docker.io/library/nginx:latest",
			endpoint: "invalid url with spaces",
			wantErr:  true,
			errIs:    errInvalidEndpoint,
		},
		{
			name:     "reference without a registry host returns error",
			imageRef: "a_b.example.com/repo:tag",
			wantErr:  true,
			errIs:    ErrMissingRegistryHost,
		},
		{
			name:       "endpoint supplies the host for a reference without one",
			imageRef:   "a_b.example.com/repo:tag",
			endpoint:   "mirror.example.com",
			wantHost:   "mirror.example.com",
			wantScheme: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := reference.ParseNormalizedNamed(tt.imageRef)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected parse error: %v", err)
				}

				return
			}

			got, err := getChallengeURL(newTestLogger(t), ref, registryConfig.Registry{}, tt.endpoint)
			if tt.wantErr {
				require.ErrorIs(t, err, tt.errIs)
				assert.Empty(t, got.Host)

				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.wantHost, got.Host)
			assert.Equal(t, tt.wantScheme, got.Scheme)
			assert.Equal(t, "/v2/", got.Path)
		})
	}
}

func TestGetChallengeRequest(t *testing.T) {
	server := newTestServer(t, http.StatusOK, "")
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	tests := []struct {
		name    string
		url     url.URL
		wantErr bool
	}{
		{
			name:    "creates GET request with context",
			url:     *serverURL,
			wantErr: false,
		},
		{
			name:    "empty URL still creates request",
			url:     url.URL{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getChallengeRequest(newTestLogger(t), context.Background(), tt.url)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, got.Method)
		})
	}
}

func Test_handleSuccessfulChallenge(t *testing.T) {
	tests := []struct {
		name         string
		redirected   bool
		redirectHost string
		want         TokenResult
	}{
		{
			name:         "non-redirected response",
			redirected:   false,
			redirectHost: "",
			want: TokenResult{
				Token:         "",
				ChallengeHost: "",
				Redirected:    false,
				RedirectHost:  "",
			},
		},
		{
			name:         "redirected response",
			redirected:   true,
			redirectHost: "new-host.example.com",
			want: TokenResult{
				Token:         "",
				ChallengeHost: "",
				Redirected:    true,
				RedirectHost:  "new-host.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handleSuccessfulChallenge(tt.redirected, tt.redirectHost)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_handleEmptyAuthHeader(t *testing.T) {
	ctx := context.Background()
	mockContainer := mockTypes.NewMockContainer(t)
	mockClient := mockAuth.NewMockClient(t)

	tests := []struct {
		name         string
		response     challengeResponse
		registryAuth string
		wantErr      error
	}{
		{
			name: "401 without WWW-Authenticate returns no credentials error",
			response: challengeResponse{
				statusCode:    http.StatusUnauthorized,
				wwwAuthHeader: "",
				redirected:    false,
				redirectHost:  "",
			},
			registryAuth: "",
			wantErr:      errNoCredentials,
		},
		{
			name: "non-200 without WWW-Authenticate returns unexpected status error",
			response: challengeResponse{
				statusCode:    http.StatusInternalServerError,
				wwwAuthHeader: "",
				redirected:    false,
				redirectHost:  "",
			},
			registryAuth: "",
			wantErr:      errUnexpectedStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processChallengeResponse(newTestLogger(t), ctx,
				mockContainer,
				tt.registryAuth,
				mockClient,
				registryConfig.Registry{},
				tt.response.redirected,
				tt.response.redirectHost,
				"",
				"",
				"",
				tt.response,
			)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			t.Fatal("expected error")
		})
	}
}

func Test_handleBasicAuthChallenge(t *testing.T) {
	tests := []struct {
		name           string
		registryAuth   string
		redirected     bool
		redirectHost   string
		originalHost   string
		originalScheme string
		redirectScheme string
		want           TokenResult
		wantErr        bool
		errContains    string
	}{
		{
			name:         "valid registry auth returns token result",
			registryAuth: "dGVzdA==",
			redirected:   false,
			redirectHost: "",
			originalHost: "registry.example.com",
			want: TokenResult{
				Token:         "Basic dGVzdA==",
				ChallengeHost: "",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name:         "empty registry auth returns error",
			registryAuth: "",
			redirected:   false,
			redirectHost: "",
			originalHost: "registry.example.com",
			want:         TokenResult{},
			wantErr:      true,
		},
		{
			name:         "cross-origin redirect returns error",
			registryAuth: "dGVzdA==",
			redirected:   true,
			redirectHost: "evil.example.com",
			originalHost: "registry.example.com",
			want:         TokenResult{},
			wantErr:      true,
		},
		{
			name:           "same-host https redirect returns token result",
			registryAuth:   "dGVzdA==",
			redirected:     true,
			redirectHost:   "registry.example.com",
			originalHost:   "registry.example.com",
			originalScheme: "https",
			redirectScheme: "https",
			want: TokenResult{
				Token:         "Basic dGVzdA==",
				ChallengeHost: "",
				Redirected:    true,
				RedirectHost:  "registry.example.com",
			},
			wantErr: false,
		},
		{
			name:           "same-host https to http downgrade returns error",
			registryAuth:   "dGVzdA==",
			redirected:     true,
			redirectHost:   "registry.example.com",
			originalHost:   "registry.example.com",
			originalScheme: "https",
			redirectScheme: "http",
			want:           TokenResult{},
			wantErr:        true,
			errContains:    "cross-origin redirect not allowed for basic auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handleBasicAuthChallenge(newTestLogger(t), tt.registryAuth, tt.redirected, tt.redirectHost, tt.originalHost, tt.originalScheme, tt.redirectScheme)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_handleUnsupportedChallenge(t *testing.T) {
	tests := []struct {
		name      string
		challenge string
		wantErr   bool
	}{
		{
			name:      "basic challenge returns error",
			challenge: "basic realm=\"test\"",
			wantErr:   true,
		},
		{
			name:      "empty challenge returns error",
			challenge: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleUnsupportedChallenge(newTestLogger(t), tt.challenge)
			require.Error(t, err)
			assert.ErrorIs(t, err, errUnsupportedChallenge)
		})
	}
}

func Test_processChallengeResponse(t *testing.T) {
	tests := []struct {
		name           string
		response       challengeResponse
		registryAuth   string
		originalHost   string
		originalScheme string
		redirectScheme string
		setupMock      func(*mockAuth.MockClient)
		want           TokenResult
		wantErr        bool
		errContains    string
	}{
		{
			name: "401 with bearer challenge routes to bearer auth",
			response: challengeResponse{
				statusCode:    http.StatusUnauthorized,
				wwwAuthHeader: `Bearer realm="https://challenge.test.com/token",service="challenge.test.com"`,
			},
			registryAuth: "",
			originalHost: "challenge.test.com",
			setupMock: func(mockClient *mockAuth.MockClient) {
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token":"test-token"}`)),
				}, nil).Once()
			},
			want: TokenResult{
				Token:         "Bearer test-token",
				ChallengeHost: "challenge.test.com",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name: "200 with empty auth header returns success",
			response: challengeResponse{
				statusCode:    http.StatusOK,
				wwwAuthHeader: "",
			},
			registryAuth: "",
			originalHost: "",
			setupMock:    func(*mockAuth.MockClient) {},
			want: TokenResult{
				Token:         "",
				ChallengeHost: "",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name: "401 with basic auth challenge",
			response: challengeResponse{
				statusCode:    http.StatusUnauthorized,
				wwwAuthHeader: `Basic realm="test"`,
			},
			registryAuth: "dGVzdA==",
			originalHost: "registry.example.com",
			setupMock:    func(*mockAuth.MockClient) {},
			want: TokenResult{
				Token:         "Basic dGVzdA==",
				ChallengeHost: "",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name: "cross-origin redirect with basic auth challenge returns error",
			response: challengeResponse{
				statusCode:    http.StatusUnauthorized,
				wwwAuthHeader: `Basic realm="test"`,
				redirected:    true,
				redirectHost:  "evil.example.com",
			},
			registryAuth: "dGVzdA==",
			originalHost: "registry.example.com",
			setupMock:    func(*mockAuth.MockClient) {},
			want:         TokenResult{},
			wantErr:      true,
			errContains:  "cross-origin redirect not allowed for basic auth",
		},
		{
			name: "same-host https to http downgrade with basic auth challenge returns error",
			response: challengeResponse{
				statusCode:    http.StatusUnauthorized,
				wwwAuthHeader: `Basic realm="test"`,
				redirected:    true,
				redirectHost:  "registry.example.com",
			},
			registryAuth:   "dGVzdA==",
			originalHost:   "registry.example.com",
			originalScheme: "https",
			redirectScheme: "http",
			setupMock:      func(*mockAuth.MockClient) {},
			want:           TokenResult{},
			wantErr:        true,
			errContains:    "cross-origin redirect not allowed for basic auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockAuth.NewMockClient(t)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			mockContainer := mockTypes.NewMockContainer(t)
			mockContainer.On("ImageName").Return("test/image").Maybe()
			mockContainer.On("IsNoPull", mock.Anything).Return(false).Maybe()
			mockContainer.On("IsStale").Return(false).Maybe()

			ctx := context.Background()

			got, err := processChallengeResponse(newTestLogger(t), ctx,
				mockContainer,
				tt.registryAuth,
				mockClient,
				registryConfig.Registry{},
				tt.response.redirected,
				tt.response.redirectHost,
				tt.originalHost,
				tt.originalScheme,
				tt.redirectScheme,
				tt.response,
			)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_handleBearerAuth(t *testing.T) {
	tests := []struct {
		name          string
		wwwAuthHeader string
		registryAuth  string
		setupMock     func(*mockAuth.MockClient)
		want          TokenResult
		wantErr       bool
		errContains   string
	}{
		{
			name:          "successful bearer auth",
			wwwAuthHeader: `Bearer realm="https://success.test.com/token",service="success.test.com"`,
			registryAuth:  "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token":"test-token"}`)),
				}, nil).Once()
			},
			want: TokenResult{
				Token:         "Bearer test-token",
				ChallengeHost: "success.test.com",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name:          "client Do error returns wrapped error",
			wwwAuthHeader: `Bearer realm="https://error.test.com/token",service="error.test.com"`,
			registryAuth:  "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				mockClient.On("Do", mock.Anything).Return(nil, assert.AnError).Once()
			},
			want:        TokenResult{},
			wantErr:     true,
			errContains: "failed to execute bearer token request",
		},
		{
			name:          "empty token returns error",
			wwwAuthHeader: `Bearer realm="https://empty.test.com/token",service="empty.test.com"`,
			registryAuth:  "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token":""}`)),
				}, nil).Once()
			},
			want:        TokenResult{},
			wantErr:     true,
			errContains: "empty token in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockAuth.NewMockClient(t)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			mockContainer := mockTypes.NewMockContainer(t)
			mockContainer.On("ImageName").Return("test/image").Maybe()
			mockContainer.On("IsNoPull", mock.Anything).Return(false).Maybe()
			mockContainer.On("IsStale").Return(false).Maybe()

			ctx := context.Background()

			got, err := handleBearerAuth(newTestLogger(t), ctx, tt.wwwAuthHeader, mockContainer, tt.registryAuth, mockClient, registryConfig.Registry{}, false, "")
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetToken(t *testing.T) {
	tests := []struct {
		name         string
		container    types.Container
		registryAuth string
		endpoint     string
		setupMock    func(*mockAuth.MockClient)
		want         TokenResult
		wantErr      bool
		errContains  string
	}{
		{
			name:         "no-pull container returns error on empty image name",
			container:    newNoPullContainer(t),
			registryAuth: "",
			endpoint:     "",
			setupMock:    func(*mockAuth.MockClient) {},
			want:         TokenResult{},
			wantErr:      true,
			errContains:  "failed to parse image name",
		},
		{
			name:         "localhost registry returns success with empty token",
			container:    newLocalhostContainer(t),
			registryAuth: "",
			endpoint:     "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				challengeURL, _ := url.Parse("https://localhost:5000/v2/")
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       http.NoBody,
					Header:     make(http.Header),
					Request:    &http.Request{URL: challengeURL},
				}, nil).Once()
			},
			want: TokenResult{
				Token:         "",
				ChallengeHost: "",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name:         "successful token fetch",
			container:    newRemoteContainer(t),
			registryAuth: "",
			endpoint:     "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				challengeURL, _ := url.Parse("https://index.docker.io/v2/")
				tokenURL, _ := url.Parse("https://index.docker.io/token?service=index.docker.io&scope=repository%3Atest%2Fimage%3Apull")

				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       http.NoBody,
					Header:     http.Header{"Www-Authenticate": []string{`Bearer realm="https://index.docker.io/token",service="index.docker.io"`}},
					Request:    &http.Request{URL: challengeURL},
				}, nil).Once()

				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"token":"test-token","expires_in":3600}`)),
					Header:     make(http.Header),
					Request:    &http.Request{URL: tokenURL},
				}, nil).Once()
			},
			want: TokenResult{
				Token:         "Bearer test-token",
				ChallengeHost: "index.docker.io",
				Redirected:    false,
				RedirectHost:  "",
			},
			wantErr: false,
		},
		{
			name:         "client Do error returns wrapped error",
			container:    newRemoteContainer(t),
			registryAuth: "",
			endpoint:     "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				challengeURL, _ := url.Parse("https://index.docker.io/v2/")
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       http.NoBody,
					Header:     make(http.Header),
					Request:    &http.Request{URL: challengeURL},
				}, assert.AnError).Once()
			},
			want:        TokenResult{},
			wantErr:     true,
			errContains: "failed to execute challenge request",
		},
		{
			name:         "challenge 429 returns rate-limit error",
			container:    newRemoteContainer(t),
			registryAuth: "",
			endpoint:     "",
			setupMock: func(mockClient *mockAuth.MockClient) {
				challengeURL, _ := url.Parse("https://index.docker.io/v2/")
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusTooManyRequests,
					Body: io.NopCloser(strings.NewReader(
						"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute",
					)),
					Header:  http.Header{"Retry-After": []string{"7200"}},
					Request: &http.Request{URL: challengeURL},
				}, nil).Once()
			},
			want:        TokenResult{},
			wantErr:     true,
			errContains: "registry rate limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockAuth.NewMockClient(t)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			ctx := context.Background()

			got, err := GetToken(newTestLogger(t), ctx, tt.container, tt.registryAuth, mockClient, registryConfig.Registry{}, tt.endpoint)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				if tt.errContains == "registry rate limited" {
					require.ErrorIs(t, err, ratelimit.ErrRateLimited)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetToken_anonymousGHCRSkipsChallengeOnCachedToken(t *testing.T) {
	seedAnonymousGHCRToken(t)

	mockClient := mockAuth.NewMockClient(t)
	got, err := GetToken(newTestLogger(t), context.Background(), newGHCRContainer(t), "", mockClient, registryConfig.Registry{}, "")
	require.NoError(t, err)
	assert.Equal(t, TokenResult{
		Token:         testAnonymousGHCRToken,
		ChallengeHost: "ghcr.io",
	}, got)
}

func TestGetToken_anonymousLSCRSkipsChallengeOnCachedToken(t *testing.T) {
	seedAnonymousGHCRToken(t)

	mockClient := mockAuth.NewMockClient(t)
	got, err := GetToken(newTestLogger(t), context.Background(), newLSCRContainer(t), "", mockClient, registryConfig.Registry{}, "")
	require.NoError(t, err)
	assert.Equal(t, TokenResult{
		Token:         testAnonymousGHCRToken,
		ChallengeHost: "ghcr.io",
	}, got)
}

func TestGetToken_anonymousGHCRChallengesWhenCacheEmpty(t *testing.T) {
	resetTokenCache(t)

	mockClient := mockAuth.NewMockClient(t)
	challengeURL, err := url.Parse("https://ghcr.io/v2/")
	require.NoError(t, err)

	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       http.NoBody,
		Header:     http.Header{"Www-Authenticate": []string{`Bearer realm="https://ghcr.io/token",service="ghcr.io"`}},
		Request:    &http.Request{URL: challengeURL},
	}, nil).Once()
	mockClient.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"token":"fresh-ghcr-token","expires_in":3600}`)),
		Header:     make(http.Header),
		Request:    &http.Request{URL: mustParseURL("https://ghcr.io/token")},
	}, nil).Once()

	got, err := GetToken(newTestLogger(t), context.Background(), newGHCRContainer(t), "", mockClient, registryConfig.Registry{}, "")
	require.NoError(t, err)
	assert.Equal(t, TokenResult{
		Token:         "Bearer fresh-ghcr-token",
		ChallengeHost: "ghcr.io",
	}, got)
}

func TestGetToken_anonymousGHCRDoesNotSkipMirrorOrCredentials(t *testing.T) {
	seedAnonymousGHCRToken(t)

	tests := []struct {
		name         string
		registryAuth string
		endpoint     string
		challengeURL string
	}{
		{
			name:         "credentials still challenge",
			registryAuth: "dXNlcjpwYXNz",
			challengeURL: "https://ghcr.io/v2/",
		},
		{
			name:         "mirror endpoint still challenge",
			endpoint:     "https://mirror.example.com",
			challengeURL: "https://mirror.example.com/v2/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockAuth.NewMockClient(t)
			challengeURL, err := url.Parse(tt.challengeURL)
			require.NoError(t, err)

			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
				Request:    &http.Request{URL: challengeURL},
			}, nil).Once()

			got, err := GetToken(newTestLogger(t), context.Background(), newGHCRContainer(t), tt.registryAuth, mockClient, registryConfig.Registry{}, tt.endpoint)
			require.NoError(t, err)
			assert.Equal(t, TokenResult{
				Redirected:   false,
				RedirectHost: "",
			}, got)
		})
	}
}

func newNoPullContainer(t *testing.T) types.Container {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.On("ImageName").Return("").Maybe()
	container.On("IsNoPull", mock.Anything).Return(true).Maybe()
	container.On("IsStale").Return(false).Maybe()
	container.On("IsRunning").Return(true).Maybe()

	return container
}

func newLocalhostContainer(t *testing.T) types.Container {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.On("ImageName").Return("localhost:5000/test/image:latest").Maybe()
	container.On("IsNoPull", mock.Anything).Return(false).Maybe()
	container.On("IsStale").Return(false).Maybe()
	container.On("IsRunning").Return(true).Maybe()

	return container
}

func newRemoteContainer(t *testing.T) types.Container {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.On("ImageName").Return("test/image:latest").Maybe()
	container.On("IsNoPull", mock.Anything).Return(false).Maybe()
	container.On("IsStale").Return(false).Maybe()
	container.On("IsRunning").Return(true).Maybe()

	return container
}

func newGHCRContainer(t *testing.T) types.Container {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.On("ImageName").Return("ghcr.io/linuxserver/sonarr:latest").Maybe()
	container.On("IsNoPull", mock.Anything).Return(false).Maybe()
	container.On("IsStale").Return(false).Maybe()
	container.On("IsRunning").Return(true).Maybe()

	return container
}

func newLSCRContainer(t *testing.T) types.Container {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.On("ImageName").Return("lscr.io/linuxserver/sonarr:latest").Maybe()
	container.On("IsNoPull", mock.Anything).Return(false).Maybe()
	container.On("IsStale").Return(false).Maybe()
	container.On("IsRunning").Return(true).Maybe()

	return container
}

// newTestServer creates an HTTP test server that returns the given status and body.
func newTestServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)

		if body != "" {
			_, _ = fmt.Fprint(w, body)
		}
	}))
}
