package flags

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// DockerAPIMinVersion is the minimum version of the docker api required to
// use watchtower
const DockerAPIMinVersion string = "1.25"

// RegisterDockerFlags that are used directly by the docker api client
func RegisterDockerFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	flags.StringP("host", "H", viper.GetString("DOCKER_HOST"), "daemon socket to connect to")
	flags.BoolP("tlsverify", "v", viper.GetBool("DOCKER_TLS_VERIFY"), "use TLS and verify the remote")
	flags.StringP("api-version", "a", viper.GetString("DOCKER_API_VERSION"), "api version to use by docker client")
}

// RegisterSystemFlags that are used by watchtower to modify the program flow
func RegisterSystemFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	flags.IntP(
		"interval",
		"i",
		viper.GetInt("WATCHTOWER_POLL_INTERVAL"),
		"Poll interval (in seconds)")

	flags.StringP(
		"schedule",
		"s",
		viper.GetString("WATCHTOWER_SCHEDULE"),
		"The cron expression which defines when to update")

	flags.DurationP(
		"stop-timeout",
		"t",
		viper.GetDuration("WATCHTOWER_TIMEOUT"),
		"Timeout before a container is forcefully stopped")

	flags.BoolP(
		"no-pull",
		"",
		viper.GetBool("WATCHTOWER_NO_PULL"),
		"Do not pull any new images")

	flags.BoolP(
		"no-restart",
		"",
		viper.GetBool("WATCHTOWER_NO_RESTART"),
		"Do not restart any containers")

	flags.BoolP(
		"no-startup-message",
		"",
		viper.GetBool("WATCHTOWER_NO_STARTUP_MESSAGE"),
		"Prevents watchtower from sending a startup message")

	flags.BoolP(
		"cleanup",
		"c",
		viper.GetBool("WATCHTOWER_CLEANUP"),
		"Remove previously used images after updating")

	flags.BoolP(
		"remove-volumes",
		"",
		viper.GetBool("WATCHTOWER_REMOVE_VOLUMES"),
		"Remove attached volumes before updating")

	flags.BoolP(
		"label-enable",
		"e",
		viper.GetBool("WATCHTOWER_LABEL_ENABLE"),
		"Watch containers where the com.centurylinklabs.watchtower.enable label is true")

	flags.BoolP(
		"debug",
		"d",
		viper.GetBool("WATCHTOWER_DEBUG"),
		"Enable debug mode with verbose logging")

	flags.BoolP(
		"trace",
		"",
		viper.GetBool("WATCHTOWER_TRACE"),
		"Enable trace mode with very verbose logging - caution, exposes credentials")

	flags.BoolP(
		"monitor-only",
		"m",
		viper.GetBool("WATCHTOWER_MONITOR_ONLY"),
		"Will only monitor for new images, not update the containers")

	flags.BoolP(
		"run-once",
		"R",
		viper.GetBool("WATCHTOWER_RUN_ONCE"),
		"Run once now and exit")

	flags.BoolP(
		"include-restarting",
		"",
		viper.GetBool("WATCHTOWER_INCLUDE_RESTARTING"),
		"Will also include restarting containers")

	flags.BoolP(
		"include-stopped",
		"S",
		viper.GetBool("WATCHTOWER_INCLUDE_STOPPED"),
		"Will also include created and exited containers")

	flags.BoolP(
		"revive-stopped",
		"",
		viper.GetBool("WATCHTOWER_REVIVE_STOPPED"),
		"Will also start stopped containers that were updated, if include-stopped is active")

	flags.BoolP(
		"enable-lifecycle-hooks",
		"",
		viper.GetBool("WATCHTOWER_LIFECYCLE_HOOKS"),
		"Enable the execution of commands triggered by pre- and post-update lifecycle hooks")

	flags.BoolP(
		"rolling-restart",
		"",
		viper.GetBool("WATCHTOWER_ROLLING_RESTART"),
		"Restart containers one at a time")

	flags.BoolP(
		"http-api-update",
		"",
		viper.GetBool("WATCHTOWER_HTTP_API_UPDATE"),
		"Runs Watchtower in HTTP API mode, so that image updates must to be triggered by a request")
	flags.BoolP(
		"http-api-metrics",
		"",
		viper.GetBool("WATCHTOWER_HTTP_API_METRICS"),
		"Runs Watchtower with the Prometheus metrics API enabled")

	flags.StringP(
		"http-api-token",
		"",
		viper.GetString("WATCHTOWER_HTTP_API_TOKEN"),
		"Sets an authentication token to HTTP API requests.")

	flags.BoolP(
		"http-api-periodic-polls",
		"",
		viper.GetBool("WATCHTOWER_HTTP_API_PERIODIC_POLLS"),
		"Also run periodic updates (specified with --interval and --schedule) if HTTP API is enabled")

	// https://no-color.org/
	flags.BoolP(
		"no-color",
		"",
		viper.IsSet("NO_COLOR"),
		"Disable ANSI color escape codes in log output")

	flags.StringP(
		"scope",
		"",
		viper.GetString("WATCHTOWER_SCOPE"),
		"Defines a monitoring scope for the Watchtower instance.")

	flags.StringP(
		"porcelain",
		"P",
		viper.GetString("WATCHTOWER_PORCELAIN"),
		`Write session results to stdout using a stable versioned format. Supported values: "v1"`)
		
}

// RegisterNotificationFlags that are used by watchtower to send notifications
func RegisterNotificationFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()

	flags.StringSliceP(
		"notifications",
		"n",
		viper.GetStringSlice("WATCHTOWER_NOTIFICATIONS"),
		" Notification types to send (valid: email, slack, msteams, gotify, shoutrrr)")

	flags.String(
		"notifications-level",
		viper.GetString("WATCHTOWER_NOTIFICATIONS_LEVEL"),
		"The log level used for sending notifications. Possible values: panic, fatal, error, warn, info or debug")

	flags.IntP(
		"notifications-delay",
		"",
		viper.GetInt("WATCHTOWER_NOTIFICATIONS_DELAY"),
		"Delay before sending notifications, expressed in seconds")

	flags.StringP(
		"notifications-hostname",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATIONS_HOSTNAME"),
		"Custom hostname for notification titles")

	flags.StringP(
		"notification-email-from",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_FROM"),
		"Address to send notification emails from")

	flags.StringP(
		"notification-email-to",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_TO"),
		"Address to send notification emails to")

	flags.IntP(
		"notification-email-delay",
		"",
		viper.GetInt("WATCHTOWER_NOTIFICATION_EMAIL_DELAY"),
		"Delay before sending notifications, expressed in seconds")

	flags.StringP(
		"notification-email-server",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER"),
		"SMTP server to send notification emails through")

	flags.IntP(
		"notification-email-server-port",
		"",
		viper.GetInt("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT"),
		"SMTP server port to send notification emails through")

	flags.BoolP(
		"notification-email-server-tls-skip-verify",
		"",
		viper.GetBool("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY"),
		`Controls whether watchtower verifies the SMTP server's certificate chain and host name.
Should only be used for testing.`)

	flags.StringP(
		"notification-email-server-user",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER"),
		"SMTP server user for sending notifications")

	flags.StringP(
		"notification-email-server-password",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD"),
		"SMTP server password for sending notifications")

	flags.StringP(
		"notification-email-subjecttag",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG"),
		"Subject prefix tag for notifications via mail")

	flags.StringP(
		"notification-slack-hook-url",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL"),
		"The Slack Hook URL to send notifications to")

	flags.StringP(
		"notification-slack-identifier",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER"),
		"A string which will be used to identify the messages coming from this watchtower instance")

	flags.StringP(
		"notification-slack-channel",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_CHANNEL"),
		"A string which overrides the webhook's default channel. Example: #my-custom-channel")

	flags.StringP(
		"notification-slack-icon-emoji",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI"),
		"An emoji code string to use in place of the default icon")

	flags.StringP(
		"notification-slack-icon-url",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_SLACK_ICON_URL"),
		"An icon image URL string to use in place of the default icon")

	flags.StringP(
		"notification-msteams-hook",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL"),
		"The MSTeams WebHook URL to send notifications to")

	flags.BoolP(
		"notification-msteams-data",
		"",
		viper.GetBool("WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA"),
		"The MSTeams notifier will try to extract log entry fields as MSTeams message facts")

	flags.StringP(
		"notification-gotify-url",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_GOTIFY_URL"),
		"The Gotify URL to send notifications to")

	flags.StringP(
		"notification-gotify-token",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN"),
		"The Gotify Application required to query the Gotify API")

	flags.BoolP(
		"notification-gotify-tls-skip-verify",
		"",
		viper.GetBool("WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY"),
		`Controls whether watchtower verifies the Gotify server's certificate chain and host name.
Should only be used for testing.`)

	flags.String(
		"notification-template",
		viper.GetString("WATCHTOWER_NOTIFICATION_TEMPLATE"),
		"The shoutrrr text/template for the messages")

	flags.StringArray(
		"notification-url",
		viper.GetStringSlice("WATCHTOWER_NOTIFICATION_URL"),
		"The shoutrrr URL to send notifications to")

	flags.Bool("notification-report",
		viper.GetBool("WATCHTOWER_NOTIFICATION_REPORT"),
		"Use the session report as the notification template data")

	flags.StringP(
		"notification-title-tag",
		"",
		viper.GetString("WATCHTOWER_NOTIFICATION_TITLE_TAG"),
		"Title prefix tag for notifications")

	flags.Bool("notification-skip-title",
		viper.GetBool("WATCHTOWER_NOTIFICATION_SKIP_TITLE"),
		"Do not pass the title param to notifications")

	flags.String(
		"warn-on-head-failure",
		viper.GetString("WATCHTOWER_WARN_ON_HEAD_FAILURE"),
		"When to warn about HEAD pull requests failing. Possible values: always, auto or never")

	flags.Bool(
		"notification-log-stdout",
		viper.GetBool("WATCHTOWER_NOTIFICATION_LOG_STDOUT"),
		"Write notification logs to stdout instead of logging (to stderr)")
}

// SetDefaults provides default values for environment variables
func SetDefaults() {
	day := (time.Hour * 24).Seconds()
	viper.AutomaticEnv()
	viper.SetDefault("DOCKER_HOST", "unix:///var/run/docker.sock")
	viper.SetDefault("DOCKER_API_VERSION", DockerAPIMinVersion)
	viper.SetDefault("WATCHTOWER_POLL_INTERVAL", day)
	viper.SetDefault("WATCHTOWER_TIMEOUT", time.Second*10)
	viper.SetDefault("WATCHTOWER_NOTIFICATIONS", []string{})
	viper.SetDefault("WATCHTOWER_NOTIFICATIONS_LEVEL", "info")
	viper.SetDefault("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT", 25)
	viper.SetDefault("WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG", "")
	viper.SetDefault("WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER", "watchtower")
}

// EnvConfig translates the command-line options into environment variables
// that will initialize the api client
func EnvConfig(cmd *cobra.Command) error {
	var err error
	var host string
	var tls bool
	var version string

	flags := cmd.PersistentFlags()

	if host, err = flags.GetString("host"); err != nil {
		return err
	}
	if tls, err = flags.GetBool("tlsverify"); err != nil {
		return err
	}
	if version, err = flags.GetString("api-version"); err != nil {
		return err
	}
	if err = setEnvOptStr("DOCKER_HOST", host); err != nil {
		return err
	}
	if err = setEnvOptBool("DOCKER_TLS_VERIFY", tls); err != nil {
		return err
	}
	if err = setEnvOptStr("DOCKER_API_VERSION", version); err != nil {
		return err
	}
	return nil
}

// ReadFlags reads common flags used in the main program flow of watchtower
func ReadFlags(cmd *cobra.Command) (bool, bool, bool, time.Duration) {
	flags := cmd.PersistentFlags()

	var err error
	var cleanup bool
	var noRestart bool
	var monitorOnly bool
	var timeout time.Duration

	if cleanup, err = flags.GetBool("cleanup"); err != nil {
		log.Fatal(err)
	}
	if noRestart, err = flags.GetBool("no-restart"); err != nil {
		log.Fatal(err)
	}
	if monitorOnly, err = flags.GetBool("monitor-only"); err != nil {
		log.Fatal(err)
	}
	if timeout, err = flags.GetDuration("stop-timeout"); err != nil {
		log.Fatal(err)
	}

	return cleanup, noRestart, monitorOnly, timeout
}

func setEnvOptStr(env string, opt string) error {
	if opt == "" || opt == os.Getenv(env) {
		return nil
	}
	err := os.Setenv(env, opt)
	if err != nil {
		return err
	}
	return nil
}

func setEnvOptBool(env string, opt bool) error {
	if opt {
		return setEnvOptStr(env, "1")
	}
	return nil
}

// GetSecretsFromFiles checks if passwords/tokens/webhooks have been passed as a file instead of plaintext.
// If so, the value of the flag will be replaced with the contents of the file.
func GetSecretsFromFiles(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()

	secrets := []string{
		"notification-email-server-password",
		"notification-slack-hook-url",
		"notification-msteams-hook",
		"notification-gotify-token",
		"notification-url",
	}
	for _, secret := range secrets {
		getSecretFromFile(flags, secret)
	}
}

// getSecretFromFile will check if the flag contains a reference to a file; if it does, replaces the value of the flag with the contents of the file.
func getSecretFromFile(flags *pflag.FlagSet, secret string) {
	flag := flags.Lookup(secret)
	if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
		oldValues := sliceValue.GetSlice()
		values := make([]string, 0, len(oldValues))
		for _, value := range oldValues {
			if value != "" && isFile(value) {
				file, err := os.Open(value)
				if err != nil {
					log.Fatal(err)
				}
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}
					values = append(values, line)
				}
			} else {
				values = append(values, value)
			}
		}
		sliceValue.Replace(values)
		return
	}

	value := flag.Value.String()
	if value != "" && isFile(value) {
		file, err := ioutil.ReadFile(value)
		if err != nil {
			log.Fatal(err)
		}
		err = flags.Set(secret, strings.TrimSpace(string(file)))
		if err != nil {
			log.Error(err)
		}
	}
}

func isFile(s string) bool {
	firstColon := strings.IndexRune(s, ':')
	if firstColon != 1 && firstColon != -1 {
		// If the string contains a ':', but it's not the second character, it's probably not a file
		// and will cause a fatal error on windows if stat'ed
		// This still allows for paths that start with 'c:\' etc.
		return false
	}
	_, err := os.Stat(s)
	return !errors.Is(err, os.ErrNotExist)
}

// ProcessFlagAliases updates the value of flags that are being set by helper flags
func ProcessFlagAliases(flags *pflag.FlagSet) {

	porcelain, err := flags.GetString(`porcelain`)
	if err != nil {
		log.Fatalf(`Failed to get flag: %v`, err)
	}
	if porcelain != "" {
	    if porcelain != "v1" {
	        log.Fatalf(`Unknown porcelain version %q. Supported values: "v1"`, porcelain)
	    }
		if err = appendFlagValue(flags, `notification-url`, `logger://`); err != nil {
			log.Errorf(`Failed to set flag: %v`, err)
		}
		setFlagIfDefault(flags, `notification-log-stdout`, `true`)
		setFlagIfDefault(flags, `notification-report`, `true`)
		tpl := fmt.Sprintf(`porcelain.%s.summary-no-log`, porcelain)
		setFlagIfDefault(flags, `notification-template`, tpl)
	}

	if flags.Changed(`interval`) && flags.Changed(`schedule`) {
		log.Fatal(`Only schedule or interval can be defined, not both.`)
	}

	// update schedule flag to match interval if it's set, or to the default if none of them are
	if flags.Changed(`interval`) || !flags.Changed(`schedule`) {
		interval, _ := flags.GetInt(`interval`)
		flags.Set(`schedule`, fmt.Sprintf(`@every %ds`, interval))
	}
}

func appendFlagValue(flags *pflag.FlagSet, name string, values ...string) error {
	flag := flags.Lookup(name)
	if flag == nil {
		return fmt.Errorf(`invalid flag name %q`, name)
	}

	if flagValues, ok := flag.Value.(pflag.SliceValue); ok {
		for _, value := range values {
			flagValues.Append(value)
		}
	} else {
		return fmt.Errorf(`the value for flag %q is not a slice value`, name)
	}

	return nil
}

func setFlagIfDefault(flags *pflag.FlagSet, name string, value string) {
	if flags.Changed(name) {
		return
	}
	if err := flags.Set(name, value); err != nil {
		log.Errorf(`Failed to set flag: %v`, err)
	}
}
