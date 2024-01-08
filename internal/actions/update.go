package actions

import (
	"errors"
	"time"

	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/session"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) (types.Report, error) {
	log.Debug("Checking containers for updated images")
	progress := &session.Progress{}
	staleCount := 0

	if params.LifecycleHooks {
		lifecycle.ExecutePreChecks(client, params)
	}

	containers, err := client.ListContainers(params.Filter)
	if err != nil {
		return nil, err
	}

	staleCheckFailed := 0

	for i, targetContainer := range containers {
		// stale will be true if there is a more recent image than the current container is using
		stale, newestImage, err := client.IsContainerStale(targetContainer, params)
		shouldUpdate := stale && !params.NoRestart && !targetContainer.IsMonitorOnly(params)
		imageUpdateDeferred := false
		imageAgeDays := 0
		if err == nil && shouldUpdate {
			// Check to make sure we have all the necessary information for recreating the container, including ImageInfo
			err = targetContainer.VerifyConfiguration()
			if err == nil {
				if params.DeferDays > 0 {
					imageAgeDays, imageErr := getImageAgeDays(targetContainer.ImageInfo().Created)
					err = imageErr
					if err == nil {
						imageUpdateDeferred = imageAgeDays < params.DeferDays
					}
				}
			} else if log.IsLevelEnabled(log.TraceLevel) {
				// If the image information is incomplete and trace logging is enabled, log it for further diagnosis
				imageInfo := targetContainer.ImageInfo()
				log.Tracef("Image info: %#v", imageInfo)
				log.Tracef("Container info: %#v", targetContainer.ContainerInfo())
				if imageInfo != nil {
					log.Tracef("Image config: %#v", imageInfo.Config)
				}
			}
		}

		if err != nil {
			log.Infof("Unable to update container %q: %v. Proceeding to next.", targetContainer.Name(), err)
			stale = false
			staleCheckFailed++
			progress.AddSkipped(targetContainer, err)
		} else if imageUpdateDeferred {
			log.Infof("New image found for %s that was created %d day(s) ago but update deferred until %d day(s) after creation", targetContainer.Name(), imageAgeDays, params.DeferDays)
			// technically the container is stale but we set it to false here because it is this stale flag that tells downstream methods whether to perform the update
			stale = false
			progress.AddScanned(targetContainer, newestImage)
			progress.MarkDeferred(targetContainer.ID())
		} else {
			progress.AddScanned(targetContainer, newestImage)
		}
		containers[i].SetStale(stale)

		if stale {
			staleCount++
		}
	}

	containers, err = sorter.SortByDependencies(containers)
	if err != nil {
		return nil, err
	}

	UpdateImplicitRestart(containers)

	// containersToUpdate will contain all containers, not just those that need to be updated. The "stale" flag is checked via container.ToRestart()
	// within stopContainersInReversedOrder and restartContainersInSortedOrder to skip containers with stale set to false (unless LinkedToRestarting set)
	// NOTE: This logic is changing with latest PR on main repo
	var containersToUpdate []types.Container
	for _, c := range containers {
		// pulling this change in from PR 1895 for now to avoid updating status incorrectly
		if c.ToRestart() && !c.IsMonitorOnly(params) {
			containersToUpdate = append(containersToUpdate, c)
			progress.MarkForUpdate(c.ID())
		}
	}

	if params.RollingRestart {
		progress.UpdateFailed(performRollingRestart(containersToUpdate, client, params))
	} else {
		failedStop, stoppedImages := stopContainersInReversedOrder(containersToUpdate, client, params)
		progress.UpdateFailed(failedStop)
		failedStart := restartContainersInSortedOrder(containersToUpdate, client, params, stoppedImages)
		progress.UpdateFailed(failedStart)
	}

	if params.LifecycleHooks {
		lifecycle.ExecutePostChecks(client, params)
	}
	return progress.Report(), nil
}

func performRollingRestart(containers []types.Container, client container.Client, params types.UpdateParams) map[types.ContainerID]error {
	cleanupImageIDs := make(map[types.ImageID]bool, len(containers))
	failed := make(map[types.ContainerID]error, len(containers))

	for i := len(containers) - 1; i >= 0; i-- {
		if containers[i].ToRestart() {
			err := stopStaleContainer(containers[i], client, params)
			if err != nil {
				failed[containers[i].ID()] = err
			} else {
				if err := restartStaleContainer(containers[i], client, params); err != nil {
					failed[containers[i].ID()] = err
				} else if containers[i].IsStale() {
					// Only add (previously) stale containers' images to cleanup
					cleanupImageIDs[containers[i].ImageID()] = true
				}
			}
		}
	}

	if params.Cleanup {
		cleanupImages(client, cleanupImageIDs)
	}
	return failed
}

func stopContainersInReversedOrder(containers []types.Container, client container.Client, params types.UpdateParams) (failed map[types.ContainerID]error, stopped map[types.ImageID]bool) {
	failed = make(map[types.ContainerID]error, len(containers))
	stopped = make(map[types.ImageID]bool, len(containers))
	for i := len(containers) - 1; i >= 0; i-- {
		if err := stopStaleContainer(containers[i], client, params); err != nil {
			failed[containers[i].ID()] = err
		} else {
			// NOTE: If a container is restarted due to a dependency this might be empty
			stopped[containers[i].SafeImageID()] = true
		}

	}
	return
}

func stopStaleContainer(container types.Container, client container.Client, params types.UpdateParams) error {
	if container.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", container.Name())
		return nil
	}

	if !container.ToRestart() {
		return nil
	}

	// Perform an additional check here to prevent us from stopping a linked container we cannot restart
	if container.IsLinkedToRestarting() {
		if err := container.VerifyConfiguration(); err != nil {
			return err
		}
	}

	if params.LifecycleHooks {
		skipUpdate, err := lifecycle.ExecutePreUpdateCommand(client, container)
		if err != nil {
			log.Error(err)
			log.Info("Skipping container as the pre-update command failed")
			return err
		}
		if skipUpdate {
			log.Debug("Skipping container as the pre-update command returned exit code 75 (EX_TEMPFAIL)")
			return errors.New("skipping container as the pre-update command returned exit code 75 (EX_TEMPFAIL)")
		}
	}

	if err := client.StopContainer(container, params.Timeout); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func restartContainersInSortedOrder(containers []types.Container, client container.Client, params types.UpdateParams, stoppedImages map[types.ImageID]bool) map[types.ContainerID]error {
	cleanupImageIDs := make(map[types.ImageID]bool, len(containers))
	failed := make(map[types.ContainerID]error, len(containers))

	for _, c := range containers {
		if !c.ToRestart() {
			continue
		}
		if stoppedImages[c.SafeImageID()] {
			if err := restartStaleContainer(c, client, params); err != nil {
				failed[c.ID()] = err
			} else if c.IsStale() {
				// Only add (previously) stale containers' images to cleanup
				cleanupImageIDs[c.ImageID()] = true
			}
		}
	}

	if params.Cleanup {
		cleanupImages(client, cleanupImageIDs)
	}

	return failed
}

func cleanupImages(client container.Client, imageIDs map[types.ImageID]bool) {
	for imageID := range imageIDs {
		if imageID == "" {
			continue
		}
		if err := client.RemoveImageByID(imageID); err != nil {
			log.Error(err)
		}
	}
}

func restartStaleContainer(container types.Container, client container.Client, params types.UpdateParams) error {
	// Since we can't shutdown a watchtower container immediately, we need to
	// start the new one while the old one is still running. This prevents us
	// from re-using the same container name so we first rename the current
	// instance so that the new one can adopt the old name.
	if container.IsWatchtower() {
		if err := client.RenameContainer(container, util.RandName()); err != nil {
			log.Error(err)
			return nil
		}
	}

	if !params.NoRestart {
		if newContainerID, err := client.StartContainer(container); err != nil {
			log.Error(err)
			return err
		} else if container.ToRestart() && params.LifecycleHooks {
			lifecycle.ExecutePostUpdateCommand(client, newContainerID)
		}
	}
	return nil
}

// UpdateImplicitRestart iterates through the passed containers, setting the
// `LinkedToRestarting` flag if any of it's linked containers are marked for restart
func UpdateImplicitRestart(containers []types.Container) {

	for ci, c := range containers {
		if c.ToRestart() {
			// The container is already marked for restart, no need to check
			continue
		}

		if link := linkedContainerMarkedForRestart(c.Links(), containers); link != "" {
			log.WithFields(log.Fields{
				"restarting": link,
				"linked":     c.Name(),
			}).Debug("container is linked to restarting")
			// NOTE: To mutate the array, the `c` variable cannot be used as it's a copy
			containers[ci].SetLinkedToRestarting(true)
		}

	}
}

// linkedContainerMarkedForRestart returns the name of the first link that matches a
// container marked for restart
func linkedContainerMarkedForRestart(links []string, containers []types.Container) string {
	for _, linkName := range links {
		for _, candidate := range containers {
			if candidate.Name() == linkName && candidate.ToRestart() {
				return linkName
			}
		}
	}
	return ""
}

// Finds the difference between now and a given date, in full days. Input date is expected to originate
// from an image's Created attribute which will follow ISO 3339/8601 format.
// Reference: https://docs.docker.com/engine/api/v1.43/#tag/Image/operation/ImageInspect
func getImageAgeDays(imageCreatedDateTime string) (int, error) {
	imageCreatedDate, error := time.Parse(time.RFC3339Nano, imageCreatedDateTime)

	if error != nil {
		log.Errorf("Error parsing imageCreatedDateTime date (%s). Error: %s", imageCreatedDateTime, error)
		return -1, error
	}

	return int(time.Since(imageCreatedDate).Hours() / 24), nil

}
