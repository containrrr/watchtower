package actions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/release"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/lifecycle"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// defaultPullFailureDelay defines the default delay after a failed Watchtower self-update pull in run-once mode.
	defaultPullFailureDelay = 5 * time.Minute

	// defaultHealthCheckTimeout defines the default timeout for waiting for container health checks.
	defaultHealthCheckTimeout = 5 * time.Minute

	// defaultRestartPolicyTimeout defines the fallback timeout for restart policy
	// updates when config.Timeout is non-positive.
	defaultRestartPolicyTimeout = 30 * time.Second

	// defaultCreateStartTimeout defines the minimum timeout for container create
	// and start operations during restart. It guards against slow hosts where
	// Docker API calls can exceed the default 30s restart policy timeout.
	defaultCreateStartTimeout = 5 * time.Minute
)

// isRecoverableOrphan determines whether a container is a recoverable orphaned
// Watchtower instance. It excludes the current container, old-container names,
// non-Watchtower containers, containers that are not in the created state, and
// containers from a different scope.
//
// Parameters:
//   - c: Candidate container to evaluate.
//   - currentContainer: The current running container to exclude.
//   - currentScope: Normalized scope of the current container ("none" when unset).
//
// Returns:
//   - bool: True if the candidate is a recoverable orphan, false otherwise.
func isRecoverableOrphan(
	c types.Container,
	currentContainer types.Container,
	currentScope string,
) bool {
	if !c.IsWatchtower() {
		return false
	}

	if c.ID() == currentContainer.ID() {
		return false
	}

	if container.IsOldContainer(c.Name()) {
		return false
	}

	if !c.IsCreated() {
		return false
	}

	candidateScope, candidateHasScope := c.Scope()
	if !candidateHasScope || candidateScope == "" {
		candidateScope = "none"
	}

	if candidateScope != currentScope {
		return false
	}

	return true
}

// TryRecoverOrphanedContainer attempts to start an orphaned Watchtower container
// that is stuck in the Docker "created" state. This can happen when a self-update
// creates a replacement container but fails to start it, leaving the old container
// running and the new container never started.
//
// It lists all containers, filters for Watchtower containers in the created state
// that are not the current container, and attempts to start the first match.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker API operations.
//   - currentContainer: The current running container to exclude from recovery.
//
// Returns:
//   - types.Container: The recovered container if successful, nil otherwise.
//   - bool: True if a container was found and started, false otherwise.
func TryRecoverOrphanedContainer(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	currentContainer types.Container,
) (types.Container, bool) {
	if currentContainer == nil {
		return nil, false
	}

	allContainers, err := client.ListContainers(ctx, filters.NoFilter)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to list containers for orphaned Watchtower recovery")

		return nil, false
	}

	currentScope, currentHasScope := currentContainer.Scope()
	if !currentHasScope || currentScope == "" {
		currentScope = "none"
	}

	for _, c := range allContainers {
		if !isRecoverableOrphan(c, currentContainer, currentScope) {
			continue
		}

		log.Info().
			Str("container_id", string(c.ID())).
			Str("container_name", c.Name()).
			Msg("Attempting to recover orphaned Watchtower container")

		err := client.StartContainerByID(ctx, c.ID())
		if err != nil {
			log.Debug().
				Err(err).
				Str("container_id", string(c.ID())).
				Str("container_name", c.Name()).
				Msg("Failed to start orphaned Watchtower container")

			continue
		}

		log.Info().
			Str("container_id", string(c.ID())).
			Str("container_name", c.Name()).
			Msg("Successfully recovered orphaned Watchtower container")

		return c, true
	}

	return nil, false
}

// Update scans and updates containers based on parameters.
//
// It checks container staleness, sorts by dependencies, and updates or restarts
// containers as needed, collecting cleaned image info for cleanup.
// Non-stale linked containers are restarted but not marked as updated.
// Containers with pinned images (referenced by digest) are skipped to
// preserve immutability.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for interacting with Docker API.
//   - config: UpdateParams specifying behavior like cleanup, restart, and filtering.
//   - gitClient: Git monitor client. Nil when Git monitoring is unused.
//
// Returns:
//   - types.Report: Session report summarizing scanned, updated, and failed containers.
//   - []types.RemovedImageInfo: Slice of cleaned image info to clean up after updates.
//   - error: Non-nil if listing or sorting fails, nil on success.
func Update(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	config types.UpdateParams,
	gitClient ...*git.Client,
) (types.Report, []types.RemovedImageInfo, error) {
	// Check for context cancellation early
	select {
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("update canceled: %w", ctx.Err())
	default:
	}

	var watcher *git.Client
	if len(gitClient) > 0 {
		watcher = gitClient[0]
	}

	gitSession := newGitSession(watcher)

	// Initialize logging for the update process start.
	log.Debug().Msg("Starting container update check")

	err := checkImageDiskSpace(log, ctx, client, config)
	if err != nil {
		return nil, nil, err
	}

	// Fetch all containers for monitoring
	allContainers, err := client.ListContainers(
		ctx,
		filters.NoFilter,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Initialize a slice to collect cleaned image info for cleanup after updates.
	cleanupImageInfos := []types.RemovedImageInfo{}

	// Self-check: if the current container is an old Watchtower container,
	// update its restart policy to "no" and exit to prevent Docker from
	// restarting it. This catches cases where the startup check was bypassed
	// or the container was restarted despite other safeguards.
	if config.CurrentContainerID != "" {
		for _, c := range allContainers {
			if c.ID() == config.CurrentContainerID && c.IsWatchtower() {
				if container.IsOldContainer(c.Name()) {
					log.Debug().
						Str("container", c.Name()).
						Msg("Current container is an old Watchtower container, stopping self")

					recoverCtx, recoverCancel := context.WithTimeout(
						context.Background(),
						restartPolicyTimeout(config.Timeout),
					)
					defer recoverCancel()

					//nolint:contextcheck // recoverCtx is intentionally created with timeout and passed to recovery
					recoveredContainer, recovered := TryRecoverOrphanedContainer(log,
						recoverCtx,
						client,
						c,
					)
					if recovered {
						log.Info().
							Str("container", recoveredContainer.Name()).
							Msg("Recovered orphaned Watchtower container, exiting old instance")
					}

					setNoRestartCtx, setNoRestartCancel := context.WithTimeout(
						context.Background(),
						restartPolicyTimeout(config.Timeout),
					)
					defer setNoRestartCancel()

					//nolint:contextcheck // setNoRestartCtx is intentionally used for restart policy update
					client.SetRestartPolicy(
						setNoRestartCtx,
						c,
						dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
					)

					return nil, nil, errOldSelfDetected
				}

				break
			}
		}
	}

	// Clean up any old Watchtower containers that linger from a previous
	// self-update. This runs each update cycle to catch containers that the
	// startup cleanup may have missed (e.g., they were still stopping at startup).
	// Scope is derived from the current container to avoid crossing scope boundaries.
	// When the current container isn't found in the list, cleanup is
	// skipped to prevent accidentally removing old containers from other
	// scoped instances. Unscoped containers ("") are normalized to "none".
	if config.CurrentContainerID != "" {
		currentScope, found := deriveScopeFromCurrentContainer(log,
			allContainers,
			config.CurrentContainerID,
		)

		if !found {
			log.Debug().Msg("Skipping old container cleanup: current container not found in list")
		} else {
			if currentScope == "" {
				currentScope = "none"
			}

			_, cleanupErr := CleanupOldWatchtowerContainers(log,
				ctx,
				client,
				config.Cleanup,
				currentScope,
				config.CurrentContainerID,
				&cleanupImageInfos,
			)
			if cleanupErr != nil {
				log.Warn().
					Err(cleanupErr).
					Msg("Failed to clean up old Watchtower containers, continuing update cycle")
			}
		}
	}

	// Create a progress tracker for reporting scanned, updated, and skipped containers.
	progress := &session.Progress{}
	// Track the number of stale containers for logging.
	var staleCount int
	// Track if Watchtower self-update pull failed to add safeguard delay.
	var watchtowerPullFailed bool

	// Filter containers based on the provided filter (e.g., all, specific names).
	// Use a non-nil empty slice so lifecycle hooks can distinguish a valid empty
	// snapshot from an uninitialized list.
	filteredContainers := make([]types.Container, 0, len(allContainers))

	for _, c := range allContainers {
		if config.Filter == nil || config.Filter(c) {
			filteredContainers = append(filteredContainers, c)
		}
	}

	// Run pre-check lifecycle hooks if enabled to validate the environment before updates.
	if config.LifecycleHooks {
		log.Debug().Msg("Executing pre-check lifecycle hooks")
		lifecycle.ExecutePreChecks(log, ctx, client, config, filteredContainers)
	}

	// Prepare a list of container names and images for detailed debugging output.
	filteredContainerNames := make([]string, len(filteredContainers))
	for i, c := range filteredContainers {
		filteredContainerNames[i] = fmt.Sprintf(
			"%s (%s)",
			c.Name(),
			c.ImageName(),
		)
	}
	// Log the retrieved containers and filter details.
	log.Debug().
		Int("count", len(filteredContainers)).
		Strs("containers", filteredContainerNames).
		Str("filter", fmt.Sprintf("%T", config.Filter)).
		Msg("Retrieved containers for update check")

	// Skip monitored containers that reference themselves as dependencies
	// via the Watchtower depends-on label.
	for _, monitoredContainer := range filteredContainers {
		if hasSelfDependency(log, monitoredContainer) {
			progress.AddSkipped(log,
				monitoredContainer,
				errSelfDependency,
				config,
			)
			log.Warn().
				Str("container", monitoredContainer.Name()).
				Str("id", monitoredContainer.ID().ShortID()).
				Msg("Skipping container update (self-dependency)")
		}
	}

	// Build the dependency graph once. The cycle check, every dependency sort,
	// and the implicit restarts below use it, so each link resolves to the same
	// container throughout the scan. A graph that cannot be built fails the
	// first sort. Unmonitored containers are passed so that a link naming one
	// resolves to it, and is left alone, rather than to a monitored container.
	graph, graphErr := sorter.NewDependencyGraph(log,
		filteredContainers,
		unmonitoredContainers(allContainers, filteredContainers),
		config.UseComposeDependsOn,
	)
	if graphErr == nil {
		for _, link := range graph.UnresolvedDependsOn() {
			log.Warn().
				Str("container", link.Container.Name()).
				Str("depends_on", link.Link).
				Msg("Ignoring depends-on entry that names no container")
		}
	}

	// Skip every container in a circular dependency. Cycle members are also
	// kept out of the dependency sorts and implicit restarts below, so a cycle
	// cannot fail the session or be restarted through a link to an updated
	// container.
	inCycle := make(map[types.ContainerID]struct{})

	var cycleMembers []types.Container
	if graphErr == nil {
		cycleMembers = graph.CycleMembers()
	}

	for _, c := range cycleMembers {
		inCycle[c.ID()] = struct{}{}

		progress.AddSkipped(log, c, errCircularDependency, config)
		log.Warn().
			Str("container", c.Name()).
			Str("id", c.ID().ShortID()).
			Msg("Skipping container update (circular dependency)")
	}

	// Track containers that fail staleness checks for reporting.
	staleCheckFailed := 0

	// Prepare containers for staleness checks, skipping already-processed ones.
	type checkTask struct {
		index     int
		container types.Container
	}

	var checkTasks []checkTask

	// Iterate through containers to check staleness and prepare for updates or restarts.
	for i, sourceContainer := range filteredContainers {
		// Check for context cancellation to enable faster shutdown during long update cycles.
		select {
		case <-ctx.Done():
			return progress.Report(log), cleanupImageInfos, ctx.Err()
		default:
		}

		// Skip containers already processed (e.g., skipped due to circular dependencies).
		_, exists := (*progress)[sourceContainer.ID()]
		if exists {
			continue
		}

		// Set up logging fields for the current container.
		clogVal := log.With().
			Str("container", sourceContainer.Name()).
			Str("image", sourceContainer.ImageName()).
			Logger()
		clog := &clogVal

		// Check if the container uses a pinned (digest-based) image to skip updates.
		isPinnedVal, err := isPinned(log, sourceContainer, progress, config, gitSession)
		if err != nil {
			// Log and skip containers with unparsable image references, marking as skipped.
			clog.Debug().
				Err(err).
				Msg("Failed to check pinned image - skipping container")

			progress.AddSkipped(log,
				sourceContainer,
				fmt.Errorf("%w: %w", errParseImageReference, err),
				config,
			)

			staleCheckFailed++

			continue
		}

		if isPinnedVal {
			// Skip staleness checks for pinned images and mark as scanned.
			clog.Debug().Msg("Skipping staleness check for pinned image")

			continue
		}

		checkTasks = append(checkTasks, checkTask{
			index:     i,
			container: sourceContainer,
		})
	}

	// Check for context cancellation before launching parallel staleness checks.
	select {
	case <-ctx.Done():
		return progress.Report(log), cleanupImageInfos, ctx.Err()
	default:
	}

	// Parallelize staleness checks with bounded concurrency.
	const maxConcurrentChecks = 20

	var checkGroup errgroup.Group
	checkGroup.SetLimit(maxConcurrentChecks)

	var (
		resultMu                     sync.Mutex
		parallelStaleCheckFailed     int
		parallelWatchtowerPullFailed bool
		parallelStaleCount           int
	)

	// One resolver per session so sibling containers on the same upstream image
	// share a single release tag lookup.
	tagResolver := release.NewResolver(nil)

	for _, task := range checkTasks {
		checkGroup.Go(func() error {
			// Check for context cancellation to enable faster shutdown during long update cycles.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			sourceContainer := task.container
			clogVal := log.With().
				Str("container", sourceContainer.Name()).
				Str("image", sourceContainer.ImageName()).
				Logger()
			clog := &clogVal

			var (
				stale       bool
				newestImage types.ImageID
				newDigest   string
				gitWatched  bool
				checkErr    error
				verifyErr   error
			)

			// Determine if the container is stale and needs updating.
			// If the container is Watchtower and SkipSelfUpdate is enabled, skip the update
			// by setting stale to false and using the current image. Otherwise, check staleness.
			switch {
			case sourceContainer.IsWatchtower() && config.SkipSelfUpdate:
				stale = false
				newestImage = sourceContainer.ImageID()
			case gitSession.watching(log, sourceContainer, config):
				gitWatched = true

				stale, newestImage, checkErr = gitSession.check(ctx, sourceContainer, config)
			default:
				stale, newestImage, newDigest, checkErr = client.IsContainerStale(
					ctx,
					sourceContainer,
					config,
				)
			}

			if checkErr != nil && (errors.Is(checkErr, context.Canceled) || errors.Is(checkErr, context.DeadlineExceeded)) {
				return fmt.Errorf("staleness check canceled: %w", checkErr)
			}

			// Determine if the container should be updated based on staleness and config.
			shouldUpdate := shouldUpdateContainer(sourceContainer, stale, config)
			if gitSession.watching(log, sourceContainer, config) && sourceContainer.IsNoPull(config) {
				shouldUpdate = false
			}

			// Log when skipping Watchtower self-update in run-once mode.
			if stale && sourceContainer.IsWatchtower() && config.RunOnce {
				clog.Info().Msg("Skipping Watchtower self-update in run-once mode")
			}

			// Verify the container's configuration if it's slated for update to
			// ensure recreation is possible.
			if checkErr == nil && shouldUpdate {
				verifyErr = sourceContainer.VerifyConfiguration()
				if verifyErr != nil {
					clog.Debug().
						Err(verifyErr).
						Str("container_id", sourceContainer.ID().ShortID()).
						Str("image_id", sourceContainer.ImageID().ShortID()).
						Msg("Failed to verify container configuration")
				}

				if verifyErr != nil &&
					(errors.Is(verifyErr, context.Canceled) ||
						errors.Is(verifyErr, context.DeadlineExceeded)) {
					return fmt.Errorf("configuration verification canceled: %w", verifyErr)
				}
			}

			// Record the new image's OCI metadata so a notification can describe
			// what the container is moving to. This runs only for a container
			// that will actually be updated. The inspect and the optional
			// release tag lookup happen before resultMu is taken, because a
			// registry round trip under that lock would serialize every
			// parallel check behind one container's request.
			// A Git-watched container is excluded because the build path resolves
			// and logs its own changelog from the resolved tag afterwards.
			resolveLatest := stale && shouldUpdate && !gitWatched && checkErr == nil && verifyErr == nil

			var (
				changelogVars container.ChangelogVars
				latestAnns    oci.Annotations
			)

			if resolveLatest {
				latest := latestImageMetadata{
					annotations: client.GetImageAnnotations(ctx, sourceContainer.ImageName()),
					digest:      newDigest,
				}

				// Bound the whole tag lookup, not just its requests, so an
				// unresponsive registry cannot stall the session behind it.
				lookupCtx, cancelLookup := context.WithTimeout(ctx, releaseTagLookupTimeout)
				changelogVars, latestAnns = resolveLatestImageMeta(
					log,
					lookupCtx,
					tagResolver,
					sourceContainer,
					config,
					latest,
				)

				cancelLookup()
			}

			resultMu.Lock()
			defer resultMu.Unlock()

			// Handle staleness check results, logging skips or adding to the progress report.
			switch {
			case checkErr != nil:
				// Skip containers with staleness check errors, marking them as skipped.
				warnGitConfigSkip(log, sourceContainer.Name(), sourceContainer.ImageName(), checkErr)

				if !errors.Is(checkErr, container.ErrImageCooldown) {
					parallelStaleCheckFailed++
				}

				if ratelimit.Is(checkErr) {
					// One line per container per cycle. Log-based notifications
					// carry it, and the report template lists the container under
					// Failed instead. A Retry-After beyond the honor window was
					// already logged at warn by the retry loop.
					if !ratelimit.ExceedsHonorWindow(checkErr) {
						clog.Warn().
							Err(checkErr).
							Msg("Registry rate limit retries exhausted. Container failed for this cycle")
					}

					progress.AddFailed(log, sourceContainer, checkErr, config)
				} else {
					progress.AddSkipped(log, sourceContainer, checkErr, config)
				}

				// Restore rich cooldown metadata for reports/notifications (preserves the
				// structured CooldownAge/Delay/Remaining/Passed fields that the removed
				// high-level block used to populate via SetCooldownInfo). The rich
				// *container.CooldownError carries the details.
				cooldownErr, ok := errors.AsType[*container.CooldownError](checkErr)
				if ok {
					progress.SetCooldownInfo(log,
						sourceContainer.ID(),
						cooldownErr.Age,
						cooldownErr.Delay,
						cooldownErr.Remaining,
						cooldownErr.EligibleAt,
						cooldownErr.Passed,
					)
				} else if errors.Is(checkErr, container.ErrImageCooldown) {
					// Fallback for plain sentinel (keeps basic deferral visible)
					progress.SetCooldownInfo(log, sourceContainer.ID(), "", "", "", time.Time{}, false)
				}

				// Track if Watchtower self-update pull failed for safeguard.
				// Only set to true if we actually attempted a self-update
				// (i.e., SkipSelfUpdate is false) and the error is a real
				// failure, not a cooldown deferral.
				if sourceContainer.IsWatchtower() &&
					!config.SkipSelfUpdate &&
					!errors.Is(checkErr, container.ErrImageCooldown) {
					parallelWatchtowerPullFailed = true
				}
			case verifyErr != nil:
				parallelStaleCheckFailed++

				progress.AddSkipped(log, sourceContainer, verifyErr, config)

				if sourceContainer.IsWatchtower() &&
					!config.SkipSelfUpdate &&
					!errors.Is(verifyErr, container.ErrImageCooldown) {
					parallelWatchtowerPullFailed = true
				}
			default:
				// For fresh containers, set newestImage to current image ID for proper categorization.
				// (Cooldown decision and any layer pull now happen inside pkg/container/image.go
				// IsOutsideCooldown + guarded PullImage, after digest staleness and before layers.)
				if !stale {
					newestImage = sourceContainer.ImageID()
				}

				// Log successful staleness check and add to scanned containers.
				clog.Debug().
					Bool("stale", stale).
					Str("newest_image", string(newestImage)).
					Msg("Checked container staleness")
				progress.AddScanned(log,
					sourceContainer,
					newestImage,
					config,
				)
			}

			// Write the pre-resolved metadata now that the session state is locked.
			if resolveLatest {
				applyLatestImageMeta(
					clog,
					progress,
					sourceContainer,
					config,
					changelogVars,
					latestAnns,
				)
			}

			// Track old image ID before update for cleanup notifications.
			if shouldUpdate {
				c, ok := filteredContainers[task.index].(*container.Container)
				if ok {
					c.SetOldImageID(sourceContainer.ImageID())
				}
			}

			// Update the container's stale status for dependency sorting.
			// Only mark as stale if the container should actually be updated.
			filteredContainers[task.index].SetStale(stale && shouldUpdate && checkErr == nil && verifyErr == nil)

			// Increment stale count for logging summary.
			if stale {
				parallelStaleCount++
			}

			return nil
		})
	}

	err = checkGroup.Wait()
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Parallel staleness checks completed with error")

		// Surface context cancellation from parallel workers as the final Update error.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return progress.Report(log), cleanupImageInfos, fmt.Errorf("update canceled: %w", err)
		}
	}

	// Accumulate parallel results with pre-parallel sequential counts so failures
	// from pinned-image parsing and other pre-filter checks are preserved.
	staleCheckFailed += parallelStaleCheckFailed
	watchtowerPullFailed = watchtowerPullFailed || parallelWatchtowerPullFailed
	staleCount += parallelStaleCount

	// Log the summary of staleness checks, including total, stale, and failed counts.
	log.Debug().
		Int("total", len(filteredContainers)).
		Int("stale", staleCount).
		Int("failed", staleCheckFailed).
		Msg("Completed container staleness check")

	// Build a map for a lookup of containers by ID.
	containerByID := make(map[types.ContainerID]types.Container, len(allContainers))
	for _, ac := range allContainers {
		containerByID[ac.ID()] = ac
	}

	// Propagate stale status to allContainers since they are different instances.
	for _, c := range filteredContainers {
		if c.IsStale() {
			ac, ok := containerByID[c.ID()]
			if ok {
				ac.SetStale(true)
			}
		}
	}

	// Sort containers by dependencies to ensure correct update and restart order.
	// Cycle members cannot be ordered, so they are placed after the others.
	// Every sort keeps only the graph's dependencies between the containers it
	// sorts, so leaving a container out cannot redirect a link to a different
	// container and form a cycle that the graph does not have.
	sortable := make([]types.Container, 0, len(filteredContainers))
	unsortable := make([]types.Container, 0, len(inCycle))

	for _, c := range filteredContainers {
		if _, skip := inCycle[c.ID()]; skip {
			unsortable = append(unsortable, c)
		} else {
			sortable = append(sortable, c)
		}
	}

	err = graphErr
	if err == nil {
		err = graph.Sort(log, sortable)
	}

	if err != nil {
		// Log and return an error if dependency sorting fails.
		log.Debug().
			Err(err).
			Msg("Failed to sort containers by dependencies")

		return nil, cleanupImageInfos, fmt.Errorf(
			"%w: %w",
			errSortDependenciesFailed,
			err,
		)
	}

	// Write the order back, with cycle members last.
	copy(filteredContainers, sortable)
	copy(filteredContainers[len(sortable):], unsortable)

	// Collect all containers to restart: updates, and containers linked to
	// restarting ones, which restart without updating.
	allContainersToRestart := reconcileImplicitRestartsExcluding(log,
		graph,
		allContainers,
		filteredContainers,
		config,
		inCycle,
	)

	// Sort containers to restart by dependencies to ensure correct update and restart order.
	err = graph.Sort(log, allContainersToRestart)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to sort all containers to restart by dependencies")

		return nil, cleanupImageInfos, fmt.Errorf(
			"%w: %w",
			errSortDependenciesFailed,
			err,
		)
	}

	// Log the number of containers prepared for restart.
	log.Debug().
		Int("restart_count", len(allContainersToRestart)).
		Msg("Prepared containers for restart")

	// Perform updates and restarts, either with rolling restarts or in batches.
	var (
		failedStop    map[types.ContainerID]error
		stoppedImages []types.RemovedImageInfo
		failedStart   map[types.ContainerID]error
	)

	gitFailed := make(map[types.ContainerID]error)
	gitSession.prepareRebuilds(
		log,
		ctx,
		client,
		allContainersToRestart,
		config,
		progress,
		gitFailed,
	)
	progress.UpdateFailed(log, gitFailed)

	// A failed Git apply cleared Stale. Recompute linked restarts so those
	// dependencies are not stopped. Containers that are still stale stay anchors.
	// Cycle members stay excluded, as in the first pass.
	excluded := make(map[types.ContainerID]struct{}, len(inCycle))
	maps.Copy(excluded, gitSession.noRestartBuiltIDs(config))
	maps.Copy(excluded, inCycle)

	allContainersToRestart = reconcileImplicitRestartsExcluding(
		log,
		graph,
		allContainers,
		filteredContainers,
		config,
		excluded,
	)

	err = graph.Sort(log, allContainersToRestart)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to sort all containers to restart by dependencies")

		return nil, cleanupImageInfos, fmt.Errorf(
			"%w: %w",
			errSortDependenciesFailed,
			err,
		)
	}

	// Compose apply already recreated those services. Do not inspect-recreate.
	allContainersToRestart = gitSession.excludeApplied(allContainersToRestart)

	// Git no-restart builds must not enter stop/create.
	// The default non-rolling path stops first.
	// skipRecreate after stop would leave the running container deleted.
	allContainersToRestart = gitSession.excludeNoRestart(
		allContainersToRestart,
		config,
	)

	if config.RollingRestart {
		// Apply rolling restarts for all containers in dependency order.
		rollingFailed, rollingErr := performRollingRestart(
			log,
			ctx,
			allContainersToRestart,
			client,
			config,
			&cleanupImageInfos,
			progress,
			gitSession,
		)
		recordRestartOutcomes(log, progress, rollingFailed)

		if rollingErr != nil {
			return progress.Report(log), cleanupImageInfos, rollingErr
		}
	} else {
		// Mark containers to update for update in progress
		for _, c := range allContainersToRestart {
			if c.IsStale() {
				progress.MarkForUpdate(log, c.ID())
			}
		}

		// Stop and restart containers in batches, respecting dependency order.
		failedStop, stoppedImages = stopContainersInReversedOrder(
			log,
			ctx,
			allContainersToRestart,
			client,
			config,
		)
		recordRestartOutcomes(log, progress, failedStop)

		failedStart = restartContainersInSortedOrder(
			log,
			ctx,
			allContainersToRestart,
			client,
			config,
			stoppedImages,
			&cleanupImageInfos,
			progress,
			gitSession,
		)
		recordRestartOutcomes(log, progress, failedStart)
	}

	// Run post-check lifecycle hooks if enabled to finalize the update process.
	// Passing nil lists the containers again, so the hooks run in the
	// replacements of updated containers rather than in the removed originals
	// held by the pre-update scan.
	if config.LifecycleHooks {
		log.Debug().Msg("Executing post-check lifecycle hooks")
		lifecycle.ExecutePostChecks(log, ctx, client, config, nil)
	}

	// A run-once Watchtower exits after the update, and a restart policy can
	// start it again at once. Delay the exit after a failed self-update pull
	// to prevent rapid restarts. A continuous Watchtower keeps running, and
	// its schedule spaces the next attempt.
	if watchtowerPullFailed && config.RunOnce {
		delay := config.PullFailureDelay
		if delay == 0 {
			delay = defaultPullFailureDelay // Default delay
		}

		log.Info().
			Dur("delay", delay).
			Msg("Watchtower self-update pull failed - sleeping to prevent rapid restarts")

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			log.Debug().
				Err(ctx.Err()).
				Msg("Context canceled during pull-failure delay - skipping remaining delay")
		}
	}

	// Return the final report summarizing the session and the cleanup image infos.
	return progress.Report(log), cleanupImageInfos, nil
}

// hasSelfDependency checks if a container has a self-dependency in its Watchtower depends-on label.
// It now uses the shared GetLinksFromWatchtowerLabel helper function for parsing and normalization.
func hasSelfDependency(log *zerolog.Logger, c types.Container) bool {
	sourceContainer, ok := c.(*container.Container)
	if !ok {
		return false
	}

	linkLog := log.With().Str("container", c.Name()).Logger()
	links := container.GetLinksFromWatchtowerLabel(
		sourceContainer,
		&linkLog,
	)

	return slices.Contains(links, c.Name())
}

// noRestartBuiltIDs returns a snapshot of container IDs built during a no-restart update.
//
// Parameters:
//   - params: Update parameters.
//
// Returns:
//   - map[types.ContainerID]struct{}: Built container IDs, or nil when restart is enabled.
func (s *gitSession) noRestartBuiltIDs(params types.UpdateParams) map[types.ContainerID]struct{} {
	if s == nil || !params.NoRestart {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[types.ContainerID]struct{}, len(s.built))
	for id := range s.built {
		ids[id] = struct{}{}
	}

	return ids
}

// reconcileImplicitRestartsExcluding clears linked-restart marks and derives restart candidates while omitting excluded containers.
//
// Parameters:
//   - log: Process logger.
//   - graph: The scan's dependency graph.
//   - allContainers: Full list of containers being managed.
//   - containers: Containers eligible for this session.
//   - params: Update parameters.
//   - excluded: Container IDs that must not propagate or receive implicit restarts.
//
// Returns:
//   - []types.Container: Containers that should still restart.
func reconcileImplicitRestartsExcluding(
	log *zerolog.Logger,
	graph *sorter.DependencyGraph,
	allContainers, containers []types.Container,
	params types.UpdateParams,
	excluded map[types.ContainerID]struct{},
) []types.Container {
	for _, c := range containers {
		c.SetLinkedToRestarting(false)
	}

	markImplicitRestarts(log, graph, allContainers, containers, excluded)

	restart := make([]types.Container, 0, len(containers))
	for _, c := range containers {
		if _, skip := excluded[c.ID()]; skip {
			continue
		}

		if c.ToRestart() && !c.IsMonitorOnly(params) {
			restart = append(restart, c)
		}
	}

	return restart
}

// shouldUpdateContainer determines if a container should be updated
// based on its staleness and update parameters.
//
// It checks multiple conditions:
//   - The container must be stale (outdated image)
//   - The container must not be monitor-only
//   - Watchtower containers are skipped in run-once mode
//   - Watchtower self-updates are skipped if SkipSelfUpdate is true
//
// Parameters:
//   - container: The container to check.
//   - stale: Whether the container's image is outdated.
//   - config: Update parameters controlling update behavior.
//
// Returns:
//   - bool: True if the container should be updated, false otherwise.
func shouldUpdateContainer(
	container types.Container,
	stale bool,
	config types.UpdateParams,
) bool {
	// Must be stale to update
	if !stale {
		return false
	}

	// Skip old Watchtower containers — they are predecessors from a
	// self-update and should only be removed, never updated.
	if filters.IsOldWatchtower(container) {
		return false
	}

	// Skip monitor-only containers
	if container.IsMonitorOnly(config) {
		return false
	}

	// Skip Watchtower self-update in run-once mode
	if config.RunOnce && container.IsWatchtower() {
		return false
	}

	// Skip Watchtower self-update if SkipSelfUpdate is true
	if config.SkipSelfUpdate && container.IsWatchtower() {
		return false
	}

	// Skip other Watchtower containers from self-updates
	if container.IsWatchtower() && config.CurrentContainerID != "" &&
		container.ID() != config.CurrentContainerID {
		return false
	}

	return true
}

// logChangelog emits the Changelog notification entry for an updated container.
//
// The entry is only produced when the changelog feature is enabled for the
// container and a URL was resolved, so a default session produces no entry at
// all. The legacy template renders it as a single "Changelog: <url>" line.
//
// Parameters:
//   - log: Process logger.
//   - c: Container the entry describes.
//   - params: Update parameters carrying the changelog enable gate.
//   - meta: Resolved report metadata.
//
// Returns:
//   - none.
func logChangelog(
	log *zerolog.Logger,
	c types.Container,
	params types.UpdateParams,
	meta container.ReportMeta,
) {
	if c == nil || meta.Changelog == "" {
		return
	}

	concrete, ok := c.(*container.Container)
	if !ok || !concrete.IsChangelogEnabled(params) {
		return
	}

	log.Info().
		Str("container", c.Name()).
		Str("changelog", meta.Changelog).
		Msg("Changelog")
}

// parseReference validates a Docker image reference with logging.
//
// Parameters:
//   - imageName: Image name to parse.
//   - configImage: Config.Image value for log context.
//   - fallbackImage: Fallback image name for log context.
//   - cont: Container being processed (for log fields).
//
// Returns:
//   - error: Non-nil if the image name cannot be parsed as a Docker reference.
func parseReference(log *zerolog.Logger, imageName, configImage, fallbackImage string,
	cont types.Container,
) error {
	// Set up logging with container and image details.
	clogVal := log.With().
		Str("container", cont.Name()).
		Str("image", imageName).
		Logger()
	clog := &clogVal

	// Parse the image reference using the Docker reference library.
	normalizedRef, err := reference.ParseDockerRef(imageName)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("image_name", imageName).
			Msg("Failed to parse image reference")

		return fmt.Errorf(
			"failed to parse image reference %s: %w",
			imageName,
			err,
		)
	}

	// Log successful parsing with reference type and context.
	clog.Debug().
		Str("image_name", imageName).
		Str("config_image", configImage).
		Str("fallback_image", fallbackImage).
		Str("ref_type", fmt.Sprintf("%T", normalizedRef)).
		Msg("Parsed image reference")

	return nil
}

// isPinned checks if a container's image is pinned by a digest reference.
//
// It resolves a usable image name from ImageName(), Config.Image, or a fallback,
// then delegates pin detection to container.IsImagePinnedByDigest. Pin detection
// runs before parse-fallback so a digest reference is not replaced by a non-pinned
// fallback when ParseDockerRef fails. If pinned, it marks the container as scanned.
//
// Parameters:
//   - cont: The container to check for a pinned image.
//   - progress: The progress tracker to update for scanned or skipped containers.
//   - params: Update parameters for monitor-only check.
//
// Returns:
//   - bool: True if the image is pinned by digest, false otherwise.
//   - error: Non-nil if no valid image reference can be resolved, nil on success.
func isPinned(
	log *zerolog.Logger,
	cont types.Container,
	progress *session.Progress,
	config types.UpdateParams,
	gitSession *gitSession,
) (bool, error) {
	// Set up logging with container and image details for debugging.
	clogVal := log.With().
		Str("container", cont.Name()).
		Str("image", cont.ImageName()).
		Logger()
	clog := &clogVal

	// Get initial image name and configuration.
	imageName := cont.ImageName()

	var configImage string
	if info := cont.ContainerInfo(); info != nil && info.Config != nil {
		configImage = info.Config.Image
	}

	fallbackImage := getFallbackImage(cont)

	// Check if ImageName is invalid and fall back to Config.Image or a derived name.
	if isInvalidImageName(imageName) {
		clog.Debug().
			Str("invalid_image", imageName).
			Msg("Invalid ImageName detected")

		if configImage != "" && !isInvalidImageName(configImage) {
			imageName = configImage
			clog.Debug().
				Str("config_image", configImage).
				Msg("Using Config.Image as fallback")
		} else {
			imageName = fallbackImage
			clog.Debug().
				Str("fallback_image", fallbackImage).
				Msg("Using derived fallback image")
		}
	}

	// If the final imageName is still invalid, skip the container.
	if isInvalidImageName(imageName) {
		return false, errInvalidImageReference
	}

	// Detect digests before parse-fallback.
	// A repo@sha256 reference must stay pinned even when ParseDockerRef fails
	// for unrelated reasons.
	if container.IsImagePinnedByDigest(imageName) {
		if gitSession.watching(log, cont, config) {
			clog.Debug().Msg("Pinned digest uses Git path because watcher is on")

			return false, nil
		}

		clog.Debug().
			Bool("is_digested", true).
			Msg("Pinned image detected, marking as scanned")
		progress.AddScanned(log,
			cont,
			cont.ImageID(),
			config,
		)

		return true, nil
	}

	// Non-pinned names must still be parseable. Retry with fallback when needed.
	err := parseReference(log, imageName, configImage, fallbackImage, cont)
	if err != nil {
		if imageName != fallbackImage {
			clog.Debug().Msg("Retrying with fallback image")

			fallbackErr := parseReference(log,
				fallbackImage,
				configImage,
				fallbackImage,
				cont,
			)
			if fallbackErr != nil {
				return false, err
			}

			// Fallback name might itself be digest-pinned (unlikely but consistent).
			if container.IsImagePinnedByDigest(fallbackImage) {
				if gitSession.watching(log, cont, config) {
					return false, nil
				}

				clog.Debug().
					Bool("is_digested", true).
					Msg("Pinned image detected via fallback, marking as scanned")
				progress.AddScanned(log,
					cont,
					cont.ImageID(),
					config,
				)

				return true, nil
			}

			return false, nil
		}

		return false, err
	}

	return false, nil
}

// getFallbackImage derives a fallback image name from container info.
// Uses the container name with ":latest" as the fallback image.
func getFallbackImage(container types.Container) string {
	return container.Name() + ":latest"
}

// isInvalidImageName checks if an image name is invalid.
// Returns true if the name is empty, ":latest", or starts with ":".
func isInvalidImageName(name string) bool {
	return name == "" || name == ":latest" || strings.HasPrefix(name, ":")
}

// performRollingRestart updates containers with rolling restarts.
//
// It processes containers sequentially in forward order, stopping and restarting each as needed,
// collecting cleaned image info for stale containers only to ensure proper cleanup.
// The function checks for context cancellation at the start of each iteration to enable
// prompt exit when the context is canceled.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - containers: List of containers to update or restart.
//   - client: Container client for Docker operations.
//   - config: Update options controlling restart behavior.
//   - cleanupImageInfos: Pointer to slice to collect cleaned image info for deferred cleanup.
//   - progress: Progress tracker to update with new container IDs.
//
// Returns:
//   - map[types.ContainerID]error: Map of container IDs to errors for failed updates.
//   - error: Non-nil if context was canceled, nil otherwise.
func performRollingRestart(
	log *zerolog.Logger,
	ctx context.Context,
	containers []types.Container,
	client container.Client,
	config types.UpdateParams,
	cleanupImageInfos *[]types.RemovedImageInfo,
	progress *session.Progress,
	gitSession *gitSession,
) (map[types.ContainerID]error, error) {
	failed := make(map[types.ContainerID]error, len(containers))

	containerNames := make([]string, len(containers))
	for i, c := range containers {
		containerNames[i] = c.Name()
	}

	log.Debug().
		Strs("processing_order", containerNames).
		Msg("Starting performRollingRestart")

	// Process containers in forward order to respect dependency chains.
	for i := range containers {
		// Check for context cancellation to enable prompt exit when context is canceled.
		select {
		case <-ctx.Done():
			// Handle the current container that was not processed due to cancellation.
			c := containers[i]
			log.Info().
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Str("container_id", c.ID().ShortID()).
				Msg("Skipped container restart due to context cancellation")
			failed[c.ID()] = skipped(fmt.Errorf("restart skipped: %w", ctx.Err()))

			// Handle remaining containers that were not processed due to cancellation.
			for j := i + 1; j < len(containers); j++ {
				untouched := containers[j]
				log.Info().
					Str("container", untouched.Name()).
					Str("image", untouched.ImageName()).
					Str("container_id", untouched.ID().ShortID()).
					Msg("Skipped container restart due to context cancellation")
				failed[untouched.ID()] = skipped(fmt.Errorf("restart skipped: %w", ctx.Err()))
			}

			return failed, fmt.Errorf("rolling restart canceled: %w", ctx.Err())
		default:
		}

		c := containers[i]
		if !c.ToRestart() {
			continue
		}

		if gitSession.skipRecreate(c, config) {
			continue
		}

		fields := map[string]any{
			"container": c.Name(),
			"image":     c.ImageName(),
		}

		log.Debug().
			Fields(fields).
			Msg("Processing container for rolling restart")

		// Mark for update if stale
		if c.IsStale() && progress != nil {
			progress.MarkForUpdate(log, c.ID())
		}

		// Stop the container, handling any errors.
		err := stopStaleContainer(log, ctx, c, client, config)
		if err != nil {
			failed[c.ID()] = err
		} else {
			newContainerID, renamed, err := restartStaleContainer(log,
				ctx,
				c,
				client,
				config,
			)
			if err != nil {
				failed[c.ID()] = err
			} else {
				// Set the new container ID in progress
				if progress != nil {
					status, exists := (*progress)[c.ID()]
					if exists {
						status.SetNewContainerID(newContainerID)
						// Mark as restarted if not stale (not updated)
						if !c.IsStale() {
							progress.MarkRestarted(log, c.ID())
						}
					}
				}

				// Wait for the container to become healthy if it has a health check
				waitErr := client.WaitForContainerHealthy(
					ctx,
					newContainerID,
					defaultHealthCheckTimeout,
				)
				if waitErr != nil {
					log.Warn().
						Err(waitErr).
						Fields(fields).
						Msg("Failed to wait for container to become healthy")

					// Don't fail the update, just log the warning
				}

				if c.IsStale() && !renamed {
					// Only collect cleaned image info for stale containers that were not renamed, as renamed
					// containers (Watchtower self-updates) are cleaned up by CheckForMultipleWatchtowerInstances
					// in the new container.
					addCleanupImageInfo(
						cleanupImageInfos,
						c.ImageID(),
						c.ImageName(),
						c.Name(),
						c.ID(),
					)

					log.Debug().
						Fields(fields).
						Msg("Updated container")
				}
			}
		}
	}

	return failed, nil
}

// stopContainersInReversedOrder stops containers in reverse order.
//
// It stops each container, tracking stopped images and errors, to prepare for restarts while
// respecting dependency order.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - containers: List of containers to stop.
//   - client: Container client for Docker operations.
//   - config: Update options specifying stop timeout and other behaviors.
//
// Returns:
//   - map[types.ContainerID]error: Map of container IDs to errors for failed stops.
//   - []types.RemovedImageInfo: Slice of cleaned image info for stopped containers.
func stopContainersInReversedOrder(
	log *zerolog.Logger,
	ctx context.Context,
	containers []types.Container,
	client container.Client,
	config types.UpdateParams,
) (map[types.ContainerID]error, []types.RemovedImageInfo) {
	failed := make(map[types.ContainerID]error, len(containers))
	stopped := make([]types.RemovedImageInfo, 0, len(containers))

	// Stop containers in reverse order to avoid breaking dependencies.
	for i, v := range slices.Backward(containers) {
		c := v

		// Check for context cancellation to avoid additional work when context is canceled.
		// First, log and track the current container, then iterate remaining containers.
		if ctx.Err() != nil {
			// Handle the current container that was not processed due to cancellation.
			log.Info().
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Str("container_id", c.ID().ShortID()).
				Msg("Skipped container stop due to context cancellation")
			failed[c.ID()] = skipped(fmt.Errorf("stop skipped: %w", ctx.Err()))

			// Handle remaining containers that were not processed due to cancellation.
			for j := i - 1; j >= 0; j-- {
				untouched := containers[j]
				log.Info().
					Str("container", untouched.Name()).
					Str("image", untouched.ImageName()).
					Str("container_id", untouched.ID().ShortID()).
					Msg("Skipped container stop due to context cancellation")
				failed[untouched.ID()] = skipped(fmt.Errorf("stop skipped: %w", ctx.Err()))
			}

			return failed, stopped
		}

		fields := map[string]any{
			"container": c.Name(),
			"image":     c.ImageName(),
		}

		err := stopStaleContainer(log, ctx, c, client, config)
		if err != nil {
			failed[c.ID()] = err
		} else {
			stopped = append(
				stopped,
				types.RemovedImageInfo{
					ImageID:       c.ImageID(),
					ContainerID:   c.ID(),
					ImageName:     c.ImageName(),
					ContainerName: c.Name(),
				},
			)

			log.Debug().
				Fields(fields).
				Msg("Stopped container")
		}
	}

	return failed, stopped
}

// recordRestartOutcomes records the containers whose stop or restart did not
// succeed: as skipped when the update deliberately left them untouched, and as
// failed otherwise. A container that already failed in an earlier phase stays
// failed, even when a later phase skips it.
//
// Parameters:
//   - log: Process logger.
//   - progress: Session progress to update.
//   - outcomes: The error of each container that was not stopped or restarted.
func recordRestartOutcomes(log *zerolog.Logger, progress *session.Progress, outcomes map[types.ContainerID]error) {
	failed := make(map[types.ContainerID]error, len(outcomes))
	skips := make(map[types.ContainerID]error)

	for id, err := range outcomes {
		if isSkip(err) {
			skips[id] = err
		} else {
			failed[id] = err
		}
	}

	progress.UpdateSkipped(log, skips)
	progress.UpdateFailed(log, failed)
}

// stopStaleContainer stops a stale container if eligible.
//
// It skips Watchtower containers or those not marked for restart, runs pre-update hooks if enabled,
// and stops the container with the specified timeout.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - container: Container to stop.
//   - client: Container client for Docker operations.
//   - config: Update options specifying stop timeout and lifecycle hooks.
//
// Returns:
//   - error: Non-nil if stop fails, nil on success or if skipped.
func stopStaleContainer(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	client container.Client,
	config types.UpdateParams,
) error {
	fields := map[string]any{
		"container": container.Name(),
		"image":     container.ImageName(),
	}

	// Skip Watchtower containers to avoid self-interruption.
	if container.IsWatchtower() {
		log.Debug().
			Fields(fields).
			Msg("Skipping Watchtower container")

		return nil
	}

	// Skip containers not marked for restart (e.g., not stale or linked).
	if !container.ToRestart() {
		return nil
	}

	log.Debug().
		Fields(fields).
		Msg("Stopping container for restart")

	// Verify configuration for linked containers to ensure restart compatibility.
	if container.IsLinkedToRestarting() {
		err := container.VerifyConfiguration()
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Msg("Failed to verify container configuration")

			return fmt.Errorf("%w: %w", errVerifyConfigFailed, err)
		}
	}

	// Execute pre-update lifecycle hooks if enabled, checking for skip conditions.
	if config.LifecycleHooks {
		skipUpdate, err := lifecycle.ExecutePreUpdateCommand(log,
			ctx,
			client,
			container,
			config.LifecycleUID,
			config.LifecycleGID,
		)
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Msg("Pre-update command execution failed")

			return fmt.Errorf("%w: %w", errPreUpdateFailed, err)
		}

		if skipUpdate {
			log.Debug().
				Fields(fields).
				Msg("Skipping container due to pre-update exit code 75")

			return skipped(errSkipUpdate)
		}
	}

	err := snapshotCopyFilesForRecreate(ctx, client, container)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to snapshot copy-file paths")

		return err
	}

	// Stop the container with the configured timeout.
	err = client.StopAndRemoveContainer(
		ctx,
		container,
		config.Timeout,
	)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			discardCopyFilesForRecreate(client, container.ID())
		}

		// Check if the container is already gone (e.g., "No such container" error).
		// Treat this as non-fatal, similar to RemoveExcessWatchtowerInstances.
		if cerrdefs.IsNotFound(err) {
			log.Debug().
				Err(err).
				Fields(fields).
				Msg("Container not found, treating as already stopped")

			return nil
		}

		log.Error().
			Err(err).
			Fields(fields).
			Msg("Failed to stop container")

		return fmt.Errorf("%w: %w", errStopContainerFailed, err)
	}

	return nil
}

// snapshotCopyFilesForRecreate stores labeled files before the source is removed.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - client: Docker client. No-op unless it implements container.CopyFileStore.
//   - source: Container about to be stopped and removed.
//
// Returns:
//   - error: Non-nil if a labeled path cannot be snapshotted.
func snapshotCopyFilesForRecreate(
	ctx context.Context,
	client container.Client,
	source types.Container,
) error {
	store, ok := client.(container.CopyFileStore)
	if !ok {
		return nil
	}

	err := store.SnapshotCopyFiles(ctx, source)
	if err != nil {
		return fmt.Errorf("snapshot copy-file paths: %w", err)
	}

	return nil
}

// discardCopyFilesForRecreate drops a leftover snapshot after a failed stop.
//
// Parameters:
//   - client: Docker client. No-op unless it implements container.CopyFileStore.
//   - containerID: Source container ID whose snapshot should be discarded.
func discardCopyFilesForRecreate(client container.Client, containerID types.ContainerID) {
	store, ok := client.(container.CopyFileStore)
	if !ok {
		return
	}

	store.DiscardCopyFiles(containerID)
}

// restartContainersInSortedOrder restarts stopped containers.
//
// It restarts containers in dependency order, collecting cleaned image info
// for stale containers that were not renamed during a self-update, and
// tracking any restart failures.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - containers: List of containers to restart.
//   - client: Container client for Docker operations.
//   - config: Update options controlling restart behavior.
//   - stoppedImages: Slice of cleaned image info for previously stopped containers.
//   - cleanupImageInfos: Pointer to slice to collect cleaned image info for deferred cleanup.
//   - progress: Progress tracker to update with new container IDs.
//
// Returns:
//   - map[types.ContainerID]error: Map of container IDs to errors for failed restarts.
func restartContainersInSortedOrder(
	log *zerolog.Logger,
	ctx context.Context,
	containers []types.Container,
	client container.Client,
	config types.UpdateParams,
	stoppedImages []types.RemovedImageInfo,
	cleanupImageInfos *[]types.RemovedImageInfo,
	progress *session.Progress,
	gitSession *gitSession,
) map[types.ContainerID]error {
	failed := make(map[types.ContainerID]error, len(containers))
	// Track renamed containers to skip cleanup.
	renamedContainers := make(map[types.ContainerID]bool)

	// Restart containers in sorted order to respect dependency chains.
	for i := range containers {
		c := containers[i]

		if !c.ToRestart() {
			continue
		}

		if gitSession.skipRecreate(c, config) {
			continue
		}

		fields := map[string]any{
			"container": c.Name(),
			"image":     c.ImageName(),
		}

		// Check if container was previously stopped by looking in stoppedImages slice.
		wasStopped := false

		for _, sc := range stoppedImages {
			if sc.ContainerID == c.ID() {
				wasStopped = true

				break
			}
		}

		// Skip other Watchtower containers, and every Watchtower container while
		// self-updates are disabled. A Watchtower container restarted with a
		// dependency would otherwise be recreated anyway.
		if c.IsWatchtower() && (config.SkipSelfUpdate ||
			config.CurrentContainerID != "" && c.ID() != config.CurrentContainerID) {
			continue
		}

		// Parent cancellation must not strand containers already removed in the
		// stop phase. restartStaleContainer uses a detached context for create/start,
		// so continue recreating stopped (and self-update) instances. Containers that
		// were never stopped can be skipped safely.
		if ctx.Err() != nil && !c.IsWatchtower() && !wasStopped {
			log.Info().
				Str("container", c.Name()).
				Str("image", c.ImageName()).
				Str("container_id", c.ID().ShortID()).
				Msg("Skipped container restart due to context cancellation")
			failed[c.ID()] = skipped(fmt.Errorf("restart skipped: %w", ctx.Err()))

			continue
		}

		if ctx.Err() != nil && (c.IsWatchtower() || wasStopped) {
			log.Info().
				Fields(fields).
				Str("container_id", c.ID().ShortID()).
				Msg("Parent context canceled. Continuing restart of stopped container")
		}

		// Restart Watchtower containers regardless of stoppedImages, as they are renamed.
		// Otherwise, restart only containers that were previously stopped.
		if c.IsWatchtower() || wasStopped {
			newContainerID, renamed, err := restartStaleContainer(log,
				ctx,
				c,
				client,
				config,
			)
			if err != nil {
				failed[c.ID()] = err
			} else {
				// Set the new container ID in progress
				if progress != nil {
					status, exists := (*progress)[c.ID()]
					if exists {
						status.SetNewContainerID(newContainerID)
						// Mark as restarted if not stale (not updated)
						if !c.IsStale() {
							progress.MarkRestarted(log, c.ID())
						}
					}
				}

				log.Debug().
					Fields(fields).
					Msg("Restarted container")

				if renamed {
					renamedContainers[c.ID()] = true
				}
				// Only collect cleaned image info for stale containers that were
				// not renamed, as renamed containers (Watchtower self-updates)
				// are cleaned up by CheckForMultipleWatchtowerInstances
				// in the new container.
				if c.IsStale() && !renamedContainers[c.ID()] {
					addCleanupImageInfo(
						cleanupImageInfos,
						c.ImageID(),
						c.ImageName(),
						c.Name(),
						c.ID(),
					)
				}
			}
		}
	}

	return failed
}

// addCleanupImageInfo adds cleanup info if not already present.
//
// Deduplication is based on the (ImageID, ContainerName) pair so that when
// notifications are split by container, each container that used the old image
// receives its own "Removing image" entry.
//
// Parameters:
//   - cleanupImageInfos: Pointer to slice to collect cleaned image info.
//   - imageID: ID of the image to clean up.
//   - imageName: Name of the image.
//   - containerName: Name of the container.
//   - containerID: ID of the container (optional, pass empty string if not available).
func addCleanupImageInfo(
	cleanupImageInfos *[]types.RemovedImageInfo,
	imageID types.ImageID,
	imageName, containerName string,
	containerID types.ContainerID,
) {
	for _, existing := range *cleanupImageInfos {
		if existing.ImageID == imageID && existing.ContainerName == containerName {
			return
		}
	}

	*cleanupImageInfos = append(*cleanupImageInfos, types.RemovedImageInfo{
		ImageID:       imageID,
		ContainerID:   containerID,
		ImageName:     imageName,
		ContainerName: containerName,
	})
}

// restartStaleContainer restarts a stale container.
//
// It renames Watchtower containers if applicable, starts a new container,
// and runs post-update hooks.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - sourceContainer: Container to restart.
//   - client: Container client for Docker operations.
//   - config: Update options controlling restart and lifecycle hooks.
//
// Returns:
//   - types.ContainerID: ID of the new container if started, original ID if renamed only, empty otherwise.
//   - bool: True if the container was renamed, false otherwise.
//   - error: Non-nil if restart fails, nil on success.
func restartStaleContainer(
	log *zerolog.Logger,
	ctx context.Context,
	sourceContainer types.Container,
	client container.Client,
	config types.UpdateParams,
) (types.ContainerID, bool, error) {
	// Create a detached context to survive parent context cancellation.
	// This ensures container cleanup and update operations complete even if the
	// parent context is canceled during the restart process.
	// A finite fallback is applied when config.Timeout is non-positive, and the
	// create/start phase uses at least defaultCreateStartTimeout to accommodate
	// slow hosts.
	detachedTimeout := max(restartPolicyTimeout(config.Timeout), defaultCreateStartTimeout)

	detachedCtx, cancelDetached := context.WithTimeout(
		context.Background(),
		detachedTimeout,
	)

	defer cancelDetached()

	fields := map[string]any{
		"container": sourceContainer.Name(),
		"image":     sourceContainer.ImageName(),
	}

	var (
		renamed                bool
		originalWatchtowerName string
	)

	// Rename Watchtower containers regardless of NoRestart flag,
	// but skip in run-once mode as there's no need to avoid conflicts
	// with a continuously running instance.
	if sourceContainer.IsWatchtower() && !config.RunOnce {
		// Opt-in ephemeral self-update: use a short-lived orchestrator container
		// to perform the transition atomically. The orchestrator handles stopping
		// the old container, creating and starting the new one, and cleanup.
		// EphemeralSelfUpdate returns immediately after starting the orchestrator.
		// The orchestrator completes the replacement asynchronously. The current
		// Watchtower process will be stopped by the orchestrator shortly after.
		if config.EphemeralSelfUpdate {
			log.Debug().
				Fields(fields).
				Msg("Using ephemeral self-update")

			_, renamed, err := EphemeralSelfUpdate(log,
				ctx,
				client,
				sourceContainer,
				config,
			)
			if err != nil {
				return "", false, err
			}

			// Skip health check and post-update hooks: the new container's ID is
			// not known to this process, and the orchestrator will stop this process
			// shortly. The new Watchtower instance handles its own lifecycle.
			return "", renamed, nil
		}

		targetOldName := types.WatchtowerOldPrefix + sourceContainer.ID().ShortID()

		// Redundant rename guard: the lingering old instance already has the
		// target name from a prior rename. Skip to avoid a same-name error.
		if container.IsOldContainer(sourceContainer.Name()) {
			log.Debug().
				Fields(fields).
				Str("target_name", targetOldName).
				Msg("Skipping rename of already-renamed Watchtower container")

			renamed = true
		} else {
			originalWatchtowerName = sourceContainer.Name()
			newName := targetOldName

			err := client.RenameContainer(
				ctx,
				sourceContainer,
				newName,
			)
			if err != nil {
				log.Debug().
					Err(err).
					Str("container", sourceContainer.Name()).
					Str("new_name", newName).
					Msg("Failed to rename Watchtower container")

				return "",
					false,
					fmt.Errorf(
						"%w: %w",
						errRenameWatchtowerFailed,
						err,
					)
			}

			log.Debug().
				Fields(fields).
				Str("new_name", newName).
				Msg("Renamed Watchtower container")

			renamed = true
		}
	}

	// For Watchtower self-updates, accumulate container ID chain in labels.
	if sourceContainer.IsWatchtower() {
		c, ok := sourceContainer.(*container.Container)
		if ok {
			containerInfo := c.ContainerInfo()
			if containerInfo != nil && containerInfo.Config != nil {
				existingChain, _ := c.GetContainerChain()

				var newChain string
				if existingChain != "" {
					newChain = existingChain + "," + string(c.ID())
				} else {
					newChain = string(c.ID())
				}

				c.SetLabel(container.ContainerChainLabel, newChain)
				log.Debug().
					Fields(fields).
					Str("container_chain", newChain).
					Msg("Updated container chain label for Watchtower self-update")
			}
		}
	}

	// Create the new container with updated configuration.
	//nolint:contextcheck // Using detached context intentionally to survive parent cancellation
	newContainerID, err := client.CreateContainer(detachedCtx, sourceContainer)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to create container")

		// Restore the original name so the running instance is not left only as
		// watchtower-old-* with the canonical name free and unused.
		// Use a fresh context here: detachedCtx may already be expired if the
		// create call hit the timeout, and we do not want recovery to fail for
		// the same reason.
		if renamed && originalWatchtowerName != "" && sourceContainer.IsWatchtower() {
			renameBackCtx, renameBackCancel := context.WithTimeout(
				context.Background(),
				restartPolicyTimeout(config.Timeout),
			)
			defer renameBackCancel()

			//nolint:contextcheck // fresh context is intentional for recovery after timeout
			renameBackErr := client.RenameContainer(
				renameBackCtx,
				sourceContainer,
				originalWatchtowerName,
			)
			if renameBackErr != nil {
				log.Debug().
					Err(renameBackErr).
					Fields(fields).
					Str("original_name", originalWatchtowerName).
					Msg("Failed to rename Watchtower container back after create failure")
			} else {
				renamed = false

				log.Debug().
					Fields(fields).
					Str("original_name", originalWatchtowerName).
					Msg("Restored Watchtower container name after create failure")
			}
		}

		return "",
			renamed,
			fmt.Errorf(
				"%w: %w",
				errCreateContainerFailed,
				err,
			)
	}

	// Start the new container based on restart settings:
	//   - Watchtower containers bypass the NoRestart check
	//   - All containers (including Watchtower) start only if they were running or ReviveStopped is enabled
	if (!config.NoRestart || sourceContainer.IsWatchtower()) && (sourceContainer.IsRunning() || config.ReviveStopped) {
		log.Debug().
			Fields(fields).
			Msg("Starting container with updated configuration")

		//nolint:contextcheck // Using detached context intentionally to survive parent cancellation
		err = client.StartContainerByID(detachedCtx, newContainerID)
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Msg("Failed to start container")

			// On start failure after a Watchtower rename, remove the failed replacement
			// container only. The renamed source still holds the running process and must
			// remain so the host keeps a Watchtower instance.
			// Use a fresh context because detachedCtx may already be expired if the start
			// call hit the timeout and we do not want cleanup to fail for the same reason.
			if renamed && sourceContainer.IsWatchtower() {
				log.Debug().
					Fields(fields).
					Str("new_id", newContainerID.ShortID()).
					Msg("Cleaning up failed Watchtower container")

				cleanupCtx, cleanupCancel := context.WithTimeout(
					context.Background(),
					restartPolicyTimeout(config.Timeout),
				)
				defer cleanupCancel()

				//nolint:contextcheck // fresh context is intentional for cleanup after timeout
				failedNew, getErr := client.GetContainer(cleanupCtx, newContainerID)
				if getErr != nil {
					log.Debug().
						Err(getErr).
						Fields(fields).
						Str("new_id", newContainerID.ShortID()).
						Msg("Failed to inspect failed Watchtower container for cleanup")
				} else {
					//nolint:contextcheck // fresh context is intentional for cleanup after timeout
					cleanupErr := client.StopAndRemoveContainer(
						cleanupCtx,
						failedNew,
						config.Timeout,
					)
					if cleanupErr != nil {
						log.Debug().
							Err(cleanupErr).
							Fields(fields).
							Str("new_id", newContainerID.ShortID()).
							Msg("Failed to stop failed Watchtower container")
					}
				}
			}

			return "",
				renamed,
				fmt.Errorf(
					"%w: %w",
					errStartContainerFailed,
					err,
				)
		}

		log.Info().
			Fields(fields).
			Str("new_id", newContainerID.ShortID()).
			Msg("Started new container")

		// Run post-update lifecycle hooks for restarting containers if enabled.
		if sourceContainer.ToRestart() && config.LifecycleHooks {
			log.Debug().
				Fields(fields).
				Msg("Executing post-update command")
			//nolint:contextcheck // Using detached context intentionally to survive parent cancellation
			lifecycle.ExecutePostUpdateCommand(log,
				detachedCtx,
				client,
				newContainerID,
				config.LifecycleUID,
				config.LifecycleGID,
			)
		}
	}

	// For renamed Watchtower containers, update restart policy to "no" to prevent auto-restart.
	if renamed && sourceContainer.IsWatchtower() {
		log.Debug().
			Fields(fields).
			Msg("Updating restart policy for old Watchtower container")

		//nolint:contextcheck // Using detached context intentionally to survive parent cancellation
		client.SetRestartPolicy(
			detachedCtx,
			sourceContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)
	}

	return newContainerID, renamed, nil
}

// deriveScopeFromCurrentContainer finds the current container by ID in the
// provided list and returns its scope and whether it was found. Returns ""
// for unscoped containers (found but no scope) and found=false when the
// container ID isn't in the list. The caller normalizes "" to "none" for
// cleanup and skips cleanup when found=false.
func deriveScopeFromCurrentContainer(log *zerolog.Logger, allContainers []types.Container,
	currentContainerID types.ContainerID,
) (string, bool) {
	for _, c := range allContainers {
		if c.ID() == currentContainerID {
			containerScope, containerHasScope := c.Scope()
			if !containerHasScope || containerScope == "" {
				return "", true
			}

			return containerScope, true
		}
	}

	log.Debug().
		Str("current_container_id", string(currentContainerID)).
		Msg("Current container not found in list")

	return "", false
}

// restartPolicyTimeout returns the timeout to use for restart policy operations.
// If the provided timeout is positive, it is returned as-is.
// Otherwise, a finite fallback is used to prevent indefinite blocking.
func restartPolicyTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}

	return defaultRestartPolicyTimeout
}
