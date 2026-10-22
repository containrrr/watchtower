package preview

import (
	"sort"

	"github.com/nicholas-fedor/tplprev/internal/report"
)

// State is the outcome of a container in a session report.
type State string

const (
	ScannedState   State = "scanned"
	UpdatedState   State = "updated"
	FailedState    State = "failed"
	SkippedState   State = "skipped"
	RestartedState State = "restarted"
	StaleState     State = "stale"
	FreshState     State = "fresh"
)

var _ report.Report = (*sessionReport)(nil)

// StatesFromString parses a string of state characters and returns a slice of the corresponding report states.
//
// Parameters:
//   - str: Compact state characters (c, u, e, k, r, t, f).
//
// Returns:
//   - []State: Parsed report states. Unknown characters are skipped.
func StatesFromString(str string) []State {
	states := make([]State, 0, len(str))

	for _, char := range str {
		switch char {
		case 'c':
			states = append(states, ScannedState)
		case 'u':
			states = append(states, UpdatedState)
		case 'e':
			states = append(states, FailedState)
		case 'k':
			states = append(states, SkippedState)
		case 'r':
			states = append(states, RestartedState)
		case 't':
			states = append(states, StaleState)
		case 'f':
			states = append(states, FreshState)
		default:
			continue
		}
	}

	return states
}

type sessionReport struct {
	scanned   []report.ContainerReport
	updated   []report.ContainerReport
	failed    []report.ContainerReport
	skipped   []report.ContainerReport
	stale     []report.ContainerReport
	fresh     []report.ContainerReport
	restarted []report.ContainerReport
}

func (r *sessionReport) Scanned() []report.ContainerReport {
	return r.scanned
}

func (r *sessionReport) Updated() []report.ContainerReport {
	return r.updated
}

func (r *sessionReport) Failed() []report.ContainerReport {
	return r.failed
}

func (r *sessionReport) Skipped() []report.ContainerReport {
	return r.skipped
}

func (r *sessionReport) Stale() []report.ContainerReport {
	return r.stale
}

func (r *sessionReport) Fresh() []report.ContainerReport {
	return r.fresh
}

func (r *sessionReport) Restarted() []report.ContainerReport {
	return r.restarted
}

func (r *sessionReport) All() []report.ContainerReport {
	allLen := len(r.scanned) + len(r.updated) + len(r.failed) + len(r.skipped) +
		len(r.stale) + len(r.fresh) + len(r.restarted)
	all := make([]report.ContainerReport, 0, allLen)

	presentIDs := map[report.ContainerID][]string{}

	appendUnique := func(reports []report.ContainerReport) {
		for _, item := range reports {
			_, found := presentIDs[item.ID()]
			if found {
				continue
			}

			all = append(all, item)
			presentIDs[item.ID()] = nil
		}
	}

	appendUnique(r.updated)
	appendUnique(r.restarted)
	appendUnique(r.failed)
	appendUnique(r.skipped)
	appendUnique(r.stale)
	appendUnique(r.fresh)
	appendUnique(r.scanned)

	sort.Sort(sortableContainers(all))

	return all
}

type sortableContainers []report.ContainerReport

func (s sortableContainers) Len() int { return len(s) }

func (s sortableContainers) Less(i, j int) bool { return s[i].ID() < s[j].ID() }

func (s sortableContainers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
