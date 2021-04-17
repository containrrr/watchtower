package session

type State int

const (
	UnknownState State = iota
	SkippedState
	ScannedState
	UpdatedState
	FailedState
	FreshState
	StaleState
)

type ContainerStatus struct {
	containerID   string
	oldImage      string
	newImage      string
	containerName string
	imageName     string
	error
	state State
}

func (u *ContainerStatus) ID() string {
	return u.containerID
}

func (u *ContainerStatus) Name() string {
	return u.containerName
}

func (u *ContainerStatus) OldImageID() string {
	return u.oldImage
}

func (u *ContainerStatus) NewImageID() string {
	return u.oldImage
}

func (u *ContainerStatus) ImageName() string {
	return u.imageName
}

func (u *ContainerStatus) Error() string {
	if u.error == nil {
		return ""
	}
	return u.error.Error()
}

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
