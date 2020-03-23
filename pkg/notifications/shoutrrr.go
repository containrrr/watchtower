package notifications

import (
	"fmt"
	"github.com/containrrr/shoutrrr"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	shoutrrrType = "shoutrrr"
)

// Implements Notifier, logrus.Hook
type shoutrrrTypeNotifier struct {
	Urls      []string
	entries   []*log.Entry
	logLevels []log.Level
}

func newShoutrrrNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {
	flags := c.PersistentFlags()

	urls, _ := flags.GetStringArray("notification-url")

	n := &shoutrrrTypeNotifier{
		Urls:      urls,
		logLevels: acceptedLogLevels,
	}

	log.AddHook(n)

	return n
}

func (e *shoutrrrTypeNotifier) buildMessage(entries []*log.Entry) string {
	body := ""
	for _, entry := range entries {
		body += entry.Time.Format("2006-01-02 15:04:05") + " (" + entry.Level.String() + "): " + entry.Message + "\r\n"
		// We don't use fields in watchtower, so don't bother sending them.
	}

	return body
}

func (e *shoutrrrTypeNotifier) sendEntries(entries []*log.Entry) {
	// Do the sending in a separate goroutine so we don't block the main process.
	msg := e.buildMessage(entries)
	go func() {
		router, _ := shoutrrr.CreateSender(e.Urls...)
		errs := router.Send(msg, nil)

		for i, err := range errs {
			if err != nil {
				// Use fmt so it doesn't trigger another notification.
				fmt.Println("Failed to send notification via shoutrrr (url="+e.Urls[i]+"): ", err)
			}
		}
	}()
}

func (e *shoutrrrTypeNotifier) StartNotification() {
	if e.entries == nil {
		e.entries = make([]*log.Entry, 0, 10)
	}
}

func (e *shoutrrrTypeNotifier) SendNotification() {
	if e.entries == nil || len(e.entries) <= 0 {
		return
	}

	e.sendEntries(e.entries)
	e.entries = nil
}

func (e *shoutrrrTypeNotifier) Levels() []log.Level {
	return e.logLevels
}

func (e *shoutrrrTypeNotifier) Fire(entry *log.Entry) error {
	if e.entries != nil {
		e.entries = append(e.entries, entry)
	} else {
		// Log output generated outside a cycle is sent immediately.
		e.sendEntries([]*log.Entry{entry})
	}
	return nil
}
