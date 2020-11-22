package notifications

import (
	shoutrrrTeams "github.com/containrrr/shoutrrr/pkg/services/teams"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net/url"
)

const (
	msTeamsType = "msteams"
)

type msTeamsTypeNotifier struct {
	webHookURL string
	levels     []log.Level
	data       bool
}

// NewMsTeamsNotifier is a factory method creating a new teams notifier instance
func NewMsTeamsNotifier() t.ConvertibleNotifier {
	return newMsTeamsNotifier()
}

func newMsTeamsNotifier() t.ConvertibleNotifier {

	webHookURL := viper.GetString("notification-msteams-hook")
	if len(webHookURL) <= 0 {
		log.Fatal("Required argument --notification-msteams-hook(cli) or WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL(env) is empty.")
	}

	withData := viper.GetBool("notification-msteams-data")
	n := &msTeamsTypeNotifier{
		webHookURL: webHookURL,
		data:       withData,
	}

	return n
}

func (n *msTeamsTypeNotifier) GetURL() (string, error) {
	webhookURL, err := url.Parse(n.webHookURL)
	if err != nil {
		return "", err
	}

	config, err := shoutrrrTeams.ConfigFromWebhookURL(*webhookURL)
	if err != nil {
		return "", err
	}

	config.Color = ColorHex
	config.Title = GetTitle()

	return config.GetURL().String(), nil
}
