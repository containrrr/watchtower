package session

import (
	"github.com/containrrr/watchtower/pkg/types"
	dockerTypes "github.com/docker/docker/api/types"
)

// Progress contains the current session container status
type Progress map[types.ContainerID]*ContainerStatus

// UpdateFromContainer sets various status fields from their corresponding container equivalents
func UpdateFromContainer(cont types.Container, newImage types.ImageID, state State) *ContainerStatus {

	var beforeMeta imageMeta
	if imageInfo := cont.ImageInfo(); imageInfo != nil && imageInfo.Config != nil {
		beforeMeta = imageMetaFromLabels(imageInfo.Config.Labels)
	} else {
		beforeMeta = make(imageMeta)
	}

	return &ContainerStatus{
		containerID:   cont.ID(),
		containerName: cont.Name(),
		imageName:     cont.ImageName(),
		oldImage:      cont.SafeImageID(),
		newImage:      newImage,
		state:         state,
		beforeMeta:    beforeMeta,
		afterMeta:     beforeMeta,
	}
}

// AddSkipped adds a container to the Progress with the state set as skipped
func (m Progress) AddSkipped(cont types.Container, err error) {
	update := UpdateFromContainer(cont, cont.SafeImageID(), SkippedState)
	update.error = err
	m.Add(update)
}

// AddScanned adds a container to the Progress with the state set as scanned
func (m Progress) AddScanned(cont types.Container, newImageID types.ImageID) {
	m.Add(UpdateFromContainer(cont, newImageID, ScannedState))

}

// UpdateFailed updates the containers passed, setting their state as failed with the supplied error
func (m Progress) UpdateFailed(failures map[types.ContainerID]error) {
	for id, err := range failures {
		update := m[id]
		update.error = err
		update.state = FailedState
	}
}

// Add a container to the map using container ID as the key
func (m Progress) Add(update *ContainerStatus) {
	m[update.containerID] = update
}

// MarkForUpdate marks the container identified by containerID for update
func (m Progress) MarkForUpdate(containerID types.ContainerID) {
	m[containerID].state = UpdatedState
}

// Report creates a new Report from a Progress instance
func (m Progress) Report() types.Report {
	return NewReport(m)
}

func (m Progress) UpdateLatestImage(containerID types.ContainerID, image dockerTypes.ImageInspect) {
	if image.Config != nil {
		m[containerID].afterMeta = imageMetaFromLabels(image.Config.Labels)
	}
}
