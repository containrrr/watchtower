package logging

import (
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// APIVersionProvider reports the Docker API version for startup messaging.
//
// Defined here (rather than depending on pkg/container.Client) so this package
// stays free of container→flags import cycles when flags configures logging.
type APIVersionProvider interface {
	GetVersion() string
}

// StartupParams holds resolved process values for startup messaging.
//
// Callers must populate these from config.Load output. Do not read CLI flags here.
// Schedule and mode fields live on the embedded ScheduleInfo (single source of truth).
type StartupParams struct {
	// ScheduleInfo holds run-once, update-on-start, HTTP API, and next-run schedule values.
	ScheduleInfo

	// Logger is the zerolog logger used for startup messages. Required when NoStartupMessage is false.
	Logger *zerolog.Logger
	// NoStartupMessage suppresses all startup logs and notifications when true.
	NoStartupMessage bool
	// Filtering is a human-readable description of the container filter.
	Filtering string
	// Scope is the operational scope name, or empty when unset.
	Scope string
	// Client is the Docker client used for API version reporting.
	Client APIVersionProvider
	// Notifier sends batched startup messages when not suppressed.
	Notifier types.Notifier
	// Version is the Watchtower version string.
	Version string
	// DiskSpaceMaxBytes is the parsed Docker image-usage block threshold, or 0 if unset.
	DiskSpaceMaxBytes int64
	// DiskSpaceWarnBytes is the parsed Docker image-usage warning threshold, or 0 if unset.
	DiskSpaceWarnBytes int64
}

// WriteStartupMessage logs or notifies startup information from resolved configuration.
//
// It reports Watchtower's version, notification setup, container filtering details,
// scheduling information, and HTTP API status. Callers that suppress startup messages
// set NoStartupMessage and return without requiring Logger.
//
// Parameters:
//   - params: Resolved startup messaging inputs from config.Load (no CLI flag reads).
//     When NoStartupMessage is false, Logger must be non-nil and should be the hooked
//     process logger (after Notifier.RegisterHook) so batching captures startup lines.
func WriteStartupMessage(params StartupParams) {
	// If startup messages are suppressed, skip all logging and notifier batching.
	if params.NoStartupMessage {
		return
	}

	if params.Logger == nil {
		panic("logging.WriteStartupMessage: Logger is required when NoStartupMessage is false")
	}

	// Batch startup lines through the notifier when present (suppression already returned).
	startupLog := setupStartupLogger(params.Logger, params.Notifier)

	var apiVersion string
	if params.Client != nil {
		apiVersion = params.Client.GetVersion()
	}

	startupLog.Info().
		Msg("Watchtower " + params.Version + " using Docker API v" + apiVersion)

	if params.DiskSpaceMaxBytes > 0 || params.DiskSpaceWarnBytes > 0 {
		startupLog.Info().
			Int64("disk_space_max", params.DiskSpaceMaxBytes).
			Int64("disk_space_warn", params.DiskSpaceWarnBytes).
			Msg("Docker image usage budget enabled")
	}

	// Log details about configured notifiers or lack thereof.
	var notifierNames []string
	if params.Notifier != nil {
		notifierNames = params.Notifier.GetNames()
	}

	logNotifierInfo(startupLog, notifierNames)

	// Log filtering information, using structured logging for scope when set.
	if params.Scope != "" {
		startupLog.Info().
			Str("scope", params.Scope).
			Msg("Only checking containers in scope")
	} else {
		startupLog.Debug().Msg(params.Filtering)
	}

	// Log scheduling or run mode information based on configuration.
	logScheduleInfo(startupLog, params.ScheduleInfo)

	// Send batched notifications if not suppressed, ensuring startup info reaches users.
	if params.Notifier != nil {
		params.Notifier.SendNotification(nil)
	}

	// Warn about trace-level logging if enabled, as it may expose sensitive data.
	if startupLog.GetLevel() <= zerolog.TraceLevel {
		startupLog.Warn().
			Msg("Trace-level logging enabled: Sensitive credentials and tokens may be included in logs")
	}
}

// setupStartupLogger prepares the logger for startup messages and starts notifier batching.
//
// Callers that suppress startup messages must return before invoking this helper.
// When notifier is non-nil, StartNotification batches subsequent startup lines for
// a single SendNotification.
//
// Parameters:
//   - log: The zerolog logger used for startup messages.
//   - notifier: The notification system instance for batching messages, or nil.
//
// Returns:
//   - *zerolog.Logger: The logger to use for writing startup messages.
func setupStartupLogger(log *zerolog.Logger, notifier types.Notifier) *zerolog.Logger {
	if notifier != nil {
		notifier.StartNotification(false)
	}

	return log
}

// logNotifierInfo logs details about the notification setup for Watchtower.
//
// It reports the list of configured notifier names (for example "email, slack") or
// indicates that no notifications are set up.
//
// Parameters:
//   - log: The zerolog logger used to write the notification information.
//   - notifierNames: Names of configured notifiers.
func logNotifierInfo(log *zerolog.Logger, notifierNames []string) {
	if len(notifierNames) > 0 {
		log.Info().Msg("Using notifications: " + strings.Join(notifierNames, ", "))
	} else {
		log.Info().Msg("Using no notifications")
	}
}

// ScheduleInfo holds resolved schedule and mode values for startup schedule messaging.
//
// Values come from config.Load projections rather than CLI flag reads.
type ScheduleInfo struct {
	// RunOnce indicates a single update run then exit.
	RunOnce bool
	// UpdateOnStart is the effective update-on-start value, or nil when unset or false.
	UpdateOnStart *bool
	// HTTPAPIUpdate is true when the HTTP update API is enabled.
	HTTPAPIUpdate bool
	// HTTPAPIPeriodicPolls is true when scheduled polls run with the HTTP API.
	HTTPAPIPeriodicPolls bool
	// Sched is the time of the first scheduled run, or zero if none.
	Sched time.Time
}

// logScheduleInfo logs information about the scheduling or run mode configuration.
//
// It handles scheduled runs with timing details, one-time updates, or indicates no periodic runs,
// ensuring users understand when and how updates will occur. It also warns about flag conflicts
// such as when both run-once and update-on-start are enabled.
//
// Parameters:
//   - log: The zerolog logger used to write the schedule information.
//   - info: Resolved schedule and mode values (no flag reads).
func logScheduleInfo(log *zerolog.Logger, info ScheduleInfo) {
	// Use provided update-on-start value when set. Otherwise treat as disabled.
	var updateOnStartVal bool
	if info.UpdateOnStart != nil {
		updateOnStartVal = *info.UpdateOnStart
	}

	// Check if run-once is enabled.
	if info.RunOnce {
		// Warn if disregarding update-on-start when already performing a one-time update.
		if updateOnStartVal {
			log.Warn().Msg("Run once mode: Disregarding update on start")
		} else {
			log.Info().Msg("Running a one time update")
		}

		return
	}

	// Check if update on start is enabled.
	if updateOnStartVal {
		log.Info().Msg("Update on startup enabled: Performing immediate check")
	}

	// Handle HTTP API update configurations.
	if info.HTTPAPIUpdate {
		if info.HTTPAPIPeriodicPolls {
			log.Info().Msg("HTTP API and periodic updates enabled")
		} else {
			log.Info().Msg("HTTP API enabled and periodic updates disabled")

			return
		}
	}

	// Log details of the next scheduled run if scheduling is active.
	if !info.Sched.IsZero() {
		until := util.FormatDuration(time.Until(info.Sched))
		// Example: Next scheduled run: 2025-10-22 00:31:25 MST in 24 hours.
		log.Info().Msg(
			"Next scheduled run: " + info.Sched.Format(
				"2006-01-02 15:04:05 MST",
			) + " in " + until,
		)
	}

	// Default periodic updates are enabled.
	if !updateOnStartVal && !info.HTTPAPIUpdate && info.Sched.IsZero() {
		log.Info().Msg("Periodic updates are enabled with default schedule")
	}
}
