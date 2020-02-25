package actions

import (
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// SetResourceLimit looks at the running Docker containers to see if any of them needs to have a resource limitation
// A container needs resource limitation if it does not define a memory limit or
// it has a memory limit greater than the configured (default or as command argument) limit
func SetResourceLimit(client container.Client, params types.UpdateParams) error {
	log.Debug("Checking if resource limit needs to be set ...")

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return err
	}
	if !(len(containers) > 0) {
		log.Infof("No container found!!!")
		return nil
	}

	// check if memory needs to be set
	for _, container := range containers {
		log.Infof("SetMaxMemoryLimit ---")
		error := client.SetMaxMemoryLimit(container, params.MaxMemoryPerContainer)
		if error != nil {
			log.Infof("Error while setting the resource limit. Detail: %s", err)
		}
	}
	return nil
}
