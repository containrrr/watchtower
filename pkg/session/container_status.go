package session

import wt "github.com/containrrr/watchtower/pkg/types"

// State indicates what the current state is of the container
type State int

// State enum values
const (
	// UnknownState is only used to represent an uninitialized State value
	UnknownState State = iota
	SkippedState
	ScannedState
	UpdatedState
	FailedState
	FreshState
	StaleState
)

// ContainerStatus contains the container state during a session
type ContainerStatus struct {
	containerID   wt.ContainerID
	oldImage      wt.ImageID
	newImage      wt.ImageID
	containerName string
	imageName     string
	error
	state State
}

// ID returns the container ID
func (u *ContainerStatus) ID() wt.ContainerID {
	return u.containerID
}

// Name returns the container name
func (u *ContainerStatus) Name() string {
	return u.containerName
}

// CurrentImageID returns the image ID that the container used when the session started
func (u *ContainerStatus) CurrentImageID() wt.ImageID {
	return u.oldImage
}

// LatestImageID returns the newest image ID found during the session
func (u *ContainerStatus) LatestImageID() wt.ImageID {
	return u.newImage
}

// ImageName returns the name:tag that the container uses
func (u *ContainerStatus) ImageName() string {
	return u.imageName
}

// Error returns the error (if any) that was encountered for the container during a session
func (u *ContainerStatus) Error() string {
	if u.error == nil {
		return ""
	}
	return u.error.Error()
}

// State returns the current State that the container is in
func (u *ContainerStatus) State() string {
	switch u.state {
	case SkippedState:
		return "Skipped"
	case ScannedState:
		return "Scanned"
	case UpdatedState:
		return "Updated"
	case FailedState:
		return "Failed"
	case FreshState:
		return "Fresh"
	case StaleState:
		return "Stale"
	default:
		return "Unknown"
	}
}
