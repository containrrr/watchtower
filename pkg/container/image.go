package container

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
	"github.com/nicholas-fedor/watchtower/pkg/registry/digest"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Warning strategies for HEAD request failures.
const (
	// warnAlways indicates warnings should always be logged for HEAD request failures.
	warnAlways WarningStrategy = "always"
	// warnNever indicates warnings should never be logged for HEAD request failures.
	warnNever WarningStrategy = "never"
	// warnAuto indicates warnings should be logged for HEAD request failures based on registry heuristics.
	warnAuto WarningStrategy = "auto"

	// maxConcurrentPulls caps simultaneous ImagePull calls per registry host
	// so digest checks can stay parallel without bursting blob downloads
	// against one registry.
	maxConcurrentPulls = 1
)

// pullSlots serializes layer pulls per registry host.
var (
	pullSlotsMu sync.Mutex
	pullSlots   = map[string]chan struct{}{}
)

// IsImagePinnedByDigest reports whether imageName is an immutable digest reference.
//
// It matches bare digests (sha256:...) and repository-qualified digests
// (repo@sha256:...), consistent with Config.Image / ImageName(log, ) for digest-pinned
// containers. Tag references such as nginx:latest are not pinned.
//
// This is the shared pin-detection primitive used by pull/check paths and by the
// update flow after image-name resolution.
//
// Parameters:
//   - imageName: Image name from ImageName(log, ) or Config.Image.
//
// Returns:
//   - bool: True if the image is digest-pinned, false otherwise.
func IsImagePinnedByDigest(imageName string) bool {
	if imageName == "" {
		return false
	}

	// Bare content digests used as image names.
	if strings.HasPrefix(imageName, "sha256:") {
		return true
	}

	ref, err := reference.ParseDockerRef(imageName)
	if err != nil {
		// Parse can fail on some edge forms. Still treat explicit @sha256 as pinned.
		return strings.Contains(imageName, "@sha256:")
	}

	_, digested := ref.(reference.Digested)

	return digested
}

// WarningStrategy defines the policy for logging warnings when HEAD requests fail during image pulls.
//
// It allows configuration of verbosity:
//   - "always" logs all failures
//   - "never" suppresses them
//   - "auto" delegates to registry-specific logic (e.g., WarnOnAPIConsumption).
type WarningStrategy string

// ImageAPI is the Docker API subset the image client uses to inspect, pull,
// and remove images and to read the daemon's registry mirrors.
type ImageAPI interface {
	Info(ctx context.Context, options dockerClient.InfoOptions) (dockerClient.SystemInfoResult, error)
	ImageInspect(
		ctx context.Context,
		image string,
		options ...dockerClient.ImageInspectOption,
	) (dockerClient.ImageInspectResult, error)
	ImagePull(
		ctx context.Context,
		ref string,
		options dockerClient.ImagePullOptions,
	) (dockerClient.ImagePullResponse, error)
	ImageRemove(
		ctx context.Context,
		image string,
		options dockerClient.ImageRemoveOptions,
	) (dockerClient.ImageRemoveResult, error)
	ContainerList(
		ctx context.Context,
		options dockerClient.ContainerListOptions,
	) (dockerClient.ContainerListResult, error)
}

var _ ImageAPI = dockerClient.APIClient(nil)

// imageClient manages image-related operations for Watchtower.
//
// It uses a Docker API client for image tasks.
// imageClient performs image inspect, pull, and stale checks.
type imageClient struct {
	api ImageAPI
	log *zerolog.Logger
	// daemonInfo shares Docker Info and mirrors across check and pull paths.
	daemonInfo *daemonInfoCache
	// registrySettings holds the settings for registry requests.
	registrySettings registryConfig.Registry
	// rateLimiter paces pulls, digest checks, and image age requests per registry host.
	rateLimiter *ratelimit.Limiter
}

// IsContainerStale determines if a container's image is outdated.
//
// It skips pulling if NoPull is set, otherwise pulls and compares images.
// If the image is within the cooldown window, it returns false with
// ErrImageCooldown, indicating the update should be deferred.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container to check.
//   - params: Update parameters (e.g., NoPull flag, cooldown delay).
//   - warnOnHeadFailed: Strategy for logging warnings on HEAD request failures.
//
// Returns:
//   - bool: True if image is stale, false otherwise.
//   - types.ImageID: Latest image ID (or current if not pulled).
//   - string: Latest registry manifest digest (empty if unavailable).
//   - error: Non-nil if pull or inspection fails or cooldown defers the update.
func (c imageClient) IsContainerStale(
	ctx context.Context,
	sourceContainer types.Container,
	params types.UpdateParams,
	warnOnHeadFailed WarningStrategy,
) (bool, types.ImageID, string, error) {
	clogVal := c.logger().With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal

	// Skip pull if NoPull is enabled.
	if sourceContainer.IsNoPull(params) {
		return c.checkLocalImageStaleness(ctx, sourceContainer, clog)
	}

	err := c.PullImage(ctx, sourceContainer, warnOnHeadFailed, params)
	if err != nil {
		if errors.Is(err, ErrImageCooldown) {
			clog.Debug().
				Err(err).
				Msg("Cooldown active - pull skipped")

			return false, sourceContainer.ImageID(), "", err
		}

		if errors.Is(err, errPullImageNotFound) {
			clog.Debug().
				Err(err).
				Msg("Image not found in any registry - treating as up-to-date")

			return false, sourceContainer.ImageID(), "", nil
		}

		clog.Debug().
			Err(err).
			Msg("Failed to pull image")

		return false, sourceContainer.ImageID(), "", err
	}

	return c.HasNewImage(ctx, sourceContainer)
}

// CheckContainerUpdate reports whether a newer image is available without
// downloading image layers.
//
// When NoPull is active it inspects the local Docker cache only. Otherwise it
// compares the container's local digests against the registry (HEAD with GET
// fallback). Cooldown is not applied. It remains an apply-time gate for updates.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container to check.
//   - params: Update parameters (NoPull, LabelPrecedence).
//
// Returns:
//   - bool: True if an update is available, false otherwise.
//   - types.ImageID: Latest local image ID when known (empty on remote-only mismatch).
//   - string: Latest registry manifest digest (empty if unavailable).
//   - error: Non-nil if the check fails, nil on success.
func (c imageClient) CheckContainerUpdate(
	ctx context.Context,
	sourceContainer types.Container,
	params types.UpdateParams,
) (bool, types.ImageID, string, error) {
	clogVal := c.logger().With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal

	// Respect no-pull: monitor local image cache only.
	if sourceContainer.IsNoPull(params) {
		return c.checkLocalImageStaleness(ctx, sourceContainer, clog)
	}

	// Pinned digests cannot receive tag-based updates.
	if IsImagePinnedByDigest(sourceContainer.ImageName()) {
		clog.Debug().Msg("Skipping update check for pinned digest image")

		return false, sourceContainer.ImageID(), "", nil
	}

	clog.Debug().Msg("Checking registry digests for update availability")

	opts, err := registry.GetPullOptions(c.logger(), sourceContainer.ImageName())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to load authentication credentials")

		return false, sourceContainer.ImageID(), "", fmt.Errorf(
			"%w: %s: %w",
			errFailedToLoadPullOptions,
			sourceContainer.ImageName(),
			err,
		)
	}

	mirrorInfo := c.resolveRegistryMirrorConfig(ctx)
	endpoints := c.buildMirrorEndpoints(mirrorInfo, sourceContainer.ImageName())

	match, remoteDigest, err := digest.CompareDigestWithRemote(c.logger(),
		ctx,
		sourceContainer,
		opts.RegistryAuth,
		c.registrySettings,
		c.rateLimiter,
		endpoints...,
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to compare registry digests")

		return false, sourceContainer.ImageID(), "", err
	}

	if match {
		clog.Debug().
			Str("latest_digest", remoteDigest).
			Msg("No update available")

		return false, sourceContainer.ImageID(), remoteDigest, nil
	}

	clog.Info().
		Str("new_id", types.ImageID(remoteDigest).ShortID()).
		Str("latest_digest", remoteDigest).
		Msg("Found new image")

	return true, "", remoteDigest, nil
}

// HasNewImage checks if a newer image exists for the container.
//
// It compares the latest image ID with the current one.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container to check.
//
// Returns:
//   - bool: True if a newer image exists, false if current is latest.
//   - types.ImageID: Latest image ID.
//   - string: Latest registry manifest digest (empty if unavailable).
//   - error: Non-nil if inspection fails, nil on success.
func (c imageClient) HasNewImage(
	ctx context.Context,
	sourceContainer types.Container,
) (bool, types.ImageID, string, error) {
	clogVal := c.logger().With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal
	currentImageID := types.ImageID(sourceContainer.ContainerInfo().Image)

	clog.Debug().Msg("Inspecting latest image")

	// Inspect the latest image by name.
	newImageInfo, err := c.api.ImageInspect(
		ctx,
		sourceContainer.ImageName(),
	)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to inspect latest image")

		return false, currentImageID, "", fmt.Errorf(
			"%w: %s: %w",
			errInspectImageFailed,
			sourceContainer.ImageName(),
			err,
		)
	}

	// Compare IDs to determine staleness.
	newImageID := types.ImageID(newImageInfo.ID)
	if newImageID == currentImageID {
		clog.Debug().Msg("No new image found")

		return false,
			currentImageID,
			ExtractImageDigest(
				newImageInfo.RepoDigests,
				sourceContainer.ImageName(),
			),
			nil
	}

	clog.Info().
		Str("new_id", newImageID.ShortID()).
		Msg("Found new image")

	return true,
		newImageID,
		ExtractImageDigest(
			newImageInfo.RepoDigests,
			sourceContainer.ImageName(),
		),
		nil
}

// ExtractImageDigest extracts the manifest digest portion from RepoDigests.
//
// When imageName is non-empty, it prefers a RepoDigest whose name portion
// matches the image repository (tag stripped). Otherwise it returns the digest
// from the first RepoDigest that contains an "@" separator.
//
// Parameters:
//   - repoDigests: Slice of RepoDigest strings, each in the format "name@sha256:...".
//   - imageName: Optional container image name used to prefer a matching RepoDigest.
//
// Returns:
//   - string: The extracted digest (e.g., "sha256:abc..."), or "" if unavailable.
func ExtractImageDigest(repoDigests []string, imageName string) string {
	var preferred string

	if imageName != "" {
		// Strip any digest suffix so normalization operates on the base name.
		namePart, _, found := strings.Cut(imageName, "@")
		if found {
			imageName = namePart
		}

		// Derive the canonical repository name for preferred-match comparison.
		ref, err := reference.ParseNormalizedNamed(imageName)
		if err == nil {
			preferred = reference.TrimNamed(ref).Name()
		}
	}

	var fallback string

	for _, repoDigest := range repoDigests {
		namePart, digest, found := strings.Cut(repoDigest, "@")
		if !found || digest == "" {
			continue
		}

		// Remember the first valid digest in case no preferred match is found.
		if fallback == "" {
			fallback = digest
		}

		if preferred != "" {
			// Normalize the RepoDigest name and compare canonical repository names.
			ref, err := reference.ParseNormalizedNamed(namePart)
			if err == nil {
				if reference.TrimNamed(ref).Name() == preferred {
					return digest
				}
			}
		}
	}

	return fallback
}

// PullImage fetches the latest image for a container.
//
// It skips pinned images, checks digests, and performs a cooldown check
// before pulling the image.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container whose image to pull.
//   - warnOnHeadFailed: Strategy for logging warnings on HEAD request failures.
//   - params: Update parameters (for per-container cooldown via label or global).
//
// Returns:
//   - Non-nil error if pull fails or cooldown defers the pull.
//   - nil on success or skip.
func (c imageClient) PullImage(
	ctx context.Context,
	sourceContainer types.Container,
	warnOnHeadFailed WarningStrategy,
	params types.UpdateParams,
) error {
	clogVal := c.logger().With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal

	// Skip pulling immutable digest-pinned images (bare sha256: or repo@sha256:).
	if IsImagePinnedByDigest(sourceContainer.ImageName()) {
		clog.Debug().Msg("Skipping pull of pinned digest image")

		return errPinnedImage
	}

	clog.Debug().Msg("Loading authentication credentials")

	// Get pull options with authentication.
	opts, err := registry.GetPullOptions(c.logger(), sourceContainer.ImageName())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to load authentication credentials")

		return fmt.Errorf("%w: %s: %w", errPullImageFailed, sourceContainer.ImageName(), err)
	}

	// Log if authentication credentials are successfully loaded.
	if opts.RegistryAuth != "" {
		clog.Debug().Msg("Authentication credentials loaded")
	}

	// Skip the pull if the digest matches the current image (or local-only).
	skip, skipErr := c.shouldSkipPull(ctx, sourceContainer, opts.RegistryAuth, warnOnHeadFailed)
	if skipErr != nil {
		return skipErr
	}

	if skip {
		return nil
	}

	// Perform cooldown check to prevent downloading images that are still
	// inside the cooldown window.
	_, cooldownErr := c.isOutsideCooldown(ctx, sourceContainer, params)
	if cooldownErr != nil {
		return cooldownErr
	}

	return c.performImagePull(ctx, sourceContainer.ImageName(), opts)
}

// RemoveImageByID deletes an image from the Docker host.
//
// It lists containers first to skip images still in use. Context cancellation
// during that check or the removal itself is returned without a warning.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - imageID: ID of the image to remove.
//   - imageName: Name of the image to remove (for logging purposes).
//
// Returns:
//   - error: Non-nil if removal fails, nil on success.
func (c imageClient) RemoveImageByID(ctx context.Context, imageID types.ImageID, imageName string) error {
	clogVal := c.logger().With().
		Str("image_id", imageID.ShortID()).
		Str("image_name", imageName).
		Logger()
	clog := &clogVal

	containers, err := c.api.ContainerList(
		ctx,
		dockerClient.ContainerListOptions{All: true},
	)
	if err != nil {
		if ctx.Err() != nil {
			clog.Debug().
				Err(err).
				Msg("Image usage check interrupted by cancellation, skipping removal")

			return fmt.Errorf("cannot verify image usage: %w", ctx.Err())
		}

		clog.Warn().
			Err(err).
			Msg("Failed to list containers for image usage check, skipping removal")

		return fmt.Errorf("cannot verify image usage: %w", err)
	}

	for _, container := range containers.Items {
		state := container.State
		if container.ImageID == string(imageID) &&
			(state == dockerContainer.StateRunning ||
				state == dockerContainer.StateRestarting ||
				state == dockerContainer.StatePaused ||
				state == dockerContainer.StateCreated) {
			return ErrImageInUse
		}
	}

	clog.Info().
		Str("notify", "yes").
		Msg("Removing image")

	// Perform image removal with force and pruning.
	items, err := c.api.ImageRemove(
		ctx,
		string(imageID),
		dockerClient.ImageRemoveOptions{
			Force:         true,
			PruneChildren: true,
		},
	)
	if err != nil {
		if ctx.Err() != nil {
			clog.Debug().
				Err(err).
				Msg("Image removal interrupted by cancellation")

			return fmt.Errorf("%w: %s: %w", errRemoveImageFailed, imageID, ctx.Err())
		}

		if cerrdefs.IsNotFound(err) {
			clog.Debug().
				Err(err).
				Msg("Image not found, no removal needed")

			return fmt.Errorf("%w: %s", err, imageID)
		}

		clog.Debug().
			Err(err).
			Msg("Failed to remove image")

		return fmt.Errorf("%w: %s: %w", errRemoveImageFailed, imageID, err)
	}

	// Log removal details if debug is enabled.
	if c.logger().GetLevel() <= zerolog.DebugLevel {
		logImageRemovalDetails(c.logger(), items.Items, imageID, imageName)
	}

	clog.Debug().Msg("Cleaned up old image")

	return nil
}

// logImageRemovalDetails logs detailed information about image removal.
//
// It builds strings of deleted and untagged image IDs and logs them at debug level.
//
// Parameters:
//   - items: Response items from the image removal operation.
//   - imageID: ID of the image that was removed.
//   - imageName: Name of the image that was removed.
func logImageRemovalDetails(log *zerolog.Logger, items []dockerImage.DeleteResponse, imageID types.ImageID, imageName string) {
	deleted := strings.Builder{}
	untagged := strings.Builder{}

	for _, item := range items {
		if item.Deleted != "" {
			if deleted.Len() > 0 {
				deleted.WriteString(", ")
			}

			deleted.WriteString(types.ImageID(item.Deleted).ShortID())
		}

		if item.Untagged != "" {
			if untagged.Len() > 0 {
				untagged.WriteString(", ")
			}

			untagged.WriteString(types.ImageID(item.Untagged).ShortID())
		}
	}

	log.Debug().
		Str("deleted", deleted.String()).
		Str("image_id", imageID.ShortID()).
		Str("image_name", imageName).
		Str("untagged", untagged.String()).
		Msg("Image removal details")
}

// GetImageAnnotations reads the OCI annotations of an image by reference.
//
// This is a local Docker daemon inspect, not a registry request, so it never
// counts against a registry rate limit. It resolves the new image's annotations
// after a pull, which the running container's inspect cannot provide.
//
// A failed inspect is not an error for the caller: an image that is absent or
// unreadable simply has no annotations, and metadata must never fail a session.
//
// Parameters:
//   - ctx: Context for operation control.
//   - imageRef: Image reference, tag or digest, to inspect.
//
// Returns:
//   - oci.Annotations: Annotations from the image config labels, or empty.
func (c imageClient) GetImageAnnotations(
	ctx context.Context,
	imageRef string,
) oci.Annotations {
	clogVal := c.logger().With().Str("image", imageRef).Logger()
	clog := &clogVal

	if imageRef == "" {
		return oci.Annotations{}
	}

	imageInfo, err := c.api.ImageInspect(ctx, imageRef)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to inspect image for OCI annotations")

		return oci.Annotations{}
	}

	if imageInfo.Config == nil {
		return oci.Annotations{}
	}

	anns := oci.FromLabels(imageInfo.Config.Labels)
	clog.Debug().
		Str("version", anns.Version).
		Str("revision", anns.Revision).
		Msg("Read OCI annotations from image")

	return anns
}

// logger returns the image client's logger, or a discarded nop if unset.
func (c imageClient) logger() *zerolog.Logger {
	if c.log != nil {
		return c.log
	}

	return nopLog()
}

// newImageClient creates a new imageClient instance.
//
// The client shares the process-wide Docker Info cache used for registry mirrors.
//
// Parameters:
//   - api: Docker API used for image operations.
//   - log: Logger for debug output.
//   - registrySettings: Settings for registry requests.
//   - rateLimiter: Rate limiter shared by registry requests. Must not be nil.
//
// Returns:
//   - imageClient: Initialized client for image operations.
func newImageClient(
	api ImageAPI,
	log *zerolog.Logger,
	registrySettings registryConfig.Registry,
	rateLimiter *ratelimit.Limiter,
) imageClient {
	return imageClient{
		api:              api,
		log:              log,
		daemonInfo:       sharedDaemonInfoCache,
		registrySettings: registrySettings,
		rateLimiter:      rateLimiter,
	}
}

// shouldSkipPull determines if an image pull can be skipped.
//
// It compares digests via HEAD request to avoid unnecessary pulls.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container to check.
//   - registryAuth: Registry authentication credentials.
//   - warnOnHeadFailed: Strategy for logging warnings on HEAD request failures.
//
// Returns:
//   - bool: True if pull can be skipped, false otherwise.
//   - error: Non-nil when a registry rate limit should abort the pull.
func (c imageClient) shouldSkipPull(
	ctx context.Context,
	sourceContainer types.Container,
	registryAuth string,
	warnOnHeadFailed WarningStrategy,
) (bool, error) {
	clogVal := c.logger().With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal
	clog.Debug().Msg("Checking if pull is needed")

	warn := c.warnOnHeadFailed(sourceContainer, warnOnHeadFailed)

	// Resolve registry mirror configuration from Docker daemon.
	mirrorInfo := c.resolveRegistryMirrorConfig(ctx)

	// Build candidate endpoints: Hub mirrors first, then canonical (empty string).
	// Non-Hub images skip daemon registry-mirrors.
	endpoints := c.buildMirrorEndpoints(mirrorInfo, sourceContainer.ImageName())

	// Compare current and remote digests, trying each endpoint.
	// Local-only images are handled inside CompareDigest (match=true, err=nil).
	match, err := digest.CompareDigest(c.logger(), ctx, sourceContainer, registryAuth, c.registrySettings, c.rateLimiter, endpoints...)
	if err != nil {
		clog.Debug().
			Bool("match", match).
			Err(err).
			Msg("Digest comparison result")
	} else {
		clog.Debug().
			Bool("match", match).
			Msg("Digest comparison result")
	}

	switch {
	case ratelimit.Is(err):
		clog.Debug().
			Err(err).
			Msg("Registry rate limited digest check. Aborting pull")

		return false, fmt.Errorf("digest check rate limited: %w", err)
	case err != nil:
		// Digest retrieval failed. Log based on warning strategy and proceed with pull.
		// The entry stays out of notifications because the pull decides the outcome
		// and reports its own failures.
		headLevel := zerolog.DebugLevel
		if warn {
			headLevel = zerolog.WarnLevel
		}

		clog.WithLevel(headLevel).
			Err(err).
			Str("notify", "no").
			Msg("Digest retrieval failed, falling back to full pull")

		return false, nil
	case match:
		// Digests match (or local-only image treated as up-to-date). No pull needed.
		clog.Debug().Msg("Digest match, skipping pull")

		return true, nil
	default:
		// Digests differ. Proceed with pull.
		clog.Debug().Msg("Digest mismatch, proceeding with pull")

		return false, nil
	}
}

// performImagePull executes a full image pull.
//
// It pulls the image and reads the response to ensure completion.
// The per-host pull slot is held only around the daemon ImagePull
// so cooldown sleeps and pulls to other registries stay unblocked.
//
// Parameters:
//   - ctx: Context for operation control.
//   - imageName: Image to pull.
//   - opts: Pull options with auth.
//
// Returns:
//   - error: Non-nil if pull or read fails, nil on success.
func (c imageClient) performImagePull(
	ctx context.Context,
	imageName string,
	opts dockerClient.ImagePullOptions,
) error {
	clogVal := c.logger().With().
		Str("image", imageName).
		Logger()
	clog := &clogVal
	clog.Debug().Msg("Initiating image pull")

	address, hostErr := auth.GetRegistryAddress(clog, imageName)
	if hostErr != nil || address == "" {
		clog.Debug().
			Err(hostErr).
			Msg("Failed to resolve registry host for rate limiting")
	}

	pullHost := ratelimit.Scope(address, opts.RegistryAuth != "")

	pullErr := c.rateLimiter.Do(ctx, clog, pullHost, func() error {
		err := acquirePullSlot(ctx, pullHost)
		if err != nil {
			return err
		}
		defer releasePullSlot(pullHost)

		// A sibling pull may have recorded a 429 after this attempt passed Wait
		// and while it was queued on the slot. Recheck cooldown without taking
		// another quota token.
		cooldownErr := c.rateLimiter.WaitCooldown(ctx, pullHost)
		if cooldownErr != nil {
			return fmt.Errorf("image pull cooldown wait: %w", cooldownErr)
		}

		response, err := c.api.ImagePull(ctx, imageName, opts)
		if err != nil {
			info := ratelimit.FromErrorMessage(err.Error())
			if info != nil {
				c.rateLimiter.Observe(pullHost, info)

				return info
			}

			// Differentiate error types for appropriate logging and handling.
			// Auth failures and missing images return distinct sentinel errors
			// so callers can programmatically distinguish error categories.
			switch {
			case cerrdefs.IsUnauthorized(err):
				clog.Warn().
					Err(err).
					Msg("Image pull failed: authentication required")

				return fmt.Errorf("%w: %s: %w", errPullImageUnauthorized, imageName, err)
			case cerrdefs.IsNotFound(err):
				clog.Debug().
					Err(err).
					Msg("Image pull failed: image not found in registry")

				return fmt.Errorf("%w: %s: %w", errPullImageNotFound, imageName, err)
			default:
				clog.Debug().
					Err(err).
					Msg("Failed to initiate image pull")

				return fmt.Errorf("%w: %s: %w", errPullImageFailed, imageName, err)
			}
		}
		defer response.Close()

		waitErr := response.Wait(ctx)
		if waitErr != nil {
			info := ratelimit.FromErrorMessage(waitErr.Error())
			if info != nil {
				c.rateLimiter.Observe(pullHost, info)
				clog.Debug().
					Err(info).
					Msg("Registry rate limited image pull")

				return info
			}

			clog.Debug().
				Err(waitErr).
				Msg("Failed to read image pull response")

			return fmt.Errorf("%w: %s: %w", errReadPullResponseFailed, imageName, waitErr)
		}

		clog.Debug().Msg("Image pull completed")

		return nil
	})
	if pullErr != nil {
		return fmt.Errorf("image pull: %w", pullErr)
	}

	return nil
}

// pullSlotFor returns the per-host pull slot channel, creating it when missing.
//
// Parameters:
//   - host: Registry host key. Empty hosts share one unknown-host slot.
//
// Returns:
//   - chan struct{}: Buffered slot channel with capacity maxConcurrentPulls.
func pullSlotFor(host string) chan struct{} {
	pullSlotsMu.Lock()
	defer pullSlotsMu.Unlock()

	slot := pullSlots[host]
	if slot == nil {
		slot = make(chan struct{}, maxConcurrentPulls)
		pullSlots[host] = slot
	}

	return slot
}

// acquirePullSlot waits for a pull concurrency slot for host.
//
// Parameters:
//   - ctx: Context that can cancel the wait.
//   - host: Registry host whose slot should be acquired.
//
// Returns:
//   - error: ctx.Err() when canceled. Nil when a slot was acquired.
func acquirePullSlot(ctx context.Context, host string) error {
	ctxErr := ctx.Err()
	if ctxErr != nil {
		return fmt.Errorf("image pull slot wait canceled: %w", ctxErr)
	}

	select {
	case pullSlotFor(host) <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("image pull slot wait canceled: %w", ctx.Err())
	}
}

// releasePullSlot returns a pull concurrency slot for host.
//
// The caller must have acquired a slot with acquirePullSlot for the same host.
//
// Parameters:
//   - host: Registry host whose slot should be released.
func releasePullSlot(host string) {
	<-pullSlotFor(host)
}

// warnOnHeadFailed decides whether to warn about failed HEAD requests during image pulls.
// It evaluates the warning strategy: "always" returns true, "never" returns false, and "auto" delegates
// to registry-specific logic.
//
// Parameters:
//   - sourceContainer: The container whose image is being checked.
//   - warnOnHeadFailed: The configured warning strategy.
//
// Returns:
//   - bool: True if a warning should be logged, false otherwise.
func (c imageClient) warnOnHeadFailed(
	sourceContainer types.Container,
	warnOnHeadFailed WarningStrategy,
) bool {
	if warnOnHeadFailed == warnAlways {
		return true
	}

	if warnOnHeadFailed == warnNever {
		return false
	}

	return registry.WarnOnAPIConsumption(c.logger(), sourceContainer)
}

// checkLocalImageStaleness checks if a container's image is stale without pulling.
//
// It performs local image inspection and comparison, handling logging.
//
// Parameters:
//   - ctx: Context for operation control.
//   - sourceContainer: Container to check.
//   - clog: Logger with container and image fields.
//
// Returns:
//   - bool: True if image is stale, false otherwise.
//   - types.ImageID: Latest image ID.
//   - error: Non-nil if inspection fails, nil on success.
func (c imageClient) checkLocalImageStaleness(
	ctx context.Context,
	sourceContainer types.Container,
	clog *zerolog.Logger,
) (bool, types.ImageID, string, error) {
	clog.Debug().Msg("Skipping image pull due to no-pull setting - checking local image only")
	clog.Debug().
		Str("current_image_id", string(sourceContainer.ImageID())).
		Msg("Current container image ID")

	stale, latestID, latestDigest, err := c.HasNewImage(ctx, sourceContainer)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to check local image")

		return false, sourceContainer.ImageID(), "", err
	}

	clog.Debug().
		Bool("stale", stale).
		Str("latest_image_id", string(latestID)).
		Msg("Local image check result")

	return stale, latestID, latestDigest, nil
}
