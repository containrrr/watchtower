// Package notify registers notification-related CLI and environment flags.
package notify

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// DefaultEmailServerPort is the static default SMTP port.
const DefaultEmailServerPort = 25

// Specs returns notify domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Notify flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "notification-url",
			Kind:      spec.KindStringArray,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_NOTIFICATION_URL"},
			ListParse: spec.ListNotificationURLs,
			Help:      "The shoutrrr URL to send notifications to",
		},
		{
			Name:    "notifications-level",
			Kind:    spec.KindString,
			Default: "info",
			EnvKeys: []string{"WATCHTOWER_NOTIFICATIONS_LEVEL"},
			Help:    "The log level used for sending notifications. Possible values: panic, fatal, error, warn, info or debug",
		},
		{
			Name:    "notifications-delay",
			Kind:    spec.KindInt,
			Default: 0,
			EnvKeys: []string{"WATCHTOWER_NOTIFICATIONS_DELAY"},
			Help:    "Delay before sending notifications, expressed in seconds",
		},
		{
			Name:    "notifications-hostname",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_NOTIFICATIONS_HOSTNAME"},
			Help:    "Custom hostname for notification titles",
		},
		{
			Name:    "notification-template",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_TEMPLATE"},
			Help:    "The shoutrrr text/template for the messages",
		},
		{
			Name:    "notification-template-file",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_TEMPLATE_FILE"},
			Help:    "Path to a file containing the Shoutrrr text/template for the messages",
		},
		{
			Name:    "notification-report",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_REPORT"},
			Help:    "Use the session report as the notification template data",
		},
		{
			Name:    "notification-title-tag",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_TITLE_TAG"},
			Help:    "Title prefix tag for notifications",
		},
		{
			Name:    "notification-skip-title",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_SKIP_TITLE"},
			Help:    "Do not pass the title param to notifications",
		},
		{
			Name:    "notification-log-stdout",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_LOG_STDOUT"},
			Help:    "Write notification logs to stdout instead of logging (to stderr)",
		},
		{
			Name:    "notification-split-by-container",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NOTIFICATION_SPLIT_BY_CONTAINER"},
			Help:    "Send separate notifications for each updated container instead of grouping them",
		},

		{
			Name:       "notifications",
			Shorthand:  "n",
			Kind:       spec.KindStringSlice,
			Default:    []string{},
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATIONS"},
			ListParse:  spec.ListCommaOrSpace,
			Deprecated: "Use --notification-url with the appropriate Shoutrrr URL scheme instead.",
			Help:       "Notification types to send [legacy types (email, slack, msteams, gotify)].",
		},
		{
			Name:       "notification-email-from",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_FROM"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "Address to send notification emails from",
		},
		{
			Name:       "notification-email-to",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_TO"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "Address to send notification emails to",
		},
		{
			Name:       "notification-email-delay",
			Kind:       spec.KindInt,
			Default:    0,
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_DELAY"},
			Deprecated: "Use --notifications-delay instead.",
			Help:       "Delay before sending notifications, expressed in seconds",
		},
		{
			Name:       "notification-email-server",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SERVER"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "SMTP server to send notification emails through",
		},
		{
			Name:       "notification-email-server-port",
			Kind:       spec.KindInt,
			Default:    DefaultEmailServerPort,
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "SMTP server port to send notification emails through",
		},
		{
			Name:       "notification-email-server-tls-skip-verify",
			Kind:       spec.KindBool,
			Default:    false,
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "Controls whether watchtower verifies the SMTP server's certificate chain and host name. Should only be used for testing.",
		},
		{
			Name:       "notification-email-server-user",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "SMTP server user for sending notifications",
		},
		{
			Name:       "notification-email-server-password",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD"},
			Deprecated: "Use --notification-url with an smtp:// URL.",
			Help:       "SMTP server password for sending notifications",
		},
		{
			Name:       "notification-email-subjecttag",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG"},
			Deprecated: "Use --notification-title-tag instead.",
			Help:       "Subject prefix tag for notifications via mail",
		},
		{
			Name:       "notification-slack-hook-url",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL"},
			Deprecated: "Use --notification-url with a slack:// or discord:// URL.",
			Help:       "The Slack Hook URL to send notifications to",
		},
		{
			Name:       "notification-slack-identifier",
			Kind:       spec.KindString,
			Default:    "watchtower",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER"},
			Deprecated: "Use --notification-url with a slack:// or discord:// URL.",
			Help:       "A string which will be used to identify the messages coming from this watchtower instance",
		},
		{
			Name:       "notification-slack-channel",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_SLACK_CHANNEL"},
			Deprecated: "Use --notification-url with a slack:// or discord:// URL.",
			Help:       "A Slack channel ID which overrides the default channel of a legacy webhook. Example: C0123456789",
		},
		{
			Name:       "notification-slack-icon-emoji",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI"},
			Deprecated: "Use --notification-url with a slack:// or discord:// URL.",
			Help:       "An emoji code string to use in place of the default icon",
		},
		{
			Name:       "notification-slack-icon-url",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_SLACK_ICON_URL"},
			Deprecated: "Use --notification-url with a slack:// or discord:// URL.",
			Help:       "An icon image URL string to use in place of the default icon",
		},
		{
			Name:       "notification-msteams-hook",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL"},
			Deprecated: "Use --notification-url with a teams:// URL.",
			Help:       "The MSTeams WebHook URL to send notifications to",
		},
		{
			Name:       "notification-gotify-url",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_GOTIFY_URL"},
			Deprecated: "Use --notification-url with a gotify:// URL.",
			Help:       "The Gotify URL to send notifications to",
		},
		{
			Name:       "notification-gotify-token",
			Kind:       spec.KindString,
			Default:    "",
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN"},
			Deprecated: "Use --notification-url with a gotify:// URL.",
			Help:       "The Gotify Application required to query the Gotify API",
		},
		{
			Name:       "notification-gotify-tls-skip-verify",
			Kind:       spec.KindBool,
			Default:    false,
			EnvKeys:    []string{"WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY"},
			Deprecated: "Use --notification-url with a gotify:// URL.",
			Help:       "Controls whether watchtower verifies the Gotify server's certificate chain and host name. Should only be used for testing.",
		},
	}
}

// Register adds notification flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
