package actions

import (
	"sort"

	"github.com/v2tec/watchtower/container"
)

// CheckPrereqs will ensure that there are not multiple instances of the
// watchtower running simultaneously. If multiple watchtower containers are
// detected, this function will stop and remove all but the most recently
// started container.
func CheckPrereqs(client container.Client, cleanup bool) error {
	containers, err := client.ListContainers(container.WatchtowerContainersFilter)
	if err != nil {
		return err
	}

	if len(containers) > 1 {
		sort.Sort(container.ByCreated(containers))

		// Iterate over all containers execept the last one
		for _, c := range containers[0 : len(containers)-1] {
			client.StopContainer(c, 60)

			if cleanup {
				client.RemoveImage(c)
			}
		}
	}

	return nil
}
