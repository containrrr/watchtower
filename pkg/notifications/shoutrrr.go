package notifications

import (
	"bytes"
	"fmt"
	stdlog "log"
	"strings"
	"text/template"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/types"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	shoutrrrDefaultTemplate = "{{range .}}{{.Message}}{{println}}{{end}}"
	shoutrrrType            = "shoutrrr"
)

type router interface {
	Send(message string, params *types.Params) []error
}

// Implements Notifier, logrus.Hook
type shoutrrrTypeNotifier struct {
	Urls      []string
	Router    router
	entries   []*log.Entry
	logLevels []log.Level
	template  *template.Template
	messages  chan string
	done      chan bool
}

func (n *shoutrrrTypeNotifier) GetNames() []string {
	names := make([]string, len(n.Urls))
	for i, u := range n.Urls {
		schemeEnd := strings.Index(u, ":")
		if schemeEnd <= 0 {
			names[i] = "invalid"
			continue
		}
		names[i] = u[:schemeEnd]
	}
	return names
}

func newShoutrrrNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {
	flags := c.PersistentFlags()
	urls, _ := flags.GetStringArray("notification-url")
	tpl := getShoutrrrTemplate(c)
	return createSender(urls, acceptedLogLevels, tpl)
}

func newShoutrrrNotifierFromURL(c *cobra.Command, url string, levels []log.Level) t.Notifier {
	tpl := getShoutrrrTemplate(c)
	return createSender([]string{url}, levels, tpl)
}

func createSender(urls []string, levels []log.Level, template *template.Template) t.Notifier {

	traceWriter := log.StandardLogger().WriterLevel(log.TraceLevel)
	r, err := shoutrrr.NewSender(stdlog.New(traceWriter, "Shoutrrr: ", 0), urls...)
	if err != nil {
		log.Fatalf("Failed to initialize Shoutrrr notifications: %s\n", err.Error())
	}

	n := &shoutrrrTypeNotifier{
		Urls:      urls,
		Router:    r,
		messages:  make(chan string, 1),
		done:      make(chan bool),
		logLevels: levels,
		template:  template,
	}

	log.AddHook(n)

	// Do the sending in a separate goroutine so we don't block the main process.
	go sendNotifications(n)

	return n
}

func sendNotifications(n *shoutrrrTypeNotifier) {
	for msg := range n.messages {
		errs := n.Router.Send(msg, nil)

		for i, err := range errs {
			if err != nil {
				// Use fmt so it doesn't trigger another notification.
				fmt.Println("Failed to send notification via shoutrrr (url="+n.Urls[i]+"): ", err)
			}
		}
	}

	n.done <- true
}

func (n *shoutrrrTypeNotifier) buildMessage(entries []*log.Entry) string {
	var body bytes.Buffer
	if err := n.template.Execute(&body, entries); err != nil {
		fmt.Printf("Failed to execute Shoutrrrr template: %s\n", err.Error())
	}

	return body.String()
}

func (n *shoutrrrTypeNotifier) sendEntries(entries []*log.Entry) {
	msg := n.buildMessage(entries)
	n.messages <- msg
}

func (n *shoutrrrTypeNotifier) StartNotification() {
	if n.entries == nil {
		n.entries = make([]*log.Entry, 0, 10)
	}
}

func (n *shoutrrrTypeNotifier) SendNotification() {
	if n.entries == nil || len(n.entries) <= 0 {
		return
	}

	n.sendEntries(n.entries)
	n.entries = nil
}

func (n *shoutrrrTypeNotifier) Close() {
	close(n.messages)

	// Use fmt so it doesn't trigger another notification.
	fmt.Println("Waiting for the notification goroutine to finish")

	_ = <-n.done
}

func (n *shoutrrrTypeNotifier) Levels() []log.Level {
	return n.logLevels
}

func (n *shoutrrrTypeNotifier) Fire(entry *log.Entry) error {
	if n.entries != nil {
		n.entries = append(n.entries, entry)
	} else {
		// Log output generated outside a cycle is sent immediately.
		n.sendEntries([]*log.Entry{entry})
	}
	return nil
}

func getShoutrrrTemplate(c *cobra.Command) *template.Template {
	var tpl *template.Template

	flags := c.PersistentFlags()

	tplString, err := flags.GetString("notification-template")

	funcs := template.FuncMap{
		"ToUpper": strings.ToUpper,
		"ToLower": strings.ToLower,
		"Title":   strings.Title,
	}

	// If we succeed in getting a non-empty template configuration
	// try to parse the template string.
	if tplString != "" && err == nil {
		tpl, err = template.New("").Funcs(funcs).Parse(tplString)
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
		tpl = template.Must(template.New("").Funcs(funcs).Parse(shoutrrrDefaultTemplate))
	}

	return tpl
}
