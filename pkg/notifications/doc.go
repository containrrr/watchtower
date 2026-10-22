// Package notifications sends Watchtower's notifications through Shoutrrr.
//
// The notifier collects log entries during an update session and sends them
// with the session report, rendered through the default or a custom template.
// The legacy email, Slack, Microsoft Teams, and Gotify settings are converted
// to Shoutrrr URLs and are deprecated in favor of the notification-url
// configuration option.
package notifications
