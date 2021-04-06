package actions

import (
	"errors"
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	metrics2 "github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) (*metrics2.Metric, error) {
	log.Debug("Checking containers for updated images")
	metric := &metrics2.Metric{}
	staleCount := 0

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return nil, err
	}

	staleCheckFailed := 0

	for i, targetContainer := range containers {
		stale, err := client.IsContainerStale(targetContainer)
		if stale && !params.NoRestart && !params.MonitorOnly && !targetContainer.IsMonitorOnly() && !targetContainer.HasImageInfo() {
			err = errors.New("no available image info")
		}
		if err != nil {
			log.Infof("Unable to update container %q: %v. Proceeding to next.", containers[i].Name(), err)
			stale = false
			staleCheckFailed++
			metric.Failed++
		}
		containers[i].Stale = stale

		if stale {
			staleCount++
		}
	}

	containers, err = sorter.SortByDependencies(containers)
	metric.Scanned = len(containers)
	if err != nil {
		return nil, err
	}

	checkDependencies(containers)

	containersToUpdate := []container.Container{}
	if !params.MonitorOnly {
		for i := len(containers) - 1; i >= 0; i-- {
			if !containers[i].IsMonitorOnly() {
				containersToUpdate = append(containersToUpdate, containers[i])
			}
		}
	}

	if params.RollingRestart {
		metric.Failed += performRollingRestart(containersToUpdate, client, params)
	} else {
		imageIDsOfStoppedContainers := make(map[string]bool)
		metric.Failed,imageIDsOfStoppedContainers = stopContainersInReversedOrder(containersToUpdate, client, params)
		metric.Failed += restartContainersInSortedOrder(containersToUpdate, client, params, imageIDsOfStoppedContainers)
	}

	metric.Updated = staleCount - (metric.Failed - staleCheckFailed)

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}
	return metric, nil
}

func performRollingRestart(containers []container.Container, client container.Client, params types.UpdateParams) int {
	cleanupImageIDs := make(map[string]bool)
	failed := 0

	for i := len(containers) - 1; i >= 0; i-- {
		if containers[i].Stale {
			err := stopStaleContainer(containers[i], client, params)
			if err != nil {
				failed++
			} else {
				if err := restartStaleContainer(containers[i], client, params); err != nil {
					failed++
				}
				cleanupImageIDs[containers[i].ImageID()] = true
			}
		}
	}

	if params.Cleanup {
		cleanupImages(client, cleanupImageIDs)
	}
	return failed
}

func stopContainersInReversedOrder(containers []container.Container, client container.Client, params types.UpdateParams) (int, map[string]bool) {
	imageIDsOfStoppedContainers := make(map[string]bool)
	failed := 0
	for i := len(containers) - 1; i >= 0; i-- {
		if err := stopStaleContainer(containers[i], client, params); err != nil {
			failed++
		} else {
			imageIDsOfStoppedContainers[containers[i].ImageID()] = true
		}
		
	}
	return failed, imageIDsOfStoppedContainers
}

func stopStaleContainer(container container.Container, client container.Client, params types.UpdateParams) error {
	if container.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", container.Name())
		return nil
	}

	if !container.Stale {
		return nil
	}
	if params.LifecycleHooks {
		SkipUpdate,err := lifecycle.ExecutePreUpdateCommand(client, container)
		if  err != nil {
			log.Error(err)
			log.Info("Skipping container as the pre-update command failed")
			return err
		}
		if SkipUpdate {
			log.Debug("Skipping container as the pre-update command returned exit code 75 (EX_TEMPFAIL)")
			return errors.New("Skipping container as the pre-update command returned exit code 75 (EX_TEMPFAIL)")
		}
	}

	if err := client.StopContainer(container, params.Timeout); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func restartContainersInSortedOrder(containers []container.Container, client container.Client, params types.UpdateParams, imageIDsOfStoppedContainers map[string]bool) int {
	imageIDs := make(map[string]bool)

	failed := 0

	for _, c := range containers {
		if !c.Stale {
			continue
		}
		if imageIDsOfStoppedContainers[c.ImageID()] {
			if err := restartStaleContainer(c, client, params); err != nil {
				failed++
			}
			imageIDs[c.ImageID()] = true
		}
	}

	if params.Cleanup {
		cleanupImages(client, imageIDs)
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
		} else if container.Stale && params.LifecycleHooks {
			lifecycle.ExecutePostUpdateCommand(client, newContainerID)
		}
	}
	return nil
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
