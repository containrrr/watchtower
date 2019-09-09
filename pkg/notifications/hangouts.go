package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	hangoutsType = "hangouts"
)

type hangoutsTypeNotifier struct {
	hangoutsURL      string
	logLevels      []log.Level
}

func newHangoutsNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {
	flags := c.PersistentFlags()

	hangoutsURL, _ := flags.GetString("notification-hangouts-url")
	if len(hangoutsURL) < 1 {
		log.Fatal("Required argument --notification-hangouts-url(cli) or WATCHTOWER_NOTIFICATION_HANGOUTS_CHAT_WEBHOOK_URL(env) is empty.")
	} else if !(strings.HasPrefix(hangoutsURL, "http://") || strings.HasPrefix(hangoutsURL, "https://")) {
		log.Fatal("Hangouts URL must start with \"http://\" or \"https://\"")
	} else if strings.HasPrefix(hangoutsURL, "http://") {
		log.Warn("Using an HTTP url for Hangouts is insecure")
	}

	n := &hangoutsTypeNotifier{
		hangoutsURL:      hangoutsURL,
		logLevels:      acceptedLogLevels,
	}

	log.AddHook(n)

	return n
}

func (n *hangoutsTypeNotifier) StartNotification() {}

func (n *hangoutsTypeNotifier) SendNotification() {}

func (n *hangoutsTypeNotifier) Levels() []log.Level {
	return n.logLevels
}

func (n *hangoutsTypeNotifier) getURL() string {
	return n.hangoutsURL
}

func (n *hangoutsTypeNotifier) Fire(entry *log.Entry) error {

	go func() {
		jsonBody, err := json.Marshal(hangoutsMessage{
			Text:  "(" + entry.Level.String() + "): " + entry.Message,
		})
		if err != nil {
			fmt.Println("Failed to create JSON body for Hangouts notification: ", err)
			return
		}

		jsonBodyBuffer := bytes.NewBuffer([]byte(jsonBody))
		resp, err := http.Post(n.getURL(), "application/json", jsonBodyBuffer)
		if err != nil {
			fmt.Println("Failed to send Hangouts notification: ", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Printf("Hangouts notification returned %d HTTP status code", resp.StatusCode)
		}

	}()
	return nil
}

type hangoutsMessage struct {
	Text  string `json:"text"`
}
