package container

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/moby/moby/client/pkg/versions"
	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// cleanupTimeout is the timeout for cleanup operations (e.g., container remove).
	// Using a dedicated timeout ensures cleanup completes even if the parent context is canceled.
	cleanupTimeout = 10 * time.Second

	// manifestsMinAPIVersion is the first Docker API version whose image
	// inspect reports the image's manifests.
	manifestsMinAPIVersion = "1.48"
)

// startTargetContainer starts a container created from sourceContainer.
//
// It leaves the container stopped when sourceContainer is not running and
// reviveStopped is false. When the start fails, it removes the container if it
// is still stopped.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - clog: Logger with container context fields.
//   - sourceContainer: Source container the new container replaces.
//   - containerID: ID of the created container to start.
//   - reviveStopped: If true, starts the new container even if the source is stopped.
//
// Returns:
//   - error: Non-nil if the start fails, nil on success or when the start is skipped.
func startTargetContainer(
	ctx context.Context,
	api dockerClient.APIClient,
	clog *zerolog.Logger,
	sourceContainer types.Container,
	containerID types.ContainerID,
	reviveStopped bool,
) error {
	if !sourceContainer.IsRunning() && !reviveStopped {
		clog.Debug().
			Str("new_id", string(containerID)).
			Msg("Created container, not starting due to stopped state")

		return nil
	}

	_, err := api.ContainerStart(
		ctx,
		string(containerID),
		dockerClient.ContainerStartOptions{},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("new_id", string(containerID)).
			Msg("Failed to start new container")

		// Start failures from cancellation or transport errors can leave the
		// container state ambiguous. Inspect first and force-remove only when
		// the container is still stopped so a late-started instance is kept.
		cleanupFailedStartContainer(ctx, api, clog, containerID)

		return fmt.Errorf("%w: %w", errStartContainerFailed, err)
	}

	message := "Started new container"
	if sourceContainer.IsLinkedToRestarting() {
		message = "Started linked container"
	}

	clog.Debug().
		Str("new_id", containerID.ShortID()).
		Msg(message)

	return nil
}

// cleanupFailedStartContainer inspects a container that failed to start and
// force-removes it only when it is still stopped. Cancellation or transport
// failures can leave state ambiguous. A late-started container is left alone.
//
// Parameters:
//   - parentCtx: Parent context. Deadlines are detached so cleanup can finish.
//   - api: Interface for container operations.
//   - clog: Logger with container context fields.
//   - containerID: ID of the container created before the start failure.
func cleanupFailedStartContainer(
	parentCtx context.Context,
	api dockerClient.APIClient,
	clog *zerolog.Logger,
	containerID types.ContainerID,
) {
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(parentCtx),
		cleanupTimeout,
	)
	defer cancel()

	inspectResult, inspectErr := api.ContainerInspect(
		cleanupCtx,
		string(containerID),
		dockerClient.ContainerInspectOptions{},
	)
	if inspectErr != nil {
		clog.Debug().
			Err(inspectErr).
			Str("new_id", string(containerID)).
			Msg("Failed to inspect container after start error")

		return
	}

	state := inspectResult.Container.State
	if state != nil && state.Running {
		clog.Debug().
			Str("new_id", string(containerID)).
			Msg("Skipped cleanup of running container after start error")

		return
	}

	_, rmErr := api.ContainerRemove(
		cleanupCtx,
		string(containerID),
		dockerClient.ContainerRemoveOptions{Force: true},
	)
	if rmErr != nil {
		clog.Debug().
			Err(rmErr).
			Str("new_id", string(containerID)).
			Msg("Failed to clean up container after start error")
	}
}

// createTargetContainer creates a new container based on the source container's
// configuration but does not start it. It applies the provided network configuration,
// renames the container, and attaches additional networks for legacy API versions.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - sourceContainer: Source container to replicate.
//   - networkConfig: Network configuration to apply to the new container.
//   - clientVersion: Docker API version used by the client.
//   - disableMemorySwappiness: If true, disables memory swappiness for Podman compatibility.
//   - cpuCopyMode: CPU copy mode for container recreation, used for compatibility with Podman.
//   - isPodman: If true, indicates Podman is being used for CPU compatibility.
//
// Returns:
//   - types.ContainerID: ID of the new container.
//   - error: Non-nil if creation fails, nil on success.
func createTargetContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	sourceContainer types.Container,
	networkConfig *dockerNetwork.NetworkingConfig,
	clientVersion string,
	disableMemorySwappiness bool,
	cpuCopyMode string,
	isPodman bool,
) (types.ContainerID, error) {
	clogVal := log.With().
		Str("container", sourceContainer.Name()).
		Str("id", sourceContainer.ID().ShortID()).
		Logger()
	clog := &clogVal

	// Extract configuration from the source container.
	config := sourceContainer.GetCreateConfig()
	hostConfig := sourceContainer.GetCreateHostConfig()

	syncComposeImageLabel(ctx, api, clog, sourceContainer, config, clientVersion)

	// Set MemorySwappiness to nil for Podman compatibility if flag is enabled.
	if disableMemorySwappiness {
		hostConfig.MemorySwappiness = nil

		clog.Debug().Msg("Disabled memory swappiness for Podman compatibility")
	}

	// Handle CPU settings based on copy mode.
	handleCPUSettings(hostConfig, cpuCopyMode, isPodman, clog)

	// Log network details for debugging, including MAC address validation.
	isHostNetwork := false

	info := sourceContainer.ContainerInfo()
	if info != nil && info.HostConfig != nil {
		isHostNetwork = info.HostConfig.NetworkMode.IsHost()
	}

	debugLogMacAddress(log,
		networkConfig,
		sourceContainer.ID(),
		clientVersion,
		flags.DockerAPIMinVersion,
		isHostNetwork,
	)

	// Determine network config for container creation based on API version.
	createNetworkConfig := networkConfig

	if versions.LessThan(clientVersion, "1.44") && len(networkConfig.EndpointsConfig) > 1 {
		// Legacy API (< 1.44) with multiple networks: use first network for creation.
		var firstNetworkName string

		createNetworkConfig = newEmptyNetworkConfig()

		for name, endpoint := range networkConfig.EndpointsConfig {
			firstNetworkName = name
			createNetworkConfig.EndpointsConfig[name] = endpoint

			clog.Debug().
				Str("network", firstNetworkName).
				Msg("Selected first network for container creation")

			break // Use only the first network initially.
		}
	} else {
		clog.Debug().Msg("Using full network config for API version >= 1.44 or single network")
	}

	// Create container with the selected network config.
	clog.Debug().Msg("Creating new container")

	createdContainer, err := api.ContainerCreate(
		ctx,
		dockerClient.ContainerCreateOptions{
			Config:           config,
			HostConfig:       hostConfig,
			NetworkingConfig: createNetworkConfig,
			Name:             "",
		},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to create new container")

		return "", fmt.Errorf("%w: %w", errCreateContainerFailed, err)
	}

	createdContainerID := types.ContainerID(createdContainer.ID)
	clog.Debug().
		Str("new_id", string(createdContainerID)).
		Msg("Created container successfully")

	// Rename the container to the correct name to avoid conflicts during self-update
	clog.Debug().Msg("Renaming container to correct name")

	_, err = api.ContainerRename(
		ctx,
		createdContainer.ID,
		dockerClient.ContainerRenameOptions{
			NewName: sourceContainer.Name(),
		},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to rename container")

		// Clean up the created container to avoid orphaned resources
		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			cleanupTimeout,
		)
		defer cancel()

		_, rmErr := api.ContainerRemove(
			cleanupCtx,
			createdContainer.ID,
			dockerClient.ContainerRemoveOptions{Force: true},
		)
		if rmErr != nil {
			clog.Warn().
				Err(rmErr).
				Msg("Failed to clean up container after rename error")
		}

		return "", fmt.Errorf("%w: %w", errRenameContainerFailed, err)
	}

	// Attach additional networks for legacy API if needed.
	if versions.LessThan(clientVersion, "1.44") && len(networkConfig.EndpointsConfig) > 1 {
		err := attachNetworks(
			ctx,
			api,
			createdContainer.ID,
			networkConfig,
			createNetworkConfig,
			clog,
		)
		if err != nil {
			// Clean up the created container to avoid orphaned resources.
			cleanupCtx, cancel := context.WithTimeout(
				context.WithoutCancel(ctx),
				cleanupTimeout,
			)
			defer cancel()

			_, rmErr := api.ContainerRemove(
				cleanupCtx,
				createdContainer.ID,
				dockerClient.ContainerRemoveOptions{Force: true},
			)
			if rmErr != nil {
				clog.Warn().
					Err(rmErr).
					Msg("Failed to clean up container after network attachment error")
			}

			return "", err
		}
	}

	return createdContainerID, nil
}

// syncComposeImageLabel points the Compose image label in config at the
// image the new container is created from.
//
// The label is copied from the source container, so after an update it still
// names the old image, and Docker Compose would recreate the container on its
// next deploy. The label is left unchanged when it is absent, when the new
// image has no value in the same form, or when an image inspect fails.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - clog: Logger with container context fields.
//   - sourceContainer: Source container the new container replaces.
//   - config: Creation config for the new container, updated in place.
//   - clientVersion: Docker API version used by the client.
func syncComposeImageLabel(
	ctx context.Context,
	api dockerClient.APIClient,
	clog *zerolog.Logger,
	sourceContainer types.Container,
	config *dockerContainer.Config,
	clientVersion string,
) {
	label := config.Labels[compose.ComposeImageLabel]

	info := sourceContainer.ContainerInfo()
	if label == "" || info == nil {
		return
	}

	value, ok, err := resolveComposeImageLabel(ctx, api, label, info.Image, config.Image, clientVersion)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to resolve Compose image label, keeping it unchanged")

		return
	}

	if !ok {
		clog.Debug().
			Str("label_value", label).
			Msg("Found no matching value for Compose image label, keeping it unchanged")

		return
	}

	if value == label {
		return
	}

	// Clone the labels, which may be shared with the source container's inspect data.
	config.Labels = maps.Clone(config.Labels)
	config.Labels[compose.ComposeImageLabel] = value

	clog.Debug().
		Str("old_value", label).
		Str("new_value", value).
		Msg("Updated Compose image label")
}

// resolveComposeImageLabel inspects the old and new images and returns the
// Compose image label value for the new image.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - label: Compose image label value recorded for the old image.
//   - oldImageRef: ID of the image the source container runs.
//   - newImageRef: Reference the new container is created from.
//   - clientVersion: Docker API version used by the client.
//
// Returns:
//   - string: Label value for the new image.
//   - bool: False when the new image has no value in the same form as label.
//   - error: Non-nil if an image inspect fails.
func resolveComposeImageLabel(
	ctx context.Context,
	api dockerClient.APIClient,
	label, oldImageRef, newImageRef, clientVersion string,
) (string, bool, error) {
	oldImage, err := inspectImageWithManifests(ctx, api, oldImageRef, clientVersion)
	if err != nil {
		return "", false, err
	}

	newImage, err := inspectImageWithManifests(ctx, api, newImageRef, clientVersion)
	if err != nil {
		return "", false, err
	}

	value, ok := compose.ImageLabelValue(label, oldImage, newImage)

	return value, ok, nil
}

// inspectImageWithManifests inspects an image on the local Docker daemon,
// including its manifests when clientVersion reports them.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - imageRef: Image ID or reference to inspect.
//   - clientVersion: Docker API version used by the client.
//
// Returns:
//   - *dockerImage.InspectResponse: Image metadata.
//   - error: Non-nil if the inspect fails.
func inspectImageWithManifests(
	ctx context.Context,
	api dockerClient.APIClient,
	imageRef, clientVersion string,
) (*dockerImage.InspectResponse, error) {
	var opts []dockerClient.ImageInspectOption
	if versions.GreaterThanOrEqualTo(clientVersion, manifestsMinAPIVersion) {
		opts = append(opts, dockerClient.ImageInspectWithManifests(true))
	}

	result, err := api.ImageInspect(ctx, imageRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errInspectImageFailed, imageRef, err)
	}

	return &result.InspectResponse, nil
}

// attachNetworks connects a container to additional networks for legacy Docker API versions (< 1.44).
//
// It iterates through the provided network config, attaching all networks not included in the initial
// creation config, ensuring compatibility with Docker API < 1.44 where multiple network endpoints may fail.
//
// Parameters:
//   - ctx: Context for container API operations.
//   - api: Docker API client.
//   - containerID: ID of the container to attach networks to.
//   - networkConfig: Full network configuration with all desired endpoints.
//   - initialNetworkConfig: Network configuration used during container creation.
//   - clog: Logger with container-specific context for logging.
//
// Returns:
//   - error: Non-nil if attaching any network fails, nil on success.
func attachNetworks(
	ctx context.Context,
	api dockerClient.APIClient,
	containerID string,
	networkConfig *dockerNetwork.NetworkingConfig,
	initialNetworkConfig *dockerNetwork.NetworkingConfig,
	clog *zerolog.Logger,
) error {
	// Identify the initial network used during creation to skip it.
	var initialNetworkName string
	for name := range initialNetworkConfig.EndpointsConfig {
		initialNetworkName = name

		break
	}

	// Attach each additional network sequentially.
	for name, endpoint := range networkConfig.EndpointsConfig {
		if name != initialNetworkName && name != "" {
			clog.Debug().
				Str("network", name).
				Msg("Attaching additional network to container")

			_, err := api.NetworkConnect(
				ctx,
				name,
				dockerClient.NetworkConnectOptions{
					Container:      containerID,
					EndpointConfig: endpoint,
				},
			)
			if err != nil {
				clog.Error().
					Err(err).
					Str("network", name).
					Msg("Failed to attach additional network")

				return fmt.Errorf("failed to attach network %s: %w", name, err)
			}

			clog.Debug().
				Str("network", name).
				Msg("Successfully attached additional network")
		}
	}

	return nil
}

// renameTargetContainer renames an existing container to the specified target name in Watchtower.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - api: Docker API client.
//   - targetContainer: Container to be renamed.
//   - targetName: New name for the container.
//
// Returns:
//   - error: Non-nil if rename fails, nil on success.
func renameTargetContainer(log *zerolog.Logger,
	ctx context.Context,
	api dockerClient.APIClient,
	targetContainer types.Container,
	targetName string,
) error {
	clogVal := log.With().
		Str("container", targetContainer.Name()).
		Str("id", targetContainer.ID().ShortID()).
		Str("target_name", targetName).
		Logger()
	clog := &clogVal

	// Attempt to rename the container.
	clog.Debug().Msg("Renaming container")

	_, err := api.ContainerRename(
		ctx,
		string(targetContainer.ID()),
		dockerClient.ContainerRenameOptions{
			NewName: targetName,
		},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to rename container")

		return fmt.Errorf("%w: %w", errRenameContainerFailed, err)
	}

	clog.Debug().Msg("Renamed container successfully")

	return nil
}

// handleCPUSettings applies CPU configuration based on the specified copy mode.
//
// It handles Podman compatibility by filtering NanoCPUs when necessary.
// Modes: "auto" (detect Podman and filter), "full" (copy all), "none" (strip all).
func handleCPUSettings(
	hostConfig *dockerContainer.HostConfig,
	cpuCopyMode string,
	isPodman bool,
	clog *zerolog.Logger,
) {
	switch cpuCopyMode {
	case "none":
		// Strip all CPU limits
		hostConfig.NanoCPUs = 0
		hostConfig.CPUShares = 0
		hostConfig.CPUQuota = 0
		hostConfig.CPUPeriod = 0
		hostConfig.CpusetCpus = ""
		hostConfig.CpusetMems = ""

		clog.Debug().Msg("Stripped all CPU settings")
	case "full":
		// Copy all CPU settings unchanged (default behavior)
		clog.Debug().Msg("Copied all CPU settings unchanged")
	case "auto":
		// Use isPodman flag to filter NanoCPUs if Podman
		if isPodman {
			hostConfig.NanoCPUs = 0

			clog.Debug().Msg("Detected Podman, filtered NanoCPUs for compatibility")
		} else {
			clog.Debug().Msg("Detected Docker, copied all CPU settings")
		}
	default:
		clog.Debug().
			Str("mode", cpuCopyMode).
			Msg("Unknown CPU copy mode, defaulting to full")
	}
}
