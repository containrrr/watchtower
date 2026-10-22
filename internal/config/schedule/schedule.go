// Package schedule holds poll interval and cron schedule settings.
package schedule

// Schedule holds when periodic updates run.
type Schedule struct {
	// IntervalSeconds is the poll interval in seconds when not using a cron expression.
	IntervalSeconds int
	// Spec is the cron schedule expression (may be derived from IntervalSeconds).
	Spec string
	// UpdateOnStart runs an update check immediately on startup.
	UpdateOnStart bool
}
