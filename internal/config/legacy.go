package config

import (
	"github.com/spf13/cobra"
)

// RegisterLegacyNotificationFlags registers all the flags related to the old notification system
func RegisterLegacyNotificationFlags(cmd *cobra.Command) {
	ob := NewOptBuilder(cmd.PersistentFlags())
	// Hide all legacy notification flags from the `--help` to reduce clutter
	ob.Hide = true

	ob.String(NotificationEmailFrom, "",
		"Address to send notification emails from", "WATCHTOWER_NOTIFICATION_EMAIL_FROM")

	ob.String(NotificationEmailTo, "",
		"Address to send notification emails to", "WATCHTOWER_NOTIFICATION_EMAIL_TO")

	ob.Int(NotificationEmailDelay, 0,
		"Delay before sending notifications, expressed in seconds", "WATCHTOWER_NOTIFICATION_EMAIL_DELAY")
	_ = ob.Flags.MarkDeprecated(string(NotificationEmailDelay),
		"use "+string(NotificationsDelay)+" instead")

	ob.String(NotificationEmailServer, "",
		"SMTP server to send notification emails through", "WATCHTOWER_NOTIFICATION_EMAIL_SERVER")

	ob.Int(NotificationEmailServerPort, 25,
		"SMTP server port to send notification emails through", "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT")

	ob.Bool(NotificationEmailServerTLSSkipVerify, false,
		`Controls whether watchtower verifies the SMTP server's certificate chain and host name.
Should only be used for testing.`,
		"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY")

	ob.String(NotificationEmailServerUser, "",
		"SMTP server user for sending notifications",
		"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER")

	ob.String(NotificationEmailServerPassword, "",
		"SMTP server password for sending notifications",
		"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD")

	ob.String(NotificationEmailSubjectTag, "",
		"Subject prefix tag for notifications via mail",
		"WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG")

	ob.String(NotificationSlackHookURL, "",
		"The Slack Hook URL to send notifications to",
		"WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL")

	ob.String(NotificationSlackIdentifier, "watchtower",
		"A string which will be used to identify the messages coming from this watchtower instance",
		"WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER")

	ob.String(NotificationSlackChannel, "",
		"A string which overrides the webhook's default channel. Example: #my-custom-channel",
		"WATCHTOWER_NOTIFICATION_SLACK_CHANNEL")

	ob.String(NotificationSlackIconEmoji, "",
		"An emoji code string to use in place of the default icon",
		"WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI")

	ob.String(NotificationSlackIconURL, "",
		"An icon image URL string to use in place of the default icon",
		"WATCHTOWER_NOTIFICATION_SLACK_ICON_URL")

	ob.String(NotificationMSTeamsHook, "",
		"The MSTeams WebHook URL to send notifications to",
		"WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL")

	ob.Bool(NotificationMSTeamsData, false,
		"The MSTeams notifier will try to extract log entry fields as MSTeams message facts",
		"WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA")

	ob.String(NotificationGotifyURL, "",
		"The Gotify URL to send notifications to", "WATCHTOWER_NOTIFICATION_GOTIFY_URL")

	ob.String(NotificationGotifyToken, "",
		"The Gotify Application required to query the Gotify API", "WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN")

	ob.Bool(NotificationGotifyTLSSkipVerify, false,
		`Controls whether watchtower verifies the Gotify server's certificate chain and host name.
Should only be used for testing.`,
		"WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY")

}
