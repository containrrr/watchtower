package flags

import "github.com/spf13/pflag"

// RegisterLegacyNotificationFlags registers all the flags related to the old notification system
func RegisterLegacyNotificationFlags(flags *pflag.FlagSet) {
	depFlags := NewDeprecator(flags, "use notification-url instead")
	depFlags.Deprecate = false

	depFlags.Prefix = "notification-email-"

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_FROM"),
	depFlags.String("from", "", "Address to send notification emails from")

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_TO"),
	depFlags.String("to", "", "Address to send notification emails to")

	//viper.GetInt("WATCHTOWER_NOTIFICATION_EMAIL_DELAY"),
	depFlags.Int("delay", 0, "Delay before sending notifications, expressed in seconds")

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER"),
	depFlags.String("server", "", "SMTP server to send notification emails through")

	// viper.GetInt("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT"),
	depFlags.Int("server-port", 25, "SMTP server port to send notification emails through")

	// viper.GetBool("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY"),
	depFlags.Bool("server-tls-skip-verify", false, `Controls whether watchtower verifies the SMTP server's certificate chain and host name.
Should only be used for testing.`)

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER"),
	depFlags.String("server-user", "", "SMTP server user for sending notifications")

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD"),
	depFlags.String("server-password", "", "SMTP server password for sending notifications")

	// viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG"),
	depFlags.String("subjecttag", "", "Subject prefix tag for notifications via mail")

	depFlags.Prefix = "notification-slack-"

	// viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL"),
	depFlags.String("hook-url", "", "The Slack Hook URL to send notifications to")

	// viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER"),
	depFlags.String("identifier", "watchtower", "A string which will be used to identify the messages coming from this watchtower instance")

	// viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_CHANNEL"),
	depFlags.String("channel", "", "A string which overrides the webhook's default channel. Example: #my-custom-channel")

	// viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI"),
	depFlags.String("icon-emoji", "", "An emoji code string to use in place of the default icon")

	// viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_ICON_URL"),
	depFlags.String("icon-url", "", "An icon image URL string to use in place of the default icon")

	depFlags.Prefix = "notification-msteams-"

	// viper.GetString("WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL"),
	depFlags.String("hook", "", "The MSTeams WebHook URL to send notifications to")

	// viper.GetBool("WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA"),
	depFlags.Bool("data", false, "The MSTeams notifier will try to extract log entry fields as MSTeams message facts")

	depFlags.Prefix = "notification-gotify-"

	// viper.GetString("WATCHTOWER_NOTIFICATION_GOTIFY_URL"),
	depFlags.String("url", "", "The Gotify URL to send notifications to")

	// viper.GetString("WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN"),
	depFlags.String("token", "", "The Gotify Application required to query the Gotify API")

	// viper.GetBool("WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY"),
	depFlags.Bool("tls-skip-verify", false, `Controls whether watchtower verifies the Gotify server's certificate chain and host name.
Should only be used for testing.`)

}
