package container

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containrrr/watchtower/internal/util"

	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
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
	LinkedToRestarting bool
	Stale              bool

	containerInfo *types.ContainerJSON
	imageInfo     *types.ImageInspect
}

// ContainerInfo fetches JSON info for the container
func (c Container) ContainerInfo() *types.ContainerJSON {
	return c.containerInfo
}

// ID returns the Docker container ID.
func (c Container) ID() string {
	return c.containerInfo.ID
}

// IsRunning returns a boolean flag indicating whether or not the current
// container is running. The status is determined by the value of the
// container's "State.Running" property.
func (c Container) IsRunning() bool {
	return c.containerInfo.State.Running
}

// IsRestarting returns a boolean flag indicating whether or not the current
// container is restarting. The status is determined by the value of the
// container's "State.Restarting" property.
func (c Container) IsRestarting() bool {
	return c.containerInfo.State.Restarting
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
	imageName, ok := c.getLabelValue(zodiacLabel)
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
	rawBool, ok := c.getLabelValue(enableLabel)
	if !ok {
		return false, false
	}

	parsedBool, err := strconv.ParseBool(rawBool)
	if err != nil {
		return false, false
	}

	return parsedBool, true
}

// IsMonitorOnly returns the value of the monitor-only label. If the label
// is not set then false is returned.
func (c Container) IsMonitorOnly() bool {
	rawBool, ok := c.getLabelValue(monitorOnlyLabel)
	if !ok {
		return false
	}

	parsedBool, err := strconv.ParseBool(rawBool)
	if err != nil {
		return false
	}

	return parsedBool
}

// Scope returns the value of the scope UID label and if the label
// was set.
func (c Container) Scope() (string, bool) {
	rawString, ok := c.getLabelValue(scope)
	if !ok {
		return "", false
	}

	return rawString, true
}

// Links returns a list containing the names of all the containers to which
// this container is linked.
func (c Container) Links() []string {
	var links []string

	dependsOnLabelValue := c.getLabelValueOrEmpty(dependsOnLabel)

	if dependsOnLabelValue != "" {
		links := strings.Split(dependsOnLabelValue, ",")
		return links
	}

	if (c.containerInfo != nil) && (c.containerInfo.HostConfig != nil) {
		for _, link := range c.containerInfo.HostConfig.Links {
			name := strings.Split(link, ":")[0]
			links = append(links, name)
		}
	}

	return links
}

// ToRestart return whether the container should be restarted, either because
// is stale or linked to another stale container.
func (c Container) ToRestart() bool {
	return c.Stale || c.LinkedToRestarting
}

// IsWatchtower returns a boolean flag indicating whether or not the current
// container is the watchtower container itself. The watchtower container is
// identified by the presence of the "com.centurylinklabs.watchtower" label in
// the container metadata.
func (c Container) IsWatchtower() bool {
	return ContainsWatchtowerLabel(c.containerInfo.Config.Labels)
}

// PreUpdateTimeout checks whether a container has a specific timeout set
// for how long the pre-update command is allowed to run. This value is expressed
// either as an integer, in minutes, or as 0 which will allow the command/script
// to run indefinitely. Users should be cautious with the 0 option, as that
// could result in watchtower waiting forever.
func (c Container) PreUpdateTimeout() int {
	var minutes int
	var err error

	val := c.getLabelValueOrEmpty(preUpdateTimeoutLabel)

	minutes, err = strconv.Atoi(val)
	if err != nil || val == "" {
		return 1
	}

	return minutes
}

// StopSignal returns the custom stop signal (if any) that is encoded in the
// container's metadata. If the container has not specified a custom stop
// signal, the empty string "" is returned.
func (c Container) StopSignal() string {
	return c.getLabelValueOrEmpty(signalLabel)
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
	hostConfig := c.containerInfo.HostConfig
	imageConfig := c.imageInfo.Config

	if config.WorkingDir == imageConfig.WorkingDir {
		config.WorkingDir = ""
	}

	if config.User == imageConfig.User {
		config.User = ""
	}

	if hostConfig.NetworkMode.IsContainer() {
		config.Hostname = ""
	}

	if util.SliceEqual(config.Entrypoint, imageConfig.Entrypoint) {
		config.Entrypoint = nil
		if util.SliceEqual(config.Cmd, imageConfig.Cmd) {
			config.Cmd = nil
		}
	}

	config.Env = util.SliceSubtract(config.Env, imageConfig.Env)

	config.Labels = util.StringMapSubtract(config.Labels, imageConfig.Labels)

	config.Volumes = util.StructMapSubtract(config.Volumes, imageConfig.Volumes)

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

// HasImageInfo returns whether image information could be retrieved for the container
func (c Container) HasImageInfo() bool {
	return c.imageInfo != nil
}

// ImageInfo fetches the ImageInspect data of the current container
func (c Container) ImageInfo() *types.ImageInspect {
	return c.imageInfo
}

// VerifyConfiguration checks the container and image configurations for nil references to make sure
// that the container can be recreated once deleted
func (c Container) VerifyConfiguration() error {
	if c.imageInfo == nil {
		return errorNoImageInfo
	}

	containerInfo := c.ContainerInfo()
	if containerInfo == nil {
		return errorInvalidConfig
	}

	containerConfig := containerInfo.Config
	if containerConfig == nil {
		return errorInvalidConfig
	}

	hostConfig := containerInfo.HostConfig
	if hostConfig == nil {
		return errorInvalidConfig
	}

	if len(hostConfig.PortBindings) > 0 && containerConfig.ExposedPorts == nil {
		return errorNoExposedPorts
	}

	return nil
}
