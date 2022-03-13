package config

type stringConfKey string
type boolConfKey string
type intConfKey string
type durationConfKey string
type sliceConfKey string

//
const (
	NoPull               boolConfKey   = "no-pull"
	NoRestart            boolConfKey   = "no-restart"
	NoStartupMessage     boolConfKey   = "no-startup-message"
	Cleanup              boolConfKey   = "cleanup"
	RemoveVolumes        boolConfKey   = "remove-volumes"
	LabelEnable          boolConfKey   = "label-enable"
	Debug                boolConfKey   = "debug"
	Trace                boolConfKey   = "trace"
	MonitorOnly          boolConfKey   = "monitor-only"
	RunOnce              boolConfKey   = "run-once"
	IncludeRestarting    boolConfKey   = "include-restarting"
	IncludeStopped       boolConfKey   = "include-stopped"
	ReviveStopped        boolConfKey   = "revive-stopped"
	EnableLifecycleHooks boolConfKey   = "enable-lifecycle-hooks"
	RollingRestart       boolConfKey   = "rolling-restart"
	WarnOnHeadFailure    stringConfKey = "warn-on-head-failure"

	HttpApiUpdate        boolConfKey   = "http-api-update"
	HttpApiMetrics       boolConfKey   = "http-api-metrics"
	HttpApiPeriodicPolls boolConfKey   = "http-api-periodic-polls"
	HttpApiToken         stringConfKey = "HttpApiToken"

	NoColor boolConfKey = "no-color"

	NotificationGotifyTlsSkipVerify boolConfKey = "notification-gotify-tls-skip-verify"

	Schedule stringConfKey = "schedule"
	Interval intConfKey    = "interval"

	StopTimeout durationConfKey = "stop-timeout"

	Scope stringConfKey = "Scope"

	/* Docker v*/
	DockerHost       stringConfKey = "host"
	DockerApiVersion stringConfKey = "api-version"
	DockerTlSVerify  boolConfKey   = "tlsverify"

	Notifications         sliceConfKey  = "notifications"
	NotificationsLevel    stringConfKey = "notifications-level"
	NotificationsDelay    intConfKey    = "notifications-delay"
	NotificationsHostname stringConfKey = "notifications-hostname"
	NotificationTemplate  stringConfKey = "notification-template"
	NotificationReport    boolConfKey   = "notification-report"
	NotificationUrl       sliceConfKey  = "notification-url"

	NotificationEmailFrom                stringConfKey = "notification-email-from"
	NotificationEmailTo                  stringConfKey = "notification-email-to"
	NotificationEmailServer              stringConfKey = "notification-email-server"
	NotificationEmailServerUser          stringConfKey = "notification-email-server-user"
	NotificationEmailServerPassword      stringConfKey = "notification-email-server-password"
	NotificationEmailSubjecttag          stringConfKey = "notification-email-subjecttag"
	NotificationEmailDelay               intConfKey    = "notification-email-delay"
	NotificationEmailServerPort          intConfKey    = "notification-email-server-port"
	NotificationEmailServerTlsSkipVerify boolConfKey   = "notification-email-server-tls-skip-verify"

	NotificationSlackHookUrl    stringConfKey = "notification-slack-hook-url"
	NotificationSlackIdentifier stringConfKey = "notification-slack-identifier"
	NotificationSlackChannel    stringConfKey = "notification-slack-channel"
	NotificationSlackIconEmoji  stringConfKey = "notification-slack-icon-emoji"
	NotificationSlackIconUrl    stringConfKey = "notification-slack-icon-url"

	NotificationMsteamsHook stringConfKey = "notification-msteams-hook"
	NotificationMsteamsData boolConfKey   = "notification-msteams-data"

	NotificationGotifyUrl   stringConfKey = "notification-gotify-url"
	NotificationGotifyToken stringConfKey = "notification-gotify-token"
)
