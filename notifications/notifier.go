package notifications

import (
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

	// Parse types and create notifiers.
	types := c.GlobalStringSlice("notifications")
	for _, t := range types {
		var tn typeNotifier
		switch t {
		case emailType:
			tn = newEmailNotifier(c)
		case slackType:
			tn = newSlackNotifier(c)
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
