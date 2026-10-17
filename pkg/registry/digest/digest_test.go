package digest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func testLog() *zerolog.Logger {
	return logging.NopLogger()
}

func TestNormalizeDigest(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		expected string
	}{
		{
			name:     "strips sha256 prefix",
			digest:   "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:     "returns unchanged when no prefix matches",
			digest:   "md5:abc123",
			expected: "md5:abc123",
		},
		{
			name:     "empty string returns empty",
			digest:   "",
			expected: "",
		},
		{
			name:     "sha512 prefix is not stripped",
			digest:   "sha512:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12",
			expected: "sha512:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:     "sha256 prefix with short value",
			digest:   "sha256:abc",
			expected: "abc",
		},
		{
			name:     "sha256 colon only",
			digest:   "sha256:",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDigest(testLog(), tt.digest)
			assert.Equal(t, tt.expected, result, "NormalizeDigest(%q)", tt.digest)
		})
	}
}

func TestDigestsMatch(t *testing.T) {
	tests := []struct {
		name         string
		localDigests []string
		remoteDigest string
		expected     bool
	}{
		{
			name:         "matching digest with sha256 prefix on remote",
			localDigests: []string{"repo@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			remoteDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "matching digest with sha256 prefix on local only",
			localDigests: []string{"repo@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			remoteDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "matching digest with sha256 prefix on both",
			localDigests: []string{"repo@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			remoteDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "non-matching digests",
			localDigests: []string{"repo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			remoteDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			expected:     false,
		},
		{
			name:         "empty local digests returns false",
			localDigests: []string{},
			remoteDigest: "sha256:abc",
			expected:     false,
		},
		{
			name:         "nil local digests returns false",
			localDigests: nil,
			remoteDigest: "sha256:abc",
			expected:     false,
		},
		{
			name: "multiple local digests with one match",
			localDigests: []string{
				"repo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"repo@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"repo@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			remoteDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "digest with empty repo prefix",
			localDigests: []string{"@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			remoteDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "skips malformed digest without @ separator",
			localDigests: []string{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
			remoteDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     false,
		},
		{
			name: "mixed malformed and valid digests",
			localDigests: []string{
				"sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				"repo@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			remoteDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected:     true,
		},
		{
			name:         "empty remote digest",
			localDigests: []string{"repo@sha256:abc"},
			remoteDigest: "",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := digestsMatch(testLog(), tt.localDigests, tt.remoteDigest)
			assert.Equal(t, tt.expected, result, "digestsMatch(%v, %q)", tt.localDigests, tt.remoteDigest)
		})
	}
}

func TestFormatDigest(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "already prefixed", input: "sha256:abc", want: "sha256:abc"},
		{name: "raw hash", input: "abcdef", want: "sha256:abcdef"},
		{name: "other algorithm preserved", input: "sha512:deadbeef", want: "sha512:deadbeef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatDigest(tt.input))
		})
	}
}

func TestCompareDigestWithRemote(t *testing.T) {
	const remoteHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("outdated")
	mc.On("HasImageInfo").Return(true)
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			"registry.example.com/myapp@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:"+remoteHash)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	parts := strings.SplitN(server.URL, "://", 2)
	host := parts[len(parts)-1]
	mc.On("ImageName").Return(host + "/myapp:latest")

	match, remoteDigest, err := CompareDigestWithRemote(testLog(), context.Background(), mc, "", registryConfig.Registry{}, server.URL)
	require.NoError(t, err)
	assert.False(t, match)
	assert.Equal(t, "sha256:"+remoteHash, remoteDigest)
}

func TestCompareDigestWithRemote_Head429DoesNotFallBackToGET(t *testing.T) {
	ratelimit.ResetForTest()
	t.Cleanup(ratelimit.ResetForTest)

	settings := registryConfig.Registry{TLSSkip: true}

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("radarr")
	mc.On("HasImageInfo").Return(true)
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			"lscr.io/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	var getManifests atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/") {
			getManifests.Add(1)

			t.Errorf("HEAD 429 must not fall back to GET of %s", r.URL.Path)
			w.WriteHeader(http.StatusOK)

			return
		}

		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Retry-After", "7200")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute"))

			return
		}

		if r.Method == http.MethodGet && (r.URL.Path == "/v2/" || r.URL.Path == "/v2") {
			w.WriteHeader(http.StatusOK)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	parts := strings.SplitN(server.URL, "://", 2)
	host := parts[len(parts)-1]
	mc.On("ImageName").Return(host + "/linuxserver/radarr:latest")

	match, _, err := CompareDigestWithRemote(testLog(), context.Background(), mc, "", settings, server.URL)
	require.Error(t, err)
	require.ErrorIs(t, err, ratelimit.ErrRateLimited)
	assert.False(t, match)
	assert.Equal(t, int64(0), getManifests.Load())
}

// TestCompareDigestWithRemote_Retry429DoesNotContinueToNextEndpoint is a
// regression test for a 429 on the challenge-host retry. That error must abort
// the endpoint loop instead of trying the next mirror.
func TestCompareDigestWithRemote_Retry429DoesNotContinueToNextEndpoint(t *testing.T) {
	ratelimit.ResetForTest()
	t.Cleanup(ratelimit.ResetForTest)

	settings := registryConfig.Registry{TLSSkip: true}

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("radarr")
	mc.On("HasImageInfo").Return(true)
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			"lscr.io/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	var (
		primaryHeads atomic.Int64
		mirrorHeads  atomic.Int64
	)

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/v2/" || r.URL.Path == "/v2") {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer realm="http://%s/token",service="test-service",scope="repository:linuxserver/radarr:pull"`,
				r.Host,
			))
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"test-token"}`))

			return
		}

		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/") {
			if primaryHeads.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			w.Header().Set("Retry-After", "7200")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute"))

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer primary.Close()

	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/") {
			mirrorHeads.Add(1)
			w.Header().Set("Docker-Content-Digest", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
			w.WriteHeader(http.StatusOK)

			return
		}

		if r.Method == http.MethodGet && (r.URL.Path == "/v2/" || r.URL.Path == "/v2") {
			w.WriteHeader(http.StatusOK)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer mirror.Close()

	parts := strings.SplitN(primary.URL, "://", 2)
	host := parts[len(parts)-1]
	mc.On("ImageName").Return(host + "/linuxserver/radarr:latest")

	match, _, err := CompareDigestWithRemote(
		testLog(),
		context.Background(),
		mc,
		"",
		settings,
		primary.URL,
		mirror.URL,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ratelimit.ErrRateLimited)
	assert.False(t, match)
	assert.GreaterOrEqual(t, primaryHeads.Load(), int64(2))
	assert.Equal(t, int64(0), mirrorHeads.Load())
}

// TestCompareDigestWithRemote_UnauthorizedOnBothHostsIsError verifies that a
// registry that rejects the manifest request on both its own host and its
// token host produces an error, not an empty digest that marks the image as
// out of date and causes a pull.
func TestCompareDigestWithRemote_UnauthorizedOnBothHostsIsError(t *testing.T) {
	ratelimit.ResetForTest()
	t.Cleanup(ratelimit.ResetForTest)

	settings := registryConfig.Registry{TLSSkip: true}

	// The token host issues tokens but rejects manifest requests.
	tokenHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"test-token"}`))

			return
		}

		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer tokenHost.Close()

	// The registry sends clients to the token host and rejects manifest requests.
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" || r.URL.Path == "/v2" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer realm="%s/token",service="test-service",scope="repository:linuxserver/radarr:pull"`,
				tokenHost.URL,
			))
		}

		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer registry.Close()

	host := strings.TrimPrefix(registry.URL, "http://")

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("radarr").Maybe()
	mc.On("HasImageInfo").Return(true).Maybe()
	mc.On("ImageName").Return(host + "/linuxserver/radarr:latest")
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			host + "/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	match, remoteDigest, err := CompareDigestWithRemote(testLog(), context.Background(), mc, "", settings)
	require.ErrorIs(t, err, errUnresolvedManifestRetry)
	assert.False(t, match)
	assert.Empty(t, remoteDigest)
}

// TestCompareDigestWithRemote_HeadRejectedFallsBackToGet verifies that when a
// registry that is its own token host rejects HEAD manifest requests, the
// retried HEAD falls back to GET, which returns the digest.
func TestCompareDigestWithRemote_HeadRejectedFallsBackToGet(t *testing.T) {
	ratelimit.ResetForTest()
	t.Cleanup(ratelimit.ResetForTest)

	settings := registryConfig.Registry{TLSSkip: true}

	const remote = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/" || r.URL.Path == "/v2":
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer realm="http://%s/token",service="test-service",scope="repository:linuxserver/radarr:pull"`,
				r.Host,
			))
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"test-token"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Docker-Content-Digest", remote)
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer registry.Close()

	host := strings.TrimPrefix(registry.URL, "http://")

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("radarr").Maybe()
	mc.On("HasImageInfo").Return(true).Maybe()
	mc.On("ImageName").Return(host + "/linuxserver/radarr:latest")
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			host + "/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	})

	match, remoteDigest, err := CompareDigestWithRemote(testLog(), context.Background(), mc, "", settings)
	require.NoError(t, err)
	assert.False(t, match)
	assert.Equal(t, remote, remoteDigest)
}

// clearLocalOnlyImageCache removes all cached local-only image entries, so
// each test starts without remembered local-only images.
func clearLocalOnlyImageCache() {
	localOnlyImageCache.Range(func(key, _ any) bool {
		localOnlyImageCache.Delete(key)

		return true
	})
}

// TestCompareDigestWithRemote_LocalOnly404 is a regression test for locally built
// images (e.g. atlas-badgerdb) that have non-empty registry-less RepoDigests.
// A HEAD 404 must be treated as up-to-date with no error so callers skip the pull
// without logging "Digest retrieval failed, falling back to full pull".
func TestCompareDigestWithRemote_LocalOnly404(t *testing.T) {
	clearLocalOnlyImageCache()
	t.Cleanup(clearLocalOnlyImageCache)

	settings := registryConfig.Registry{TLSSkip: true}

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("atlas-badgerdb-1")
	mc.On("ImageName").Return("atlas-badgerdb:latest")
	mc.On("HasImageInfo").Return(true)
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		RepoDigests: []string{
			"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
		},
	})
	mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{
			Image: "atlas-badgerdb",
		},
	})

	var headManifestCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Auth probe may GET /v2/. Only manifest GET would indicate unwanted fallback.
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/") {
			t.Errorf("unexpected GET to %s. Local-only 404 must not fall back to GET", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)

			return
		}

		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/") {
			headManifestCalls++

			w.WriteHeader(http.StatusNotFound)

			return
		}

		// Unauthenticated registry probe.
		if r.Method == http.MethodGet && (r.URL.Path == "/v2/" || r.URL.Path == "/v2") {
			w.WriteHeader(http.StatusOK)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	parts := strings.SplitN(server.URL, "://", 2)
	host := parts[len(parts)-1]

	match, remoteDigest, err := CompareDigestWithRemote(testLog(), context.Background(),
		mc,
		"",
		settings,
		host,
	)
	require.NoError(t, err)
	assert.True(t, match)
	assert.Empty(t, remoteDigest)
	assert.GreaterOrEqual(t, headManifestCalls, 1)
}

// TestIsLocalImageNotFound verifies domain-less Config.Image + errManifestNotFound
// identity matching (a prior dual-sentinel bug broke errors.Is).
func TestIsLocalImageNotFound(t *testing.T) {
	t.Run("true for domain-less Config.Image with wrapped errManifestNotFound", func(t *testing.T) {
		mc := mockTypes.NewMockContainer(t)
		mc.On("HasImageInfo").Return(true)
		mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "atlas-badgerdb"},
		})

		err := fmt.Errorf("%w: %w: status 404 Not Found", errManifestNotFound, errInvalidRegistryResponse)
		assert.True(t, isLocalImageNotFound(mc, err))
	})

	t.Run("true for domain-less image with version tag containing dots", func(t *testing.T) {
		mc := mockTypes.NewMockContainer(t)
		mc.On("HasImageInfo").Return(true)
		mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"my-app@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "my-app:1.0"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.True(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for domain-ful Config.Image", func(t *testing.T) {
		mc := mockTypes.NewMockContainer(t)
		mc.On("HasImageInfo").Return(true)
		mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "registry.example.com/app:latest"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.False(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for Hub short name with registry-qualified RepoDigests", func(t *testing.T) {
		// Official images often keep Config.Image as "nginx" while RepoDigests
		// records docker.io/library/nginx@sha256:.... A transient 404 must not
		// mark them local-only for the process lifetime.
		mc := mockTypes.NewMockContainer(t)
		mc.On("HasImageInfo").Return(true)
		mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"docker.io/library/nginx@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "nginx:latest"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.False(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for non-manifest errors", func(t *testing.T) {
		mc := mockTypes.NewMockContainer(t)
		err := fmt.Errorf("%w: status 500", errInvalidRegistryResponse)
		assert.False(t, isLocalImageNotFound(mc, err))
	})
}

// TestLocalOnlyImageCache ensures a confirmed local-only image skips subsequent
// registry probes for the same image name+ID.
func TestLocalOnlyImageCache(t *testing.T) {
	clearLocalOnlyImageCache()
	t.Cleanup(clearLocalOnlyImageCache)

	settings := registryConfig.Registry{TLSSkip: true}

	const imageID = "sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35"

	mc := mockTypes.NewMockContainer(t)
	mc.On("Name").Return("atlas-badgerdb-1")
	mc.On("ImageName").Return("atlas-badgerdb:latest")
	mc.On("HasImageInfo").Return(true)
	mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
		ID: imageID,
		RepoDigests: []string{
			"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
		},
	})
	mc.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{Image: "atlas-badgerdb"},
	})

	var headManifestCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/manifests/") {
			headManifestCalls++

			w.WriteHeader(http.StatusNotFound)

			return
		}

		if r.Method == http.MethodGet && (r.URL.Path == "/v2/" || r.URL.Path == "/v2") {
			w.WriteHeader(http.StatusOK)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	parts := strings.SplitN(server.URL, "://", 2)
	host := parts[len(parts)-1]

	// First call: probe registry, cache local-only.
	match, _, err := CompareDigestWithRemote(testLog(), context.Background(), mc, "", settings, host)
	require.NoError(t, err)
	assert.True(t, match)
	assert.Equal(t, 1, headManifestCalls)

	// Second call: cache hit, no additional HEAD.
	match, _, err = CompareDigestWithRemote(testLog(), context.Background(), mc, "", settings, host)
	require.NoError(t, err)
	assert.True(t, match)
	assert.Equal(t, 1, headManifestCalls, "cached local-only image must not re-probe registry")
}

func TestCompareDigest(t *testing.T) {
	tests := []struct {
		name           string
		setupContainer func(t *testing.T) *mockTypes.MockContainer
		setupServer    func() *httptest.Server
		expected       bool
		wantErr        bool
	}{
		{
			name: "returns error when container has no image info",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("test-container")
				mc.On("ImageName").Return("nginx:latest")
				mc.On("HasImageInfo").Return(false)

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("server should not be called when image info is missing")
				}))
			},
			expected: false,
			wantErr:  true,
		},
		{
			name: "returns true for locally built image with empty RepoDigests",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("local-image")
				mc.On("ImageName").Return("local-image:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{RepoDigests: []string{}})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("server should not be called for locally built image")
				}))
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "matching digests via HEAD request",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("up-to-date")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Docker-Content-Digest", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
					w.WriteHeader(http.StatusOK)
				}))
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "non-matching digests via HEAD request",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("outdated")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Docker-Content-Digest", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
					w.WriteHeader(http.StatusOK)
				}))
			},
			expected: false,
			wantErr:  false,
		},
		{
			name: "falls back to GET when HEAD returns non-2xx without 404",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("fallback-test")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodHead {
						w.WriteHeader(http.StatusInternalServerError)

						return
					}

					w.Header().Set("Docker-Content-Digest", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
					w.WriteHeader(http.StatusOK)
				}))
			},
			expected: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := tt.setupContainer(t)

			server := tt.setupServer()
			defer server.Close()

			serverURL := server.URL
			parts := strings.SplitN(serverURL, "://", 2)
			host := parts[len(parts)-1]

			container.On("ImageName").Return(host + "/myapp:latest").Maybe()

			got, err := CompareDigest(testLog(), context.Background(), container, "", registryConfig.Registry{}, serverURL)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.expected, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFetchDigestWithGet(t *testing.T) {
	tests := []struct {
		name           string
		setupContainer func(t *testing.T) *mockTypes.MockContainer
		setupServer    func() *httptest.Server
		expected       string
		wantErr        bool
	}{
		{
			name: "extracts digest from GET response header",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("fetch-header")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Docker-Content-Digest", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
					w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json"}`))
				}))
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "extracts digest from GET response JSON body",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("fetch-json-body")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
					w.WriteHeader(http.StatusOK)

					body, err := json.Marshal(map[string]string{"digest": "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"})
					if err != nil {
						panic(err)
					}

					_, _ = w.Write(body)
				}))
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "extracts digest from plain text body",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("fetch-plain-text")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"))
				}))
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "returns error when no digest header and empty body",
			setupContainer: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("fetch-empty-body")
				mc.On("ImageName").Return("registry.example.com/myapp:latest")
				mc.On("HasImageInfo").Return(true)
				mc.On("ImageInfo").Return(&dockerImage.InspectResponse{
					RepoDigests: []string{
						"registry.example.com/myapp@sha256:abc",
					},
				})

				return mc
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := tt.setupContainer(t)

			server := tt.setupServer()
			defer server.Close()

			parts := strings.SplitN(server.URL, "://", 2)
			host := parts[len(parts)-1]
			container.On("ImageName").Return(host + "/myapp:latest").Maybe()

			got, err := fetchDigestWithGet(testLog(), context.Background(), container, "", registryConfig.Registry{}, server.URL)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBuildManifestURL(t *testing.T) {
	tests := []struct {
		name         string
		tlsSkip      bool
		container    func(t *testing.T) *mockTypes.MockContainer
		hostOverride string
		expectedURL  string
		expectedHost string
		wantErr      bool
	}{
		{
			name: "builds HTTPS manifest URL with empty override",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("https-default")
				mc.On("ImageName").Return("nginx:latest")

				return mc
			},
			hostOverride: "",
			expectedURL:  "https://index.docker.io/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
			wantErr:      false,
		},
		{
			name:    "builds HTTP manifest URL when TLS is skipped",
			tlsSkip: true,
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("http-tls-skip")
				mc.On("ImageName").Return("nginx:latest")

				return mc
			},
			hostOverride: "",
			expectedURL:  "http://index.docker.io/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
			wantErr:      false,
		},
		{
			name: "overrides host with bare host string",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("bare-host-override")
				mc.On("ImageName").Return("nginx:latest")

				return mc
			},
			hostOverride: "mirror.example.com",
			expectedURL:  "https://mirror.example.com/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
			wantErr:      false,
		},
		{
			name: "overrides host and scheme from full URL",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("full-url-override")
				mc.On("ImageName").Return("nginx:latest")

				return mc
			},
			hostOverride: "http://mirror.example.com:5000",
			expectedURL:  "http://mirror.example.com:5000/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
			wantErr:      false,
		},
		{
			name: "redirects lscr.io to ghcr.io",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()
				mc := mockTypes.NewMockContainer(t)
				mc.On("Name").Return("lscr-redirect")
				mc.On("ImageName").Return("lscr.io/linuxserver/radarr:latest")

				return mc
			},
			hostOverride: "",
			expectedURL:  "https://ghcr.io/v2/linuxserver/radarr/manifests/latest",
			expectedHost: "lscr.io",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := registryConfig.Registry{TLSSkip: tt.tlsSkip}

			gotURL, gotHost, _, err := buildManifestURL(testLog(), tt.container(t), settings, tt.hostOverride)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, gotURL)
			assert.Equal(t, tt.expectedHost, gotHost)
		})
	}
}

func TestHandleManifestResponse(t *testing.T) {
	tests := []struct {
		name           string
		setupResponse  func() *http.Response
		method         string
		originalHost   string
		challengeHost  string
		redirected     bool
		parsedURL      *url.URL
		currentHost    string
		expectedDigest string
		expectedURL    string
		expectedRetry  bool
		wantErr        bool
	}{
		{
			name:   "successful HEAD 200 without digest header returns error",
			method: http.MethodHead,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "registry.example.com",
			wantErr:       true,
		},
		{
			name:   "successful GET 200 returns digest from header",
			method: http.MethodGet,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:   "registry.example.com",
			challengeHost:  "",
			redirected:     false,
			parsedURL:      mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:    "registry.example.com",
			expectedDigest: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expectedURL:    "",
			expectedRetry:  false,
			wantErr:        false,
		},
		{
			name:   "HEAD 404 returns error without retry",
			method: http.MethodHead,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Header:     http.Header{},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "registry.example.com",
			wantErr:       true,
		},
		{
			name:   "GET 404 returns error without retry",
			method: http.MethodGet,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Header:     http.Header{},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "registry.example.com",
			wantErr:       true,
		},
		{
			name:   "non-redirected HEAD 401 retries on challenge host",
			method: http.MethodHead,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     http.Header{"Www-Authenticate": []string{"Bearer"}},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "ghcr.io",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "registry.example.com",
			expectedURL:   "https://ghcr.io/v2/manifests/latest",
			expectedRetry: true,
			wantErr:       false,
		},
		{
			name:   "3xx redirect updates URL host",
			method: http.MethodGet,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusMovedPermanently,
					Status:     "301 Moved Permanently",
					Header:     http.Header{"Location": []string{"https://new-registry.example.com/v2/manifests/latest"}},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "registry.example.com",
			expectedURL:   "https://new-registry.example.com/v2/manifests/latest",
			expectedRetry: true,
			wantErr:       false,
		},
		{
			name:   "401 on redirected host retries on original host",
			method: http.MethodGet,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     http.Header{"Www-Authenticate": []string{"Bearer"}},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://mirror.example.com/v2/manifests/latest")},
				}
			},
			originalHost:  "registry.example.com",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://registry.example.com/v2/manifests/latest"),
			currentHost:   "mirror.example.com",
			expectedURL:   "https://registry.example.com/v2/manifests/latest",
			expectedRetry: true,
			wantErr:       false,
		},
		{
			name:   "HEAD 429 returns rate-limit error without GET fallback",
			method: http.MethodHead,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Status:     "429 Too Many Requests",
					Header:     http.Header{"Retry-After": []string{"2"}},
					Body: io.NopCloser(strings.NewReader(
						"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute",
					)),
					Request: &http.Request{URL: mustParseURL("https://ghcr.io/v2/linuxserver/nginx/manifests/latest")},
				}
			},
			originalHost:  "ghcr.io",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://ghcr.io/v2/linuxserver/nginx/manifests/latest"),
			currentHost:   "ghcr.io",
			wantErr:       true,
		},
		{
			name:   "GET 429 returns rate-limit error",
			method: http.MethodGet,
			setupResponse: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Status:     "429 Too Many Requests",
					Header:     http.Header{"Retry-After": []string{"2"}},
					Body: io.NopCloser(strings.NewReader(
						"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute",
					)),
					Request: &http.Request{URL: mustParseURL("https://ghcr.io/v2/linuxserver/nginx/manifests/latest")},
				}
			},
			originalHost:  "ghcr.io",
			challengeHost: "",
			redirected:    false,
			parsedURL:     mustParseURL("https://ghcr.io/v2/linuxserver/nginx/manifests/latest"),
			currentHost:   "ghcr.io",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResponse()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			gotDigest, gotURL, gotRetry, err := handleManifestResponse(testLog(), resp,
				tt.method,
				tt.originalHost,
				tt.challengeHost,
				tt.redirected,
				tt.parsedURL,
				tt.currentHost,
			)
			if tt.wantErr {
				require.Error(t, err)

				if resp.StatusCode == http.StatusTooManyRequests {
					require.ErrorIs(t, err, ratelimit.ErrRateLimited)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedDigest, gotDigest)
			assert.Equal(t, tt.expectedURL, gotURL)
			assert.Equal(t, tt.expectedRetry, gotRetry)
		})
	}
}

func TestExtractHeadDigest(t *testing.T) {
	tests := []struct {
		name      string
		setupResp func() *http.Response
		expected  string
		wantErr   bool
	}{
		{
			name: "extracts digest from Docker-Content-Digest header",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}},
					Body:       http.NoBody,
				}
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "returns error when Docker-Content-Digest header is missing",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       http.NoBody,
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "includes www-authenticate header in error for 401",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     http.Header{"Www-Authenticate": []string{"Bearer realm=\"https://auth.example.com/token\""}},
					Body:       http.NoBody,
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "normalizes sha256 prefix from header",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{"sha256:abc"}},
					Body:       http.NoBody,
				}
			},
			expected: "abc",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResp()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			got, err := extractHeadDigest(testLog(), resp)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExtractGetDigest(t *testing.T) {
	tests := []struct {
		name      string
		setupResp func() *http.Response
		expected  string
		wantErr   bool
	}{
		{
			name: "extracts digest from response header",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}},
					Body:       http.NoBody,
				}
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "extracts digest from JSON manifest body with valid content type",
			setupResp: func() *http.Response {
				body, err := json.Marshal(map[string]string{"digest": "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"})
				if err != nil {
					panic(err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/vnd.docker.distribution.manifest.v2+json"}},
					Body:       io.NopCloser(strings.NewReader(string(body))),
				}
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "returns error when JSON body has empty digest field",
			setupResp: func() *http.Response {
				body, err := json.Marshal(map[string]string{"digest": ""})
				if err != nil {
					panic(err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/vnd.oci.image.manifest.v1+json"}},
					Body:       io.NopCloser(strings.NewReader(string(body))),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "rejects JSON body with unsupported content type",
			setupResp: func() *http.Response {
				body, err := json.Marshal(map[string]string{"digest": "sha256:abc"})
				if err != nil {
					panic(err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
					Body:       io.NopCloser(strings.NewReader(string(body))),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "extracts digest from plain text body starting with sha256",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
					Body:       io.NopCloser(strings.NewReader("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")),
				}
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "returns error when body is too short for plain text digest",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
					Body:       io.NopCloser(strings.NewReader("sha256:abc")),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "returns error when body is empty",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("")),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "returns error when body is whitespace only",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("   \n\t  ")),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "returns error when body is invalid JSON without sha256 prefix",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader("not json")),
				}
			},
			expected: "",
			wantErr:  true,
		},
		{
			name: "returns error when reading body fails",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       &errReadCloser{},
				}
			},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResp()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			got, err := extractGetDigest(testLog(), resp)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// errReadCloser is an io.ReadCloser that always returns an error.
type errReadCloser struct{}

func (e *errReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read error")
}

func (e *errReadCloser) Close() error {
	return nil
}

func TestExtractGetDigest_ManifestSizeLimits(t *testing.T) {
	const digestPrefix = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{
			name: "accepts body of exactly maxManifestSize",
			size: maxManifestSize,
		},
		{
			name:    "rejects body of maxManifestSize plus one",
			size:    maxManifestSize + 1,
			wantErr: errManifestTooLarge,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := digestPrefix + strings.Repeat("a", tc.size-len(digestPrefix))
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}

			t.Cleanup(func() { _ = resp.Body.Close() })

			got, err := extractGetDigest(testLog(), resp)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, strings.TrimPrefix(body, "sha256:"), got)
		})
	}
}

func TestMakeManifestRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		manifestURL string
		token       string
		verify      func(t *testing.T, req *http.Request, err error)
	}{
		{
			name:        "creates HEAD request without auth token",
			method:      http.MethodHead,
			manifestURL: "https://registry.example.com/v2/manifests/latest",
			token:       "",
			verify: func(t *testing.T, req *http.Request, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, req)
				assert.Equal(t, http.MethodHead, req.Method)
				assert.Equal(t, "https://registry.example.com/v2/manifests/latest", req.URL.String())
				assert.Empty(t, req.Header.Get("Authorization"))
			},
		},
		{
			name:        "creates GET request with auth token",
			method:      http.MethodGet,
			manifestURL: "https://registry.example.com/v2/manifests/latest",
			token:       "Bearer abc123",
			verify: func(t *testing.T, req *http.Request, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, req)
				assert.Equal(t, http.MethodGet, req.Method)
				assert.Equal(t, "Bearer abc123", req.Header.Get("Authorization"))
			},
		},
		{
			name:        "sets Accept header for manifest request",
			method:      http.MethodHead,
			manifestURL: "https://registry.example.com/v2/manifests/latest",
			token:       "",
			verify: func(t *testing.T, req *http.Request, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, req)
				accept := req.Header.Get("Accept")
				assert.Contains(t, accept, "application/vnd.docker.distribution.manifest.v2+json")
				assert.Contains(t, accept, "application/vnd.oci.image.manifest.v1+json")
			},
		},
		{
			name:        "sets User-Agent header",
			method:      http.MethodHead,
			manifestURL: "https://registry.example.com/v2/manifests/latest",
			token:       "",
			verify: func(t *testing.T, req *http.Request, err error) {
				t.Helper()
				require.NoError(t, err)
				require.NotNil(t, req)
				assert.NotEmpty(t, req.Header.Get("User-Agent"))
			},
		},
		{
			name:        "returns error for invalid URL",
			method:      http.MethodHead,
			manifestURL: "://invalid-url",
			token:       "",
			verify: func(t *testing.T, req *http.Request, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Nil(t, req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req, err := makeManifestRequest(ctx, tt.method, tt.manifestURL, tt.token)
			tt.verify(t, req, err)
		})
	}
}

func TestRetryManifestRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		updatedURL  string
		token       string
		setupClient func(t *testing.T) *mockAuth.MockClient
		expected    string
		wantErr     bool
		rateLimited bool
		// wantErrIs, when set, is an error the result must wrap.
		wantErrIs error
		// wantErrContains, when set, is text the error must contain.
		wantErrContains string
		// challengeHost is the token host. Empty means ghcr.io.
		challengeHost string
		// currentURL is the URL the retry runs on. Empty means the original host.
		currentURL string
	}{
		{
			name:       "returns digest from successful retry response",
			method:     http.MethodHead,
			updatedURL: "https://registry.example.com/v2/manifests/latest",
			token:      "Bearer token123",
			setupClient: func(t *testing.T) *mockAuth.MockClient {
				t.Helper()
				mc := mockAuth.NewMockClient(t)
				mc.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{"sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"}},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}, nil)

				return mc
			},
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name:       "returns error when client Do fails",
			method:     http.MethodHead,
			updatedURL: "https://registry.example.com/v2/manifests/latest",
			token:      "",
			setupClient: func(t *testing.T) *mockAuth.MockClient {
				t.Helper()
				mc := mockAuth.NewMockClient(t)
				mc.On("Do", mock.Anything).Return(nil, errors.New("connection refused"))

				return mc
			},
			expected: "",
			wantErr:  true,
		},
		{
			name:       "returns error when manifest response is invalid",
			method:     http.MethodGet,
			updatedURL: "https://registry.example.com/v2/manifests/latest",
			token:      "",
			setupClient: func(t *testing.T) *mockAuth.MockClient {
				t.Helper()
				mc := mockAuth.NewMockClient(t)
				mc.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusInternalServerError,
					Status:     "500 Internal Server Error",
					Header:     http.Header{},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}, nil)

				return mc
			},
			expected: "",
			wantErr:  true,
		},
		{
			name:       "returns rate-limit error on 429",
			method:     http.MethodHead,
			updatedURL: "https://registry.example.com/v2/manifests/latest",
			token:      "",
			setupClient: func(t *testing.T) *mockAuth.MockClient {
				t.Helper()

				header := http.Header{}
				header.Set("Retry-After", "7200")

				mc := mockAuth.NewMockClient(t)
				mc.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusTooManyRequests,
					Status:     "429 Too Many Requests",
					Header:     header,
					Body:       io.NopCloser(strings.NewReader("toomanyrequests: retry-after: 2h, allowed: 44000/minute")),
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}, nil)

				return mc
			},
			expected:    "",
			wantErr:     true,
			rateLimited: true,
		},
		{
			name:        "returns error when the GET retry on the token host asks for the original host",
			method:      http.MethodGet,
			updatedURL:  "https://ghcr.io/v2/manifests/latest",
			currentURL:  "https://ghcr.io/v2/manifests/latest",
			setupClient: unauthorizedManifestClient,
			wantErr:     true,
			wantErrIs:   errUnresolvedManifestRetry,
			// The error names the original-host URL the registry asked for.
			wantErrContains: "https://registry.example.com/v2/manifests/latest",
		},
		{
			name:          "returns an empty digest when the HEAD retry on a same-host token host asks again",
			method:        http.MethodHead,
			updatedURL:    "https://registry.example.com/v2/manifests/latest",
			challengeHost: "registry.example.com",
			setupClient:   unauthorizedManifestClient,
			expected:      "",
		},
		{
			name:       "returns an empty digest for a failed HEAD so the caller falls back to GET",
			method:     http.MethodHead,
			updatedURL: "https://registry.example.com/v2/manifests/latest",
			setupClient: func(t *testing.T) *mockAuth.MockClient {
				t.Helper()
				mc := mockAuth.NewMockClient(t)
				mc.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusInternalServerError,
					Status:     "500 Internal Server Error",
					Header:     http.Header{},
					Body:       http.NoBody,
					Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
				}, nil)

				return mc
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratelimit.ResetForTest()
			t.Cleanup(ratelimit.ResetForTest)

			client := tt.setupClient(t)

			challengeHost := cmp.Or(tt.challengeHost, "ghcr.io")
			currentURL := cmp.Or(tt.currentURL, "https://registry.example.com/v2/manifests/latest")

			got, err := retryManifestRequest(testLog(), context.Background(),
				tt.method,
				tt.updatedURL,
				tt.token,
				"registry.example.com",
				challengeHost,
				false,
				mustParseURL(currentURL),
				"",
				client,
			)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				assert.Equal(t, tt.rateLimited, ratelimit.Is(err))

				if tt.wantErrIs != nil {
					require.ErrorIs(t, err, tt.wantErrIs)
				}

				if tt.wantErrContains != "" {
					require.ErrorContains(t, err, tt.wantErrContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// unauthorizedManifestClient returns a registry client that answers every
// manifest request with 401, which asks for a retry on the other host.
func unauthorizedManifestClient(t *testing.T) *mockAuth.MockClient {
	t.Helper()

	mc := mockAuth.NewMockClient(t)
	mc.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		Header:     http.Header{},
		Body:       http.NoBody,
		Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
	}, nil)

	return mc
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("failed to parse URL %q: %v", raw, err))
	}

	return u
}
