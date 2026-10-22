// Package mocks provides mock implementations for testing Watchtower components.
package mocks

import (
	"fmt"
	"strings"
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// CreateMockContainer creates a container substitute valid for testing.
// It initializes a basic container with the specified ID, name, image, and creation time.
func CreateMockContainer(id, name, image string, created time.Time) types.Container {
	content := dockerContainer.InspectResponse{
		ID:      id,
		Image:   image,
		Name:    name,
		Created: created.Format(time.RFC3339Nano), // Use RFC3339Nano format
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{
			Image:        image,
			Labels:       make(map[string]string),
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	return container.NewContainer(nil,
		&content,
		CreateMockImageInfo(image),
	)
}

// CreateMockImageInfo returns a mock image info struct based on the passed image.
// It provides a minimal image representation for testing purposes.
func CreateMockImageInfo(mockImage string) *dockerImage.InspectResponse {
	return &dockerImage.InspectResponse{
		ID: mockImage,
		RepoDigests: []string{
			fmt.Sprintf(
				"%s@sha256:%s",
				mockImage,
				strings.ReplaceAll(mockImage, ":", "_"),
			), // Unique digest
		},
	}
}

// CreateMockContainerWithImageInfoP creates a container with a pointer to custom image info for testing.
// It uses a pointer to the image info struct, suitable for cases where the info is pre-allocated.
func CreateMockContainerWithImageInfoP(
	id string,
	name string,
	image string,
	created time.Time,
	imageInfo *dockerImage.InspectResponse,
) types.Container {
	content := dockerContainer.InspectResponse{
		ID:      id,
		Image:   image,
		Name:    name,
		Created: created.Format(time.RFC3339Nano),
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{
			Image:        image,
			Labels:       make(map[string]string),
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	return container.NewContainer(nil,
		&content,
		imageInfo,
	)
}

// CreateMockContainerWithConfig creates a container substitute with custom configuration for testing.
// It allows specifying running/restarting states and config details for flexible test scenarios.
func CreateMockContainerWithConfig(
	id string,
	name string,
	image string,
	running bool,
	restarting bool,
	created time.Time,
	config *dockerContainer.Config,
) types.Container {
	content := dockerContainer.InspectResponse{
		ID:    id,
		Image: image,
		Name:  name,
		State: &dockerContainer.State{
			Running:    running,
			Restarting: restarting,
		},
		Created: created.Format(time.RFC3339Nano), // Use RFC3339Nano format
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: config,
	}

	imageInfo := CreateMockImageInfo(image)

	return container.NewContainer(nil,
		&content,
		imageInfo,
	)
}

// CreateMockContainerWithLinks creates a container with specified links for testing.
// It is useful for testing dependency-related behaviors in Watchtower.
func CreateMockContainerWithLinks(
	id string,
	name string,
	image string,
	created time.Time,
	links []string,
	imageInfo *dockerImage.InspectResponse,
) types.Container {
	content := dockerContainer.InspectResponse{
		ID:      id,
		Image:   image,
		Name:    name,
		Created: created.Format(time.RFC3339Nano),
		HostConfig: &dockerContainer.HostConfig{
			Links:        links,
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{
			Image:        image,
			Labels:       make(map[string]string),
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	return container.NewContainer(nil,
		&content,
		imageInfo,
	)
}
