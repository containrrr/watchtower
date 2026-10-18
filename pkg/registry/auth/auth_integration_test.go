// This file provides comprehensive tests for the registry authentication
// functionality in Watchtower.

package auth

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// testAuthClient is a custom implementation of the AuthClient interface for testing.
// It wraps an HTTP client with configurable TLS settings to bypass certificate
// verification in test scenarios involving mock TLS servers.
type testAuthClient struct {
	client *http.Client // The underlying HTTP client for making requests.
}

// TestAuth executes the registry authentication test suite using the Ginkgo
// testing framework. It registers Gomega's fail handler to report test failures
// and runs the full set of specifications defined in this file.
func TestAuth(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Registry Auth Suite")
}

// SkipIfCredentialsEmpty creates a test function that conditionally skips execution
// based on the presence of registry credentials. It checks if the username or password
// is empty, skipping the test with an appropriate message if either is missing, and
// otherwise returns the provided test function for execution.
func SkipIfCredentialsEmpty(credentials *types.RegistryCredentials, testFunc func()) func() {
	switch {
	case credentials.Username == "":
		return func() {
			ginkgo.Skip("Username missing. Skipping integration test")
		}
	case credentials.Password == "":
		return func() {
			ginkgo.Skip("Password missing. Skipping integration test")
		}
	default:
		return testFunc
	}
}

// Do executes an HTTP request using the underlying HTTP client.
//
// This method satisfies the AuthClient interface, delegating the request execution
// to the embedded HTTP client.
//
// Parameters:
//   - req: The HTTP request to execute.
//
// Returns:
//   - *http.Response: The HTTP response from the registry, if successful.
//   - error: Non-nil if the request fails, nil otherwise.
func (t *testAuthClient) Do(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

// newTLSServerWithMaxVersion creates a ghttp TLS server constrained to the given max version.
func newTLSServerWithMaxVersion(maxVersion uint16) *ghttp.Server {
	server := ghttp.NewUnstartedServer()
	server.HTTPTestServer.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: maxVersion,
	}
	server.HTTPTestServer.StartTLS()

	return server
}

var GHCRCredentials = &types.RegistryCredentials{
	Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
	Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
}

// testRegistry holds the registry request settings the suite passes to the
// auth functions. Specs change it to exercise TLS skip and TLS versions.
var testRegistry registryConfig.Registry

var _ = ginkgo.BeforeSuite(func() {
	// Start every suite run from the default registry settings.
	testRegistry = registryConfig.Registry{}
	// Suite setup (logger is constructed per-test where needed).
})

var _ = ginkgo.Describe("the auth module", func() {
	// mockImage is the default image name for the initial mock container, representing
	// a real-world registry image used in the bearer token test.
	const mockImage = "ghcr.io/k6io/operator:latest"

	// runBasicAuthTest is a helper function to reduce duplication in GetToken tests
	// that use a mock HTTPS server to simulate basic auth challenges.
	runBasicAuthTest := func(challengeHeader, creds, expectedToken, expectedErr string) {
		// Create a TLS test server to simulate the registry.
		server := ghttp.NewTLSServer()

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(
					http.StatusUnauthorized,
					"",
					http.Header{challengeHeaderName: []string{challengeHeader}},
				),
			),
		)
		defer server.Close()

		// Configure the container with the test server's address.
		serverURL, _ := url.Parse(server.URL())
		containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		containerInstance.On("ImageName").Return(serverURL.Host + "/test/image").Maybe()

		// Create an authentication client with TLS verification disabled for the mock server.
		client := &testAuthClient{
			client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						MinVersion:         tls.VersionTLS12,
					},
				},
			},
		}

		// Temporarily disable WATCHTOWER_REGISTRY_TLS_SKIP to ensure HTTPS scheme.
		originalTLSSkip := testRegistry.TLSSkip

		testRegistry.TLSSkip = false
		defer func() { testRegistry.TLSSkip = originalTLSSkip }()

		// Transform credentials to base64-encoded format as production code does.
		registryAuth := TransformAuth(newNopLogger(), creds)

		// Execute GetToken and verify the result.
		result, err := GetToken(newNopLogger(), context.Background(),
			containerInstance,
			registryAuth,
			client,
			testRegistry,
			"",
		)
		token := result.Token

		gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))

		if expectedErr != "" {
			gomega.Expect(err).
				To(gomega.HaveOccurred(), fmt.Sprintf("Expected an error for challenge '%s'", challengeHeader))
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring(expectedErr), fmt.Sprintf("Expected error to contain '%s'", expectedErr))
			gomega.Expect(token).To(gomega.Equal(""), "Expected empty token on failure")
		} else {
			gomega.Expect(err).
				NotTo(gomega.HaveOccurred(), "Expected no error when fetching basic auth token")
			gomega.Expect(token).
				To(gomega.Equal(expectedToken), fmt.Sprintf("Expected token to match '%s'", expectedToken))
		}
	}

	// runBearerHeaderTest is a helper function to reduce duplication in getBearerToken tests
	// that use a mock HTTPS server to simulate bearer token retrieval.
	runBearerHeaderTest := func(creds, expectedToken string, expectAuthFailure bool) {
		// Transform credentials to base64-encoded format as production code does.
		registryAuth := TransformAuth(newNopLogger(), creds)

		// Create a TLS test server to simulate the registry.
		server := ghttp.NewTLSServer()

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/"),
				func(w http.ResponseWriter, r *http.Request) {
					if expectAuthFailure {
						auth := r.Header.Get("Authorization")
						if auth != "Basic dXNlcjpwYXNz" {
							w.WriteHeader(http.StatusUnauthorized)

							return
						}
					}

					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"token": "%s"}`, expectedToken)
				},
			),
		)
		defer server.Close()

		// Create an authentication client with TLS verification disabled for the mock server.
		client := &testAuthClient{
			client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		}

		// Construct the challenge header for the bearer token request.
		challenge := fmt.Sprintf(
			`bearer realm="%s",service="test-service",scope="repository:test/image:pull"`,
			server.URL(),
		)
		ref, _ := reference.ParseNormalizedNamed("test/image")

		// Execute getBearerToken and verify the result.
		token, err := getBearerToken(newNopLogger(), context.Background(), challenge, ref, registryAuth, client, testRegistry)

		gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(token).To(gomega.Equal("Bearer " + expectedToken))
	}

	ginkgo.Describe("GetToken", func() {
		// Test case: Verifies that GetToken retrieves a bearer token successfully when
		// provided with valid GHCR credentials. This is an integration test that runs
		// only if credentials are present, ensuring real-world registry interaction.
		ginkgo.It("should parse the token from a bearer response",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				creds := fmt.Sprintf("%s:%s", GHCRCredentials.Username, GHCRCredentials.Password)
				mockContainerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				mockContainerInstance.On("ImageName").Return(mockImage).Maybe()

				client := NewAuthClient(nil, testRegistry)
				result, err := GetToken(newNopLogger(), context.Background(),
					mockContainerInstance,
					TransformAuth(newNopLogger(), base64.StdEncoding.EncodeToString([]byte(creds))),
					client,
					testRegistry,
					"",
				)
				token := result.Token

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(token).NotTo(gomega.Equal(""))
			}),
		)

		// Test case: Ensures GetToken returns a basic auth token when the registry
		// responds with a "Basic" challenge.
		ginkgo.It("should return basic auth token when challenged with basic", func() {
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			runBasicAuthTest("Basic realm=\"test\"", "eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InBhc3MifQ==", "Basic dXNlcjpwYXNz", "")
		})

		// Test case: Verifies that GetToken fails when no credentials are provided for
		// a basic auth challenge.
		ginkgo.It("should fail with no credentials for basic auth", func() {
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			runBasicAuthTest("Basic realm=\"test\"", "", "", "no credentials available")
		})

		// Test case: Ensures GetToken returns an error for an unsupported challenge type
		// (e.g., "Digest").
		ginkgo.It("should fail with unsupported challenge type", func() {
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			runBasicAuthTest(
				"Digest realm=\"test\"",
				"eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InBhc3MifQ==",
				"",
				"unsupported challenge type from registry",
			)
		})

		// Test case: Tests GetToken's behavior when an HTTP request fails (e.g., due to an
		// unreachable host). Uses a non-existent URL to trigger a network error, ensuring
		// the function handles such failures gracefully.
		ginkgo.It("should handle HTTP request failure", func() {
			// Use a valid image name with an unreachable host
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return("nonexistent.local/test/image").Maybe()

			client := NewAuthClient(nil, testRegistry)
			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"user:pass",
				client,
				testRegistry,
				"",
			)
			token := result.Token

			gomega.Expect(err).
				To(gomega.HaveOccurred(), "Expected error due to HTTP request failure")
			gomega.Expect(token).To(gomega.Equal(""), "Expected empty token on failure")
		})

		// Test case: Verifies that GetToken returns an empty token for an unauthenticated
		// local HTTP registry responding with 200 OK when TLS verification is skipped.
		ginkgo.It(
			"should return empty token for local HTTP registry (200 OK) with TLS skip",
			func() {
				// Create an HTTP test server to simulate the registry.
				server := ghttp.NewServer()

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/"),
						ghttp.RespondWith(http.StatusOK, ""),
					),
				)
				defer server.Close()

				// Parse the server URL to extract the host for the container's image name.
				serverURL, _ := url.Parse(server.URL())
				containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
				containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

				// Simulate WATCHTOWER_REGISTRY_TLS_SKIP=true to disable TLS verification.
				testRegistry.TLSSkip = true
				defer func() { testRegistry.TLSSkip = false }()

				// Create a custom authentication client with HTTP-only transport.
				client := &testAuthClient{
					client: &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: nil, // Disable TLS for HTTP requests
						},
					},
				}

				// Execute GetToken and verify the result.
				result, err := GetToken(newNopLogger(), context.Background(),
					containerInstance,
					"",
					client,
					testRegistry,
					"",
				)
				token := result.Token

				gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(token).To(gomega.Equal(""))
			},
		)

		// Test case: Ensures GetToken fails when attempting to connect to an HTTP registry
		// using HTTPS without TLS verification skipped, resulting in a connection error.
		ginkgo.It("should fail for HTTPS to HTTP registry without TLS skip", func() {
			// Create an HTTP test server to simulate the registry.
			server := ghttp.NewServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)
			defer server.Close()

			// Parse the server URL to extract the host for the container's image name.
			serverURL, _ := url.Parse(server.URL())
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

			// Ensure TLS verification is enabled.
			testRegistry.TLSSkip = false
			defer func() { testRegistry.TLSSkip = false }()

			// Create a new authentication client with default TLS settings.
			client := NewAuthClient(nil, testRegistry)

			// Execute GetToken and verify the expected failure.
			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token

			gomega.Expect(server.ReceivedRequests()).To(gomega.BeEmpty())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("http: server gave HTTP response to HTTPS client"))
			gomega.Expect(token).To(gomega.Equal(""))
		})

		// Test case: Verifies that GetToken returns an authentication error when
		// the registry responds with 401 and an empty WWW-Authenticate header.
		ginkgo.It("should return an authentication error for empty WWW-Authenticate header with 401 status", func() {
			// Create an HTTP test server to simulate the registry.
			server := ghttp.NewServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusUnauthorized, ""),
				),
			)
			defer server.Close()

			// Parse the server URL to extract the host for the container's image name.
			serverURL, _ := url.Parse(server.URL())
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

			// Create an authentication client with TLS verification disabled for the mock server.
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					},
				},
			}

			// Simulate WATCHTOWER_REGISTRY_TLS_SKIP=true to allow HTTP scheme.
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			// Execute GetToken and verify the result.
			_, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("no credentials available"))
		})

		// Test case: Verifies that GetToken returns an empty token for an HTTPS registry
		// responding with 200 OK without requiring authentication, even without TLS skip.
		ginkgo.It("should handle HTTPS registry with 200 OK without TLS skip", func() {
			// Create a TLS test server to simulate a secure registry.
			server := ghttp.NewTLSServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)
			defer server.Close()

			// Parse the server URL to extract the host for the container's image name.
			serverURL, _ := url.Parse(server.URL())
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

			// Create an authentication client with TLS verification disabled for the mock server.
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					},
				},
			}

			// Ensure TLS verification is enabled.
			testRegistry.TLSSkip = false

			// Execute GetToken and verify the result.
			result, err := GetToken(newNopLogger(), context.Background(), containerInstance, "", client, testRegistry, "")
			token := result.Token

			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal(""))
		})

		// Test case: Verifies that configureTLS handles an invalid TLS minimum version by
		// defaulting to TLS 1.2.
		ginkgo.It("should handle invalid TLS min version", func() {
			testRegistry.TLSMinVersion = "TLS9.9"
			defer func() { testRegistry.TLSMinVersion = "" }()

			tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
			configureTLS(nil, tlsConfig, testRegistry)
			gomega.Expect(tlsConfig.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS12)))
		})

		// Test case: Ensures GetToken fails when the TLS minimum version is set to an
		// incompatible value (e.g., TLS 1.3) for a registry supporting a lower version.
		ginkgo.It("should fail with TLS version mismatch", func() {
			// Create a TLS test server constrained to TLS 1.2 to simulate a secure registry.
			server := newTLSServerWithMaxVersion(tls.VersionTLS12)

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)
			defer server.Close()

			// Parse the server URL to extract the host for the container's image name.
			serverURL, _ := url.Parse(server.URL())
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

			// Create a fresh non-cached client requiring TLS 1.3.
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							MinVersion:         tls.VersionTLS13,
							InsecureSkipVerify: true,
						},
					},
				},
			}

			// Execute GetToken and verify the expected failure.
			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("failed to execute challenge request"))
			gomega.Expect(token).To(gomega.Equal(""))
		})

		ginkgo.It("should extract the challenge host for bearer token", func() {
			defer ginkgo.GinkgoRecover()

			redirectServer := ghttp.NewTLSServer()

			redirectServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
				),
			)
			defer redirectServer.Close()

			server := ghttp.NewTLSServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(
						http.StatusUnauthorized,
						"",
						http.Header{
							"WWW-Authenticate": []string{
								fmt.Sprintf(
									`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
									redirectServer.Addr(),
								),
							},
						},
					),
				),
			)
			defer server.Close()

			serverAddr := server.Addr()
			redirectAddr := redirectServer.Addr()
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverAddr + "/test/image:latest").Maybe()

			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			testRegistry.TLSSkip = false
			defer func() { testRegistry.TLSSkip = false }()

			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token
			challengeHost := result.ChallengeHost
			redirected := result.Redirected
			redirectHost := result.RedirectHost

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal("Bearer mock-token"))
			gomega.Expect(challengeHost).To(gomega.Equal(redirectAddr))
			gomega.Expect(redirected).To(gomega.BeFalse())   // No HTTP redirect occurred
			gomega.Expect(redirectHost).To(gomega.Equal("")) // No redirect, so redirectHost is empty
			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(redirectServer.ReceivedRequests()).To(gomega.HaveLen(1))
		})
		// Test case: Verifies that GetToken returns redirect=false when the challenge request is not redirected.
		ginkgo.It("should return redirect=false when challenge request is not redirected", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewTLSServer()
			server.RouteToHandler("GET", "/v2/", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(
					http.StatusUnauthorized,
					"",
					http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								server.Addr(),
							),
						},
					},
				),
			))

			server.RouteToHandler("GET", "/token", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token"),
				ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
			))
			defer server.Close()

			serverAddr := server.Addr()
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverAddr + "/test/image:latest").Maybe()

			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			testRegistry.TLSSkip = false
			defer func() { testRegistry.TLSSkip = false }()

			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token
			challengeHost := result.ChallengeHost
			redirected := result.Redirected
			redirectHost := result.RedirectHost

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal("Bearer mock-token"))
			gomega.Expect(challengeHost).To(gomega.Equal(serverAddr))
			gomega.Expect(redirected).To(gomega.BeFalse())
			gomega.Expect(redirectHost).To(gomega.Equal("")) // No redirect, so redirectHost is empty
			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(2))
		})

		ginkgo.It("should derive bearer service from realm when registry challenge omits service", func() {
			defer ginkgo.GinkgoRecover()

			server := ghttp.NewTLSServer()
			server.RouteToHandler("GET", "/v2/", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(
					http.StatusUnauthorized,
					"",
					http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(`Bearer realm="https://%s/token"`, server.Addr()),
						},
					},
				),
			))

			server.RouteToHandler("GET", "/token", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token", "scope=repository%3Atest%2Fimage%3Apull&service="+url.QueryEscape(server.Addr())),
				ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
			))
			defer server.Close()

			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(server.Addr() + "/test/image:latest").Maybe()

			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			testRegistry.TLSSkip = false
			defer func() { testRegistry.TLSSkip = false }()

			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token
			challengeHost := result.ChallengeHost
			redirected := result.Redirected
			redirectHost := result.RedirectHost

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal("Bearer mock-token"))
			gomega.Expect(challengeHost).To(gomega.Equal(server.Addr()))
			gomega.Expect(redirected).To(gomega.BeFalse())
			gomega.Expect(redirectHost).To(gomega.Equal(""))
			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(2))
		})
		// Test case: Verifies that GetToken returns redirect=true when the challenge request is redirected.
		ginkgo.It("should return redirect=true when challenge request is redirected", func() {
			defer ginkgo.GinkgoRecover()

			redirectServer := ghttp.NewTLSServer()
			redirectServer.RouteToHandler("GET", "/v2/", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(
					http.StatusUnauthorized,
					"",
					http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="https://%s/token",service="test-service",scope="repository:test/image:pull"`,
								redirectServer.Addr(),
							),
						},
					},
				),
			))

			redirectServer.RouteToHandler("GET", "/token", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token"),
				ghttp.RespondWith(http.StatusOK, `{"token": "mock-token"}`),
			))
			defer redirectServer.Close()

			server := ghttp.NewTLSServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(
						http.StatusFound,
						"",
						http.Header{
							"Location": []string{
								fmt.Sprintf("https://%s/v2/", redirectServer.Addr()),
							},
						},
					),
				),
			)
			defer server.Close()

			serverAddr := server.Addr()
			redirectAddr := redirectServer.Addr()
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverAddr + "/test/image:latest").Maybe()

			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			testRegistry.TLSSkip = false
			defer func() { testRegistry.TLSSkip = false }()

			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token
			challengeHost := result.ChallengeHost
			redirected := result.Redirected
			redirectHost := result.RedirectHost

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal("Bearer mock-token"))
			gomega.Expect(challengeHost).To(gomega.Equal(redirectAddr))
			gomega.Expect(redirected).To(gomega.BeTrue())              // HTTP redirect occurred
			gomega.Expect(redirectHost).To(gomega.Equal(redirectAddr)) // Redirect host matches the final destination
			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(redirectServer.ReceivedRequests()).To(gomega.HaveLen(2))
		})

		// Test case: Ensures GetToken fails with an error when the HTTP client exceeds
		// the maximum number of redirects, verifying proper handling of redirect limits.
		ginkgo.It("should fail when exceeding maximum redirects", func() {
			// Create a chain of test servers to simulate multiple redirects
			redirectCount := defaultMaxRedirects + 1 // Exceed limit (3 + 1 = 4 redirects)

			servers := make([]*ghttp.Server, redirectCount)
			for i := range redirectCount {
				servers[i] = ghttp.NewServer()
				defer servers[i].Close()
			}

			for i := range redirectCount {
				if i < redirectCount-1 {
					servers[i].AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/v2/"),
							ghttp.RespondWith(
								http.StatusTemporaryRedirect,
								"",
								http.Header{"Location": []string{servers[i+1].URL() + "/v2/"}},
							),
						),
					)
				} else {
					servers[i].AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/v2/"),
							ghttp.RespondWith(
								http.StatusOK,
								`{"token": "final-token"}`,
								http.Header{"Content-Type": []string{"application/json"}},
							),
						),
					)
				}
			}

			// Configure container with the first server's address
			serverURL, _ := url.Parse(servers[0].URL())
			containerInstance := mockTypes.NewMockContainer(ginkgo.GinkgoT())
			containerInstance.On("ImageName").Return(serverURL.Host + "/test/image:latest").Maybe()

			// Create a custom client to enforce redirect limit
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
					CheckRedirect: func(_ *http.Request, via []*http.Request) error {
						if len(via) >= defaultMaxRedirects {
							return fmt.Errorf(
								"stopped after %d redirects",
								defaultMaxRedirects,
							)
						}

						return nil
					},
				},
			}

			// Use HTTP scheme to match non-TLS servers
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			// Execute GetToken and verify failure due to too many redirects
			result, err := GetToken(newNopLogger(), context.Background(),
				containerInstance,
				"",
				client,
				testRegistry,
				"",
			)
			token := result.Token

			gomega.Expect(err).
				To(gomega.HaveOccurred(), "Expected error due to exceeding redirect limit")
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("stopped after 3 redirects"), "Expected error indicating redirect limit exceeded")
			gomega.Expect(token).To(gomega.Equal(""), "Expected empty token on redirect failure")
			gomega.Expect(servers[0].ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(servers[1].ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(servers[2].ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(servers[3].ReceivedRequests()).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("getChallengeRequest", func() {
		// Test case: Verifies that getChallengeRequest constructs a valid HTTP GET request
		// with the expected headers and URL. Ensures the request is properly formed for
		// registry challenges.
		ginkgo.It("should create a valid HTTP request", func() {
			url := url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/v2/",
			}
			req, err := getChallengeRequest(newNopLogger(), context.Background(), url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(req.Method).To(gomega.Equal(http.MethodGet))
			gomega.Expect(req.URL.String()).To(gomega.Equal("https://example.com/v2/"))
			gomega.Expect(req.Header.Get("Accept")).To(gomega.Equal("*/*"))
			gomega.Expect(req.Header.Get("User-Agent")).To(gomega.Equal(meta.UserAgent))
			gomega.Expect(req.Context()).To(gomega.Equal(context.Background()))
		})

		// Test case: Ensures getChallengeRequest returns an error when given an invalid URL.
		// Tests error handling for malformed inputs, such as an invalid scheme.
		ginkgo.It("should return an error for invalid URL", func() {
			url := url.URL{
				Scheme: "://", // Invalid scheme
				Host:   "example.com",
			}
			req, err := getChallengeRequest(newNopLogger(), context.Background(), url)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(req).To(gomega.BeNil())
		})
	})

	ginkgo.Describe("getBearerToken", func() {
		// Test case: Verifies that getBearerToken fetches a bearer token successfully from
		// a mock registry response without credentials.
		ginkgo.It("should fetch a bearer token successfully", func() {
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			runBearerHeaderTest("", "test-token", false)
		})

		// Test case: Ensures getBearerToken fetches a bearer token when credentials are
		// provided, validating the Authorization header.
		ginkgo.It("should fetch a bearer token with credentials", func() {
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			runBearerHeaderTest("eyJ1c2VybmFtZSI6InVzZXIiLCJwYXNzd29yZCI6InBhc3MifQ==", "auth-token", true)
		})

		// Test case: Tests getBearerToken's error handling when the HTTP request fails
		// (e.g., due to an unreachable host). Ensures proper error propagation.
		ginkgo.It("should fail on HTTP request error", func() {
			client := &testAuthClient{
				client: &http.Client{
					Timeout: 1 * time.Second,
				},
			}
			challenge := `bearer realm="http://nonexistent.local/token",service="test-service",scope="repository:test/image:pull"`
			ref, _ := reference.ParseNormalizedNamed("test/image")
			token, err := getBearerToken(newNopLogger(), context.Background(), challenge, ref, "", client, testRegistry)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal(""))
		})

		// Test case: Verifies getBearerToken fails when the registry returns invalid JSON.
		ginkgo.It("should fail on invalid JSON response", func() {
			// Create a TLS test server to simulate the registry.
			server := ghttp.NewTLSServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/"),
					func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprintf(w, `{"invalid": "json"`)
					},
				),
			)
			defer server.Close()

			// Create an authentication client with TLS verification disabled.
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			// Construct the challenge header.
			challenge := fmt.Sprintf(
				`bearer realm="%s",service="test-service",scope="repository:test/image:pull"`,
				server.URL(),
			)
			ref, _ := reference.ParseNormalizedNamed("test/image")

			// Execute getBearerToken and verify the failure.
			token, err := getBearerToken(newNopLogger(), context.Background(), challenge, ref, "", client, testRegistry)

			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(token).To(gomega.Equal(""))
		})

		ginkgo.It("should handle missing scope in WWW-Authenticate header gracefully", func() {
			// Create a TLS test server
			server := ghttp.NewTLSServer()
			server.RouteToHandler("GET", "/v2/", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
				ghttp.RespondWith(
					http.StatusUnauthorized,
					"",
					http.Header{
						"WWW-Authenticate": []string{
							fmt.Sprintf(
								`Bearer realm="%s/token",service="test-service"`,
								server.URL(),
							),
						},
					},
				),
			))

			server.RouteToHandler("GET", "/token", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/token"),
				ghttp.RespondWith(
					http.StatusOK,
					`{"token": "mock-token"}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			))
			defer server.Close()

			// Set up the mock reference
			ref, err := reference.ParseNormalizedNamed("test/image:latest")
			gomega.Expect(err).
				NotTo(gomega.HaveOccurred(), "Expected no error parsing image reference")

			// Create a client with TLS skip
			client := &testAuthClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				},
			}

			// Set TLS skip
			testRegistry.TLSSkip = true
			defer func() { testRegistry.TLSSkip = false }()

			// Construct the challenge header
			challenge := fmt.Sprintf(
				`bearer realm="%s/token",service="test-service"`,
				server.URL(),
			)

			// Call getBearerToken directly to test processChallenge
			token, err := getBearerToken(newNopLogger(), context.Background(), challenge, ref, "", client, testRegistry)

			gomega.Expect(server.ReceivedRequests()).To(gomega.HaveLen(1))
			gomega.Expect(err).
				NotTo(gomega.HaveOccurred(), "Expected no error from GetBearerToken")
			gomega.Expect(token).
				To(gomega.Equal("Bearer mock-token"), "Expected token to be 'Bearer mock-token'")
		})
	})

	ginkgo.Describe("getAuthURL", func() {
		// Test case: Ensures getAuthURL constructs a valid URL from a bearer challenge
		// header, including realm, service, and scope parameters, for a given image reference.
		ginkgo.It(
			"should create a valid auth url object based on the challenge header supplied",
			func() {
				challenge := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
				imageRef, err := reference.ParseNormalizedNamed("nicholas-fedor/watchtower")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				expected := &url.URL{
					Host:     "ghcr.io",
					Scheme:   "https",
					Path:     "/token",
					RawQuery: "scope=repository%3Anicholas-fedor%2Fwatchtower%3Apull&service=ghcr.io",
				}

				URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(URL).To(gomega.Equal(expected))
			},
		)

		ginkgo.When("given an invalid challenge header", func() {
			ginkgo.It("should return an error", func() {
				challenge := `bearer service="ghcr.io"`
				imageRef, err := reference.ParseNormalizedNamed("nicholas-fedor/watchtower")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(URL).To(gomega.BeNil())
			})
		})

		ginkgo.It("should derive service from realm when challenge omits service", func() {
			challenge := `bearer realm="https://registry.example.com/token"`
			imageRef, err := reference.ParseNormalizedNamed("registry.example.com/test/image:latest")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL.String()).
				To(gomega.Equal("https://registry.example.com/token?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com"))
		})

		ginkgo.It("should derive service from realm when service is explicitly empty", func() {
			challenge := `bearer realm="https://registry.example.com/token",service=""`
			imageRef, err := reference.ParseNormalizedNamed("registry.example.com/test/image:latest")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL.String()).
				To(gomega.Equal("https://registry.example.com/token?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com"))
		})

		ginkgo.It("should derive service from realm host when realm includes a port", func() {
			challenge := `bearer realm="http://localhost:5000/token"`
			imageRef, err := reference.ParseNormalizedNamed("localhost:5000/test/image:latest")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL.String()).
				To(gomega.Equal("http://localhost:5000/token?scope=repository%3Atest%2Fimage%3Apull&service=localhost%3A5000"))
		})

		ginkgo.It("should derive service from realm host when realm has a trailing slash", func() {
			challenge := `bearer realm="https://registry.example.com/token/"`
			imageRef, err := reference.ParseNormalizedNamed("registry.example.com/test/image:latest")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL.String()).
				To(gomega.Equal("https://registry.example.com/token/?scope=repository%3Atest%2Fimage%3Apull&service=registry.example.com"))
		})

		ginkgo.It("should return an error when realm lacks a scheme and service is omitted", func() {
			challenge := `bearer realm="registry.example.com/token"`
			imageRef, err := reference.ParseNormalizedNamed("registry.example.com/test/image:latest")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			URL, err := getAuthURL(newNopLogger(), challenge, imageRef, "", testRegistry)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.BeNil())
		})

		ginkgo.When("deriving the auth scope from an image name", func() {
			// Test case: Ensures getAuthURL prepends "library/" to official Docker Hub images,
			// validating correct scope derivation for standard images.
			ginkgo.It("should prepend official dockerhub images with \"library/\"", func() {
				gomega.Expect(getScopeFromImageAuthURL("registry")).
					To(gomega.Equal("library/registry"))
				gomega.Expect(getScopeFromImageAuthURL("docker.io/registry")).
					To(gomega.Equal("library/registry"))
			})

			// Test case: Verifies getAuthURL excludes vanity hosts (e.g., "docker.io") from the
			// scope, ensuring clean repository paths for Docker Hub images.
			ginkgo.It("should not include vanity hosts", func() {
				gomega.Expect(getScopeFromImageAuthURL("docker.io/nickfedor/watchtower")).
					To(gomega.Equal("nickfedor/watchtower"))
				gomega.Expect(getScopeFromImageAuthURL("index.docker.io/nickfedor/watchtower")).
					To(gomega.Equal("nickfedor/watchtower"))
			})

			// Test case: Confirms getAuthURL handles non-Docker Hub images correctly, extracting
			// the repository path without additional prefixes for registries like GHCR.
			ginkgo.It("should handle non-Docker Hub images correctly", func() {
				gomega.Expect(getScopeFromImageAuthURL("ghcr.io/watchtower")).
					To(gomega.Equal("watchtower"))
				gomega.Expect(getScopeFromImageAuthURL("ghcr.io/nicholas-fedor/watchtower")).
					To(gomega.Equal("nicholas-fedor/watchtower"))
			})
		})

		// Test case: Ensures getAuthURL does not panic when the challenge header includes an
		// empty field, testing robustness against incomplete but valid inputs.
		ginkgo.It("should not crash when an empty field is received", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",`
			imageRef, err := reference.ParseNormalizedNamed("nicholas-fedor/watchtower")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			res, err := getAuthURL(newNopLogger(), input, imageRef, "", testRegistry)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res).NotTo(gomega.BeNil())
		})

		// Test case: Verifies getAuthURL handles a valueless key in the challenge header
		// without crashing, ensuring stability with unusual but parsable inputs.
		ginkgo.It("should not crash when a field without a value is received", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull",valuelesskey`
			imageRef, err := reference.ParseNormalizedNamed("nicholas-fedor/watchtower")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			res, err := getAuthURL(newNopLogger(), input, imageRef, "", testRegistry)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("getChallengeURL", func() {
		// Test case: Ensures getChallengeURL constructs a correct challenge URL for a
		// GHCR-hosted image, validating registry address extraction and URL formatting.
		ginkgo.It(
			"should create a valid challenge url object based on the image ref supplied",
			func() {
				expected := url.URL{Host: "ghcr.io", Scheme: "https", Path: "/v2/"}
				imageRef, _ := reference.ParseNormalizedNamed(
					"ghcr.io/nicholas-fedor/watchtower:latest",
				)
				gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "")).To(gomega.Equal(expected))
			},
		)

		// Test case: Verifies getChallengeURL defaults to Docker Hub (index.docker.io) for
		// images without an explicit registry, ensuring correct fallback behavior.
		ginkgo.It("should assume Docker Hub for image refs with no explicit registry", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("nickfedor/watchtower:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "")).To(gomega.Equal(expected))
		})

		// Test case: Confirms getChallengeURL uses "index.docker.io" for "docker.io" registry
		// references, validating consistent handling of Docker Hub vanity URLs.
		ginkgo.It("should use index.docker.io if the image ref specifies docker.io", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/nickfedor/watchtower:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "")).To(gomega.Equal(expected))
		})

		// Test case: Verifies getChallengeURL uses the endpoint host when provided.
		ginkgo.It("should use endpoint host and scheme when provided", func() {
			expected := url.URL{Host: "mirror.example.com", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "https://mirror.example.com")).To(gomega.Equal(expected))
		})

		// Test case: Verifies getChallengeURL uses default https scheme for a bare endpoint.
		ginkgo.It("should use default https scheme for bare endpoint", func() {
			expected := url.URL{Host: "mirror.example.com", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "mirror.example.com")).To(gomega.Equal(expected))
		})

		// Test case: Verifies getChallengeURL preserves http scheme when endpoint specifies it.
		ginkgo.It("should use http scheme when endpoint specifies http", func() {
			expected := url.URL{Host: "mirror.example.com", Scheme: "http", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "http://mirror.example.com")).To(gomega.Equal(expected))
		})

		// Test case: Verifies getChallengeURL preserves port from endpoint.
		ginkgo.It("should preserve port from endpoint", func() {
			expected := url.URL{Host: "mirror.example.com:5000", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "https://mirror.example.com:5000")).To(gomega.Equal(expected))
		})

		// Test case: Verifies getChallengeURL uses endpoint as bare host when URL parsing fails.
		ginkgo.It("should use endpoint as bare host when URL parsing fails", func() {
			expected := url.URL{Host: "invalid url with spaces", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("docker.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "invalid url with spaces")).To(gomega.Equal(expected))
		})

		// Test case: Verifies endpoint takes precedence over lscr.io special handling.
		ginkgo.It("should use endpoint even for lscr.io images", func() {
			expected := url.URL{Host: "mirror.example.com", Scheme: "https", Path: "/v2/"}
			imageRef, _ := reference.ParseNormalizedNamed("lscr.io/library/nginx:latest")
			gomega.Expect(getChallengeURL(newNopLogger(), imageRef, testRegistry, "https://mirror.example.com")).To(gomega.Equal(expected))
		})
	})

	ginkgo.Describe("GetRegistryAddress", func() {
		ginkgo.It("should return error if passed empty string", func() {
			_, err := GetRegistryAddress(newNopLogger(), "")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should return index.docker.io for image refs with no explicit registry", func() {
			gomega.Expect(GetRegistryAddress(newNopLogger(), "watchtower")).To(gomega.Equal("index.docker.io"))
			gomega.Expect(GetRegistryAddress(newNopLogger(), "nickfedor/watchtower")).
				To(gomega.Equal("index.docker.io"))
		})
		ginkgo.It("should return index.docker.io for image refs with docker.io domain", func() {
			gomega.Expect(GetRegistryAddress(newNopLogger(), "docker.io/watchtower")).
				To(gomega.Equal("index.docker.io"))
			gomega.Expect(GetRegistryAddress(newNopLogger(), "docker.io/nickfedor/watchtower")).
				To(gomega.Equal("index.docker.io"))
		})
		ginkgo.It("should return the host if passed an image name containing a local host", func() {
			gomega.Expect(GetRegistryAddress(newNopLogger(), "henk:80/watchtower")).To(gomega.Equal("henk:80"))
			gomega.Expect(GetRegistryAddress(newNopLogger(), "localhost/watchtower")).
				To(gomega.Equal("localhost"))
		})
		ginkgo.It(
			"should return the server address if passed a fully qualified image name",
			func() {
				gomega.Expect(GetRegistryAddress(newNopLogger(), "github.com/nicholas-fedor/config")).
					To(gomega.Equal("github.com"))
			},
		)
	})

	ginkgo.Describe("TransformAuth", func() {
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

			result := TransformAuth(newNopLogger(), inputAuth)
			expected := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
			gomega.Expect(result).To(gomega.Equal(expected))
		})

		ginkgo.It("should return original input if decoding fails", func() {
			inputAuth := "invalid-base64-string"
			result := TransformAuth(newNopLogger(), inputAuth)
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

			result := TransformAuth(newNopLogger(), inputAuth)
			gomega.Expect(result).To(gomega.Equal(inputAuth))
		})

		ginkgo.It("should handle malformed JSON", func() {
			inputAuth := base64.StdEncoding.EncodeToString([]byte("invalid json"))
			result := TransformAuth(newNopLogger(), inputAuth)
			gomega.Expect(result).To(gomega.Equal(inputAuth))
		})

		ginkgo.It("should handle missing password field", func() {
			creds := struct {
				Username string `json:"username"`
			}{
				Username: "testuser",
			}
			jsonData, _ := json.Marshal(creds)
			inputAuth := base64.StdEncoding.EncodeToString(jsonData)

			result := TransformAuth(newNopLogger(), inputAuth)
			gomega.Expect(result).To(gomega.Equal(inputAuth))
		})

		ginkgo.It("should transform password-only credentials to Basic auth", func() {
			creds := struct {
				Password string `json:"password"`
			}{
				Password: "testpass",
			}
			jsonData, _ := json.Marshal(creds)
			inputAuth := base64.StdEncoding.EncodeToString(jsonData)

			result := TransformAuth(newNopLogger(), inputAuth)
			decoded, err := base64.StdEncoding.DecodeString(result)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(decoded)).To(gomega.Equal(":testpass"))
		})
	})

	// NewAuthClientCachingTests tests the HTTP client caching behavior using sync.Once.
	// These tests verify that the cached client is properly initialized once and reused
	// across multiple calls, ensuring thread-safety and efficient resource usage.
	ginkgo.Describe("NewAuthClient caching", func() {
		// Test case: Verifies that multiple calls to NewAuthClient return the same
		// client instance, ensuring proper caching behavior.
		ginkgo.It("should return the same client instance on multiple calls", func() {
			client1 := NewAuthClient(nil, testRegistry)
			client2 := NewAuthClient(nil, testRegistry)
			client3 := NewAuthClient(nil, testRegistry)

			// All calls should return the exact same client instance
			gomega.Expect(client1).To(gomega.BeIdenticalTo(client2))
			gomega.Expect(client2).To(gomega.BeIdenticalTo(client3))
		})

		// Test case: Verifies that concurrent calls to NewAuthClient are thread-safe
		// and return the same client instance without race conditions.
		ginkgo.It("should handle concurrent access safely", func() {
			const numGoroutines = 100

			clients := make([]Client, numGoroutines)

			var wg sync.WaitGroup

			for i := range numGoroutines {
				wg.Add(1)

				go func(idx int) {
					defer wg.Done()

					clients[idx] = NewAuthClient(nil, testRegistry)
				}(i)
			}

			wg.Wait()

			// All goroutines should have received the same client instance
			for i := 1; i < numGoroutines; i++ {
				gomega.Expect(clients[i]).To(gomega.BeIdenticalTo(clients[0]))
			}
		})

		// Test case: Verifies that the client returned by NewAuthClient is usable
		// for making HTTP requests, ensuring the cached client is fully functional.
		ginkgo.It("should return a functional client", func() {
			// Create a mock HTTP server to simulate a registry response.
			server := ghttp.NewServer()

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/get"),
					ghttp.RespondWith(200, ""),
				),
			)
			defer server.Close()

			client := NewAuthClient(nil, testRegistry)

			// Create a simple HTTP request to the mock server.
			req, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				server.URL()+"/get",
				nil,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// The client should be able to execute requests without panic or error.
			_, _ = client.Do(req)
		})
	})
})

// getScopeFromImageAuthURL extracts and returns the repository path from an auth URL's
// scope parameter for a given image name. It constructs a mock challenge header without
// a scope, builds the auth URL, and strips the "repository:" prefix and ":pull" suffix,
// providing the clean path used in registry authentication.
func getScopeFromImageAuthURL(imageName string) string {
	normalizedRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "" // Return empty string on parse failure to avoid panic
	}

	challenge := `bearer realm="https://dummy.host/token",service="dummy.host"`

	URL, err := getAuthURL(newNopLogger(), challenge, normalizedRef, "", testRegistry)
	if err != nil {
		return "" // Return empty string on auth URL failure
	}

	scope := URL.Query().Get("scope")

	return strings.TrimSuffix(strings.TrimPrefix(scope, "repository:"), ":pull")
}
