package container

import (
	"github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

type MockContainerUpdate func(*types.ContainerJSON, *types.ImageInspect)

func MockContainer(updates ...MockContainerUpdate) *Container {
	containerInfo := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         "container_id",
			Image:      "image",
			Name:       "test-containrrr",
			HostConfig: &dockerContainer.HostConfig{},
		},
		Config: &dockerContainer.Config{
			Labels: map[string]string{},
		},
	}
	image := types.ImageInspect{
		ID: "image_id",
	}

	for _, update := range updates {
		update(&containerInfo, &image)
	}
	return NewContainer(&containerInfo, &image)
}

func WithPortBindings(portBindingSources ...string) MockContainerUpdate {
	return func(c *types.ContainerJSON, i *types.ImageInspect) {
		portBindings := nat.PortMap{}
		for _, pbs := range portBindingSources {
			portBindings[nat.Port(pbs)] = []nat.PortBinding{}
		}
		c.HostConfig.PortBindings = portBindings
	}
}

func WithImageName(name string) MockContainerUpdate {
	return func(c *types.ContainerJSON, i *types.ImageInspect) {
		c.Config.Image = name
		i.RepoTags = append(i.RepoTags, name)
	}
}

func WithLinks(links []string) MockContainerUpdate {
	return func(c *types.ContainerJSON, i *types.ImageInspect) {
		c.HostConfig.Links = links
	}
}

func WithLabels(labels map[string]string) MockContainerUpdate {
	return func(c *types.ContainerJSON, i *types.ImageInspect) {
		c.Config.Labels = labels
	}
}

func WithContainerState(state types.ContainerState) MockContainerUpdate {
	return func(cnt *types.ContainerJSON, img *types.ImageInspect) {
		cnt.State = &state
	}
}
