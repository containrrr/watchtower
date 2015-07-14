package updater

import (
	"github.com/CenturyLinkLabs/watchtower/docker"
)

func Run() error {
	client := docker.NewClient()
	containers, err := client.ListContainers()
	if err != nil {
		return err
	}

	for i := range containers {
		if err := client.RefreshImage(&containers[i]); err != nil {
			return err
		}
	}

	containers, err = sortContainers(containers)
	if err != nil {
		return err
	}

	checkDependencies(containers)

	// Stop stale containers in reverse order
	for i := len(containers) - 1; i >= 0; i-- {
		container := containers[i]
		if container.Stale {
			if err := client.Stop(container); err != nil {
				return err
			}
		}
	}

	// Restart stale containers in sorted order
	for _, container := range containers {
		if container.Stale {
			if err := client.Start(container); err != nil {
				return err
			}
		}
	}

	return nil
}

func sortContainers(containers []docker.Container) ([]docker.Container, error) {
	sorter := ContainerSorter{}
	return sorter.Sort(containers)
}

func checkDependencies(containers []docker.Container) {

	for i, parent := range containers {
		if parent.Stale {
			continue
		}

	LinkLoop:
		for _, linkName := range parent.Links() {
			for _, child := range containers {
				if child.Name() == linkName && child.Stale {
					containers[i].Stale = true
					break LinkLoop
				}
			}
		}
	}
}
