package config

import (
	"time"

	"github.com/spf13/cobra"
)

// DockerAPIMinVersion is the minimum version of the docker api required to
// use watchtower
const DockerAPIMinVersion string = "1.25"

// DefaultInterval is the default time between the start of update checks
const DefaultInterval = int(time.Hour * 24 / time.Second)

// RegisterDockerOptions that are used directly by the docker api client
func RegisterDockerOptions(rootCmd *cobra.Command) {
	ob := NewOptBuilder(rootCmd.PersistentFlags())

	ob.StringP(DockerHost, "H", "unix:///var/run/docker.sock",
		"daemon socket to connect to",
		"DOCKER_HOST")

	ob.BoolP(DockerTlSVerify, "v", false,
		"use TLS and verify the remote",
		"DOCKER_TLS_VERIFY")

	ob.StringP(DockerAPIVersion, "a", DockerAPIMinVersion,
		"api version to use by docker client",
		"DOCKER_API_VERSION")
}

// RegisterSystemOptions that are used by watchtower to modify the program flow
func RegisterSystemOptions(rootCmd *cobra.Command) {
	ob := NewOptBuilder(rootCmd.PersistentFlags())

	ob.IntP(Interval, "i", DefaultInterval,
		"poll interval (in seconds)",
		"WATCHTOWER_POLL_INTERVAL")

	ob.StringP(Schedule, "s", "",
		"The cron expression which defines when to update",
		"WATCHTOWER_SCHEDULE")

	ob.DurationP(StopTimeout, "t", time.Second*10,
		"Timeout before a container is forcefully stopped",
		"WATCHTOWER_TIMEOUT")

	ob.Bool(NoPull, false,
		"Do not pull any new images",
		"WATCHTOWER_NO_PULL")

	ob.Bool(NoRestart, false,
		"Do not restart any containers",
		"WATCHTOWER_NO_RESTART")

	ob.Bool(NoStartupMessage, false,
		"Prevents watchtower from sending a startup message",
		"WATCHTOWER_NO_STARTUP_MESSAGE")

	ob.BoolP(Cleanup, "c", false,
		"Remove previously used images after updating",
		"WATCHTOWER_CLEANUP")

	ob.BoolP(RemoveVolumes,
		"",
		false,
		"Remove attached volumes before updating",
		"WATCHTOWER_REMOVE_VOLUMES")

	ob.BoolP(LabelEnable,
		"e",
		false,
		"Watch containers where the com.centurylinklabs.watchtower.enable label is true",
		"WATCHTOWER_LABEL_ENABLE")

	ob.BoolP(Debug,
		"d",
		false,
		"Enable debug mode with verbose logging",
		"WATCHTOWER_DEBUG")

	ob.Bool(Trace,
		false,
		"Enable trace mode with very verbose logging - caution, exposes credentials",
		"WATCHTOWER_TRACE")

	ob.BoolP(MonitorOnly, "m", false,
		"Will only monitor for new images, not update the containers",
		"WATCHTOWER_MONITOR_ONLY")

	ob.BoolP(RunOnce, "R", false,
		"Run once now and exit",
		"WATCHTOWER_RUN_ONCE")

	ob.BoolP(IncludeRestarting, "", false,
		"Will also include restarting containers",
		"WATCHTOWER_INCLUDE_RESTARTING")

	ob.BoolP(IncludeStopped, "S", false,
		"Will also include created and exited containers",
		"WATCHTOWER_INCLUDE_STOPPED")

	ob.Bool(ReviveStopped, false,
		"Will also start stopped containers that were updated, if include-stopped is active",
		"WATCHTOWER_REVIVE_STOPPED")

	ob.Bool(EnableLifecycleHooks, false,
		"Enable the execution of commands triggered by pre- and post-update lifecycle hooks",
		"WATCHTOWER_LIFECYCLE_HOOKS")

	ob.Bool(RollingRestart, false,
		"Restart containers one at a time",
		"WATCHTOWER_ROLLING_RESTART")

	ob.Bool(HTTPAPIUpdate, false,
		"Runs Watchtower in HTTP API mode, so that image updates must to be triggered by a request",
		"WATCHTOWER_HTTP_API_UPDATE")

	ob.Bool(HTTPAPIMetrics, false,
		"Runs Watchtower with the Prometheus metrics API enabled",
		"WATCHTOWER_HTTP_API_METRICS")

	ob.String(HTTPAPIToken, "",
		"Sets an authentication token to HTTP API requests.",
		"WATCHTOWER_HTTP_API_TOKEN")

	ob.Bool(HTTPAPIPeriodicPolls, false,
		"Also run periodic updates (specified with --interval and --schedule) if HTTP API is enabled",
		"WATCHTOWER_HTTP_API_PERIODIC_POLLS")

	// https://no-color.org/
	ob.Bool(NoColor, false,
		"Disable ANSI color escape codes in log output",
		"NO_COLOR")

	ob.String(Scope, "",
		"Defines a monitoring scope for the Watchtower instance.",
		"WATCHTOWER_SCOPE")
}

// RegisterNotificationOptions that are used by watchtower to send notifications
func RegisterNotificationOptions(cmd *cobra.Command) {
	ob := NewOptBuilder(cmd.PersistentFlags())

	ob.StringSliceP(Notifications, "n", []string{},
		" Notification types to send (valid: email, slack, msteams, gotify, shoutrrr)",
		"WATCHTOWER_NOTIFICATIONS")

	ob.String(NotificationsLevel, "info",
		"The log level used for sending notifications. Possible values: panic, fatal, error, warn, info or debug",
		"WATCHTOWER_NOTIFICATIONS_LEVEL")

	ob.Int(NotificationsDelay, 0,
		"Delay before sending notifications, expressed in seconds",
		"WATCHTOWER_NOTIFICATIONS_DELAY")

	ob.String(NotificationsHostname, "",
		"Custom hostname for notification titles",
		"WATCHTOWER_NOTIFICATIONS_HOSTNAME")

	ob.String(NotificationTemplate, "",
		"The shoutrrr text/template for the messages",
		"WATCHTOWER_NOTIFICATION_TEMPLATE")

	ob.StringArray(NotificationURL, []string{},
		"The shoutrrr URL to send notifications to",
		"WATCHTOWER_NOTIFICATION_URL")

	ob.Bool(NotificationReport, false,
		"Use the session report as the notification template data",
		"WATCHTOWER_NOTIFICATION_REPORT")

	ob.String(WarnOnHeadFailure, "auto",
		"When to warn about HEAD pull requests failing. Possible values: always, auto or never",
		"WATCHTOWER_WARN_ON_HEAD_FAILURE")

	RegisterLegacyNotificationFlags(cmd)
}
