// Package notifications provides the notification client for sending Watchtower update messages.
// It integrates with Shoutrrr for service delivery, supporting custom templates, batching, and JSON marshaling.
//
// Key components:
//   - NewNotifier: Constructs the client from a *zerolog.Logger and internal/config/notify.Notify (notifier.go).
//   - RegisterHook: Attaches the notifier as a zerolog.Hook on the process logger (shoutrrr.go).
//   - Shoutrrr Integration: Handles message sending and batching (shoutrrr.go).
//   - JSON Marshaling: Formats notification data (json.go).
//
// Note: The legacy notification types (email, slack, msteams, gotify) and their individual flags
// (e.g., --notification-email-from, --notification-slack-hook-url) are deprecated.
// Use --notification-url with the appropriate shoutrrr URL scheme instead.
// See the deprecation notices on specific types and functions for details.
//
// Usage example (after config.Load):
//
//	notifier := notifications.NewNotifier(log, cfg.Notify)
//	notifier.RegisterHook(log) // updates *log in place to the hooked logger
//	notifier.StartNotification(false)
//	// ... application logging via the hooked logger is batched ...
//	notifier.SendNotification(report)
//	notifier.Close()
//
// The package uses Shoutrrr for service abstraction and custom templates.
// Logging uses *zerolog.Logger passed from the composition root (no global logger).
//
// RegisterHook captures zerolog events for notification batching. All process logging
// uses github.com/rs/zerolog. There is no dual-logger hook shim.
package notifications
