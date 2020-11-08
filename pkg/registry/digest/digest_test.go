package digest

import (
	"github.com/docker/docker/api/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"os"
	"testing"
)

func TestDigest(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Digest Suite")
}

var image = types.ImageInspect{
	ID: "sha256:6972c414f322dfa40324df3c503d4b217ccdec6d576e408ed10437f508f4181b",
	RepoTags: []string {
		"ghcr.io/k6io/operator:latest",
	},
	RepoDigests: []string {
		"ghcr.io/k6io/operator@sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547",
	},
}

var (
	DH_USERNAME = os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_USERNAME")
	DH_PASSWORD = os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_PASSWORD")
	GH_USERNAME = os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_USERNAME")
	GH_PASSWORD = os.Getenv("CI_INTEGRATION_TEST_REGISTRY_DH_PASSWORD")
)

var _ = Describe("Digests", func() {
	When("fetching a bearer token", func() {
		It("should parse the token from the response", func() {
			token, err := GetToken(image, DH_USERNAME, DH_PASSWORD)
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(Equal(""))
		})
	})
	When("a digest comparison is done", func() {
		It("should return true if digests match", func() {
			matches, err := CompareDigest(image, DH_USERNAME, DH_PASSWORD)
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(Equal(true))
		})
		It("should return false if digests differ", func() {

		})
		It("should return an error if the registry isn't available", func() {

		})
	})
	When("using different registries", func() {
		It("should work with DockerHub", func() {

		})
		It("should work with GitHub Container Registry", func() {

		})
	})
})
