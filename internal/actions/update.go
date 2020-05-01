package actions

import (
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) error {
	log.Debug("Checking containers for updated images")

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return err
	}

	for i, container := range containers {
		stale, err := client.IsContainerStale(container)
		if err != nil {
			log.Infof("Unable to update container %s. Proceeding to next.", containers[i].Name())
			log.Debug(err)
			stale = false
		}
		containers[i].Stale = stale
	}

	containers, err = sorter.SortByDependencies(containers)
	if err != nil {
		return err
	}

	checkDependencies(containers)

	if params.MonitorOnly {
		if params.LifecycleHooks {
			lifecycle.ExecutePostChecks(client, params)
		}
		return nil
	}

	stopContainersInReversedOrder(containers, client, params)
	restartContainersInSortedOrder(containers, client, params)

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}
	return nil
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

func restartContainersInSortedOrder(containers []container.Container, client container.Client, params types.UpdateParams) {
	imageIDs := make(map[string]bool)

	for _, container := range containers {
		if !container.Stale {
			continue
		}
		restartStaleContainer(container, client, params)
		imageIDs[container.ImageID()] = true
	}
	if params.Cleanup {
		for imageID := range imageIDs {
			if err := client.RemoveImageByID(imageID); err != nil {
				log.Error(err)
			}
		}
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
