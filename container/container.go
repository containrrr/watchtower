package container

import (
	"fmt"
	"strings"

	"github.com/samalba/dockerclient"
)

func NewContainer(containerInfo *dockerclient.ContainerInfo, imageInfo *dockerclient.ImageInfo) *Container {
	return &Container{
		containerInfo: containerInfo,
		imageInfo:     imageInfo,
	}
}

type Container struct {
	Stale bool

	containerInfo *dockerclient.ContainerInfo
	imageInfo     *dockerclient.ImageInfo
}

func (c Container) Name() string {
	return c.containerInfo.Name
}

func (c Container) Links() []string {
	links := []string{}

	if (c.containerInfo != nil) && (c.containerInfo.HostConfig != nil) {
		for _, link := range c.containerInfo.HostConfig.Links {
			name := strings.Split(link, ":")[0]
			links = append(links, name)
		}
	}

	return links
}

func (c Container) IsWatchtower() bool {
	val, ok := c.containerInfo.Config.Labels["com.centurylinklabs.watchtower"]
	return ok && val == "true"
}

// Ideally, we'd just be able to take the ContainerConfig from the old container
// and use it as the starting point for creating the new container; however,
// the ContainerConfig that comes back from the Inspect call merges the default
// configuration (the stuff specified in the metadata for the image itself)
// with the overridden configuration (the stuff that you might specify as part
// of the "docker run"). In order to avoid unintentionally overriding the
// defaults in the new image we need to separate the override options from the
// default options. To do this we have to compare the ContainerConfig for the
// running container with the ContainerConfig from the image that container was
// started from. This function returns a ContainerConfig which contains just
// the options overridden at runtime.
func (c Container) runtimeConfig() *dockerclient.ContainerConfig {
	config := c.containerInfo.Config
	imageConfig := c.imageInfo.Config

	if config.WorkingDir == imageConfig.WorkingDir {
		config.WorkingDir = ""
	}

	if config.User == imageConfig.User {
		config.User = ""
	}

	if sliceEqual(config.Cmd, imageConfig.Cmd) {
		config.Cmd = nil
	}

	if sliceEqual(config.Entrypoint, imageConfig.Entrypoint) {
		config.Entrypoint = nil
	}

	config.Env = sliceSubtract(config.Env, imageConfig.Env)

	config.Labels = stringMapSubtract(config.Labels, imageConfig.Labels)

	config.Volumes = structMapSubtract(config.Volumes, imageConfig.Volumes)

	config.ExposedPorts = structMapSubtract(config.ExposedPorts, imageConfig.ExposedPorts)
	for p, _ := range c.containerInfo.HostConfig.PortBindings {
		config.ExposedPorts[p] = struct{}{}
	}

	return config
}

// Any links in the HostConfig need to be re-written before they can be
// re-submitted to the Docker create API.
func (c Container) hostConfig() *dockerclient.HostConfig {
	hostConfig := c.containerInfo.HostConfig

	for i, link := range hostConfig.Links {
		name := link[0:strings.Index(link, ":")]
		alias := link[strings.LastIndex(link, "/"):]

		hostConfig.Links[i] = fmt.Sprintf("%s:%s", name, alias)
	}

	return hostConfig
}
