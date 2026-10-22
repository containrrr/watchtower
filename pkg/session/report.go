package session

import (
	"sort"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// report implements the Report interface for session results.
type report struct {
	scanned   []types.ContainerReport // Scanned containers.
	updated   []types.ContainerReport // Updated containers.
	failed    []types.ContainerReport // Failed containers.
	skipped   []types.ContainerReport // Skipped containers.
	stale     []types.ContainerReport // Stale containers.
	fresh     []types.ContainerReport // Fresh containers.
	restarted []types.ContainerReport // Restarted containers (linked dependencies).
}

// SingleContainerReport implements types.Report for individual container notifications.
//
// This struct is used when notification splitting by container is enabled (--notification-split-by-container).
// Unlike the standard report which groups all containers from a session, SingleContainerReport focuses
// on a specific container while providing context from all other containers in the session.
// This allows notifications to be sent separately for each updated or restarted container while maintaining
// awareness of the overall session state (failed, skipped, stale, fresh containers).
type SingleContainerReport struct {
	UpdatedReports   []types.ContainerReport // Primary container(s) that were updated in this notification
	RestartedReports []types.ContainerReport // Primary container(s) that were restarted in this notification
	ScannedReports   []types.ContainerReport // All containers scanned during the session (for context)
	FailedReports    []types.ContainerReport // All containers that failed to update (for context)
	SkippedReports   []types.ContainerReport // All containers that were skipped (for context)
	StaleReports     []types.ContainerReport // All containers with stale images (for context)
	FreshReports     []types.ContainerReport // All containers with fresh images (for context)
}

// sortableContainers implements sort.Interface for reports.
type sortableContainers []types.ContainerReport

// Scanned returns scanned containers.
//
// Returns:
//   - []types.ContainerReport: Scanned list.
func (r *report) Scanned() []types.ContainerReport {
	return r.scanned
}

// Updated returns updated containers.
//
// Returns:
//   - []types.ContainerReport: Updated list.
func (r *report) Updated() []types.ContainerReport {
	return r.updated
}

// Failed returns failed containers.
//
// Returns:
//   - []types.ContainerReport: Failed list.
func (r *report) Failed() []types.ContainerReport {
	return r.failed
}

// Skipped returns skipped containers.
//
// Returns:
//   - []types.ContainerReport: Skipped list.
func (r *report) Skipped() []types.ContainerReport {
	return r.skipped
}

// Stale returns stale containers.
//
// Returns:
//   - []types.ContainerReport: Stale list.
func (r *report) Stale() []types.ContainerReport {
	return r.stale
}

// Fresh returns fresh containers.
//
// Returns:
//   - []types.ContainerReport: Fresh list.
func (r *report) Fresh() []types.ContainerReport {
	return r.fresh
}

// Restarted returns restarted containers.
//
// Returns:
//   - []types.ContainerReport: Restarted list.
func (r *report) Restarted() []types.ContainerReport {
	return r.restarted
}

// allFromSlices returns deduplicated containers from the provided slices, prioritized by state.
//
// This function ensures that each container appears only once in the final result, with priority
// given to containers in more significant states (updated > restarted > failed > skipped > stale > fresh > scanned).
// The priority order reflects the importance of the container's update status for notification purposes.
//
// Parameters:
//   - scanned, updated, restarted, failed, skipped, stale, fresh: Slices of container reports categorized by their update state.
//
// Returns:
//   - []types.ContainerReport: Sorted, unique list with containers prioritized by their most significant state.
func allFromSlices(
	scanned, updated, restarted, failed, skipped, stale, fresh []types.ContainerReport,
) []types.ContainerReport {
	// Calculate total capacity for all containers to pre-allocate slice efficiently.
	allLen := len(scanned) + len(updated) + len(restarted) + len(failed) + len(skipped) + len(stale) + len(fresh)
	all := make([]types.ContainerReport, 0, allLen)
	// Track container IDs to prevent duplicates.
	presentIDs := make(map[types.ContainerID]struct{}, allLen)

	// appendUnique adds containers from a slice only if they haven't been added before.
	// This ensures deduplication while maintaining the priority order defined by the calling sequence.
	appendUnique := func(reports []types.ContainerReport) {
		for _, report := range reports {
			if _, found := presentIDs[report.ID()]; found {
				// Skip containers already added from higher-priority categories.
				continue
			}

			all = append(all, report)
			// Mark this container ID as processed.
			presentIDs[report.ID()] = struct{}{}
		}
	}

	// Add containers in priority order: updated containers get highest priority,
	// followed by restarted, failed, skipped, stale, fresh, and finally scanned (lowest priority).
	// This ensures that if a container appears in multiple categories, only the most
	// significant state representation is included in the final list.
	appendUnique(updated)   // Highest priority - containers that were successfully updated
	appendUnique(restarted) // Containers that were restarted (linked dependencies)
	appendUnique(failed)    // Containers that failed to update
	appendUnique(skipped)   // Containers that were intentionally skipped
	appendUnique(stale)     // Containers with stale images available
	appendUnique(fresh)     // Containers with fresh images (no update needed)
	appendUnique(scanned)   // Lowest priority - all containers that were scanned

	sort.Sort(sortableContainers(all)) // Sort final list by container ID for consistent ordering

	return all
}

// All returns deduplicated containers, prioritized by state.
//
// Returns:
//   - []types.ContainerReport: Sorted, unique list.
func (r *report) All() []types.ContainerReport {
	return allFromSlices(r.scanned, r.updated, r.restarted, r.failed, r.skipped, r.stale, r.fresh)
}

// newReport creates a report from progress data.
//
// Parameters:
//   - progress: Progress map to process.
//
// Returns:
//   - types.Report: Categorized and sorted report.
func newReport(log *zerolog.Logger, progress Progress) types.Report {
	report := &report{
		scanned:   make([]types.ContainerReport, 0, len(progress)),
		updated:   make([]types.ContainerReport, 0),
		failed:    make([]types.ContainerReport, 0),
		skipped:   make([]types.ContainerReport, 0),
		stale:     make([]types.ContainerReport, 0),
		fresh:     make([]types.ContainerReport, 0),
		restarted: make([]types.ContainerReport, 0),
	}

	// Categorize each container status.
	for _, update := range progress {
		categorizeContainer(log, report, update)
	}

	// Sort all categories by ID.
	sortCategories(report)

	return report
}

// categorizeContainer assigns a status to report categories.
//
// Parameters:
//   - report: Report to update.
//   - update: Container status to categorize.
func categorizeContainer(log *zerolog.Logger, report *report, update *ContainerStatus) {
	if update.state == SkippedState {
		report.skipped = append(report.skipped, update)

		return
	}

	// Add non-skipped to scanned list.
	report.scanned = append(report.scanned, update)

	log.Debug().
		Str("container", update.containerName).
		Str("state_before_image_check", update.State()).
		Str("new_image", update.newImage.ShortID()).
		Str("old_image", update.oldImage.ShortID()).
		Bool("images_equal", update.newImage == update.oldImage).
		Msg("Categorizing container status")

	// Categorize based on image or state.
	if update.newImage == update.oldImage &&
		update.state != RestartedState &&
		update.state != FailedState {
		log.Debug().
			Str("container", update.containerName).
			Str("old_state", update.State()).
			Str("new_state", freshStateString).
			Msg("Setting container state to fresh due to equal images")
		update.state = FreshState
		report.fresh = append(report.fresh, update)

		return
	}

	// Handle explicit states.
	//nolint:exhaustive // Other states handled above.
	switch update.state {
	case UpdatedState:
		report.updated = append(report.updated, update)
	case FailedState:
		report.failed = append(report.failed, update)
	case StaleState:
		report.stale = append(report.stale, update)
	case RestartedState:
		log.Debug().
			Str("container", update.containerName).
			Msg("Adding container to restarted list")
		report.restarted = append(report.restarted, update)
	default:
		update.state = StaleState
		report.stale = append(report.stale, update)
	}
}

// sortCategories sorts all report categories by container ID.
//
// Parameters:
//   - report: Report to sort.
func sortCategories(report *report) {
	sort.Sort(sortableContainers(report.scanned))
	sort.Sort(sortableContainers(report.updated))
	sort.Sort(sortableContainers(report.failed))
	sort.Sort(sortableContainers(report.skipped))
	sort.Sort(sortableContainers(report.stale))
	sort.Sort(sortableContainers(report.fresh))
	sort.Sort(sortableContainers(report.restarted))
}

// Len returns the slice length.
//
// Returns:
//   - int: Number of reports.
func (s sortableContainers) Len() int {
	return len(s)
}

// Less compares container IDs.
//
// Parameters:
//   - i, j: Indices to compare.
//
// Returns:
//   - bool: True if i's ID is less than j's.
func (s sortableContainers) Less(i, j int) bool {
	return s[i].ID() < s[j].ID()
}

// Scanned returns scanned containers.
func (r *SingleContainerReport) Scanned() []types.ContainerReport { return r.ScannedReports }

// Updated returns updated containers (only one for split notifications).
func (r *SingleContainerReport) Updated() []types.ContainerReport { return r.UpdatedReports }

// Failed returns failed containers.
func (r *SingleContainerReport) Failed() []types.ContainerReport { return r.FailedReports }

// Skipped returns skipped containers.
func (r *SingleContainerReport) Skipped() []types.ContainerReport { return r.SkippedReports }

// Stale returns stale containers.
func (r *SingleContainerReport) Stale() []types.ContainerReport { return r.StaleReports }

// Fresh returns fresh containers.
func (r *SingleContainerReport) Fresh() []types.ContainerReport { return r.FreshReports }

// Restarted returns restarted containers.
func (r *SingleContainerReport) Restarted() []types.ContainerReport { return r.RestartedReports }

// All returns deduplicated containers, prioritized by state.
//
// Returns:
//   - []types.ContainerReport: Sorted, unique list.
func (r *SingleContainerReport) All() []types.ContainerReport {
	return allFromSlices(
		r.ScannedReports,
		r.UpdatedReports,
		r.RestartedReports,
		r.FailedReports,
		r.SkippedReports,
		r.StaleReports,
		r.FreshReports,
	)
}

// Swap exchanges two reports.
//
// Parameters:
//   - i, j: Indices to swap.
func (s sortableContainers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
