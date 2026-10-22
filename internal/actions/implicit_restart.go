package actions

import (
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// markImplicitRestarts marks each container that depends on a restarting
// container, directly or through other marked containers, as linked to a
// restarting container.
//
// Dependencies are the links of the scan's dependency graph, so a link names
// the same container here as when the containers are sorted. A container that
// is already restarting is not marked again.
//
// Parameters:
//   - log: Process logger.
//   - graph: The scan's dependency graph.
//   - allContainers: Every listed container. Those restarting start the marking.
//   - containers: Containers that may be marked.
//   - excluded: Containers that are neither marked nor treated as restarting.
//
// This function mutates the LinkedToRestarting state of containers in place.
func markImplicitRestarts(
	log *zerolog.Logger,
	graph *sorter.DependencyGraph,
	allContainers, containers []types.Container,
	excluded map[types.ContainerID]struct{},
) {
	eligible := make(map[types.ContainerID]bool, len(containers))

	for _, c := range containers {
		if _, skip := excluded[c.ID()]; !skip {
			eligible[c.ID()] = true
		}
	}

	queue := make([]types.Container, 0, len(allContainers))

	for _, c := range allContainers {
		if _, skip := excluded[c.ID()]; !skip && c.ToRestart() {
			queue = append(queue, c)
		}
	}

	for len(queue) > 0 {
		restarting := queue[0]
		queue = queue[1:]

		for _, dependent := range graph.Dependents(restarting) {
			if !eligible[dependent.ID()] || dependent.ToRestart() {
				continue
			}

			dependent.SetLinkedToRestarting(true)
			queue = append(queue, dependent)

			log.Debug().
				Str("container", dependent.Name()).
				Str("restarting", restarting.Name()).
				Msg("Marked container as linked to restarting")
		}
	}
}

// unmonitoredContainers returns the containers of allContainers that are not
// among the monitored containers.
//
// Parameters:
//   - allContainers: Every listed container.
//   - monitored: The monitored containers.
//
// Returns:
//   - []types.Container: The other containers, in listed order.
func unmonitoredContainers(allContainers, monitored []types.Container) []types.Container {
	isMonitored := make(map[types.ContainerID]bool, len(monitored))
	for _, c := range monitored {
		isMonitored[c.ID()] = true
	}

	unmonitored := make([]types.Container, 0, max(len(allContainers)-len(monitored), 0))

	for _, c := range allContainers {
		if !isMonitored[c.ID()] {
			unmonitored = append(unmonitored, c)
		}
	}

	return unmonitored
}
