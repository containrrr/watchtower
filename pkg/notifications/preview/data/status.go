package data

import wt "github.com/containrrr/watchtower/pkg/types"

type containerStatus struct {
	containerID   wt.ContainerID
	oldImage      wt.ImageID
	newImage      wt.ImageID
	containerName string
	imageName     string
	error
	state State
}

func (u *containerStatus) ID() wt.ContainerID {
	return u.containerID
}

func (u *containerStatus) Name() string {
	return u.containerName
}

func (u *containerStatus) CurrentImageID() wt.ImageID {
	return u.oldImage
}

func (u *containerStatus) LatestImageID() wt.ImageID {
	return u.newImage
}

func (u *containerStatus) ImageName() string {
	return u.imageName
}

func (u *containerStatus) Error() string {
	if u.error == nil {
		return ""
	}
	return u.error.Error()
}

func (u *containerStatus) State() string {
	return string(u.state)
}
