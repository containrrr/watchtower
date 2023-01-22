package manifest_test

import (
	"testing"
	"time"

	"github.com/containrrr/watchtower/internal/actions/mocks"
	"github.com/containrrr/watchtower/pkg/registry/manifest"
	apiTypes "github.com/docker/docker/api/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

var _ = Describe("the manifest module", func() {
	Describe("BuildManifestURL", func() {
		It("should return a valid url given a fully qualified image", func() {
			imageRef := "ghcr.io/containrrr/watchtower:mytag"
			expected := "https://ghcr.io/v2/containrrr/watchtower/manifests/mytag"

			URL, err := buildMockContainerManifestURL(imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(URL).To(Equal(expected))
		})
		It("should assume Docker Hub for image refs with no explicit registry", func() {
			imageRef := "containrrr/watchtower:latest"
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(URL).To(Equal(expected))
		})
		It("should assume latest for image refs with no explicit tag", func() {
			imageRef := "containrrr/watchtower"
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(URL).To(Equal(expected))
		})
		It("should not prepend library/ for single-part container names in registries other than Docker Hub", func() {
			imageRef := "docker-registry.domain/imagename:latest"
			expected := "https://docker-registry.domain/v2/imagename/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			Expect(err).NotTo(HaveOccurred())
			Expect(URL).To(Equal(expected))
		})
		It("should throw an error on pinned images", func() {
			imageRef := "docker-registry.domain/imagename@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7"
			URL, err := buildMockContainerManifestURL(imageRef)
			Expect(err).To(HaveOccurred())
			Expect(URL).To(BeEmpty())
		})
	})
})

func buildMockContainerManifestURL(imageRef string) (string, error) {
	imageInfo := apiTypes.ImageInspect{
		RepoTags: []string{
			imageRef,
		},
	}
	mockID := "mock-id"
	mockName := "mock-container"
	mockCreated := time.Now()
	mock := mocks.CreateMockContainerWithImageInfo(mockID, mockName, imageRef, mockCreated, imageInfo)

	return manifest.BuildManifestURL(mock)
}
