package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// stopContainerTimeout sets the container stop timeout.
const stopContainerTimeout = 10 * time.Minute

// maxRemovalAttempts sets the maximum number of retries for container removal operations.
// Docker's default stop timeout is 10 seconds, but our stopContainerTimeout overrides it
// to 10 minutes. With 30 attempts and a 1s delay between retries, the total retry window
// is approximately 30 seconds (30 x 1s), which covers the default Docker stop timeout
// plus overhead for image removal delays.
const maxRemovalAttempts = 30

// removalRetryDelay sets the delay before retrying removal operations.
var removalRetryDelay = 1 * time.Second

// RemoveExcessWatchtowerInstances ensures a single Watchtower container within the same scope.
//
// It identifies multiple Watchtower containers within the same scope, stops all but the current,
// and collects removed images for deferred removal if enabled, preventing conflicts from concurrent containers.
// Chain identification uses the current container's labels to determine old containers to remove.
// Scoped instances only remove other instances in the same scope, allowing coexistence with different scopes.
// Removal operations respect scope boundaries to prevent cross-scope interference.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - cleanupImages: Remove images if true.
//   - watchtowerScope: Scope to filter Watchtower containers.
//   - removeImageInfos: List that receives the images of removed instances for the caller to remove.
//     Nil removes them right away.
//   - currentContainer: The current running Watchtower container.
//
// Returns:
//   - int: Number of removed Watchtower containers.
//   - error: Non-nil if removal fails, nil if single instance or successful removal.
func RemoveExcessWatchtowerInstances(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	cleanupImages bool,
	scope string,
	removeImageInfos *[]types.RemovedImageInfo,
	currentContainer types.Container,
) (int, error) {
	log.Debug().
		Str("scope", scope).
		Bool("cleanup_images", cleanupImages).
		Str("current_container_id", func() string {
			if currentContainer != nil {
				return string(currentContainer.ID())
			}

			return ""
		}()).
		Msg("Starting removal of excess Watchtower containers")

	// List all containers to find excess instances
	allContainers, err := client.ListContainers(ctx, filters.NoFilter)
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	// Retrieve containers that are excess Watchtower containers within the same scope
	excessWatchtowerContainers := getExcessContainers(log,
		scope,
		currentContainer,
		allContainers,
	)

	// If no excess containers found, nothing to remove
	if len(excessWatchtowerContainers) == 0 {
		log.Debug().
			Str("scope", scope).
			Msg("No excess containers found")

		return 0, nil
	}

	// Stop and remove the excess containers, collecting removed image info if removal is enabled
	removed, err := removeExcessContainers(log,
		ctx,
		client,
		excessWatchtowerContainers,
		cleanupImages,
		currentContainer,
		removeImageInfos,
	)
	if err != nil {
		return removed, err
	}

	return removed, nil
}

// cleanupOldWatchtowerContainers removes old Watchtower containers that
// linger from a previous self-update. Unlike RemoveExcessWatchtowerInstances
// (which runs once at startup), this is designed to be called during each update
// cycle to catch any old containers that the startup cleanup may have missed.
//
// It identifies containers matching the watchtower-old- prefix within the same
// scope as the current container, stops them, and optionally cleans up their
// images. This ensures that even if an old container survives the initial
// cleanup (e.g., it was still stopping), it won't persist across update cycles.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - cleanupImages: Remove images if true.
//   - scope: Scope to filter Watchtower containers (empty for unscoped).
//   - currentContainerID: ID of the currently running Watchtower container.
//   - removeImageInfos: List that receives the images of removed instances for the caller to remove.
//     Nil removes them right away.
//
// Returns:
//   - int: Number of removed old Watchtower containers.
//   - error: Non-nil if removal fails, nil if none found or successful removal.
func cleanupOldWatchtowerContainers(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	cleanupImages bool,
	scope string,
	currentContainerID types.ContainerID,
	removeImageInfos *[]types.RemovedImageInfo,
) (int, error) {
	log.Debug().
		Str("scope", scope).
		Str("current_id", string(currentContainerID)).
		Bool("cleanup_images", cleanupImages).
		Msg("Checking for old Watchtower containers")

	// Normalize empty scope to "none" for consistent comparison.
	if scope == "" {
		scope = "none"
	}

	// List all containers to find old instances
	allContainers, err := client.ListContainers(ctx, filters.NoFilter)
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	// Find old Watchtower containers within the same scope
	var oldContainers []types.Container

	// Iterate all containers to find old Watchtower containers that
	// should be cleaned up. Non-Watchtower containers and containers with
	// normal names are skipped immediately.
	for _, c := range allContainers {
		// Skip non-Watchtower containers
		if !c.IsWatchtower() {
			continue
		}

		// Skip containers that are not old predecessors
		if !container.IsOldContainer(c.Name()) {
			continue
		}

		// Scope check: only clean up old containers in the same scope
		containerScope, containerHasScope := c.Scope()
		if !containerHasScope || containerScope == "" {
			containerScope = "none"
		}

		if containerScope != scope {
			log.Debug().
				Str("container", c.Name()).
				Str("container_scope", containerScope).
				Str("current_scope", scope).
				Msg("Skipping old Watchtower container in different scope")

			continue
		}

		// The updater's self-detection handles the case where the current
		// container is old (it exits before reaching this point), but
		// we still guard here for safety in case this function is called from
		// a different code path.
		if c.ID() == currentContainerID {
			continue
		}

		oldContainers = append(oldContainers, c)
	}

	// Find orphaned Watchtower containers that are stuck in the created state.
	// These can be left behind when a self-update creates a replacement but
	// fails to start it. The new container never runs and the old instance
	// may already be running under its original name.
	var orphanedCreatedContainers []types.Container

	for _, c := range allContainers {
		if !c.IsWatchtower() {
			continue
		}

		if c.ID() == currentContainerID {
			continue
		}

		if container.IsOldContainer(c.Name()) {
			continue
		}

		if !c.IsCreated() {
			continue
		}

		// Scope check: only clean up orphaned containers in the same scope.
		containerScope, containerHasScope := c.Scope()
		if !containerHasScope || containerScope == "" {
			containerScope = "none"
		}

		if containerScope != scope {
			log.Debug().
				Str("container", c.Name()).
				Str("container_scope", containerScope).
				Str("current_scope", scope).
				Msg("Skipping orphaned Watchtower container in different scope")

			continue
		}

		orphanedCreatedContainers = append(orphanedCreatedContainers, c)
	}

	oldContainers = append(oldContainers, orphanedCreatedContainers...)

	if len(oldContainers) == 0 {
		log.Debug().Msg("No old or orphaned Watchtower containers found")

		return 0, nil
	}

	log.Info().
		Int("count", len(oldContainers)).
		Int("old_count", len(oldContainers)-len(orphanedCreatedContainers)).
		Int("orphaned_created_count", len(orphanedCreatedContainers)).
		Strs("containers", containerNames(oldContainers)).
		Msg("Found old or orphaned Watchtower containers, cleaning up")

	// Find the current container in the list so image collection works
	var currentContainerObj types.Container

	for _, c := range allContainers {
		if c.ID() == currentContainerID {
			currentContainerObj = c

			break
		}
	}

	// Reuse the existing removal logic for old containers
	removed, err := removeExcessContainers(log,
		ctx,
		client,
		oldContainers,
		cleanupImages,
		currentContainerObj,
		removeImageInfos,
	)
	if err != nil {
		return removed, err
	}

	return removed, nil
}

// getExcessContainers retrieves a list of excess Watchtower containers that should be removed.
//
// It identifies containers that are duplicates within the same scope or part of a container chain,
// excluding the current running container, to ensure only one Watchtower instance operates per scope.
//
// Parameters:
//   - watchtowerScope: Scope to filter containers, empty for unscoped.
//   - currentContainer: The current running Watchtower container (nil if not applicable).
//   - allContainers: All containers to search for excess instances.
//
// Returns:
//   - []types.Container: Slice of containers to remove.
func getExcessContainers(log *zerolog.Logger, watchtowerScope string,
	currentContainer types.Container,
	allContainers []types.Container,
) []types.Container {
	log.Debug().
		Str("scope", watchtowerScope).
		Str("current_container_id", func() string {
			if currentContainer != nil {
				return string(currentContainer.ID())
			}

			return ""
		}()).
		Msg("Retrieving excess containers")

	filteredContainers := getFilteredContainers(log,
		watchtowerScope,
		currentContainer,
		allContainers,
	)

	var chainedContainers []types.Container
	if currentContainer != nil {
		chainedContainers = getChainedContainers(log, allContainers, currentContainer)
	}

	excessContainers := addExcessContainers(filteredContainers, chainedContainers)

	return excessContainers
}

// getFilteredContainers returns the Watchtower containers in the given scope other
// than the current one.
//
// When the current container is an old predecessor, the successor it hands off
// to counts as the current one instead, when one exists. The successor is the
// non-old container in the same scope whose chain label lists the predecessor's
// ID, or else the first non-old container in that scope.
//
// Parameters:
//   - log: Process logger.
//   - scope: Scope UID to filter containers, empty for unscoped.
//   - currentContainer: The current running Watchtower container (nil if not applicable).
//   - allContainers: All containers to filter from.
//
// Returns:
//   - []types.Container: Excess containers to remove, empty when currentContainer is nil.
func getFilteredContainers(log *zerolog.Logger, scope string,
	currentContainer types.Container,
	allContainers []types.Container,
) []types.Container {
	if currentContainer == nil {
		return []types.Container{}
	}

	var filter types.Filter

	switch {
	case scope != "":
		filter = filters.FilterByScope(log, scope, filters.WatchtowerContainersFilter)
	default:
		filter = filters.UnscopedWatchtowerContainersFilter
	}

	var filteredContainers []types.Container

	for _, c := range allContainers {
		if filter == nil || filter(c) {
			filteredContainers = append(filteredContainers, c)
		}
	}

	var excessContainers []types.Container

	currentID := string(currentContainer.ID())
	if container.IsOldContainer(currentContainer.Name()) {
		// Detection selected an old predecessor. Resolve the true
		// successor by lineage: prefer a non-old container whose
		// chain label contains the old container's ID, confirming it is
		// the direct successor in the self-update chain.
		currentScope, currentHasScope := currentContainer.Scope()
		if !currentHasScope || currentScope == "" {
			currentScope = "none"
		}

		chainMatchID := ""
		scopeMatchID := ""

		for _, c := range filteredContainers {
			if container.IsOldContainer(c.Name()) {
				continue
			}

			candidateScope, candidateHasScope := c.Scope()
			if !candidateHasScope || candidateScope == "" {
				candidateScope = "none"
			}

			if candidateScope != currentScope {
				continue
			}

			if scopeMatchID == "" {
				scopeMatchID = string(c.ID())
			}

			chainValue, hasChain := c.GetContainerChain()
			if !hasChain || chainValue == "" {
				continue
			}

			for chainID := range strings.SplitSeq(chainValue, ",") {
				if strings.TrimSpace(chainID) == string(currentContainer.ID()) {
					chainMatchID = string(c.ID())

					break
				}
			}

			if chainMatchID != "" {
				break
			}
		}

		if chainMatchID != "" {
			currentID = chainMatchID
		} else if scopeMatchID != "" {
			currentID = scopeMatchID
		}
	}

	for _, c := range filteredContainers {
		if string(c.ID()) != currentID {
			excessContainers = append(excessContainers, c)
		}
	}

	log.Debug().
		Str("scope", scope).
		Int("excess_containers_found", len(excessContainers)).
		Int("filtered_containers_total", len(filteredContainers)).
		Msg("Filtered excess containers")

	return excessContainers
}

// getChainedContainers retrieves containers linked in a chain based on the current container's chain label.
//
// It parses the container chain label from the current container, identifies the linked
// Watchtower containers excluding the current one, and returns them as a slice. A linked
// container that is not a Watchtower container is never returned. If the current container
// has no chain label or an empty chain label, an empty slice is returned.
//
// Parameters:
//   - log: Process logger.
//   - allContainers: All containers to search for chained containers.
//   - currentContainer: The current running Watchtower container (nil if not applicable).
//
// Returns:
//   - []types.Container: Slice of chained containers excluding the current one.
func getChainedContainers(log *zerolog.Logger,
	allContainers []types.Container,
	currentContainer types.Container,
) []types.Container {
	var chainedContainers []types.Container

	effectiveCurrent := currentContainer
	if currentContainer != nil && container.IsOldContainer(currentContainer.Name()) {
		// Detection selected old. Resolve the true successor by
		// lineage: prefer a non-old Watchtower container whose chain
		// label contains the old container's ID. Fall back to scope-only
		// matching if no explicit lineage link exists.
		currentScope, currentHasScope := currentContainer.Scope()
		if !currentHasScope || currentScope == "" {
			currentScope = "none"
		}

		chainMatch := false

		for _, c := range allContainers {
			if !c.IsWatchtower() || container.IsOldContainer(c.Name()) {
				continue
			}

			candidateScope, candidateHasScope := c.Scope()
			if !candidateHasScope || candidateScope == "" {
				candidateScope = "none"
			}

			if candidateScope != currentScope {
				continue
			}

			chainValue, hasChain := c.GetContainerChain()
			if !hasChain || chainValue == "" {
				if !chainMatch {
					effectiveCurrent = c
				}

				continue
			}

			for chainID := range strings.SplitSeq(chainValue, ",") {
				if strings.TrimSpace(chainID) == string(currentContainer.ID()) {
					effectiveCurrent = c
					chainMatch = true

					break
				}
			}

			if chainMatch {
				break
			}
		}

		if !chainMatch && effectiveCurrent == currentContainer {
			effectiveCurrent = nil
		}
	}

	if effectiveCurrent == nil {
		return []types.Container{}
	}

	// Get the (effective) current Watchtower container's com.centurylinklabs.watchtower.container-chain label.
	chainLabelValue, present := effectiveCurrent.GetContainerChain()

	// If it's not present, there are no chained containers.
	if !present {
		return []types.Container{}
	}

	// If it's empty, there are no chained containers.
	if chainLabelValue == "" {
		return []types.Container{}
	}

	// Collect the container IDs from the chain for efficient lookup.
	containerChainMap := make(map[string]struct{})

	for id := range strings.SplitSeq(chainLabelValue, ",") {
		if id = strings.TrimSpace(id); id != "" {
			containerChainMap[id] = struct{}{}
		}
	}

	// Filter containers that are in the chain, present on the host, and not the effective current.
	// Chained containers are parent containers that must be removed regardless of scope.
	for _, c := range allContainers {
		_, exists := containerChainMap[string(c.ID())]
		if !exists || c.ID() == effectiveCurrent.ID() {
			continue
		}

		// Only a Watchtower container can be a replaced instance.
		if !c.IsWatchtower() {
			log.Debug().
				Str("container_id", string(c.ID())).
				Str("container_name", c.Name()).
				Msg("Skipping chained container that is not a Watchtower container")

			continue
		}

		chainedContainers = append(chainedContainers, c)
	}

	return chainedContainers
}

// addExcessContainers combines and deduplicates excess and chain containers for removal.
//
// It creates a map to deduplicate containers by ID, adding both excess and chain containers,
// then returns a slice of unique containers to remove.
//
// Parameters:
//   - excessContainers: Containers identified as excess within the scope.
//   - chainContainers: Containers linked in a chain excluding the current one.
//
// Returns:
//   - []types.Container: Deduplicated slice of containers to remove.
func addExcessContainers(excessContainers, chainContainers []types.Container) []types.Container {
	containersToRemoveMap := make(map[types.ContainerID]types.Container)
	for _, c := range excessContainers {
		containersToRemoveMap[c.ID()] = c
	}

	for _, c := range chainContainers {
		containersToRemoveMap[c.ID()] = c
	}

	containersToRemove := make([]types.Container, 0, len(containersToRemoveMap))
	for _, c := range containersToRemoveMap {
		containersToRemove = append(containersToRemove, c)
	}

	return containersToRemove
}

// removeExcessContainers attempts to stop and remove a list of excess containers with retries.
//
// It stops and removes the provided containers, handling retries on failure, tracks removal successes,
// and optionally collects image information for deferred removal if images should be cleaned up.
// Excludes the current running container from removal and manages image cleanup based on removal success.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - excessWatchtowerContainers: Slice of Watchtower containers to stop and remove.
//   - cleanupImages: Remove images if true.
//   - currentContainer: The current running Watchtower container (nil if not applicable).
//   - removeImageInfos: List that receives the images of removed instances for the caller to remove.
//     Nil removes them right away.
//
// Returns:
//   - int: Number of successfully removed containers.
//   - error: Non-nil if any container removal failed or insufficient removals occurred.
func removeExcessContainers(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	excessWatchtowerContainers []types.Container,
	cleanupImages bool,
	currentContainer types.Container,
	removeImageInfos *[]types.RemovedImageInfo,
) (int, error) {
	log.Debug().
		Int("excess_count", len(excessWatchtowerContainers)).
		Bool("cleanup_images", cleanupImages).
		Msg("Starting removal of excess containers")

	// collected holds the images of the instances this call removes.
	var collected []types.RemovedImageInfo

	excessInstancesRemoved := 0

	for i, c := range excessWatchtowerContainers {
		log.Debug().
			Str("container_id", string(c.ID())).
			Str("container_name", c.Name()).
			Msg("Starting removal attempts for excess container")

		succeeded := false
		wasNotFound := false

		for attempt := range maxRemovalAttempts {
			log.Debug().
				Str("container_id", string(c.ID())).
				Int("attempt", attempt+1).
				Int("max_attempts", maxRemovalAttempts).
				Msg("Attempting to stop and remove container")

			err := client.StopAndRemoveContainer(ctx, c, stopContainerTimeout)
			if err == nil {
				log.Debug().
					Str("container_id", string(c.ID())).
					Int("attempt", attempt+1).
					Msg("Successfully stopped and removed container")

				succeeded = true

				break
			}

			if cerrdefs.IsNotFound(err) {
				log.Debug().
					Str("container_id", string(c.ID())).
					Int("attempt", attempt+1).
					Msg("Container not found, considering as removed")

				succeeded = true
				wasNotFound = true

				break
			}

			log.Debug().
				Err(err).
				Str("container_id", string(c.ID())).
				Int("attempt", attempt+1).
				Msg("Failed to stop and remove container")

			if attempt < maxRemovalAttempts-1 {
				select {
				case <-time.After(removalRetryDelay):
					// continue to next retry attempt
				case <-ctx.Done():
					// The caller still removes the images of the instances
					// already removed, except images the remaining ones use.
					if removeImageInfos != nil {
						addCollectedImages(removeImageInfos, collected, excessWatchtowerContainers[i:])
					}

					return excessInstancesRemoved, fmt.Errorf("context canceled during retry delay: %w", ctx.Err())
				}
			}
		}

		if succeeded {
			excessInstancesRemoved++

			if cleanupImages && currentContainer != nil &&
				c.ImageID() != currentContainer.ImageID() &&
				!wasNotFound {
				log.Debug().
					Str("container_id", string(c.ID())).
					Str("image_id", string(c.ImageID())).
					Str("image_name", c.ImageName()).
					Msg("Collecting image info for deferred removal")

				collected = append(collected, types.RemovedImageInfo{
					ImageID:       c.ImageID(),
					ContainerID:   c.ID(),
					ImageName:     c.ImageName(),
					ContainerName: c.Name(),
				})
			}
		}
	}

	// Keep every image while an instance is still running. Only the images
	// of this call are dropped, never the caller's earlier entries.
	if excessInstancesRemoved < len(excessWatchtowerContainers) {
		collected = nil
	}

	if removeImageInfos != nil {
		// The caller removes the images, once per image, with the rest of
		// its cleanup.
		addCollectedImages(removeImageInfos, collected, nil)
	} else if len(collected) > 0 {
		removedInfos, err := removeImages(log, ctx, client, deduplicateByImageID(collected))
		if err != nil {
			log.Error().
				Err(err).
				Int("removed_images_count", len(removedInfos)).
				Interface("image_infos", removedInfos).
				Bool("cleanup_images", true).
				Msg("failed to remove excess images")
		}
	}

	if excessInstancesRemoved < len(excessWatchtowerContainers) {
		return excessInstancesRemoved, fmt.Errorf(
			"%w: %d of %d instances failed to stop",
			errStopWatchtowerFailed,
			len(excessWatchtowerContainers)-excessInstancesRemoved,
			len(excessWatchtowerContainers),
		)
	}

	log.Info().
		Int("removed_instances", excessInstancesRemoved).
		Msg("Successfully removed all excess Watchtower containers")

	return excessInstancesRemoved, nil
}

// addCollectedImages adds collected images to the caller's cleanup list,
// skipping images that a remaining instance still uses.
//
// Parameters:
//   - removeImageInfos: The caller's cleanup list.
//   - collected: Images of the instances that were removed.
//   - remaining: Instances that were not removed.
func addCollectedImages(
	removeImageInfos *[]types.RemovedImageInfo,
	collected []types.RemovedImageInfo,
	remaining []types.Container,
) {
	inUse := make(map[types.ImageID]struct{}, len(remaining))
	for _, c := range remaining {
		inUse[c.ImageID()] = struct{}{}
	}

	for _, info := range collected {
		if _, used := inUse[info.ImageID]; used {
			continue
		}

		addCleanupImageInfo(removeImageInfos, info.ImageID, info.ImageName, info.ContainerName, info.ContainerID)
	}
}

// removeImages removes specified images and returns successfully removed ones.
//
// It iterates through the provided images, attempting to remove each from the Docker environment,
// logging successes or failures for debugging and monitoring. Tracks successfully removed image info.
// If no images are provided, it returns an empty slice and no error.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - images: Slice of images to remove.
//
// Returns:
//   - []RemovedImageInfo: Slice of successfully removed image info.
//   - error: Non-nil if any image removal failed, nil otherwise.
func removeImages(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	images []types.RemovedImageInfo,
) ([]types.RemovedImageInfo, error) {
	// Return early if no images need removal.
	if len(images) == 0 {
		log.Debug().Msg("No images provided for removal, skipping")

		return []types.RemovedImageInfo{}, nil
	}

	removed := []types.RemovedImageInfo{}

	var removalErrors []error

	for _, image := range images {
		imageID := image.ImageID
		if imageID == "" {
			continue // Skip empty IDs to avoid invalid operations.
		}

		log.Debug().
			Str("image_id", string(imageID)).
			Str("image_name", image.ImageName).
			Str("container_id", string(image.ContainerID)).
			Msg("Attempting to remove image")

		err := client.RemoveImageByID(ctx, imageID, image.ImageName)
		if err != nil {
			// Check if this is a "not found" error (expected when multiple instances remove the same image)
			switch {
			case cerrdefs.IsNotFound(err):
				log.Debug().
					Str("image_id", string(imageID)).
					Str("image_name", image.ImageName).
					Msg("Image already removed")
			case cerrdefs.IsConflict(err) || errors.Is(err, container.ErrImageInUse):
				log.Debug().
					Str("image_id", string(imageID)).
					Str("image_name", image.ImageName).
					Msg("Image is in use by active container, skipping removal")
			case ctx.Err() != nil,
				errors.Is(err, context.Canceled),
				errors.Is(err, context.DeadlineExceeded):
				log.Debug().
					Err(err).
					Str("image_id", string(imageID)).
					Str("image_name", image.ImageName).
					Msg("Image removal interrupted by cancellation, skipping")
			default:
				log.Debug().
					Err(err).
					Str("image_id", string(imageID)).
					Str("image_name", image.ImageName).
					Msg("Failed to remove image")
				removalErrors = append(
					removalErrors,
					fmt.Errorf("failed to remove image %s: %w", imageID, err),
				)
			}
		} else {
			log.Debug().
				Str("image_id", imageID.ShortID()).
				Str("image_name", image.ImageName).
				Msg("Removed old image")
			removed = append(
				removed,
				types.RemovedImageInfo{
					ImageID:       imageID,
					ContainerID:   image.ContainerID,
					ImageName:     image.ImageName,
					ContainerName: image.ContainerName,
				},
			)
		}
	}

	if len(removalErrors) > 0 {
		return removed, fmt.Errorf(
			"%w: %d of %d image removals failed",
			errImageRemovalFailed,
			len(removalErrors),
			len(images),
		)
	}

	return removed, nil
}

// containerNames extracts names from a container list.
//
// It creates a slice of container names for logging or debugging purposes, preserving order.
//
// Parameters:
//   - containers: List of containers.
//
// Returns:
//   - []string: List of container names.
func containerNames(containers []types.Container) []string {
	names := make([]string, len(containers))
	for i, c := range containers {
		names[i] = c.Name()
	}

	return names
}
