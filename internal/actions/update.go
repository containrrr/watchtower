package actions

import (
	"errors"
	"fmt"

	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/lifecycle"
	"github.com/containrrr/watchtower/pkg/session"
	"github.com/containrrr/watchtower/pkg/sorter"
	"github.com/containrrr/watchtower/pkg/types"

	log "github.com/sirupsen/logrus"
)

type updateSession struct {
	client   container.Client
	params   types.UpdateParams
	progress *session.Progress
}

// Update looks at the running Docker containers to see if any of the images
// used to start those containers have been updated. If a change is detected in
// any of the images, the associated containers are stopped and restarted with
// the new image.
func Update(client container.Client, params types.UpdateParams) (types.Report, error) {
	log.Debug("Starting new update session")
	us := updateSession{client: client, params: params, progress: &session.Progress{}}

	us.TryExecuteLifecycleCommands(types.PreCheck)

	if err := us.run(); err != nil {
		return nil, err
	}

	us.TryExecuteLifecycleCommands(types.PostCheck)

	return us.progress.Report(), nil
}

func (us *updateSession) run() (err error) {

	containers, err := us.client.ListContainers(us.params.Filter)
	if err != nil {
		return err
	}

	for i, targetContainer := range containers {
		stale, newestImage, err := us.client.IsContainerStale(targetContainer, us.params)
		shouldUpdate := stale && !us.params.NoRestart && !targetContainer.IsMonitorOnly(us.params)

		if err == nil && shouldUpdate {
			// Check to make sure we have all the necessary information for recreating the container
			err = targetContainer.VerifyConfiguration()
			if err != nil && log.IsLevelEnabled(log.TraceLevel) {
				// If the image information is incomplete and trace logging is enabled, log it for further diagnosis
				log.WithError(err).Trace("Cannot obtain enough information to recreate container")
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
			us.progress.AddSkipped(targetContainer, err)
			containers[i].SetMarkedForUpdate(false)
		} else {
			us.progress.AddScanned(targetContainer, newestImage)
			containers[i].SetMarkedForUpdate(shouldUpdate)
		}
	}

	containers, err = sorter.SortByDependencies(containers)
	if err != nil {
		return fmt.Errorf("failed to sort containers for updating: %v", err)
	}

	UpdateImplicitRestart(containers)

	var containersToUpdate []types.Container
	for _, c := range containers {
		if c.ToRestart() {
			containersToUpdate = append(containersToUpdate, c)
			us.progress.MarkForUpdate(c.ID())
		}
	}

	if us.params.RollingRestart {
		us.performRollingRestart(containersToUpdate)
	} else {
		stoppedImages := us.stopContainersInReversedOrder(containersToUpdate)
		us.restartContainersInSortedOrder(containersToUpdate, stoppedImages)
	}

	return nil
}

func (us *updateSession) performRollingRestart(containers []types.Container) {
	cleanupImageIDs := make(map[types.ImageID]bool, len(containers))
	failed := make(map[types.ContainerID]error, len(containers))

	for i := len(containers) - 1; i >= 0; i-- {
		if containers[i].ToRestart() {
			err := us.stopContainer(containers[i])
			if err != nil {
				failed[containers[i].ID()] = err
			} else {
				if err := us.restartContainer(containers[i]); err != nil {
					failed[containers[i].ID()] = err
				} else if containers[i].IsMarkedForUpdate() {
					// Only add (previously) stale containers' images to cleanup
					cleanupImageIDs[containers[i].ImageID()] = true
				}
			}
		}
	}

	if us.params.Cleanup {
		us.cleanupImages(cleanupImageIDs)
	}
	us.progress.UpdateFailed(failed)
}

func (us *updateSession) stopContainersInReversedOrder(containers []types.Container) (stopped map[types.ImageID]bool) {
	failed := make(map[types.ContainerID]error, len(containers))
	stopped = make(map[types.ImageID]bool, len(containers))
	for i := len(containers) - 1; i >= 0; i-- {
		if err := us.stopContainer(containers[i]); err != nil {
			failed[containers[i].ID()] = err
		} else {
			// NOTE: If a container is restarted due to a dependency this might be empty
			stopped[containers[i].SafeImageID()] = true
		}

	}
	us.progress.UpdateFailed(failed)

	return stopped
}

func (us *updateSession) stopContainer(c types.Container) error {
	if c.IsWatchtower() {
		log.Debugf("This is the watchtower container %s", c.Name())
		return nil
	}

	if !c.ToRestart() {
		return nil
	}

	// Perform an additional check here to prevent us from stopping a linked container we cannot restart
	if c.IsLinkedToRestarting() {
		if err := c.VerifyConfiguration(); err != nil {
			return err
		}
	}

	if us.params.LifecycleHooks {
		err := lifecycle.ExecuteLifeCyclePhaseCommand(types.PreUpdate, us.client, c)
		if err != nil {

			if errors.Is(err, container.ErrorLifecycleSkip) {
				log.Debug(err)
				return err
			}

			log.Error(err)
			log.Info("Skipping container as the pre-update command failed")
			return err
		}
	}

	if err := us.client.StopContainer(c, us.params.Timeout); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (us *updateSession) restartContainersInSortedOrder(containers []types.Container, stoppedImages map[types.ImageID]bool) {
	cleanupImageIDs := make(map[types.ImageID]bool, len(containers))
	failed := make(map[types.ContainerID]error, len(containers))

	for _, c := range containers {
		if !c.ToRestart() {
			continue
		}
		if stoppedImages[c.SafeImageID()] {
			if err := us.restartContainer(c); err != nil {
				failed[c.ID()] = err
			} else if c.IsMarkedForUpdate() {
				// Only add (previously) stale containers' images to cleanup
				cleanupImageIDs[c.ImageID()] = true
			}
		}
	}

	if us.params.Cleanup {
		us.cleanupImages(cleanupImageIDs)
	}

	us.progress.UpdateFailed(failed)
}

func (us *updateSession) cleanupImages(imageIDs map[types.ImageID]bool) {
	for imageID := range imageIDs {
		if imageID == "" {
			continue
		}
		if err := us.client.RemoveImageByID(imageID); err != nil {
			log.Error(err)
		}
	}
}

func (us *updateSession) restartContainer(container types.Container) error {
	if container.IsWatchtower() {
		// Since we can't shut down a watchtower container immediately, we need to
		// start the new one while the old one is still running. This prevents us
		// from re-using the same container name, so we first rename the current
		// instance so that the new one can adopt the old name.
		if err := us.client.RenameContainer(container, util.RandName()); err != nil {
			log.Error(err)
			return nil
		}
	}

	if !us.params.NoRestart {
		if newContainerID, err := us.client.StartContainer(container); err != nil {
			log.Error(err)
			return err
		} else if container.ToRestart() && us.params.LifecycleHooks {
			lifecycle.ExecutePostUpdateCommand(us.client, newContainerID)
		}
	}
	return nil
}

// UpdateImplicitRestart iterates through the passed containers, setting the
// `linkedToRestarting` flag if any of its linked containers are marked for restart
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

// TryExecuteLifecycleCommands tries to run the corresponding lifecycle hook for all containers included by the current filter.
func (us *updateSession) TryExecuteLifecycleCommands(phase types.LifecyclePhase) {
	if !us.params.LifecycleHooks {
		return
	}

	containers, err := us.client.ListContainers(us.params.Filter)
	if err != nil {
		log.WithError(err).Warn("Skipping lifecycle commands. Failed to list containers.")
		return
	}

	for _, c := range containers {
		err := lifecycle.ExecuteLifeCyclePhaseCommand(phase, us.client, c)
		if err != nil {
			log.WithField("container", c.Name()).Error(err)
		}
	}
}
