package lifecycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// checkCommandTimeout is the timeout in minutes for check commands (pre/post check hooks).
const checkCommandTimeout = 1

// Errors for lifecycle hook execution.
var (
	// errPreUpdateFailed indicates a failure in executing the pre-update command.
	errPreUpdateFailed = errors.New("pre-update command execution failed")
)

// ExecutePreChecks runs pre-check lifecycle hooks for filtered containers.
//
// When listed is non-nil, those containers are used and ListContainers is skipped.
// A non-nil empty slice is a valid filtered snapshot. Nil falls back to listing.
//
// Parameters:
//   - log: Logger for debug output.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - params: Update parameters with filter.
//   - listed: Already-listed containers from the current scan, or nil.
func ExecutePreChecks(log *zerolog.Logger, ctx context.Context, client container.Client, params types.UpdateParams, listed []types.Container) {
	containers, ok := resolveCheckContainers(log, ctx, client, params, listed, "pre-checks")
	if !ok {
		// Listing failed. Hooks are skipped for this phase.
		return
	}

	for _, currentContainer := range containers {
		executePreCheckCommand(log, ctx, client, currentContainer, params.LifecycleUID, params.LifecycleGID)
	}
}

// ExecutePostChecks runs post-check lifecycle hooks for filtered containers.
//
// When listed is non-nil, those containers are used and ListContainers is skipped.
// A non-nil empty slice is a valid filtered snapshot. Nil falls back to listing.
//
// Parameters:
//   - log: Logger for debug output.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - params: Update parameters with filter.
//   - listed: Already-listed containers from the current scan, or nil.
func ExecutePostChecks(log *zerolog.Logger, ctx context.Context, client container.Client, params types.UpdateParams, listed []types.Container) {
	containers, ok := resolveCheckContainers(log, ctx, client, params, listed, "post-checks")
	if !ok {
		// Listing failed. Hooks are skipped for this phase.
		return
	}

	for _, currentContainer := range containers {
		executePostCheckCommand(log, ctx, client, currentContainer, params.LifecycleUID, params.LifecycleGID)
	}
}

// resolveCheckContainers returns listed containers, or lists them when listed is nil.
//
// Parameters:
//   - log: Logger for debug output.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for listing when listed is empty.
//   - params: Update parameters whose filter is applied to a fresh list.
//   - listed: Already-listed containers from the current scan, or nil.
//   - phase: Log label for the pre-check or post-check phase.
//
// Returns:
//   - []types.Container: Containers to run hooks against.
//   - bool: False when listing fails.
func resolveCheckContainers(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	params types.UpdateParams,
	listed []types.Container,
	phase string,
) ([]types.Container, bool) {
	clogVal := log.With().
		Str("filter", fmt.Sprintf("%v", params.Filter)).
		Logger()
	clog := &clogVal

	if listed != nil {
		// Use the scan snapshot instead of listing again.
		clog.Debug().
			Int("count", len(listed)).
			Msg("Found containers for " + phase)

		return listed, true
	}

	// Fetch containers using the provided filter.
	clog.Debug().Msg("Listing containers for " + phase)

	containers, err := client.ListContainers(ctx, params.Filter)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to list containers for " + phase)

		return nil, false
	}

	clog.Debug().
		Int("count", len(containers)).
		Msg("Found containers for " + phase)

	return containers, true
}

// executePreCheckCommand executes the pre-check hook for a container.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - container: Container to process.
//   - uid: Default UID to run command as.
//   - gid: Default GID to run command as.
func executePreCheckCommand(log *zerolog.Logger, ctx context.Context, client container.Client, container types.Container, uid, gid int) {
	executeCheckCommand(
		log,
		ctx,
		client,
		container,
		uid,
		gid,
		container.GetLifecyclePreCheckCommand(),
		"pre-check",
	)
}

// executePostCheckCommand executes the post-check hook for a container.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - container: Container to process.
//   - uid: Default UID to run command as.
//   - gid: Default GID to run command as.
func executePostCheckCommand(log *zerolog.Logger, ctx context.Context, client container.Client, container types.Container, uid, gid int) {
	executeCheckCommand(
		log,
		ctx,
		client,
		container,
		uid,
		gid,
		container.GetLifecyclePostCheckCommand(),
		"post-check",
	)
}

// executeCheckCommand runs a pre-check or post-check lifecycle command.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - cont: Container to process.
//   - uid: Default UID to run command as.
//   - gid: Default GID to run command as.
//   - command: Command string from the container labels.
//   - phase: Label for logs ("pre-check" or "post-check").
func executeCheckCommand(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	cont types.Container,
	uid, gid int,
	command string,
	phase string,
) {
	clogVal := log.With().
		Str("container", cont.Name()).
		Logger()
	clog := &clogVal

	// Determine effective UID/GID: use container labels if set, otherwise use defaults.
	effectiveUID := uid

	containerUID, ok := cont.GetLifecycleUID()
	if ok {
		effectiveUID = containerUID
	}

	effectiveGID := gid

	containerGID, ok := cont.GetLifecycleGID()
	if ok {
		effectiveGID = containerGID
	}

	// Skip if no command is set.
	if command == "" {
		clog.Debug().
			Msg("No " + phase + " command supplied. Skipping")

		return
	}

	// Execute command with fixed short timeout (1 minute).
	// Check commands are lightweight health checks that should complete quickly,
	// unlike update commands which may perform complex operations and use configurable timeouts.
	clog.Debug().
		Str("command", command).
		Msg("Executing " + phase + " command")

	_, err := client.ExecuteCommand(ctx, cont, command, checkCommandTimeout, effectiveUID, effectiveGID)
	if err != nil {
		// Match historical wording: "Pre-check command failed" / "Post-check command failed".
		failedMsg := "Pre-check command failed"
		if phase == "post-check" {
			failedMsg = "Post-check command failed"
		}

		clog.Debug().
			Err(err).
			Msg(failedMsg)
	}
}

// ExecutePreUpdateCommand executes the pre-update hook for a container.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - container: Container to process.
//   - uid: UID to run command as.
//   - gid: GID to run command as.
//
// Returns:
//   - bool: True if the update should be skipped, because the command exited
//     with code 75 or failed. False if the command succeeded or did not run.
//   - error: Non-nil if the command failed, nil otherwise.
func ExecutePreUpdateCommand(log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	container types.Container,
	uid int,
	gid int,
) (bool, error) {
	timeout := container.PreUpdateTimeout()
	command := container.GetLifecyclePreUpdateCommand()
	clogVal := log.With().
		Str("container", container.Name()).
		Int("timeout", timeout).
		Logger()
	clog := &clogVal

	// Skip if no command or container isn't running.
	if len(command) == 0 {
		clog.Debug().Msg("No pre-update command supplied. Skipping")

		return false, nil
	}

	if !container.IsRunning() || container.IsRestarting() {
		clog.Debug().
			Bool("is_running", container.IsRunning()).
			Bool("is_restarting", container.IsRestarting()).
			Msg("Container is not running. Skipping pre-update command")

		return false, nil
	}

	// Determine effective UID/GID: use container labels if set, otherwise use defaults.
	effectiveUID := uid

	containerUID, ok := container.GetLifecycleUID()
	if ok {
		effectiveUID = containerUID
	}

	effectiveGID := gid

	containerGID, ok := container.GetLifecycleGID()
	if ok {
		effectiveGID = containerGID
	}

	// Execute command with configured timeout.
	clog.Debug().
		Str("command", command).
		Msg("Executing pre-update command")

	success, err := client.ExecuteCommand(ctx, container, command, timeout, effectiveUID, effectiveGID)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Pre-update command failed")

		return true, fmt.Errorf(
			"%w for container %s: %w",
			errPreUpdateFailed,
			container.Name(),
			err,
		)
	}

	clog.Debug().
		Bool("success", success).
		Msg("Pre-update command executed")

	return success, nil
}

// ExecutePostUpdateCommand executes the post-update hook for a container.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeout.
//   - client: Container client for execution.
//   - newContainerID: ID of the updated container.
//   - uid: UID to run command as.
//   - gid: GID to run command as.
func ExecutePostUpdateCommand(log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	newContainerID types.ContainerID,
	uid int,
	gid int,
) {
	clogVal := log.With().
		Str("container_id", newContainerID.ShortID()).
		Logger()
	clog := &clogVal
	clog.Debug().Msg("Retrieving container for post-update")

	// Retrieve container by ID.
	newContainer, err := client.GetContainer(ctx, newContainerID)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to get container for post-update")

		return
	}

	timeout := newContainer.PostUpdateTimeout()
	clogVal = log.With().
		Str("container", newContainer.Name()).
		Int("timeout", timeout).
		Logger()
	clog = &clogVal
	command := newContainer.GetLifecyclePostUpdateCommand()

	// Determine effective UID/GID: use container labels if set, otherwise use defaults.
	effectiveUID := uid

	containerUID, ok := newContainer.GetLifecycleUID()
	if ok {
		effectiveUID = containerUID
	}

	effectiveGID := gid

	containerGID, ok := newContainer.GetLifecycleGID()
	if ok {
		effectiveGID = containerGID
	}

	// Skip if no command is set.
	if len(command) == 0 {
		clog.Debug().Msg("No post-update command supplied. Skipping")

		return
	}

	// Execute command with configured timeout.
	clog.Debug().
		Str("command", command).
		Msg("Executing post-update command")

	_, err = client.ExecuteCommand(ctx, newContainer, command, timeout, effectiveUID, effectiveGID)
	if err != nil {
		clog.Debug().
			Err(err).
			Str("container_id", newContainerID.ShortID()).
			Msg("Post-update command failed")
	}
}
