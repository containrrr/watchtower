package actions

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Update runs update for the external test package with a rate limiter of its
// own, so a whole update scan can be run against a mock client.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client.
//   - config: Update parameters.
//   - gitClient: Optional Git monitor client.
//
// Returns:
//   - types.Report: Results of the update.
//   - []types.RemovedImageInfo: Images eligible for cleanup.
//   - error: Non-nil when the update fails.
func Update(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	config types.UpdateParams,
	gitClient ...*git.Client,
) (types.Report, []types.RemovedImageInfo, error) {
	return update(log, ctx, client, config, ratelimit.New(), gitClient...)
}

// EphemeralSelfUpdate exposes ephemeralSelfUpdate to the external test
// package, so the orchestrator handoff can be run against a mock client.
var EphemeralSelfUpdate = ephemeralSelfUpdate

// SetRemovalRetryDelay sets the delay between removal retries until the test
// ends, so retried removals do not wait in real time.
//
// Parameters:
//   - t: Test that restores the previous delay when it ends.
//   - delay: Delay between retries.
func SetRemovalRetryDelay(t *testing.T, delay time.Duration) {
	t.Helper()

	previous := removalRetryDelay
	removalRetryDelay = delay

	t.Cleanup(func() { removalRetryDelay = previous })
}

// UpdateImplicitRestart marks the containers that depend on a restarting
// container as linked to restarting, as an update scan does. The dependency
// graph is built from containers, with the rest of allContainers unmonitored.
//
// Parameters:
//   - log: Process logger.
//   - allContainers: Every listed container.
//   - containers: The monitored containers, which may be marked.
//   - useComposeDependsOn: Whether to include Compose depends_on labels as links.
func UpdateImplicitRestart(log *zerolog.Logger, allContainers, containers []types.Container, useComposeDependsOn bool) {
	graph, err := sorter.NewDependencyGraph(log,
		containers,
		unmonitoredContainers(allContainers, containers),
		useComposeDependsOn,
	)
	if err != nil {
		return
	}

	markImplicitRestarts(log, graph, allContainers, containers, nil)
}

// reconcileImplicitRestarts clears linked-restart marks and derives them again
// with a dependency graph built from containers, with the rest of
// allContainers unmonitored.
//
// Parameters:
//   - log: Process logger.
//   - allContainers: Every listed container.
//   - containers: The monitored containers.
//   - params: Update parameters.
//
// Returns:
//   - []types.Container: Containers that should still restart.
func reconcileImplicitRestarts(
	log *zerolog.Logger,
	allContainers, containers []types.Container,
	params types.UpdateParams,
) []types.Container {
	graph, err := sorter.NewDependencyGraph(log,
		containers,
		unmonitoredContainers(allContainers, containers),
		params.UseComposeDependsOn,
	)
	if err != nil {
		return nil
	}

	return reconcileImplicitRestartsExcluding(log, graph, allContainers, containers, params, nil)
}
