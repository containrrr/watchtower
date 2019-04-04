package container

import (
	"fmt"
	"strconv"
	"strings"
	log "github.com/sirupsen/logrus"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
)

const (
	watchtowerLabel = "com.centurylinklabs.watchtower"
	signalLabel     = "com.centurylinklabs.watchtower.stop-signal"
	enableLabel     = "com.centurylinklabs.watchtower.enable"
	zodiacLabel     = "com.centurylinklabs.zodiac.original-image"
)

// NewContainer returns a new Container instance instantiated with the
// specified ContainerInfo and ImageInfo structs.
func NewContainer(containerInfo *types.ContainerJSON, imageInfo *types.ImageInspect) *Container {
	return &Container{
		containerInfo: containerInfo,
		imageInfo:     imageInfo,
	}
}

// Container represents a running Docker container.
type Container struct {
	Stale bool

	containerInfo *types.ContainerJSON
	imageInfo     *types.ImageInspect
}

// ID returns the Docker container ID.
func (c Container) ID() string {
	return c.containerInfo.ID
}

// Name returns the Docker container name.
func (c Container) Name() string {
	return c.containerInfo.Name
}

// ImageID returns the ID of the Docker image that was used to start the
// container.
func (c Container) ImageID() string {
	return c.imageInfo.ID
}

// ImageName returns the name of the Docker image that was used to start the
// container. If the original image was specified without a particular tag, the
// "latest" tag is assumed.
func (c Container) ImageName() string {
	// Compatibility w/ Zodiac deployments
	imageName, ok := c.containerInfo.Config.Labels[zodiacLabel]
	if !ok {
		imageName = c.containerInfo.Config.Image
	}

	if !strings.Contains(imageName, ":") {
		imageName = fmt.Sprintf("%s:latest", imageName)
	}

	return imageName
}

// Enabled returns the value of the container enabled label and if the label
// was set.
func (c Container) Enabled() (bool, bool) {
	rawBool, ok := c.containerInfo.Config.Labels[enableLabel]
	if !ok {
		return false, false
	}

	parsedBool, err := strconv.ParseBool(rawBool)
	if err != nil {
		return false, false
	}

	return parsedBool, true
}

// Links returns a list containing the names of all the containers to which
// this container is linked.
func (c Container) Links() []string {
	var links []string

	if (c.containerInfo != nil) && (c.containerInfo.HostConfig != nil) {
		for _, link := range c.containerInfo.HostConfig.Links {
			name := strings.Split(link, ":")[0]
			links = append(links, name)
		}
	}

	return links
}

// IsWatchtower returns a boolean flag indicating whether or not the current
// container is the watchtower container itself. The watchtower container is
// identified by the presence of the "com.centurylinklabs.watchtower" label in
// the container metadata.
func (c Container) IsWatchtower() bool {
	log.Debugf("Checking if %s is a watchtower instance.", c.Name())
	wasWatchtower := ContainsWatchtowerLabel(c.containerInfo.Config.Labels)
	return wasWatchtower
}

// StopSignal returns the custom stop signal (if any) that is encoded in the
// container's metadata. If the container has not specified a custom stop
// signal, the empty string "" is returned.
func (c Container) StopSignal() string {
	if val, ok := c.containerInfo.Config.Labels[signalLabel]; ok {
		return val
	}

	return ""
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
func (c Container) runtimeConfig() *dockercontainer.Config {
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

	// subtract ports exposed in image from container
	for k := range config.ExposedPorts {
		if _, ok := imageConfig.ExposedPorts[k]; ok {
			delete(config.ExposedPorts, k)
		}
	}
	for p := range c.containerInfo.HostConfig.PortBindings {
		config.ExposedPorts[p] = struct{}{}
	}

	config.Image = c.ImageName()
	return config
}

// Any links in the HostConfig need to be re-written before they can be
// re-submitted to the Docker create API.
func (c Container) hostConfig() *dockercontainer.HostConfig {
	hostConfig := c.containerInfo.HostConfig

	for i, link := range hostConfig.Links {
		name := link[0:strings.Index(link, ":")]
		alias := link[strings.LastIndex(link, "/"):]

		hostConfig.Links[i] = fmt.Sprintf("%s:%s", name, alias)
	}

	return hostConfig
}

// ContainsWatchtowerLabel takes a map of labels and values and tells
// the consumer whether it contains a valid watchtower instance label
func ContainsWatchtowerLabel(labels map[string]string) bool {
	val, ok := labels[watchtowerLabel]
	return ok && val == "true"
}