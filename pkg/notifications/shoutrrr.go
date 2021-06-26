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
)

const (
	shoutrrrDefaultLegacyTemplate = "{{range .}}{{.Message}}{{println}}{{end}}"
	shoutrrrDefaultTemplate       = `{{- with .Report -}}
{{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed
{{range .Updated -}}
- {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
{{end -}}
{{range .Fresh -}}
- {{.Name}} ({{.ImageName}}): {{.State}}
{{end -}}
{{range .Skipped -}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
{{end -}}
{{range .Failed -}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
{{end -}}
{{end -}}`
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
	logLevels      []log.Level
	template       *template.Template
	messages       chan string
	done           chan bool
	legacyTemplate bool
}

// GetScheme returns the scheme part of a Shoutrrr URL
func GetScheme(url string) string {
	schemeEnd := strings.Index(url, ":")
	if schemeEnd <= 0 {
		return "invalid"
	}
	return url[:schemeEnd]
}

func (n *shoutrrrTypeNotifier) GetNames() []string {
	names := make([]string, len(n.Urls))
	for i, u := range n.Urls {
		names[i] = GetScheme(u)
	}
	return names
}

func newShoutrrrNotifier(tplString string, acceptedLogLevels []log.Level, legacy bool, urls ...string) t.Notifier {

	notifier := createNotifier(urls, acceptedLogLevels, tplString, legacy)
	log.AddHook(notifier)

	// Do the sending in a separate goroutine so we don't block the main process.
	go sendNotifications(notifier)

	return notifier
}

func createNotifier(urls []string, levels []log.Level, tplString string, legacy bool) *shoutrrrTypeNotifier {
	tpl, err := getShoutrrrTemplate(tplString, legacy)
	if err != nil {
		log.Errorf("Could not use configured notification template: %s. Using default template", err)
	}

	traceWriter := log.StandardLogger().WriterLevel(log.TraceLevel)
	r, err := shoutrrr.NewSender(stdlog.New(traceWriter, "Shoutrrr: ", 0), urls...)
	if err != nil {
		log.Fatalf("Failed to initialize Shoutrrr notifications: %s\n", err.Error())
	}

	return &shoutrrrTypeNotifier{
		Urls:           urls,
		Router:         r,
		messages:       make(chan string, 1),
		done:           make(chan bool),
		logLevels:      levels,
		template:       tpl,
		legacyTemplate: legacy,
	}
}

func sendNotifications(n *shoutrrrTypeNotifier) {
	for msg := range n.messages {
		errs := n.Router.Send(msg, nil)

		for i, err := range errs {
			if err != nil {
				scheme := GetScheme(n.Urls[i])
				// Use fmt so it doesn't trigger another notification.
				fmt.Printf("Failed to send shoutrrr notification (#%d, %s): %v\n", i, scheme, err)
			}
		}
	}

	n.done <- true
}

func (n *shoutrrrTypeNotifier) buildMessage(data Data) string {
	var body bytes.Buffer
	var templateData interface{} = data
	if n.legacyTemplate {
		templateData = data.Entries
	}
	if err := n.template.Execute(&body, templateData); err != nil {
		fmt.Printf("Failed to execute Shoutrrrr template: %s\n", err.Error())
	}

	return body.String()
}

func (n *shoutrrrTypeNotifier) sendEntries(entries []*log.Entry, report t.Report) {
	msg := n.buildMessage(Data{entries, report})
	n.messages <- msg
}

func (n *shoutrrrTypeNotifier) StartNotification() {
	if n.entries == nil {
		n.entries = make([]*log.Entry, 0, 10)
	}
}

func (n *shoutrrrTypeNotifier) SendNotification(report t.Report) {
	//if n.entries == nil || len(n.entries) <= 0 {
	//	return
	//}

	n.sendEntries(n.entries, report)
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
		n.sendEntries([]*log.Entry{entry}, nil)
	}
	return nil
}

func getShoutrrrTemplate(tplString string, legacy bool) (tpl *template.Template, err error) {
	funcs := template.FuncMap{
		"ToUpper": strings.ToUpper,
		"ToLower": strings.ToLower,
		"Title":   strings.Title,
	}
	tplBase := template.New("").Funcs(funcs)

	// If we succeed in getting a non-empty template configuration
	// try to parse the template string.
	if tplString != "" {
		tpl, err = tplBase.Parse(tplString)
	}

	// If we had an error (either from parsing the template string
	// or from getting the template configuration) or we a
	// template wasn't configured (the empty template string)
	// fallback to using the default template.
	if err != nil || tplString == "" {
		defaultTemplate := shoutrrrDefaultTemplate
		if legacy {
			defaultTemplate = shoutrrrDefaultLegacyTemplate
		}

		tpl = template.Must(tplBase.Parse(defaultTemplate))
	}

	return
}

// Data is the notification template data model
type Data struct {
	Entries []*log.Entry
	Report  t.Report
}
