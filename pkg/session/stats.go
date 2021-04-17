package session

import (
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/types"
)

type Report struct {
	scanned []types.ContainerReport
	updated []types.ContainerReport
	failed  []types.ContainerReport
	skipped []types.ContainerReport
	stale   []types.ContainerReport
	fresh   []types.ContainerReport
}

func (r *Report) Scanned() []types.ContainerReport {
	return r.scanned
}
func (r *Report) Updated() []types.ContainerReport {
	return r.updated
}
func (r *Report) Failed() []types.ContainerReport {
	return r.failed
}
func (r *Report) Skipped() []types.ContainerReport {
	return r.skipped
}
func (r *Report) Stale() []types.ContainerReport {
	return r.stale
}
func (r *Report) Fresh() []types.ContainerReport {
	return r.fresh
}

func (m Report) Metric() *metrics.Metric {
	return &metrics.Metric{
		len(m.scanned),
		len(m.updated),
		len(m.failed),
	}
}

func NewReport(progress Progress) *Report {
	report := &Report{
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
