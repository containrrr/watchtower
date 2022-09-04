package mocks

import (
	"errors"

	"github.com/containrrr/watchtower/pkg/session"
	wt "github.com/containrrr/watchtower/pkg/types"
)

// CreateMockProgressReport creates a mock report from a given set of container states
// All containers will be given a unique ID and name based on its state and index
func CreateMockProgressReport(states ...session.State) wt.Report {

	stateNums := make(map[session.State]int)
	progress := session.Progress{}
	failed := make(map[wt.ContainerID]error)

	for _, state := range states {
		index := stateNums[state]

		switch state {
		case session.SkippedState:
			c, _ := CreateContainerForProgress(index, 41, "skip%d")
			progress.AddSkipped(c, errors.New("unpossible"))
		case session.FreshState:
			c, _ := CreateContainerForProgress(index, 31, "frsh%d")
			progress.AddScanned(c, c.ImageID())
		case session.UpdatedState:
			c, newImage := CreateContainerForProgress(index, 11, "updt%d")
			progress.AddScanned(c, newImage)
			progress.MarkForUpdate(c.ID())
		case session.FailedState:
			c, newImage := CreateContainerForProgress(index, 21, "fail%d")
			progress.AddScanned(c, newImage)
			failed[c.ID()] = errors.New("accidentally the whole container")
		}

		stateNums[state] = index + 1
	}
	progress.UpdateFailed(failed)

	return progress.Report()

}
