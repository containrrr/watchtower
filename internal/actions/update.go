package actions

import (
	"errors"
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// CreateUndirectedLinks creates a map of undirected links
// Key: Name of a container
// Value: List of containers that are linked to the container
// i.e if Container A depends on B, undirectedNodes['A'] will initially contain B.
// This function adds 'A' into undirectedNodes['B'] to make the link undirected.
func CreateUndirectedLinks(containers []container.Container) map[string][]string {

	undirectedNodes := make(map[string][]string)
	for i:= 0; i < len(containers); i++ {
		undirectedNodes[containers[i].Name()] = containers[i].Links()
	}

	for i:= 0; i< len(containers); i++ {
		for j:=0; j < len(containers[i].Links()); j++ {
			undirectedNodes[containers[i].Links()[j]] = append(undirectedNodes[containers[i].Links()[j]], containers[i].Name())
		}
	}

	return undirectedNodes;
}

// PrepareContainerList prepares a dependency sorted list of list of containers
// Each list inside the outer list contains containers that are related by links
// This method checks for staleness, checks dependencies, sorts the containers and returns the final
// [][]container.Container
func PrepareContainerList(client container.Client, params types.UpdateParams) ([][]container.Container, error) {

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return nil, err
	}

	for i, targetContainer := range containers {
		stale, err := client.IsContainerStale(targetContainer)
		if stale && !params.NoRestart && !params.MonitorOnly && !targetContainer.IsMonitorOnly() && !targetContainer.HasImageInfo() {
			err = errors.New("no available image info")
		}
		if err != nil {
			log.Infof("Unable to update container %q: %v. Proceeding to next.", containers[i].Name(), err)
			stale = false
		}
		containers[i].Stale = stale
	}

	checkDependencies(containers)

	var dependencySortedGraphs [][]container.Container

	undirectedNodes := CreateUndirectedLinks(containers)
	dependencySortedGraphs, err = sorter.SortByDependencies(containers,undirectedNodes)

	if err != nil {
		return nil, err
	}

	return dependencySortedGraphs, nil
}

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) error {
	log.Debug("Checking containers for updated images")

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containersToUpdate := []container.Container{}
	if !params.MonitorOnly {
		for i := len(containers) - 1; i >= 0; i-- {
			if !containers[i].IsMonitorOnly() {
				containersToUpdate = append(containersToUpdate, containers[i])
			}
		}
	}

	//shared map for independent and linked update
	imageIDs := make(map[string]bool)

	dependencySortedGraphs, err := PrepareContainerList(client, params)
	if err != nil {
		return err
	}

	//Use ordered start and stop for each independent set of containers
	for _, dependencyGraph:= range dependencySortedGraphs {
		stopContainersInReversedOrder(dependencyGraph, client, params)
		restartContainersInSortedOrder(dependencyGraph, client, params, imageIDs)
	}

	//clean up outside after containers updated
	if params.Cleanup {
		for imageID := range imageIDs {
			if err := client.RemoveImageByID(imageID); err != nil {
				log.Error(err)
			}
		}
	}

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}

	return nil
}

func performRollingRestart(containers []container.Container, client container.Client, params types.UpdateParams) {
	cleanupImageIDs := make(map[string]bool)

	for i := len(containers) - 1; i >= 0; i-- {
		if containers[i].Stale {
			stopStaleContainer(containers[i], client, params)
			restartStaleContainer(containers[i], client, params)
		}
	}

	if params.Cleanup {
		cleanupImages(client, cleanupImageIDs)
	}
}

func stopContainersInReversedOrder(containers []container.Container, client container.Client, params types.UpdateParams) {
	for i := len(containers) - 1; i >= 0; i-- {
		stopStaleContainer(containers[i], client, params)
	}
}

func stopStaleContainer(container container.Container, client container.Client, params types.UpdateParams) {
	if container.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", container.Name())
		return
	}

	if !container.Stale {
		return
	}
	if params.LifecycleHooks {
		if err := lifecycle.ExecutePreUpdateCommand(client, container); err != nil {
			log.Error(err)
			log.Info("Skipping container as the pre-update command failed")
			return
		}
	}

	if err := client.StopContainer(container, params.Timeout); err != nil {
		log.Error(err)
	}
}

func restartContainersInSortedOrder(containers []container.Container, client container.Client, params types.UpdateParams, imageIDs map[string]bool) {
	for _, container := range containers {
		if !container.Stale {
			continue
		}
		restartStaleContainer(staleContainer, client, params)
		imageIDs[staleContainer.ImageID()] = true
	}
}

func restartStaleContainer(container container.Container, client container.Client, params types.UpdateParams) {
	// Since we can't shutdown a watchtower container immediately, we need to
	// start the new one while the old one is still running. This prevents us
	// from re-using the same container name so we first rename the current
	// instance so that the new one can adopt the old name.
	if container.IsWatchtower() {
		if err := client.RenameContainer(container, util.RandName()); err != nil {
			log.Error(err)
			return
		}
	}

	if !params.NoRestart {
		if newContainerID, err := client.StartContainer(container); err != nil {
			log.Error(err)
		} else if container.Stale && params.LifecycleHooks {
			lifecycle.ExecutePostUpdateCommand(client, newContainerID)
		}
	}
}

func checkDependencies(containers []container.Container) {

	for i, parent := range containers {
		if parent.ToRestart() {
			continue
		}

	LinkLoop:
		for _, linkName := range parent.Links() {
			for _, child := range containers {
				if child.Name() == linkName && child.ToRestart() {
					containers[i].Linked = true
					break LinkLoop
				}
			}
		}
	}
}