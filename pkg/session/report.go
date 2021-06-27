package session

import (
	"sort"
	"time"
)

type Report struct {
	Started time.Time
	Ended   time.Time
	Trigger Trigger
	Scanned []*ContainerStatus
	Updated []*ContainerStatus
	Failed  []*ContainerStatus
	Skipped []*ContainerStatus
	Stale   []*ContainerStatus
	Fresh   []*ContainerStatus
}

// NewReport creates a types.Report from the supplied Progress
// s.Started, time.Now().UTC(), s.Trigger, s.Progress
func NewReport(started, ended time.Time, trigger Trigger, progress Progress) *Report {
	report := &Report{
		Started: started,
		Ended:   ended,
		Trigger: trigger,
		Scanned: []*ContainerStatus{},
		Updated: []*ContainerStatus{},
		Failed:  []*ContainerStatus{},
		Skipped: []*ContainerStatus{},
		Stale:   []*ContainerStatus{},
		Fresh:   []*ContainerStatus{},
	}

	for _, update := range progress {
		if update.State == SkippedState {
			report.Skipped = append(report.Skipped, update)
			continue
		}

		report.Scanned = append(report.Scanned, update)
		if update.NewImageID == update.OldImageID {
			update.State = FreshState
			report.Fresh = append(report.Fresh, update)
			continue
		}

		switch update.State {
		case UpdatedState:
			report.Updated = append(report.Updated, update)
		case FailedState:
			report.Failed = append(report.Failed, update)
		default:
			update.State = StaleState
			report.Stale = append(report.Stale, update)
		}
	}

	sort.Sort(sortableContainers(report.Scanned))
	sort.Sort(sortableContainers(report.Updated))
	sort.Sort(sortableContainers(report.Failed))
	sort.Sort(sortableContainers(report.Skipped))
	sort.Sort(sortableContainers(report.Stale))
	sort.Sort(sortableContainers(report.Fresh))

	return report
}

type sortableContainers []*ContainerStatus

// Len implements sort.Interface.Len
func (s sortableContainers) Len() int { return len(s) }

// Less implements sort.Interface.Less
func (s sortableContainers) Less(i, j int) bool { return s[i].ID < s[j].ID }

// Swap implements sort.Interface.Swap
func (s sortableContainers) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
