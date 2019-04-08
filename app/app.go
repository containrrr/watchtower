package app

import (
	"time"

	"github.com/urfave/cli"
)

// SetupCliFlags registers flags on the supplied urfave app
func SetupCliFlags(app *cli.App) {
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.IntFlag{
			Name:   "interval, i",
			Usage:  "poll interval (in seconds)",
			Value:  300,
			EnvVar: "WATCHTOWER_POLL_INTERVAL",
		},
		cli.StringFlag{
			Name:   "schedule, s",
			Usage:  "the cron expression which defines when to update",
			EnvVar: "WATCHTOWER_SCHEDULE",
		},
		cli.BoolFlag{
			Name:   "no-pull",
			Usage:  "do not pull new images",
			EnvVar: "WATCHTOWER_NO_PULL",
		},
		cli.BoolFlag{
			Name:   "no-restart",
			Usage:  "do not restart containers",
			EnvVar: "WATCHTOWER_NO_RESTART",
		},
		cli.BoolFlag{
			Name:   "cleanup",
			Usage:  "remove old images after updating",
			EnvVar: "WATCHTOWER_CLEANUP",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.DurationFlag{
			Name:   "stop-timeout",
			Usage:  "timeout before container is forcefully stopped",
			Value:  time.Second * 10,
			EnvVar: "WATCHTOWER_TIMEOUT",
		},
		cli.BoolFlag{
			Name:   "label-enable",
			Usage:  "watch containers where the com.centurylinklabs.watchtower.enable label is true",
			EnvVar: "WATCHTOWER_LABEL_ENABLE",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
		},
		cli.StringSliceFlag{
			Name:   "notifications",
			Value:  &cli.StringSlice{},
			Usage:  "notification types to send (valid: email, slack, msteams)",
			EnvVar: "WATCHTOWER_NOTIFICATIONS",
		},
		cli.StringFlag{
			Name:   "notifications-level",
			Usage:  "The log level used for sending notifications. Possible values: \"panic\", \"fatal\", \"error\", \"warn\", \"info\" or \"debug\"",
			EnvVar: "WATCHTOWER_NOTIFICATIONS_LEVEL",
			Value:  "info",
		},
		cli.StringFlag{
			Name:   "notification-email-from",
			Usage:  "Address to send notification e-mails from",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_FROM",
		},
		cli.StringFlag{
			Name:   "notification-email-to",
			Usage:  "Address to send notification e-mails to",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_TO",
		},
		cli.StringFlag{
			Name:   "notification-email-server",
			Usage:  "SMTP server to send notification e-mails through",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER",
		},
		cli.IntFlag{
			Name:   "notification-email-server-port",
			Usage:  "SMTP server port to send notification e-mails through",
			Value:  25,
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT",
		},
		cli.BoolFlag{
			Name: "notification-email-server-tls-skip-verify",
			Usage: "Controls whether watchtower verifies the SMTP server's certificate chain and host name. " +
				"If set, TLS accepts any certificate " +
				"presented by the server and any host name in that certificate. " +
				"In this mode, TLS is susceptible to man-in-the-middle attacks. " +
				"This should be used only for testing.",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY",
		},
		cli.StringFlag{
			Name:   "notification-email-server-user",
			Usage:  "SMTP server user for sending notifications",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER",
		},
		cli.StringFlag{
			Name:   "notification-email-server-password",
			Usage:  "SMTP server password for sending notifications",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD",
		},
		cli.StringFlag{
			Name:   "notification-slack-hook-url",
			Usage:  "The Slack Hook URL to send notifications to",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL",
		},
		cli.StringFlag{
			Name:   "notification-slack-identifier",
			Usage:  "A string which will be used to identify the messages coming from this watchtower instance. Default if omitted is \"watchtower\"",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER",
			Value:  "watchtower",
		},
		cli.StringFlag{
			Name:   "notification-slack-channel",
			Usage:  "A string which overrides the webhook's default channel. Example: #my-custom-channel",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_CHANNEL",
		},
		cli.StringFlag{
			Name:   "notification-slack-icon-emoji",
			Usage:  "An emoji code string to use in place of the default icon",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI",
		},
		cli.StringFlag{
			Name:   "notification-slack-icon-url",
			Usage:  "An icon image URL string to use in place of the default icon",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_ICON_URL",
		},
		cli.StringFlag{
			Name:   "notification-msteams-hook",
			Usage:  "The MSTeams WebHook URL to send notifications to",
			EnvVar: "WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL",
		},
		cli.BoolFlag{
			Name:   "notification-msteams-data",
			Usage:  "The MSTeams notifier will try to extract log entry fields as MSTeams message facts",
			EnvVar: "WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA",
		},
		cli.BoolFlag{
			Name:   "monitor-only",
			Usage:  "Will only monitor for new images, not update the containers",
			EnvVar: "WATCHTOWER_MONITOR_ONLY",
		},
		cli.BoolFlag{
			Name:  "run-once",
			Usage: "Run once now and exit",
		},
	}
}
