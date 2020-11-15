package digest

import (
	"context"
	"fmt"
	"github.com/containrrr/watchtower/pkg/logger"
	"github.com/containrrr/watchtower/pkg/registry/auth"
	wtTypes "github.com/containrrr/watchtower/pkg/types"
	"github.com/docker/docker/api/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"
)

func TestDigest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Digest Suite")
}

var ghImage = types.ImageInspect{
	ID: "sha256:6972c414f322dfa40324df3c503d4b217ccdec6d576e408ed10437f508f4181b",
	RepoTags: []string{
		"ghcr.io/k6io/operator:latest",
	},
	RepoDigests: []string{
		"ghcr.io/k6io/operator@sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547",
	},
}

var DockerHubCredentials = &wtTypes.RegistryCredentials{
	Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_USERNAME"),
	Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_PASSWORD"),
}
var GHCRCredentials = &wtTypes.RegistryCredentials{
	Username: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_USERNAME"),
	Password: os.Getenv("CI_INTEGRATION_TEST_REGISTRY_GH_PASSWORD"),
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

var _ = Describe("Digests", func() {
	var ctx = logger.AddDebugLogger(context.Background())

	When("fetching a bearer token", func() {

		It("should parse the token from the response",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				token, err := auth.GetToken(ctx, ghImage, GHCRCredentials)
				Expect(err).NotTo(HaveOccurred())
				Expect(token).NotTo(Equal(""))
			}),
		)
	})
	When("a digest comparison is done", func() {
		It("should return true if digests match",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				matches, err := CompareDigest(ctx, ghImage, GHCRCredentials)
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
		It("should work with DockerHub", func() {

		})
		It("should work with GitHub Container Registry",
			SkipIfCredentialsEmpty(GHCRCredentials, func() {
				fmt.Println(GHCRCredentials != nil) // to avoid crying linters
			}),
		)
	})
})
