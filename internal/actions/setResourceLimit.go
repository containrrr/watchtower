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
	log.Debug("Checking if we should set the resource limit ...")
	if params.MaxMemoryPerContainer > 0 {
		containers, err := client.ListContainers(params.Filter)
		if err != nil {
			return err
		}

		for _, container := range containers {
			if _, error := client.SetMaxMemoryLimit(container, params.MaxMemoryPerContainer); error != nil {
				log.Errorf("Error while setting the resource limit. Detail: %s", error)
			}
		}
		return nil
	}
	log.Debug("MaxMemoryPerContainer is 0. Therefore no action necessary")
	return nil
}
