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

	HTTPAPIUpdate        boolConfKey   = "http-api-update"
	HTTPAPIMetrics       boolConfKey   = "http-api-metrics"
	HTTPAPIPeriodicPolls boolConfKey   = "http-api-periodic-polls"
	HTTPAPIToken         stringConfKey = "HTTPAPIToken"

	NoColor boolConfKey = "no-color"

	NotificationGotifyTLSSkipVerify boolConfKey = "notification-gotify-tls-skip-verify"

	Schedule stringConfKey = "schedule"
	Interval intConfKey    = "interval"

	StopTimeout durationConfKey = "stop-timeout"

	Scope stringConfKey = "Scope"

	/* Docker v*/
	DockerHost       stringConfKey = "host"
	DockerAPIVersion stringConfKey = "api-version"
	DockerTlSVerify  boolConfKey   = "tlsverify"

	Notifications         sliceConfKey  = "notifications"
	NotificationsLevel    stringConfKey = "notifications-level"
	NotificationsDelay    intConfKey    = "notifications-delay"
	NotificationsHostname stringConfKey = "notifications-hostname"
	NotificationTemplate  stringConfKey = "notification-template"
	NotificationReport    boolConfKey   = "notification-report"
	NotificationURL       sliceConfKey  = "notification-url"

	NotificationEmailFrom                stringConfKey = "notification-email-from"
	NotificationEmailTo                  stringConfKey = "notification-email-to"
	NotificationEmailServer              stringConfKey = "notification-email-server"
	NotificationEmailServerUser          stringConfKey = "notification-email-server-user"
	NotificationEmailServerPassword      stringConfKey = "notification-email-server-password"
	NotificationEmailSubjectTag          stringConfKey = "notification-email-subjecttag"
	NotificationEmailDelay               intConfKey    = "notification-email-delay"
	NotificationEmailServerPort          intConfKey    = "notification-email-server-port"
	NotificationEmailServerTLSSkipVerify boolConfKey   = "notification-email-server-tls-skip-verify"

	NotificationSlackHookURL    stringConfKey = "notification-slack-hook-url"
	NotificationSlackIdentifier stringConfKey = "notification-slack-identifier"
	NotificationSlackChannel    stringConfKey = "notification-slack-channel"
	NotificationSlackIconEmoji  stringConfKey = "notification-slack-icon-emoji"
	NotificationSlackIconURL    stringConfKey = "notification-slack-icon-url"

	NotificationMSTeamsHook stringConfKey = "notification-msteams-hook"
	NotificationMSTeamsData boolConfKey   = "notification-msteams-data"

	NotificationGotifyURL   stringConfKey = "notification-gotify-url"
	NotificationGotifyToken stringConfKey = "notification-gotify-token"
)
