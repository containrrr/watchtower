package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// TestWarnOnAPIConsumption verifies that WarnOnAPIConsumption returns true for
// registries that support HEAD requests (Docker Hub, GHCR) and false otherwise.
func TestWarnOnAPIConsumption(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		want      bool
	}{
		{
			name:      "ghcr.io image",
			imageName: "ghcr.io/nicholas-fedor/watchtower",
			want:      true,
		},
		{
			name:      "implicit docker hub image",
			imageName: "docker:latest",
			want:      true,
		},
		{
			name:      "explicit index.docker.io image",
			imageName: "index.docker.io/docker:latest",
			want:      true,
		},
		{
			name:      "explicit docker.io image",
			imageName: "docker.io/docker:latest",
			want:      true,
		},
		{
			name:      "other registry fsf.org",
			imageName: "docker.fsf.org/docker:latest",
			want:      false,
		},
		{
			name:      "other registry altavista.com",
			imageName: "altavista.com/docker:latest",
			want:      false,
		},
		{
			name:      "other registry gitlab.com",
			imageName: "gitlab.com/docker:latest",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := mockTypes.NewMockContainer(t)
			container.EXPECT().Name().Return("test-container").Maybe()
			container.EXPECT().ImageName().Return(tt.imageName)

			got := WarnOnAPIConsumption(newTestLogger(t), container)
			assert.Equal(t, tt.want, got)
		})
	}
}
