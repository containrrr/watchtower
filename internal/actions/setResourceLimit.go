package actions

import (
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
func SetResourceLimit(client container.Client, params types.UpdateParams) error {
	log.Debug("Checking if resource limit needs to be set ...")

	//check the flag to know if we need to do something at all
	// types.resourcelimit active

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return err
	}

	for i, container := range containers {
		// check if memory needs to be set
		needRestart, _ := client.SetMaxMemoryLimit(container, params.MaxMemoryPerContainer)
		// mark container for restart
		containers[i].NeedUpdate = needRestart
	}

	containers, err = sorter.SortByDependencies(containers)
	if err != nil {
		return err
	}

	CheckDependencies(containers)

	StopContainersInReversedOrder(containers, client, params)
	RestartContainersInSortedOrder(containers, client, params)

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}
	return nil
}
