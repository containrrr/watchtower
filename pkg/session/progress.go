package session

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Progress tracks container statuses during a session.
type Progress map[types.ContainerID]*ContainerStatus

// updateFromContainer creates a status from container data.
//
// Parameters:
//   - container: Container to update from.
//   - newImage: Latest image ID.
//   - state: Container state.
//   - params: Update parameters for monitor-only check.
//
// Returns:
//   - *ContainerStatus: Updated status.
func updateFromContainer(log *zerolog.Logger,
	container types.Container,
	newImage types.ImageID,
	state State,
	params types.UpdateParams,
) *ContainerStatus {
	update := &ContainerStatus{
		containerID:    container.ID(),
		oldImage:       container.ImageID(),
		newImage:       newImage,
		containerName:  container.Name(),
		imageName:      container.ImageName(),
		containerError: nil,
		state:          state,
		monitorOnly:    container.IsMonitorOnly(params),
		newContainerID: "",
	}
	applyReportMeta(update, container, params)
	log.Debug().
		Str("container_id", container.ID().ShortID()).
		Str("name", container.Name()).
		Str("state", update.State()).
		Msg("Updated container status from container")

	return update
}

// AddSkipped adds a container as skipped with an error.
//
// Parameters:
//   - container: Container to add.
//   - err: Skip reason error.
//   - params: Update parameters for monitor-only check.
func (m Progress) AddSkipped(log *zerolog.Logger, container types.Container, err error, params types.UpdateParams) {
	update := updateFromContainer(log, container, container.ImageID(), SkippedState, params)
	update.containerError = err
	m.Add(log, update)
	log.Debug().
		Err(err).
		Str("container_id", container.ID().ShortID()).
		Str("name", container.Name()).
		Msg("Added container as skipped")
}

// AddFailed adds a container as failed with an error.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - container: Container to add.
//   - err: Failure reason error.
//   - params: Update parameters for monitor-only check.
func (m Progress) AddFailed(log *zerolog.Logger, container types.Container, err error, params types.UpdateParams) {
	update := updateFromContainer(log, container, container.ImageID(), FailedState, params)
	update.containerError = err
	m.Add(log, update)
	log.Debug().
		Err(err).
		Str("container_id", container.ID().ShortID()).
		Str("name", container.Name()).
		Msg("Added container as failed")
}

// AddScanned adds a container as scanned with a new image.
//
// Parameters:
//   - container: Container to add.
//   - newImage: Latest image ID.
//   - params: Update parameters for monitor-only check.
func (m Progress) AddScanned(log *zerolog.Logger,
	container types.Container,
	newImage types.ImageID,
	params types.UpdateParams,
) {
	m.Add(log, updateFromContainer(log, container, newImage, ScannedState, params))
	log.Debug().
		Str("container_id", container.ID().ShortID()).
		Str("name", container.Name()).
		Str("new_image", newImage.ShortID()).
		Msg("Added container as scanned")
}

// UpdateFailed marks containers as failed with errors.
//
// Parameters:
//   - failures: Map of container IDs to errors.
func (m Progress) UpdateFailed(log *zerolog.Logger, failures map[types.ContainerID]error) {
	for containerID, err := range failures {
		update, exists := m[containerID]
		if !exists {
			log.Debug().
				Str("container_id", containerID.ShortID()).
				Msg("Container not found in progress map, cannot mark as failed")

			continue
		}

		update.containerError = err
		update.state = FailedState
		log.Warn().
			Err(err).
			Str("container_id", containerID.ShortID()).
			Str("name", update.Name()).
			Msg("Updated container state to failed")
	}
}

// UpdateSkipped marks containers as skipped, with the reason for each. A
// container already marked as failed keeps its failure and error, so a later
// phase skipping it cannot hide why it failed.
//
// Parameters:
//   - log: Process logger.
//   - skips: Map of container IDs to skip reasons.
func (m Progress) UpdateSkipped(log *zerolog.Logger, skips map[types.ContainerID]error) {
	for containerID, reason := range skips {
		update, exists := m[containerID]
		if !exists {
			log.Debug().
				Str("container_id", containerID.ShortID()).
				Msg("Container not found in progress map, cannot mark as skipped")

			continue
		}

		if update.state == FailedState {
			log.Debug().
				Err(reason).
				Str("container_id", containerID.ShortID()).
				Str("name", update.Name()).
				Msg("Container already failed, keeping its failure")

			continue
		}

		update.containerError = reason
		update.state = SkippedState
		log.Debug().
			Err(reason).
			Str("container_id", containerID.ShortID()).
			Str("name", update.Name()).
			Msg("Updated container state to skipped")
	}
}

// applyReportMeta copies Git and OCI fields onto a scanned container status.
//
// Parameters:
//   - status: Status to update.
//   - c: Source container.
//   - params: Update parameters used to resolve report metadata.
func applyReportMeta(status *ContainerStatus, c types.Container, params types.UpdateParams) {
	if status == nil {
		return
	}

	if _, ok := c.(*container.Container); !ok {
		return
	}

	meta := container.ResolveReportMeta(c, params, container.ChangelogVars{}, oci.Annotations{})
	status.SetGitMetadata(meta)
}

// SetLatestImage updates the latest image ID recorded for a container.
//
// Parameters:
//   - log: Process logger.
//   - containerID: Container identity.
//   - image: Newest image ID, including a Git-built image.
//
// Returns:
//   - none.
func (m Progress) SetLatestImage(log *zerolog.Logger, containerID types.ContainerID, image types.ImageID) {
	update, exists := m[containerID]
	if !exists {
		return
	}

	update.newImage = image
	log.Debug().
		Str("container_id", containerID.ShortID()).
		Str("image", image.ShortID()).
		Msg("Updated latest image on container status")
}

// SetLatestImageMeta re-resolves report metadata for a known new image and
// applies it to a container's status.
//
// It serves both update paths. A registry update passes the pulled image's
// annotations and, when the changelog feature is enabled, a probed release
// tag. A Git rebuild passes its resolved tag and commit with no latest
// annotations, so LatestImageVersion stays empty.
//
// Parameters:
//   - log: Process logger.
//   - c: Container whose report should be updated.
//   - params: Update parameters.
//   - vars: New-version values for changelog placeholders.
//   - latest: OCI annotations from a newly pulled image, or empty for none.
//
// Returns:
//   - container.ReportMeta: The applied metadata, or empty when nothing changed.
func (m Progress) SetLatestImageMeta(
	log *zerolog.Logger,
	c types.Container,
	params types.UpdateParams,
	vars container.ChangelogVars,
	latest oci.Annotations,
) container.ReportMeta {
	if c == nil {
		return container.ReportMeta{}
	}

	update, exists := m[c.ID()]
	if !exists {
		return container.ReportMeta{}
	}

	meta := container.ResolveReportMeta(c, params, vars, latest)
	update.SetGitMetadata(meta)

	log.Debug().
		Str("container_id", c.ID().ShortID()).
		Str("name", update.Name()).
		Str("latest_version", meta.LatestVersion).
		Str("changelog", meta.Changelog).
		Msg("Applied latest image metadata to container status")

	return meta
}

// Add inserts a container status into the progress map.
//
// Parameters:
//   - update: Status to add.
func (m Progress) Add(log *zerolog.Logger, update *ContainerStatus) {
	m[update.containerID] = update
	log.Debug().
		Str("container_id", update.containerID.ShortID()).
		Str("name", update.containerName).
		Str("state", update.State()).
		Msg("Added container status to progress map")
}

// MarkForUpdate sets a container's state to updated.
//
// Parameters:
//   - containerID: ID of container to mark.
func (m Progress) MarkForUpdate(log *zerolog.Logger, containerID types.ContainerID) {
	update, exists := m[containerID]
	if !exists {
		log.Debug().
			Str("container_id", containerID.ShortID()).
			Msg("Attempted to mark non-existent container for update")

		return
	}

	update.state = UpdatedState
	log.Debug().
		Str("container_id", containerID.ShortID()).
		Str("name", update.Name()).
		Msg("Marked container for update")
}

// MarkRestarted sets a container's state to restarted.
//
// Parameters:
//   - containerID: ID of container to mark.
func (m Progress) MarkRestarted(log *zerolog.Logger, containerID types.ContainerID) {
	update, exists := m[containerID]
	if !exists {
		log.Debug().
			Str("container_id", containerID.ShortID()).
			Msg("Attempted to mark non-existent container as restarted")

		return
	}

	update.state = RestartedState
	log.Debug().
		Str("container_id", containerID.ShortID()).
		Str("name", update.Name()).
		Msg("Marked container as restarted")
}

// SetCooldownInfo sets cooldown metadata on a container's status.
//
// Parameters:
//   - containerID: Container ID.
//   - age: Human-readable image age.
//   - delay: Human-readable cooldown duration.
//   - remaining: Human-readable remaining time (empty if passed).
//   - eligibleAt: Time when the container becomes eligible for update.
func (m Progress) SetCooldownInfo(log *zerolog.Logger,
	containerID types.ContainerID,
	age,
	delay,
	remaining string,
	eligibleAt time.Time,
) {
	update, exists := m[containerID]
	if !exists {
		log.Debug().
			Str("container_id", containerID.ShortID()).
			Msg("Attempted to set cooldown info on non-existent container")

		return
	}

	update.SetCooldownInfo(age, delay, remaining, eligibleAt)
	log.Debug().
		Str("container_id", containerID.ShortID()).
		Str("name", update.Name()).
		Str("image_age", age).
		Str("cooldown", delay).
		Str("remaining", remaining).
		Msg("Set cooldown info on container")
}

// Report generates a report from the progress data.
//
// Returns:
//   - types.Report: New report instance.
func (m Progress) Report(log *zerolog.Logger) types.Report {
	log.Debug().
		Int("count", len(m)).
		Msg("Generating report")

	return newReport(log, m)
}
