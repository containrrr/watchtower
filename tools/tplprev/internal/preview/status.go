package preview

import "github.com/nicholas-fedor/tplprev/internal/report"

var _ report.ContainerReport = (*containerStatus)(nil)

//nolint:errname // containerStatus is not an error type. It contains an error field.
type containerStatus struct {
	containerID          report.ContainerID
	oldImage             report.ImageID
	newImage             report.ImageID
	containerName        string
	imageName            string
	containerError       error
	state                State
	monitorOnly          bool
	newContainerID       report.ContainerID
	gitRepo              string
	gitRef               string
	changelog            string
	source               string
	imageURL             string
	documentation        string
	currentImageVersion  string
	latestImageVersion   string
	currentImageRevision string
	latestImageRevision  string
}

func (u *containerStatus) ID() report.ContainerID {
	return u.containerID
}

func (u *containerStatus) Name() string {
	return u.containerName
}

func (u *containerStatus) CurrentImageID() report.ImageID {
	return u.oldImage
}

func (u *containerStatus) LatestImageID() report.ImageID {
	return u.newImage
}

func (u *containerStatus) ImageName() string {
	return u.imageName
}

func (u *containerStatus) Error() string {
	if u.containerError == nil {
		return ""
	}

	return u.containerError.Error()
}

func (u *containerStatus) State() string {
	return string(u.state)
}

func (u *containerStatus) IsMonitorOnly() bool {
	return u.monitorOnly
}

func (u *containerStatus) NewContainerID() report.ContainerID {
	return u.newContainerID
}

func (u *containerStatus) GitRepo() string {
	return u.gitRepo
}

func (u *containerStatus) GitRef() string {
	return u.gitRef
}

func (u *containerStatus) Changelog() string {
	return u.changelog
}

func (u *containerStatus) Source() string {
	return u.source
}

func (u *containerStatus) ImageURL() string {
	return u.imageURL
}

func (u *containerStatus) Documentation() string {
	return u.documentation
}

func (u *containerStatus) CurrentImageVersion() string {
	return u.currentImageVersion
}

func (u *containerStatus) LatestImageVersion() string {
	return u.latestImageVersion
}

func (u *containerStatus) CurrentImageRevision() string {
	return u.currentImageRevision
}

func (u *containerStatus) LatestImageRevision() string {
	return u.latestImageRevision
}
