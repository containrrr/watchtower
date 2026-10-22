package release

import (
	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// testContainer returns a minimal container reference for probe tests.
func testContainer() types.Container {
	return stubContainer{imageName: "org/app:latest"}
}

// pinnedContainer returns a container whose image is pinned to a digest.
func pinnedContainer() types.Container {
	return stubContainer{imageName: "org/app@sha256:abc"}
}

// stubContainer is the smallest types.Container the probe needs.
type stubContainer struct {
	types.Container

	imageName string
}

func (c stubContainer) ID() types.ContainerID { return "stub" }

func (c stubContainer) Name() string { return "/stub" }

func (c stubContainer) ImageName() string { return c.imageName }

func (c stubContainer) ImageID() types.ImageID { return "sha256:image" }

func (c stubContainer) ContainerInfo() *dockerContainer.InspectResponse {
	return &dockerContainer.InspectResponse{
		ID:     "stub",
		Config: &dockerContainer.Config{Image: c.imageName},
	}
}

func (c stubContainer) HasImageInfo() bool { return false }

func (c stubContainer) IsNoPull(_ types.UpdateParams) bool { return false }
