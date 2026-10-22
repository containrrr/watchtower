// Package notify holds notification settings for the config domain.
package notify

// Notify holds resolved notification configuration for the process.
//
// Values come from CLI flags and environment variables. Pass this value to
// notifications.NewNotifier to build the client used for update status messages.
type Notify struct {
	// URLs are Shoutrrr notification service URLs (--notification-url / WATCHTOWER_NOTIFICATION_URL).
	URLs []string
	// LegacyTypes are deprecated notification type names: email, slack, msteams, gotify
	// (--notifications / WATCHTOWER_NOTIFICATIONS). Prefer Shoutrrr URLs instead.
	LegacyTypes []string
	// Level is the log level used for sending notifications
	// (--notifications-level / WATCHTOWER_NOTIFICATIONS_LEVEL).
	Level string
	// Template is the Shoutrrr text/template for messages
	// (--notification-template / WATCHTOWER_NOTIFICATION_TEMPLATE).
	Template string
	// TemplateFile is an optional path to a template file. When set it overrides Template
	// (--notification-template-file / WATCHTOWER_NOTIFICATION_TEMPLATE_FILE).
	TemplateFile string
	// Report enables report-based notification templates
	// (--notification-report / WATCHTOWER_NOTIFICATION_REPORT).
	Report bool
	// SplitByContainer sends a separate notification for each updated container
	// (--notification-split-by-container / WATCHTOWER_NOTIFICATION_SPLIT_BY_CONTAINER).
	SplitByContainer bool
	// SkipTitle omits the title parameter for services that support it
	// (--notification-skip-title / WATCHTOWER_NOTIFICATION_SKIP_TITLE).
	SkipTitle bool
	// LogStdout writes notification logs to stdout instead of logging to stderr
	// (--notification-log-stdout / WATCHTOWER_NOTIFICATION_LOG_STDOUT).
	LogStdout bool
	// DelaySeconds is the delay before sending notifications, in seconds
	// (--notifications-delay / WATCHTOWER_NOTIFICATIONS_DELAY).
	DelaySeconds int
	// Hostname overrides the system hostname in notification titles
	// (--notifications-hostname / WATCHTOWER_NOTIFICATIONS_HOSTNAME).
	Hostname string
	// TitleTag prefixes the notification title
	// (--notification-title-tag / WATCHTOWER_NOTIFICATION_TITLE_TAG).
	TitleTag string
	// EmailSubjectTag is the deprecated email subject tag fallback when TitleTag is empty
	// (--notification-email-subjecttag / WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG).
	EmailSubjectTag string
	// Legacy holds deprecated per-type notification settings used only when LegacyTypes is set.
	Legacy Legacy
}

// Legacy holds deprecated notification-type-specific settings.
//
// These fields support the legacy email, Slack, MSTeams, and Gotify flag sets.
// Prefer --notification-url with the appropriate Shoutrrr scheme instead.
//
// TODO: Remove Legacy when legacy notification types are removed in v2.
//
//nolint:godox
type Legacy struct {
	// EmailFrom is the SMTP From address (--notification-email-from).
	EmailFrom string
	// EmailTo is the SMTP To address (--notification-email-to).
	EmailTo string
	// EmailServer is the SMTP server host (--notification-email-server).
	EmailServer string
	// EmailUser is the SMTP username (--notification-email-server-user).
	EmailUser string
	// EmailPassword is the SMTP password (--notification-email-server-password).
	EmailPassword string
	// EmailPort is the SMTP server port (--notification-email-server-port).
	EmailPort int
	// EmailTLSSkipVerify skips SMTP TLS verification (--notification-email-server-tls-skip-verify).
	EmailTLSSkipVerify bool
	// EmailDelay is the legacy email send delay in seconds (--notification-email-delay).
	EmailDelay int
	// SlackHookURL is the Slack/Discord webhook URL (--notification-slack-hook-url).
	SlackHookURL string
	// SlackIdentifier identifies this Watchtower instance in Slack messages.
	SlackIdentifier string
	// SlackChannel overrides the webhook default channel (--notification-slack-channel).
	SlackChannel string
	// SlackIconEmoji is an emoji code for the message icon (--notification-slack-icon-emoji).
	SlackIconEmoji string
	// SlackIconURL is an image URL for the message icon (--notification-slack-icon-url).
	SlackIconURL string
	// MSTeamsHook is the Microsoft Teams webhook URL (--notification-msteams-hook).
	MSTeamsHook string
	// GotifyURL is the Gotify server URL (--notification-gotify-url).
	GotifyURL string
	// GotifyToken is the Gotify application token (--notification-gotify-token).
	GotifyToken string
	// GotifyTLSSkipVerify skips Gotify TLS verification (--notification-gotify-tls-skip-verify).
	GotifyTLSSkipVerify bool
}
