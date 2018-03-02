package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	discordType = "discord"
)

type discordTypeNotifier struct {
	webHookURL     string
	acceptedLevels []log.Level
}

func newDiscordNotifier(c *cli.Context, acceptedLogLevels []log.Level) typeNotifier {
	n := &discordTypeNotifier{
		webHookURL:     c.GlobalString("notification-discord-webhook"),
		acceptedLevels: acceptedLogLevels,
	}

	log.AddHook(n)

	return n
}

func (n *discordTypeNotifier) sendEntry(entry *log.Entry) {

	message := "(" + entry.Level.String() + "): " + entry.Message
	go func() {
		webHookBody := discordWebHookBody{Content: message}
		jsonBody, err := json.Marshal(webHookBody)
		if err != nil {
			fmt.Println("Failed to build JSON body for Discord notificattion: ", err)
			return
		}

		resp, err := http.Post(n.webHookURL, "application/json", bytes.NewBuffer([]byte(jsonBody)))
		if err != nil {
			fmt.Println("Failed to send Discord notificattion: ", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fmt.Println("Failed to send Discord notificattion. HTTP RESPONSE STATUS: ", resp.StatusCode)
		}
	}()
}

func (n *discordTypeNotifier) StartNotification() {}

func (n *discordTypeNotifier) SendNotification() {}

func (n *discordTypeNotifier) Fire(entry *log.Entry) error {
	n.sendEntry(entry)
	return nil
}

func (e *discordTypeNotifier) Levels() []log.Level {
	return e.acceptedLevels
}

type discordWebHookBody struct {
	Content string `json:"content"`
}
