package helpers

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helper Suite")
}

var _ = Describe("the helpers", func() {
	Describe("GetRegistryAddress", func() {
		It("should return error if passed empty string", func() {
			_, err := GetRegistryAddress("")
			Expect(err).To(HaveOccurred())
		})
		It("should return index.docker.io if passed an image name with no explicit domain", func() {
			Expect(GetRegistryAddress("watchtower")).To(Equal("index.docker.io"))
			Expect(GetRegistryAddress("containrrr/watchtower")).To(Equal("index.docker.io"))
		})
		It("should return the host if passed an image name containing a local host", func() {
			Expect(GetRegistryAddress("henk:80/watchtower")).To(Equal("henk:80"))
			Expect(GetRegistryAddress("localhost/watchtower")).To(Equal("localhost"))
		})
		It("should return the server name if passed a fully qualified image name", func() {
			Expect(GetRegistryAddress("github.com/containrrr/config")).To(Equal("github.com"))
		})
	})
})
