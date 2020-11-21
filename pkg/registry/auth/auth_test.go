package auth

import (
	"net/url"
	"testing"
	. "github.com/onsi/gomega"
	. "github.com/onsi/ginkgo"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Auth Suite")
}

var _ = Describe("the auth module", func() {
	When("getting an auth url", func() {
		It("should create a valid auth url object based on the challenge header supplied", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/image:pull"`
			expected := &url.URL{
				Host: "ghcr.io",
				Scheme: "https",
				Path: "/token",
				RawQuery: "scope=repository%3Acontainrrr%2Fwatchtower%3Apull&service=ghcr.io",
			}
			res, err := GetAuthURL(input, "containrrr/watchtower")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should create a valid auth url object based on the challenge header supplied", func() {
			input := `bearer realm="https://ghcr.io/token",service="ghcr.io"`
			res, err := GetAuthURL(input, "containrrr/watchtower")
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeNil())
		})
	})
	When("getting a challenge url", func() {
		It("should create a valid challenge url object based on the image ref supplied", func() {
			expected := url.URL{ Host: "ghcr.io", Scheme: "https", Path: "/v2/"}
			Expect(GetChallengeURL("ghcr.io/containrrr/watchtower:latest")).To(Equal(expected))
		})
		It("should assume dockerhub if the image ref is not fully qualified", func() {
			expected := url.URL{ Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			Expect(GetChallengeURL("containrrr/watchtower:latest")).To(Equal(expected))
		})
		It("should convert legacy dockerhub hostnames to index.docker.io", func() {
			expected := url.URL{ Host: "index.docker.io", Scheme: "https", Path: "/v2/"}
			Expect(GetChallengeURL("docker.io/containrrr/watchtower:latest")).To(Equal(expected))
			Expect(GetChallengeURL("registry-1.docker.io/containrrr/watchtower:latest")).To(Equal(expected))
		})
	})
})

