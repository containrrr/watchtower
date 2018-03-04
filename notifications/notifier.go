package notifications

import (
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type typeNotifier interface {
	StartNotification()
	SendNotification()
}

// Notifier can send log output as notification to admins, with optional batching.
type Notifier struct {
	types []typeNotifier
}

// NewNotifier creates and returns a new Notifier, using global configuration.
func NewNotifier(c *cli.Context) *Notifier {
	n := &Notifier{}

	logLevel, err := log.ParseLevel(c.GlobalString("notifications-level"))
	if err != nil {
		log.Fatalf("Notifications invalid log level: %s", err.Error())
	}

	acceptedLogLevels := slackrus.LevelThreshold(logLevel)

	// Parse types and create notifiers.
	types := c.GlobalStringSlice("notifications")
	for _, t := range types {
		var tn typeNotifier
		switch t {
		case emailType:
			tn = newEmailNotifier(c, acceptedLogLevels)
		case slackType:
			tn = newSlackNotifier(c, acceptedLogLevels)
		case msTeamsType:
			tn = newMsTeamsNotifier(c, acceptedLogLevels)
		default:
			log.Fatalf("Unknown notification type %q", t)
		}
		n.types = append(n.types, tn)
	}

	return n
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
