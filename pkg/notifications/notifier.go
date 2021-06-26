package notifications

import (
	"os"
	"time"

	ty "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// NewNotifier creates and returns a new Notifier, using global configuration.
func NewNotifier() ty.Notifier {
	level := viper.GetString("notifications-level")
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Fatalf("Notifications invalid log level: %s", err.Error())
	}

	acceptedLogLevels := slackrus.LevelThreshold(logLevel)
	// slackrus does not allow log level TRACE, even though it's an accepted log level for logrus
	if len(acceptedLogLevels) == 0 {
		log.Fatalf("Unsupported notification log level provided: %s", level)
	}

	reportTemplate := viper.GetBool("notification-report")
	tplString := viper.GetString("notification-template")
	urls := viper.GetStringSlice("notification-url")

	hostname := GetHostname()
	urls, delay := AppendLegacyUrls(urls, GetTitle(hostname))

	return newShoutrrrNotifier(tplString, acceptedLogLevels, !reportTemplate, hostname, delay, urls...)
}

// AppendLegacyUrls creates shoutrrr equivalent URLs from legacy notification flags
func AppendLegacyUrls(urls []string, title string) ([]string, time.Duration) {

	// Parse types and create notifiers.
	types := viper.GetStringSlice("notifications")

	legacyDelay := time.Duration(0)

	for _, t := range types {

		var legacyNotifier ty.ConvertibleNotifier
		var err error

		switch t {
		case emailType:
			legacyNotifier = newEmailNotifier()
		case slackType:
			legacyNotifier = newSlackNotifier()
		case msTeamsType:
			legacyNotifier = newMsTeamsNotifier()
		case gotifyType:
			legacyNotifier = newGotifyNotifier()
		case shoutrrrType:
			continue
		default:
			log.Fatalf("Unknown notification type %q", t)
			// Not really needed, used for nil checking static analysis
			continue
		}

		shoutrrrURL, err := legacyNotifier.GetURL(title)
		if err != nil {
			log.Fatal("failed to create notification config: ", err)
		}
		urls = append(urls, shoutrrrURL)

		if delayNotifier, ok := legacyNotifier.(ty.DelayNotifier); ok {
			legacyDelay = delayNotifier.GetDelay()
		}

		log.WithField("URL", shoutrrrURL).Trace("created Shoutrrr URL from legacy notifier")
	}

	delay := GetDelay(legacyDelay)
	return urls, delay
}

// GetDelay returns the legacy delay if defined, otherwise the delay as set by args is returned
func GetDelay(legacyDelay time.Duration) time.Duration {
	if legacyDelay > 0 {
		return legacyDelay
	}

	delay := viper.GetInt("notifications-delay")
	if delay > 0 {
		return time.Duration(delay) * time.Second
	}
	return time.Duration(0)
}

// GetTitle returns a common notification title with hostname appended
func GetTitle(hostname string) string {
	title := "Watchtower updates"
	if hostname != "" {
		title += " on " + hostname
	}
	return title
}

// GetHostname returns the hostname as set by args or resolved from OS
func GetHostname() string {
	hostname := viper.GetString("notifications-hostname")

	if hostname != "" {
		return hostname
	} else if hostname, err := os.Hostname(); err == nil {
		return hostname
	}

	return ""
}

// ColorHex is the default notification color used for services that support it (formatted as a CSS hex string)
const ColorHex = "#406170"

// ColorInt is the default notification color used for services that support it (as an int value)
const ColorInt = 0x406170
