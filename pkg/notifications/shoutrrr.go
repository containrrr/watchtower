package notifications

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	shoutrrrDefaultTemplate = "{{range .}}{{.Message}}{{println}}{{end}}"
	shoutrrrType            = "shoutrrr"
)

// Implements Notifier, logrus.Hook
type shoutrrrTypeNotifier struct {
	Urls      []string
	Router    *router.ServiceRouter
	entries   []*log.Entry
	logLevels []log.Level
	template  *template.Template
}

func newShoutrrrNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {
	flags := c.PersistentFlags()

	urls, _ := flags.GetStringArray("notification-url")
	r, err := shoutrrr.CreateSender(urls...)
	if err != nil {
		log.Fatalf("Failed to initialize Shoutrrr notifications: %s\n", err.Error())
	}

	n := &shoutrrrTypeNotifier{
		Urls:      urls,
		Router:    r,
		logLevels: acceptedLogLevels,
		template:  getShoutrrrTemplate(c),
	}

	log.AddHook(n)

	return n
}

func (e *shoutrrrTypeNotifier) buildMessage(entries []*log.Entry) string {
	var body bytes.Buffer
	if err := e.template.Execute(&body, entries); err != nil {
		fmt.Printf("Failed to execute Shoutrrrr template: %s\n", err.Error())
	}

	return body.String()
}

func (e *shoutrrrTypeNotifier) sendEntries(entries []*log.Entry) {

	msg := e.buildMessage(entries)

	// Do the sending in a separate goroutine so we don't block the main process.
	go func() {
		errs := e.Router.Send(msg, nil)

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

func getShoutrrrTemplate(c *cobra.Command) *template.Template {
	var tpl *template.Template

	flags := c.PersistentFlags()

	tplString, err := flags.GetString("notification-template")

	// If we succeed in getting a non-empty template configuration
	// try to parse the template string.
	if tplString != "" && err == nil {
		tpl, err = template.New("").Parse(tplString)
	}

	// In case of errors (either from parsing the template string
	// or from getting the template configuration) log an error
	// message about this and the fact that we'll use the default
	// template instead.
	if err != nil {
		log.Errorf("Could not use configured notification template: %s. Using default template", err)
	}

	// If we had an error (either from parsing the template string
	// or from getting the template configuration) or we a
	// template wasn't configured (the empty template string)
	// fallback to using the default template.
	if err != nil || tplString == "" {
		tpl = template.Must(template.New("").Parse(shoutrrrDefaultTemplate))
	}

	return tpl
}
