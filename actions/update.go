package actions

import (
	"fmt"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/v2tec/watchtower/container"
)

var (
	letters  = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	waitTime = 10 * time.Second
)

func allContainersFilter(container.Container) bool { return true }

func containerFilter(names []string) container.Filter {
	if len(names) == 0 {
		return allContainersFilter
	}

	return func(c container.Container) bool {
		for _, name := range names {
			if (name == c.Name()) || (name == c.Name()[1:]) {
				return true
			}
		}
		return false
	}
}

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, names []string, cleanup bool, noRestart bool) ([]string, []string, error) {
	log.Info("Checking containers for updated images")

	// helper vars for notification
	var (
		updatedContainers []string
		errorMessages     []string
	)

	containers, err := client.ListContainers(containerFilter(names))
	if err != nil {
		return updatedContainers, errorMessages, err
	}

	for i, container := range containers {
		stale, err := client.IsContainerStale(container)
		if err != nil {
			log.Infof("Unable to update container %s. Proceeding to next.", containers[i].Name())
			log.Debug(err)
			stale = false
			errorMessages = append(errorMessages, fmt.Sprintf("Unable to update container %s: %v", container.Details(), err))
		}
		containers[i].Stale = stale
	}

	containers, err = container.SortByDependencies(containers)
	if err != nil {
		return updatedContainers, errorMessages, err
	}

	checkDependencies(containers)

	// Stop stale containers in reverse order
	for i := len(containers) - 1; i >= 0; i-- {
		container := containers[i]

		if container.IsWatchtower() {
			continue
		}

		if container.Stale {
			if err := client.StopContainer(container, waitTime); err != nil {
				log.Error(err)
				errorMessages = append(errorMessages, fmt.Sprintf("Unable to stop container %s: %v", container.Details(), err))
			}
		}
	}

	// Restart stale containers in sorted order
	for _, container := range containers {
		if container.Stale {
			// Since we can't shutdown a watchtower container immediately, we need to
			// start the new one while the old one is still running. This prevents us
			// from re-using the same container name so we first rename the current
			// instance so that the new one can adopt the old name.
			if container.IsWatchtower() {
				if err := client.RenameContainer(container, randName()); err != nil {
					log.Error(err)
					errorMessages = append(errorMessages, fmt.Sprintf("Unable to rename container %s: %v", container.Details(), err))
					continue
				}
			}

			if !noRestart {
				if err := client.StartContainer(container); err != nil {
					log.Error(err)
					errorMessages = append(errorMessages, fmt.Sprintf("Unable to restart container %s: %v", container.Details(), err))
				}
			}

			if cleanup {
				client.RemoveImage(container)
			}

			updatedContainers = append(updatedContainers, container.Details())
		}
	}

	return updatedContainers, errorMessages, nil
}

func checkDependencies(containers []container.Container) {

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

// Generates a random, 32-character, Docker-compatible container name.
func randName() string {
	b := make([]rune, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
