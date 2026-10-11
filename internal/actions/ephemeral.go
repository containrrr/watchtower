package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// orchestratorTimeout defines the maximum duration for the orchestrator to complete.
// This covers: inspect old, stop old, create new, start new, remove old.
const orchestratorTimeout = 5 * time.Minute

// orchestratorStopTimeout defines the timeout for stopping the old container.
const orchestratorStopTimeout = 30 * time.Second

// orchestratorCreateTimeout is the timeout for the detached context used when
// creating the ephemeral orchestrator container.
const orchestratorCreateTimeout = 60 * time.Second

// Errors for ephemeral self-update operations in ephemeral.go.
var (
	// errEphemeralOrchestratorFailed indicates the ephemeral orchestrator failed.
	errEphemeralOrchestratorFailed = errors.New("ephemeral orchestrator failed")
	// errOrchestratorMissingEnv indicates a required environment variable is missing.
	errOrchestratorMissingEnv = errors.New("missing orchestrator environment variable")
	// errOrchestratorOldContainerNotFound indicates the old container was not found.
	errOrchestratorOldContainerNotFound = errors.New("old container not found")
	// errOrchestratorStopFailed indicates a failure to stop the old container.
	errOrchestratorStopFailed = errors.New("failed to stop old container")
	// errOrchestratorRemoveFailed indicates a failure to remove the old container.
	errOrchestratorRemoveFailed = errors.New("failed to remove old container")
	// errOrchestratorCreateFailed indicates a failure to create the new container.
	errOrchestratorCreateFailed = errors.New("failed to create new container")
	// errOrchestratorStartFailed indicates a failure to start the new container.
	errOrchestratorStartFailed = errors.New("failed to start new container")
	// errOrchestratorInspectFailed indicates a failure to inspect a container during orchestration.
	errOrchestratorInspectFailed = errors.New("failed to inspect container during orchestration")
	// errOrchestratorRenameFailed indicates a failure to rename the old container for handoff.
	errOrchestratorRenameFailed = errors.New("failed to rename old container for handoff")
	// errNewContainerNotRunning indicates the new container is not running after start.
	errNewContainerNotRunning = errors.New("new container is not running after start")
)

// EphemeralSelfUpdate performs a self-update using an ephemeral orchestrator container.
//
// Instead of the rename-based approach, this creates a short-lived container that:
//  1. Inspects the old container's configuration
//  2. Stops the old container
//  3. Creates a new container from the new image with the same config
//  4. Starts the new container
//  5. Removes the old container
//  6. When cleanup is enabled, removes the unused predecessor image
//  7. Exits (AutoRemove cleans up the orchestrator)
//
// This function returns immediately after starting the orchestrator. The
// orchestrator handles the full replacement sequence asynchronously. The
// current Watchtower process will be stopped by the orchestrator shortly
// after this function returns.
//
// The ephemeral container uses the same Watchtower image (already pulled) and
// mounts the Docker socket for container management.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - sourceContainer: Current Watchtower container being replaced.
//   - config: Update parameters.
//
// Returns:
//   - types.ContainerID: Empty string (the new container's ID is not known to the caller).
//   - bool: True so the dying process skips image cleanup. The orchestrator
//     removes the old container and, when cleanup is enabled, the old image.
//   - error: Non-nil if orchestrator creation fails.
func EphemeralSelfUpdate(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	sourceContainer types.Container,
	config types.UpdateParams,
) (types.ContainerID, bool, error) {
	clogVal := log.With().
		Str("container", sourceContainer.Name()).
		Str("image", sourceContainer.ImageName()).
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Initiating ephemeral self-update for Watchtower")

	// Create a detached context for the orchestrator creation to survive parent cancellation.
	// Uses a 60-second timeout to prevent indefinite hangs during orchestrator creation.
	detachedCtx, cancelDetached := context.WithTimeout(
		context.Background(),
		orchestratorCreateTimeout,
	)
	defer cancelDetached()

	// The image name from the source container reflects the latest pulled image.
	// This is the same image the ephemeral container will use.
	newImage := sourceContainer.ImageName()

	// Compute the container chain for lineage tracking. The orchestrator will
	// set this on the new container's labels via the WT_ORCHESTRATOR_CONTAINER_CHAIN
	// environment variable. This preserves the cleanup behavior used in the rename path.
	existingChain, _ := sourceContainer.GetContainerChain()

	var newChain string
	if existingChain != "" {
		newChain = existingChain + "," + string(sourceContainer.ID())
	} else {
		newChain = string(sourceContainer.ID())
	}

	clog.Debug().
		Str("container_chain", newChain).
		Msg("Computed container chain for ephemeral self-update")

	clog.Debug().Msg("Creating ephemeral orchestrator for self-update")

	// Log "Stopping container" for notification template compatibility.
	// The orchestrator will handle the actual stop/start/remove operations,
	// but we emit these Info entries so notifications match the normal update flow.
	log.Info().
		Str("container", sourceContainer.Name()).
		Str("id", sourceContainer.ID().ShortID()).
		Str("old_image_id", sourceContainer.ImageID().ShortID()).
		Msg("Stopping container")

	// Create the ephemeral orchestrator container.
	//nolint:contextcheck // detached context is intentional for orchestrator lifecycle
	orchestratorID, err := client.CreateEphemeralOrchestrator(
		detachedCtx,
		sourceContainer,
		newImage,
		newChain,
		config.Cleanup,
	)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to create ephemeral orchestrator")

		return "", false, fmt.Errorf("%w: %w", errEphemeralOrchestratorFailed, err)
	}

	// Create and start the new container.
	log.Info().Msg("Starting new Watchtower container")

	// Log that the orchestrator has been started. The orchestrator ID identifies
	// the ephemeral container that will perform the replacement. The actual new
	// container's ID is determined by the orchestrator and emitted in its own
	// "Started new container" log entry.
	log.Debug().
		Str("container", sourceContainer.Name()).
		Str("orchestrator_id", orchestratorID.ShortID()).
		Msg("Started self-update orchestrator")

	// Return immediately. The orchestrator handles the full replacement sequence
	// asynchronously: stopping the old container, creating and starting the new one,
	// and cleaning up. The current Watchtower process will be stopped by the
	// orchestrator shortly after this function returns.
	//
	// Return an empty container ID because the new container's ID is not known
	// to this process — it is created by the orchestrator after this process is
	// stopped. Callers must handle the ephemeral case specially (skip health checks
	// and post-update hooks since the old process exits before the new container exists).
	//
	// Return true for "renamed" so the dying process does not collect the old
	// image for cleanup. This process is about to be stopped and still uses that
	// image. The orchestrator removes the old container and, when cleanup is
	// enabled, the old image after the replacement is running.
	return "", true, nil
}

// RunOrchestrator executes the orchestrator mode for self-update.
//
// This is the entry point when Watchtower is started with --self-update-orchestrator.
// It reads environment variables to determine the old container ID, new image, and
// original container name, then performs the container replacement sequence.
//
// The orchestrator follows a deterministic state machine:
//  1. VALIDATE: Read and validate environment variables
//  2. INSPECT: Get the old container's full configuration
//  3. STOP OLD: Stop the old container (frees host ports)
//  4. RENAME OLD: Move the stopped predecessor off the original name
//  5. CREATE NEW: Create a new container from the new image with the same config
//  6. START NEW: Start the new Watchtower container
//  7. VERIFY: Confirm the new container is running
//  8. REMOVE OLD: Delete the renamed predecessor
//  9. REMOVE OLD IMAGE: When cleanup is enabled, delete the unused predecessor image
//
// Stop before create releases published host ports. Rename keeps the stopped
// predecessor for recovery if create or start fails (restore name and start).
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
func RunOrchestrator(log *zerolog.Logger, ctx context.Context, client container.Client) {
	clogVal := log.With().
		Str("mode", "orchestrator").
		Logger()
	clog := &clogVal

	clog.Debug().Msg("Starting Watchtower self-update orchestrator")

	// Read environment variables.
	oldID, newImage, originalName, containerChain, cleanup, err := readOrchestratorEnv()
	if err != nil {
		clog.Fatal().
			Err(err).
			Msg("Failed to read orchestrator environment variables")
	}

	clog.Debug().
		Str("old_id", oldID).
		Str("new_image", newImage).
		Str("original_name", originalName).
		Str("container_chain", containerChain).
		Bool("cleanup", cleanup).
		Msg("Read orchestrator environment variables")

	// Create a timeout context for the entire orchestration.
	orchCtx, orchCancel := context.WithTimeout(
		ctx,
		orchestratorTimeout,
	)

	// Execute the orchestration sequence.
	err = orchestrateSelfUpdate(clog,
		orchCtx,
		client,
		oldID,
		newImage,
		originalName,
		containerChain,
		cleanup,
	)
	// Determine exit code and log result.
	exitCode := 0
	if err != nil {
		exitCode = 1

		clog.Error().
			Err(err).
			Msg("Orchestration failed")
	} else {
		clog.Debug().Msg("Self-update orchestration completed successfully")
	}

	// Explicitly cancel before exit since deferred calls do not run on os.Exit.
	orchCancel()
	os.Exit(exitCode)
}

// readOrchestratorEnv reads and validates the environment variables required
// by the orchestrator.
//
// Returns:
//   - string: Old container ID.
//   - string: New image reference.
//   - string: Original container name.
//   - string: Container chain for lineage tracking.
//   - bool: Whether old-image cleanup is enabled.
//   - error: Non-nil if any required variable is missing.
func readOrchestratorEnv() (string, string, string, string, bool, error) {
	oldID := os.Getenv("WT_ORCHESTRATOR_OLD_ID")
	if oldID == "" {
		return "", "", "", "", false, fmt.Errorf(
			"%w: WT_ORCHESTRATOR_OLD_ID",
			errOrchestratorMissingEnv,
		)
	}

	newImage := os.Getenv("WT_ORCHESTRATOR_NEW_IMAGE")
	if newImage == "" {
		return "", "", "", "", false, fmt.Errorf(
			"%w: WT_ORCHESTRATOR_NEW_IMAGE",
			errOrchestratorMissingEnv,
		)
	}

	originalName := os.Getenv("WT_ORCHESTRATOR_ORIGINAL_NAME")
	if originalName == "" {
		return "", "", "", "", false, fmt.Errorf(
			"%w: WT_ORCHESTRATOR_ORIGINAL_NAME",
			errOrchestratorMissingEnv,
		)
	}

	containerChain := os.Getenv("WT_ORCHESTRATOR_CONTAINER_CHAIN")

	cleanup, _ := strconv.ParseBool(os.Getenv("WT_ORCHESTRATOR_CLEANUP"))

	return oldID, newImage, originalName, containerChain, cleanup, nil
}

// orchestrateSelfUpdate performs the container replacement sequence for a
// Watchtower self-update.
//
// Sequence:
//  1. Inspect the old container and propagate the chain label.
//  2. Pin Config.Image to newImage for create.
//  3. Stop the old container (frees host ports).
//  4. Rename the stopped old container off the original name.
//  5. Create and start the new container under the original name.
//  6. Verify the new container is running.
//  7. Remove the renamed predecessor.
//  8. When cleanup is enabled, remove the unused predecessor image.
//
// Stop before create releases published host ports. Rename keeps the stopped
// predecessor available for recovery. When the rename fails, the old container
// is started again. On create/start/verify failure after rename, the original
// name is restored and the old container is started again.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - oldID: ID of the old Watchtower container to replace.
//   - newImage: Image reference for the new container.
//   - originalName: Original container name to preserve on the new container.
//   - containerChain: Container chain label for lineage tracking.
//   - cleanup: Remove the old image after the predecessor container is gone.
//
// Returns:
//   - error: Non-nil if any step in the orchestration fails.
func orchestrateSelfUpdate(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	oldID string,
	newImage string,
	originalName string,
	containerChain string,
	cleanup bool,
) error {
	clogVal := log.With().
		Str("old_id", oldID).
		Str("new_image", newImage).
		Str("original_name", originalName).
		Logger()
	clog := &clogVal

	oldContainer, err := inspectOldContainer(
		ctx,
		client,
		clog,
		oldID,
		containerChain,
	)
	if err != nil {
		return err
	}

	// Capture before pinContainerCreateImage overwrites the cached image name.
	oldImageID := oldContainer.ImageID()
	oldImageName := oldContainer.ImageName()

	pinContainerCreateImage(oldContainer, newImage, clog)

	// Free host ports before create. Keep the container so rename can preserve
	// it for recovery if the replacement fails.
	oldGone, err := stopOldContainer(ctx, client, clog, oldContainer)
	if err != nil {
		return err
	}

	if !oldGone {
		// renamed controls whether a rename was performed (skip if already
		// watchtower-old-*). Recovery uses !oldGone so a predecessor left under
		// that prefix from an earlier attempt is still restored and restarted.
		_, err = renameOldContainerForHandoff(ctx, client, clog, oldContainer, originalName)
		if err != nil {
			// The old container keeps its name, so starting it again leaves
			// a Watchtower instance running.
			restartOldContainer(ctx, client, clog, oldContainer)

			return err
		}
	}

	newContainerID, err := createAndStartNewContainer(
		ctx,
		client,
		clog,
		oldContainer,
	)
	if err != nil {
		// Recover any existing predecessor, including one already renamed to
		// watchtower-old-* from an earlier attempt (renamed may be false).
		if !oldGone {
			restoreAndStartOldContainer(ctx, client, clog, oldContainer, originalName)
		}

		return err
	}

	err = ensureContainerRunning(ctx, client, clog, newContainerID)
	if err != nil {
		cleanupFailedNewContainer(ctx, client, clog, newContainerID)

		if !oldGone {
			restoreAndStartOldContainer(ctx, client, clog, oldContainer, originalName)
		}

		return err
	}

	if !oldGone {
		removeErr := removeOldContainer(ctx, client, clog, oldContainer)
		if removeErr != nil {
			clog.Warn().
				Err(removeErr).
				Msg("New Watchtower is running but cleanup of the old container failed")
		}
	}

	if cleanup {
		removeOldImage(ctx, client, clog, oldImageID, oldImageName)
	}

	// Emit the actual new container ID at Info level so notification templates
	// can consume the correct "new_id" field.
	clog.Info().
		Str("new_id", newContainerID.ShortID()).
		Msg("Started new container")

	return nil
}

// pinContainerCreateImage sets Config.Image on a concrete container.Container so
// GetCreateConfig uses newImage for the replacement instance.
//
// SetImageName also updates the cached ImageName used after the pin.
//
// Parameters:
//   - oldContainer: Source container whose create config is pinned.
//   - newImage: Image reference to use when creating the replacement.
//   - clog: Logger with container context fields.
func pinContainerCreateImage(oldContainer types.Container, newImage string, clog *zerolog.Logger) {
	if newImage == "" {
		return
	}

	c, ok := oldContainer.(*container.Container)
	if !ok {
		clog.Debug().Msg("Old container is not a concrete Container. Cannot pin create image")

		return
	}

	info := c.ContainerInfo()
	if info == nil || info.Config == nil {
		return
	}

	c.SetImageName(newImage)
	clog.Debug().
		Str("pinned_image", newImage).
		Msg("Pinned create image for ephemeral self-update")
}

// renameOldContainerForHandoff renames the stopped old container off its original
// name so StartContainer can claim that name. The container is kept for recovery.
//
// Returns:
//   - renamed: True when a rename was performed.
//   - error: Non-nil if rename fails.
func renameOldContainerForHandoff(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
	originalName string,
) (bool, error) {
	if container.IsOldContainer(oldContainer.Name()) {
		clog.Debug().Msg("Old container already has a watchtower-old name. Skipping rename")

		return false, nil
	}

	targetName := types.WatchtowerOldPrefix + oldContainer.ID().ShortID()
	clog.Debug().
		Str("target_name", targetName).
		Msg("Renaming stopped Watchtower container to free original name")

	err := client.RenameContainer(ctx, oldContainer, targetName)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to rename old container before handoff")

		return false, fmt.Errorf("%w: %w", errOrchestratorRenameFailed, err)
	}

	clog.Debug().
		Str("original_name", originalName).
		Str("target_name", targetName).
		Msg("Renamed stopped Watchtower container for handoff")

	return true, nil
}

// restoreAndStartOldContainer renames the predecessor back to originalName and
// starts it after a failed create/start so a Watchtower instance remains running.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldContainer: Stopped predecessor to restore.
//   - originalName: Canonical name to restore on the predecessor.
func restoreAndStartOldContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
	originalName string,
) {
	if originalName == "" {
		return
	}

	clog.Warn().
		Str("original_name", originalName).
		Msg("Restoring old Watchtower container after handoff failure")

	err := client.RenameContainer(ctx, oldContainer, originalName)
	if err != nil {
		clog.Error().
			Err(err).
			Str("original_name", originalName).
			Msg("Failed to restore old Watchtower container name. Manual intervention required")

		return
	}

	err = client.StartContainerByID(ctx, oldContainer.ID())
	if err != nil {
		clog.Error().
			Err(err).
			Str("original_name", originalName).
			Msg("Failed to restart old Watchtower container after name restore. Manual intervention required")

		return
	}

	clog.Info().
		Str("original_name", originalName).
		Msg("Restored and restarted old Watchtower container after handoff failure")
}

// restartOldContainer starts the predecessor again after a failed rename, so
// a Watchtower instance remains running under its original name.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldContainer: Stopped predecessor to start.
func restartOldContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
) {
	err := client.StartContainerByID(ctx, oldContainer.ID())
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to restart old Watchtower container after rename failure. Manual intervention required")

		return
	}

	clog.Info().Msg("Restarted old Watchtower container after rename failure")
}

// cleanupFailedNewContainer best-effort removes a new container that failed
// verification so the original name can be restored on the predecessor.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - newContainerID: ID of the failed replacement container to remove.
func cleanupFailedNewContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	newContainerID types.ContainerID,
) {
	if newContainerID == "" {
		return
	}

	failedNew, err := client.GetContainer(ctx, newContainerID)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("new_id", newContainerID.ShortID()).
			Msg("Failed to inspect failed new container for cleanup")

		return
	}

	cleanupErr := client.StopAndRemoveContainer(ctx, failedNew, orchestratorStopTimeout)
	if cleanupErr != nil {
		clog.Debug().
			Err(cleanupErr).
			Str("new_id", newContainerID.ShortID()).
			Msg("Failed to remove failed new container after verify error")
	}
}

// inspectOldContainer retrieves the old container's configuration and propagates
// the container chain label to it. The chain label is set on the old container's
// config so that StartContainer's GetCreateConfig() includes it on the new container.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldID: ID of the old Watchtower container to inspect.
//   - containerChain: Container chain label for lineage tracking.
//
// Returns:
//   - types.Container: The inspected old container.
//   - error: Non-nil if inspection fails.
func inspectOldContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldID string,
	containerChain string,
) (types.Container, error) {
	clog.Debug().Msg("Inspecting old container")

	oldContainer, err := client.GetContainer(
		ctx,
		types.ContainerID(oldID),
	)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			clog.Error().Msg("Old container not found")

			return nil, fmt.Errorf("%w: %s", errOrchestratorOldContainerNotFound, oldID)
		}

		clog.Error().
			Err(err).
			Msg("Failed to inspect old container")

		return nil, fmt.Errorf("%w: %w", errOrchestratorInspectFailed, err)
	}

	if !oldContainer.IsRunning() {
		clog.Warn().Msg("Old container is not running - proceeding with creation only")
	}

	clog.Debug().
		Str("old_name", oldContainer.Name()).
		Msg("Inspected old container successfully")

	// Propagate the container chain label to the old container's config, so
	// StartContainer's GetCreateConfig() includes the label on the new container.
	if containerChain != "" {
		concrete, ok := oldContainer.(*container.Container)
		if ok {
			concrete.SetLabel(container.ContainerChainLabel, containerChain)
			clog.Debug().
				Str("container_chain", containerChain).
				Msg("Set container chain label on source container config")
		}
	}

	return oldContainer, nil
}

// stopOldContainer stops the old container so published host ports are released
// before the replacement is created. The container is left in place for rename
// and optional recovery.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldContainer: The old Watchtower container to stop.
//
// Returns:
//   - oldGone: True when the container is already absent (NotFound on stop).
//   - error: Non-nil if stopping fails fatally while the container still runs.
func stopOldContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
) (bool, error) {
	clog.Debug().Msg("Stopping old Watchtower container")

	err := client.StopContainer(
		ctx,
		oldContainer,
		orchestratorStopTimeout,
	)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			clog.Debug().Msg("Old container already removed")

			return true, nil
		}

		// Re-inspect after a non-NotFound stop error. The cached view may be stale.
		freshContainer, inspectErr := client.GetContainer(
			ctx,
			oldContainer.ID(),
		)
		if inspectErr != nil {
			if cerrdefs.IsNotFound(inspectErr) {
				clog.Debug().Msg("Old container already removed")

				return true, nil
			}

			clog.Error().
				Err(inspectErr).
				Msg("Failed to re-inspect old container after stop failure")
			clog.Error().
				Err(err).
				Msg("Failed to stop old container")

			return false, fmt.Errorf("%w: %w", errOrchestratorStopFailed, err)
		}

		if !freshContainer.IsRunning() {
			clog.Debug().Msg("Old container is not running after stop attempt")

			return false, nil
		}

		clog.Error().
			Err(err).
			Msg("Failed to stop old container")

		return false, fmt.Errorf("%w: %w", errOrchestratorStopFailed, err)
	}

	clog.Debug().Msg("Old container stopped")

	return false, nil
}

// removeOldContainer removes the renamed predecessor after a successful handoff.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldContainer: The old Watchtower container to remove.
//
// Returns:
//   - error: Non-nil if removal fails for a reason other than NotFound.
func removeOldContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
) error {
	clog.Debug().Msg("Removing old Watchtower container after successful handoff")

	err := client.RemoveContainer(ctx, oldContainer)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			clog.Debug().Msg("Old container already removed")

			return nil
		}

		clog.Error().
			Err(err).
			Msg("Failed to remove old container")

		return fmt.Errorf("%w: %w", errOrchestratorRemoveFailed, err)
	}

	clog.Debug().Msg("Old container removed")

	return nil
}

// removeOldImage removes the predecessor Watchtower image after a successful
// handoff. NotFound and in-use errors are skipped. Other failures are logged
// and do not fail orchestration because the replacement is already running.
// Docker calls use a detached timeout so SIGTERM cannot abort removal.
//
// Parameters:
//   - ctx: Parent session context. Cancellation is detached for Docker calls.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - imageID: ID of the predecessor image captured before create-image pinning.
//   - imageName: Name of the predecessor image captured before create-image pinning.
func removeOldImage(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	imageID types.ImageID,
	imageName string,
) {
	if imageID == "" {
		return
	}

	clog.Debug().
		Str("image_id", imageID.ShortID()).
		Str("image_name", imageName).
		Msg("Removing old Watchtower image after ephemeral handoff")

	// Detach from the session context so SIGTERM cannot abort predecessor
	// image removal after the replacement is already running.
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		restartPolicyTimeout(0),
	)
	defer cancel()

	err := client.RemoveImageByID(cleanupCtx, imageID, imageName)
	if err == nil {
		return
	}

	switch {
	case cerrdefs.IsNotFound(err),
		cerrdefs.IsConflict(err),
		errors.Is(err, container.ErrImageInUse),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		cleanupCtx.Err() != nil:
		clog.Debug().
			Err(err).
			Str("image_id", imageID.ShortID()).
			Str("image_name", imageName).
			Msg("Skipped old Watchtower image removal")
	default:
		clog.Warn().
			Err(err).
			Str("image_id", imageID.ShortID()).
			Str("image_name", imageName).
			Msg("New Watchtower is running but cleanup of the old image failed")
	}
}

// createAndStartNewContainer creates and starts a new container using the old
// container's configuration. StartContainer handles config extraction, container
// creation, renaming, network attachment, and starting.
//
// StartContainer resolves the image from the source container's config
// (GetCreateConfig().Image), not from the WT_ORCHESTRATOR_NEW_IMAGE env var.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - oldContainer: The old container whose config is used for the new one.
//
// Returns:
//   - types.ContainerID: ID of the newly created container.
//   - error: Non-nil if creation or starting fails.
func createAndStartNewContainer(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	oldContainer types.Container,
) (types.ContainerID, error) {
	clog.Debug().Msg("Creating and starting new Watchtower container")

	newContainerID, err := client.StartContainer(
		ctx,
		oldContainer,
	)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to create and start new container")

		return "", fmt.Errorf("%w: %w", errOrchestratorCreateFailed, err)
	}

	return newContainerID, nil
}

// ensureContainerRunning verifies the container is running and starts it
// explicitly if needed. StartContainer may create the container without
// starting it if the source container was stopped and the reviveStopped
// option is not enabled.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - clog: Logger with container context fields.
//   - containerID: ID of the container to verify and start.
//
// Returns:
//   - error: Non-nil if the container cannot be verified or started.
func ensureContainerRunning(
	ctx context.Context,
	client container.Client,
	clog *zerolog.Logger,
	containerID types.ContainerID,
) error {
	clog.Debug().
		Str("new_id", containerID.ShortID()).
		Msg("Verifying new container is running")

	ctr, err := client.GetContainer(ctx, containerID)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to inspect new container")

		return fmt.Errorf("%w: %w", errOrchestratorInspectFailed, err)
	}

	if ctr.IsRunning() {
		clog.Debug().Msg("New container verified as running")

		return nil
	}

	// Container was created but not started. Start it explicitly.
	clog.Debug().Msg("New container was created but not started, starting it now")

	err = client.StartContainerByID(ctx, containerID)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to start new container")

		return fmt.Errorf("%w: %w", errOrchestratorStartFailed, err)
	}

	// Re-verify the container is running after the explicit start call.
	ctr, err = client.GetContainer(ctx, containerID)
	if err != nil {
		clog.Error().
			Err(err).
			Msg("Failed to inspect new container after start")

		return fmt.Errorf("%w: %w", errOrchestratorInspectFailed, err)
	}

	if !ctr.IsRunning() {
		clog.Error().Msg("New container is not running after explicit start")

		return fmt.Errorf("%w: %s", errNewContainerNotRunning, containerID.ShortID())
	}

	clog.Debug().Msg("New container verified as running")

	return nil
}
