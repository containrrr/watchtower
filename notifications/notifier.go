package notifications

import (
	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

type typeNotifier interface {
	StartNotification()
	SendNotification()
}

type Notifier struct {
	types []typeNotifier
}

func NewNotifier(c *cli.Context) *Notifier {
	n := &Notifier{}

	// Parse types and create notifiers.
	types := c.GlobalStringSlice("notifications")
	for _, t := range types {
		var tn typeNotifier
		switch t {
		case emailType:
			tn = newEmailNotifier(c)
		default:
			log.Fatalf("Unknown notification type %q", t)
		}
		n.types = append(n.types, tn)
	}

	return n
}

func (n *Notifier) StartNotification() {
	for _, t := range n.types {
		t.StartNotification()
	}
}

func (n *Notifier) SendNotification() {
	for _, t := range n.types {
		t.SendNotification()
	}
}
