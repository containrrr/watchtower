package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerImage "github.com/moby/moby/api/types/image"

	typemocks "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func TestRead(t *testing.T) {
	t.Parallel()

	t.Run("nil container", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, Annotations{}, Read(nil))
	})

	t.Run("no image info", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().HasImageInfo().Return(false)
		assert.Equal(t, Annotations{}, Read(c))
	})

	t.Run("nil image inspect", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().HasImageInfo().Return(true)
		c.EXPECT().ImageInfo().Return(nil)
		assert.Equal(t, Annotations{}, Read(c))
	})

	t.Run("nil image config", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().HasImageInfo().Return(true)
		c.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{})
		assert.Equal(t, Annotations{}, Read(c))
	})

	t.Run("nil labels", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().HasImageInfo().Return(true)
		c.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{
			Config: &dockerspec.DockerOCIImageConfig{},
		})
		assert.Equal(t, Annotations{}, Read(c))
	})

	t.Run("reads and trims labels", func(t *testing.T) {
		t.Parallel()

		c := typemocks.NewMockContainer(t)
		c.EXPECT().HasImageInfo().Return(true)

		cfg := &dockerspec.DockerOCIImageConfig{}
		cfg.Labels = map[string]string{
			SourceLabel:        "  https://github.com/org/app  ",
			URLLabel:           "https://example.com/image",
			DocumentationLabel: "https://example.com/docs",
			RevisionLabel:      "abc123",
			VersionLabel:       "v1.2.3",
		}
		c.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{Config: cfg})

		assert.Equal(t, Annotations{
			Source:        "https://github.com/org/app",
			URL:           "https://example.com/image",
			Documentation: "https://example.com/docs",
			Revision:      "abc123",
			Version:       "v1.2.3",
		}, Read(c))
	})
}

func TestFromLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels map[string]string
		want   Annotations
	}{
		{
			name:   "nil labels",
			labels: nil,
			want:   Annotations{},
		},
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   Annotations{},
		},
		{
			name:   "trims surrounding whitespace",
			labels: map[string]string{VersionLabel: "\t1.2.3\n", SourceLabel: " https://example.com/r "},
			want:   Annotations{Version: "1.2.3", Source: "https://example.com/r"},
		},
		{
			name:   "whitespace only becomes empty",
			labels: map[string]string{RevisionLabel: "   "},
			want:   Annotations{},
		},
		{
			name:   "ignores non-oci labels",
			labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			want:   Annotations{},
		},
		{
			name: "reads every annotation",
			labels: map[string]string{
				SourceLabel:        "https://github.com/org/app",
				URLLabel:           "https://example.com/image",
				DocumentationLabel: "https://example.com/docs",
				RevisionLabel:      "abc123",
				VersionLabel:       "1.2.3",
			},
			want: Annotations{
				Source:        "https://github.com/org/app",
				URL:           "https://example.com/image",
				Documentation: "https://example.com/docs",
				Revision:      "abc123",
				Version:       "1.2.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, FromLabels(tt.labels))
		})
	}
}

// Read must delegate to FromLabels so the new-image read path shares one implementation.
func TestReadDelegatesToFromLabels(t *testing.T) {
	t.Parallel()

	c := typemocks.NewMockContainer(t)
	c.EXPECT().HasImageInfo().Return(true)

	cfg := &dockerspec.DockerOCIImageConfig{}
	cfg.Labels = map[string]string{VersionLabel: "1.2.3"}
	c.EXPECT().ImageInfo().Return(&dockerImage.InspectResponse{Config: cfg})

	assert.Equal(t, FromLabels(cfg.Labels), Read(c))
}

func TestLooksLikeTag(t *testing.T) {
	t.Parallel()

	assert.True(t, LooksLikeTag("v1.2.3"))
	assert.True(t, LooksLikeTag("1.2.3"))
	assert.True(t, LooksLikeTag("  1.2.3  "))
	assert.False(t, LooksLikeTag("nightly"))
	assert.False(t, LooksLikeTag(""))
	assert.False(t, LooksLikeTag("  "))
}
