package auth_test

import (
	"context"
	"fmt"
	"github.com/containrrr/watchtower/internal/actions/mocks"
	"github.com/containrrr/watchtower/pkg/registry/auth"
	"net/url"
	"os"
	"testing"
	"time"

	wtTypes "github.com/containrrr/watchtower/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Auth Suite")
}
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

var GHCRCredentials = &wtTypes.RegistryCredentials{
	Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
	Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
}

var _ = Describe("the auth module", func() {
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

	When("getting an auth url", func() {
		It("should parse the token from the response",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				creds := fmt.Sprintf("%s:%s", GHCRCredentials.Username, GHCRCredentials.Password)
				token, err := auth.GetToken(context.Background(), mockContainer, creds)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).NotTo(Equal(""))
			}),
		)

		It("should create a valid auth url object based on the challenge header supplied", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
			expected := &url.URL{
				Host:     "ghcr.io",
				Scheme:   "https",
				Path:     "/token",
				RawQuery: "scope=repository%3Acontainrrr%2Fwatchtower%3Apull&service=ghcr.io",
			}
			res, err := auth.GetAuthURL(input, "containrrr/watchtower")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should create a valid auth url object based on the challenge header supplied", func() {
			input := `bearer realm="https://ghcr.io/token"`
			res, err := auth.GetAuthURL(input, "containrrr/watchtower")
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeNil())
		})
	})
	When("getting a challenge url", func() {
		It("should create a valid challenge url object based on the image ref supplied", func() {
			expected := url.URL{Host: "ghcr.io", Scheme: "https", Path: "/v2/"}
			Expect(auth.GetChallengeURL("ghcr.io/containrrr/watchtower:latest")).To(Equal(expected))
		})
		It("should assume dockerhub if the image ref is not fully qualified", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			Expect(auth.GetChallengeURL("containrrr/watchtower:latest")).To(Equal(expected))
		})
		It("should convert legacy dockerhub hostnames to index.docker.io", func() {
			expected := url.URL{Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			Expect(auth.GetChallengeURL("docker.io/containrrr/watchtower:latest")).To(Equal(expected))
			Expect(auth.GetChallengeURL("registry-1.docker.io/containrrr/watchtower:latest")).To(Equal(expected))
		})
	})
})
