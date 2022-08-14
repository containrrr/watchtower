package notifications

import (
	"os"
	"strings"
	"time"

	ty "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewNotifier creates and returns a new Notifier, using global configuration.
func NewNotifier(c *cobra.Command) ty.Notifier {
	f := c.PersistentFlags()

	level, _ := f.GetString("notifications-level")
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Fatalf("Notifications invalid log level: %s", err.Error())
	}

	levels := slackrus.LevelThreshold(logLevel)
	// slackrus does not allow log level TRACE, even though it's an accepted log level for logrus
	if len(levels) == 0 {
		log.Fatalf("Unsupported notification log level provided: %s", level)
	}

	reportTemplate, _ := f.GetBool("notification-report")
	stdout, _ := f.GetBool("notification-log-stdout")
	tplString, _ := f.GetString("notification-template")
	urls, _ := f.GetStringArray("notification-url")

	data := GetTemplateData(c)
	urls, delay := AppendLegacyUrls(urls, c, data.Title)

	return newShoutrrrNotifier(tplString, levels, !reportTemplate, data, delay, stdout, urls...)
}

// AppendLegacyUrls creates shoutrrr equivalent URLs from legacy notification flags
func AppendLegacyUrls(urls []string, cmd *cobra.Command, title string) ([]string, time.Duration) {

	// Parse types and create notifiers.
	types, err := cmd.Flags().GetStringSlice("notifications")
	if err != nil {
		log.WithError(err).Fatal("could not read notifications argument")
	}

	legacyDelay := time.Duration(0)

	for _, t := range types {

		var legacyNotifier ty.ConvertibleNotifier
		var err error

		switch t {
		case emailType:
			legacyNotifier = newEmailNotifier(cmd, []log.Level{})
		case slackType:
			legacyNotifier = newSlackNotifier(cmd, []log.Level{})
		case msTeamsType:
			legacyNotifier = newMsTeamsNotifier(cmd, []log.Level{})
		case gotifyType:
			legacyNotifier = newGotifyNotifier(cmd, []log.Level{})
		case shoutrrrType:
			continue
		default:
			log.Fatalf("Unknown notification type %q", t)
			// Not really needed, used for nil checking static analysis
			continue
		}

		shoutrrrURL, err := legacyNotifier.GetURL(cmd, title)
		if err != nil {
			log.Fatal("failed to create notification config: ", err)
		}
		urls = append(urls, shoutrrrURL)

		if delayNotifier, ok := legacyNotifier.(ty.DelayNotifier); ok {
			legacyDelay = delayNotifier.GetDelay()
		}

		log.WithField("URL", shoutrrrURL).Trace("created Shoutrrr URL from legacy notifier")
	}

	delay := GetDelay(cmd, legacyDelay)
	return urls, delay
}

// GetDelay returns the legacy delay if defined, otherwise the delay as set by args is returned
func GetDelay(c *cobra.Command, legacyDelay time.Duration) time.Duration {
	if legacyDelay > 0 {
		return legacyDelay
	}

	delay, _ := c.PersistentFlags().GetInt("notifications-delay")
	if delay > 0 {
		return time.Duration(delay) * time.Second
	}
	return time.Duration(0)
}

// GetTitle formats the title based on the passed hostname and tag
func GetTitle(hostname string, tag string) string {
	tb := strings.Builder{}

	if tag != "" {
		tb.WriteRune('[')
		tb.WriteString(tag)
		tb.WriteRune(']')
		tb.WriteRune(' ')
	}

	tb.WriteString("Watchtower updates")

	if hostname != "" {
		tb.WriteString(" on ")
		tb.WriteString(hostname)
	}

	return tb.String()
}

// GetTemplateData populates the static notification data from flags and environment
func GetTemplateData(c *cobra.Command) StaticData {
	f := c.PersistentFlags()

	hostname, _ := f.GetString("notifications-hostname")
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	title := ""
	if skip, _ := f.GetBool("notification-skip-title"); !skip {
		tag, _ := f.GetString("notification-title-tag")
		if tag == "" {
			// For legacy email support
			tag, _ = f.GetString("notification-email-subjecttag")
		}
		title = GetTitle(hostname, tag)
	}

	return StaticData{
		Host:  hostname,
		Title: title,
	}
}

// ColorHex is the default notification color used for services that support it (formatted as a CSS hex string)
const ColorHex = "#406170"

// ColorInt is the default notification color used for services that support it (as an int value)
const ColorInt = 0x406170
