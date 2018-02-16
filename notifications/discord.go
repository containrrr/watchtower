package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	discordType = "discord"
)

type discordTypeNotifier struct {
	WebhookURL string
	entries    []*log.Entry
}

func newDiscordNotifier(c *cli.Context) typeNotifier {
	n := &discordTypeNotifier{
		WebhookURL: c.GlobalString("notification-discord-webhook"),
	}

	log.AddHook(n)

	return n
}

func (n *discordTypeNotifier) sendEntries(entries []*log.Entry) {
	// Do the sending in a separate goroutine so we don't block the main process.
	message := ""
	for _, entry := range entries {
		message += entry.Time.Format("2006-01-02 15:04:05") + " (" + entry.Level.String() + "): " + entry.Message + "\r\n"
	}

	go func() {
		webhookBody := discordWebhookBody{Content: message}
		jsonBody, err := json.Marshal(webhookBody)
		if err != nil {
			fmt.Println("Failed to build JSON body for Discord notificattion: ", err)
			return
		}

		resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer([]byte(jsonBody)))
		if err != nil {
			fmt.Println("Failed to send Discord notificattion: ", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fmt.Println("Failed to send Discord notificattion. HTTP RESPONSE STATUS: ", resp.StatusCode)
		}
	}()
}

func (n *discordTypeNotifier) StartNotification() {
	if n.entries == nil {
		n.entries = make([]*log.Entry, 0, 10)
	}
}

func (n *discordTypeNotifier) SendNotification() {
	if n.entries != nil && len(n.entries) != 0 {
		n.sendEntries(n.entries)
	}
	n.entries = nil
}

func (n *discordTypeNotifier) Levels() []log.Level {
	// TODO: Make this configurable.
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	}
}

func (n *discordTypeNotifier) Fire(entry *log.Entry) error {
	if n.entries != nil {
		n.entries = append(n.entries, entry)
	} else {
		// Log output generated outside a cycle is sent immediately.
		n.sendEntries([]*log.Entry{entry})
	}
	return nil
}

type discordWebhookBody struct {
	Content string `json:"content"`
}
