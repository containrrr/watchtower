package types

import "github.com/rs/zerolog"

// Notifier defines the common interface for notification services.
//
// Log events are received via a zerolog hook registered with RegisterHook.
// The notifier does not expose queued entry state. Batching, filtering, and
// deduplication happen inside the implementation driven by the hook callback.
type Notifier interface {
	StartNotification(suppressSummary bool) // Begin queuing messages.
	SendNotification(report Report)         // Send queued messages with report.
	// RegisterHook attaches this notifier as a zerolog.Hook on the given logger.
	// Implementations update *log in place to the hooked logger so the composition
	// root continues using the same pointer for subsequent application logging.
	RegisterHook(log *zerolog.Logger)
	GetNames() []string // Service names.
	GetURLs() []string  // Service URLs.
	Close()             // Stop and flush notifications.

	// ShouldSendNotification checks if a notification should be sent for the given
	// report based on the notifier's configuration.
	ShouldSendNotification(report Report) bool
}
