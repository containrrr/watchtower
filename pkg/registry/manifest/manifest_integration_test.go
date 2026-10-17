// Package manifest_test provides tests for constructing manifest URLs in Watchtower.
// It verifies the behavior of BuildManifestURLForImage across various image reference scenarios.
package manifest_test

import (
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	dockerImage "github.com/moby/moby/api/types/image"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/manifest"
)

// testLog returns a logger that discards every event, for tests that do not
// assert on logs.
func testLog() *zerolog.Logger {
	return new(zerolog.Nop())
}

func TestManifest(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Manifest Suite")
}

// testScheme is the URL scheme the suite passes to BuildManifestURLForImage. Specs
// change it to exercise plain HTTP manifest URLs.
var testScheme string

var _ = ginkgo.BeforeEach(func() {
	// Build HTTPS manifest URLs unless a spec asks for HTTP.
	testScheme = "https"
})

var _ = ginkgo.Describe("the manifest module", func() {
	ginkgo.Describe("BuildManifestURLForImage", func() {
		ginkgo.It("should return a valid url given a fully qualified image", func() {
			imageRef := "ghcr.io/nicholas-fedor/watchtower:mytag"
			expected := "https://ghcr.io/v2/nicholas-fedor/watchtower/manifests/mytag"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should assume Docker Hub for image refs with no explicit registry", func() {
			imageRef := "nickfedor/watchtower:latest"
			expected := "https://index.docker.io/v2/nickfedor/watchtower/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should assume latest for image refs with no explicit tag", func() {
			imageRef := "nickfedor/watchtower"
			expected := "https://index.docker.io/v2/nickfedor/watchtower/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It(
			"should not prepend library/ for single-part container names in registries other than Docker Hub",
			func() {
				imageRef := "docker-registry.domain/imagename:latest"
				expected := "https://docker-registry.domain/v2/imagename/manifests/latest"

				URL, err := buildMockContainerManifestURL(imageRef)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(URL).To(gomega.Equal(expected))
			},
		)

		ginkgo.It("should return a valid URL for pinned images with digest", func() {
			imageRef := "docker-registry.domain/imagename@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7"
			expected := "https://docker-registry.domain/v2/imagename/manifests/sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should return an error for invalid image references", func() {
			imageRef := "invalid:image:ref:!!"
			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
			gomega.Expect(URL).To(gomega.BeEmpty())
		})

		ginkgo.It("should prepend library/ for Docker Hub official images", func() {
			imageRef := "nginx:latest"
			expected := "https://index.docker.io/v2/library/nginx/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should handle hosts with ports correctly", func() {
			imageRef := "localhost:5000/repo/image:tag"
			expected := "https://localhost:5000/v2/repo/image/manifests/tag"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should use the HTTP scheme when requested", func() {
			testScheme = "http"

			imageRef := "ghcr.io/nicholas-fedor/watchtower:mytag"
			expected := "http://ghcr.io/v2/nicholas-fedor/watchtower/manifests/mytag"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should handle tags with hyphens and numbers", func() {
			imageRef := "registry.example.com/my-project/my-app:v1.2.3-rc1"
			expected := "https://registry.example.com/v2/my-project/my-app/manifests/v1.2.3-rc1"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should handle multi-level image paths", func() {
			imageRef := "registry.example.com/org/team/project/image:tag"
			expected := "https://registry.example.com/v2/org/team/project/image/manifests/tag"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should return an error for uppercase registry names", func() {
			imageRef := "REGISTRY.EXAMPLE.COM/IMAGE:TAG"
			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
			gomega.Expect(URL).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle image names with dots and dashes", func() {
			imageRef := "registry.example.com/my.project/my-app:latest"
			expected := "https://registry.example.com/v2/my.project/my-app/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should handle Docker Hub official images with namespace", func() {
			imageRef := "docker.io/library/nginx:latest"
			expected := "https://index.docker.io/v2/library/nginx/manifests/latest"

			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(URL).To(gomega.Equal(expected))
		})

		ginkgo.It("should return an error for references with invalid digest format", func() {
			imageRef := "registry.example.com/image@sha256:invalid-digest"
			URL, err := buildMockContainerManifestURL(imageRef)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to parse image name"))
			gomega.Expect(URL).To(gomega.BeEmpty())
		})
	})
})

// buildMockContainerManifestURL creates a mock container and builds its manifest URL.
// It constructs a container with the given image reference for testing BuildManifestURLForImage.
func buildMockContainerManifestURL(imageRef string) (string, error) {
	imageInfo := dockerImage.InspectResponse{
		RepoTags: []string{
			imageRef,
		},
	}
	mockID := "mock-id"
	mockName := "mock-container"
	mockCreated := time.Now()
	mock := mockActions.CreateMockContainerWithImageInfo(
		mockID,
		mockName,
		imageRef,
		mockCreated,
		imageInfo,
	)

	return manifest.BuildManifestURLForImage(testLog(), mock.ImageName(), mock.Name(), testScheme)
}
