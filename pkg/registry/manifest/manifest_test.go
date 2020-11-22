package manifest_test

import (
	"github.com/containrrr/watchtower/internal/actions/mocks"
	"github.com/containrrr/watchtower/pkg/registry/manifest"
	apiTypes "github.com/docker/docker/api/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

var _ = Describe("the manifest module", func() {
	mockId := "mock-id"
	mockName := "mock-container"
	mockCreated := time.Now()

	When("building a manifest url", func() {
		It("should return a valid url given a fully qualified image", func() {
			expected := "https://ghcr.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{
				RepoTags: []string{
					"ghcr.io/k6io/operator:latest",
				},
			}
			mock := mocks.CreateMockContainerWithImageInfo(mockId, mockName, "ghcr.io/containrrr/watchtower:latest", mockCreated, imageInfo)
			res, err := manifest.BuildManifestURL(mock)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should assume dockerhub for non-qualified images", func() {
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{
				RepoTags: []string{
					"containrrr/watchtower:latest",
				},
			}

			mock := mocks.CreateMockContainerWithImageInfo(mockId, mockName, "containrrr/watchtower:latest", mockCreated, imageInfo)
			res, err := manifest.BuildManifestURL(mock)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should assume latest for images that lack an explicit tag", func() {
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{

				RepoTags: []string{
					"containrrr/watchtower",
				},
			}

			mock := mocks.CreateMockContainerWithImageInfo(mockId, mockName, "containrrr/watchtower", mockCreated, imageInfo)

			res, err := manifest.BuildManifestURL(mock)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
	})

})
