package manifest_test

import (
	"github.com/containrrr/watchtower/pkg/registry/manifest"
	apiTypes "github.com/docker/docker/api/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifest Suite")
}

var _ = Describe("the manifest module", func() {

	When("building a manifest url", func() {
		It("should return a valid url given a fully qualified image", func() {
			expected := "https://ghcr.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{
				RepoTags: []string {
					"ghcr.io/containrrr/watchtower:latest",
				},
			}

			res, err := manifest.BuildManifestURL(imageInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should assume dockerhub for non-qualified images", func() {
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{
				RepoTags: []string {
					"containrrr/watchtower:latest",
				},
			}

			res, err := manifest.BuildManifestURL(imageInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
		It("should assume latest for images that lack an explicit tag", func() {
			expected := "https://index.docker.io/v2/containrrr/watchtower/manifests/latest"
			imageInfo := apiTypes.ImageInspect{
				RepoTags: []string {
					"containrrr/watchtower",
				},
			}

			res, err := manifest.BuildManifestURL(imageInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expected))
		})
	})

})
