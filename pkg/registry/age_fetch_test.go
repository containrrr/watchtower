package registry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
)

// Digests served by the fake registries.
const (
	fakeConfigDigest      = "sha256:c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0"
	fakeOtherConfigDigest = "sha256:c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1c1"
	fakeRuntimeManifest   = "sha256:a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0"
	fakeOtherManifest     = "sha256:a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"
	fakeBearerToken       = "fake-registry-token"
	fakeOtherCreated      = "2023-06-01T00:00:00Z"
	fakeRepositoryPath    = "/v2/library/app"
	fakeManifestTagPath   = fakeRepositoryPath + "/manifests/1.0"
)

// fakeRegistry is a registry served over plain HTTP that answers each request
// path with a fixed handler and records every request it receives, noting
// which ones carried an Authorization header.
type fakeRegistry struct {
	server   *httptest.Server
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	requests []string
}

// newFakeRegistry starts a registry that answers the /v2/ challenge with 200
// and every other path with 404 until handlers are added.
func newFakeRegistry(t *testing.T) *fakeRegistry {
	t.Helper()

	registry := &fakeRegistry{handlers: map[string]http.HandlerFunc{}}
	registry.handle("/v2/", respondWith(http.StatusOK, "", "{}"))

	registry.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := r.Method + " " + r.URL.Path
		if r.Header.Get("Authorization") != "" {
			request += " (authorized)"
		}

		registry.mu.Lock()
		registry.requests = append(registry.requests, request)
		handler, ok := registry.handlers[r.URL.Path]
		registry.mu.Unlock()

		if !ok {
			http.NotFound(w, r)

			return
		}

		handler(w, r)
	}))
	t.Cleanup(registry.server.Close)

	return registry
}

// host returns the registry's host and port.
func (r *fakeRegistry) host() string {
	return strings.TrimPrefix(r.server.URL, "http://")
}

// handle serves path with handler.
func (r *fakeRegistry) handle(path string, handler http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[path] = handler
}

// served returns the recorded requests in the order they arrived.
func (r *fakeRegistry) served() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.requests...)
}

// respondWith returns a handler that writes a fixed response.
func respondWith(status int, contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// requireBearer wraps handler so it answers 401 unless the request carries
// the fake registry's bearer token.
func requireBearer(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+fakeBearerToken {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		handler(w, r)
	}
}

// singlePlatformManifest returns a handler for a single-platform manifest.
func singlePlatformManifest(configDigest string) http.HandlerFunc {
	return respondWith(http.StatusOK, "application/vnd.docker.distribution.manifest.v2+json",
		validManifestJSON(configDigest))
}

// platformIndex returns a handler for an image index with one manifest for the
// runtime platform and one for a platform the tests never run on.
func platformIndex() http.HandlerFunc {
	return respondWith(http.StatusOK, "application/vnd.oci.image.index.v1+json", fmt.Sprintf(`{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": %q,
				"size": 500,
				"platform": {"architecture": %q, "os": %q}
			},
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest": %q,
				"size": 500,
				"platform": {"architecture": "s390x", "os": "plan9"}
			}
		]
	}`, fakeRuntimeManifest, runtime.GOARCH, runtime.GOOS, fakeOtherManifest))
}

// configBlob returns a handler for an image config with the given created time.
func configBlob(created string) http.HandlerFunc {
	return respondWith(http.StatusOK, "application/vnd.oci.image.config.v1+json", validConfigJSON(created))
}

// fetchCreated runs FetchImageCreationTime for an image in registry, selecting
// the runtime platform from an image index.
func fetchCreated(t *testing.T, registry *fakeRegistry, image string) (time.Time, error) {
	t.Helper()

	return fetchCreatedFor(t, registry, image, registryConfig.Platform{})
}

// fetchCreatedFor runs FetchImageCreationTime for an image in registry,
// selecting platform from an image index. Plain HTTP is enabled so the fake
// registry can serve the requests.
func fetchCreatedFor(
	t *testing.T,
	registry *fakeRegistry,
	image string,
	platform registryConfig.Platform,
) (time.Time, error) {
	t.Helper()

	settings := registryConfig.Registry{CooldownPlatform: platform, TLSSkip: true}

	return FetchImageCreationTime(testLog(), t.Context(), newMockContainer(t, registry.host()+"/"+image), "", settings, ratelimit.New())
}

// mustParseTime parses an RFC 3339 timestamp.
func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)

	return parsed
}

// TestFetchImageCreationTime_SinglePlatform verifies the anonymous path for a
// single-platform image: the challenge, the manifest, then the config blob.
func TestFetchImageCreationTime_SinglePlatform(t *testing.T) {
	registry := newFakeRegistry(t)
	registry.handle(fakeManifestTagPath, singlePlatformManifest(fakeConfigDigest))
	registry.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, configBlob(testCreatedTimestamp))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))

	assert.Equal(t, []string{
		"GET /v2/",
		"GET " + fakeManifestTagPath,
		"GET " + fakeRepositoryPath + "/blobs/" + fakeConfigDigest,
	}, registry.served())
}

// TestFetchImageCreationTime_PlatformSelection verifies that an image index
// resolves to the manifest for the runtime platform by default, and to the
// configured platform when the cooldown platform settings are set.
func TestFetchImageCreationTime_PlatformSelection(t *testing.T) {
	tests := []struct {
		name     string
		platform registryConfig.Platform
		want     string
	}{
		{
			name: "runtime platform",
			want: testCreatedTimestamp,
		},
		{
			name:     "configured platform",
			platform: registryConfig.Platform{OS: "plan9", Architecture: "s390x"},
			want:     fakeOtherCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := newFakeRegistry(t)
			registry.handle(fakeManifestTagPath, platformIndex())
			registry.handle(fakeRepositoryPath+"/manifests/"+fakeRuntimeManifest, singlePlatformManifest(fakeConfigDigest))
			registry.handle(fakeRepositoryPath+"/manifests/"+fakeOtherManifest, singlePlatformManifest(fakeOtherConfigDigest))
			registry.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, configBlob(testCreatedTimestamp))
			registry.handle(fakeRepositoryPath+"/blobs/"+fakeOtherConfigDigest, configBlob(fakeOtherCreated))

			created, err := fetchCreatedFor(t, registry, "library/app:1.0", tt.platform)
			require.NoError(t, err)
			assert.True(t, mustParseTime(t, tt.want).Equal(created), "created %s, want %s", created, tt.want)
		})
	}
}

// TestFetchImageCreationTime_BearerToken verifies that a bearer challenge is
// answered with a token from the realm, and that the token is sent with every
// manifest and blob request.
func TestFetchImageCreationTime_BearerToken(t *testing.T) {
	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(registry))
	registry.handle("/token", tokenEndpoint())
	registry.handle(fakeManifestTagPath, requireBearer(platformIndex()))
	registry.handle(fakeRepositoryPath+"/manifests/"+fakeRuntimeManifest,
		requireBearer(singlePlatformManifest(fakeConfigDigest)))
	registry.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, requireBearer(configBlob(testCreatedTimestamp)))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))

	assert.Equal(t, []string{
		"GET /v2/",
		"GET /token",
		"GET " + fakeManifestTagPath + " (authorized)",
		"GET " + fakeRepositoryPath + "/manifests/" + fakeRuntimeManifest + " (authorized)",
		"GET " + fakeRepositoryPath + "/blobs/" + fakeConfigDigest + " (authorized)",
	}, registry.served())
}

// TestFetchImageCreationTime_ChallengeHostFallback verifies that when the
// registry does not serve the image but its bearer realm is on another host,
// the manifests and the config blob are fetched from the realm's host.
func TestFetchImageCreationTime_ChallengeHostFallback(t *testing.T) {
	realm := newFakeRegistry(t)
	realm.handle("/token", tokenEndpoint())
	realm.handle(fakeManifestTagPath, requireBearer(platformIndex()))
	realm.handle(fakeRepositoryPath+"/manifests/"+fakeRuntimeManifest,
		requireBearer(singlePlatformManifest(fakeConfigDigest)))
	realm.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, requireBearer(configBlob(testCreatedTimestamp)))

	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(realm))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))

	assert.Equal(t, []string{
		"GET /v2/",
		"GET " + fakeManifestTagPath + " (authorized)",
	}, registry.served(), "the image registry only answers the challenge and the first manifest request")
	assert.Equal(t, []string{
		"GET /token",
		"GET " + fakeManifestTagPath + " (authorized)",
		"GET " + fakeRepositoryPath + "/manifests/" + fakeRuntimeManifest + " (authorized)",
		"GET " + fakeRepositoryPath + "/blobs/" + fakeConfigDigest + " (authorized)",
	}, realm.served())
}

// TestFetchImageCreationTime_PlatformManifestRetry records that when the
// bearer realm is on the image registry's own host and the platform manifest
// is missing, the identical platform manifest request is sent a second time
// before the fetch fails.
func TestFetchImageCreationTime_PlatformManifestRetry(t *testing.T) {
	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(registry))
	registry.handle("/token", tokenEndpoint())
	registry.handle(fakeManifestTagPath, requireBearer(platformIndex()))

	_, err := fetchCreated(t, registry, "library/app:1.0")
	require.ErrorIs(t, err, errFetchManifestFailed)

	platformRequest := "GET " + fakeRepositoryPath + "/manifests/" + fakeRuntimeManifest + " (authorized)"
	assert.Equal(t, []string{
		"GET /v2/",
		"GET /token",
		"GET " + fakeManifestTagPath + " (authorized)",
		platformRequest,
		platformRequest,
	}, registry.served())
}

// TestFetchImageCreationTime_Failures verifies the error reported for each way
// the registry can fail to provide a creation time.
func TestFetchImageCreationTime_Failures(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		handlers map[string]http.HandlerFunc
		wantErr  error
	}{
		{
			name:    "manifest not found",
			image:   "library/app:1.0",
			wantErr: errFetchManifestFailed,
		},
		{
			name:  "manifest is not JSON",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: respondWith(http.StatusOK, "application/vnd.docker.distribution.manifest.v2+json", "not json"),
			},
			wantErr: errFetchManifestFailed,
		},
		{
			name:  "manifest without a config digest",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: singlePlatformManifest(""),
			},
			wantErr: errNoConfigDigest,
		},
		{
			name:  "manifest larger than the limit",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: respondWith(http.StatusOK, "application/vnd.docker.distribution.manifest.v2+json",
					strings.Repeat(" ", maxManifestSize+1)),
			},
			wantErr: errManifestTooLarge,
		},
		{
			name:  "no manifest for the platform",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: respondWith(http.StatusOK, "application/vnd.oci.image.index.v1+json",
					`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[`+
						`{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"`+fakeOtherManifest+`",`+
						`"platform":{"architecture":"s390x","os":"plan9"}}]}`),
			},
			wantErr: errNoPlatformMatch,
		},
		{
			name:  "config blob not found",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: singlePlatformManifest(fakeConfigDigest),
			},
			wantErr: errFetchConfigFailed,
		},
		{
			name:  "config blob is not JSON",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: singlePlatformManifest(fakeConfigDigest),
				fakeRepositoryPath + "/blobs/" + fakeConfigDigest: respondWith(http.StatusOK, "", "not json"),
			},
			wantErr: errParseConfigFailed,
		},
		{
			name:  "config blob without a created time",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				fakeManifestTagPath: singlePlatformManifest(fakeConfigDigest),
				fakeRepositoryPath + "/blobs/" + fakeConfigDigest: respondWith(http.StatusOK, "", configJSONWithoutCreated()),
			},
			wantErr: errImageCreationTimeMissing,
		},
		{
			name:  "challenge with an unexpected status",
			image: "library/app:1.0",
			handlers: map[string]http.HandlerFunc{
				"/v2/": respondWith(http.StatusInternalServerError, "", ""),
			},
			wantErr: errFetchManifestFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := newFakeRegistry(t)
			for path, handler := range tt.handlers {
				registry.handle(path, handler)
			}

			_, err := fetchCreated(t, registry, tt.image)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// bearerChallenge returns a /v2/ handler that challenges for a token from the
// realm served by realm.
func bearerChallenge(realm *fakeRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate",
			fmt.Sprintf(`Bearer realm="%s/token",service="fake-registry"`, realm.server.URL))
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// tokenEndpoint returns a handler that issues the fake registry's token.
func tokenEndpoint() http.HandlerFunc {
	return respondWith(http.StatusOK, "application/json", fmt.Sprintf(`{"token":%q}`, fakeBearerToken))
}

// TestFetchImageCreationTime_BlobRedirect verifies that a config blob
// redirected to a storage host is fetched from there without the registry's
// token. The redirect names the storage server as localhost so its hostname
// differs from the registry's, as a real storage host's would.
func TestFetchImageCreationTime_BlobRedirect(t *testing.T) {
	storage := newFakeRegistry(t)
	storage.handle("/blobs/config", configBlob(testCreatedTimestamp))

	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(registry))
	registry.handle("/token", tokenEndpoint())
	registry.handle(fakeManifestTagPath, requireBearer(singlePlatformManifest(fakeConfigDigest)))
	registry.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, requireBearer(func(w http.ResponseWriter, r *http.Request) {
		storageURL := strings.Replace(storage.server.URL, "127.0.0.1", "localhost", 1)
		http.Redirect(w, r, storageURL+"/blobs/config", http.StatusTemporaryRedirect)
	}))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))

	assert.Equal(t, []string{"GET /blobs/config"}, storage.served())
}

// TestFetchImageCreationTime_PlatformManifestOnChallengeHost verifies that a
// platform manifest missing from the image registry is fetched from the bearer
// realm's host, along with its config blob.
func TestFetchImageCreationTime_PlatformManifestOnChallengeHost(t *testing.T) {
	realm := newFakeRegistry(t)
	realm.handle("/token", tokenEndpoint())
	realm.handle(fakeRepositoryPath+"/manifests/"+fakeRuntimeManifest,
		requireBearer(singlePlatformManifest(fakeConfigDigest)))
	realm.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, requireBearer(configBlob(testCreatedTimestamp)))

	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(realm))
	registry.handle(fakeManifestTagPath, requireBearer(platformIndex()))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))

	assert.Equal(t, []string{
		"GET /v2/",
		"GET " + fakeManifestTagPath + " (authorized)",
		"GET " + fakeRepositoryPath + "/manifests/" + fakeRuntimeManifest + " (authorized)",
	}, registry.served())
	assert.Equal(t, []string{
		"GET /token",
		"GET " + fakeRepositoryPath + "/manifests/" + fakeRuntimeManifest + " (authorized)",
		"GET " + fakeRepositoryPath + "/blobs/" + fakeConfigDigest + " (authorized)",
	}, realm.served())
}

// TestFetchImageCreationTime_PlatformManifestRetrySucceeds records that when
// the bearer realm is on the image registry's own host, a platform manifest
// that is missing on the first request and present on the identical second
// request is used.
func TestFetchImageCreationTime_PlatformManifestRetrySucceeds(t *testing.T) {
	var attempts atomic.Int32

	registry := newFakeRegistry(t)
	registry.handle("/v2/", bearerChallenge(registry))
	registry.handle("/token", tokenEndpoint())
	registry.handle(fakeManifestTagPath, requireBearer(platformIndex()))
	registry.handle(fakeRepositoryPath+"/manifests/"+fakeRuntimeManifest, requireBearer(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.NotFound(w, r)

			return
		}

		singlePlatformManifest(fakeConfigDigest)(w, r)
	}))
	registry.handle(fakeRepositoryPath+"/blobs/"+fakeConfigDigest, requireBearer(configBlob(testCreatedTimestamp)))

	created, err := fetchCreated(t, registry, "library/app:1.0")
	require.NoError(t, err)
	assert.True(t, mustParseTime(t, testCreatedTimestamp).Equal(created))
	assert.Equal(t, int32(2), attempts.Load())
}
