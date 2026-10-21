package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/auth"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
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

func TestRegistryRateLimitError(t *testing.T) {
	assert.NoError(t, registryRateLimitError(nil))
	assert.NoError(t, registryRateLimitError(&http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://ghcr.io/v2/linuxserver/nginx/manifests/latest", nil)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"2"}},
		Body:       io.NopCloser(strings.NewReader("toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute")),
		Request:    req,
	}

	got := registryRateLimitError(resp)
	require.Error(t, got)
	require.ErrorIs(t, got, ratelimit.ErrRateLimited)
}

// newMockContainer creates a mockery-generated mock Container with the given image name.
func newMockContainer(t *testing.T, imageName string) *mockTypes.MockContainer {
	t.Helper()

	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return("test-container").Maybe()
	container.EXPECT().ImageName().Return(imageName).Maybe()

	return container
}

const testCreatedTimestamp = "2024-01-15T10:30:00Z"

// --- JSON helpers ---

func validManifestJSON(configDigest string) string {
	return fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"digest": "%s",
			"size": 1234
		}
	}`, configDigest)
}

func validConfigJSON(created string) string {
	return fmt.Sprintf(`{"created":"%s"}`, created)
}

func configJSONWithoutCreated() string {
	return `{"architecture":"amd64","os":"linux"}`
}

func multiPlatformIndexJSON(digestCurrentPlatform, digestOther string) string {
	return fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": "%s",
				"size": 500,
				"platform": {
					"architecture": "%s",
					"os": "%s"
				}
			},
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": "%s",
				"size": 500,
				"platform": {
					"architecture": "arm64",
					"os": "linux"
				}
			}
		]
	}`, digestCurrentPlatform, runtime.GOARCH, runtime.GOOS, digestOther)
}

func platformManifestJSON(configDigest string) string {
	return fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest": "%s",
			"size": 789
		}
	}`, configDigest)
}

// --- fetchManifestForAge tests ---
// These test the manifest fetching logic using a mock HTTP server and
// an injected auth.Client (the test server's HTTP client).

func TestFetchManifestForAge_SinglePlatform(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:abc123def456789012345678901234567890123456789012345678901234567890"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	got, _, err := fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "single_platform"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestFetchManifestForAge_RetriesSubMillisecondRateLimit(t *testing.T) {
	server := httpstub.New(t)

	configDigest := "sha256:abc123def456789012345678901234567890123456789012345678901234567890"
	limitKey := "age-retry-ghcr.io"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(
				http.StatusTooManyRequests,
				"toomanyrequests: retry-after: 23.722µs, allowed: 44000/minute",
				nil,
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	got, _, err := fetchManifestForAge(newTestLogger(t), t.Context(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		limitKey,
		map[string]any{"test": "rate_limit_retry"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestFetchManifestForAge_MultiPlatformIndex(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:cfg1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"
	platformDigest := fmt.Sprintf("sha256:plt%s%s67890abcdef1234567890abcdef1234567890abcdef1234567890", runtime.GOOS, runtime.GOARCH)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, multiPlatformIndexJSON(platformDigest, "sha256:arm64digest"),
				http.Header{"Content-Type": {"application/vnd.oci.image.index.v1+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/test/manifests/sha256:plt.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	got, _, err := fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "multi_platform"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestFetchManifestForAge_AuthFailure(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusUnauthorized, "unauthorized", nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "auth_failure"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchManifestFailed)
}

func TestFetchManifestForAge_NotFound(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusNotFound, "not found", nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "not_found"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchManifestFailed)
}

func TestFetchManifestForAge_MissingConfigDigest(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, `{
				"schemaVersion": 2,
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"config": {}
			}`, http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}}),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "missing_digest"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoConfigDigest)
}

// --- fetchConfigBlob tests ---

func TestFetchConfigBlob_Success(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:abc123def456789012345678901234567890123456789012345678901234567890"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/myrepo/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusOK, validConfigJSON(testCreatedTimestamp), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/myrepo/manifests/latest")
	require.NoError(t, err)

	body, err := fetchConfigBlob(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL,
		configDigest,
		"Bearer test-token",
		"",
		map[string]any{"test": "config_success"},
	)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	require.NotNil(t, config.Created)

	expected, _ := time.Parse(time.RFC3339, testCreatedTimestamp)
	assert.Equal(t, expected.UTC(), config.Created.UTC())
}

func TestFetchConfigBlob_Failure500(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:fail567890123456789012345678901234567890123456789012345678901234"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/myrepo/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusInternalServerError, "internal error", nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/myrepo/manifests/latest")
	require.NoError(t, err)

	_, err = fetchConfigBlob(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL,
		configDigest,
		"Bearer test-token",
		"",
		map[string]any{"test": "config_500"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchConfigFailed)
}

func TestFetchConfigBlob_Redirect(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:redir789012345678901234567890123456789012345678901234567890123456"
	redirectPath := "/redirected-config"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/myrepo/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusFound, "",
				http.Header{"Location": {server.URL() + redirectPath}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(redirectPath)),
			httpstub.RespondWith(http.StatusOK, validConfigJSON(testCreatedTimestamp), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/myrepo/manifests/latest")
	require.NoError(t, err)

	body, err := fetchConfigBlob(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL,
		configDigest,
		"Bearer test-token",
		"",
		map[string]any{"test": "config_redirect"},
	)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	require.NotNil(t, config.Created)

	expected, _ := time.Parse(time.RFC3339, testCreatedTimestamp)
	assert.Equal(t, expected.UTC(), config.Created.UTC())
}

// --- isIndexMediaType tests ---

func TestIsIndexMediaType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "OCI image index",
			input:    "application/vnd.oci.image.index.v1+json",
			expected: true,
		},
		{
			name:     "Docker manifest list",
			input:    "application/vnd.docker.distribution.manifest.list.v2+json",
			expected: true,
		},
		{
			name:     "Docker v2 manifest is not an index",
			input:    "application/vnd.docker.distribution.manifest.v2+json",
			expected: false,
		},
		{
			name:     "OCI image manifest is not an index",
			input:    "application/vnd.oci.image.manifest.v1+json",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "arbitrary media type",
			input:    "application/octet-stream",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isIndexMediaType(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// --- buildManifestURLForContainer tests ---

func TestBuildManifestURLForContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		imageName string
		scheme    string
		want      string
		wantErr   bool
		// errIs is the sentinel the error must wrap, nil to skip the check.
		errIs error
	}{
		{
			name:      "Docker Hub image with tag",
			imageName: "alpine:3.19",
			scheme:    "https",
			want:      "https://index.docker.io/v2/library/alpine/manifests/3.19",
		},
		{
			name:      "Docker Hub image with HTTPS scheme",
			imageName: "nginx:latest",
			scheme:    "https",
			want:      "https://index.docker.io/v2/library/nginx/manifests/latest",
		},
		{
			name:      "custom registry with tag",
			imageName: "ghcr.io/owner/repo:v1.0",
			scheme:    "https",
			want:      "https://ghcr.io/v2/owner/repo/manifests/v1.0",
		},
		{
			name:      "HTTP scheme",
			imageName: "alpine:latest",
			scheme:    "http",
			want:      "http://index.docker.io/v2/library/alpine/manifests/latest",
		},
		{
			name:      "private registry",
			imageName: "myregistry.io/myimage:tag",
			scheme:    "https",
			want:      "https://myregistry.io/v2/myimage/manifests/tag",
		},
		{
			name:      "invalid image name returns error",
			imageName: "INVALID UPPER",
			scheme:    "https",
			wantErr:   true,
		},
		{
			name:      "image name without a registry host returns error",
			imageName: "a_b.example.com/repo:tag",
			scheme:    "https",
			wantErr:   true,
			errIs:     auth.ErrMissingRegistryHost,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			container := newMockContainer(t, tc.imageName)

			got, err := buildManifestURLForContainer(newTestLogger(t), container, tc.scheme)
			if tc.wantErr {
				require.Error(t, err)

				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// --- buildManifestURLForAge tests ---

func TestBuildManifestURLForAge(t *testing.T) {
	t.Parallel()

	t.Run("standard Docker Hub image", func(t *testing.T) {
		t.Parallel()

		container := newMockContainer(t, "alpine:3.19")

		gotURL, gotHost, gotParsed, err := buildManifestURLForAge(newTestLogger(t), container, registryConfig.Registry{}, "")
		require.NoError(t, err)

		assert.Contains(t, gotURL, "/v2/library/alpine/manifests/3.19")
		assert.Equal(t, "index.docker.io", gotHost)
		assert.NotNil(t, gotParsed)
	})

	t.Run("scheme follows the TLS skip setting", func(t *testing.T) {
		t.Parallel()

		container := newMockContainer(t, "alpine:3.19")

		secureURL, _, _, err := buildManifestURLForAge(newTestLogger(t), container, registryConfig.Registry{}, "")
		require.NoError(t, err)

		plainURL, _, _, err := buildManifestURLForAge(newTestLogger(t), container, registryConfig.Registry{TLSSkip: true}, "")
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(secureURL, "https://"), "got %s", secureURL)
		assert.True(t, strings.HasPrefix(plainURL, "http://"), "got %s", plainURL)
	})

	t.Run("host override replaces host", func(t *testing.T) {
		t.Parallel()

		container := newMockContainer(t, "alpine:3.19")

		gotURL, _, _, err := buildManifestURLForAge(newTestLogger(t), container, registryConfig.Registry{}, "custom.registry.io")
		require.NoError(t, err)

		assert.Contains(t, gotURL, "custom.registry.io")
		assert.Contains(t, gotURL, "/v2/library/alpine/manifests/3.19")
	})

	t.Run("lscr.io is swapped to ghcr.io", func(t *testing.T) {
		t.Parallel()

		container := newMockContainer(t, "lscr.io/owner/image:tag")

		gotURL, gotHost, _, err := buildManifestURLForAge(newTestLogger(t), container, registryConfig.Registry{}, "")
		require.NoError(t, err)

		assert.Equal(t, "ghcr.io", gotHost)
		assert.Contains(t, gotURL, "ghcr.io")
		assert.Contains(t, gotURL, "/v2/owner/image/manifests/tag")
	})
}

// --- selectPlatformManifest tests ---

func TestSelectPlatformManifest_PlatformMatch(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:cfgmatch56789012345678901234567890123456789012345678901234567890123"
	platformDigest := fmt.Sprintf("sha256:pltmatch%s%s90123456789012345678901234567890123456789012", runtime.GOOS, runtime.GOARCH)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/repo/manifests/sha256:pltmatch.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/repo/manifests/latest")
	require.NoError(t, err)

	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    platformDigest,
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
			},
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:arm64digest",
				Size:      500,
				Platform: platform{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
		},
	}

	got, _, err := selectPlatformManifest(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		idx,
		parsedURL,
		"Bearer test-token",
		"", "", "",
		"",
		"",
		map[string]any{"test": "platform_match"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestSelectPlatformManifest_NoMatch(t *testing.T) {
	t.Parallel()

	parsedURL, err := url.Parse("https://registry.example.com/v2/repo/manifests/latest")
	require.NoError(t, err)

	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:unknownplatform",
				Size:      500,
				Platform: platform{
					Architecture: "unknown_arch",
					OS:           "unknown_os",
				},
			},
		},
	}

	_, _, err = selectPlatformManifest(newTestLogger(t), context.Background(),
		nil,
		ratelimit.New(),
		idx,
		parsedURL,
		"",
		"", "", "",
		"",
		"",
		map[string]any{"test": "no_match"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoPlatformMatch)
}

func TestSelectPlatformManifest_SkipsAttestationManifests(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:cfgnoattest01abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	platformDigest := fmt.Sprintf("sha256:pltnoattest%s%s345678901234567890123456789012345678901234", runtime.GOOS, runtime.GOARCH)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/repo/manifests/sha256:pltnoattest.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/repo/manifests/latest")
	require.NoError(t, err)

	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:attestationdigest",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
				Annotations: map[string]string{
					"vnd.docker.reference.type": "attestation-manifest",
				},
			},
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    platformDigest,
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
			},
		},
	}

	got, _, err := selectPlatformManifest(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		idx,
		parsedURL,
		"Bearer test-token",
		"", "", "",
		"",
		"",
		map[string]any{"test": "skips_attestation"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestSelectPlatformManifest_MissingConfigDigest(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	platformDigest := fmt.Sprintf("sha256:pltnodgst%s%s56789012345678901234567890123456789012345678", runtime.GOOS, runtime.GOARCH)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/repo/manifests/sha256:pltnodgst.*`)),
			httpstub.RespondWith(http.StatusOK, `{
				"schemaVersion": 2,
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"config": {}
			}`, http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}}),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/repo/manifests/latest")
	require.NoError(t, err)

	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    platformDigest,
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
			},
		},
	}

	_, _, err = selectPlatformManifest(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		idx,
		parsedURL,
		"Bearer test-token",
		"", "", "",
		"",
		"",
		map[string]any{"test": "missing_digest"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoConfigDigest)
}

// --- End-to-end pipeline tests: manifest fetch → config fetch → parse ---

func TestPipeline_SinglePlatformManifest_ReturnsCreationTime(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:ab4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusOK, validConfigJSON(testCreatedTimestamp), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_single"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)
	assert.Equal(t, configDigest, digest)

	body, err := fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	require.NotNil(t, config.Created)

	expected, _ := time.Parse(time.RFC3339, testCreatedTimestamp)
	assert.Equal(t, expected.UTC(), config.Created.UTC())
}

func TestPipeline_MultiPlatformIndex_ReturnsCreationTime(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	expectedTime := "2024-02-20T14:00:00Z"
	configDigest := "sha256:configdef4567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	platformDigest := fmt.Sprintf("sha256:platform%s%s7890abcdef1234567890abcdef1234567890abcdef12345678", runtime.GOOS, runtime.GOARCH)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, multiPlatformIndexJSON(platformDigest, "sha256:arm64digest"),
				http.Header{"Content-Type": {"application/vnd.oci.image.index.v1+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/library/alpine/manifests/sha256:platform.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusOK, validConfigJSON(expectedTime), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_multi"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)
	assert.Equal(t, configDigest, digest)

	body, err := fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	require.NotNil(t, config.Created)

	expected, _ := time.Parse(time.RFC3339, expectedTime)
	assert.Equal(t, expected.UTC(), config.Created.UTC())
}

func TestPipeline_MissingCreationTimestamp(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:confignocr890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusOK, configJSONWithoutCreated(), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_missing_created"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)

	body, err := fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	assert.Nil(t, config.Created, "created field should be nil when absent")
}

func TestPipeline_ConfigBlobNotFound(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:configfail01abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusInternalServerError,
				`{"errors":[{"code":"UNKNOWN","message":"internal error"}]}`,
				nil,
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_config_fail"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)

	_, err = fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchConfigFailed)
}

func TestPipeline_InvalidConfigJSON(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:configinval01abcdef1234567890abcdef1234567890abcdef1234567890abcd"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusOK, "not valid json", nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_invalid_json"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)

	body, err := fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	assert.Error(t, err, "expected JSON decode error for malformed config")
}

func TestPipeline_RedirectOnBlob(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	expectedTime := "2024-03-10T08:00:00Z"
	configDigest := "sha256:configredir01abcdef1234567890abcdef1234567890abcdef1234567890abcde"
	redirectPath := "/redirected-blob"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusOK, validManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/blobs/"+configDigest)),
			httpstub.RespondWith(http.StatusFound, "",
				http.Header{"Location": {server.URL() + redirectPath}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(redirectPath)),
			httpstub.RespondWith(http.StatusOK, validConfigJSON(expectedTime), nil),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	client := server.Client()
	ctx := context.Background()
	fields := map[string]any{"test": "pipeline_redirect"}

	digest, _, err := fetchManifestForAge(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL.String(), "", parsedURL, "", "", "", "", parsedURL.Host, "", fields)
	require.NoError(t, err)

	body, err := fetchConfigBlob(newTestLogger(t), ctx, client, ratelimit.New(), parsedURL, digest, "", "", fields)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })

	var config imageConfig

	err = json.NewDecoder(body).Decode(&config)
	require.NoError(t, err)
	require.NotNil(t, config.Created)

	expected, _ := time.Parse(time.RFC3339, expectedTime)
	assert.Equal(t, expected.UTC(), config.Created.UTC())
}

func TestPipeline_ManifestNotFound(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusNotFound,
				`{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`,
				nil,
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "pipeline_not_found"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchManifestFailed)
}

func TestPipeline_AuthFailure(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/library/alpine/manifests/3.19")),
			httpstub.RespondWith(http.StatusUnauthorized,
				`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`,
				nil,
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/library/alpine/manifests/3.19")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer invalid",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "pipeline_auth_fail"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errFetchManifestFailed)
}

func TestFetchManifestForAge_OversizedManifest(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	// Build a JSON manifest larger than maxManifestSize (1 MiB) by padding the digest field.
	oversizedDigest := "sha256:" + strings.Repeat("a", maxManifestSize+1024)
	oversizedManifest := fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"config": {
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"digest": "%s",
			"size": %d
		}
	}`, oversizedDigest, len(oversizedDigest))

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, oversizedManifest,
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	_, _, err = fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "oversized_manifest"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errManifestTooLarge)
}

func TestSelectPlatformManifest_AmbiguousPlatformMatch(t *testing.T) {
	t.Parallel()

	parsedURL, err := url.Parse("https://registry.example.com/v2/repo/manifests/latest")
	require.NoError(t, err)

	// Two entries matching runtime.GOOS/runtime.GOARCH with different variants
	// forces selectPlatformCandidate to return errAmbiguousPlatformMatch.
	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:digest1variantv7",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v7",
				},
			},
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:digest2variantv8",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v8",
				},
			},
		},
	}

	_, _, err = selectPlatformManifest(newTestLogger(t), context.Background(),
		nil,
		ratelimit.New(),
		idx,
		parsedURL,
		"",
		"", "", "",
		"",
		"",
		map[string]any{"test": "ambiguous_platform_match"},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, errAmbiguousPlatformMatch)
}

func TestSelectPlatformManifest_VariantSelection(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:cfgvariantsel01abcdef1234567890abcdef1234567890abcdef1234567890abc"
	platformDigestV8 := "sha256:pltv8variant01abcdef1234567890abcdef1234567890abcdef123456789012"

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/repo/manifests/sha256:pltv8.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/repo/manifests/latest")
	require.NoError(t, err)

	// Two entries matching runtime.GOOS/runtime.GOARCH with different variants.
	// Specifying "v8" should select the v8 variant instead of errAmbiguousPlatformMatch.
	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:digest1variantv7",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v7",
				},
			},
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    platformDigestV8,
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v8",
				},
			},
		},
	}

	got, _, err := selectPlatformManifest(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		idx,
		parsedURL,
		"Bearer test-token",
		runtime.GOOS,
		runtime.GOARCH,
		"v8", // Specify target variant
		"",
		"",
		map[string]any{"test": "variant_selection"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}

func TestSelectPlatformCandidate_VariantFiltering(t *testing.T) {
	t.Parallel()

	// Two entries matching runtime.GOOS/runtime.GOARCH with different variants.
	idx := imageIndex{
		MediaType:     "application/vnd.oci.image.index.v1+json",
		SchemaVersion: 2,
		Manifests: []indexEntry{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:digestv7variant001",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v7",
				},
			},
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:digestv8variant002",
				Size:      500,
				Platform: platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
					Variant:      "v8",
				},
			},
		},
	}

	t.Run("selects matching variant", func(t *testing.T) {
		t.Parallel()

		digest, err := selectPlatformCandidate(newTestLogger(t), idx,
			runtime.GOOS,
			runtime.GOARCH,
			"v8",
			map[string]any{"test": "variant_filter_v8"},
		)
		require.NoError(t, err)
		assert.Equal(t, "sha256:digestv8variant002", digest)
	})

	t.Run("no matching variant returns no platform match", func(t *testing.T) {
		t.Parallel()

		_, err := selectPlatformCandidate(newTestLogger(t), idx,
			runtime.GOOS,
			runtime.GOARCH,
			"v9", // No v9 variant exists
			map[string]any{"test": "variant_filter_no_match"},
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, errNoPlatformMatch)
	})

	t.Run("empty variant returns ambiguity error for multiple variants", func(t *testing.T) {
		t.Parallel()

		_, err := selectPlatformCandidate(newTestLogger(t), idx,
			runtime.GOOS,
			runtime.GOARCH,
			"", // No variant specified
			map[string]any{"test": "variant_filter_empty"},
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, errAmbiguousPlatformMatch)
	})
}

// indexJSONWithoutMediaType returns an image index JSON without the top-level mediaType field.
// This simulates registries (e.g., Amazon ECR) that omit mediaType from the JSON body
// but set the Content-Type header correctly.
func indexJSONWithoutMediaType(digestCurrentPlatform, digestOther string) string {
	return fmt.Sprintf(`{
		"schemaVersion": 2,
		"manifests": [
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": "%s",
				"size": 500,
				"platform": {
					"architecture": "%s",
					"os": "%s"
				}
			},
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": "%s",
				"size": 500,
				"platform": {
					"architecture": "arm64",
					"os": "linux"
				}
			}
		]
	}`, digestCurrentPlatform, runtime.GOARCH, runtime.GOOS, digestOther)
}

func TestFetchManifestForAge_IndexWithoutMediaType_ContentTypeFallback(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)

	configDigest := "sha256:cfgnometype01abcdef1234567890abcdef1234567890abcdef1234567890abcde"
	platformDigest := fmt.Sprintf("sha256:pltnometype%s%s01abcdef1234567890abcdef1234567890abcdef", runtime.GOOS, runtime.GOARCH)

	// First response: index JSON WITHOUT top-level mediaType, but Content-Type header indicates an index.
	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, indexJSONWithoutMediaType(platformDigest, "sha256:arm64digest"),
				http.Header{"Content-Type": {"application/vnd.oci.image.index.v1+json"}},
			),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`/v2/test/manifests/sha256:pltnometype.*`)),
			httpstub.RespondWith(http.StatusOK, platformManifestJSON(configDigest),
				http.Header{"Content-Type": {"application/vnd.oci.image.manifest.v1+json"}},
			),
		),
	)

	parsedURL, err := url.Parse(server.URL() + "/v2/test/manifests/latest")
	require.NoError(t, err)

	got, _, err := fetchManifestForAge(newTestLogger(t), context.Background(),
		server.Client(),
		ratelimit.New(),
		parsedURL.String(),
		"Bearer test-token",
		parsedURL,
		"", "", "",
		"",
		parsedURL.Host,
		"",
		map[string]any{"test": "index_no_mediatype"},
	)
	require.NoError(t, err)
	assert.Equal(t, configDigest, got)
}
