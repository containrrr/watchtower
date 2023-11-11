package notifications

import (
	"bytes"
	stdlog "log"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/types"
	"github.com/containrrr/watchtower/pkg/notifications/templates"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

// LocalLog is a logrus logger that does not send entries as notifications
var LocalLog = log.WithField("notify", "no")

const (
	shoutrrrType = "shoutrrr"
)

type router interface {
	Send(message string, params *types.Params) []error
}

// Implements Notifier, logrus.Hook
type shoutrrrTypeNotifier struct {
	Urls           []string
	Router         router
	entries        []*log.Entry
	logLevel       log.Level
	template       *template.Template
	messages       chan string
	done           chan bool
	legacyTemplate bool
	params         *types.Params
	data           StaticData
	receiving      bool
	delay          time.Duration
}

// GetScheme returns the scheme part of a Shoutrrr URL
func GetScheme(url string) string {
	schemeEnd := strings.Index(url, ":")
	if schemeEnd <= 0 {
		return "invalid"
	}
	return url[:schemeEnd]
}

// GetNames returns a list of notification services that has been added
func (n *shoutrrrTypeNotifier) GetNames() []string {
	names := make([]string, len(n.Urls))
	for i, u := range n.Urls {
		names[i] = GetScheme(u)
	}
	return names
}

// GetURLs returns a list of URLs for notification services that has been added
func (n *shoutrrrTypeNotifier) GetURLs() []string {
	return n.Urls
}

// AddLogHook adds the notifier as a receiver of log messages and starts a go func for processing them
func (n *shoutrrrTypeNotifier) AddLogHook() {
	if n.receiving {
		return
	}
	n.receiving = true
	log.AddHook(n)

	// Do the sending in a separate goroutine, so we don't block the main process.
	go sendNotifications(n)
}

func createNotifier(urls []string, level log.Level, tplString string, legacy bool, data StaticData, stdout bool, delay time.Duration) *shoutrrrTypeNotifier {
	tpl, err := getShoutrrrTemplate(tplString, legacy)
	if err != nil {
		log.Errorf("Could not use configured notification template: %s. Using default template", err)
	}

	var logger types.StdLogger
	if stdout {
		logger = stdlog.New(os.Stdout, ``, 0)
	} else {
		logger = stdlog.New(log.StandardLogger().WriterLevel(log.TraceLevel), "Shoutrrr: ", 0)
	}
	r, err := shoutrrr.NewSender(logger, urls...)
	if err != nil {
		log.Fatalf("Failed to initialize Shoutrrr notifications: %s\n", err.Error())
	}

	params := &types.Params{}
	if data.Title != "" {
		params.SetTitle(data.Title)
	}

	return &shoutrrrTypeNotifier{
		Urls:           urls,
		Router:         r,
		messages:       make(chan string, 1),
		done:           make(chan bool),
		logLevel:       level,
		template:       tpl,
		legacyTemplate: legacy,
		data:           data,
		params:         params,
		delay:          delay,
	}
}

func sendNotifications(n *shoutrrrTypeNotifier) {
	for msg := range n.messages {
		time.Sleep(n.delay)
		errs := n.Router.Send(msg, n.params)

		for i, err := range errs {
			if err != nil {
				scheme := GetScheme(n.Urls[i])
				// Use fmt so it doesn't trigger another notification.
				LocalLog.WithFields(log.Fields{
					"service": scheme,
					"index":   i,
				}).WithError(err).Error("Failed to send shoutrrr notification")
			}
		}
	}

	n.done <- true
}

func (n *shoutrrrTypeNotifier) buildMessage(data Data) (string, error) {
	var body bytes.Buffer
	var templateData interface{} = data
	if n.legacyTemplate {
		templateData = data.Entries
	}
	if err := n.template.Execute(&body, templateData); err != nil {
		return "", err
	}

	return body.String(), nil
}

func (n *shoutrrrTypeNotifier) sendEntries(entries []*log.Entry, report t.Report) {
	msg, err := n.buildMessage(Data{n.data, entries, report})

	if msg == "" {
		// Log in go func in case we entered from Fire to avoid stalling
		go func() {
			if err != nil {
				LocalLog.WithError(err).Fatal("Notification template error")
			} else if len(n.Urls) > 1 {
				LocalLog.Info("Skipping notification due to empty message")
			}
		}()
		return
	}
	n.messages <- msg
}

// StartNotification begins queueing up messages to send them as a batch
func (n *shoutrrrTypeNotifier) StartNotification() {
	if n.entries == nil {
		n.entries = make([]*log.Entry, 0, 10)
	}
}

// SendNotification sends the queued up messages as a notification
func (n *shoutrrrTypeNotifier) SendNotification(report t.Report) {
	n.sendEntries(n.entries, report)
	n.entries = nil
}

// Close prevents further messages from being queued and waits until all the currently queued up messages have been sent
func (n *shoutrrrTypeNotifier) Close() {
	close(n.messages)

	// Use fmt so it doesn't trigger another notification.
	LocalLog.Info("Waiting for the notification goroutine to finish")

	<-n.done
}

// Levels return what log levels trigger notifications
func (n *shoutrrrTypeNotifier) Levels() []log.Level {
	return log.AllLevels[:n.logLevel+1]
}

// Fire is the hook that logrus calls on a new log message
func (n *shoutrrrTypeNotifier) Fire(entry *log.Entry) error {
	if entry.Data["notify"] == "no" {
		// Skip logging if explicitly tagged as non-notify
		return nil
	}
	if n.entries != nil {
		n.entries = append(n.entries, entry)
	} else {
		// Log output generated outside a cycle is sent immediately.
		n.sendEntries([]*log.Entry{entry}, nil)
	}
	return nil
}

func getShoutrrrTemplate(tplString string, legacy bool) (tpl *template.Template, err error) {

	tplBase := template.New("").Funcs(templates.Funcs)

	if builtin, found := commonTemplates[tplString]; found {
		log.WithField(`template`, tplString).Debug(`Using common template`)
		tplString = builtin
	}

	// If we succeed in getting a non-empty template configuration
	// try to parse the template string.
	if tplString != "" {
		tpl, err = tplBase.Parse(tplString)
	}

	// If we had an error (either from parsing the template string
	// or from getting the template configuration) or a
	// template wasn't configured (the empty template string)
	// fallback to using the default template.
	if err != nil || tplString == "" {
		defaultKey := `default`
		if legacy {
			defaultKey = `default-legacy`
		}

		tpl = template.Must(tplBase.Parse(commonTemplates[defaultKey]))
	}

	return
}
