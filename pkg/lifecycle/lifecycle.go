package lifecycle

import (
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// ExecutePreChecks tries to run the pre-check lifecycle hook for all containers included by the current filter.
func ExecutePreChecks(client container.Client, params types.UpdateParams) {
	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return
	}
	for _, currentContainer := range containers {
		ExecutePreCheckCommand(client, currentContainer)
	}
}

// ExecutePostChecks tries to run the post-check lifecycle hook for all containers included by the current filter.
func ExecutePostChecks(client container.Client, params types.UpdateParams) {
	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return
	}
	for _, currentContainer := range containers {
		ExecutePostCheckCommand(client, currentContainer)
	}
}

// ExecutePreCheckCommand tries to run the pre-check lifecycle hook for a single container.
func ExecutePreCheckCommand(client container.Client, container container.Container) {
	command := container.GetLifecyclePreCheckCommand()
	if len(command) == 0 {
		log.Debug("No pre-check command supplied. Skipping")
		return
	}

	log.Debug("Executing pre-check command.")
	_, err := client.ExecuteCommand(container.ID(), command, 1)
	if err != nil {
		log.Error(err)
	}
}

// ExecutePostCheckCommand tries to run the post-check lifecycle hook for a single container.
func ExecutePostCheckCommand(client container.Client, container container.Container) {
	command := container.GetLifecyclePostCheckCommand()
	if len(command) == 0 {
		log.Debug("No post-check command supplied. Skipping")
		return
	}

	log.Debug("Executing post-check command.")
	_, err := client.ExecuteCommand(container.ID(), command, 1)
	if err != nil {
		log.Error(err)
	}
}

// ExecutePreUpdateCommand tries to run the pre-update lifecycle hook for a single container.
func ExecutePreUpdateCommand(client container.Client, container container.Container) (SkipUpdate bool, err error) {
	timeout := container.PreUpdateTimeout()
	command := container.GetLifecyclePreUpdateCommand()
	if len(command) == 0 {
		log.Debug("No pre-update command supplied. Skipping")
		return false, nil
	}

	if !container.IsRunning() || container.IsRestarting() {
		log.Debug("Container is not running. Skipping pre-update command.")
		return false, nil
	}

	log.Debug("Executing pre-update command.")
	return client.ExecuteCommand(container.ID(), command, timeout)
}

// ExecutePostUpdateCommand tries to run the post-update lifecycle hook for a single container.
func ExecutePostUpdateCommand(client container.Client, newContainerID types.ContainerID) {
	newContainer, err := client.GetContainer(newContainerID)
	if err != nil {
		log.Error(err)
		return
	}

	command := newContainer.GetLifecyclePostUpdateCommand()
	if len(command) == 0 {
		log.Debug("No post-update command supplied. Skipping")
		return
	}

	log.Debug("Executing post-update command.")
	_, err = client.ExecuteCommand(newContainerID, command, 1)

	if err != nil {
		log.Error(err)
	}
}
