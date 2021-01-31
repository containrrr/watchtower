package notifications

import (
	"strings"

	shoutrrrTeams "github.com/containrrr/shoutrrr/pkg/services/teams"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
func NewMsTeamsNotifier(cmd *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {
	return newMsTeamsNotifier(cmd, acceptedLogLevels)
}

func newMsTeamsNotifier(cmd *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {

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

func (n *msTeamsTypeNotifier) GetURL() string {

	webhookURL := n.webHookURL
	if webhookURL[len(webhookURL)-1] != '/' {
		webhookURL += "/"
	}

	config, err := (&shoutrrrTeams.Config{}).SetFromWebhookURL(webhookURL)

	if err != nil {
		log.WithFields(
			log.Fields{
				"Original Webhook URL": n.webHookURL,
				"Mutated Webhook URL":  webhookURL,
			}).Error(err)
		return ""
	}

	return config.GetURL().String()
}

func (n *msTeamsTypeNotifier) StartNotification()          {}
func (n *msTeamsTypeNotifier) SendNotification()           {}
func (n *msTeamsTypeNotifier) Close()                      {}
func (n *msTeamsTypeNotifier) Levels() []log.Level         { return nil }
func (n *msTeamsTypeNotifier) Fire(entry *log.Entry) error { return nil }
