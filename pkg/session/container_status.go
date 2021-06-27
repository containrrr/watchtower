package session

import (
	wt "github.com/containrrr/watchtower/pkg/types"
	"strings"
)

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
	ID         wt.ContainerID
	Name       string
	OldImageID wt.ImageID
	NewImageID wt.ImageID
	ImageName  string
	Error      error
	State      State
}

func (state State) String() string {
	switch state {
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

// MarshalJSON marshals State as a string
func (state State) MarshalJSON() ([]byte, error) {
	sb := strings.Builder{}
	sb.WriteString(`"`)
	sb.WriteString(state.String())
	sb.WriteString(`"`)
	return []byte(sb.String()), nil
}
