package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"io/ioutil"
)

const (
	msTeamsType = "msteams"
)

type msTeamsTypeNotifier struct {
	webHookURL string
	levels     []log.Level
	data       bool
}

func newMsTeamsNotifier(c *cli.Context, acceptedLogLevels []log.Level) typeNotifier {

	webHookURL := c.GlobalString("notification-msteams-hook")
	if len(webHookURL) <= 0 {
		log.Fatal("Required argument --notification-msteams-hook(cli) or WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL(env) is empty.")
	}

	n := &msTeamsTypeNotifier{
		levels:     acceptedLogLevels,
		webHookURL: webHookURL,
		data:       c.GlobalBool("notification-msteams-data"),
	}

	log.AddHook(n)

	return n
}

func (n *msTeamsTypeNotifier) StartNotification() {}

func (n *msTeamsTypeNotifier) SendNotification() {}

func (n *msTeamsTypeNotifier) Levels() []log.Level {
	return n.levels
}

func (n *msTeamsTypeNotifier) Fire(entry *log.Entry) error {

	message := "(" + entry.Level.String() + "): " + entry.Message

	go func() {
		webHookBody := messageCard{
			CardType: "MessageCard",
			Context:  "http://schema.org/extensions",
			Markdown: true,
			Text:     message,
		}

		if n.data && entry.Data != nil && len(entry.Data) > 0 {
			section := messageCardSection{
				Facts: make([]messageCardSectionFact, len(entry.Data)),
				Text:  "",
			}

			index := 0
			for k, v := range entry.Data {
				section.Facts[index] = messageCardSectionFact{
					Name:  k,
					Value: fmt.Sprint(v),
				}
				index++
			}

			webHookBody.Sections = []messageCardSection{section}
		}

		jsonBody, err := json.Marshal(webHookBody)
		if err != nil {
			fmt.Println("Failed to build JSON body for MSTeams notificattion: ", err)
			return
		}

		resp, err := http.Post(n.webHookURL, "application/json", bytes.NewBuffer([]byte(jsonBody)))
		if err != nil {
			fmt.Println("Failed to send MSTeams notificattion: ", err)
		}

		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fmt.Println("Failed to send MSTeams notificattion. HTTP RESPONSE STATUS: ", resp.StatusCode)
			if resp.Body != nil {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					bodyString := string(bodyBytes)
					fmt.Println(bodyString)
				}
			}
		}
	}()

	return nil
}

type messageCard struct {
	CardType      string               `json:"@type"`
	Context       string               `json:"@context"`
	CorrelationID string               `json:"correlationId,omitempty"`
	ThemeColor    string               `json:"themeColor,omitempty"`
	Summary       string               `json:"summary,omitempty"`
	Title         string               `json:"title,omitempty"`
	Text          string               `json:"text,omitempty"`
	Markdown      bool                 `json:"markdown,bool"`
	Sections      []messageCardSection `json:"sections,omitempty"`
}

type messageCardSection struct {
	Title            string                   `json:"title,omitempty"`
	Text             string                   `json:"text,omitempty"`
	ActivityTitle    string                   `json:"activityTitle,omitempty"`
	ActivitySubtitle string                   `json:"activitySubtitle,omitempty"`
	ActivityImage    string                   `json:"activityImage,omitempty"`
	ActivityText     string                   `json:"activityText,omitempty"`
	HeroImage        string                   `json:"heroImage,omitempty"`
	Facts            []messageCardSectionFact `json:"facts,omitempty"`
}

type messageCardSectionFact struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
