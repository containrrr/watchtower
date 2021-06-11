package digest_test

import (
	"fmt"
	"github.com/containrrr/watchtower/internal/actions/mocks"
	"github.com/containrrr/watchtower/pkg/registry/digest"
	wtTypes "github.com/containrrr/watchtower/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestDigest(t *testing.T) {

	RegisterFailHandler(Fail)
	RunSpecs(GinkgoT(), "Digest Suite")
}

var (
	DockerHubCredentials = &wtTypes.RegistryCredentials{
		Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_USERNAME"),
		Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_PASSWORD"),
	}
	GHCRCredentials = &wtTypes.RegistryCredentials{
		Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
		Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
	}
)

func SkipIfCredentialsEmpty(credentials *wtTypes.RegistryCredentials, fn func()) func() {
	if credentials.Username == "" {
		return func() {
			Skip("Username missing. Skipping integration test")
		}
	} else if credentials.Password == "" {
		return func() {
			Skip("Password missing. Skipping integration test")
		}
	} else {
		return fn
	}
}

var _ = Describe("Digests", func() {
	mockId := "mock-id"
	mockName := "mock-container"
	mockImage := "ghcr.io/k6io/operator:latest"
	mockCreated := time.Now()
	mockDigest := "ghcr.io/k6io/operator@sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"

	mockContainer := mocks.CreateMockContainerWithDigest(
		mockId,
		mockName,
		mockImage,
		mockCreated,
		mockDigest)

	When("a digest comparison is done", func() {
		It("should return true if digests match",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				creds := fmt.Sprintf("%s:%s", GHCRCredentials.Username, GHCRCredentials.Password)
				matches, err := digest.CompareDigest(mockContainer, creds)
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(Equal(true))
			}),
		)

		It("should return false if digests differ", func() {

		})
		It("should return an error if the registry isn't available", func() {

		})
	})
	When("using different registries", func() {
		It("should work with DockerHub",
			SkipIfCredentialsEmpty(DockerHubCredentials, func() {
				fmt.Println(DockerHubCredentials != nil) // to avoid crying linters
			}),
		)
		It("should work with GitHub Container Registry",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				fmt.Println(GHCRCredentials != nil) // to avoid crying linters
			}),
		)
	})
	When("sending a HEAD request", func() {
		var server *ghttp.Server
		BeforeEach(func() {
			server = ghttp.NewServer()
		})
		AfterEach(func() {
			server.Close()
		})
		It("should use a custom user-agent", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyHeader(http.Header{
						"User-Agent": []string{"Watchtower/v0.0.0-unknown"},
					}),
					ghttp.RespondWith(http.StatusOK, "", http.Header{
						digest.ContentDigestHeader: []string{
							mockDigest,
						},
					}),
				),
			)
			dig, err := digest.GetDigest(server.URL(), "token")
			println(dig)
			Expect(server.ReceivedRequests()).Should(HaveLen(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(dig).To(Equal(mockDigest))
		})
	})
})
