package session

import (
	"github.com/containrrr/watchtower/pkg/types"
)

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

func NewReport(progress Progress) types.Report {
	report := &report{
		scanned: []types.ContainerReport{},
		updated: []types.ContainerReport{},
		failed:  []types.ContainerReport{},
		skipped: []types.ContainerReport{},
		stale:   []types.ContainerReport{},
		fresh:   []types.ContainerReport{},
	}

	for _, update := range progress {
		if update.state == SkippedState {
			report.skipped = append(report.skipped, update)
			continue
		}

		report.scanned = append(report.scanned, update)
		if update.newImage == update.oldImage {
			update.state = FreshState
			report.fresh = append(report.fresh, update)
			continue
		}

		switch update.state {
		case UpdatedState:
			report.updated = append(report.updated, update)
		case FailedState:
			report.failed = append(report.failed, update)
		default:
			update.state = StaleState
			report.stale = append(report.stale, update)
		}

	}

	return report
}
