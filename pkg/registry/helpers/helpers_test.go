package helpers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helper Suite")
}

var _ = Describe("the helpers", func() {

	When("converting an url to a hostname", func() {
		It("should return docker.io given docker.io/containrrr/watchtower:latest", func() {
			host, port, err := ConvertToHostname("docker.io/containrrr/watchtower:latest")
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal("docker.io"))
			Expect(port).To(BeEmpty())
		})
	})
	When("normalizing the registry information", func() {
		It("should return index.docker.io given docker.io", func() {
			out, err := NormalizeRegistry("docker.io/containrrr/watchtower:latest")
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(Equal("index.docker.io"))
		})
	})
})
