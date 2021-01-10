package actions

import (
	"fmt"
	"sort"
	"time"

	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/sirupsen/logrus"

	log "github.com/sirupsen/logrus"

	"github.com/containrrr/watchtower/pkg/container"
)

// CheckForMultipleWatchtowerInstances will ensure that there are not multiple instances of the
// watchtower running simultaneously. If multiple watchtower containers are detected, this function
// will stop and remove all but the most recently started container. This behaviour can be bypassed
// if a scope UID is defined.
func CheckForMultipleWatchtowerInstances(client container.Client, cleanup bool, scope string) error {
	awaitDockerClient()
	containers, err := client.ListContainers(filters.FilterByScope(scope, filters.WatchtowerContainersFilter))

	if err != nil {
		log.Fatal(err)
		return err
	}

	if len(containers) <= 1 {
		log.Debug("There are no additional watchtower containers")
		return nil
	}

	log.Info("Found multiple running watchtower instances. Cleaning up.")
	return cleanupExcessWatchtowers(containers, client, cleanup)
}

func cleanupExcessWatchtowers(containers []container.Container, client container.Client, cleanup bool) error {
	var stopErrors int

	sort.Sort(sorter.ByCreated(containers))
	allContainersExceptLast := containers[0 : len(containers)-1]

	for _, c := range allContainersExceptLast {
		if err := client.StopContainer(c, 10*time.Minute); err != nil {
			// logging the original here as we're just returning a count
			logrus.WithError(err).Error("Could not stop a previous watchtower instance.")
			stopErrors++
			continue
		}

		if cleanup {
			if err := client.RemoveImageByID(c.ImageID()); err != nil {
				logrus.WithError(err).Warning("Could not cleanup watchtower images, possibly because of other watchtowers instances in other scopes.")
			}
		}
	}

	if stopErrors > 0 {
		return fmt.Errorf("%d errors while stopping watchtower containers", stopErrors)
	}

	return nil
}

func awaitDockerClient() {
	log.Debug("Sleeping for a second to ensure the docker api client has been properly initialized.")
	time.Sleep(1 * time.Second)
}
