package digest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
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

// testDigestHash is the manifest digest the stub registries serve.
const testDigestHash = "sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"

// testRepoDigest is the repo digest of the image the stub registry containers run.
const testRepoDigest = "ghcr.io/k6io/operator@" + testDigestHash

// newRegistryContainer returns a mock container that runs image, whose image has
// repoDigests as its repo digests.
//
// Parameters:
//   - t: Test that owns the mock.
//   - image: Image reference the container runs.
//   - repoDigests: The image's repo digests.
//
// Returns:
//   - *mockTypes.MockContainer: Mock that answers the container reads the digest code makes.
func newRegistryContainer(t *testing.T, image string, repoDigests ...string) *mockTypes.MockContainer {
	t.Helper()

	imageInfo := &dockerImage.InspectResponse{ID: image, RepoDigests: repoDigests}

	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return("mock-container").Maybe()
	container.EXPECT().ImageName().Return(image).Maybe()
	container.EXPECT().HasImageInfo().Return(true).Maybe()
	container.EXPECT().ImageInfo().Return(imageInfo).Maybe()
	container.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{Image: image},
	}).Maybe()

	return container
}

// bearerChallengeHandler returns the handler of a registry that answers /v2/
// with a bearer challenge whose realm is the /token endpoint on realmAddr.
//
// Parameters:
//   - t: Test that receives verification failures.
//   - realmAddr: Host and port of the registry that issues tokens.
//   - scope: Repository scope named in the challenge.
//
// Returns:
//   - http.HandlerFunc: Handler for the /v2/ request.
func bearerChallengeHandler(t *testing.T, realmAddr, scope string) http.HandlerFunc {
	t.Helper()

	challenge := fmt.Sprintf(`Bearer realm="http://%s/token",service="test-service",scope="%s"`, realmAddr, scope)

	return httpstub.CombineHandlers(
		httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
		httpstub.RespondWith(http.StatusUnauthorized, "", http.Header{"WWW-Authenticate": {challenge}}),
	)
}

// tokenHandler returns the handler of a registry's /token endpoint, which
// issues a bearer token.
//
// Parameters:
//   - t: Test that receives verification failures.
//
// Returns:
//   - http.HandlerFunc: Handler for the token request.
func tokenHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return httpstub.CombineHandlers(
		httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/token")),
		httpstub.RespondWith(http.StatusOK, `{"token": "mock-token"}`, nil),
	)
}

// bearerChallengeHandlers returns the handlers of a registry that answers /v2/
// with a bearer challenge whose realm is its own /token endpoint, and then
// issues a token.
//
// Parameters:
//   - t: Test that receives verification failures.
//   - addr: Host and port of the registry.
//   - scope: Repository scope named in the challenge.
//
// Returns:
//   - []http.HandlerFunc: Challenge and token handlers, in request order.
func bearerChallengeHandlers(t *testing.T, addr, scope string) []http.HandlerFunc {
	t.Helper()

	return []http.HandlerFunc{bearerChallengeHandler(t, addr, scope), tokenHandler(t)}
}

// anonymousRegistryHandler returns the handler of a registry that answers /v2/
// without asking for authentication.
//
// Parameters:
//   - t: Test that receives verification failures.
//
// Returns:
//   - http.HandlerFunc: Handler for the /v2/ request.
func anonymousRegistryHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return httpstub.CombineHandlers(
		httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
		httpstub.RespondWith(http.StatusOK, "", nil),
	)
}

// manifestHandler returns a handler that verifies a manifest request and
// answers it with status and header and no body.
//
// Parameters:
//   - t: Test that receives verification failures.
//   - method: Expected request method.
//   - path: Expected manifest path.
//   - status: Response status code.
//   - header: Response headers, or nil for none.
//
// Returns:
//   - http.HandlerFunc: Handler for one manifest request.
func manifestHandler(t *testing.T, method, path string, status int, header http.Header) http.HandlerFunc {
	t.Helper()

	return httpstub.CombineHandlers(
		httpstub.VerifyRequest(t, method, httpstub.Path(path)),
		httpstub.RespondWith(status, "", header),
	)
}

// digestHeader returns response headers that carry digest as the manifest digest.
//
// Parameters:
//   - digest: Manifest digest to report.
//
// Returns:
//   - http.Header: Headers with the Docker-Content-Digest header set.
func digestHeader(digest string) http.Header {
	return http.Header{ContentDigestHeader: {digest}}
}

// newLocalOnlyContainer returns a mock container for a locally built image
// whose repo digest names no registry, as Docker records for an image built and
// tagged on the host.
//
// Parameters:
//   - t: Test that owns the mock.
//   - imageID: ID of the container's image.
//
// Returns:
//   - *mockTypes.MockContainer: Mock that answers the container reads the digest code makes.
func newLocalOnlyContainer(t *testing.T, imageID string) *mockTypes.MockContainer {
	t.Helper()

	mc := mockTypes.NewMockContainer(t)
	mc.EXPECT().Name().Return("atlas-badgerdb-1").Maybe()
	mc.EXPECT().ImageName().Return("atlas-badgerdb:latest").Maybe()
	mc.EXPECT().HasImageInfo().Return(true).Maybe()
	mc.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{
		ID: imageID,
		RepoDigests: []string{
			"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
		},
	}).Maybe()
	mc.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{Image: "atlas-badgerdb"},
	}).Maybe()

	return mc
}

// clearLocalOnlyImageCache removes all cached local-only image entries, so
// each test starts without remembered local-only images.
func clearLocalOnlyImageCache() {
	localOnlyImageCache.Range(func(key, _ any) bool {
		localOnlyImageCache.Delete(key)

		return true
	})
}

// unauthorizedManifestClient returns a registry client that answers every
// manifest request with 401, which asks for a retry on the other host.
func unauthorizedManifestClient(t *testing.T) *mockAuth.MockClient {
	t.Helper()

	mc := mockAuth.NewMockClient(t)
	mc.EXPECT().Do(mock.Anything).Return(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		Header:     http.Header{},
		Body:       http.NoBody,
		Request:    &http.Request{URL: mustParseURL("https://registry.example.com/v2/manifests/latest")},
	}, nil)

	return mc
}

// mustParseURL parses raw and panics when it is not a valid URL.
//
// Parameters:
//   - raw: URL to parse.
//
// Returns:
//   - *url.URL: Parsed URL.
func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("failed to parse URL %q: %v", raw, err))
	}

	return u
}

// TestNormalizeDigest verifies that the sha256: prefix is stripped and any other
// digest is returned unchanged.
func TestNormalizeDigest(t *testing.T) {
	t.Parallel()

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
		{
			name:     "digest without prefix is unchanged",
			digest:   "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547",
			expected: "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NormalizeDigest(newTestLogger(t), tt.digest)
			assert.Equal(t, tt.expected, result, "NormalizeDigest(%q)", tt.digest)
		})
	}
}

// TestDigestsMatch verifies matching a remote digest against local repo
// digests, including prefixes, multiple digests, and malformed entries.
func TestDigestsMatch(t *testing.T) {
	t.Parallel()

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
		{
			name:         "skips local digest with nothing after @",
			localDigests: []string{"repo@"},
			remoteDigest: testDigestHash,
			expected:     false,
		},
		{
			name:         "does not match a local digest without the remote hash",
			localDigests: []string{"repo@singlepart"},
			remoteDigest: testDigestHash,
			expected:     false,
		},
		{
			name:         "does not match a local digest with multiple @ separators",
			localDigests: []string{"repo@namespace@" + testDigestHash},
			remoteDigest: testDigestHash,
			expected:     false,
		},
		{
			name:         "multiple local digests with no match",
			localDigests: []string{"repo1@sha256:hash1", "repo2@sha256:hash2"},
			remoteDigest: "sha256:hash3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := digestsMatch(newTestLogger(t), tt.localDigests, tt.remoteDigest)
			assert.Equal(t, tt.expected, result, "digestsMatch(%v, %q)", tt.localDigests, tt.remoteDigest)
		})
	}
}

// TestFormatDigest verifies that a bare hash gains the sha256: prefix and a
// prefixed digest is kept as it is.
func TestFormatDigest(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			assert.Equal(t, tt.want, formatDigest(tt.input))
		})
	}
}

// TestCompareDigest verifies digest comparison for containers without image
// info, locally built images, and anonymous registries that answer the HEAD
// request or only a GET request.
func TestCompareDigest(t *testing.T) {
	t.Parallel()

	const (
		manifestPath = "/v2/myapp/manifests/latest"
		localHash    = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	)

	tests := []struct {
		name string
		// container returns the container to compare.
		container func(t *testing.T) *mockTypes.MockContainer
		// handlers returns the registry's handlers in request order.
		handlers func(t *testing.T) []http.HandlerFunc
		expected bool
		wantErr  bool
	}{
		{
			name: "returns error when container has no image info",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()

				mc := mockTypes.NewMockContainer(t)
				mc.EXPECT().Name().Return("test-container")
				mc.EXPECT().ImageName().Return("nginx:latest")
				mc.EXPECT().HasImageInfo().Return(false)

				return mc
			},
			expected: false,
			wantErr:  true,
		},
		{
			name: "returns true for locally built image with empty RepoDigests",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()

				return newRegistryContainer(t, "local-image:latest")
			},
			expected: true,
		},
		{
			name: "matching digests via HEAD request",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()

				return newRegistryContainer(t, "registry.example.com/myapp:latest", "registry.example.com/myapp@"+localHash)
			},
			handlers: func(t *testing.T) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					anonymousRegistryHandler(t),
					manifestHandler(t, http.MethodHead, manifestPath, http.StatusOK, digestHeader(localHash)),
				}
			},
			expected: true,
		},
		{
			name: "non-matching digests via HEAD request",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()

				return newRegistryContainer(t, "registry.example.com/myapp:latest",
					"registry.example.com/myapp@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
			},
			handlers: func(t *testing.T) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					anonymousRegistryHandler(t),
					manifestHandler(t, http.MethodHead, manifestPath, http.StatusOK,
						digestHeader("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")),
				}
			},
			expected: false,
		},
		{
			name: "falls back to GET when HEAD returns non-2xx without 404",
			container: func(t *testing.T) *mockTypes.MockContainer {
				t.Helper()

				return newRegistryContainer(t, "registry.example.com/myapp:latest", "registry.example.com/myapp@"+localHash)
			},
			handlers: func(t *testing.T) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					anonymousRegistryHandler(t),
					manifestHandler(t, http.MethodHead, manifestPath, http.StatusInternalServerError, nil),
					anonymousRegistryHandler(t),
					manifestHandler(t, http.MethodGet, manifestPath, http.StatusOK, digestHeader(localHash)),
				}
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httpstub.New(t)
			if tt.handlers != nil {
				server.AppendHandlers(tt.handlers(t)...)
			}

			got, err := CompareDigest(newTestLogger(t), context.Background(),
				tt.container(t), "", registryConfig.Registry{}, ratelimit.New(), server.URL())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestCompareDigest_StubRegistry verifies digest comparison against a plain HTTP
// registry, from the /v2/ challenge through the manifest HEAD request.
func TestCompareDigest_StubRegistry(t *testing.T) {
	t.Parallel()

	const manifestPath = "/v2/test/image/manifests/latest"

	tests := []struct {
		name string
		// handlers returns the registry's handlers in request order.
		handlers func(t *testing.T, addr string) []http.HandlerFunc
		want     bool
		// errContains is the expected error text, empty when no error is expected.
		errContains string
		// wantRequests is the number of requests the registry must receive,
		// or zero when the count depends on transport retries.
		wantRequests int
	}{
		{
			name: "should return true when HEAD request succeeds with matching digest",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
					),
				)
			},
			want:         true,
			wantRequests: 3,
		},
		{
			name: "should send the Watchtower user agent on the HEAD request",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
						httpstub.VerifyHeader(t, http.Header{"User-Agent": {meta.UserAgent}}),
						httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
					),
				)
			},
			want:         true,
			wantRequests: 3,
		},
		{
			name: "should not fall back to GET when HEAD returns 404",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusNotFound, "", nil),
					),
				)
			},
			errContains:  errInvalidRegistryResponse.Error(),
			wantRequests: 3,
		},
		{
			name: "should return an error if HEAD request fails",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				dropConnection := httpstub.CombineHandlers(
					httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
					func(w http.ResponseWriter, _ *http.Request) {
						conn, _, err := http.NewResponseController(w).Hijack()
						if assert.NoError(t, err) {
							conn.Close()
						}
					},
				)

				// The HTTP transport retries a HEAD request once when the
				// kept-alive connection it reused fails, so both attempts drop.
				// A GET fallback would be unhandled and fail the test.
				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					dropConnection, dropConnection)
			},
			errContains: "EOF",
		},
		{
			name: "should handle malformed WWW-Authenticate header",
			handlers: func(t *testing.T, _ string) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
						httpstub.RespondWith(http.StatusUnauthorized, "", http.Header{
							"WWW-Authenticate": {`Bearer realm="invalid"`},
						}),
					),
				}
			},
			errContains:  "challenge header did not include all values needed to construct an auth url",
			wantRequests: 1,
		},
		{
			name: "should fetch digest from HTTP registry with TLS skip",
			handlers: func(t *testing.T, _ string) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					anonymousRegistryHandler(t),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
					),
				}
			},
			want:         true,
			wantRequests: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httpstub.New(t)
			server.AppendHandlers(tt.handlers(t, server.Addr())...)

			container := newRegistryContainer(t, server.Addr()+"/test/image:latest", testRepoDigest)

			got, err := CompareDigest(newTestLogger(t), context.Background(),
				container, "token", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)

			if tt.wantRequests > 0 {
				assert.Len(t, server.ReceivedRequests(), tt.wantRequests)
			}
		})
	}
}

// TestCompareDigest_RedirectedLSCRImage verifies that an lscr.io image whose
// registry redirects the /v2/ challenge to another host still compares its
// digest with a HEAD request.
func TestCompareDigest_RedirectedLSCRImage(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)
	redirectServer := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
			httpstub.RespondWith(http.StatusFound, "", http.Header{
				"Location": {fmt.Sprintf("http://%s/v2/", redirectServer.Addr())},
			}),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/lscr.io/test/image/manifests/latest")),
			httpstub.RespondWith(http.StatusNotFound, "", nil),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path("/v2/lscr.io/test/image/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
		),
	)
	redirectServer.AppendHandlers(
		append(bearerChallengeHandlers(t, redirectServer.Addr(), "repository:lscr.io/test/image:pull"),
			httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
		)...,
	)

	container := newRegistryContainer(t, server.Addr()+"/lscr.io/test/image:latest", testRepoDigest)

	got, err := CompareDigest(newTestLogger(t), context.Background(),
		container, "token", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
	require.NoError(t, err)
	assert.True(t, got)
}

// TestCompareDigestWithRemote verifies that a registry digest that differs from
// the local one is a mismatch and is returned as the remote digest.
func TestCompareDigestWithRemote(t *testing.T) {
	t.Parallel()

	const remoteHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	server := httpstub.New(t)
	server.AppendHandlers(
		anonymousRegistryHandler(t),
		manifestHandler(t, http.MethodHead, "/v2/myapp/manifests/latest", http.StatusOK, digestHeader("sha256:"+remoteHash)),
	)

	container := newRegistryContainer(t, server.Addr()+"/myapp:latest",
		"registry.example.com/myapp@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	match, remoteDigest, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", registryConfig.Registry{}, ratelimit.New(), server.URL())
	require.NoError(t, err)
	assert.False(t, match)
	assert.Equal(t, "sha256:"+remoteHash, remoteDigest)
}

// TestCompareDigestWithRemote_Head429DoesNotFallBackToGET verifies that a rate
// limited HEAD request returns the rate limit error instead of retrying the
// manifest with GET, which would spend more of the registry's quota.
func TestCompareDigestWithRemote_Head429DoesNotFallBackToGET(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)
	server.AppendHandlers(
		anonymousRegistryHandler(t),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path("/v2/linuxserver/radarr/manifests/latest")),
			httpstub.RespondWith(http.StatusTooManyRequests,
				"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute",
				http.Header{"Retry-After": {"7200"}}),
		),
	)

	container := newRegistryContainer(t, server.Addr()+"/linuxserver/radarr:latest",
		"lscr.io/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	match, _, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", registryConfig.Registry{TLSSkip: true}, ratelimit.New(), server.URL())
	require.ErrorIs(t, err, ratelimit.ErrRateLimited)
	assert.False(t, match)
}

// TestCompareDigestWithRemote_Retry429DoesNotContinueToNextEndpoint is a
// regression test for a 429 on the challenge-host retry. That error must abort
// the endpoint loop instead of trying the next mirror.
func TestCompareDigestWithRemote_Retry429DoesNotContinueToNextEndpoint(t *testing.T) {
	t.Parallel()

	const manifestPath = "/v2/linuxserver/radarr/manifests/latest"

	primary := httpstub.New(t)
	primary.AppendHandlers(
		append(bearerChallengeHandlers(t, primary.Addr(), "repository:linuxserver/radarr:pull"),
			manifestHandler(t, http.MethodHead, manifestPath, http.StatusUnauthorized, nil),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path(manifestPath)),
				httpstub.RespondWith(http.StatusTooManyRequests,
					"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute",
					http.Header{"Retry-After": {"7200"}}),
			),
		)...,
	)

	// The mirror has no handlers, so any request to it fails the test.
	mirror := httpstub.New(t)

	container := newRegistryContainer(t, primary.Addr()+"/linuxserver/radarr:latest",
		"lscr.io/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	match, _, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", registryConfig.Registry{TLSSkip: true}, ratelimit.New(), primary.URL(), mirror.URL())
	require.ErrorIs(t, err, ratelimit.ErrRateLimited)
	assert.False(t, match)
	assert.Len(t, primary.ReceivedRequests(), 4)
	assert.Empty(t, mirror.ReceivedRequests())
}

// TestCompareDigestWithRemote_UnauthorizedOnBothHostsIsError verifies that a
// registry that rejects the manifest request on both its own host and its
// token host produces an error, not an empty digest that marks the image as
// out of date and causes a pull.
func TestCompareDigestWithRemote_UnauthorizedOnBothHostsIsError(t *testing.T) {
	t.Parallel()

	const (
		manifestPath = "/v2/linuxserver/radarr/manifests/latest"
		scope        = "repository:linuxserver/radarr:pull"
	)

	// The token host issues tokens but rejects manifest requests.
	tokenHost := httpstub.New(t)
	tokenHost.AppendHandlers(
		tokenHandler(t),
		manifestHandler(t, http.MethodHead, manifestPath, http.StatusUnauthorized, nil),
		manifestHandler(t, http.MethodGet, manifestPath, http.StatusUnauthorized, nil),
	)

	// The registry sends clients to the token host and rejects manifest requests.
	registry := httpstub.New(t)
	registry.AppendHandlers(
		bearerChallengeHandler(t, tokenHost.Addr(), scope),
		manifestHandler(t, http.MethodHead, manifestPath, http.StatusUnauthorized, nil),
		bearerChallengeHandler(t, tokenHost.Addr(), scope),
		manifestHandler(t, http.MethodGet, manifestPath, http.StatusUnauthorized, nil),
	)

	container := newRegistryContainer(t, registry.Addr()+"/linuxserver/radarr:latest",
		registry.Addr()+"/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	match, remoteDigest, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
	require.ErrorIs(t, err, errUnresolvedManifestRetry)
	assert.False(t, match)
	assert.Empty(t, remoteDigest)
}

// TestCompareDigestWithRemote_HeadRejectedFallsBackToGet verifies that when a
// registry that is its own token host rejects HEAD manifest requests, the
// retried HEAD falls back to GET, which returns the digest.
func TestCompareDigestWithRemote_HeadRejectedFallsBackToGet(t *testing.T) {
	t.Parallel()

	const (
		manifestPath = "/v2/linuxserver/radarr/manifests/latest"
		remote       = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	registry := httpstub.New(t)
	registry.AppendHandlers(
		append(bearerChallengeHandlers(t, registry.Addr(), "repository:linuxserver/radarr:pull"),
			manifestHandler(t, http.MethodHead, manifestPath, http.StatusUnauthorized, nil),
			manifestHandler(t, http.MethodHead, manifestPath, http.StatusUnauthorized, nil),
			bearerChallengeHandler(t, registry.Addr(), "repository:linuxserver/radarr:pull"),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
				httpstub.RespondWith(http.StatusOK, `{"schemaVersion":2}`, http.Header{
					ContentDigestHeader: {remote},
					"Content-Type":      {"application/vnd.oci.image.index.v1+json"},
				}),
			),
		)...,
	)

	container := newRegistryContainer(t, registry.Addr()+"/linuxserver/radarr:latest",
		registry.Addr()+"/linuxserver/radarr@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	match, remoteDigest, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
	require.NoError(t, err)
	assert.False(t, match)
	assert.Equal(t, remote, remoteDigest)
}

// TestCompareDigestWithRemote_LocalOnly404 is a regression test for locally built
// images (e.g. atlas-badgerdb) that have non-empty registry-less RepoDigests.
// A HEAD 404 must be treated as up-to-date with no error so callers skip the pull
// without logging "Digest retrieval failed, falling back to full pull".
func TestCompareDigestWithRemote_LocalOnly404(t *testing.T) {
	clearLocalOnlyImageCache()
	t.Cleanup(clearLocalOnlyImageCache)

	// A GET fallback after the 404 would be unhandled and fail the test.
	server := httpstub.New(t)
	server.AppendHandlers(
		anonymousRegistryHandler(t),
		manifestHandler(t, http.MethodHead, "/v2/library/atlas-badgerdb/manifests/latest", http.StatusNotFound, nil),
	)

	match, remoteDigest, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		newLocalOnlyContainer(t, ""), "", registryConfig.Registry{TLSSkip: true}, ratelimit.New(), server.Addr())
	require.NoError(t, err)
	assert.True(t, match)
	assert.Empty(t, remoteDigest)
	assert.Len(t, server.ReceivedRequests(), 2)
}

// TestIsLocalImageNotFound verifies domain-less Config.Image + errManifestNotFound
// identity matching (a prior dual-sentinel bug broke errors.Is).
func TestIsLocalImageNotFound(t *testing.T) {
	t.Parallel()

	t.Run("true for domain-less Config.Image with wrapped errManifestNotFound", func(t *testing.T) {
		t.Parallel()

		mc := mockTypes.NewMockContainer(t)
		mc.EXPECT().HasImageInfo().Return(true)
		mc.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "atlas-badgerdb"},
		})

		err := fmt.Errorf("%w: %w: status 404 Not Found", errManifestNotFound, errInvalidRegistryResponse)
		assert.True(t, isLocalImageNotFound(mc, err))
	})

	t.Run("true for domain-less image with version tag containing dots", func(t *testing.T) {
		t.Parallel()

		mc := mockTypes.NewMockContainer(t)
		mc.EXPECT().HasImageInfo().Return(true)
		mc.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"my-app@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "my-app:1.0"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.True(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for domain-ful Config.Image", func(t *testing.T) {
		t.Parallel()

		mc := mockTypes.NewMockContainer(t)
		mc.EXPECT().HasImageInfo().Return(true)
		mc.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "registry.example.com/app:latest"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.False(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for Hub short name with registry-qualified RepoDigests", func(t *testing.T) {
		t.Parallel()

		// Official images often keep Config.Image as "nginx" while RepoDigests
		// records docker.io/library/nginx@sha256:.... A transient 404 must not
		// mark them local-only for the process lifetime.
		mc := mockTypes.NewMockContainer(t)
		mc.EXPECT().HasImageInfo().Return(true)
		mc.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{
			RepoDigests: []string{
				"docker.io/library/nginx@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
			},
		})
		mc.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Image: "nginx:latest"},
		})

		err := fmt.Errorf("%w: status 404 Not Found", errManifestNotFound)
		assert.False(t, isLocalImageNotFound(mc, err))
	})

	t.Run("false for non-manifest errors", func(t *testing.T) {
		t.Parallel()

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
	container := newLocalOnlyContainer(t, "sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35")

	// Only the first comparison may reach the registry.
	server := httpstub.New(t)
	server.AppendHandlers(
		anonymousRegistryHandler(t),
		manifestHandler(t, http.MethodHead, "/v2/library/atlas-badgerdb/manifests/latest", http.StatusNotFound, nil),
	)

	// First call: probe registry, cache local-only.
	match, _, err := CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", settings, ratelimit.New(), server.Addr())
	require.NoError(t, err)
	assert.True(t, match)

	// Second call: cache hit, no additional request.
	match, _, err = CompareDigestWithRemote(newTestLogger(t), context.Background(),
		container, "", settings, ratelimit.New(), server.Addr())
	require.NoError(t, err)
	assert.True(t, match)
	assert.Len(t, server.ReceivedRequests(), 2, "cached local-only image must not re-probe registry")
}

// TestFetchDigestWithGet verifies digest retrieval from an anonymous registry's
// GET response header, JSON body, and plain text body, and the error for a
// response that carries no digest.
func TestFetchDigestWithGet(t *testing.T) {
	t.Parallel()

	const (
		manifestPath = "/v2/myapp/manifests/latest"
		localHash    = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	)

	tests := []struct {
		name string
		// manifest answers the GET manifest request.
		manifest http.HandlerFunc
		expected string
		wantErr  bool
	}{
		{
			name: "extracts digest from GET response header",
			manifest: httpstub.RespondWith(http.StatusOK,
				`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json"}`,
				http.Header{
					ContentDigestHeader: {localHash},
					"Content-Type":      {"application/vnd.docker.distribution.manifest.v2+json"},
				}),
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name: "extracts digest from GET response JSON body",
			manifest: httpstub.RespondWith(http.StatusOK,
				fmt.Sprintf(`{"digest":%q}`, localHash),
				http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}}),
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:     "extracts digest from plain text body",
			manifest: httpstub.RespondWith(http.StatusOK, localHash, http.Header{"Content-Type": {"text/plain"}}),
			expected: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:     "returns error when no digest header and empty body",
			manifest: httpstub.RespondWith(http.StatusOK, "", nil),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httpstub.New(t)
			server.AppendHandlers(
				anonymousRegistryHandler(t),
				httpstub.CombineHandlers(
					httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
					tt.manifest,
				),
			)

			container := newRegistryContainer(t, "registry.example.com/myapp:latest", "registry.example.com/myapp@"+localHash)

			got, err := fetchDigestWithGet(newTestLogger(t), context.Background(),
				container, "", registryConfig.Registry{}, ratelimit.New(), server.URL())
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

// TestFetchDigestWithGet_StubRegistry verifies digest retrieval with a GET
// request against a plain HTTP registry, and the failures that happen before
// any request is made.
func TestFetchDigestWithGet_StubRegistry(t *testing.T) {
	t.Parallel()

	const manifestPath = "/v2/test/image/manifests/latest"

	tests := []struct {
		name string
		// handlers returns the registry's handlers in request order.
		handlers func(t *testing.T, addr string) []http.HandlerFunc
		// image returns the image reference the container runs.
		image func(addr string) string
		// repoDigests are the image's repo digests.
		repoDigests []string
		want        string
		// errContains is the expected error text, empty when no error is expected.
		errContains string
	}{
		{
			name: "should fetch a digest successfully",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
					),
				)
			},
			want: NormalizeDigest(newNopLogger(), testDigestHash),
		},
		{
			name: "should handle no authentication required",
			handlers: func(t *testing.T, _ string) []http.HandlerFunc {
				t.Helper()

				return []http.HandlerFunc{
					anonymousRegistryHandler(t),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
					),
				}
			},
			want: NormalizeDigest(newNopLogger(), testDigestHash),
		},
		{
			name: "should return an error if GET request fails after token",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusInternalServerError, "server error", nil),
					),
				)
			},
			errContains: errInvalidRegistryResponse.Error(),
		},
		{
			name: "should handle plain text 404 responses (non-JSON body)",
			handlers: func(t *testing.T, addr string) []http.HandlerFunc {
				t.Helper()

				return append(bearerChallengeHandlers(t, addr, "repository:test/image:pull"),
					httpstub.CombineHandlers(
						httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
						httpstub.RespondWith(http.StatusOK, "404 Not Found", nil),
					),
				)
			},
			errContains: "invalid digest format in body",
		},
		{
			name:        "should return an error if manifest URL build fails",
			image:       func(string) string { return "example.com/test/image:" },
			errContains: errFailedParseImageName.Error(),
		},
		{
			name:        "should handle URL parsing failure",
			image:       func(string) string { return "http://invalid url with spaces/test/image:latest" },
			errContains: errFailedBuildManifestURL.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httpstub.New(t)
			if tt.handlers != nil {
				server.AppendHandlers(tt.handlers(t, server.Addr())...)
			}

			image := server.Addr() + "/test/image:latest"
			if tt.image != nil {
				image = tt.image(server.Addr())
			}

			container := newRegistryContainer(t, image, testRepoDigest)

			got, err := fetchDigestWithGet(newTestLogger(t), context.Background(),
				container, "token", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFetchDigestWithGet_LocalImages verifies that locally built images, which
// have no repo digests, skip the registry for any registry host.
func TestFetchDigestWithGet_LocalImages(t *testing.T) {
	t.Parallel()

	for _, image := range []string{
		"localhost/test/image:latest",
		"local/test/image:latest",
		"127.0.0.1/test/image:latest",
	} {
		t.Run(image, func(t *testing.T) {
			t.Parallel()

			got, err := fetchDigestWithGet(newTestLogger(t), context.Background(),
				newRegistryContainer(t, image), "", registryConfig.Registry{}, ratelimit.New())
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}
}

// TestFetchDigestWithGet_RedirectedChallenge verifies that when the /v2/
// challenge redirects to another host, the manifest request goes to that host.
func TestFetchDigestWithGet_RedirectedChallenge(t *testing.T) {
	t.Parallel()

	const manifestPath = "/v2/test/image/manifests/latest"

	server := httpstub.New(t)
	redirectServer := httpstub.New(t)

	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
			httpstub.RespondWith(http.StatusFound, "", http.Header{
				"Location": {fmt.Sprintf("http://%s/v2/", redirectServer.Addr())},
			}),
		),
	)
	redirectServer.AppendHandlers(
		append(bearerChallengeHandlers(t, redirectServer.Addr(), "repository:test/image:pull"),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path(manifestPath)),
				httpstub.RespondWith(http.StatusOK, "", http.Header{ContentDigestHeader: {testDigestHash}}),
			),
		)...,
	)

	container := newRegistryContainer(t, server.Addr()+"/test/image:latest", testRepoDigest)

	got, err := fetchDigestWithGet(newTestLogger(t), context.Background(),
		container, "token", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
	require.NoError(t, err)
	assert.Equal(t, "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547", got)
}

// TestFetchDigestWithGet_SlowResponse verifies that a manifest request the
// registry does not answer before the context deadline fails with the deadline
// error instead of waiting for the registry.
func TestFetchDigestWithGet_SlowResponse(t *testing.T) {
	t.Parallel()

	server := httpstub.New(t)
	server.AppendHandlers(
		append(bearerChallengeHandlers(t, server.Addr(), "repository:test/image:pull"),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/test/image/manifests/latest")),
				func(_ http.ResponseWriter, r *http.Request) {
					// Answer only once the client gives up on the request.
					<-r.Context().Done()
				},
			),
		)...,
	)

	container := newRegistryContainer(t, server.Addr()+"/test/image:latest", testRepoDigest)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	_, err := fetchDigestWithGet(newTestLogger(t), ctx,
		container, "token", registryConfig.Registry{TLSSkip: true}, ratelimit.New())
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestBuildManifestURL verifies the manifest URL and registry host built for an
// image, with the TLS setting, host overrides, and the lscr.io redirect.
func TestBuildManifestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tlsSkip      bool
		image        string
		hostOverride string
		expectedURL  string
		expectedHost string
	}{
		{
			name:         "builds HTTPS manifest URL with empty override",
			image:        "nginx:latest",
			expectedURL:  "https://index.docker.io/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
		},
		{
			name:         "builds HTTP manifest URL when TLS is skipped",
			tlsSkip:      true,
			image:        "nginx:latest",
			expectedURL:  "http://index.docker.io/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
		},
		{
			name:         "overrides host with bare host string",
			image:        "nginx:latest",
			hostOverride: "mirror.example.com",
			expectedURL:  "https://mirror.example.com/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
		},
		{
			name:         "overrides host and scheme from full URL",
			image:        "nginx:latest",
			hostOverride: "http://mirror.example.com:5000",
			expectedURL:  "http://mirror.example.com:5000/v2/library/nginx/manifests/latest",
			expectedHost: "index.docker.io",
		},
		{
			name:         "redirects lscr.io to ghcr.io",
			image:        "lscr.io/linuxserver/radarr:latest",
			expectedURL:  "https://ghcr.io/v2/linuxserver/radarr/manifests/latest",
			expectedHost: "lscr.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			settings := registryConfig.Registry{TLSSkip: tt.tlsSkip}

			gotURL, gotHost, _, err := buildManifestURL(newTestLogger(t), newRegistryContainer(t, tt.image), settings, tt.hostOverride)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, gotURL)
			assert.Equal(t, tt.expectedHost, gotHost)
		})
	}
}

// TestHandleManifestResponse verifies how manifest responses are turned into a
// digest, a retry on another host, or an error.
func TestHandleManifestResponse(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			resp := tt.setupResponse()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			gotDigest, gotURL, gotRetry, err := handleManifestResponse(newTestLogger(t), resp,
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

// TestExtractHeadDigest verifies digest extraction from a HEAD response's
// Docker-Content-Digest header and the error when it is missing.
func TestExtractHeadDigest(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			resp := tt.setupResp()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			got, err := extractHeadDigest(newTestLogger(t), resp)
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

// TestExtractGetDigest verifies digest extraction from a GET response's header,
// JSON body, or plain text body, and the errors for responses without one.
func TestExtractGetDigest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupResp func() *http.Response
		expected  string
		wantErr   bool
		// errContains is the expected error text, empty to skip the check.
		errContains string
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
					Body:       io.NopCloser(iotest.ErrReader(errors.New("simulated read error"))),
				}
			},
			expected:    "",
			wantErr:     true,
			errContains: "failed to read response body",
		},
		{
			name: "returns error when plain text body is not a digest",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("invalid-digest-format")),
				}
			},
			wantErr:     true,
			errContains: "invalid digest format in body",
		},
		{
			name: "prefers the digest header over an unparseable body",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Docker-Content-Digest": []string{testDigestHash}},
					Body:       io.NopCloser(strings.NewReader("invalid-json")),
				}
			},
			expected: NormalizeDigest(newNopLogger(), testDigestHash),
		},
		{
			name: "extracts digest from JSON body with application/json content type",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"digest": "%s"}`, testDigestHash))),
				}
			},
			expected: NormalizeDigest(newNopLogger(), testDigestHash),
		},
		{
			name: "extracts digest from OCI image index JSON body",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/vnd.oci.image.index.v1+json"}},
					Body: io.NopCloser(strings.NewReader(
						`{"digest": "sha256:ociindexdigest123456789012345678901234567890123456789012345678901234567890"}`,
					)),
				}
			},
			expected: "ociindexdigest123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name: "returns error when JSON body has no digest field",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"other_field": "value"}`)),
				}
			},
			wantErr:     true,
			errContains: "empty digest in JSON manifest",
		},
		{
			name: "rejects JSON body without content type",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(`{"digest": "sha256:missingcontenttypedigest"}`)),
				}
			},
			wantErr:     true,
			errContains: "unsupported content type for JSON parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := tt.setupResp()
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			got, err := extractGetDigest(newTestLogger(t), resp)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)

				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestExtractGetDigest_ManifestSizeLimits verifies that a manifest body may be
// exactly maxManifestSize long and no longer.
func TestExtractGetDigest_ManifestSizeLimits(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			body := digestPrefix + strings.Repeat("a", tc.size-len(digestPrefix))
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}

			t.Cleanup(func() { _ = resp.Body.Close() })

			got, err := extractGetDigest(newTestLogger(t), resp)
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

// TestMakeManifestRequest verifies the method, authorization, Accept, and
// User-Agent of manifest requests, and the error for an invalid URL.
func TestMakeManifestRequest(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			ctx := context.Background()
			req, err := makeManifestRequest(ctx, tt.method, tt.manifestURL, tt.token)
			tt.verify(t, req, err)
		})
	}
}

// TestRetryManifestRequest verifies the outcome of retrying a manifest request
// on another host: a digest, a GET fallback, or an error.
func TestRetryManifestRequest(t *testing.T) {
	t.Parallel()

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
				mc.EXPECT().Do(mock.Anything).Return(&http.Response{
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
				mc.EXPECT().Do(mock.Anything).Return(nil, errors.New("connection refused"))

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
				mc.EXPECT().Do(mock.Anything).Return(&http.Response{
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
				mc.EXPECT().Do(mock.Anything).Return(&http.Response{
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
				mc.EXPECT().Do(mock.Anything).Return(&http.Response{
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
			t.Parallel()

			client := tt.setupClient(t)

			challengeHost := cmp.Or(tt.challengeHost, "ghcr.io")
			currentURL := cmp.Or(tt.currentURL, "https://registry.example.com/v2/manifests/latest")

			got, err := retryManifestRequest(newTestLogger(t), context.Background(),
				tt.method,
				tt.updatedURL,
				tt.token,
				"registry.example.com",
				challengeHost,
				false,
				mustParseURL(currentURL),
				"",
				client,
				ratelimit.New(),
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
