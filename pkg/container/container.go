package container

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Constants for container operations.
const (
	linkPartsCount = 2 // Number of parts expected in a link (name:alias)
)

// nopLog returns a fresh discarded logger for optional construction and fallbacks.
// It is not a package-level logger store: each call allocates a new Nop instance.
func nopLog() *zerolog.Logger {
	n := zerolog.Nop()

	return &n
}

// Container represents a running Docker container managed by Watchtower.
//
// It implements the types.Container interface, storing state and metadata
// for container operations such as updates and lifecycle hooks.
//

type Container struct {
	mu                 sync.RWMutex                     // Protects concurrent access to mutable fields
	LinkedToRestarting bool                             // Indicates if linked to a restarting container
	Stale              bool                             // Marks the container as having an outdated image
	OldImageID         types.ImageID                    // Stores the image ID before update for cleanup tracking
	normalizedName     string                           // Cached normalized container name
	imageName          string                           // Cached resolved image name with tag
	containerInfo      *dockerContainer.InspectResponse // Docker container metadata
	imageInfo          *dockerImage.InspectResponse     // Docker image metadata
	// log is the process logger for operational Warn/Error/Debug on this instance.
	// Set at construction (client list/get). Nil falls back to nopLog() so interface
	// methods never panic. Production paths always set a real logger.
	log *zerolog.Logger
}

// NewContainer creates a new Container instance with the specified metadata.
//
// The resolved image name is cached so later ImageName calls do not recompute it.
//
// Parameters:
//   - log: Process logger. A nop logger is used when nil.
//   - containerInfo: Docker container metadata.
//   - imageInfo: Docker image metadata.
//
// Returns:
//   - *Container: Initialized container instance.
func NewContainer(
	log *zerolog.Logger,
	containerInfo *dockerContainer.InspectResponse,
	imageInfo *dockerImage.InspectResponse,
) *Container {
	if log == nil {
		log = nopLog()
	}

	name := ""
	if containerInfo != nil {
		name = containerInfo.Name
	}
	// Initialize with default state.
	c := &Container{
		LinkedToRestarting: false,
		Stale:              false,
		OldImageID:         "",
		normalizedName:     util.NormalizeContainerName(name),
		containerInfo:      containerInfo,
		imageInfo:          imageInfo,
		log:                log,
	}
	// Cache the resolved image name at construct time.
	c.imageName = c.resolveImageName()
	c.logger().Debug().
		Str("container", c.Name()).
		Str("id", c.ID().ShortID()).
		Str("image", string(c.ImageID())).
		Msg("Created new container instance")

	return c
}

// IsLinkedToRestarting returns whether the container is linked to a restarting container.
//
// Returns:
//   - bool: True if linked, false otherwise.
func (c *Container) IsLinkedToRestarting() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.LinkedToRestarting
}

// SetLinkedToRestarting sets the linked-to-restarting state.
//
// Parameters:
//   - value: New state value.
func (c *Container) SetLinkedToRestarting(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.LinkedToRestarting = value
}

// IsStale returns whether the container's image is outdated.
//
// Returns:
//   - bool: True if stale, false otherwise.
func (c *Container) IsStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Stale
}

// SetStale marks the container as having an outdated image.
//
// Parameters:
//   - value: New stale value.
func (c *Container) SetStale(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Stale = value
}

// SetOldImageID sets the old image ID for cleanup tracking.
//
// Parameters:
//   - id: The old image ID.
func (c *Container) SetOldImageID(id types.ImageID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.OldImageID = id
}

// ToRestart determines if the container should be restarted.
//
// Returns:
//   - bool: True if stale or linked to restarting, false otherwise.
func (c *Container) ToRestart() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Stale || c.LinkedToRestarting
}

// ContainerInfo returns the full Docker container metadata.
//
// Returns:
//   - *dockerContainer.InspectResponse: Container metadata.
func (c *Container) ContainerInfo() *dockerContainer.InspectResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.containerInfo
}

// ID returns the unique identifier of the container.
//
// Returns:
//   - types.ContainerID: Container ID.
func (c *Container) ID() types.ContainerID {
	info := c.ContainerInfo()

	if info == nil {
		return ""
	}

	return types.ContainerID(info.ID)
}

// IsRunning checks if the container is currently running.
//
// Returns:
//   - bool: True if running, false otherwise.
func (c *Container) IsRunning() bool {
	info := c.ContainerInfo()

	if info == nil || info.State == nil {
		return false
	}

	return info.State.Running
}

// IsRestarting checks if the container is currently restarting.
//
// Returns:
//   - bool: True if restarting, false otherwise.
func (c *Container) IsRestarting() bool {
	info := c.ContainerInfo()

	if info == nil || info.State == nil {
		return false
	}

	return info.State.Restarting
}

// IsCreated checks if the container is in the Docker "created" state,
// meaning it was successfully created but never started.
//
// Returns:
//   - bool: True if the container is in created state, false otherwise.
func (c *Container) IsCreated() bool {
	info := c.ContainerInfo()

	if info == nil || info.State == nil {
		return false
	}

	return info.State.Status == dockerContainer.StateCreated
}

// Name returns the normalized name of the container.
//
// Returns:
//   - string: Normalized container name.
func (c *Container) Name() string {
	return c.normalizedName
}

// ImageID returns the ID of the container's image.
//
// Returns:
//   - types.ImageID: Image ID or empty string if imageInfo is nil.
func (c *Container) ImageID() types.ImageID {
	if c.imageInfo == nil {
		return ""
	}

	return types.ImageID(c.imageInfo.ID)
}

// ImageName returns the cached name of the container's image.
//
// The value is resolved once in NewContainer from the Zodiac label or Config.Image.
// When containerInfo is later cleared, the name is resolved again.
// Callers that already hold c.mu must use imageNameLocked.
//
// Returns:
//   - string: Image name (e.g., "alpine:latest").
func (c *Container) ImageName() string {
	if c == nil {
		return "unknown:latest"
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.imageNameLocked()
}

// SetImageName updates Config.Image and the cached image name used by ImageName.
//
// An untagged name receives a ":latest" suffix so ImageName stays consistent
// with resolveImageName. Registry host ports are not treated as tags.
//
// Parameters:
//   - name: Image reference to store.
func (c *Container) SetImageName(name string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	normalized := ensureImageTag(name)

	// Keep Config.Image in sync so GetCreateConfig uses the pinned reference.
	if c.containerInfo != nil && c.containerInfo.Config != nil {
		c.replaceInspectLocked(func(info *dockerContainer.InspectResponse) {
			info.Config.Image = normalized
		})
	}

	c.imageName = normalized
}

// HasImageInfo indicates whether image metadata is available.
//
// Returns:
//   - bool: True if imageInfo is non-nil, false otherwise.
func (c *Container) HasImageInfo() bool {
	return c.imageInfo != nil
}

// ImageInfo returns the Docker image metadata.
//
// Returns:
//   - *dockerImage.InspectResponse: Image metadata or nil if unavailable.
func (c *Container) ImageInfo() *dockerImage.InspectResponse {
	return c.imageInfo
}

// GetCreateConfig generates a container configuration for recreation.
//
// It isolates runtime overrides from image defaults and sets the image name.
//
// Returns:
//   - *dockerContainer.Config: Configuration for container creation.
func (c *Container) GetCreateConfig() *dockerContainer.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal

	if c.containerInfo == nil {
		clog.Warn().Msg("No container info available, returning minimal config")

		return &dockerContainer.Config{Image: c.imageNameLocked()}
	}

	config := *c.containerInfo.Config
	hostConfig := c.containerInfo.HostConfig

	// Handle missing image info, including a nil image Config. The containerd
	// image store can return image inspect with Config unset.
	if c.imageInfo == nil || c.imageInfo.Config == nil {
		clog.Warn().Msg("No image config available, using container config as-is")

		config.Image = c.imageNameLocked()

		return &config
	}

	// Compare with image config to clear defaults.
	imageConfig := c.imageInfo.Config
	if config.WorkingDir == imageConfig.WorkingDir {
		config.WorkingDir = ""
	}

	if config.User == imageConfig.User {
		config.User = ""
	}

	if hostConfig.NetworkMode.IsContainer() {
		config.Hostname = "" // Clear hostname for container network mode.
	}

	if hostConfig.UTSMode != "" {
		config.Hostname = "" // Clear hostname for UTS mode.
	}

	if slices.Equal(config.Entrypoint, imageConfig.Entrypoint) {
		config.Entrypoint = nil
		if slices.Equal(config.Cmd, imageConfig.Cmd) {
			config.Cmd = nil
		}
	}
	// Clear HEALTHCHECK if it matches the image default. Work on a copy, as
	// the health check is shared with the container's inspect data.
	if config.Healthcheck != nil && imageConfig.Healthcheck != nil {
		healthcheck := *config.Healthcheck
		config.Healthcheck = &healthcheck

		if slices.Equal(config.Healthcheck.Test, imageConfig.Healthcheck.Test) {
			config.Healthcheck.Test = nil
		}

		if config.Healthcheck.Retries == imageConfig.Healthcheck.Retries {
			config.Healthcheck.Retries = 0
		}

		if config.Healthcheck.Interval == imageConfig.Healthcheck.Interval {
			config.Healthcheck.Interval = 0
		}

		if config.Healthcheck.Timeout == imageConfig.Healthcheck.Timeout {
			config.Healthcheck.Timeout = 0
		}

		if config.Healthcheck.StartPeriod == imageConfig.Healthcheck.StartPeriod {
			config.Healthcheck.StartPeriod = 0
		}
	}

	// Subtract image defaults from config.
	config.Env = util.SliceSubtract(config.Env, imageConfig.Env)

	// Preserve the watchtower label if present, as it may be subtracted as an image default.
	watchtowerLabelValue, hasWatchtowerLabel := config.Labels[watchtowerLabel]

	config.Labels = util.StringMapSubtract(config.Labels, imageConfig.Labels)
	if hasWatchtowerLabel {
		if config.Labels == nil {
			config.Labels = make(map[string]string)
		}

		config.Labels[watchtowerLabel] = watchtowerLabelValue
	}

	config.Volumes = util.StructMapSubtract(config.Volumes, imageConfig.Volumes)

	// Ensure ExposedPorts is initialized before removing image-exposed ports
	// and adding ports from host config bindings. Work on a copy, as the map
	// is shared with the container's inspect data.
	config.ExposedPorts = maps.Clone(config.ExposedPorts)
	if config.ExposedPorts == nil {
		config.ExposedPorts = dockerNetwork.PortSet{}
	}

	for port := range config.ExposedPorts {
		_, ok := imageConfig.ExposedPorts[port.String()]
		if ok {
			delete(config.ExposedPorts, port) // Remove ports exposed by image.
		}
	}

	for p := range hostConfig.PortBindings {
		config.ExposedPorts[p] = struct{}{} // Add ports from bindings.
	}

	config.Image = c.imageNameLocked()
	clog.Debug().
		Str("image", config.Image).
		Msg("Generated create config")

	return &config
}

// GetCreateHostConfig generates a host configuration for recreation.
//
// It adjusts link formats for Docker API compatibility and copies slices that
// must not alias inspect data.
//
// Returns:
//   - *dockerContainer.HostConfig: Host configuration for container creation.
func (c *Container) GetCreateHostConfig() *dockerContainer.HostConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal

	if c.containerInfo == nil || c.containerInfo.HostConfig == nil {
		clog.Warn().Msg("No container host config available")

		return &dockerContainer.HostConfig{}
	}

	hostConfigCopy := *c.containerInfo.HostConfig
	hostConfig := &hostConfigCopy

	// Deep copy slices that will be mutated to avoid modifying the original
	// under a read lock.
	if len(hostConfig.Devices) > 0 {
		devicesCopy := make([]dockerContainer.DeviceMapping, len(hostConfig.Devices))
		copy(devicesCopy, hostConfig.Devices)
		hostConfig.Devices = devicesCopy
	}

	if len(hostConfig.VolumesFrom) > 0 {
		// Copy VolumesFrom so the returned host config does not alias inspect.
		volumesFromCopy := slices.Clone(hostConfig.VolumesFrom)
		hostConfig.VolumesFrom = volumesFromCopy
	}

	// Adjust link format for each entry (and drop invalid ones).
	adjusted := make([]string, 0, len(hostConfig.Links))
	for _, link := range hostConfig.Links {
		if !strings.Contains(link, ":") {
			clog.Error().
				Str("link", link).
				Msg("Invalid link format, expected 'name:alias'")

			continue
		}

		parts := strings.SplitN(link, ":", linkPartsCount)
		if len(parts) != linkPartsCount {
			clog.Error().
				Str("link", link).
				Msg("Invalid link format, expected exactly one colon separator")

			continue
		}

		normalizedName := util.NormalizeContainerName(parts[0])
		alias := parts[1]
		adjustedLink := fmt.Sprintf("%s:%s", normalizedName, alias)
		adjusted = append(adjusted, adjustedLink)
		clog.Debug().
			Str("link", adjustedLink).
			Msg("Adjusted link for host config")
	}

	hostConfig.Links = adjusted

	// Normalize device CgroupPermissions for Podman compatibility.
	//
	// Podman leaves CgroupPermissions empty in Docker API inspect responses.
	// Both Docker and Podman treat bare device specifications (without explicit
	// permissions) as "rwm". Defaulting here prevents "empty device mode"
	// errors when recreating containers.
	for i := range hostConfig.Devices {
		if hostConfig.Devices[i].CgroupPermissions == "" {
			hostConfig.Devices[i].CgroupPermissions = "rwm"
			clog.Debug().
				Str("device", hostConfig.Devices[i].PathOnHost).
				Msg("Defaulted empty device CgroupPermissions to 'rwm'")
		}
	}

	return hostConfig
}

// VerifyConfiguration validates the container's metadata for recreation.
//
// Returns:
//   - error: Non-nil if metadata is missing or invalid, nil on success.
func (c *Container) VerifyConfiguration() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for nil image info.
	if c.imageInfo == nil {
		c.logger().Debug().
			Str("container", "<unknown>").
			Msg("No image info available")

		return errNoImageInfo
	}

	// Check for nil container info.
	if c.containerInfo == nil {
		c.logger().Debug().
			Str("container", "<unknown>").
			Msg("No container info available")

		return errNoContainerInfo
	}

	clogVal := c.logger().With().
		Str("container", c.Name()).
		Logger()
	clog := &clogVal
	// Validate config and host config presence.
	if c.containerInfo.Config == nil || c.containerInfo.HostConfig == nil {
		clog.Debug().Msg("Invalid container configuration")

		return errInvalidConfig
	}

	// Ensure ExposedPorts is initialized if PortBindings exist.
	initExposed := len(c.containerInfo.HostConfig.PortBindings) > 0 &&
		c.containerInfo.Config.ExposedPorts == nil

	// Find port bindings with empty port values.
	// Docker rejects ports with empty port numbers (e.g., "/tcp") with
	// "invalid port range: value is empty" during ContainerCreate.
	var emptyPorts []dockerNetwork.Port

	for port := range c.containerInfo.HostConfig.PortBindings {
		if port.Port() == "" {
			emptyPorts = append(emptyPorts, port)
		}
	}

	if initExposed || len(emptyPorts) > 0 {
		c.replaceInspectLocked(func(info *dockerContainer.InspectResponse) {
			exposed := maps.Clone(info.Config.ExposedPorts)
			if exposed == nil && initExposed {
				exposed = dockerNetwork.PortSet{}

				clog.Debug().Msg("Initialized ExposedPorts due to PortBindings")
			}

			bindings := maps.Clone(info.HostConfig.PortBindings)

			// Skip and remove completely empty port entries.
			for _, port := range emptyPorts {
				clog.Warn().Msg("Skipping empty port binding and exposed port")

				delete(bindings, port)
				delete(exposed, port)
			}

			info.Config.ExposedPorts = exposed
			info.HostConfig.PortBindings = bindings
		})
	}

	clog.Debug().Msg("Verified container configuration")

	return nil
}

// HasExposedPorts checks if the container has any host-bound port mappings.
//
// This is used to determine if a Watchtower container should skip self-updates,
// as host-bound ports cause port conflicts during the self-update process where
// the old container holds the port while the new container tries to bind it.
//
// Only actual host-to-container port bindings (HostConfig.PortBindings) are
// considered, not merely declared exposed ports (Config.ExposedPorts), since
// declared-but-unbound ports do not cause conflicts.
//
// Returns:
//   - bool: True if the container has host-bound port mappings, false otherwise.
func (c *Container) HasExposedPorts() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.containerInfo == nil {
		return false
	}

	// Check if there are any port bindings (host-to-container port mappings).
	// Guard against nil HostConfig before inspecting PortBindings.
	if c.containerInfo.HostConfig != nil && len(c.containerInfo.HostConfig.PortBindings) > 0 {
		return true
	}

	return false
}

// filterSelfReferences removes any links that reference the container itself.
//
// This prevents circular dependencies where a container would depend on itself,
// which could cause infinite loops during dependency resolution and update processing.
// Self-references are filtered out to ensure the dependency graph remains acyclic
// and containers are processed in the correct order.
//
// Parameters:
//   - links: List of container names this container depends on.
//   - containerName: Name of the current container being checked.
//
// Returns:
//   - []string: Filtered list with self-references removed.
func filterSelfReferences(links []string, containerName string) []string {
	filtered := make([]string, 0, len(links))
	for _, link := range links {
		// Skip links that reference the container itself to prevent self-dependencies
		if link != containerName {
			filtered = append(filtered, link)
		}
	}

	return filtered
}

// Links returns a list of container names this container depends on.
//
// Label dependencies come from one label, in priority order:
//  1. com.centurylinklabs.watchtower.depends-on label (Watchtower's native dependency format)
//  2. com.docker.compose.depends_on label (Docker Compose dependencies via v5 API)
//
// The host config dependencies are always appended after them, because the
// container cannot run without their targets:
//  1. HostConfig.Links (legacy Docker links)
//  2. NetworkMode.ConnectedContainer() (container network mode dependencies)
//  3. HostConfig.VolumesFrom identities
//
// Duplicates are skipped, so a target named by a label and by the host config
// appears once, in the label's position.
//
// Self-references are filtered out from all link sources to prevent circular
// dependencies where a container would depend on itself. This ensures the
// dependency resolution algorithm can process containers in a valid topological order.
//
// Parameters:
//   - useComposeDependsOn: Whether to include Docker Compose depends_on label in dependency resolution.
//
// Returns:
//   - []string: List of linked container names with self-references removed.
func (c *Container) Links(useComposeDependsOn bool) []string {
	var links []string

	// Check Watchtower's depends-on label first.
	if labelLinks := GetLinksFromWatchtowerLabel(c); labelLinks != nil {
		links = labelLinks
	} else if useComposeDependsOn {
		// Use Compose depends_on only when the Watchtower label is absent.
		if composeLinks := getLinksFromComposeLabel(c); composeLinks != nil {
			links = composeLinks
		}
	}

	// Always append the host config dependencies. A label must not hide the
	// container whose network or volumes this container uses, or the target
	// can be recreated while this container still uses it.
	links = appendUniqueLinks(links, getLinksFromHostConfig(c))
	links = appendUniqueLinks(links, volumesFromLinks(c))

	return filterSelfReferences(links, c.Name())
}

// replaceInspectLocked replaces the container's inspect data with a copy that
// change modifies. Inspect data returned earlier by ContainerInfo is never
// changed, so callers can read it without holding the lock. change receives
// copies of Config and HostConfig, and must copy any map or slice in them
// before changing it. The caller must hold c.mu for writing, and the inspect
// data must not be nil.
//
// Parameters:
//   - change: Modifies the copy of the inspect data.
func (c *Container) replaceInspectLocked(change func(info *dockerContainer.InspectResponse)) {
	info := *c.containerInfo

	if info.Config != nil {
		config := *info.Config
		info.Config = &config
	}

	if info.HostConfig != nil {
		hostConfig := *info.HostConfig
		info.HostConfig = &hostConfig
	}

	change(&info)
	c.containerInfo = &info
}

// imageNameLocked returns the cached image name. The caller must hold c.mu.
func (c *Container) imageNameLocked() string {
	// Re-resolve when inspect data was cleared after construction.
	if c.containerInfo == nil {
		return c.resolveImageName()
	}

	return c.imageName
}

// resolveImageName computes the image name from the Zodiac label or Config.Image.
//
// An untagged name receives a ":latest" suffix. Missing config yields "unknown:latest".
//
// Returns:
//   - string: Image name with a tag (e.g., "alpine:latest").
func (c *Container) resolveImageName() string {
	// Prefer the Zodiac label for the image name.
	// The caller holds c.mu or is constructing the container, so the label
	// is read from the field rather than through ContainerInfo.
	imageName, ok := labelValue(c.containerInfo, zodiacLabel)
	if !ok {
		if c.containerInfo == nil || c.containerInfo.Config == nil {
			c.logger().Warn().
				Str("container", c.Name()).
				Msg("No container config available, using default image name")

			return "unknown:latest"
		}

		imageName = c.containerInfo.Config.Image
	}

	return ensureImageTag(imageName)
}

// ensureImageTag appends ":latest" when the reference has no tag.
//
// A colon in the registry host (for example "registry.example:5000/app") is
// not treated as a tag.
//
// Parameters:
//   - name: Image reference to normalize.
//
// Returns:
//   - string: Image name with a tag.
func ensureImageTag(name string) string {
	if name == "" {
		return "unknown:latest"
	}

	slash := strings.LastIndex(name, "/")
	if strings.Contains(name[slash+1:], ":") {
		return name
	}

	return name + ":latest"
}

// logger returns the container's process logger, or a discarded nop if unset.
func (c *Container) logger() *zerolog.Logger {
	if c != nil && c.log != nil {
		return c.log
	}

	return nopLog()
}

// ResolveContainerIdentifier returns a standardized container identifier used
// for dependency resolution, update coordination, logging, and cycle detection.
//
// Container identifier formats:
//  1. project-service-containerNumber (if project name, service name, and container number are all available)
//  2. project-service (if project name and service name are available)
//  3. service (if only service name is available)
//  4. container name (if name is available)
//  5. container ID (fallback)
//
// Parameters:
//   - c: Container to get identifier for
//
// Returns:
//   - string: Container identifier formatted according to the prioritization
//     order, always returns a non-empty string for valid containers
func ResolveContainerIdentifier(c types.Container) string {
	info := c.ContainerInfo()
	if info == nil {
		return nameOrID(c)
	}

	cfg := info.Config
	if cfg == nil {
		return nameOrID(c)
	}

	labels := cfg.Labels
	if len(labels) == 0 {
		return nameOrID(c)
	}

	projectName := compose.GetProjectName(labels)
	serviceName := compose.GetServiceName(labels)
	containerNumber := compose.GetContainerNumber(labels)

	// Handle replica containers
	if projectName != "" && serviceName != "" &&
		strings.HasPrefix(c.Name(), projectName+"-"+serviceName+"-") {
		return c.Name()
	}

	// Prioritize identifier formats based on available information
	if projectName != "" && serviceName != "" && containerNumber != "" {
		return projectName + "-" + serviceName + "-" + containerNumber
	}

	if projectName != "" && serviceName != "" {
		return projectName + "-" + serviceName
	}

	if serviceName != "" {
		return serviceName
	}

	return nameOrID(c)
}

// nameOrID returns the container name if non-empty; otherwise, returns the container ID.
func nameOrID(c types.Container) string {
	// Return the container name if available.
	name := c.Name()
	if name != "" {
		return name
	}

	// Otherwise, return the container ID.
	return string(c.ID())
}

// GetLinksFromWatchtowerLabel extracts dependency links from the
// watchtower depends-on label.
//
// It parses the com.centurylinklabs.watchtower.depends-on label value,
// splitting on commas and normalizing each container name, returning all
// normalized links, including potential self-references.
//
// Note: Watchtower depends-on labels reference container names directly,
// unlike Compose depends-on, which references services within the same project.
// Therefore, we do not prefix with project name for Watchtower labels.
//
// Parameters:
//   - c: Container instance
//
// Returns:
//   - []string: List of all normalized links, including potential self-references, or nil if label not present
func GetLinksFromWatchtowerLabel(c *Container) []string {
	// Get the depends-on label value or empty string if not present
	dependsOnLabelValue := c.getLabelValueOrEmpty(dependsOnLabel)

	// If the label is empty, return nil
	if dependsOnLabelValue == "" {
		return nil
	}

	// Split the comma-separated values
	links := strings.Split(dependsOnLabelValue, ",")

	// Parse the links and normalize them
	normalizedLinks := make([]string, 0, len(links))
	for _, normalizedLink := range links {
		// Skip empty links
		if normalizedLink == "" {
			continue
		}

		// Normalize the link by trimming spaces and removing any leading slashes
		normalizedLink = util.NormalizeContainerName(strings.TrimSpace(normalizedLink))

		// Add the normalized link to the result slice
		normalizedLinks = append(normalizedLinks, normalizedLink)
	}

	return normalizedLinks
}

// getLinksFromComposeLabel extracts dependency links from the Docker Compose depends-on label.
//
// It parses the com.docker.compose.depends_on label value using the compose package,
// and normalizes each service name. If the container has a project label, service names
// are qualified with the project name.
//
// Parameters:
//   - c: Container instance
//
// Returns:
//   - []string: List of linked container names, empty if label not present
func getLinksFromComposeLabel(c *Container) []string {
	info := c.ContainerInfo()

	composeDependsOnLabelValue := c.getLabelValueOrEmpty(compose.ComposeDependsOnLabel)
	if composeDependsOnLabelValue == "" {
		return nil
	}

	services := compose.ParseDependsOnLabel(c.logger(), composeDependsOnLabelValue)

	projectName := compose.GetProjectName(info.Config.Labels)

	normalizedLinks := make([]string, 0, len(services))
	for _, service := range services {
		normalizedService := util.NormalizeContainerName(service)
		// If the project name isn't empty and the service name doesn't have the project name prefix,
		// then add the project name prefix to the service name.
		if projectName != "" && !strings.HasPrefix(normalizedService, projectName+"-") {
			normalizedService = projectName + "-" + normalizedService
		}

		normalizedLinks = append(normalizedLinks, normalizedService)
	}

	if len(normalizedLinks) == 0 {
		return nil
	}

	return normalizedLinks
}

// getLinksFromHostConfig extracts dependency links from Docker HostConfig.
//
// It parses HostConfig.Links and network mode to determine container
// dependencies. If the container has a project label, legacy link names are
// qualified with the project name if they are not already qualified. Network
// mode is a Docker identity, not a Compose service name, and is not
// project-prefixed. VolumesFrom is merged in Links, not here.
//
// A malformed legacy link is skipped with a warning. Docker stores links as
// /name:/container/alias, so this only happens with inspect data Docker did
// not write.
//
// Parameters:
//   - c: Container instance
//
// Returns:
//   - []string: List of linked container names
func getLinksFromHostConfig(c *Container) []string {
	info := c.ContainerInfo()

	if info == nil || info.HostConfig == nil {
		return nil
	}

	var labels map[string]string
	if info.Config != nil {
		labels = info.Config.Labels
	}

	projectName := compose.GetProjectName(labels)

	hostConfig := info.HostConfig

	// Pre-allocate for links plus a potential network mode dependency.
	capacity := len(hostConfig.Links)

	networkMode := hostConfig.NetworkMode
	if networkMode.IsContainer() {
		capacity++
	}

	normalizedLinks := make([]string, 0, capacity)

	for _, link := range hostConfig.Links {
		if !strings.Contains(link, ":") {
			c.logger().Warn().
				Str("container", c.Name()).
				Str("link", link).
				Msg("Invalid link format in host config, expected 'name:alias'")

			continue
		}

		parts := strings.SplitN(link, ":", linkPartsCount)
		if len(parts) < 1 || parts[0] == "" {
			c.logger().Warn().
				Str("container", c.Name()).
				Str("link", link).
				Msg("Invalid link format in host config, missing container name")

			continue
		}

		normalizedName := util.NormalizeContainerName(parts[0])
		if projectName != "" && !strings.HasPrefix(normalizedName, projectName+"-") {
			normalizedName = projectName + "-" + normalizedName
		}

		normalizedLinks = append(normalizedLinks, normalizedName)
	}

	// Add network dependency.
	//
	// Unlike a Compose service reference, network mode is a Docker identity.
	// The daemon stores container:<id> after create. List inspect rewrites
	// that to container:<name>. Do not prefix either form with this
	// container's Compose project.
	if networkMode.IsContainer() {
		normalizedName := util.NormalizeContainerName(networkMode.ConnectedContainer())
		normalizedLinks = append(normalizedLinks, normalizedName)
	}

	return normalizedLinks
}

// volumesFromLinks returns normalized HostConfig.VolumesFrom identities.
//
// VolumesFrom is a Docker identity (name or ID, optional :ro/:rw). Do not
// prefix with this container's Compose project.
//
// Parameters:
//   - c: Container instance.
//
// Returns:
//   - []string: Normalized volumes-from container names or IDs.
func volumesFromLinks(c *Container) []string {
	info := c.ContainerInfo()

	if info == nil || info.HostConfig == nil {
		return nil
	}

	specs := info.HostConfig.VolumesFrom
	if len(specs) == 0 {
		return nil
	}

	links := make([]string, 0, len(specs))
	for _, spec := range specs {
		name, _ := parseVolumesFromSpec(spec)
		if name == "" {
			// Skip empty or mode-only specs.
			continue
		}

		links = append(links, util.NormalizeContainerName(name))
	}

	return links
}

// appendUniqueLinks appends extra names that are not already in links.
//
// Existing order is preserved. New names are appended in extra order.
//
// Parameters:
//   - links: Existing dependency names.
//   - extra: Additional names to merge.
//
// Returns:
//   - []string: Merged list without duplicates.
func appendUniqueLinks(links, extra []string) []string {
	if len(extra) == 0 {
		return links
	}

	seen := make(map[string]struct{}, len(links)+len(extra))
	for _, link := range links {
		seen[link] = struct{}{}
	}

	for _, link := range extra {
		if _, exists := seen[link]; exists {
			// Keep the earlier occurrence so label order wins.
			continue
		}

		seen[link] = struct{}{}
		links = append(links, link)
	}

	return links
}

// parseVolumesFromSpec splits a VolumesFrom entry into container identity and mode.
//
// Docker stores entries as name-or-id, optionally followed by :ro, :rw, or a
// SELinux :z/:Z flag. A trailing colon suffix is treated as a mode only when
// every comma-separated part is one of those flags.
//
// Parameters:
//   - spec: VolumesFrom string from HostConfig.
//
// Returns:
//   - string: Container name or ID.
//   - string: Mode suffix without a leading colon, or empty.
func parseVolumesFromSpec(spec string) (string, string) {
	if spec == "" {
		return "", ""
	}

	before, after, ok := strings.CutLast(spec, ":")
	if !ok {
		return spec, ""
	}

	suffix := after
	if !isVolumesFromMode(suffix) {
		// A colon that is not a volume mode stays part of the identity.
		return spec, ""
	}

	return before, suffix
}

// isVolumesFromMode reports whether suffix is a Docker volumes-from access mode.
//
// Docker accepts ro, rw, and SELinux z or Z, including comma-separated
// combinations such as ro,z.
//
// Parameters:
//   - suffix: Trailing VolumesFrom mode after the last colon.
//
// Returns:
//   - bool: True if every comma-separated part is ro, rw, z, or Z.
func isVolumesFromMode(suffix string) bool {
	if suffix == "" {
		return false
	}

	for part := range strings.SplitSeq(suffix, ",") {
		switch part {
		case "ro", "rw", "z", "Z":
			// Recognized volume access or SELinux relabel flag.
		default:
			return false
		}
	}

	return true
}
