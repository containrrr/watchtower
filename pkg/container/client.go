package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/registry"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Constants for CPUCopyMode values.
const (
	// CPUCopyModeAuto indicates automatic detection of container runtime for
	// CPU copying behavior.
	CPUCopyModeAuto = "auto"

	// DaemonInitTimeout is the default timeout for Docker daemon initialization operations.
	// This timeout applies to initial connection, ping, API version negotiation, and
	// server version retrieval during client initialization.
	// The value is 30 seconds, which should be sufficient for most Docker daemon
	// initialization scenarios while preventing indefinite hangs.
	DaemonInitTimeout = 30 * time.Second

	// maxListRetries is the maximum number of retry attempts for transient Docker
	// connection failures when listing containers.
	maxListRetries = 3

	// maxExecOutputSize is the maximum captured exec stdout/stderr size (1 MiB).
	maxExecOutputSize = 1 << 20
)

// baseListRetryDelay is the base delay for exponential backoff between retries.
// Actual delays are baseDelay * 2^attempt (5s, 10s).
var baseListRetryDelay = 5 * time.Second

// Errors for container health operations.
var (
	// errHealthCheckTimeout indicates that waiting for a container to become healthy timed out.
	errHealthCheckTimeout = errors.New("timeout waiting for container to become healthy")
	// errHealthCheckCanceled indicates that waiting for a container to become healthy was canceled.
	errHealthCheckCanceled = errors.New("context canceled while waiting for container to become healthy")
	// errHealthCheckFailed indicates that a container's health check failed.
	errHealthCheckFailed = errors.New("container health check failed")
	// errRecreateDockerClient indicates failure recreating the Docker client after API version fallback.
	errRecreateDockerClient = errors.New("failed to recreate Docker client")
)

// Runtime represents the type of container runtime detected.
type Runtime int

const (
	// RuntimeUnknown indicates no container runtime has been detected.
	RuntimeUnknown Runtime = iota
	// RuntimeDocker indicates Docker is the container runtime.
	RuntimeDocker
	// RuntimePodman indicates Podman is the container runtime.
	RuntimePodman
)

// Client defines the interface for interacting with the Docker API within Watchtower.
//
// It provides methods for managing containers, images, and executing commands,
// abstracting the underlying Docker client operations.
type Client interface {
	// ListContainers retrieves a list of containers, optionally filtered.
	//
	// If no filters are provided, all containers are returned.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - filter: Optional filters to apply to container list. Multiple filters are combined with logical AND.
	//
	// Returns:
	//   - []types.Container: List of matching containers.
	//   - error: Non-nil if listing fails, nil on success.
	ListContainers(ctx context.Context, filter ...types.Filter) ([]types.Container, error)

	// GetContainer fetches detailed information about a specific container by its ID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - containerID: ID of the container to retrieve.
	//
	// Returns:
	//   - types.Container: Container details if found.
	//   - error: Non-nil if retrieval fails, nil on success.
	GetContainer(ctx context.Context, containerID types.ContainerID) (types.Container, error)

	// GetCurrentWatchtowerContainer retrieves minimal container information
	// for the specified container ID, skipping image inspection.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - containerID: ID of the container to retrieve.
	//
	// Returns:
	//   - types.Container: Container with imageInfo set to nil.
	//   - error: Non-nil if inspection fails, nil on success.
	GetCurrentWatchtowerContainer(ctx context.Context, containerID types.ContainerID) (types.Container, error)

	// StopContainer stops a specified container, respecting the given timeout.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to stop.
	//   - timeout: Duration to wait before forcing stop.
	//
	// Returns:
	//   - error: Non-nil if stop fails, nil on success.
	StopContainer(ctx context.Context, container types.Container, timeout time.Duration) error

	// StopAndRemoveContainer stops and removes a specified container,
	// respecting the given timeout.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to stop and remove.
	//   - timeout: Duration to wait before forcing stop.
	//
	// Returns:
	//   - error: Non-nil if stop/removal fails, nil on success.
	StopAndRemoveContainer(ctx context.Context, container types.Container, timeout time.Duration) error

	// CreateContainer creates a new container based on the provided
	// container's configuration, but does not start it.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Source container to replicate.
	//
	// Returns:
	//   - types.ContainerID: ID of the new container.
	//   - error: Non-nil if creation fails, nil on success.
	CreateContainer(ctx context.Context, container types.Container) (types.ContainerID, error)

	// StartContainer creates and starts a new container based on the provided
	// container's configuration.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Source container to replicate.
	//
	// Returns:
	//   - types.ContainerID: ID of the new container.
	//   - error: Non-nil if creation/start fails, nil on success.
	StartContainer(ctx context.Context, container types.Container) (types.ContainerID, error)

	// RenameContainer renames an existing container to the specified new name.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to rename.
	//   - newName: New name for the container.
	//
	// Returns:
	//   - error: Non-nil if rename fails, nil on success.
	RenameContainer(ctx context.Context, container types.Container, newName string) error

	// IsContainerStale checks if a container's image is outdated compared to
	// the latest available version.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to check.
	//   - params: Update parameters for staleness check.
	//
	// Returns:
	//   - bool: True if image is stale, false otherwise.
	//   - types.ImageID: Latest image ID.
	//   - string: Latest registry manifest digest (empty if unavailable).
	//   - error: Non-nil if check fails, nil on success.
	IsContainerStale(
		ctx context.Context,
		container types.Container,
		params types.UpdateParams,
	) (bool, types.ImageID, string, error)

	// CheckContainerUpdate reports whether a newer image is available without
	// pulling image layers. When NoPull is active it inspects the local cache
	// only. Otherwise, it compares registry digests. Cooldown is not applied.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to check.
	//   - params: Update parameters (NoPull, LabelPrecedence).
	//
	// Returns:
	//   - bool: True if an update is available, false otherwise.
	//   - types.ImageID: Latest local image ID when known (empty for remote-only mismatch).
	//   - string: Latest registry manifest digest (empty if unavailable).
	//   - error: Non-nil if the check fails, nil on success.
	CheckContainerUpdate(
		ctx context.Context,
		container types.Container,
		params types.UpdateParams,
	) (bool, types.ImageID, string, error)

	// GetImageAnnotations reads the OCI annotations of an image by reference.
	//
	// The read is a local Docker daemon inspect, so it never counts against a
	// registry rate limit. It resolves the new image's annotations after a pull,
	// which the running container's inspect cannot provide. An unreadable image
	// yields empty annotations rather than an error, because metadata must never
	// fail a session.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - imageRef: Image reference, tag or digest, to inspect.
	//
	// Returns:
	//   - oci.Annotations: Annotations from the image config labels, or empty.
	GetImageAnnotations(ctx context.Context, imageRef string) oci.Annotations

	// ExecuteCommand runs a command inside a container and returns whether
	// to skip updates based on the result.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to execute command in.
	//   - command: Command to execute.
	//   - timeout: Minutes to wait before timeout (0 for no timeout).
	//   - uid: UID to run command as (-1 to use container default).
	//   - gid: GID to run command as (-1 to use container default).
	//
	// Returns:
	//   - bool: True if updates should be skipped, false otherwise.
	//   - error: Non-nil if execution fails, nil on success.
	ExecuteCommand(
		ctx context.Context,
		container types.Container,
		command string,
		timeout int,
		uid int,
		gid int,
	) (bool, error)

	// RemoveImageByID deletes an image from the Docker host by its ID.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - imageID: ID of the image to remove.
	//   - imageName: Name of the image to remove (for logging purposes).
	//
	// Returns:
	//   - error: Non-nil if removal fails, nil on success.
	RemoveImageByID(ctx context.Context, imageID types.ImageID, imageName string) error

	// WarnOnHeadPullFailed determines whether to log a warning when a HEAD request fails during image pulls.
	//
	// The decision is based on the configured warning strategy and container context.
	//
	// Parameters:
	//   - container: Container to evaluate.
	//
	// Returns:
	//   - bool: True if warning is needed, false otherwise.
	WarnOnHeadPullFailed(container types.Container) bool

	// GetVersion returns the client's API version.
	//
	// Returns:
	//   - string: Docker API version (e.g., "1.44").
	GetVersion() string

	// GetImageDiskUsage returns Docker image storage usage from the daemon.
	//
	// It queries GET /system/df for images only and returns the daemon's
	// aggregated totals. Shared layers are not double-counted.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//
	// Returns:
	//   - types.ImageDiskUsage: Aggregated image usage.
	//   - error: Non-nil if retrieval fails, nil on success.
	GetImageDiskUsage(ctx context.Context) (types.ImageDiskUsage, error)

	// GetInfo returns system information from the Docker daemon.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//
	// Returns:
	//   - types.SystemInfo: Daemon identity fields used for runtime detection.
	//   - error: Non-nil if retrieval fails, nil on success.
	GetInfo(ctx context.Context) (types.SystemInfo, error)

	// Ping verifies connectivity to the Docker daemon.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//
	// Returns:
	//   - error: Non-nil if the daemon is unreachable, nil on success.
	Ping(ctx context.Context) error

	// WaitForContainerHealthy waits for a container to become healthy or times out.
	//
	// The timeout parameter controls the maximum wait duration:
	//   - timeout > 0: Wait up to the specified duration for the container to become healthy.
	//   - timeout <= 0: Wait indefinitely until the container becomes healthy, unhealthy,
	//     or the context is cancelled.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - containerID: ID of the container to wait for.
	//   - timeout: Maximum duration to wait for health. Zero or negative values cause
	//     indefinite waiting.
	//
	// Returns:
	//   - error: Non-nil if timeout is reached, container becomes unhealthy, or inspection
	//     fails. nil if healthy or no health check is configured.
	WaitForContainerHealthy(ctx context.Context, containerID types.ContainerID, timeout time.Duration) error

	// UpdateContainer updates the configuration of an existing container.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to update.
	//   - config: Update configuration containing the changes to apply.
	//
	// Returns:
	//   - error: Non-nil if update fails, nil on success.
	UpdateContainer(ctx context.Context, container types.Container, config dockerContainer.UpdateConfig) error

	// SetRestartPolicy updates a container's restart policy.
	//
	// An empty policy name is treated as no automatic restart (Docker "no").
	// MaximumRetryCount is cleared in that case because retry counts apply only
	// to on-failure policies. Only the restart policy is sent to the Engine.
	// Resource limits are left unchanged. Failures are logged and do not abort
	// the caller.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container whose restart policy should be updated.
	//   - policy: Restart policy to apply. Empty Name disables automatic restart.
	SetRestartPolicy(
		ctx context.Context,
		container types.Container,
		policy dockerContainer.RestartPolicy,
	)

	// RemoveContainer removes a container from the Docker host.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - container: Container to remove.
	//
	// Returns:
	//   - error: Non-nil if removal fails, nil on success.
	RemoveContainer(ctx context.Context, container types.Container) error

	// CreateEphemeralOrchestrator creates a short-lived container that orchestrates
	// the Watchtower self-update transition.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - sourceContainer: The current Watchtower container being replaced.
	//   - newImage: The image reference for the new Watchtower container.
	//   - containerChain: The container chain label for lineage tracking.
	//   - cleanup: When true, the orchestrator removes the old image after handoff.
	//
	// Returns:
	//   - types.ContainerID: ID of the ephemeral orchestrator container.
	//   - error: Non-nil if creation or start fails, nil on success.
	CreateEphemeralOrchestrator(
		ctx context.Context,
		sourceContainer types.Container,
		newImage string,
		containerChain string,
		cleanup bool,
	) (types.ContainerID, error)

	// StartContainerByID starts a container by its ID directly.
	//
	// Unlike StartContainer, this does not check the reviveStopped option or
	// create a new container from a source configuration. It directly starts
	// an existing, already-created container. This is used by the ephemeral
	// orchestrator to start the new container after the old one has been stopped.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - containerID: ID of the container to start.
	//
	// Returns:
	//   - error: Non-nil if starting fails, nil on success.
	StartContainerByID(ctx context.Context, containerID types.ContainerID) error

	// BuildRemoteImage builds an image from a Git URL context.
	//
	// The daemon clones the repository. Watchtower does not.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control.
	//   - remote: Docker Git context URL (https://host/repo.git#commit[:subdir]).
	//   - dockerfile: Dockerfile path relative to that context. Empty means Dockerfile.
	//   - tags: Image tags to apply to the built image.
	//
	// Returns:
	//   - types.ImageID: ID of the built image.
	//   - error: Non-nil if the build fails, nil on success.
	BuildRemoteImage(ctx context.Context, remote, dockerfile string, tags []string) (types.ImageID, error)
}

// client is the concrete implementation of the Client interface.
//
// It wraps the Docker API client and applies custom behavior via ClientOptions.
type client struct {
	ClientOptions

	api dockerClient.APIClient
	log *zerolog.Logger

	// runtimeMu protects the cached runtime detection result.
	runtimeMu sync.Mutex
	// runtimeDetected is true once the container runtime was detected successfully.
	runtimeDetected bool
	// isPodman caches the result of Podman runtime detection.
	isPodman bool

	// copyFileMu protects copyFiles between stop and create of a recreation.
	copyFileMu sync.Mutex
	// copyFiles stores labeled file snapshots keyed by the source container ID.
	copyFiles map[types.ContainerID]copyFileSnapshot
	// copyFileBytes is the total size of archives currently held in copyFiles.
	copyFileBytes int64
	// copyAPI overrides api for copy-file snapshot and inject tests.
	copyAPI ArchiveAPI
}

// ClientOptions configures container management behavior for the Docker client.
type ClientOptions struct {
	RemoveVolumes           bool
	IncludeStopped          bool
	ReviveStopped           bool
	IncludeRestarting       bool
	DisableMemorySwappiness bool
	CPUCopyMode             string
	WarnOnHeadFailed        WarningStrategy
	Fs                      afero.Fs
}

// NewClient initializes a new Client instance for Docker API interactions.
//
// It configures the client using environment variables (e.g., DOCKER_HOST, DOCKER_API_VERSION)
// and validates the API version, falling back to autonegotiation if necessary.
//
// Parameters:
//   - opts: Options to customize container management behavior.
//
// Returns:
//   - Client: Initialized client instance (exits on failure).
func NewClient(log *zerolog.Logger, opts ClientOptions) Client {
	// Initialize client from environment. FromEnv reads DOCKER_HOST, DOCKER_TLS_VERIFY,
	// DOCKER_CERT_PATH, and DOCKER_API_VERSION. When DOCKER_API_VERSION is set, the client
	// uses that fixed version. Otherwise, it defaults to the maximum supported version
	// with automatic negotiation enabled.
	//
	// Construction does not use a context. A timeout is applied only to the subsequent
	// ping and API negotiation step.
	cli, err := dockerClient.New(dockerClient.FromEnv)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to initialize Docker client")
	}

	// Set default filesystem if not provided.
	if opts.Fs == nil {
		opts.Fs = afero.NewOsFs()
	}

	cli, result, err := negotiateDockerAPI(log, cli)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to recreate Docker client")
	}

	log.Debug().
		Str("client_version", cli.ClientVersion()).
		Str("server_version", result.APIVersion).
		Msg("Initialized Docker client")

	return &client{
		api:           cli,
		log:           log,
		ClientOptions: opts,
	}
}

// negotiateDockerAPI pings the daemon to validate connectivity and negotiate the API version.
//
// When DOCKER_API_VERSION is incompatible with the server, it recreates the client without a
// fixed version so negotiation can select a supported API. Other ping failures are non-fatal:
// the client is returned and version negotiation is deferred until the first API call.
//
// Parameters:
//   - log: Process logger.
//   - cli: Docker client created from the environment.
//
// Returns:
//   - *dockerClient.Client: Client to use for subsequent API calls (possibly recreated).
//   - dockerClient.PingResult: Result of a successful ping, or zero when ping failed.
//   - error: Non-nil only when the client cannot be recreated after an incompatible API version.
func negotiateDockerAPI(
	log *zerolog.Logger,
	cli *dockerClient.Client,
) (*dockerClient.Client, dockerClient.PingResult, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		DaemonInitTimeout,
	)
	defer cancel()

	// Ping triggers API version negotiation (or validates DOCKER_API_VERSION if set).
	result, err := cli.Ping(ctx, dockerClient.PingOptions{NegotiateAPIVersion: true})
	if err != nil {
		// If DOCKER_API_VERSION was set to an incompatible value, the ping may fail
		// with a not-found error. Recreate the client without a fixed version to allow
		// negotiation to find a compatible API version.
		if cerrdefs.IsNotFound(err) {
			log.Warn().
				Err(err).
				Msg("DOCKER_API_VERSION incompatible with server. Falling back to autonegotiation")

			recreated, recreateErr := dockerClient.New(dockerClient.FromEnv, dockerClient.WithAPIVersion(""))
			if recreateErr != nil {
				return cli, dockerClient.PingResult{}, fmt.Errorf("%w: %w", errRecreateDockerClient, recreateErr)
			}

			cli = recreated
			result, err = cli.Ping(ctx, dockerClient.PingOptions{NegotiateAPIVersion: true})
		}

		if err != nil {
			log.Warn().
				Err(err).
				Msg("Ping failed during initialization. Will negotiate on first API call")

			return cli, dockerClient.PingResult{}, nil
		}
	}

	return cli, result, nil
}

// ListContainers retrieves a list of containers, optionally filtered.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - filter: Optional filters to apply to container list. Multiple filters are combined with logical AND.
//
// Returns:
//   - []types.Container: List of matching containers.
//   - error: Non-nil if listing fails, nil on success.
func (c *client) ListContainers(ctx context.Context, filter ...types.Filter) ([]types.Container, error) {
	// Podman has no restarting status, so the runtime only matters when
	// restarting containers are included.
	isPodman := c.IncludeRestarting && c.getRuntime(ctx)

	var containerFilter types.Filter

	if len(filter) > 0 {
		if len(filter) == 1 {
			// Single filter: use it directly
			containerFilter = filter[0]
		} else {
			// Multiple filters: combine them with logical AND
			// A container must pass ALL filters to be included
			containerFilter = func(container types.FilterableContainer) bool {
				for _, f := range filter {
					if !f(container) {
						return false
					}
				}

				return true
			}
		}
	}

	// Attempt to list source containers with retry for transient Docker connection failures.
	// The Docker daemon may become temporarily unreachable (e.g., during host maintenance),
	// so retrying with exponential backoff (5s then 10s between attempts) improves resilience.
	var (
		containers []types.Container
		err        error
	)

	for attempt := range maxListRetries {
		containers, err = ListSourceContainers(c.logger(),
			ctx,
			c.api,
			c.ClientOptions,
			containerFilter,
			isPodman,
		)
		if err == nil {
			break
		}

		// Only retry for transient connection errors. Fail immediately for others.
		if !isDaemonConnectionError(err) {
			break
		}

		// Don't sleep after the last attempt.
		if attempt == maxListRetries-1 {
			break
		}

		delay := baseListRetryDelay * time.Duration(1<<attempt)

		c.logger().Warn().
			Int("attempt", attempt+1).
			Int("max", maxListRetries).
			Dur("delay", delay).
			Msg("Docker daemon unavailable, retrying container list...")

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled during container list retry: %w", ctx.Err())
		}
	}

	if err != nil {
		c.logger().Debug().
			Err(err).
			Msg("Failed to list containers")

		return nil, err
	}

	c.logger().Debug().
		Int("count", len(containers)).
		Msg("Listed containers")

	return containers, nil
}

// GetContainer fetches detailed information about a specific container by its ID.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - containerID: ID of the container to retrieve.
//
// Returns:
//   - types.Container: Container details if found.
//   - error: Non-nil if retrieval fails, nil on success.
func (c *client) GetContainer(ctx context.Context, containerID types.ContainerID) (types.Container, error) {
	// Retrieve container details using helper function.
	container, err := GetSourceContainer(c.logger(), ctx, c.api, containerID)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container_id", string(containerID)).
			Msg("Failed to get container")

		return nil, err
	}

	if !container.HasImageInfo() {
		c.logger().Warn().
			Str("container", container.Name()).
			Str("container_id", string(containerID)).
			Str("image", container.ImageName()).
			Msg("Failed to retrieve image info")
	}

	c.logger().Debug().
		Str("container_id", string(containerID)).
		Msg("Retrieved container details")

	return container, nil
}

// GetCurrentWatchtowerContainer retrieves container information for the
// specified container ID, skipping image inspection.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - containerID: ID of the container to retrieve.
//
// Returns:
//   - types.Container: Container with imageInfo set to nil.
//   - error: Non-nil if inspection fails, nil on success.
func (c *client) GetCurrentWatchtowerContainer(
	ctx context.Context,
	containerID types.ContainerID,
) (types.Container, error) {
	clogVal := c.logger().With().
		Str("container_id", string(containerID)).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Inspecting current Watchtower container")

	containerInfo, err := c.api.ContainerInspect(
		ctx,
		string(containerID),
		dockerClient.ContainerInspectOptions{},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to inspect current Watchtower container")

		return nil, fmt.Errorf("%w: %w", errInspectContainerFailed, err)
	}

	clog.Debug().Msg("Retrieved minimal container info")

	return NewContainer(c.logger(), &containerInfo.Container, nil), nil
}

// StopContainer stops a specified container.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to stop.
//   - timeout: Duration to wait before forcing stop.
//
// Returns:
//   - error: Non-nil if stop fails, nil on success.
func (c *client) StopContainer(ctx context.Context, container types.Container, timeout time.Duration) error {
	// Stop container using helper function.
	err := StopSourceContainer(c.logger(), ctx, c.api, container, timeout)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Msg("Failed to stop container")

		return err
	}

	c.logger().Debug().
		Str("container", container.Name()).
		Str("image", container.ImageName()).
		Msg("Stopped container")

	return nil
}

// StopAndRemoveContainer stops and removes a specified container.
//
// AutoRemove containers that were running are left for Docker to delete after
// stop. Non-running AutoRemove containers are removed explicitly so the name
// is available for recreation.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to stop and remove.
//   - timeout: Duration to wait before forcing stop.
//
// Returns:
//   - error: Non-nil if stop/removal fails, nil on success.
func (c *client) StopAndRemoveContainer(ctx context.Context, container types.Container, timeout time.Duration) error {
	// Stop and remove container using helper function with volume option.
	err := StopAndRemoveSourceContainer(c.logger(),
		ctx,
		c.api,
		container,
		timeout,
		c.RemoveVolumes,
	)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Msg("Failed to stop and remove container")

		return err
	}

	c.logger().Debug().
		Str("container", container.Name()).
		Str("image", container.ImageName()).
		Msg("Stopped and removed container")

	return nil
}

// CreateContainer creates a new container based on the provided
// container's configuration, but does not start it.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Source container to replicate.
//
// Returns:
//   - types.ContainerID: ID of the new container.
//   - error: Non-nil if creation fails, nil on success.
func (c *client) CreateContainer(ctx context.Context, container types.Container) (types.ContainerID, error) {
	defer c.DiscardCopyFiles(container.ID())

	clogVal := c.logger().With().
		Str("container", container.Name()).
		Str("image", container.ImageName()).
		Logger()
	clog := &clogVal
	// Only the auto CPU copy mode depends on the runtime.
	isPodman := c.CPUCopyMode == CPUCopyModeAuto && c.getRuntime(ctx)

	clientVersion := c.GetVersion()

	clog.Debug().
		Str("client_version", clientVersion).
		Msg("Obtaining source container network configuration")

	networkConfig := getNetworkConfig(c.logger(), container, clientVersion)

	newID, err := CreateTargetContainer(c.logger(),
		ctx,
		c.api,
		container,
		networkConfig,
		clientVersion,
		flags.DockerAPIMinVersion, // Docker API Version 1.24
		c.DisableMemorySwappiness,
		c.CPUCopyMode,
		isPodman,
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to create new container")

		return "", err
	}

	err = c.injectStoredCopyFiles(ctx, container, newID)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("new_id", newID.ShortID()).
			Msg("Failed to copy files into new container")

		cleanupFailedStartContainer(ctx, c.api, clog, newID)

		return "", err
	}

	clog.Debug().
		Str("new_id", newID.ShortID()).
		Msg("Created new container")

	return newID, nil
}

// StartContainer creates and starts a new container based on an
// existing container's configuration.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Source container to replicate.
//
// Returns:
//   - types.ContainerID: ID of the new container.
//   - error: Non-nil if creation/start fails, nil on success.
func (c *client) StartContainer(ctx context.Context, container types.Container) (types.ContainerID, error) {
	clogVal := c.logger().With().
		Str("container", container.Name()).
		Str("image", container.ImageName()).
		Logger()
	clog := &clogVal

	newID, err := c.CreateContainer(ctx, container)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to start new container")

		return "", err
	}

	if !container.IsRunning() && !c.ReviveStopped {
		clog.Debug().
			Str("new_id", string(newID)).
			Msg("Created container, not starting due to stopped state")

		return newID, nil
	}

	_, err = c.api.ContainerStart(
		ctx,
		string(newID),
		dockerClient.ContainerStartOptions{},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("new_id", string(newID)).
			Msg("Failed to start new container")

		cleanupFailedStartContainer(ctx, c.api, clog, newID)

		return "", fmt.Errorf("%w: %w", errStartContainerFailed, err)
	}

	message := "Started new container"
	if container.IsLinkedToRestarting() {
		message = "Started linked container"
	}

	clog.Debug().
		Str("new_id", newID.ShortID()).
		Msg(message)

	return newID, nil
}

// StartContainerByID starts a container by its ID directly.
//
// Unlike StartContainer, this does not create a new container from a source
// configuration. It directly starts an existing, already-created container
// using the Docker API's ContainerStart method. Callers are responsible for
// checking whether the container should be started (e.g., the reviveStopped
// and noRestart checks in restartStaleContainer).
//
// This method is used by the ephemeral orchestrator to start the new container
// after the old one has been stopped during the self-update sequence.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - containerID: ID of the container to start.
//
// Returns:
//   - error: Non-nil if starting fails, nil on success.
func (c *client) StartContainerByID(
	ctx context.Context,
	containerID types.ContainerID,
) error {
	clogVal := c.logger().With().
		Str("container_id", containerID.ShortID()).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Starting container by ID")

	_, err := c.api.ContainerStart(
		ctx,
		string(containerID),
		dockerClient.ContainerStartOptions{},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to start container by ID")

		return fmt.Errorf("failed to start container %s: %w", containerID.ShortID(), err)
	}

	clog.Debug().Msg("Container started successfully")

	return nil
}

// UpdateContainer updates the configuration of an existing container.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to update.
//   - config: Update configuration containing the changes to apply.
//
// Returns:
//   - error: Non-nil if update fails, nil on success.
func (c *client) UpdateContainer(
	ctx context.Context,
	container types.Container,
	config dockerContainer.UpdateConfig,
) error {
	clogVal := c.logger().With().
		Str("container_id", string(container.ID())).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Updating container configuration")

	_, err := c.api.ContainerUpdate(
		ctx,
		string(container.ID()),
		dockerClient.ContainerUpdateOptions{
			Resources:     &config.Resources,
			RestartPolicy: &config.RestartPolicy,
		},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to update container")

		return fmt.Errorf("failed to update container %s: %w", container.ID(), err)
	}

	clog.Debug().Msg("Container configuration updated")

	return nil
}

// SetRestartPolicy updates a container's restart policy.
//
// An empty policy name is treated as no automatic restart (Docker "no").
// MaximumRetryCount is cleared in that case because retry counts apply only
// to on-failure policies. Only the restart policy is sent to the Engine.
// Resource limits are left unchanged. Failures are logged and do not abort
// the caller.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container whose restart policy should be updated.
//   - policy: Restart policy to apply. Empty Name disables automatic restart.
func (c *client) SetRestartPolicy(
	ctx context.Context,
	container types.Container,
	policy dockerContainer.RestartPolicy,
) {
	if container == nil {
		return
	}

	if policy.Name == "" {
		policy.Name = dockerContainer.RestartPolicyDisabled
		policy.MaximumRetryCount = 0
	}

	clogVal := c.logger().With().
		Str("container_id", string(container.ID())).
		Str("restart_policy", string(policy.Name)).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Setting container restart policy")

	_, err := c.api.ContainerUpdate(
		ctx,
		string(container.ID()),
		dockerClient.ContainerUpdateOptions{
			RestartPolicy: &policy,
		},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to set container restart policy")
	}
}

// RemoveContainer removes a container from the Docker host.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to remove.
//
// Returns:
//   - error: Non-nil if removal fails, nil on success.
func (c *client) RemoveContainer(ctx context.Context, container types.Container) error {
	clogVal := c.logger().With().
		Str("container", container.Name()).
		Str("id", container.ID().ShortID()).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Removing container")

	_, err := c.api.ContainerRemove(
		ctx,
		string(container.ID()),
		dockerClient.ContainerRemoveOptions{
			Force: true,
		},
	)
	if err != nil && !cerrdefs.IsNotFound(err) {
		clog.Debug().
			Err(err).
			Msg("Failed to remove container")

		return fmt.Errorf("%w: %w", errRemoveContainerFailed, err)
	}

	if cerrdefs.IsNotFound(err) {
		clog.Debug().Msg("Container already removed")

		return nil
	}

	clog.Debug().Msg("Container removed")

	return nil
}

// RenameContainer renames an existing container to a new name.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to rename.
//   - newName: New name for the container.
//
// Returns:
//   - error: Non-nil if rename fails, nil on success.
func (c *client) RenameContainer(ctx context.Context, container types.Container, newName string) error {
	// Perform rename using helper function.
	err := RenameTargetContainer(c.logger(), ctx, c.api, container, newName)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container", container.Name()).
			Str("new_name", newName).
			Msg("Failed to rename container")

		return err
	}

	c.logger().Debug().
		Str("container", container.Name()).
		Str("new_name", newName).
		Msg("Renamed container")

	return nil
}

// WarnOnHeadPullFailed decides whether to warn about failed HEAD requests.
//
// Parameters:
//   - container: Container to evaluate.
//
// Returns:
//   - bool: True if warning is needed, false otherwise.
func (c *client) WarnOnHeadPullFailed(container types.Container) bool {
	// Apply warning strategy based on configuration.
	if c.WarnOnHeadFailed == WarnAlways {
		return true
	}

	if c.WarnOnHeadFailed == WarnNever {
		return false
	}

	// Delegate to registry logic for auto strategy.
	return registry.WarnOnAPIConsumption(c.logger(), container)
}

// IsContainerStale checks if a container's image is outdated.
//
// Parameters:
//   - container: Container to check.
//   - params: Update parameters for staleness check.
//
// Returns:
//   - bool: True if stale, false otherwise.
//   - types.ImageID: Latest image ID.
//   - error: Non-nil if check fails, nil on success.
func (c *client) IsContainerStale(
	ctx context.Context,
	container types.Container,
	params types.UpdateParams,
) (bool, types.ImageID, string, error) {
	// Use image client to perform staleness check.
	imgClient := newImageClient(c.api, c.logger())

	stale, newestImage, latestDigest, err := imgClient.IsContainerStale(
		ctx,
		container,
		params,
		c.WarnOnHeadFailed,
	)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Msg("Failed to check container staleness")
	} else {
		c.logger().Debug().
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Bool("stale", stale).
			Str("newest_image", string(newestImage)).
			Msg("Checked container staleness")
	}

	return stale, newestImage, latestDigest, err
}

// CheckContainerUpdate reports whether a newer image is available without pulling.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to check.
//   - params: Update parameters (NoPull, LabelPrecedence).
//
// Returns:
//   - bool: True if an update is available, false otherwise.
//   - types.ImageID: Latest local image ID when known.
//   - string: Latest registry manifest digest (empty if unavailable).
//   - error: Non-nil if the check fails, nil on success.
func (c *client) CheckContainerUpdate(
	ctx context.Context,
	container types.Container,
	params types.UpdateParams,
) (bool, types.ImageID, string, error) {
	imgClient := newImageClient(c.api, c.logger())

	available, newestImage, latestDigest, err := imgClient.CheckContainerUpdate(
		ctx,
		container,
		params,
	)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Msg("Failed to check container for updates")
	} else {
		c.logger().Debug().
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Bool("update_available", available).
			Str("latest_image_id", string(newestImage)).
			Str("latest_digest", latestDigest).
			Msg("Checked container for updates")
	}

	return available, newestImage, latestDigest, err
}

// GetImageAnnotations reads the OCI annotations of an image by reference.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - imageRef: Image reference, tag or digest, to inspect.
//
// Returns:
//   - oci.Annotations: Annotations from the image config labels, or empty.
func (c *client) GetImageAnnotations(ctx context.Context, imageRef string) oci.Annotations {
	imgClient := newImageClient(c.api, c.logger())

	return imgClient.GetImageAnnotations(ctx, imageRef)
}

// ExecuteCommand runs a command inside a container and evaluates its result.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - container: Container to execute command in.
//   - command: Command to execute.
//   - timeout: Minutes to wait before timeout (0 for no timeout).
//   - uid: UID to run command as (-1 to use container default).
//   - gid: GID to run command as (-1 to use container default).
//
// Returns:
//   - bool: True if updates should be skipped, false otherwise.
//   - error: Non-nil if execution fails, nil on success.
func (c *client) ExecuteCommand(
	ctx context.Context,
	container types.Container,
	command string,
	timeout int,
	uid int,
	gid int,
) (bool, error) {
	clogVal := c.logger().With().
		Str("container_id", string(container.ID())).
		Logger()
	clog := &clogVal

	// Generate JSON metadata for the container.
	metadataJSON, err := generateContainerMetadata(container)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to generate container metadata")

		return false, err
	}

	// Set User if UID or GID are specified (non-zero).
	var user string

	switch {
	case uid > 0 && gid > 0:
		user = fmt.Sprintf("%d:%d", uid, gid)
	case uid > 0:
		user = strconv.Itoa(uid)
	case gid > 0:
		user = fmt.Sprintf(":%d", gid)
	}

	if user != "" {
		clog.Debug().
			Str("user", user).
			Msg("Setting exec user")
	}

	// Set up exec configuration with command and metadata.
	clog.Debug().
		Str("command", command).
		Msg("Creating exec instance")
	execConfig := dockerClient.ExecCreateOptions{
		TTY:          true,
		Cmd:          []string{"sh", "-c", command},
		Env:          []string{"WT_CONTAINER=" + metadataJSON},
		User:         user,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create the exec instance using the parent context.
	// Timeout management is handled by waitForExecOrTimeout.
	exec, err := c.api.ExecCreate(
		ctx,
		string(container.ID()),
		execConfig,
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to create exec instance")

		return false, fmt.Errorf("%w: %w", errCreateExecFailed, err)
	}

	// Start the exec instance.
	clog.Debug().
		Str("exec_id", exec.ID).
		Msg("Starting exec instance")

	execStartCheck := dockerClient.ExecStartOptions{Detach: true, TTY: true}

	_, err = c.api.ExecStart(ctx, exec.ID, execStartCheck)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to start exec instance")

		return false, fmt.Errorf("%w: %w", errStartExecFailed, err)
	}

	// Capture output and handle attachment.
	output, err := c.captureExecOutput(ctx, exec.ID)
	if err != nil {
		clog.Warn().
			Err(err).
			Msg("Failed to capture command output")
	}

	// Wait for completion and evaluate result.
	skipUpdate, err := c.waitForExecOrTimeout(
		ctx,
		exec.ID,
		output,
		timeout,
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to inspect exec instance")

		return true, fmt.Errorf("%w: %w", errInspectExecFailed, err)
	}

	clog.Debug().
		Str("command", command).
		Str("output", output).
		Bool("skip_update", skipUpdate).
		Msg("Executed command")

	return skipUpdate, nil
}

// generateContainerMetadata creates a JSON-formatted string of container metadata.
//
// Parameters:
//   - container: Container object to extract metadata from.
//
// Returns:
//   - string: JSON string containing metadata (e.g., name, ID, image name, stop signal, labels).
//   - error: Non-nil if JSON marshaling fails, nil otherwise.
func generateContainerMetadata(container types.Container) (string, error) {
	// Filter Watchtower-specific labels to reduce JSON size
	labels := make(map[string]string)

	containerInfo := container.ContainerInfo()
	if containerInfo != nil &&
		containerInfo.Config != nil {
		for key, value := range containerInfo.Config.Labels {
			if strings.HasPrefix(key, "com.centurylinklabs.watchtower.") {
				labels[key] = value
			}
		}
	}

	metadata := struct {
		Name       string            `json:"name"`
		ID         string            `json:"id"`
		ImageName  string            `json:"image_name"`
		StopSignal string            `json:"stop_signal"`
		Labels     map[string]string `json:"labels"`
	}{
		Name:       container.Name(),
		ID:         string(container.ID()),
		ImageName:  container.ImageName(),
		StopSignal: container.StopSignal(),
		Labels:     labels,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal container metadata: %w", err)
	}

	return string(metadataJSON), nil
}

// RemoveImageByID deletes an image from the Docker host by its ID.
//
// Parameters:
//   - imageID: ID of the image to remove.
//   - imageName: Name of the image to remove (for logging purposes).
//
// Returns:
//   - error: Non-nil if removal fails, nil on success.
func (c *client) RemoveImageByID(
	ctx context.Context,
	imageID types.ImageID,
	imageName string,
) error {
	// Use image client to remove the image.
	imgClient := newImageClient(c.api, c.logger())

	err := imgClient.RemoveImageByID(ctx, imageID, imageName)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Str("image_id", string(imageID)).
			Str("image_name", imageName).
			Msg("Failed to remove image")

		return err
	}

	c.logger().Debug().
		Str("image_id", imageID.ShortID()).
		Str("image_name", imageName).
		Msg("Cleaned up old image")

	return nil
}

// GetVersion returns the client's API version.
//
// Returns:
//   - string: Docker API version (e.g., "1.44").
func (c *client) GetVersion() string {
	return strings.Trim(c.api.ClientVersion(), "\"")
}

// Ping verifies connectivity to the Docker daemon.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//
// Returns:
//   - error: Non-nil if the daemon is unreachable, nil on success.
func (c *client) Ping(ctx context.Context) error {
	_, err := c.api.Ping(
		ctx,
		// Version renegotiation is not necessary.
		dockerClient.PingOptions{NegotiateAPIVersion: false},
	)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}

// GetInfo returns system information from the Docker daemon.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//
// Returns:
//   - types.SystemInfo: Daemon identity fields used for runtime detection.
//   - error: Non-nil if retrieval fails, nil on success.
func (c *client) GetInfo(ctx context.Context) (types.SystemInfo, error) {
	info, err := c.api.Info(ctx, dockerClient.InfoOptions{})
	if err != nil {
		c.logger().Debug().
			Err(err).
			Msg("Failed to get system info")

		return types.SystemInfo{}, fmt.Errorf("failed to get system info: %w", err)
	}

	return types.SystemInfo{
		Name:            info.Info.Name,
		ServerVersion:   info.Info.ServerVersion,
		OSType:          info.Info.OSType,
		OperatingSystem: info.Info.OperatingSystem,
		Driver:          info.Info.Driver,
	}, nil
}

// GetImageDiskUsage returns Docker image storage usage from the daemon.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//
// Returns:
//   - types.ImageDiskUsage: Aggregated image usage.
//   - error: Non-nil if retrieval fails, nil on success.
func (c *client) GetImageDiskUsage(ctx context.Context) (types.ImageDiskUsage, error) {
	result, err := c.api.DiskUsage(ctx, dockerClient.DiskUsageOptions{
		Images: true,
	})
	if err != nil {
		c.logger().Debug().
			Err(err).
			Msg("Failed to get image disk usage")

		return types.ImageDiskUsage{}, fmt.Errorf("failed to get image disk usage: %w", err)
	}

	return types.ImageDiskUsage{
		TotalSize:   result.Images.TotalSize,
		Reclaimable: result.Images.Reclaimable,
		TotalCount:  result.Images.TotalCount,
		ActiveCount: result.Images.ActiveCount,
	}, nil
}

// WaitForContainerHealthy waits for a container to become healthy or times out.
//
// The timeout parameter controls the maximum wait duration:
//   - timeout > 0: Wait up to the specified duration for the container to become healthy.
//   - timeout <= 0: Wait indefinitely until the container becomes healthy, unhealthy,
//     or the context is cancelled.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - containerID: ID of the container to wait for.
//   - timeout: Maximum duration to wait for health. Zero or negative values cause
//     indefinite waiting.
//
// Returns:
//   - error: Non-nil if timeout is reached, container becomes unhealthy, or inspection
//     fails. nil if healthy or no health check is configured.
func (c *client) WaitForContainerHealthy(
	ctx context.Context,
	containerID types.ContainerID,
	timeout time.Duration,
) error {
	// Guard against zero/negative timeouts by using a non-deadline context.
	// This allows the function to poll at least once rather than immediately timing out.
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	defer cancel()

	clogVal := c.logger().With().
		Str("container_id", string(containerID)).
		Logger()
	clog := &clogVal

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Distinguish between timeout and cancellation for clearer diagnostics
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				clog.Warn().Msg("Timeout waiting for container to become healthy")

				return fmt.Errorf("%w: %s", errHealthCheckTimeout, containerID)
			}
			// Context was cancelled (e.g., graceful shutdown, parent cancellation)
			clog.Debug().Msg("Context canceled while waiting for container to become healthy")

			return fmt.Errorf("%w: %s", errHealthCheckCanceled, containerID)
		case <-ticker.C:
			// Inspect the container to check health status
			inspect, err := c.api.ContainerInspect(
				ctx,
				string(containerID),
				dockerClient.ContainerInspectOptions{},
			)
			if err != nil {
				clog.Debug().
					Err(err).
					Msg("Failed to inspect container for health check")

				return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
			}

			// Check if health check is configured
			if inspect.Container.State == nil || inspect.Container.State.Health == nil {
				clog.Debug().Msg("No health check configured for container, proceeding")

				return nil
			}

			status := inspect.Container.State.Health.Status
			clog.Debug().
				Str("health_status", string(status)).
				Msg("Checked container health status")

			if status == "healthy" {
				clog.Debug().Msg("Container is now healthy")

				return nil
			}

			if status == "unhealthy" {
				clog.Warn().Msg("Container health check failed")

				return fmt.Errorf("%w: %s", errHealthCheckFailed, containerID)
			}

			// Continue polling for "starting" or other statuses
		}
	}
}

// logger returns the client's process logger, or a discarded nop if unset.
// Production paths always set a real logger via NewClient.
// Tests may construct partial *client values without log.
func (c *client) logger() *zerolog.Logger {
	if c != nil && c.log != nil {
		return c.log
	}

	return nopLog()
}

// detectRuntime determines the container runtime using multiple detection methods.
//
// It iterates through detection helpers in priority order, returning as soon as one
// helper indicates Podman/Docker or an error occurs.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//
// Returns:
//   - bool: True if Podman is detected, false otherwise.
//   - error: Non-nil if detection fails, nil on success.
func (c *client) detectRuntime(ctx context.Context) (bool, error) {
	// Priority 1: Check for marker files (Podman or Docker)
	runtime, err := c.detectRuntimeByMarker()
	if err != nil {
		return false, err
	}

	// Handle the detected runtime from marker files
	switch runtime {
	case RuntimePodman:
		return true, nil
	case RuntimeDocker:
		return false, nil
	case RuntimeUnknown:
		// Continue to next detection method
	default:
		// Continue to next detection method
	}

	// Priority 2: Check CONTAINER environment variable
	if c.detectRuntimeByEnv() {
		return true, nil
	}

	// Priority 3: API-based detection
	return c.detectRuntimeByAPI(ctx)
}

// detectRuntimeByMarker checks for container runtime marker files.
//
// It first checks for Podman's marker file, then checks for Docker's marker file.
// If either check returns a non-NotExist error (e.g., permission denied), an error
// is returned with details about which path failed.
//
// Parameters:
//   - c: The client instance for filesystem access.
//
// Returns:
//   - Runtime: The detected container runtime (RuntimePodman, RuntimeDocker, or RuntimeUnknown).
//   - error: Non-nil if checking fails with a non-NotExist error, nil on success.
func (c *client) detectRuntimeByMarker() (Runtime, error) {
	// Check for Podman marker file
	_, podmanErr := c.Fs.Stat("/run/.containerenv")
	if podmanErr == nil {
		c.logger().Debug().Msg("Detected Podman via marker file /run/.containerenv")

		return RuntimePodman, nil
	}

	// Check for Docker marker file
	_, dockerErr := c.Fs.Stat("/.dockerenv")
	if dockerErr == nil {
		c.logger().Debug().Msg("Detected Docker via marker file /.dockerenv")

		return RuntimeDocker, nil
	}

	// Both checks failed - determine if it's due to missing files or actual errors
	podmanNotExist := errors.Is(podmanErr, os.ErrNotExist)
	dockerNotExist := errors.Is(dockerErr, os.ErrNotExist)

	// If both files simply don't exist, that's expected (not an error)
	if podmanNotExist && dockerNotExist {
		return RuntimeUnknown, nil
	}

	// At least one check returned a non-NotExist error (e.g., permission denied)
	// Build an informative error message
	switch {
	case !podmanNotExist && dockerNotExist:
		// Podman check failed with non-NotExist error
		return RuntimeUnknown, fmt.Errorf(
			"failed to check Podman marker file /run/.containerenv: %w",
			podmanErr,
		)
	case podmanNotExist && !dockerNotExist:
		// Docker check failed with non-NotExist error
		return RuntimeUnknown, fmt.Errorf(
			"failed to check Docker marker file /.dockerenv: %w",
			dockerErr,
		)
	default:
		// Both checks failed with non-NotExist errors
		return RuntimeUnknown, fmt.Errorf(
			"failed to check marker files: /run/.containerenv: %w, /.dockerenv: %w",
			podmanErr,
			dockerErr,
		)
	}
}

// It checks if the container environment variable, which Podman sets, or its
// uppercase form is "podman" or "oci", both of which indicate Podman is the
// container runtime.
//
// Returns:
//   - bool: True if the environment variable indicates Podman, false otherwise.
func (c *client) detectRuntimeByEnv() bool {
	for _, key := range []string{"container", "CONTAINER"} {
		value := os.Getenv(key)
		if value == "podman" || value == "oci" {
			c.logger().Debug().
				Str("variable", key).
				Msg("Detected Podman via container environment variable")

			return true
		}
	}

	return false
}

// detectRuntimeByAPI uses the Docker API to detect if the runtime is Podman.
//
// It queries the system info endpoint and checks the Name and ServerVersion
// fields for Podman indicators.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - c: The client instance for API calls.
//
// Returns:
//   - bool: True if Podman is detected via API, false otherwise.
//   - error: Non-nil if API call fails, nil on success.
func (c *client) detectRuntimeByAPI(ctx context.Context) (bool, error) {
	info, err := c.GetInfo(ctx)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Msg("Failed to get system info for Podman detection, assuming Docker")

		return false, err
	}

	if info.Name == "podman" {
		c.logger().Debug().Msg("Detected Podman via API Name field")

		return true, nil
	}

	if strings.Contains(strings.ToLower(info.ServerVersion), "podman") {
		c.logger().Debug().Msg("Detected Podman via API ServerVersion field")

		return true, nil
	}

	c.logger().Debug().Msg("No Podman detection criteria met, assuming Docker")

	return false, nil
}

// getRuntime reports whether the container runtime is Podman.
//
// Callers ask only when the answer changes their behavior: the container list
// filter when restarting containers are included, and CPU handling in the auto
// CPU copy mode. A successful detection is cached to avoid repeated API calls
// and filesystem checks. The runtime is assumed to not change during the
// lifetime of the client. A failed or canceled detection is not cached, so the
// next call tries again.
//
// Parameters:
//   - ctx: Context for the detection's API call.
//
// Returns:
//   - bool: True if Podman is detected, false otherwise.
func (c *client) getRuntime(ctx context.Context) bool {
	c.runtimeMu.Lock()
	defer c.runtimeMu.Unlock()

	if c.runtimeDetected {
		return c.isPodman
	}

	isPodman, err := c.detectRuntime(ctx)
	if err != nil {
		c.logger().Debug().
			Err(err).
			Msg("Failed to detect container runtime, falling back to Docker")

		return false
	}

	c.isPodman = isPodman
	c.runtimeDetected = true

	return isPodman
}

// captureExecOutput attaches to an exec instance and captures its output.
//
// Parameters:
//   - ctx: Context for lifecycle control.
//   - execID: ID of the exec instance.
//
// Returns:
//   - string: Captured output if successful.
//   - error: Non-nil if attachment or reading fails, nil on success.
func (c *client) captureExecOutput(ctx context.Context, execID string) (string, error) {
	clogVal := c.logger().With().
		Str("exec_id", execID).
		Logger()
	clog := &clogVal

	// Attach to the exec instance for output.
	clog.Debug().Msg("Attaching to exec instance")

	response, err := c.api.ExecAttach(
		ctx,
		execID,
		dockerClient.ExecAttachOptions{TTY: true},
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to attach to exec instance")

		return "", fmt.Errorf("%w: %w", errAttachExecFailed, err)
	}

	defer response.Close()

	// Read output into a buffer with timeout and a size cap.
	var writer bytes.Buffer

	done := make(chan error, 1)

	go func() {
		_, err := io.Copy(
			&writer,
			io.LimitReader(
				response.Reader,
				maxExecOutputSize,
			),
		)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			clog.Debug().
				Err(err).
				Msg("Failed to read exec output")

			return "", fmt.Errorf("%w: %w", errReadExecOutputFailed, err)
		}
	case <-ctx.Done():
		response.Close()

		return "", fmt.Errorf("%w: %w", errReadExecOutputFailed, ctx.Err())
	}

	// Return trimmed output if any was captured.
	if writer.Len() > 0 {
		output := strings.TrimSpace(writer.String())
		clog.Debug().
			Str("output", output).
			Msg("Captured exec output")

		return output, nil
	}

	return "", nil
}

// waitForExecOrTimeout waits for an exec instance to complete or times out.
//
// Parameters:
//   - ctx: Parent context.
//   - execID: ID of the exec instance.
//   - execOutput: Captured output for error reporting.
//   - timeout: Minutes to wait (0 for no timeout).
//
// Returns:
//   - bool: True if updates should be skipped (exit code 75), false otherwise.
//   - error: Non-nil if inspection fails or command errors, nil on success.
func (c *client) waitForExecOrTimeout(
	ctx context.Context,
	execID string,
	execOutput string,
	timeout int,
) (bool, error) {
	const ExTempFail = 75

	clogVal := c.logger().With().
		Str("exec_id", execID).
		Logger()
	clog := &clogVal

	var execCtx context.Context

	var cancel context.CancelFunc

	// Set up context with timeout if specified.
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(
			ctx,
			time.Duration(timeout)*time.Minute,
		)
		defer cancel()
	} else {
		execCtx = ctx
	}

	// Poll exec status until completion.
	for {
		execInspect, err := c.api.ExecInspect(execCtx, execID, dockerClient.ExecInspectOptions{})
		if err != nil {
			clog.Debug().
				Err(err).
				Msg("Failed to inspect exec instance")

			return false, fmt.Errorf("%w: %w", errInspectExecFailed, err)
		}

		clog.Debug().
			Int("exit_code", execInspect.ExitCode).
			Bool("running", execInspect.Running).
			Msg("Checked exec status")

		if execInspect.Running {
			select {
			case <-time.After(1 * time.Second):
				continue
			case <-execCtx.Done():
				return false, fmt.Errorf("exec canceled: %w", execCtx.Err())
			}
		}

		// Log output if present.
		if len(execOutput) > 0 {
			clog.Debug().
				Int("output_length", len(execOutput)).
				Msg("Command output captured")
		}

		// Handle specific exit codes.
		if execInspect.ExitCode == ExTempFail {
			return true, nil // Skip updates on temporary failure.
		}

		if execInspect.ExitCode > 0 {
			err := fmt.Errorf(
				"%w with exit code %d: %s",
				errCommandFailed,
				execInspect.ExitCode,
				execOutput,
			)
			clog.Debug().
				Err(err).
				Msg("Command execution failed")

			return false, err
		}

		break
	}

	return false, nil
}

// isDaemonConnectionError checks if an error is a transient Docker daemon connection error.
// These errors indicate the Docker daemon is temporarily unreachable and may self-heal,
// making them suitable for retry logic.
//
// Parameters:
//   - err: The error to check.
//
// Returns:
//   - bool: True if the error is a Docker daemon connection error, false otherwise.
func isDaemonConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	return strings.Contains(errMsg, "Cannot connect to the Docker daemon") ||
		strings.Contains(errMsg, "connection refused") ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}
