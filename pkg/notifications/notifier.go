package notifications

import (
	ty "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

// Notifier can send log output as notification to admins, with optional batching.
type Notifier struct {
	types []ty.Notifier
}

// NewNotifier creates and returns a new Notifier, using global configuration.
func NewNotifier(c *cobra.Command) *Notifier {
	n := &Notifier{}

	f := c.PersistentFlags()

	level, _ := f.GetString("notifications-level")
	logLevel, err := log.ParseLevel(level)
	if err != nil {
		log.Fatalf("Notifications invalid log level: %s", err.Error())
	}

	acceptedLogLevels := slackrus.LevelThreshold(logLevel)
	// slackrus does not allow log level TRACE, even though it's an accepted log level for logrus
	if len(acceptedLogLevels) == 0 {
		log.Fatalf("Unsupported notification log level provided: %s", level)
	}

	// Parse types and create notifiers.
	types, err := f.GetStringSlice("notifications")
	if err != nil {
		log.WithField("could not read notifications argument", log.Fields{"Error": err}).Fatal()
	}

	n.types = n.getNotificationTypes(c, acceptedLogLevels, types)

	return n
}

func (n *Notifier) String() string {
	if len(n.types) < 1 {
		return ""
	}

	sb := strings.Builder{}
	for _, notif := range n.types {
		for _, name := range notif.GetNames() {
			sb.WriteString(name)
			sb.WriteString(", ")
		}
	}

	if sb.Len() < 2 {
		// No notification services are configured, return early as the separator strip is not applicable
		return "none"
	}

	names := sb.String()

	// remove the last separator
	names = names[:len(names)-2]

	return names
}

// getNotificationTypes produces an array of notifiers from a list of types
func (n *Notifier) getNotificationTypes(cmd *cobra.Command, levels []log.Level, types []string) []ty.Notifier {
	output := make([]ty.Notifier, 0)

	for _, t := range types {

		if t == shoutrrrType {
			output = append(output, newShoutrrrNotifier(cmd, levels))
			continue
		}

		var legacyNotifier ty.ConvertibleNotifier
		var err error

		switch t {
		case emailType:
			legacyNotifier = newEmailNotifier(cmd, []log.Level{})
		case slackType:
			legacyNotifier = newSlackNotifier(cmd, []log.Level{})
		case msTeamsType:
			legacyNotifier = newMsTeamsNotifier(cmd, levels)
		case gotifyType:
			legacyNotifier = newGotifyNotifier(cmd, []log.Level{})
		default:
			log.Fatalf("Unknown notification type %q", t)
			// Not really needed, used for nil checking static analysis
			continue
		}

		shoutrrrURL, err := legacyNotifier.GetURL(cmd)
		if err != nil {
			log.Fatal("failed to create notification config:", err)
		}

		log.WithField("URL", shoutrrrURL).Trace("created Shoutrrr URL from legacy notifier")

		notifier := newShoutrrrNotifierFromURL(
			cmd,
			shoutrrrURL,
			levels,
		)

		output = append(output, notifier)
	}

	return output
}

// StartNotification starts a log batch. Notifications will be accumulated after this point and only sent when SendNotification() is called.
func (n *Notifier) StartNotification() {
	for _, t := range n.types {
		t.StartNotification()
	}
}

// SendNotification sends any notifications accumulated since StartNotification() was called.
func (n *Notifier) SendNotification() {
	for _, t := range n.types {
		t.SendNotification()
	}
}

// Close closes all notifiers.
func (n *Notifier) Close() {
	for _, t := range n.types {
		t.Close()
	}
}

// GetTitle returns a common notification title with hostname appended
func GetTitle(c *cobra.Command) (title string) {
	title = "Watchtower updates"

	f := c.PersistentFlags()

	hostname, _ := f.GetString("notifications-hostname")

	if hostname != "" {
		title += " on " + hostname
	} else if hostname, err := os.Hostname(); err == nil {
		title += " on " + hostname
	}

	return
}

// ColorHex is the default notification color used for services that support it (formatted as a CSS hex string)
const ColorHex = "#406170"

// ColorInt is the default notification color used for services that support it (as an int value)
const ColorInt = 0x406170
