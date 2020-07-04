package notifications

import (
	ty "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

	// Parse types and create notifiers.
	types, err := f.GetStringSlice("notifications")
	if err != nil {
		log.WithField("could not read notifications argument", log.Fields{"Error": err}).Fatal()
	}

	n.types = n.GetNotificationTypes(c, acceptedLogLevels, types)

	return n
}

func convertToShoutrrrURL(c *cobra.Command, notificationType string) string {

	switch notificationType {
	case emailType:
		emailNotifier := newEmailNotifier(c, []log.Level{})
		return emailNotifier.GetURL()
	default:
		return ""
	}
}

// GetNotificationTypes produces an array of notifiers from a list of types
func (n *Notifier) GetNotificationTypes(c *cobra.Command, acceptedLogLevels []log.Level, types []string) []ty.Notifier {
	output := make([]ty.Notifier, 0)

	for _, t := range types {
		var tn ty.Notifier
		switch t {
		case emailType:
			tn = newEmailNotifier(c, acceptedLogLevels)
		case slackType:
			tn = newSlackNotifier(c, acceptedLogLevels)
		case msTeamsType:
			tn = newMsTeamsNotifier(c, acceptedLogLevels)
		case gotifyType:
			tn = newGotifyNotifier(c, acceptedLogLevels)
		case shoutrrrType:
			tn = newShoutrrrNotifier(c, acceptedLogLevels)
		default:
			log.Fatalf("Unknown notification type %q", t)
		}
		output = append(output, tn)
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
