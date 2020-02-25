package actions

import (
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

func StopContainersInReversedOrder(containers []container.Container, client container.Client, params types.UpdateParams) {
	for i := len(containers) - 1; i >= 0; i-- {
		StopContainer(containers[i], client, params)
	}
}

func StopContainer(container container.Container, client container.Client, params types.UpdateParams) {
	if container.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", container.Name())
		return
	}

	if !container.Stale && !container.NeedUpdate {
		return
	}

	if params.LifecycleHooks {
		lifecycle.ExecutePreUpdateCommand(client, container)

	}

	if err := client.StopContainer(container, params.Timeout); err != nil {
		log.Error(err)
	}
}

func RestartContainersInSortedOrder(containers []container.Container, client container.Client, params types.UpdateParams) {
	imageIDs := make(map[string]bool)

	for _, container := range containers {
		if !container.Stale && !container.NeedUpdate {
			continue
		}
		RestartContainer(container, client, params)
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

func RestartContainer(container container.Container, client container.Client, params types.UpdateParams) {
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

func CheckDependencies(containers []container.Container) {

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
