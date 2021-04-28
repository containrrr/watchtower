package actions

import (
	"fmt"
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) (*metrics.Metric, error) {
	log.Debug("Checking containers for updated images")
	metric := &metrics.Metric{}

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containers, err := ScanForContainerUpdates(client, params, metric)

	// Link all containers that are depended upon
	checkDependencies(containers)

	var containersToUpdate []container.Container
	if !params.MonitorOnly {
		for _, c := range containers {
			if !c.IsMonitorOnly() {
				containersToUpdate = append(containersToUpdate, c)
			}
		}
	}

	undirectedNodes := CreateUndirectedLinks(containersToUpdate)
	updateGraphs, err := sorter.SortByDependencies(containersToUpdate, undirectedNodes)
	if err != nil {
		return nil, err
	}

	imageIDs := make(map[string]bool)
	for _, graphContainers := range updateGraphs {
		if err := ensureUpdateAllowedByLabels(graphContainers); err != nil {
			log.Error(err)
			metric.Failed += len(graphContainers)
			continue
		}
		metric.Failed += stopContainersInReversedOrder(graphContainers, client, params)
		metric.Failed += restartContainersInSortedOrder(graphContainers, client, params, imageIDs)
	}

	metric.Updated = metric.StaleCount - (metric.Failed - metric.StaleCheckFailed)

	if params.Cleanup {
		cleanupImages(client, imageIDs)
	}

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}
	return metric, nil
}

func ScanForContainerUpdates(client container.Client, params types.UpdateParams, metric *metrics.Metric) ([]container.Container, error) {

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return nil, err
	}

	for i, targetContainer := range containers {
		stale, err := client.IsContainerStale(targetContainer)
		shouldUpdate := stale && !params.NoRestart && !params.MonitorOnly && !targetContainer.IsMonitorOnly()
		if err == nil && shouldUpdate {
			// Check to make sure we have all the necessary information for recreating the container
			err = targetContainer.VerifyConfiguration()
			// If the image information is incomplete and trace logging is enabled, log it for further diagnosis
			if err != nil && log.IsLevelEnabled(log.TraceLevel) {
				imageInfo := targetContainer.ImageInfo()
				log.Tracef("Image info: %#v", imageInfo)
				log.Tracef("Container info: %#v", targetContainer.ContainerInfo())
				if imageInfo != nil {
					log.Tracef("Image config: %#v", imageInfo.Config)
				}
			}
		}

		if err != nil {
			log.Infof("Unable to update container %q: %v. Proceeding to next.", targetContainer.Name(), err)
			stale = false
			metric.StaleCheckFailed++
			metric.Failed++
		}
		containers[i].Stale = stale

		if stale {
			metric.StaleCount++
		}
	}

	metric.Scanned = len(containers)

	// Update linkedToRestarting property for dependent containers
	checkDependencies(containers)

	return containers, nil
}

func stopContainersInReversedOrder(containers []container.Container, client container.Client, params types.UpdateParams) int {
	failed := 0
	for i := len(containers) - 1; i >= 0; i-- {
		if err := stopStaleContainer(containers[i], client, params); err != nil {
			failed++
		}
	}
	return failed
}

func stopStaleContainer(container container.Container, client container.Client, params types.UpdateParams) error {
	if container.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", container.Name())
		return nil
	}

	if !container.ToRestart() {
		return nil
	}
	if params.LifecycleHooks {
		if err := lifecycle.ExecutePreUpdateCommand(client, container); err != nil {
			log.Error(err)
			log.Info("Skipping container as the pre-update command failed")
			return err
		}
	}

	if err := client.StopContainer(container, params.Timeout); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func restartContainersInSortedOrder(containers []container.Container, client container.Client, params types.UpdateParams, imageIDs map[string]bool) int {

	failed := 0

	for _, c := range containers {
		if !c.ToRestart() {
			continue
		}
		if err := restartStaleContainer(c, client, params); err != nil {
			failed++
		}
		imageIDs[c.ImageID()] = true
	}

	return failed
}

func cleanupImages(client container.Client, imageIDs map[string]bool) {
	for imageID := range imageIDs {
		if err := client.RemoveImageByID(imageID); err != nil {
			log.Error(err)
		}
	}
}

func restartStaleContainer(container container.Container, client container.Client, params types.UpdateParams) error {
	// Since we can't shutdown a watchtower container immediately, we need to
	// start the new one while the old one is still running. This prevents us
	// from re-using the same container name so we first rename the current
	// instance so that the new one can adopt the old name.
	if container.IsWatchtower() {
		if err := client.RenameContainer(container, util.RandName()); err != nil {
			log.Error(err)
			return nil
		}
	}

	if !params.NoRestart {
		if newContainerID, err := client.StartContainer(container); err != nil {
			log.Error(err)
			return err
		} else if container.ToRestart() && params.LifecycleHooks {
			lifecycle.ExecutePostUpdateCommand(client, newContainerID)
		}
	}
	return nil
}

func checkDependencies(containers []container.Container) {

	// Build hash lookup map
	lookup := make(map[string]*container.Container, len(containers))
	for _, c := range containers {
		lookup[c.Name()] = &c
	}

	for _, c := range containers {
		if c.ToRestart() {
			continue
		}
		for _, linkName := range c.Links() {
			if lookup[linkName].ToRestart() {
				c.LinkedToRestarting = true
				break
			}
		}
	}
}

func ensureUpdateAllowedByLabels(updateGraph []container.Container) error {
	for _, c := range updateGraph {
		if c.ToRestart() && c.IsMonitorOnly() {
			if c.LinkedToRestarting {
				return fmt.Errorf("container %q needs to be restarted to satisfy a linked dependency, but it is set to monitor-only", c.Name())
			}
			// return fmt.Errorf("container %q needs to be restarted to satisfy a linked dependency, but it is set to monitor-only", c.Name())
		}
	}
	return nil
}

// CreateUndirectedLinks creates a map of undirected links
// Key: Name of a container
// Value: List of containers that are linked to the container
// i.e if Container A depends on B, undirectedNodes['A'] will initially contain B.
// This function adds 'A' into undirectedNodes['B'] to make the link undirected.
func CreateUndirectedLinks(containers []container.Container) map[string][]string {

	undirectedNodes := make(map[string][]string)
	for i := 0; i < len(containers); i++ {
		undirectedNodes[containers[i].Name()] = containers[i].Links()
	}

	for i := 0; i < len(containers); i++ {
		for j := 0; j < len(containers[i].Links()); j++ {
			undirectedNodes[containers[i].Links()[j]] = append(undirectedNodes[containers[i].Links()[j]], containers[i].Name())
		}
	}

	return undirectedNodes
}
