package session

import "github.com/containrrr/watchtower/pkg/container"

type Progress map[string]*ContainerStatus

func UpdateFromContainer(cont container.Interface, newImage string, state State) *ContainerStatus {
	return &ContainerStatus{
		containerID:   cont.ID(),
		containerName: cont.Name(),
		imageName:     cont.ImageName(),
		oldImage:      cont.SafeImageID(),
		newImage:      newImage,
		state:         state,
	}
}

func (m Progress) AddSkipped(cont container.Interface, err error) {
	update := UpdateFromContainer(cont, cont.SafeImageID(), SkippedState)
	update.error = err
	m.Add(update)
}

func (m Progress) AddScanned(cont container.Interface, newImage string) {
	m.Add(UpdateFromContainer(cont, newImage, ScannedState))
}

func (m Progress) UpdateFailed(failures map[string]error) {
	for id, err := range failures {
		update := m[id]
		update.error = err
		update.state = FailedState
	}
}

func (m Progress) Add(update *ContainerStatus) {
	m[update.containerID] = update
}

func (m Progress) MarkForUpdate(containerID string) {
	m[containerID].state = UpdatedState
}

func (m Progress) Report() *Report {
	return NewReport(m)
}
