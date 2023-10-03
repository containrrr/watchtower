package data

import (
	"sort"

	"github.com/containrrr/watchtower/pkg/types"
)

// State is the outcome of a container in a session report
type State string

const (
	ScannedState State = "scanned"
	UpdatedState State = "updated"
	FailedState  State = "failed"
	SkippedState State = "skipped"
	StaleState   State = "stale"
	FreshState   State = "fresh"
)

// StatesFromString parses a string of state characters and returns a slice of the corresponding report states
func StatesFromString(str string) []State {
	states := make([]State, 0, len(str))
	for _, c := range str {
		switch c {
		case 'c':
			states = append(states, ScannedState)
		case 'u':
			states = append(states, UpdatedState)
		case 'e':
			states = append(states, FailedState)
		case 'k':
			states = append(states, SkippedState)
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

type report struct {
	scanned []types.ContainerReport
	updated []types.ContainerReport
	failed  []types.ContainerReport
	skipped []types.ContainerReport
	stale   []types.ContainerReport
	fresh   []types.ContainerReport
}

func (r *report) Scanned() []types.ContainerReport {
	return r.scanned
}
func (r *report) Updated() []types.ContainerReport {
	return r.updated
}
func (r *report) Failed() []types.ContainerReport {
	return r.failed
}
func (r *report) Skipped() []types.ContainerReport {
	return r.skipped
}
func (r *report) Stale() []types.ContainerReport {
	return r.stale
}
func (r *report) Fresh() []types.ContainerReport {
	return r.fresh
}

func (r *report) All() []types.ContainerReport {
	allLen := len(r.scanned) + len(r.updated) + len(r.failed) + len(r.skipped) + len(r.stale) + len(r.fresh)
	all := make([]types.ContainerReport, 0, allLen)

	presentIds := map[types.ContainerID][]string{}

	appendUnique := func(reports []types.ContainerReport) {
		for _, cr := range reports {
			if _, found := presentIds[cr.ID()]; found {
				continue
			}
			all = append(all, cr)
			presentIds[cr.ID()] = nil
		}
	}

	appendUnique(r.updated)
	appendUnique(r.failed)
	appendUnique(r.skipped)
	appendUnique(r.stale)
	appendUnique(r.fresh)
	appendUnique(r.scanned)

	sort.Sort(sortableContainers(all))

	return all
}

type sortableContainers []types.ContainerReport

// Len implements sort.Interface.Len
func (s sortableContainers) Len() int { return len(s) }

// Less implements sort.Interface.Less
func (s sortableContainers) Less(i, j int) bool { return s[i].ID() < s[j].ID() }

// Swap implements sort.Interface.Swap
func (s sortableContainers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
