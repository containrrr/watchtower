package actions

import (
	"math/rand"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/v2tec/watchtower/container"
)

var (
	letters  = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, filter container.Filter, cleanup bool, noRestart bool, timeout time.Duration) error {
	log.Debug("Checking containers for updated images")

	containers, err := client.ListContainers(filter)
	if err != nil {
		return err
	}

	for i, container := range containers {
		stale, err := client.IsContainerStale(container)
		if err != nil {
			log.Infof("Unable to update container %s, err='%s'. Proceeding to next.", containers[i].Name(), err)
			stale = false
		}
		containers[i].Stale = stale
	}

	containers, err = container.SortByDependencies(containers)
	if err != nil {
		return err
	}

	checkDependencies(containers)

	// Stop stale containers in reverse order
	for i := len(containers) - 1; i >= 0; i-- {
		container := containers[i]

		if container.IsWatchtower() {
			continue
		}

		if container.Stale {
			if err := client.StopContainer(container, timeout); err != nil {
				log.Error(err)
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
					continue
				}
			}

			if !noRestart {
				if err := client.StartContainer(container); err != nil {
					log.Error(err)
				}
			}

			if cleanup {
				client.RemoveImage(container)
			}
		}
	}

	return nil
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
