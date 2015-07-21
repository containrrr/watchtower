package actions

import (
	"sort"

	"github.com/CenturyLinkLabs/watchtower/container"
)

func watchtowerContainersFilter(c container.Container) bool { return c.IsWatchtower() }

func CheckPrereqs(client container.Client) error {
	containers, err := client.ListContainers(watchtowerContainersFilter)
	if err != nil {
		return err
	}

	if len(containers) > 1 {
		sort.Sort(container.ByCreated(containers))

		// Iterate over all containers execept the last one
		for _, c := range containers[0 : len(containers)-1] {
			client.StopContainer(c, 60)
		}
	}

	return nil
}
