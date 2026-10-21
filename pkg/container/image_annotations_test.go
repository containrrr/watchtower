package container

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	cerrdefs "github.com/containerd/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
)

// newAnnotationsAPI returns a mocked Docker API that serves imageConfig for one
// inspect of app:latest. A nil config models an image the daemon reports
// without an OCI config.
//
// Parameters:
//   - t: Test the mock belongs to.
//   - imageConfig: OCI config the inspected image reports.
//
// Returns:
//   - *mockContainer.MockAPIClient: Mocked Docker API.
func newAnnotationsAPI(t *testing.T, imageConfig *dockerspec.DockerOCIImageConfig) *mockContainer.MockAPIClient {
	t.Helper()

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().ImageInspect(mock.Anything, "app:latest").
		Return(dockerClient.ImageInspectResult{Config: imageConfig}, nil).Once()

	return api
}

// TestImageClientImageAnnotations verifies that the image client reads OCI
// annotations from the image config labels, and that a missing config, an
// empty reference, or a failed inspect yields empty annotations.
func TestImageClientImageAnnotations(t *testing.T) {
	t.Parallel()

	t.Run("reads every OCI annotation from image config labels", func(t *testing.T) {
		t.Parallel()

		api := newAnnotationsAPI(t, &dockerspec.DockerOCIImageConfig{
			Labels: map[string]string{
				oci.SourceLabel:        "https://github.com/org/app.git",
				oci.URLLabel:           "https://example.com/image",
				oci.DocumentationLabel: "https://example.com/docs",
				oci.RevisionLabel:      "abc123",
				oci.VersionLabel:       "1.2.3",
			},
		})

		got := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New()).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, "https://github.com/org/app.git", got.Source)
		assert.Equal(t, "https://example.com/image", got.URL)
		assert.Equal(t, "https://example.com/docs", got.Documentation)
		assert.Equal(t, "abc123", got.Revision)
		assert.Equal(t, "1.2.3", got.Version)
	})

	t.Run("an image without OCI labels yields empty annotations", func(t *testing.T) {
		t.Parallel()

		api := newAnnotationsAPI(t, &dockerspec.DockerOCIImageConfig{
			Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
		})

		got := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New()).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, oci.Annotations{}, got)
	})

	t.Run("nil image config yields empty annotations", func(t *testing.T) {
		t.Parallel()

		api := newAnnotationsAPI(t, nil)

		got := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New()).
			GetImageAnnotations(t.Context(), "app:latest")

		assert.Equal(t, oci.Annotations{}, got)
	})

	t.Run("an empty image reference never reaches the API", func(t *testing.T) {
		t.Parallel()

		// The mock has no expectations, so an inspect fails the test.
		api := mockContainer.NewMockAPIClient(t)

		got := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New()).
			GetImageAnnotations(t.Context(), "")

		assert.Equal(t, oci.Annotations{}, got)
	})

	t.Run("a failed inspect yields empty annotations rather than an error", func(t *testing.T) {
		t.Parallel()

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().ImageInspect(mock.Anything, "missing:latest").
			Return(dockerClient.ImageInspectResult{}, fmt.Errorf("no such image: %w", cerrdefs.ErrNotFound)).Once()

		got := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New()).
			GetImageAnnotations(t.Context(), "missing:latest")

		assert.Equal(t, oci.Annotations{}, got)
	})
}

// TestClientImageAnnotationsDelegates verifies that the client-level delegate
// reaches the same result as the image client.
func TestClientImageAnnotationsDelegates(t *testing.T) {
	t.Parallel()

	api := newAnnotationsAPI(t, &dockerspec.DockerOCIImageConfig{
		Labels: map[string]string{oci.VersionLabel: "1.2.3"},
	})

	got := (&client{api: api, log: newTestLogger(t)}).
		GetImageAnnotations(t.Context(), "app:latest")

	assert.Equal(t, "1.2.3", got.Version)
}
