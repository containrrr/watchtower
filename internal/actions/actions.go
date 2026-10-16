package actions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// RunUpdatesWithNotificationsParams holds runtime dependencies and update policy.
//
// Update carries the full types.UpdateParams snapshot from config.UpdateParams
// (or an equivalent complete construction).
type RunUpdatesWithNotificationsParams struct {
	// Logger is the process logger for this update session. Required and must be non-nil.
	Logger *zerolog.Logger
	// Client is the Docker client for container operations.
	Client container.Client
	// GitClient inspects remotes and clones repositories when the Git watcher is used.
	GitClient *git.Client
	// Notifier sends update status messages to configured channels.
	Notifier types.Notifier
	// NotificationSplitByContainer enables a separate notification per updated container.
	NotificationSplitByContainer bool
	// NotificationReport enables report-based notification templates.
	NotificationReport bool
	// EventBroadcaster publishes SSE events during the update session.
	EventBroadcaster *events.Broadcaster
	// Update is the complete update policy for this invocation (filter, cleanup, timeouts, etc.).
	Update types.UpdateParams
	// OnOldSelfDetected is called when Watchtower finds that it runs in an old
	// instance's container, after that container's restart policy is disabled.
	// The caller should stop the process. Nil means the session simply ends.
	OnOldSelfDetected func()
}

// RunUpdatesWithNotifications performs container updates and sends notifications about the results.
//
// It executes the update action with configured parameters, batches notifications, and returns a metric
// summarizing the session for monitoring purposes, ensuring users are informed of update outcomes.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - params: The RunUpdatesWithNotificationsParams struct containing all configuration parameters.
//     params.Logger is required and must be non-nil (nil panics on first log call).
//
// Returns:
//   - *metrics.Metric: A pointer to a metric object summarizing the update session (scanned, updated, failed counts).
func RunUpdatesWithNotifications(
	ctx context.Context,
	params RunUpdatesWithNotificationsParams,
) *metrics.Metric {
	log := params.Logger

	log.Debug().Msg("Starting RunUpdatesWithNotifications")

	// Initiate notification batching.
	startNotifications(log, params.Notifier, params.NotificationSplitByContainer)

	updateConfig := params.Update

	// Publish redacted policy flags only to avoid UpdateParams leaking internal IDs.
	if params.EventBroadcaster != nil {
		params.EventBroadcaster.Publish(events.Event{
			Type:      "scan_started",
			Timestamp: time.Now().UTC(),
			Data:      events.NewScanStartedData(updateConfig),
		})
	}

	// Execute the container update operation
	result, cleanupImageInfosPtr, err := executeUpdate(log,
		ctx,
		params.Client,
		updateConfig,
		params.GitClient,
	)
	// An old instance stops instead of reporting a failed update, since nothing failed.
	if errors.Is(err, errOldSelfDetected) {
		// Process log only. Nothing failed, so this is not a notification.
		log.Info().
			Str("notify", "no").
			Msg("Watchtower is running in an old instance's container, stopping this instance")

		// End the scan for event subscribers, with nothing scanned.
		if params.EventBroadcaster != nil {
			params.EventBroadcaster.Publish(events.Event{
				Type:      "scan_completed",
				Timestamp: time.Now().UTC(),
				Data:      events.ScanCompletedData{},
			})
		}

		if params.OnOldSelfDetected != nil {
			params.OnOldSelfDetected()
		}

		return &metrics.Metric{}
	}

	// Perform image cleanup if enabled. This runs before the result is checked,
	// so the old images of containers replaced before an update error are
	// still removed.
	cleanedImages := performImageCleanup(log,
		ctx,
		params.Client,
		updateConfig.Cleanup,
		cleanupImageInfosPtr,
		updateConfig.Timeout,
	)

	// Publish image cleanup event
	if params.EventBroadcaster != nil && len(cleanedImages) > 0 {
		entries := make([]events.ImageCleanupEntry, len(cleanedImages))
		for i, img := range cleanedImages {
			entries[i] = events.ImageCleanupEntry{
				ImageID:       string(img.ImageID),
				ImageName:     img.ImageName,
				ContainerID:   string(img.ContainerID),
				ContainerName: img.ContainerName,
			}
		}

		params.EventBroadcaster.Publish(events.Event{
			Type:      "image_cleanup",
			Timestamp: time.Now().UTC(),
			Data: events.ImageCleanupData{
				Images: entries,
			},
		})
	}

	// Process update result, return metric on failure
	metric := handleUpdateResult(log, result, err, params.Notifier)
	if metric != nil {
		if params.EventBroadcaster != nil {
			errMsg := "unknown error"
			if err != nil {
				errMsg = err.Error()
			}

			params.EventBroadcaster.Publish(events.Event{
				Type:      "scan_failed",
				Timestamp: time.Now().UTC(),
				Data: events.ScanFailedData{
					Error: errMsg,
				},
			})
		}

		return metric
	}

	// Log update report details for debugging
	logUpdateReport(log, result)

	log.Debug().
		Bool("notification_split_by_container", params.NotificationSplitByContainer).
		Bool("notification_report", params.NotificationReport).
		Bool("notifier_present", params.Notifier != nil).
		Msg("About to send notifications")

	// Send notifications about update results
	sendNotifications(log,
		params.Notifier,
		params.NotificationSplitByContainer,
		params.NotificationReport,
		result,
		cleanedImages,
	)

	// Publish scan completed event
	if params.EventBroadcaster != nil {
		scanned, updated, failed, skipped := 0, 0, 0, 0
		if result != nil {
			scanned = len(result.Scanned())
			updated = len(result.Updated())
			failed = len(result.Failed())
			skipped = len(result.Skipped())
		}

		params.EventBroadcaster.Publish(events.Event{
			Type:      "scan_completed",
			Timestamp: time.Now().UTC(),
			Data: events.ScanCompletedData{
				Scanned: scanned,
				Updated: updated,
				Failed:  failed,
				Skipped: skipped,
			},
		})
	}

	// Generate and return metric summarizing the session
	return generateAndLogMetric(log, result)
}

// emptyReport is a non-nil empty report used when sending notifications about errors.
// It prevents panics in notifier implementations that may dereference the report.
type emptyReport struct{}

func (emptyReport) Scanned() []types.ContainerReport   { return nil }
func (emptyReport) Updated() []types.ContainerReport   { return nil }
func (emptyReport) Failed() []types.ContainerReport    { return nil }
func (emptyReport) Skipped() []types.ContainerReport   { return nil }
func (emptyReport) Stale() []types.ContainerReport     { return nil }
func (emptyReport) Fresh() []types.ContainerReport     { return nil }
func (emptyReport) Restarted() []types.ContainerReport { return nil }
func (emptyReport) All() []types.ContainerReport       { return nil }

// handleUpdateResult processes the result of an update operation and returns an appropriate metric.
//
// It checks for errors or nil results, logging accordingly. If an error occurred, it sends a
// notification via the provided notifier (if not nil) to alert about the failure. An update that
// fails partway still returns the report of the work done before the error, so that report is
// notified and counted. On error or nil result, it returns a non-nil metric to indicate the
// failure state. On success, it returns nil to indicate continuation of the update process.
//
// Parameters:
//   - result: The report from the update operation. It may be partial when err is non-nil.
//   - err: Any error encountered during the update.
//   - notifier: The notification system for sending error alerts. It may be nil.
//
// Returns:
//   - *metrics.Metric: A metric marked as a failed scan, with the counts of the partial report
//     on error, or zero counts when there is no report. Nil on success.
func handleUpdateResult(log *zerolog.Logger, result types.Report, err error, notifier types.Notifier) *metrics.Metric {
	// Check for errors during update execution
	if err != nil {
		log.Error().
			Err(err).
			Msg("Update execution failed")

		// Without a partial report, the notification still carries the error.
		report := result
		if report == nil {
			report = emptyReport{}
		}

		// Send notification about the error
		if notifier != nil {
			notifier.SendNotification(report)
		}

		metric := metrics.NewMetric(report)
		metric.ScanFailed = true

		return metric
	}

	// Check if update result is nil
	if result == nil {
		log.Debug().Msg("Update result is nil, returning a failed zero metric")

		return &metrics.Metric{ScanFailed: true}
	}

	return nil
}

// buildSingleContainerReport creates a SingleContainerReport for a specific updated container.
//
// It populates the report with the updated container as the primary item and includes
// all other session results (scanned, failed, skipped, stale, fresh) for comprehensive context.
//
// Parameters:
//   - updatedContainer: The container that was updated.
//   - result: The full session report containing all container statuses.
//
// Returns:
//   - *session.SingleContainerReport: A report focused on the updated container with full session context.
func buildSingleContainerReport(
	updatedContainer types.ContainerReport,
	result types.Report,
) *session.SingleContainerReport {
	return &session.SingleContainerReport{
		UpdatedReports: []types.ContainerReport{updatedContainer},
		ScannedReports: result.Scanned(),
		FailedReports:  result.Failed(),
		SkippedReports: result.Skipped(),
		StaleReports:   result.Stale(),
		FreshReports:   result.Fresh(),
	}
}

// buildSingleRestartedContainerReport creates a SingleContainerReport for a specific restarted container.
//
// It populates the report with the restarted container as the primary item and includes
// all other session results (scanned, failed, skipped, stale, fresh) for comprehensive context.
//
// Parameters:
//   - restartedContainer: The container that was restarted.
//   - result: The full session report containing all container statuses.
//
// Returns:
//   - *session.SingleContainerReport: A report focused on the restarted container with full session context.
func buildSingleRestartedContainerReport(
	restartedContainer types.ContainerReport,
	result types.Report,
) *session.SingleContainerReport {
	return &session.SingleContainerReport{
		RestartedReports: []types.ContainerReport{restartedContainer},
		ScannedReports:   result.Scanned(),
		FailedReports:    result.Failed(),
		SkippedReports:   result.Skipped(),
		StaleReports:     result.Stale(),
		FreshReports:     result.Fresh(),
	}
}

// startNotifications initiates notification batching if a notifier is provided.
//
// It starts the notification process to group update messages, or logs a debug message
// if no notifier is available. When notifications are split by container, it suppresses
// the summary notification to prevent unwanted duplicates.
//
// Parameters:
//   - notifier: The notification system instance for sending update status messages.
//   - notificationSplitByContainer: Boolean flag indicating whether notifications are split by container.
func startNotifications(log *zerolog.Logger, notifier types.Notifier, notificationSplitByContainer bool) {
	if notifier != nil {
		notifier.StartNotification(notificationSplitByContainer)
	} else {
		log.Debug().Msg("Notifier is nil, skipping notification batching")
	}
}

// executeUpdate performs the container update operation and handles errors.
//
// It calls Update with the provided parameters, including the optional Git
// monitor client used for associated containers.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for cancellation and timeouts.
//   - client: Docker client used for container operations.
//   - config: Update parameters.
//   - gitClient: Git monitor client. Nil when Git monitoring is unused.
//
// Returns:
//   - types.Report: Results of the update operation.
//   - []types.RemovedImageInfo: Images eligible for cleanup.
//   - error: Non-nil when Update fails.
func executeUpdate(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	config types.UpdateParams,
	gitClient *git.Client,
) (types.Report, []types.RemovedImageInfo, error) {
	// Log before calling the Update function
	log.Debug().Msg("About to call Update function")

	result, cleanupImageInfos, err := Update(log, ctx, client, config, gitClient)

	// Log after Update function returns
	log.Debug().Msg("Update function returned, about to check cleanup")

	return result, cleanupImageInfos, err
}

// performImageCleanup executes image cleanup if enabled.
//
// It removes old images after updates if the cleanup flag is set.
// When multiple containers share the same old image, the image is only removed once
// (preventing duplicate "Removing image" log entries), but the returned slice includes
// all container associations so that split-by-container notifications report correctly.
// Docker calls use a detached timeout so SIGTERM after a Watchtower self-update
// cannot abort leftover image removal.
//
// Parameters:
//   - ctx: Parent session context. Cancellation is detached for Docker calls.
//   - client: The Docker client instance used for container operations.
//   - cleanup: Boolean indicating whether to perform image cleanup.
//   - cleanupImageInfos: Slice of cleaned image info to be removed.
//   - timeout: Bound for detached cleanup Docker calls. Non-positive uses the restart-policy fallback.
//
// Returns:
//   - []types.RemovedImageInfo: Slice of successfully cleaned image info.
func performImageCleanup(log *zerolog.Logger, ctx context.Context,
	client container.Client,
	cleanup bool,
	cleanupImageInfos []types.RemovedImageInfo,
	timeout time.Duration,
) []types.RemovedImageInfo {
	if !cleanup || len(cleanupImageInfos) == 0 {
		return []types.RemovedImageInfo{}
	}

	// Deduplicate by ImageID so each image is only removed once.
	// This prevents duplicate "Removing image" log entries in non-split notifications
	// when multiple containers share the same old image.
	uniqueByImageID := deduplicateByImageID(cleanupImageInfos)

	// Detach from the session context so SIGTERM after self-update cannot
	// abort leftover image removal.
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		restartPolicyTimeout(timeout),
	)
	defer cancel()

	cleaned, err := RemoveImages(log, cleanupCtx, client, uniqueByImageID)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Failed to clean up some images after update")
	}

	if len(cleaned) == 0 {
		return []types.RemovedImageInfo{}
	}

	// Build a set of successfully removed image IDs.
	removedIDs := make(map[types.ImageID]bool, len(cleaned))
	for _, c := range cleaned {
		removedIDs[c.ImageID] = true
	}

	// Expand to include all container associations for each removed image.
	// This ensures split-by-container notifications report cleanup for every
	// container that used the old image, not just the one that triggered removal.
	result := make([]types.RemovedImageInfo, 0, len(cleanupImageInfos))
	for _, info := range cleanupImageInfos {
		if removedIDs[info.ImageID] {
			result = append(result, info)
		}
	}

	return result
}

// deduplicateByImageID returns a new slice containing only the first occurrence
// of each ImageID. This is used to ensure each image is removed exactly once
// even when multiple containers share the same old image.
//
// Parameters:
//   - images: Slice of RemovedImageInfo that may contain duplicate ImageIDs.
//
// Returns:
//   - []types.RemovedImageInfo: Slice with one entry per unique ImageID.
func deduplicateByImageID(images []types.RemovedImageInfo) []types.RemovedImageInfo {
	seen := make(map[types.ImageID]bool, len(images))
	result := make([]types.RemovedImageInfo, 0, len(images))

	for _, img := range images {
		if !seen[img.ImageID] {
			seen[img.ImageID] = true
			result = append(result, img)
		}
	}

	return result
}

// logUpdateReport logs the update report details for debugging purposes.
//
// It extracts updated container names and logs comprehensive session statistics.
//
// Parameters:
//   - result: The report containing the results of the update operation.
func logUpdateReport(log *zerolog.Logger, result types.Report) {
	// Initialize slice for updated container names
	updatedNames := make([]string, 0, len(result.Updated()))
	// Collect names of all updated containers
	for _, report := range result.Updated() {
		updatedNames = append(updatedNames, report.Name())
	}

	log.Debug().
		Int("scanned", len(result.Scanned())).
		Int("updated", len(result.Updated())).
		Int("failed", len(result.Failed())).
		Strs("updated_names", updatedNames).
		Msg("Report before notification")
}

// sendNotifications handles sending notifications about update results.
//
// It supports both grouped and per-container notifications based on configuration flags,
// including complex logic for splitting notifications by container. The non-split path
// sends notifications asynchronously using a goroutine with proper synchronization
// to ensure the notification completes before the notifier is closed.
//
// Parameters:
//   - notifier: The notification system instance for sending update status messages.
//   - notificationSplitByContainer: Boolean flag enabling separate notifications for each updated container.
//   - notificationReport: Boolean flag enabling report-based notifications.
//   - result: The report containing the results of the update operation.
//   - cleanedImages: Slice of successfully cleaned image info.
func sendNotifications(log *zerolog.Logger, notifier types.Notifier,
	notificationSplitByContainer, notificationReport bool,
	result types.Report,
	cleanedImages []types.RemovedImageInfo,
) {
	// Check if notifier is available
	if notifier != nil {
		// Check if notifications should be split by container
		if notificationSplitByContainer {
			sendSplitNotifications(log, notifier, notificationReport, result, cleanedImages)
		} else if notifier.ShouldSendNotification(result) {
			notifier.SendNotification(result)
		}
	}
}

// sendSplitNotifications handles sending notifications when split by container is enabled.
//
// It processes updated containers and sends either report-based or filtered entry notifications
// based on the notificationReport flag, skipping invalid containers.
// When notificationReport is true, it also sends notifications for monitor-only
// containers from the stale list.
// To prevent duplicate notifications for the same container, a map is used to track
// which container IDs have already been notified during this notification session.
// This tracking mechanism ensures that even if a container appears in multiple lists
// (e.g., due to edge cases in report generation), it receives only one notification,
// maintaining clean and non-redundant communication with users.
//
// Parameters:
//   - notifier: The notification system instance for sending update status messages.
//   - notificationReport: Boolean flag enabling report-based notifications.
//   - result: The report containing the results of the update operation.
//   - cleanedImages: Slice of successfully cleaned image info.
func sendSplitNotifications(log *zerolog.Logger, notifier types.Notifier,
	notificationReport bool,
	result types.Report,
	cleanedImages []types.RemovedImageInfo,
) {
	// Map to track notified container IDs to prevent duplicate notifications.
	// Key is the full container ID string for uniqueness, value is boolean indicating
	// whether a notification has been sent for this container.
	// This map is scoped to the function to ensure tracking is per-notification-session.
	notified := make(map[string]bool)

	log.Debug().
		Int("updated_count", len(result.Updated())).
		Int("restarted_count", len(result.Restarted())).
		Int("stale_count", len(result.Stale())).
		Int("failed_count", len(result.Failed())).
		Int("skipped_count", len(result.Skipped())).
		Int("fresh_count", len(result.Fresh())).
		Int("scanned_count", len(result.Scanned())).
		Msg("Split notifications: container counts by category")

	if notificationReport {
		// Log updated containers for debugging
		updatedNames := make([]string, 0, len(result.Updated()))
		for _, report := range result.Updated() {
			updatedNames = append(updatedNames, report.Name())
		}

		log.Debug().
			Strs("updated_containers", updatedNames).
			Msg("Split notifications: sending report notifications for updated containers")

		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Updated(),
			result:        result,
			category:      "updated",
			logBeforeSend: false,
			include:       func(r types.ContainerReport) bool { return true },
			buildReport:   buildSingleContainerReport,
		})
		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Restarted(),
			result:        result,
			category:      "restarted",
			logBeforeSend: false,
			include:       func(r types.ContainerReport) bool { return true },
			buildReport:   buildSingleRestartedContainerReport,
		})
		// Monitor-only containers appear in Stale when notificationReport is true.
		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Stale(),
			result:        result,
			category:      "stale",
			logBeforeSend: false,
			include:       func(r types.ContainerReport) bool { return r.IsMonitorOnly() },
			buildReport:   buildSingleContainerReport,
		})

		// Cooldown-skipped containers use a different report shape and stay inline.
		notifySkippedCooldownContainers(log, notifier, notified, result)
	} else {
		// Log-mode split: hook-captured entries are filtered by container inside
		// SendNotification when the report is a SingleContainerReport.
		//
		// Image removal logs once per ImageID without container scope. Expanded
		// cleanedImages retains every container association for shared images, so
		// emit a hooked container-scoped cleanup event for each entry so both
		// containers that shared an old image receive cleanup in split mode.
		for _, img := range cleanedImages {
			log.Info().
				Str("notify", "yes").
				Str("container_name", img.ContainerName).
				Str("image_name", img.ImageName).
				Str("image_id", img.ImageID.ShortID()).
				Msg("Removing image")
		}

		updatedNames := make([]string, 0, len(result.Updated()))
		for _, report := range result.Updated() {
			updatedNames = append(updatedNames, report.Name())
		}

		log.Debug().
			Strs("updated_containers", updatedNames).
			Msg("Split notifications: sending per-container notifications (log mode)")

		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Updated(),
			result:        result,
			category:      "updated",
			logBeforeSend: true,
			include:       func(r types.ContainerReport) bool { return true },
			buildReport:   buildSingleContainerReport,
		})
		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Restarted(),
			result:        result,
			category:      "restarted",
			logBeforeSend: true,
			include:       func(r types.ContainerReport) bool { return true },
			buildReport:   buildSingleRestartedContainerReport,
		})
		notifyContainers(notifyContainersParams{
			log:           log,
			notifier:      notifier,
			notified:      notified,
			reports:       result.Stale(),
			result:        result,
			category:      "monitor-only stale",
			logBeforeSend: true,
			include:       func(r types.ContainerReport) bool { return r.IsMonitorOnly() },
			buildReport:   buildSingleContainerReport,
		})

		notifySkippedCooldownContainers(log, notifier, notified, result)
	}

	log.Debug().Msg("Finished sending notifications")
}

// notifyContainersParams groups inputs for notifyContainers.
type notifyContainersParams struct {
	log           *zerolog.Logger
	notifier      types.Notifier
	notified      map[string]bool
	reports       []types.ContainerReport
	result        types.Report
	category      string
	logBeforeSend bool
	include       func(types.ContainerReport) bool
	buildReport   func(types.ContainerReport, types.Report) *session.SingleContainerReport
}

// notifyContainers sends per-container notifications for a report collection.
//
// It applies nil and empty-name filtering, container-ID de-duplication via notified,
// optional category eligibility (include), and ShouldSendNotification before send.
//
// Parameters:
//   - params: Named inputs for the category-specific notification pass.
func notifyContainers(params notifyContainersParams) {
	for _, report := range params.reports {
		if report == nil {
			params.log.Debug().Msg("Encountered nil " + params.category + " container report, skipping")

			continue
		}

		if strings.TrimSpace(report.Name()) == "" {
			params.log.Debug().
				Str("container_id", report.ID().ShortID()).
				Msg("Encountered " + params.category + " container with empty name, skipping notification")

			continue
		}

		if !params.include(report) {
			continue
		}

		containerID := string(report.ID())
		if params.notified[containerID] {
			continue
		}

		if params.logBeforeSend {
			params.log.Debug().
				Str("container", report.Name()).
				Str("image", report.ImageName()).
				Msg("Sending individual notification for " + params.category + " container")
		}

		singleContainerReport := params.buildReport(report, params.result)
		if params.notifier.ShouldSendNotification(singleContainerReport) {
			params.notifier.SendNotification(singleContainerReport)
		}

		params.notified[containerID] = true
	}
}

// notifySkippedCooldownContainers sends notifications for cooldown-skipped containers.
//
// Parameters:
//   - log: Logger (unused currently, reserved for consistency with sibling helpers).
//   - notifier: Notification client.
//   - notified: Map of container IDs already notified in this session.
//   - result: Full session report.
func notifySkippedCooldownContainers(
	_ *zerolog.Logger,
	notifier types.Notifier,
	notified map[string]bool,
	result types.Report,
) {
	for _, report := range result.Skipped() {
		if report == nil {
			continue
		}

		containerStatus, ok := report.(*session.ContainerStatus)
		if !ok || containerStatus.CooldownDelay() == "" {
			continue
		}

		containerID := string(report.ID())
		if notified[containerID] {
			continue
		}

		singleSkippedReport := &session.SingleContainerReport{
			SkippedReports:   []types.ContainerReport{report},
			ScannedReports:   result.Scanned(),
			UpdatedReports:   result.Updated(),
			RestartedReports: result.Restarted(),
			FailedReports:    result.Failed(),
			StaleReports:     result.Stale(),
			FreshReports:     result.Fresh(),
		}
		if notifier.ShouldSendNotification(singleSkippedReport) {
			notifier.SendNotification(singleSkippedReport)
		}

		notified[containerID] = true
	}
}

// generateAndLogMetric creates a metric from the update results and logs it.
//
// It builds a session summary metric and writes an Info completion line on the
// process logger. The line carries notify=no so the Shoutrrr hook does not send
// a second notification after SendNotification has already flushed the session
// batch. Without that field, legacy templates would emit a standalone
// "Update session completed" message with only the scanned, updated, failed,
// and skipped counts.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil.
//   - result: The report containing the results of the update operation.
//
// Returns:
//   - *metrics.Metric: A pointer to a metric object summarizing the update session.
func generateAndLogMetric(log *zerolog.Logger, result types.Report) *metrics.Metric {
	metricResults := metrics.NewMetric(result)

	// Process log only. Session content was already notified via sendNotifications.
	log.Info().
		Str("notify", "no").
		Int("scanned", metricResults.Scanned).
		Int("updated", metricResults.Updated).
		Int("failed", metricResults.Failed).
		Int("skipped", metricResults.Skipped).
		Msg("Update session completed")

	return metricResults
}
