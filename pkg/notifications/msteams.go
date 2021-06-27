package notifications

import (
	shoutrrrTeams "github.com/containrrr/shoutrrr/pkg/services/teams"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

func newMsTeamsNotifier(cmd *cobra.Command, acceptedLogLevels []log.Level) t.ConvertibleNotifier {

	flags := cmd.PersistentFlags()

	webHookURL, _ := flags.GetString("notification-msteams-hook")
	if len(webHookURL) <= 0 {
		log.Fatal("Required argument --notification-msteams-hook(cli) or WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL(env) is empty.")
	}

	withData, _ := flags.GetBool("notification-msteams-data")
	n := &msTeamsTypeNotifier{
		levels:     acceptedLogLevels,
		webHookURL: webHookURL,
		data:       withData,
	}

	return n
}

func (n *msTeamsTypeNotifier) GetURL(c *cobra.Command) (string, error) {
	webhookURL, err := url.Parse(n.webHookURL)
	if err != nil {
		return "", err
	}

	config, err := shoutrrrTeams.ConfigFromWebhookURL(*webhookURL)
	if err != nil {
		return "", err
	}

	config.Color = ColorHex
	config.Title = GetTitle(c)

	return config.GetURL().String(), nil
}
