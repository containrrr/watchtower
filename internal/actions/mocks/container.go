package mocks

import (
	"time"

	"github.com/containrrr/watchtower/pkg/container"
	"github.com/docker/docker/api/types"
	container2 "github.com/docker/docker/api/types/container"
)

// CreateMockContainer creates a container substitute valid for testing
func CreateMockContainer(id string, name string, image string, created time.Time) container.Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         id,
			Image:      image,
			Name:       name,
			Created:    created.String(),
			HostConfig: &container2.HostConfig{},
		},
		Config: &container2.Config{
			Labels: make(map[string]string),
		},
	}
	return *container.NewContainer(
		&content,
		&types.ImageInspect{
			ID: image,
		},
	)
}
