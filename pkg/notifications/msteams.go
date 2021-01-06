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

	baseURL := "https://outlook.office.com/webhook/"

	path := strings.Replace(n.webHookURL, baseURL, "", 1)
	rawToken := strings.Replace(path, "/IncomingWebhook", "", 1)
	token := strings.Split(rawToken, "/")
	config := &shoutrrrTeams.Config{
		Token: shoutrrrTeams.Token{
			A: token[0],
			B: token[1],
			C: token[2],
		},
	}

	return config.GetURL().String()
}

func (n *msTeamsTypeNotifier) StartNotification()          {}
func (n *msTeamsTypeNotifier) SendNotification()           {}
func (n *msTeamsTypeNotifier) Close()                      {}
func (n *msTeamsTypeNotifier) Levels() []log.Level         { return nil }
func (n *msTeamsTypeNotifier) Fire(entry *log.Entry) error { return nil }
