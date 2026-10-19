package digest_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
	"github.com/nicholas-fedor/watchtower/pkg/registry/digest"
	"github.com/nicholas-fedor/watchtower/pkg/registry/manifest"
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

// mockT is the test handle a generated mock needs. Both *testing.T and
// ginkgo.GinkgoT() satisfy it.
type mockT interface {
	mock.TestingT
	Cleanup(cleanup func())
}

// newMockContainer returns a mock container with the given name, image
// reference, and image inspect data.
//
// Parameters:
//   - t: Test that owns the mock.
//   - name: Container name.
//   - image: Image reference the container runs.
//   - imageInfo: Image inspect data, or nil when the image could not be inspected.
//
// Returns:
//   - *mockTypes.MockContainer: Mock that answers the container reads the digest code makes.
func newMockContainer(
	t mockT,
	name, image string,
	imageInfo *dockerImage.InspectResponse,
) *mockTypes.MockContainer {
	container := mockTypes.NewMockContainer(t)
	container.EXPECT().Name().Return(name).Maybe()
	container.EXPECT().ImageName().Return(image).Maybe()
	container.EXPECT().HasImageInfo().Return(imageInfo != nil).Maybe()
	container.EXPECT().ImageInfo().Return(imageInfo).Maybe()
	container.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{Image: image},
	}).Maybe()

	return container
}

// newMockContainerWithDigest returns a mock container whose image has digest
// as its only repo digest.
//
// Parameters:
//   - t: Test that owns the mock.
//   - name: Container name.
//   - image: Image reference the container runs.
//   - repoDigest: The image's repo digest.
//
// Returns:
//   - *mockTypes.MockContainer: Mock that answers the container reads the digest code makes.
func newMockContainerWithDigest(t mockT, name, image, repoDigest string) *mockTypes.MockContainer {
	return newMockContainer(t, name, image, &dockerImage.InspectResponse{
		ID:          image,
		RepoDigests: []string{repoDigest},
	})
}

const (
	httpScheme     = "http"
	httpsScheme    = "https"
	mockDigestHash = "sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"
)

var (
	DockerHubCredentials = &types.RegistryCredentials{
		Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_USERNAME"),
		Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_PASSWORD"),
	}
	GHCRCredentials = &types.RegistryCredentials{
		Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
		Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
	}
)

// testAuthClient is a custom implementation of the [auth.Client] interface for testing.
type testAuthClient struct {
	client *http.Client
}

// failingReader is a mock io.Reader that always returns an error, used for testing io.ReadAll failures.
type failingReader struct{}

func TestDigest(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Digest Suite")
}

// testRegistry holds the registry request settings the suite passes to the
// digest functions. Specs set TLSSkip to reach plain HTTP test servers.
var testRegistry registryConfig.Registry

var _ = ginkgo.BeforeSuite(func() {
	// Start from the default settings so requests use the https scheme.
	testRegistry = registryConfig.Registry{}
	// Set log level to debug to ensure debug logs are executed for coverage
})

var _ = ginkgo.Describe("Digests", func() {
	// Predefined mock data for consistent test cases
	mockName := "mock-container"
	mockImage := "ghcr.io/k6io/operator:latest"
	mockDigest := "ghcr.io/k6io/operator@" + mockDigestHash
	mockDifferentDigest := "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	mockInvalidDigest := "invalid-digest" // Malformed digest for testing

	// newTestAuthClient creates a test auth.Client with optional TLS handshake timeout.
	newTestAuthClient := func(timeout ...time.Duration) auth.Client {
		transport := &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		}
		if len(timeout) > 0 {
			transport.TLSHandshakeTimeout = timeout[0]
		}

		return &testAuthClient{
			client: &http.Client{
				Transport: transport,
			},
		}
	}

	// extractHeadDigest replicates digest.extractHeadDigest logic
	extractHeadDigest := func(resp *http.Response) (string, error) {
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf(
				"registry responded with invalid HEAD request: status %q, auth: %q",
				resp.Status,
				resp.Header.Get("WWW-Authenticate"),
			)
		}

		digestHeader := resp.Header.Get(digest.ContentDigestHeader)
		if digestHeader == "" {
			return "", fmt.Errorf(
				"registry responded with invalid HEAD request: missing %s header",
				digest.ContentDigestHeader,
			)
		}

		return digest.NormalizeDigest(newNopLogger(), digestHeader), nil
	}

	// setupRemoteDigestTestServer appends standard registry auth handlers to an existing
	// TLS test server and performs a HEAD request to fetch a remote digest.
	// The container parameter must have an image ref that includes the server address
	// (e.g., serverAddr + "/test/image:latest").
	// It returns the extracted remote digest.
	setupRemoteDigestTestServer := func(
		server *ghttp.Server,
		container types.Container,
		remoteDigestHash string,
	) string {
		serverAddr := server.Addr()

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
					"WWW-Authenticate": []string{
						fmt.Sprintf(
							`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
							serverAddr,
						),
					},
				}),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token"),
				ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
				ghttp.RespondWith(http.StatusOK, nil, http.Header{
					digest.ContentDigestHeader: []string{remoteDigestHash},
				}),
			),
		)

		client := newTestAuthClient()
		ctx := context.Background()
		registryAuth := auth.TransformAuth(newNopLogger(), "token")
		result, err := auth.GetToken(newNopLogger(), ctx, container, registryAuth, client, testRegistry, "")
		token := result.Token

		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		url, err := manifest.BuildManifestURLForImage(newNopLogger(), container.ImageName(), container.Name(), getScheme())
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set(
			"Accept",
			"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
		)
		req.Header.Set("User-Agent", meta.UserAgent)

		resp, err := client.Do(req)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		defer resp.Body.Close()

		remoteDigest, err := extractHeadDigest(resp)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		return remoteDigest
	}

	ginkgo.When("a digest comparison is done", func() {
		ginkgo.It("should return true if digests match",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				creds := fmt.Sprintf("%s:%s", GHCRCredentials.Username, GHCRCredentials.Password)
				matches, err := digest.CompareDigest(newNopLogger(), context.Background(),
					newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImage, mockDigest),
					creds,
					testRegistry,
					ratelimit.New(),
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(matches).To(gomega.BeTrue())
			}),
		)

		ginkgo.It("should skip digest fetch and return true for local images (empty RepoDigests)", func() {
			// Verify that CompareDigest detects local images by empty RepoDigests
			// and returns true without making any HTTP requests.
			mockContainerLocal := newMockContainer(ginkgo.GinkgoT(), mockName, "local/test/image:latest", &dockerImage.InspectResponse{RepoDigests: []string{}})

			matches, err := digest.CompareDigest(newNopLogger(), context.Background(),
				mockContainerLocal,
				"",
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Local images should return true (treated as up-to-date) to avoid
			// unnecessary registry requests.
			gomega.Expect(matches).To(gomega.BeTrue())
		})

		ginkgo.It("should return false when DigestsMatch is called with empty local digests", func() {
			// Verify that DigestsMatch returns false when comparing empty local digests
			// against a remote digest (manual comparison scenario).
			server := ghttp.NewTLSServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerEmptyDigests := newMockContainer(ginkgo.GinkgoT(), mockName, mockImageRef, &dockerImage.InspectResponse{RepoDigests: []string{}})

			remoteDigest := setupRemoteDigestTestServer(
				server,
				mockContainerEmptyDigests,
				mockDigestHash,
			)

			matches := digest.DigestsMatch(newNopLogger(), mockContainerEmptyDigests.ImageInfo().RepoDigests,
				remoteDigest,
			)
			gomega.Expect(matches).To(gomega.BeFalse())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should return false if digests differ", func() {
			server := ghttp.NewTLSServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDifferentDigest},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			remoteDigest, err := extractHeadDigest(resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			matches := digest.DigestsMatch(newNopLogger(), mockContainerWithServer.ImageInfo().RepoDigests,
				remoteDigest,
			)
			gomega.Expect(matches).To(gomega.BeFalse())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should return an error if the registry isn't available", func() {
			mockImageRef := "unreachable.local/test/image:latest"
			mockContainerUnreachable := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerUnreachable, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("failed to execute challenge request"))
		})

		ginkgo.It("should return an error when container contains no image info", func() {
			matches, err := digest.CompareDigest(newNopLogger(), context.Background(),
				newMockContainer(ginkgo.GinkgoT(), mockName, mockImage, nil),
				"user:pass",
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(matches).To(gomega.BeFalse())
		})

		ginkgo.It("should return an error if manifest URL build fails", func() {
			defer ginkgo.GinkgoRecover()
			// Use an invalid reference to trigger an error, as GetToken fails before BuildManifestURL
			mockImageRef := "example.com/test/image:" // Missing tag, invalid format
			mockContainerInvalidImage := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerInvalidImage, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
		})

		ginkgo.It("should return an error if HEAD request creation fails", func() {
			defer ginkgo.GinkgoRecover()
			// Use an invalid reference, as GetToken fails before request creation
			mockImageRef := "example.com/test/image:latest\x00invalid"
			mockContainerInvalidURL := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerInvalidURL, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
		})

		ginkgo.It("should return an error if HEAD request fails", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewTLSServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					func(w http.ResponseWriter, _ *http.Request) {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							return
						}

						conn.Close()
					},
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			_, err = client.Do(req)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("EOF"))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should return an error if registry responds without digest header", func() {
			server := ghttp.NewTLSServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						"Www-Authenticate": []string{"Bearer realm=invalid"},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = extractHeadDigest(resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("registry responded with invalid HEAD request"))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should handle malformed local digests", func() {
			server := ghttp.NewTLSServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithInvalidDigest := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockInvalidDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDifferentDigest},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx,
				mockContainerWithInvalidDigest,
				registryAuth,
				client,
				testRegistry,
				"",
			)
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithInvalidDigest.ImageName(), mockContainerWithInvalidDigest.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			remoteDigest, err := extractHeadDigest(resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			matches := digest.DigestsMatch(newNopLogger(), mockContainerWithInvalidDigest.ImageInfo().RepoDigests,
				remoteDigest,
			)
			gomega.Expect(matches).To(gomega.BeFalse())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		// Test case: Verifies that CompareDigest returns an error when the registry responds with
		// a 401 status and a malformed WWW-Authenticate header, simulating a misconfigured registry.
		ginkgo.It("should handle malformed WWW-Authenticate header", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{`Bearer realm="invalid"`},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{`Bearer realm="invalid"`},
					}),
				),
			)

			client := &testAuthClient{
				client: &http.Client{},
			}

			ctx := context.Background()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("challenge header did not include all values needed to construct an auth url"))

			matches, err := digest.CompareDigest(newNopLogger(), ctx,
				mockContainerWithServer,
				registryAuth,
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("challenge header did not include all values needed to construct an auth url"))
			gomega.Expect(matches).To(gomega.BeFalse())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(2))
		})

		ginkgo.It("should not fall back to GET when HEAD returns 404", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusNotFound, nil, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			registryAuth := auth.TransformAuth(newNopLogger(), "token")

			// Test that CompareDigest does not fall back to GET for 404 and returns error
			matches, err := digest.CompareDigest(newNopLogger(), ctx,
				mockContainerWithServer,
				registryAuth,
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(matches).To(gomega.BeFalse())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should return error when both HEAD and GET fail", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusNotFound, nil, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusInternalServerError, "server error"),
				),
			)

			ctx := context.Background()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			registryAuth := auth.TransformAuth(newNopLogger(), "token")

			// Test that CompareDigest fails when HEAD returns 500 (non-404 error)
			_, err := digest.CompareDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("registry responded with invalid HEAD request"))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})
		ginkgo.It("should return true when HEAD request succeeds with matching digest", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			matches, err := digest.CompareDigest(newNopLogger(), ctx, mockContainer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(matches).To(gomega.BeTrue())
		})
	})

	ginkgo.When("using different registries", func() {
		ginkgo.It("should work with DockerHub",
			SkipIfCredentialsEmpty(DockerHubCredentials, func() {
				ginkgo.GinkgoT().
					Logf("DockerHubCredentials present: %v", DockerHubCredentials != nil)
			}),
		)
		ginkgo.It("should work with GitHub Container Registry", func() {
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				ginkgo.GinkgoT().Logf("GHCRCredentials present: %v", GHCRCredentials != nil)
			})
		})
	})

	ginkgo.When("sending a HEAD request", func() {
		var server *ghttp.Server

		ginkgo.BeforeEach(func() {
			defer ginkgo.GinkgoRecover()

			server = ghttp.NewTLSServer()
		})

		ginkgo.AfterEach(func() {
			defer ginkgo.GinkgoRecover()

			server.Close()
		})

		ginkgo.It("should use a custom user-agent", func() {
			defer ginkgo.GinkgoRecover()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			customUserAgent := "Watchtower/v0.0.0-unknown"

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyHeader(http.Header{
						"User-Agent": []string{customUserAgent},
					}),
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", customUserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			remoteDigest, err := extractHeadDigest(resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			matches := digest.DigestsMatch(newNopLogger(), mockContainerWithServer.ImageInfo().RepoDigests,
				remoteDigest,
			)
			gomega.Expect(matches).To(gomega.BeTrue())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should handle HEAD request with non-200 status", func() {
			defer ginkgo.GinkgoRecover()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusNotFound, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			// Test extractHeadDigest directly with non-200 status
			_, err = extractHeadDigest(resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("registry responded with invalid HEAD request"))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should handle extractHeadDigest with missing digest header", func() {
			defer ginkgo.GinkgoRecover()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						// Intentionally omit the digest header
						"Www-Authenticate": []string{"Bearer realm=invalid"},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			// Test extractHeadDigest directly with missing digest header
			_, err = extractHeadDigest(resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("registry responded with invalid HEAD request"))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})

		ginkgo.It("should handle extractHeadDigest with valid digest header", func() {
			defer ginkgo.GinkgoRecover()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			// Test extractHeadDigest directly with valid digest header
			headResult, err := extractHeadDigest(resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headResult).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(3))
		})
	})

	ginkgo.When("testing digest matching", func() {
		ginkgo.It("should handle malformed local digests without @ separator", func() {
			localDigests := []string{"malformed-digest"}
			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should handle local digests with empty parts after @", func() {
			localDigests := []string{"repo@"}
			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should handle local digests with only one part after @", func() {
			localDigests := []string{"repo@singlepart"}
			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should match when local digest has multiple @ separators", func() {
			localDigests := []string{
				"repo@namespace@" + mockDigestHash,
			}
			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse()) // Should not match due to malformed format
		})

		ginkgo.It("should handle empty local digests slice", func() {
			localDigests := []string{}
			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should handle nil local digests slice", func() {
			var localDigests []string

			remoteDigest := mockDigestHash
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should match when local digest includes repository prefix", func() {
			localDigests := []string{"repo@sha256:abc123"}
			remoteDigest := "sha256:abc123"
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should match when remote digest has no sha256 prefix", func() {
			localDigests := []string{"repo@sha256:abc123"}
			remoteDigest := "abc123"
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should not match when remote digest is empty", func() {
			localDigests := []string{"repo@sha256:abc123"}
			remoteDigest := ""
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should match when one of multiple local digests matches remote", func() {
			localDigests := []string{"repo1@sha256:hash1", "repo2@sha256:hash2"}
			remoteDigest := "sha256:hash1"
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should not match when none of multiple local digests match remote", func() {
			localDigests := []string{"repo1@sha256:hash1", "repo2@sha256:hash2"}
			remoteDigest := "sha256:hash3"
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should match when local digest has empty repo prefix", func() {
			localDigests := []string{"@sha256:abc123"}
			remoteDigest := "sha256:abc123"
			result := digest.DigestsMatch(newNopLogger(), localDigests, remoteDigest)
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.Describe("NormalizeDigest", func() {
			ginkgo.It("should trim sha256: prefix from digest", func() {
				input := mockDigestHash
				expected := "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"
				gomega.Expect(digest.NormalizeDigest(newNopLogger(), input)).To(gomega.Equal(expected))
			})

			ginkgo.It("should return unchanged digest without recognized prefix", func() {
				input := "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"
				gomega.Expect(digest.NormalizeDigest(newNopLogger(), input)).To(gomega.Equal(input))
			})

			ginkgo.It("should handle empty digest string", func() {
				input := ""
				gomega.Expect(digest.NormalizeDigest(newNopLogger(), input)).To(gomega.Equal(""))
			})

			ginkgo.It("should handle digest with unrecognized prefix", func() {
				input := "md5:1234567890abcdef"
				gomega.Expect(digest.NormalizeDigest(newNopLogger(), input)).To(gomega.Equal(input))
			})
		})

		ginkgo.It("should skip digest fetch for local registries", func() {
			// Locally built images have empty RepoDigests, which triggers
			// the skip path in fetchDigest without making any HTTP requests.
			mockContainerLocalhost := newMockContainer(ginkgo.GinkgoT(), mockName, "localhost/test/image:latest", &dockerImage.InspectResponse{RepoDigests: []string{}})

			result, err := digest.FetchDigest(newNopLogger(), context.Background(),
				mockContainerLocalhost,
				"",
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(""))

			mockContainerLocal := newMockContainer(ginkgo.GinkgoT(), mockName, "local/test/image:latest", &dockerImage.InspectResponse{RepoDigests: []string{}})

			result, err = digest.FetchDigest(newNopLogger(), context.Background(),
				mockContainerLocal,
				"",
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(""))

			mockContainerLoopback := newMockContainer(ginkgo.GinkgoT(), mockName, "127.0.0.1/test/image:latest", &dockerImage.InspectResponse{RepoDigests: []string{}})

			result, err = digest.FetchDigest(newNopLogger(), context.Background(),
				mockContainerLoopback,
				"",
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(""))
		})
	})

	ginkgo.When("transforming authentication", func() {
		ginkgo.It("should transform valid credentials into base64", func() {
			creds := struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}{
				Username: "testuser",
				Password: "testpass",
			}
			jsonData, _ := json.Marshal(creds)
			inputAuth := base64.StdEncoding.EncodeToString(jsonData)

			result := auth.TransformAuth(newNopLogger(), inputAuth)
			expected := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
			gomega.Expect(result).To(gomega.Equal(expected))
		})

		ginkgo.It("should return original input if decoding fails", func() {
			inputAuth := "invalid-base64-string"
			result := auth.TransformAuth(newNopLogger(), inputAuth)
			gomega.Expect(result).To(gomega.Equal(inputAuth))
		})

		ginkgo.It("should handle empty credentials", func() {
			creds := struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}{
				Username: "",
				Password: "",
			}
			jsonData, _ := json.Marshal(creds)
			inputAuth := base64.StdEncoding.EncodeToString(jsonData)

			result := auth.TransformAuth(newNopLogger(), inputAuth)
			gomega.Expect(result).To(gomega.Equal(inputAuth))
		})
	})

	ginkgo.When("fetching a digest", func() {
		var server *ghttp.Server

		ginkgo.BeforeEach(func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true

			server = ghttp.NewServer()
		})

		ginkgo.AfterEach(func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = false

			server.Close()
		})

		ginkgo.It("should fetch a digest successfully", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			scheme := "https"
			if testRegistry.TLSSkip {
				scheme = "http"
			}

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="%s://%s/token",service="test-service",scope="repository:test/image:pull"`,
								scheme,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
		})

		ginkgo.It("should return an error if GET request fails after token", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusInternalServerError, "server error"),
				),
			)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("registry responded with invalid HEAD request"))
		})

		// Test case: Verifies that GetToken returns an error when the registry is unreachable.
		ginkgo.It("should return an error if GetToken fails", func() {
			defer ginkgo.GinkgoRecover()

			mockImageRef := "unreachable.local/test/image:latest"
			mockContainerUnreachable := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerUnreachable, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.MatchRegexp("Temporary failure in name resolution|no such host|server misbehaving"))
		})

		ginkgo.It("should return an error if manifest URL build fails", func() {
			defer ginkgo.GinkgoRecover()
			// Use an invalid reference, as GetToken fails before BuildManifestURL
			mockImageRef := "example.com/test/image:" // Missing tag, invalid format
			mockContainerInvalidImage := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerInvalidImage, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
		})

		ginkgo.It("should return an error if GET request creation fails", func() {
			defer ginkgo.GinkgoRecover()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest\x00invalid"
			mockContainerInvalidURL := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			_, err := auth.GetToken(newNopLogger(), ctx, mockContainerInvalidURL, registryAuth, client, testRegistry, "")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
		})

		ginkgo.It("should return an error if response decoding fails", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, "invalid-json"),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("invalid digest format in body"))
		})

		ginkgo.It("should fall back to header when JSON decoding fails", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, "invalid-json", http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			getResult, err := digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(getResult).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
		})

		ginkgo.It("should parse JSON manifest for digest extraction", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, mockDigestHash),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			getResult, err := digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(getResult).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
		})

		ginkgo.It("should handle empty body error", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("missing digest header and empty body"))
		})

		ginkgo.It("should handle malformed JSON manifest", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, `invalid json`),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("invalid digest format in body"))
		})

		// Test case: Verifies that FetchDigest successfully retrieves a digest from an HTTP registry
		// with WATCHTOWER_REGISTRY_TLS_SKIP enabled, handling empty tokens as errors for unauthenticated registries.
		ginkgo.It("should fetch digest from HTTP registry with TLS skip", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusOK, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := digest.CompareDigest(newNopLogger(), ctx,
				mockContainerWithServer,
				registryAuth,
				testRegistry,
				ratelimit.New(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeTrue())
			gomega.Expect(server.ReceivedRequests()).Should(gomega.HaveLen(2))
		})

		ginkgo.It("should parse valid JSON manifest with digest field", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(
						http.StatusOK,
						fmt.Sprintf(`{"digest": "%s"}`, mockDigestHash),
						http.Header{
							"Content-Type": []string{"application/json"},
						},
					),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			getResult, err := digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(getResult).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
		})

		ginkgo.It("should handle JSON manifest with empty digest field", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, `{"digest": ""}`, http.Header{
						"Content-Type": []string{"application/json"},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("empty digest in JSON manifest"))
		})

		ginkgo.It("should handle JSON manifest without digest field", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, `{"other_field": "value"}`, http.Header{
						"Content-Type": []string{"application/json"},
					}),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("empty digest in JSON manifest"))
		})

		ginkgo.It("should handle invalid plain text digest format", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, "invalid-digest-format"),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("invalid digest format in body"))
		})

		ginkgo.It("should handle short plain text digest", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, "sha256:short"),
				),
			)

			client := newTestAuthClient()
			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := auth.GetToken(newNopLogger(), ctx, mockContainerWithServer, registryAuth, client, testRegistry, "")
			token := result.Token

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set(
				"Accept",
				"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
			)
			req.Header.Set("User-Agent", meta.UserAgent)

			resp, err := client.Do(req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			defer resp.Body.Close()

			_, err = digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("invalid digest format in body"))
		})

		ginkgo.It("should handle io.ReadAll failure in ExtractGetDigest", func() {
			defer ginkgo.GinkgoRecover()

			// Define a failing reader that returns an error on Read
			failingReader := &failingReader{}

			// Create a mock response with the failing body
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(failingReader),
			}

			_, err := digest.ExtractGetDigest(newNopLogger(), resp)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("failed to read response body"))
		})
	})

	ginkgo.When("fetching a digest with a redirecting registry", func() {
		ginkgo.It("should update the manifest URL host based on challenge response", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			server := ghttp.NewServer()
			defer server.Close()

			redirectServer := ghttp.NewServer()
			defer redirectServer.Close()

			serverAddr := server.Addr()
			redirectAddr := redirectServer.Addr()
			// Use actual redirect server address
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusFound, nil, http.Header{
						"Location": []string{fmt.Sprintf("http://%s/v2/", redirectAddr)},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusNotFound, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)
			redirectServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								redirectAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).
				To(gomega.Equal("d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"))
		})
		ginkgo.It("should conditionally update manifest URL host only when redirected", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			server := ghttp.NewServer()
			defer server.Close()

			redirectServer := ghttp.NewServer()
			defer redirectServer.Close()

			serverAddr := server.Addr()
			redirectAddr := redirectServer.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			// Test case 1: redirected=true, should update host
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusFound, nil, http.Header{
						"Location": []string{fmt.Sprintf("http://%s/v2/", redirectAddr)},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusNotFound, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)
			redirectServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								redirectAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).
				To(gomega.Equal("d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"))
		})

		ginkgo.It("should not update manifest URL host when not redirected", func() {
			defer ginkgo.GinkgoRecover()

			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			server := ghttp.NewServer()
			defer server.Close()

			serverAddr := server.Addr()
			mockImageRef := serverAddr + "/test/image:latest"
			mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

			// Test case 2: redirected=false, should not update host
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
								serverAddr,
							),
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{mockDigestHash},
					}),
				),
			)

			ctx := context.Background()
			registryAuth := auth.TransformAuth(newNopLogger(), "token")
			result, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
		})

		ginkgo.When("testing fetchDigest function directly", func() {
			var server *ghttp.Server

			ginkgo.BeforeEach(func() {
				defer ginkgo.GinkgoRecover()

				server = ghttp.NewServer()
			})

			ginkgo.AfterEach(func() {
				defer ginkgo.GinkgoRecover()

				server.Close()
			})

			ginkgo.It("should handle no authentication required", func() {
				defer ginkgo.GinkgoRecover()

				ctx := context.Background()

				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				httpServer := ghttp.NewServer()
				defer httpServer.Close()

				serverAddr := httpServer.Addr()
				mockImageRef := serverAddr + "/test/image:latest"
				mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

				httpServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
						ghttp.RespondWith(http.StatusOK, nil, http.Header{
							digest.ContentDigestHeader: []string{mockDigestHash},
						}),
					),
				)

				registryAuth := auth.TransformAuth(newNopLogger(), "")
				result, err := digest.FetchDigest(newNopLogger(), ctx,
					mockContainerWithServer,
					registryAuth,
					testRegistry,
					ratelimit.New(),
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(result).To(gomega.Equal(digest.NormalizeDigest(newNopLogger(), mockDigestHash)))
			})

			ginkgo.It("should handle invalid image reference", func() {
				defer ginkgo.GinkgoRecover()
				// Create a mock container with invalid image reference
				// This should cause manifest.BuildManifestURLForImage to fail
				mockContainerInvalid := newMockContainerWithDigest(
					ginkgo.GinkgoT(),
					mockName,
					"example.com/test/image:", // Missing tag, invalid format
					mockDigest,
				)

				client := newTestAuthClient()
				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				_, err := auth.GetToken(newNopLogger(), ctx, mockContainerInvalid, registryAuth, client, testRegistry, "")
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("failed to parse image name"))
			})

			ginkgo.It("should handle URL parsing failure", func() {
				defer ginkgo.GinkgoRecover()

				mockContainerInvalidURL := newMockContainerWithDigest(
					ginkgo.GinkgoT(),
					mockName,
					"http://invalid url with spaces/test/image:latest", // Invalid URL
					mockDigest,
				)

				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				_, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerInvalidURL, registryAuth, testRegistry, ratelimit.New())
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("failed to build manifest URL"))
			})

			ginkgo.It("should handle plain text 404 responses (non-JSON body)", func() {
				defer ginkgo.GinkgoRecover()

				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				serverAddr := server.Addr()
				mockImageRef := serverAddr + "/test/image:latest"
				mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
							"WWW-Authenticate": []string{
								fmt.Sprintf(
									`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
									serverAddr,
								),
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/token"),
						ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
						ghttp.RespondWith(http.StatusOK, "404 Not Found"),
					),
				)

				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				_, err := digest.FetchDigest(newNopLogger(), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("invalid digest format in body"))
			})

			ginkgo.It("should handle OCI image index responses with proper Content-Type", func() {
				defer ginkgo.GinkgoRecover()

				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				serverAddr := server.Addr()
				mockImageRef := serverAddr + "/test/image:latest"
				mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
							"WWW-Authenticate": []string{
								fmt.Sprintf(
									`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
									serverAddr,
								),
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/token"),
						ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
						ghttp.RespondWith(
							http.StatusOK,
							`{"digest": "sha256:ociindexdigest123456789012345678901234567890123456789012345678901234567890"}`,
							http.Header{
								"Content-Type": []string{"application/vnd.oci.image.index.v1+json"},
							},
						),
					),
				)

				client := newTestAuthClient()
				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				result, err := auth.GetToken(newNopLogger(), ctx,
					mockContainerWithServer,
					registryAuth,
					client,
					testRegistry,
					"",
				)
				token := result.Token

				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set(
					"Accept",
					"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
				)
				req.Header.Set("User-Agent", meta.UserAgent)

				resp, err := client.Do(req)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				defer resp.Body.Close()

				getResult, err := digest.ExtractGetDigest(newNopLogger(), resp)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(getResult).
					To(gomega.Equal("ociindexdigest123456789012345678901234567890123456789012345678901234567890"))
			})

			ginkgo.It("should handle invalid Content-Type headers", func() {
				defer ginkgo.GinkgoRecover()

				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				serverAddr := server.Addr()
				mockImageRef := serverAddr + "/test/image:latest"
				mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
							"WWW-Authenticate": []string{
								fmt.Sprintf(
									`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
									serverAddr,
								),
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/token"),
						ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
						ghttp.RespondWith(
							http.StatusOK,
							`{"digest": "sha256:invalidcontenttypedigest"}`,
							http.Header{
								"Content-Type": []string{"text/plain"},
							},
						),
					),
				)

				client := newTestAuthClient()
				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				result, err := auth.GetToken(newNopLogger(), ctx,
					mockContainerWithServer,
					registryAuth,
					client,
					testRegistry,
					"",
				)
				token := result.Token

				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set(
					"Accept",
					"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
				)
				req.Header.Set("User-Agent", meta.UserAgent)

				resp, err := client.Do(req)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				defer resp.Body.Close()

				_, err = digest.ExtractGetDigest(newNopLogger(), resp)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("unsupported content type for JSON parsing"))
			})

			ginkgo.It("should handle missing or malformed Content-Type headers", func() {
				defer ginkgo.GinkgoRecover()

				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				serverAddr := server.Addr()
				mockImageRef := serverAddr + "/test/image:latest"
				mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
							"WWW-Authenticate": []string{
								fmt.Sprintf(
									`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
									serverAddr,
								),
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/token"),
						ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/test/image/manifests/latest"),
						ghttp.RespondWith(
							http.StatusOK,
							`{"digest": "sha256:missingcontenttypedigest"}`,
						),
					),
				)

				client := newTestAuthClient()
				ctx := context.Background()
				registryAuth := auth.TransformAuth(newNopLogger(), "token")
				result, err := auth.GetToken(newNopLogger(), ctx,
					mockContainerWithServer,
					registryAuth,
					client,
					testRegistry,
					"",
				)
				token := result.Token

				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				url, err := manifest.BuildManifestURLForImage(newNopLogger(), mockContainerWithServer.ImageName(), mockContainerWithServer.Name(), getScheme())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set(
					"Accept",
					"application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json",
				)
				req.Header.Set("User-Agent", meta.UserAgent)

				resp, err := client.Do(req)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				defer resp.Body.Close()

				_, err = digest.ExtractGetDigest(newNopLogger(), resp)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("unsupported content type for JSON parsing"))
			})

			ginkgo.It(
				"should successfully use HEAD requests for lscr.io images when redirected",
				func() {
					defer ginkgo.GinkgoRecover()

					ctx := context.Background()

					testRegistry.TLSSkip = true
					defer func() { testRegistry.TLSSkip = false }()

					server := ghttp.NewServer()
					defer server.Close()

					redirectServer := ghttp.NewServer()
					defer redirectServer.Close()

					serverAddr := server.Addr()
					redirectAddr := redirectServer.Addr()
					// Use lscr.io image name to trigger special handling
					mockImageRef := serverAddr + "/lscr.io/test/image:latest"
					mockContainerWithServer := newMockContainerWithDigest(ginkgo.GinkgoT(), mockName, mockImageRef, mockDigest)

					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/v2/"),
							ghttp.RespondWith(http.StatusFound, nil, http.Header{
								"Location": []string{fmt.Sprintf("http://%s/v2/", redirectAddr)},
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/v2/lscr.io/test/image/manifests/latest"),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("HEAD", "/v2/lscr.io/test/image/manifests/latest"),
							ghttp.RespondWith(http.StatusOK, nil, http.Header{
								digest.ContentDigestHeader: []string{mockDigestHash},
							}),
						),
					)
					redirectServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/v2/"),
							ghttp.RespondWith(http.StatusUnauthorized, nil, http.Header{
								"WWW-Authenticate": []string{
									fmt.Sprintf(
										`Bearer realm="http://%s/token",service="test-service",scope="repository:lscr.io/test/image:pull"`,
										redirectAddr,
									),
								},
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/token"),
							ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
						),
						ghttp.RespondWith(http.StatusOK, nil, http.Header{
							digest.ContentDigestHeader: []string{mockDigestHash},
						}),
					)

					registryAuth := auth.TransformAuth(newNopLogger(), "token")
					result, err := digest.CompareDigest(newNopLogger(), ctx,
						mockContainerWithServer,
						registryAuth,
						testRegistry,
						ratelimit.New(),
					)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(result).To(gomega.BeTrue())
				},
			)
		})
	})
})

// SkipIfCredentialsEmpty skips a test if registry credentials are incomplete.
// It checks for empty username or password, skipping with a message, otherwise runs the test.
func SkipIfCredentialsEmpty(credentials *types.RegistryCredentials, testFunc func()) func() {
	switch {
	case credentials.Username == "":
		return func() { ginkgo.Skip("Username missing. Skipping integration test") }
	case credentials.Password == "":
		return func() { ginkgo.Skip("Password missing. Skipping integration test") }
	default:
		return testFunc
	}
}

func (t *testAuthClient) Do(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

func (f *failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

func TestDigestClient_GetManifest_SlowResponse(t *testing.T) {
	testRegistry.TLSSkip = true
	defer func() { testRegistry.TLSSkip = false }()

	mockName := "mock-container"
	mockDigest := "ghcr.io/k6io/operator@" + mockDigestHash

	server := ghttp.NewServer()
	defer server.Close()

	serverAddr := server.Addr()

	server.AppendHandlers(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v2/" {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer realm="http://%s/token",service="test-service",scope="repository:test/image:pull"`,
				serverAddr,
			))
			w.WriteHeader(http.StatusUnauthorized)
		},
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/token" {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token": "mock-token"}`))
		},
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/v2/test/image/manifests/latest" {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			time.Sleep(50 * time.Millisecond)
			w.Header().Set("Docker-Content-Digest", mockDigestHash)
			w.WriteHeader(http.StatusOK)
		},
	)

	mockImageRef := serverAddr + "/test/image:latest"
	mockContainerWithServer := newMockContainerWithDigest(t, mockName, mockImageRef, mockDigest)

	registryAuth := auth.TransformAuth(newTestLogger(t), "token")

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, err := digest.FetchDigest(newTestLogger(t), ctx, mockContainerWithServer, registryAuth, testRegistry, ratelimit.New())
	if err == nil {
		t.Fatal("expected error")
	}

	matched, _ := regexp.MatchString(
		`context deadline exceeded`,
		err.Error(),
	)
	if !matched {
		t.Fatal("error not matching:", err)
	}
}

func getScheme() string {
	if testRegistry.TLSSkip {
		return httpScheme
	}

	return httpsScheme
}
