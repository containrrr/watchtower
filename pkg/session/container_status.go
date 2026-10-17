package session

import (
	"time"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	UnknownState   State = iota // Uninitialized state.
	SkippedState                // Container skipped.
	ScannedState                // Container scanned.
	UpdatedState                // Container updated.
	FailedState                 // Container update failed.
	FreshState                  // Container is fresh.
	StaleState                  // Container is stale.
	RestartedState              // Container restarted (linked dependency).
)

// State string constants.
const (
	unknownStateString   = "Unknown"
	skippedStateString   = "Skipped"
	scannedStateString   = "Scanned"
	updatedStateString   = "Updated"
	failedStateString    = "Failed"
	freshStateString     = "Fresh"
	staleStateString     = "Stale"
	restartedStateString = "Restarted"
)

// State indicates what the current state is of the container.
type State int

// ContainerStatus holds a container's state during a session.
//
//nolint:errname // ContainerStatus is not an error type, it contains an error field.
type ContainerStatus struct {
	containerID          types.ContainerID // Container ID.
	oldImage             types.ImageID     // Original image ID.
	newImage             types.ImageID     // Latest image ID.
	containerName        string            // Container name.
	imageName            string            // Image name with tag.
	containerError       error             // Error encountered, if any.
	state                State             // Current state.
	monitorOnly          bool              // Monitor-only flag.
	newContainerID       types.ContainerID // New container ID after update.
	cooldownAge          string            // Human-readable image age (e.g., "47 days, 11 hours").
	cooldownDelay        string            // Human-readable cooldown duration (e.g., "24 hours").
	cooldownRemaining    string            // Human-readable remaining time (empty if passed).
	cooldownEligibleAt   time.Time         // Time when the container becomes eligible for update.
	gitRepo              string            // Resolved Git repository URL.
	gitRef               string            // Resolved Git ref.
	changelog            string            // Changelog or releases URL.
	source               string            // OCI image source annotation.
	imageURL             string            // OCI image URL annotation.
	documentation        string            // OCI image documentation annotation.
	currentImageVersion  string            // OCI version annotation of the running image.
	latestImageVersion   string            // OCI version annotation of the new image.
	currentImageRevision string            // OCI revision annotation of the running image.
	latestImageRevision  string            // OCI revision annotation of the new image.
}

// ID returns the container ID.
//
// Returns:
//   - types.ContainerID: Container's unique identifier.
func (u *ContainerStatus) ID() types.ContainerID {
	return u.containerID
}

// Name returns the container name.
//
// Returns:
//   - string: Container's name.
func (u *ContainerStatus) Name() string {
	return u.containerName
}

// CurrentImageID returns the original image ID.
//
// Returns:
//   - types.ImageID: Image ID at session start.
func (u *ContainerStatus) CurrentImageID() types.ImageID {
	return u.oldImage
}

// LatestImageID returns the latest image ID.
//
// Returns:
//   - types.ImageID: Newest image ID from session.
func (u *ContainerStatus) LatestImageID() types.ImageID {
	return u.newImage
}

// ImageName returns the image name with tag.
//
// Returns:
//   - string: Image name (e.g., "nginx:latest").
func (u *ContainerStatus) ImageName() string {
	return u.imageName
}

// Error returns the session error, if any.
//
// Returns:
//   - string: Error message or empty if none.
func (u *ContainerStatus) Error() string {
	if u.containerError == nil {
		return ""
	}

	return u.containerError.Error()
}

// State returns the human-readable state name.
//
// Returns:
//   - string: State as a string (e.g., "Updated").
func (u *ContainerStatus) State() string {
	switch u.state {
	case UnknownState:
		return unknownStateString // Uninitialized state.
	case SkippedState:
		return skippedStateString
	case ScannedState:
		return scannedStateString
	case UpdatedState:
		return updatedStateString
	case FailedState:
		return failedStateString
	case FreshState:
		return freshStateString
	case StaleState:
		return staleStateString
	case RestartedState:
		return restartedStateString
	default:
		return unknownStateString // Fallback for unexpected values.
	}
}

// IsMonitorOnly returns whether the container is in monitor-only mode.
//
// Returns:
//   - bool: True if monitor-only, false otherwise.
func (u *ContainerStatus) IsMonitorOnly() bool {
	return u.monitorOnly
}

// NewContainerID returns the new container ID after update.
//
// Returns:
//   - types.ContainerID: New container ID or empty if not updated.
func (u *ContainerStatus) NewContainerID() types.ContainerID {
	return u.newContainerID
}

// SetNewContainerID sets the new container ID after update.
//
// Parameters:
//   - newID: The new container ID.
func (u *ContainerStatus) SetNewContainerID(newID types.ContainerID) {
	u.newContainerID = newID
}

// SetCooldownInfo sets cooldown metadata for this container.
//
// Parameters:
//   - age: Human-readable image age (e.g., "47 days, 11 hours").
//   - delay: Human-readable cooldown duration (e.g., "24 hours").
//   - remaining: Human-readable remaining time (empty if passed).
//   - eligibleAt: Time when the container becomes eligible for update.
func (u *ContainerStatus) SetCooldownInfo(
	age,
	delay,
	remaining string,
	eligibleAt time.Time,
) {
	u.cooldownAge = age
	u.cooldownDelay = delay
	u.cooldownRemaining = remaining
	u.cooldownEligibleAt = eligibleAt
}

// CooldownEligibleAt returns the time when the container becomes eligible for update.
//
// Returns:
//   - time.Time: The eligible-at timestamp (zero if not set).
func (u *ContainerStatus) CooldownEligibleAt() time.Time {
	return u.cooldownEligibleAt
}

// CooldownAge returns the human-readable image age.
func (u *ContainerStatus) CooldownAge() string {
	return u.cooldownAge
}

// CooldownDelay returns the human-readable cooldown duration.
func (u *ContainerStatus) CooldownDelay() string {
	return u.cooldownDelay
}

// CooldownRemaining returns the human-readable remaining cooldown time.
func (u *ContainerStatus) CooldownRemaining() string {
	return u.cooldownRemaining
}

// GitRepo returns the resolved Git repository URL.
//
// Returns:
//   - string: Clone URL, or empty.
func (u *ContainerStatus) GitRepo() string { return u.gitRepo }

// GitRef returns the resolved Git ref.
//
// Returns:
//   - string: Branch or tag, or empty.
func (u *ContainerStatus) GitRef() string { return u.gitRef }

// Changelog returns the changelog or releases URL.
//
// Returns:
//   - string: Changelog URL, or empty.
func (u *ContainerStatus) Changelog() string { return u.changelog }

// Source returns the OCI image source annotation.
//
// Returns:
//   - string: OCI source URL, or empty.
func (u *ContainerStatus) Source() string { return u.source }

// ImageURL returns the OCI image URL annotation.
//
// Returns:
//   - string: OCI image URL, or empty.
func (u *ContainerStatus) ImageURL() string { return u.imageURL }

// Documentation returns the OCI image documentation annotation.
//
// Returns:
//   - string: OCI documentation URL, or empty.
func (u *ContainerStatus) Documentation() string { return u.documentation }

// CurrentImageRevision returns the OCI image revision annotation of the running image.
//
// Returns:
//   - string: OCI revision, or empty.
func (u *ContainerStatus) CurrentImageRevision() string { return u.currentImageRevision }

// LatestImageRevision returns the OCI image revision annotation of a newer image.
//
// Returns:
//   - string: OCI revision, or empty.
func (u *ContainerStatus) LatestImageRevision() string { return u.latestImageRevision }

// CurrentImageVersion returns the OCI image version annotation of the running image.
//
// Returns:
//   - string: OCI version, or empty.
func (u *ContainerStatus) CurrentImageVersion() string { return u.currentImageVersion }

// LatestImageVersion returns the OCI image version annotation of a newer image.
//
// Returns:
//   - string: OCI version, or empty.
func (u *ContainerStatus) LatestImageVersion() string { return u.latestImageVersion }

// SetGitMetadata sets Git and OCI report fields for this container.
//
// Parameters:
//   - meta: Resolved report metadata.
//
// Returns:
//   - none.
func (u *ContainerStatus) SetGitMetadata(meta container.ReportMeta) {
	u.gitRepo = meta.GitRepo
	u.gitRef = meta.GitRef
	u.changelog = meta.Changelog
	u.source = meta.Source
	u.imageURL = meta.ImageURL
	u.documentation = meta.Documentation
	u.currentImageVersion = meta.CurrentVersion
	u.latestImageVersion = meta.LatestVersion
	u.currentImageRevision = meta.CurrentRevision
	u.latestImageRevision = meta.LatestRevision
}
