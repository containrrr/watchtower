package flags

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DockerAPIMinVersion is the minimum version of the docker api required to
// use watchtower
const DockerAPIMinVersion string = "1.25"

// RegisterDockerFlags that are used directly by the docker api client
func RegisterDockerFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	flags.StringP("host", "H", "unix:///var/run/docker.sock", "daemon socket to connect to")
	flags.BoolP("tlsverify", "v", false, "use TLS and verify the remote")
	flags.StringP("api-version", "a", DockerAPIMinVersion, "api version to use by docker client")
}

// RegisterSystemFlags that are used by watchtower to modify the program flow
func RegisterSystemFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	flags.IntP(
		"interval",
		"i",
		300,
		"poll interval (in seconds)")

	flags.StringP(
		"schedule",
		"s",
		"",
		"the cron expression which defines when to update")

	flags.DurationP(
		"stop-timeout",
		"t",
		time.Second*10,
		"timeout before a container is forcefully stopped")

	flags.BoolP(
		"no-pull",
		"",
		false,
		"do not pull any new images")

	flags.BoolP(
		"no-restart",
		"",
		false,
		"do not restart any containers")

	flags.BoolP(
		"no-startup-message",
		"",
		false,
		"Prevents watchtower from sending a startup message")

	flags.BoolP(
		"cleanup",
		"c",
		false,
		"remove previously used images after updating")

	flags.BoolP(
		"remove-volumes",
		"",
		false,
		"remove attached volumes before updating")

	flags.BoolP(
		"label-enable",
		"e",
		false,
		"watch containers where the com.centurylinklabs.watchtower.enable label is true")

	flags.BoolP(
		"debug",
		"d",
		false,
		"enable debug mode with verbose logging")

	flags.BoolP(
		"trace",
		"",
		false,
		"enable trace mode with very verbose logging - caution, exposes credentials")

	flags.BoolP(
		"monitor-only",
		"m",
		false,
		"Will only monitor for new images, not update the containers")

	flags.BoolP(
		"run-once",
		"R",
		false,
		"Run once now and exit")

	flags.BoolP(
		"include-stopped",
		"S",
		false,
		"Will also include created and exited containers")

	flags.BoolP(
		"revive-stopped",
		"",
		false,
		"Will also start stopped containers that were updated, if include-stopped is active")

	flags.BoolP(
		"enable-lifecycle-hooks",
		"",
		false,
		"Enable the execution of commands triggered by pre- and post-update lifecycle hooks")

	flags.BoolP(
		"rolling-restart",
		"",
		false,
		"Restart containers one at a time")

	flags.BoolP(
		"http-api",
		"",
		false,
		"Runs Watchtower in HTTP API mode, so that image updates must to be triggered by a request")

	flags.StringP(
		"http-api-token",
		"",
		"",
		"Sets an authentication token to HTTP API requests.")
	// https://no-color.org/
	flags.BoolP(
		"no-color",
		"",
		false,
		"Disable ANSI color escape codes in log output")
	flags.StringP(
		"scope",
		"",
		"",
		"Defines a monitoring scope for the Watchtower instance.")
}

// RegisterNotificationFlags that are used by watchtower to send notifications
func RegisterNotificationFlags(rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()

	flags.StringSliceP(
		"notifications",
		"n",
		[]string{},
		" notification types to send (valid: email, slack, msteams, gotify, shoutrrr)")

	flags.StringP(
		"notifications-level",
		"",
		"info",
		"The log level used for sending notifications. Possible values: panic, fatal, error, warn, info or debug")

	flags.StringP(
		"notification-email-from",
		"",
		"",
		"Address to send notification emails from")

	flags.StringP(
		"notification-email-to",
		"",
		"",
		"Address to send notification emails to")

	flags.IntP(
		"notification-email-delay",
		"",
		0,
		"Delay before sending notifications, expressed in seconds")

	flags.StringP(
		"notification-email-server",
		"",
		"",
		"SMTP server to send notification emails through")

	flags.IntP(
		"notification-email-server-port",
		"",
		25,
		"SMTP server port to send notification emails through")

	flags.BoolP(
		"notification-email-server-tls-skip-verify",
		"",
		false,
		`Controls whether watchtower verifies the SMTP server's certificate chain and host name.
Should only be used for testing.`)

	flags.StringP(
		"notification-email-server-user",
		"",
		"",
		"SMTP server user for sending notifications")

	flags.StringP(
		"notification-email-server-password",
		"",
		"",
		"SMTP server password for sending notifications")

	flags.StringP(
		"notification-email-subjecttag",
		"",
		"",
		"Subject prefix tag for notifications via mail")

	flags.StringP(
		"notification-slack-hook-url",
		"",
		"",
		"The Slack Hook URL to send notifications to")

	flags.StringP(
		"notification-slack-identifier",
		"",
		"watchtower",
		"A string which will be used to identify the messages coming from this watchtower instance")

	flags.StringP(
		"notification-slack-channel",
		"",
		"",
		"A string which overrides the webhook's default channel. Example: #my-custom-channel")

	flags.StringP(
		"notification-slack-icon-emoji",
		"",
		"",
		"An emoji code string to use in place of the default icon")

	flags.StringP(
		"notification-slack-icon-url",
		"",
		"",
		"An icon image URL string to use in place of the default icon")

	flags.StringP(
		"notification-msteams-hook",
		"",
		"",
		"The MSTeams WebHook URL to send notifications to")

	flags.BoolP(
		"notification-msteams-data",
		"",
		false,
		"The MSTeams notifier will try to extract log entry fields as MSTeams message facts")

	flags.StringP(
		"notification-gotify-url",
		"",
		"",
		"The Gotify URL to send notifications to")

	flags.StringP(
		"notification-gotify-token",
		"",
		"",
		"The Gotify Application required to query the Gotify API")

	flags.BoolP(
		"notification-gotify-tls-skip-verify",
		"",
		false,
		`Controls whether watchtower verifies the Gotify server's certificate chain and host name.
Should only be used for testing.`)

	flags.StringP(
		"notification-template",
		"",
		"",
		"The shoutrrr text/template for the messages")

	flags.StringArrayP(
		"notification-url",
		"",
		[]string{},
		"The shoutrrr URL to send notifications to")
}

// SetEnvBindings binds environment variables to their corresponding config keys
func SetEnvBindings() {
	if err := viper.BindEnv("host", "DOCKER_HOST"); err != nil {
		log.Fatalf("failed to bind env DOCKER_HOST: %v", err)
	}
	if err := viper.BindEnv("tlsverify", "DOCKER_TLS_VERIFY"); err != nil {
		log.Fatalf("failed to bind env DOCKER_TLS_VERIFY: %v", err)
	}
	if err := viper.BindEnv("api-version", "DOCKER_API_VERSION"); err != nil {
		log.Fatalf("failed to bind env DOCKER_API_VERSION: %v", err)
	}
	viper.SetEnvPrefix("WATCHTOWER")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

// BindViperFlags binds the cmd PFlags to the viper configuration
func BindViperFlags(cmd *cobra.Command) {
	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind flags: %v", err)
	}
}

// EnvConfig translates the command-line options into environment variables
// that will initialize the api client
func EnvConfig() error {
	var err error
	var tls bool
	var version string

	host := viper.GetString("host")
	tls = viper.GetBool("tlsverify")
	version = viper.GetString("api-version")
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
func GetSecretsFromFiles() {
	secrets := []string{
		"notification-email-server-password",
		"notification-slack-hook-url",
		"notification-msteams-hook",
		"notification-gotify-token",
	}
	for _, secret := range secrets {
		getSecretFromFile(secret)
	}
}

// getSecretFromFile will check if the flag contains a reference to a file; if it does, replaces the value of the flag with the contents of the file.
func getSecretFromFile(secret string) {
	value := viper.GetString(secret)
	if value != "" && isFile(value) {
		file, err := ioutil.ReadFile(value)
		if err != nil {
			log.Fatal(err)
		}
		viper.Set(secret, strings.TrimSpace(string(file)))
	}
}

func isFile(s string) bool {
	_, err := os.Stat(s)
	if os.IsNotExist(err) {
		return false
	}
	return true
}
