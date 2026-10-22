package git

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	typemocks "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func TestLastStamp(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		commit, tag := LastStamp(nil)
		assert.Empty(t, commit)
		assert.Empty(t, tag)
	})

	t.Run("reads labels", func(t *testing.T) {
		t.Parallel()

		commit, tag := LastStamp(testContainer(t, map[string]string{
			LastCommitLabel: "  abc123  ",
			LastTagLabel:    "v1.2.3",
		}, "app:latest"))
		assert.Equal(t, "abc123", commit)
		assert.Equal(t, "v1.2.3", tag)
	})

	t.Run("missing labels", func(t *testing.T) {
		t.Parallel()

		commit, tag := LastStamp(testContainer(t, nil, "app:latest"))
		assert.Empty(t, commit)
		assert.Empty(t, tag)
	})
}

func TestBaseline(t *testing.T) {
	t.Parallel()

	t.Run("stamp wins over image", func(t *testing.T) {
		t.Parallel()

		commit, tag := Baseline(testContainer(t, map[string]string{
			LastCommitLabel: "stampsha",
			LastTagLabel:    "v1.0.0",
		}, "app:git-aaa111bbbbcc"))
		assert.Equal(t, "stampsha", commit)
		assert.Equal(t, "v1.0.0", tag)
	})

	t.Run("image revision when no stamp", func(t *testing.T) {
		t.Parallel()

		commit, tag := Baseline(imageContainer(t, "app:latest", "oci123", "v1.2.3"))
		assert.Equal(t, "oci123", commit)
		assert.Equal(t, "v1.2.3", tag)
	})

	t.Run("empty when unknown", func(t *testing.T) {
		t.Parallel()

		commit, tag := Baseline(imageContainer(t, "app:latest", "", ""))
		assert.Empty(t, commit)
		assert.Empty(t, tag)
	})
}

func TestImageRevision(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		commit, tag := imageRevision(nil)
		assert.Empty(t, commit)
		assert.Empty(t, tag)
	})

	t.Run("git shortsha image name", func(t *testing.T) {
		t.Parallel()

		commit, tag := imageRevision(imageContainer(t, "localhost:5000/app:git-aaa111bbbbcc", "", ""))
		assert.Equal(t, "aaa111bbbbcc", commit)
		assert.Empty(t, tag)
	})

	t.Run("ignores non git tags", func(t *testing.T) {
		t.Parallel()

		commit, tag := imageRevision(imageContainer(t, "app:latest", "", ""))
		assert.Empty(t, commit)
		assert.Empty(t, tag)
	})
}

func TestGitSHAFromRef(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "aaa111bbbbcc", gitSHAFromRef("app:git-aaa111bbbbcc"))
	assert.Equal(t, "aaa111bbbbcc", gitSHAFromRef("localhost:5000/app:git-aaa111bbbbcc"))
	assert.Empty(t, gitSHAFromRef("app:latest"))
	assert.Empty(t, gitSHAFromRef("app:git-short"))
	assert.Empty(t, gitSHAFromRef("app@sha256:deadbeef"))
	assert.Empty(t, gitSHAFromRef(""))
}

// imageContainer returns a mock with an image name and optional OCI annotations.
//
// Parameters:
//   - t: Test handle.
//   - imageName: ImageName() result.
//   - revision: OCI revision label, or empty.
//   - version: OCI version label, or empty.
//
// Returns:
//   - *typemocks.MockContainer: Configured mock.
func imageContainer(t *testing.T, imageName, revision, version string) *typemocks.MockContainer {
	t.Helper()

	c := typemocks.NewMockContainer(t)
	c.EXPECT().GetLabel(LastCommitLabel).Return("", false).Maybe()
	c.EXPECT().GetLabel(LastTagLabel).Return("", false).Maybe()
	c.EXPECT().ImageName().Return(imageName).Maybe()

	if revision == "" && version == "" {
		c.EXPECT().HasImageInfo().Return(false).Maybe()

		return c
	}

	cfg := &dockerspec.DockerOCIImageConfig{}

	cfg.Labels = map[string]string{}
	if revision != "" {
		cfg.Labels[oci.RevisionLabel] = revision
	}

	if version != "" {
		cfg.Labels[oci.VersionLabel] = version
	}

	c.EXPECT().HasImageInfo().Return(true).Maybe()
	c.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{Config: cfg}).Maybe()

	return c
}
