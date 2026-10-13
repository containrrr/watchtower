package container

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"
	dockerAPIVersion "github.com/moby/moby/client/pkg/versions"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// defaultStopSignal is the default signal for stopping containers ("SIGTERM").
const defaultStopSignal = "SIGTERM"

// ListSourceContainers retrieves a list of containers from the container runtime host.
//
// It filters containers based on options and a provided filter function.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - opts: Client options for filtering.
//   - filter: Function to filter containers.
//   - isPodmanOptional: Optional variadic flag indicating Podman runtime (defaults to false).
//
// Returns:
//   - []types.Container: Filtered list of containers.
//   - error: Non-nil if listing fails, nil on success.
func ListSourceContainers(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	opts ClientOptions,
	filter types.Filter,
	isPodmanOptional ...bool,
) ([]types.Container, error) {
	clogVal := log.With().
		Bool("include_stopped", opts.IncludeStopped).
		Bool("include_restarting", opts.IncludeRestarting).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Retrieving container list")

	// Determine if the container runtime is Podman.
	// Default to false (Docker) if not specified.
	isPodman := false
	if len(isPodmanOptional) > 0 {
		isPodman = isPodmanOptional[0]
	}

	// Build filter arguments for container states.
	filterArgs := buildListFilterArgs(opts, isPodman)
	clog.Debug().
		Bool("custom_filter_provided", filter != nil).
		Interface("filter_args", filterArgs).
		Msg("Built filter arguments")

	// Fetch containers with status filters always applied based on ClientOptions.
	listOptions := dockerClient.ContainerListOptions{Filters: filterArgs}

	clog.Debug().Msg("API status filters applied based on ClientOptions")

	containers, err := api.ContainerList(ctx, listOptions)
	if err != nil {
		// Log detailed error information for debugging
		clog.Debug().
			Err(err).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("endpoint", "/containers/json").
			Str("api_version", strings.Trim(api.ClientVersion(), "\"")).
			Str("docker_host", os.Getenv("DOCKER_HOST")).
			Str("list_options", fmt.Sprintf("%+v", listOptions)).
			Msg("ContainerList API call failed")

		// Check for 404 responses and return an empty container list instead of failing.
		if cerrdefs.IsNotFound(err) {
			clog.Warn().
				Err(err).
				Str("endpoint", "/containers/json").
				Str("api_version", strings.Trim(api.ClientVersion(), "\"")).
				Str("docker_host", os.Getenv("DOCKER_HOST")).
				Msg("Docker API returned 404 for container list. Treating as empty list")

			return []types.Container{}, nil
		}

		clog.Debug().
			Err(err).
			Msg("Failed to list containers")

		return nil, fmt.Errorf("%w: %w", errListContainersFailed, err)
	}

	// Convert and filter containers.
	hostContainers := make([]types.Container, 0, len(containers.Items))
	// Reuse ImageInspect results when multiple containers share an image ID.
	imageCache := make(map[string]*dockerImage.InspectResponse, len(containers.Items))

	for _, runningContainer := range containers.Items {
		container, err := getSourceContainer(log, ctx, api, types.ContainerID(runningContainer.ID), imageCache)
		if err != nil {
			// Log detailed error information for debugging container inspect failures
			log.Debug().
				Str("container_id", runningContainer.ID).
				Err(err).
				Str("error_type", fmt.Sprintf("%T", err)).
				Str("api_version", strings.Trim(api.ClientVersion(), "\"")).
				Str("docker_host", os.Getenv("DOCKER_HOST")).
				Msg("Failed to inspect individual container during list")

			// Handle race condition where containers disappear between API calls
			if cerrdefs.IsNotFound(err) {
				log.Debug().
					Str("container_id", runningContainer.ID).
					Msg("Container no longer exists")

				continue
			}

			return nil, err // Logged in GetSourceContainer
		}

		if filter == nil || filter(container) {
			if !container.HasImageInfo() {
				log.Warn().
					Str("container", container.Name()).
					Str("container_id", runningContainer.ID).
					Str("image", container.ImageName()).
					Msg("Failed to retrieve image info")
			}

			hostContainers = append(hostContainers, container)
		}
	}

	clog.Debug().
		Int("count", len(hostContainers)).
		Msg("Filtered container list")

	return hostContainers, nil
}

// GetSourceContainer retrieves detailed information about a container by its ID.
//
// It resolves network mode if it references another container.
//
// Parameters:
//   - log: Logger for debug output.
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - containerID: ID of the container to inspect.
//
// Returns:
//   - types.Container: Container object if successful.
//   - error: Non-nil if inspection fails, nil on success.
func GetSourceContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	containerID types.ContainerID,
) (types.Container, error) {
	// No per-list cache for standalone inspects.
	return getSourceContainer(log, ctx, api, containerID, nil)
}

// getSourceContainer inspects a container and optionally reuses ImageInspect results.
//
// Parameters:
//   - log: Logger for debug output.
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - containerID: ID of the container to inspect.
//   - imageCache: Optional per-list cache of ImageInspect results keyed by image ID.
//
// Returns:
//   - types.Container: Container object if successful. It has no image info when only the image inspect fails.
//   - error: Non-nil if the container inspect fails or the image inspect is canceled or times out.
func getSourceContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	containerID types.ContainerID,
	imageCache map[string]*dockerImage.InspectResponse,
) (types.Container, error) {
	clogVal := log.With().
		Str("container_id", string(containerID)).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Inspecting container")

	// Inspect the container to get its details.
	containerResult, err := api.ContainerInspect(
		ctx,
		string(containerID),
		dockerClient.ContainerInspectOptions{},
	)
	if err != nil {
		// Log detailed error information for debugging
		clog.Debug().
			Err(err).
			Str("error_type", fmt.Sprintf("%T", err)).
			Str("api_version", strings.Trim(api.ClientVersion(), "\"")).
			Str("docker_host", os.Getenv("DOCKER_HOST")).
			Msg("ContainerInspect API call failed")

		clog.Debug().
			Err(err).
			Msg("Failed to inspect container")

		return nil, fmt.Errorf("%w: %w", errInspectContainerFailed, err)
	}

	containerInfo := &containerResult.Container

	// Resolve network mode if it references another container.
	if containerInfo.HostConfig != nil {
		netType, netContainerID, found := strings.Cut(string(containerInfo.HostConfig.NetworkMode), ":")
		if found && netType == "container" {
			parentResult, err := api.ContainerInspect(ctx,
				netContainerID,
				dockerClient.ContainerInspectOptions{},
			)
			if err != nil {
				clog.Warn().
					Err(err).
					Str("container", util.NormalizeContainerName(containerInfo.Name)).
					Str("network_container", netContainerID).
					Msg("Unable to resolve network container")
			} else {
				containerInfo.HostConfig.NetworkMode = dockerContainer.NetworkMode(
					"container:" + parentResult.Container.Name,
				)
				clog.Debug().
					Str("container", util.NormalizeContainerName(containerInfo.Name)).
					Str("network_container", util.NormalizeContainerName(parentResult.Container.Name)).
					Msg("Resolved network container name")
			}
		}

		// Rewrite volumes-from IDs to names while the source still exists.
		resolveVolumesFrom(ctx, api, clog, containerInfo)
	}

	imageInfo, err := resolveImageInspect(ctx, api, containerInfo.Image, imageCache)
	if err != nil {
		// A canceled or timed-out inspect ends the listing. Other failures, such
		// as a removed image, leave the container without image info.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		clog.Debug().
			Err(err).
			Str("container", util.NormalizeContainerName(containerInfo.Name)).
			Str("image", containerInfo.Image).
			Msg("Failed to retrieve image info")

		return NewContainer(log, containerInfo, nil), nil
	}

	clog.Debug().
		Str("image", containerInfo.Image).
		Msg("Retrieved container and image info")

	return NewContainer(log, containerInfo, imageInfo), nil
}

// resolveVolumesFrom rewrites HostConfig.VolumesFrom IDs to live container names.
//
// Inspect stores volumes-from as the source container ID. Recreate must use a
// name so Docker can find the replacement after the source is recreated.
// Unresolved sources stay as stored. Mode suffixes such as :rw are preserved.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - clog: Logger for warn and debug output.
//   - containerInfo: Inspected container whose HostConfig.VolumesFrom is rewritten.
func resolveVolumesFrom(
	ctx context.Context,
	api dockerClient.APIClient,
	clog *zerolog.Logger,
	containerInfo *dockerContainer.InspectResponse,
) {
	if containerInfo.HostConfig == nil || len(containerInfo.HostConfig.VolumesFrom) == 0 {
		return
	}

	rewritten := slices.Clone(containerInfo.HostConfig.VolumesFrom)
	containerName := util.NormalizeContainerName(containerInfo.Name)

	for i, spec := range rewritten {
		name, mode := parseVolumesFromSpec(spec)
		if name == "" {
			continue
		}

		sourceResult, err := api.ContainerInspect(
			ctx,
			name,
			dockerClient.ContainerInspectOptions{},
		)
		if err != nil {
			// Leave the stored ID so create fails with a clear missing-container
			// error instead of inventing a name.
			clog.Warn().
				Err(err).
				Str("container", containerName).
				Str("volumes_from", name).
				Msg("Unable to resolve volumes-from container")

			continue
		}

		resolved := util.NormalizeContainerName(sourceResult.Container.Name)
		if mode != "" {
			// Keep :ro/:rw/SELinux flags from the original spec.
			resolved = resolved + ":" + mode
		}

		rewritten[i] = resolved
		clog.Debug().
			Str("container", containerName).
			Str("volumes_from", resolved).
			Msg("Resolved volumes-from container name")
	}

	containerInfo.HostConfig.VolumesFrom = rewritten
}

// resolveImageInspect returns image inspect metadata, using imageCache when set.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - imageID: Image ID or name to inspect.
//   - imageCache: Optional per-list cache of ImageInspect results.
//
// Returns:
//   - *dockerImage.InspectResponse: Isolated inspect metadata, or nil on inspect failure.
//   - error: Non-nil if ImageInspect fails.
func resolveImageInspect(
	ctx context.Context,
	api dockerClient.APIClient,
	imageID string,
	imageCache map[string]*dockerImage.InspectResponse,
) (*dockerImage.InspectResponse, error) {
	if cached, ok := imageCache[imageID]; ok {
		return cloneImageInspect(cached), nil
	}

	imageResult, err := api.ImageInspect(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("inspect image: %w", err)
	}

	imageInfo := &imageResult.InspectResponse
	if imageCache != nil {
		imageCache[imageID] = imageInfo

		return cloneImageInspect(imageInfo), nil
	}

	return imageInfo, nil
}

// cloneImageInspect returns an independent copy of image inspect metadata.
//
// Slice and nested pointer fields are duplicated so the cached original stays
// read-only for other containers.
//
// Parameters:
//   - src: Cached inspect response, or nil.
//
// Returns:
//   - *dockerImage.InspectResponse: Isolated copy, or nil when src is nil.
func cloneImageInspect(src *dockerImage.InspectResponse) *dockerImage.InspectResponse {
	if src == nil {
		return nil
	}

	dst := *src
	dst.RepoTags = slices.Clone(src.RepoTags)
	dst.RepoDigests = slices.Clone(src.RepoDigests)
	dst.RootFS.Layers = slices.Clone(src.RootFS.Layers)
	dst.Manifests = slices.Clone(src.Manifests)

	if src.Config != nil {
		cfg := *src.Config
		cfg.Env = slices.Clone(src.Config.Env)
		cfg.Cmd = slices.Clone(src.Config.Cmd)
		cfg.Entrypoint = slices.Clone(src.Config.Entrypoint)
		cfg.OnBuild = slices.Clone(src.Config.OnBuild)
		cfg.Shell = slices.Clone(src.Config.Shell)
		cfg.Labels = maps.Clone(src.Config.Labels)
		cfg.Volumes = maps.Clone(src.Config.Volumes)
		cfg.ExposedPorts = maps.Clone(src.Config.ExposedPorts)

		if src.Config.Healthcheck != nil {
			health := *src.Config.Healthcheck
			health.Test = slices.Clone(src.Config.Healthcheck.Test)
			cfg.Healthcheck = &health
		}

		dst.Config = &cfg
	}

	if src.GraphDriver != nil {
		driver := *src.GraphDriver
		driver.Data = maps.Clone(src.GraphDriver.Data)
		dst.GraphDriver = &driver
	}

	if src.Descriptor != nil {
		desc := *src.Descriptor
		dst.Descriptor = &desc
	}

	if src.Identity != nil {
		identity := *src.Identity
		dst.Identity = &identity
	}

	return &dst
}

// StopSourceContainer stops the specified container using the Docker API's StopContainer method.
//
// It checks if the container is running, sends the configured stop signal (defaulting to SIGTERM if unset),
// and waits for the specified timeout for graceful shutdown, forcing termination with SIGKILL if necessary.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client for interacting with the Docker daemon.
//   - sourceContainer: Container object to stop, containing metadata like name and ID.
//   - timeout: Duration to wait for graceful shutdown before forcing termination with SIGKILL.
//
// Returns:
//   - error: Non-nil if stopping the container fails, nil on successful completion.
func StopSourceContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	sourceContainer types.Container,
	timeout time.Duration,
) error {
	clogVal := log.With().
		Str("container", sourceContainer.Name()).
		Str("id", sourceContainer.ID().ShortID()).
		Logger()
	clog := &clogVal

	// Check if the container is running to determine if a stop operation is needed.
	if !sourceContainer.IsRunning() {
		// Log that the container is not running, so no stop attempt is required.
		clog.Debug().Msg("Container is not running, no stop operation needed")

		return nil
	}

	// Use container's configured timeout if available and valid, otherwise use passed parameter.
	// A timeout of 0 is valid in Docker (means no grace period - immediate SIGKILL after stop signal).
	effectiveTimeout := timeout

	containerTimeout := sourceContainer.StopTimeout()
	if containerTimeout != nil && *containerTimeout >= 0 {
		effectiveTimeout = time.Duration(*containerTimeout) * time.Second
		clog.Debug().
			Dur("container_timeout", effectiveTimeout).
			Dur("default_timeout", timeout).
			Msg("Using container-specific stop timeout")
	}

	// Retrieve the container's configured stop signal, falling back to SIGTERM if not specified.
	signal := sourceContainer.StopSignal()
	if signal == "" {
		// Use default SIGTERM signal if no custom signal is provided.
		signal = defaultStopSignal
	}

	// Log the stop attempt with the signal and configured timeout for clarity.
	message := "Stopping container"
	if sourceContainer.IsLinkedToRestarting() {
		message = "Stopping linked container"
	}

	clog.Info().
		Str("signal", signal).
		Dur("timeout", effectiveTimeout).
		Msg(message)

	// Record the start time to measure elapsed duration for the stop operation.
	startTime := time.Now()

	// Convert timeout from time.Duration to seconds for Docker API's StopContainer.
	timeoutSeconds := int(effectiveTimeout / time.Second)

	// Call the Docker API's StopContainer with the stop signal and timeout in seconds.
	// The API sends the signal (SIGTERM or custom), waits for the timeout, and sends SIGKILL if needed.
	_, err := api.ContainerStop(
		ctx,
		string(sourceContainer.ID()),
		dockerClient.ContainerStopOptions{
			Signal:  signal,
			Timeout: &timeoutSeconds,
		},
	)
	if err != nil {
		// Check if the container was already removed by another process before
		// the stop call completed, treating it as already stopped.
		if cerrdefs.IsNotFound(err) {
			clog.Debug().
				Dur("elapsed", time.Since(startTime)).
				Msg("Container not found during stop, treating as already stopped")

			return nil
		}

		// Log the failure with elapsed time and error details for debugging.
		clog.Error().
			Err(err).
			Dur("elapsed", time.Since(startTime)).
			Msg("Failed to stop container")
		// Wrap the error with a specific Watchtower error type for consistent error handling.
		return fmt.Errorf("%w: %w", errStopContainerFailed, err)
	}

	// Log successful stop with elapsed time to confirm the operation's duration.
	clog.Debug().
		Dur("elapsed", time.Since(startTime)).
		Msg("Container stopped successfully")

	return nil
}

// StopAndRemoveSourceContainer stops and removes the specified container using the Docker API.
//
// It first stops the container if running, then removes it with optional volume cleanup.
// When HostConfig.AutoRemove is set, Docker only removes the container after a started
// instance exits. Explicit removal is therefore skipped only when the container was
// running before stop. Non-running containers (for example created but never started)
// are always removed explicitly so the name is freed for recreation.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client for interacting with the Docker daemon.
//   - sourceContainer: Container object to stop and remove, containing metadata like name and ID.
//   - timeout: Duration to wait for graceful shutdown before forcing termination with SIGKILL.
//   - removeVolumes: Boolean indicating whether to remove associated volumes during container removal.
//
// Returns:
//   - error: Non-nil if stopping or removing the container fails, nil on successful completion.
func StopAndRemoveSourceContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	sourceContainer types.Container,
	timeout time.Duration,
	removeVolumes bool,
) error {
	clogVal := log.With().
		Str("container", sourceContainer.Name()).
		Str("id", sourceContainer.ID().ShortID()).
		Logger()
	clog := &clogVal

	// Capture running state before stop. AutoRemove only applies after a started container exits.
	wasRunning := sourceContainer.IsRunning()

	// Stop the container first.
	err := StopSourceContainer(log, ctx, api, sourceContainer, timeout)
	if err != nil {
		return err
	}

	// HostConfig.AutoRemove removes a container after it stops. That only applies
	// when the container was running and Docker handles cleanup after stop.
	info := sourceContainer.ContainerInfo()
	autoRemove := info != nil && info.HostConfig != nil && info.HostConfig.AutoRemove

	// Skip explicit removal only when AutoRemove will run after stop. Never-started
	// or already-stopped AutoRemove containers remain until removed explicitly and
	// would otherwise block name reuse on recreate.
	if autoRemove && wasRunning {
		// Log that the container is skipped due to AutoRemove configuration.
		clog.Debug().Msg("Skipping container removal due to AutoRemove configuration")

		return nil
	}

	if autoRemove && !wasRunning {
		clog.Debug().Msg("Removing non-running AutoRemove container explicitly")
	}

	// Proceed with explicit container removal, including volumes if specified.
	// Log the start of the removal process for tracking.
	clog.Debug().Msg("Initiating container removal")

	// Record start time for the removal operation to track its duration.
	startTime := time.Now()

	// Call the Docker API's ContainerRemove to delete the container, forcing termination of any
	// lingering processes (via SIGKILL if needed) and removing volumes if specified.
	_, err = api.ContainerRemove(
		ctx,
		string(sourceContainer.ID()),
		dockerClient.ContainerRemoveOptions{
			Force:         true,          // Ensure any lingering processes are terminated before removal.
			RemoveVolumes: removeVolumes, // Remove associated volumes if the parameter is true.
		},
	)
	if err != nil && !cerrdefs.IsNotFound(err) {
		// Log removal failure with elapsed time and error details, excluding cases where the container
		// was already removed by another process.
		clog.Error().
			Err(err).
			Dur("elapsed", time.Since(startTime)).
			Msg("Failed to remove container")
		// Wrap the error with a specific Watchtower error type for consistent error handling.
		return fmt.Errorf("%w: %w", errRemoveContainerFailed, err)
	}

	if cerrdefs.IsNotFound(err) {
		// Log that the container was already removed, likely by another process or AutoRemove.
		clog.Debug().
			Dur("elapsed", time.Since(startTime)).
			Msg("Container already removed by another process")

		return nil // Container already gone.
	}

	// Log successful removal with elapsed time to confirm the operation's duration.
	clog.Debug().
		Dur("elapsed", time.Since(startTime)).
		Msg("Container removed successfully")

	return nil
}

// buildListFilterArgs builds filter arguments for retrieving container states.
//
// It uses client options to determine which statuses to include.
//
// Parameters:
//   - opts: Client options for filtering.
//   - isPodman: Whether the runtime is Podman.
//
// Returns:
//   - dockerClient.Filters: Arguments for filtering Docker containers
func buildListFilterArgs(opts ClientOptions, isPodman bool) dockerClient.Filters {
	filterArgs := make(dockerClient.Filters)

	filterArgs.Add("status", "running")

	if opts.IncludeStopped {
		filterArgs.Add("status", "created")
		filterArgs.Add("status", "exited")
	}

	// Podman doesn't have the "restarting" status
	if opts.IncludeRestarting && !isPodman {
		filterArgs.Add("status", "restarting")
	}

	return filterArgs
}

// getNetworkConfig extracts and sanitizes the network configuration from a container.
//
// It handles all network modes, including host, and supports both legacy and modern API versions.
//
// Parameters:
//   - sourceContainer: Container to extract config from.
//   - clientVersion: API version of the client.
//
// Returns:
//   - *dockerNetworkType.NetworkingConfig: Sanitized network configuration.
func getNetworkConfig(log *zerolog.Logger,
	sourceContainer types.Container,
	clientVersion string,
) *dockerNetwork.NetworkingConfig {
	clogVal := log.With().
		Str("container", sourceContainer.Name()).
		Str("id", sourceContainer.ID().ShortID()).
		Str("version", clientVersion).
		Logger()
	clog := &clogVal

	// Initialize default network config
	config := newEmptyNetworkConfig()

	clog.Debug().Msg("Initialized empty network configuration")

	// Get network settings and mode from container info
	containerInfo := sourceContainer.ContainerInfo()
	if containerInfo == nil || containerInfo.NetworkSettings == nil {
		clog.Warn().Msg("No network settings available")

		return config
	}

	var networkMode dockerContainer.NetworkMode
	if containerInfo.HostConfig != nil {
		networkMode = containerInfo.HostConfig.NetworkMode
	}

	isHostNetwork := string(networkMode) == "host"
	clog.Debug().
		Str("network_mode", string(networkMode)).
		Bool("is_host", isHostNetwork).
		Msg("Evaluated network mode")

	// Process each network endpoint
	for networkName, sourceEndpoint := range containerInfo.NetworkSettings.Networks {
		if sourceEndpoint == nil {
			clog.Debug().
				Str("network", networkName).
				Msg("Skipping nil endpoint")

			continue
		}

		targetEndpoint, err := processEndpoint(log,
			sourceEndpoint,
			sourceContainer.ID(),
			clientVersion,
			isHostNetwork,
		)
		if err != nil {
			clog.Debug().
				Err(err).
				Str("network", networkName).
				Msg("Failed to process endpoint")

			continue
		}

		config.EndpointsConfig[networkName] = targetEndpoint

		clog.Debug().
			Str("network", networkName).
			Msg("Added endpoint to network config")
	}

	// Validate MAC addresses, passing sourceContainer for state checking
	err := validateMacAddresses(log,
		config,
		sourceContainer.ID(),
		clientVersion,
		isHostNetwork,
		sourceContainer,
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("MAC address validation issue")
	}

	return config
}

// newEmptyNetworkConfig creates an empty network configuration.
//
// Returns:
//   - *dockerNetworkType.NetworkingConfig: Empty configuration with initialized EndpointsConfig.
func newEmptyNetworkConfig() *dockerNetwork.NetworkingConfig {
	return &dockerNetwork.NetworkingConfig{
		EndpointsConfig: make(map[string]*dockerNetwork.EndpointSettings),
	}
}

// processEndpoint sanitizes a single network endpoint for the target container.
//
// It filters aliases, copies IPAM config, and handles MAC addresses based on API
// version and network mode. Returns an error if sourceEndpoint is nil.
//
// Parameters:
//   - sourceEndpoint: Source endpoint to process.
//   - containerID: ID of the source container.
//   - clientVersion: API version of the client.
//   - isHostNetwork: Whether the container uses host network mode.
//
// Returns:
//   - *dockerNetwork.EndpointSettings: Sanitized endpoint settings.
//   - error: Non-nil if sourceEndpoint is nil, nil otherwise.
func processEndpoint(log *zerolog.Logger,
	sourceEndpoint *dockerNetwork.EndpointSettings,
	containerID types.ContainerID,
	clientVersion string,
	isHostNetwork bool,
) (*dockerNetwork.EndpointSettings, error) {
	if sourceEndpoint == nil {
		return nil, errNilSourceEndpoint
	}

	clogVal := log.With().
		Str("container", containerID.ShortID()).
		Str("version", clientVersion).
		Logger()
	clog := &clogVal

	// Copy endpoint to preserve all fields.
	targetEndpoint := sourceEndpoint.Copy()

	clog.Debug().Msg("Copied endpoint settings")

	// Handle aliases: clear for host mode, filter for others.
	if isHostNetwork {
		targetEndpoint.Aliases = nil

		clog.Debug().Msg("Cleared aliases for host network mode")
	} else if len(targetEndpoint.Aliases) > 0 {
		targetEndpoint.Aliases = filterAliases(targetEndpoint.Aliases, containerID.ShortID())
		clog.Debug().
			Strs("source_aliases", sourceEndpoint.Aliases).
			Strs("target_aliases", targetEndpoint.Aliases).
			Msg("Filtered aliases")
	}

	// Copy IPAM config if present and not in host mode.
	if sourceEndpoint.IPAMConfig != nil && !isHostNetwork {
		targetEndpoint.IPAMConfig = &dockerNetwork.EndpointIPAMConfig{
			IPv4Address:  sourceEndpoint.IPAMConfig.IPv4Address,
			IPv6Address:  sourceEndpoint.IPAMConfig.IPv6Address,
			LinkLocalIPs: sourceEndpoint.IPAMConfig.LinkLocalIPs,
		}

		clog.Debug().Msg("Copied IPAM configuration")
	} else {
		targetEndpoint.IPAMConfig = nil

		if isHostNetwork {
			clog.Debug().Msg("Cleared IPAM config for host network mode")
		}
	}

	// Handle aliases, MAC address, IP address, and DNS names based on API version and network mode.
	if dockerAPIVersion.LessThan(clientVersion, "1.44") || isHostNetwork {
		targetEndpoint.MacAddress = dockerNetwork.HardwareAddr{}
		targetEndpoint.IPAddress = netip.Addr{}
		targetEndpoint.DNSNames = nil

		if isHostNetwork {
			clog.Debug().Msg("Cleared MAC address, IP address, and DNS names for host network mode")
		} else {
			clog.Debug().Msg("Cleared MAC address, IP address, and DNS names for legacy API")
		}
	} else if isEngineGeneratedMAC(targetEndpoint.MacAddress, sourceEndpoint.IPAddress) {
		// Clear MACs derived from the endpoint's IPv4 address.
		//
		// Docker 28.0.0+ bridge networks assign random MACs and will not match
		// the engine-generated pattern, but older Docker versions and overlay
		// networks still produce IP-derived MACs that become stale after IP
		// reassignment.
		targetEndpoint.MacAddress = dockerNetwork.HardwareAddr{}

		clog.Debug().Msg("Cleared engine-generated MAC address. Docker will regenerate for new IP")
	}

	return targetEndpoint, nil
}

// isEngineGeneratedMAC reports whether mac matches the bridge driver's engine-generated
// pattern derived from the endpoint's current IPv4 address.
//
// Docker inspect unmarshals MacAddress into a 6-byte HardwareAddr.
// Compare the canonical colon-separated form, not the raw byte length.
// Docker < 28.0.0 bridge networks assigned MACs of the form "02:42:aa:bb:cc:dd",
// where the last four bytes are the IPv4 octets.
// Docker 28.0.0+ switched the bridge driver to fully random MACs, but overlay
// networks still use IP-derived MACs, so this check remains relevant for those
// drivers and for older Docker versions.
// A MAC matching that pattern for the given IP is not a user-configured value
// and should not be preserved across container recreation.
// If ip is not a valid IPv4 address, the MAC is assumed to be user-configured
// and false is returned.
//
// Parameters:
//   - mac: MAC address to check.
//   - ip: Endpoint's current IPv4 address.
//
// Returns:
//   - bool: True if the MAC was engine-generated from ip, false otherwise.
func isEngineGeneratedMAC(mac dockerNetwork.HardwareAddr, ip netip.Addr) bool {
	if !ip.Is4() {
		return false
	}

	ipv4 := ip.As4()
	expected := fmt.Sprintf("02:42:%02x:%02x:%02x:%02x", ipv4[0], ipv4[1], ipv4[2], ipv4[3])

	return mac.String() == expected
}

// onlyGeneratedMacs reports whether every non-empty MAC in the source
// container's original network settings matches the bridge driver's
// engine-generated pattern for that endpoint's IP, and whether every non-nil
// original endpoint is present in the processed configuration.
//
// It is used by validateMacAddresses to allow missing MACs in the processed
// config only when those MACs were intentionally cleared by processEndpoint and
// no endpoints were lost during processing.
//
// Parameters:
//   - sourceContainer: The container whose original network settings are inspected.
//   - processedConfig: The processed network configuration whose endpoint coverage is checked.
//
// Returns:
//   - bool: True if the container had at least one MAC and every one of them was
//     engine-generated, and all non-nil original endpoints appear in the processed
//     config. False otherwise.
func onlyGeneratedMacs(sourceContainer types.Container, processedConfig *dockerNetwork.NetworkingConfig) bool {
	if sourceContainer == nil {
		return false
	}

	containerInfo := sourceContainer.ContainerInfo()

	if containerInfo == nil || containerInfo.NetworkSettings == nil {
		return false
	}

	// Build a set of network names present in the processed config so we can
	// verify that every non-nil original endpoint survived processing.
	processedNetworks := make(map[string]bool, len(processedConfig.EndpointsConfig))

	for name := range processedConfig.EndpointsConfig {
		processedNetworks[name] = true
	}

	hasMac := false

	for networkName, endpoint := range containerInfo.NetworkSettings.Networks {
		// Nil endpoints carry no MAC and do not participate in coverage checks.
		if endpoint == nil {
			continue
		}

		// A non-nil original endpoint that was dropped during processing means
		// the missing-MAC result may not be fully explained by intentional
		// clearing, so do not grant the waiver.
		if !processedNetworks[networkName] {
			return false
		}

		// Endpoints without a MAC cannot be engine-generated, but they also do
		// not disprove the hypothesis that every original MAC was generated.
		if len(endpoint.MacAddress) == 0 {
			continue
		}

		hasMac = true

		// Any user-configured MAC disproves the all-generated hypothesis and
		// preserves normal missing-MAC validation behavior.
		if !isEngineGeneratedMAC(endpoint.MacAddress, endpoint.IPAddress) {
			return false
		}
	}

	// Require at least one original MAC so that containers with no MACs at all
	// do not incorrectly bypass validation.
	return hasMac
}

// validateMacAddresses verifies the presence of MAC addresses in a container's network configuration
// and logs appropriate messages based on the container's state, network mode, and Docker API version.
// It ensures that MAC addresses are correctly handled for modern API versions (>= 1.44) and logs
// warnings for potential issues in running containers while using debug logs for non-critical cases,
// such as non-running containers (e.g., created or exited states), to reduce user-facing noise.
//
// Parameters:
//   - config: The network configuration to validate, containing endpoint settings for each network.
//   - containerID: The unique identifier of the container, used for logging context.
//   - clientVersion: The Docker API version in use (e.g., "1.49"), determining MAC address handling rules.
//   - isHostNetwork: Indicates whether the container uses host network mode, where MAC addresses are not expected.
//   - sourceContainer: The container object, providing access to state (running, created, exited) and metadata.
//
// Returns:
//   - error: Returns a non-nil error (e.g., errNoMacInNonHost, errUnexpectedMacInHost) if validation
//     detects a critical issue requiring attention, such as unexpected MAC addresses in legacy APIs
//     or host mode, or missing MAC addresses in running containers with modern APIs. Returns nil
//     for non-critical cases, such as non-running containers or expected absence of MAC addresses.
func validateMacAddresses(log *zerolog.Logger,
	config *dockerNetwork.NetworkingConfig,
	containerID types.ContainerID,
	clientVersion string,
	isHostNetwork bool,
	sourceContainer types.Container,
) error {
	// Initialize logger with container and API version context for consistent log messages.
	clogVal := log.With().
		Str("container", containerID.ShortID()).
		Str("version", clientVersion).
		Logger()
	clog := &clogVal

	// Handle nil network configuration by checking if the container is running and not in host network mode.
	if config == nil {
		clog.Debug().Msg("Nil network configuration provided")
		// Check if the container is running and not in host network mode.
		isRunning := sourceContainer.IsRunning()
		// If the container is running and not in host network mode, and the API version is >= 1.44, return an error.
		if !dockerAPIVersion.LessThan(clientVersion, "1.44") && !isHostNetwork && isRunning {
			// Log a warning for running containers with missing network configuration in modern APIs.
			return errNoMacInNonHost
		}

		// For non-running containers or host network mode, return nil as no validation is needed.
		return nil
	}

	// Scan network endpoints to check for MAC address presence.
	// A MAC address is expected for running containers in non-host networks with modern API versions.
	foundMac := false

	for networkName, endpoint := range config.EndpointsConfig {
		if endpoint == nil {
			clog.Debug().
				Str("network", networkName).
				Msg("Skipping nil endpoint in MAC validation")

			continue
		}

		if len(endpoint.MacAddress) > 0 {
			foundMac = true
			// Log the found MAC address at debug level for diagnostic purposes.
			clog.Debug().
				Str("network", networkName).
				Str("mac_address", endpoint.MacAddress.String()).
				Msg("MAC address found in network configuration")
		}
	}

	// Retrieve container state to determine if it's running, which affects MAC address expectations.
	// Non-running containers (e.g., created, exited) typically lack MAC addresses due to inactive network interfaces.
	containerInfo := sourceContainer.ContainerInfo()
	isRunning := sourceContainer.IsRunning()

	// Extract the container's state (e.g., "running", "created", "exited") for logging context.
	// Use "unknown" as a fallback if container metadata is incomplete to ensure safe logging.
	containerState := "unknown"
	if containerInfo != nil && containerInfo.State != nil {
		containerState = string(containerInfo.State.Status)
	}

	// Handle legacy Docker API versions (< 1.44), where MAC address preservation is not fully supported.
	// In legacy APIs, MAC addresses should not appear in non-host networks, as they are managed differently.
	if dockerAPIVersion.LessThan(clientVersion, "1.44") {
		if foundMac && !isHostNetwork {
			// Unexpected MAC address in a legacy API is a potential misconfiguration
			// Log a warning and return an error.
			clog.Warn().Msg("Unexpected MAC address in legacy config")

			return fmt.Errorf("%w: API version %s", errUnexpectedMacInLegacy, clientVersion)
		}
		// No MAC address in legacy config is expected.
		// Log at debug level and return no error.
		clog.Debug().Msg("No MAC address in legacy API configuration (expected)")

		return nil
	}

	// Handle host network mode, where the container uses the host's network stack and should not have its own MAC addresses.
	if isHostNetwork {
		if foundMac {
			// MAC addresses in host mode are unexpected and indicate a misconfiguration.
			// Log a warning and return an error.
			clog.Warn().Msg("Unexpected MAC address in host network config")

			return errUnexpectedMacInHost
		}
		// No MAC address in host mode is correct.
		// Log at debug level and return no error.
		clog.Debug().Msg("No MAC address in host network mode (expected)")

		return nil
	}

	// Handle non-host network mode (e.g., bridge, overlay) for modern API versions (>= 1.44).
	// MAC addresses are expected for running containers but not for non-running ones.
	if !foundMac {
		if !isRunning {
			// Non-running containers (e.g., created, exited) typically lack MAC addresses
			// because their network interfaces are inactive.
			clog.Debug().
				Str("state", containerState).
				Msg("No MAC address for non-running container (expected)")

			return nil
		}

		// If all original MACs were engine-generated and every non-nil original
		// endpoint is present in the processed config, the absence is expected
		// because processEndpoint intentionally clears them to avoid stale MACs
		// after IP reassignment.
		if onlyGeneratedMacs(sourceContainer, config) {
			clog.Debug().Msg("No MAC address found. All original MACs were engine-generated and intentionally cleared")

			return nil
		}

		// Running containers should have MAC addresses, but absence may indicate
		// either a lack of support or a configuration issue.
		clog.Debug().
			Str("state", containerState).
			Msg("No MAC address found in non-host network config")

		return errNoMacInNonHost
	}

	// MAC addresses are expected to be found in a running container with a modern API.
	// Log at debug level to confirm successful validation.
	clog.Debug().Msg("MAC address validation passed")

	return nil
}

// filterAliases removes the container's short ID from the list of aliases.
//
// Parameters:
//   - aliases: List of aliases to filter.
//   - shortID: Short ID to remove.
//
// Returns:
//   - []string: Filtered list of aliases.
func filterAliases(aliases []string, shortID string) []string {
	result := make([]string, 0, len(aliases))

	for _, alias := range aliases {
		if alias != shortID {
			result = append(result, alias)
		}
	}

	return result
}

// debugLogMacAddress logs MAC address info for a container's network config.
//
// Parameters:
//   - networkConfig: Network configuration to check.
//   - containerID: ID of the container.
//   - clientVersion: API version of the client.
//   - minSupportedVersion: Minimum API version for MAC preservation.
//   - isHostNetwork: Whether the container uses host network mode.
func debugLogMacAddress(log *zerolog.Logger,
	networkConfig *dockerNetwork.NetworkingConfig,
	containerID types.ContainerID,
	clientVersion string,
	minSupportedVersion string,
	isHostNetwork bool,
) {
	clogVal := log.With().
		Str("container", containerID.ShortID()).
		Str("version", clientVersion).
		Str("min_version", minSupportedVersion).
		Logger()
	clog := &clogVal

	// Check for MAC addresses in the config.
	foundMac := false

	// Iterate through network endpoints to find MAC addresses.
	if networkConfig != nil {
		// Iterate through network endpoints to find MAC addresses.
		for networkName, endpoint := range networkConfig.EndpointsConfig {
			// Check if the endpoint has a MAC address.
			if len(endpoint.MacAddress) > 0 {
				clog.Debug().
					Str("network", networkName).
					Str("mac_address", endpoint.MacAddress.String()).
					Msg("Found MAC address in config")

				// Set flag to indicate MAC address was found.
				foundMac = true
			}
		}
	}

	// Log based on API version, MAC presence, and network mode.
	switch {
	// API < v1.44, MAC present
	case dockerAPIVersion.LessThan(clientVersion, minSupportedVersion):
		if foundMac {
			clog.Debug().Msg("Unexpected MAC address in legacy config")

			return
		}

		clog.Debug().Msg("No MAC address in legacy config, Docker will handle")
	// API < v1.44, MAC present
	case dockerAPIVersion.LessThan(clientVersion, "1.44") && !isHostNetwork:
		if foundMac {
			clog.Debug().Msg("Unexpected MAC address in legacy config")

			return
		}

		clog.Debug().Msg("No MAC address in legacy config, as expected")
	// API < v1.44, MAC present
	case foundMac:
		clog.Debug().Msg("Verified MAC address configuration")
	// API >= v1.44, no MAC, non-host network
	case !isHostNetwork:
		clog.Debug().Msg("No MAC address found in config")
	// API >= v1.44, no MAC, host network
	default:
		clog.Debug().Msg("No MAC address in host network mode, as expected")
	}
}

// IsWatchtowerParent checks if the current container ID exists in the comma-separated container-chain label values.
//
// It handles edge cases like empty chain or invalid IDs by returning false appropriately.
// The chain values are trimmed of whitespace before comparison.
//
// Parameters:
//   - currentID: The container ID to check for in the chain.
//   - chain: Comma-separated string of container IDs from the container-chain label.
//
// Returns:
//   - bool: True if currentID is found in the chain, false otherwise.
func IsWatchtowerParent(currentID types.ContainerID, chain string) bool {
	if currentID == "" || chain == "" {
		return false
	}

	ids := strings.SplitSeq(chain, ",")
	for id := range ids {
		if strings.TrimSpace(id) == string(currentID) {
			return true
		}
	}

	return false
}
