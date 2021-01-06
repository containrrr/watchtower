package notifications

import (
	"strings"

	shoutrrrSlack "github.com/containrrr/shoutrrr/pkg/services/slack"
	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	slackType = "slack"
)

type slackTypeNotifier struct {
	slackrus.SlackrusHook
}

// NewSlackNotifier is a factory function used to generate new instance of the slack notifier type
func NewSlackNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {
	return newSlackNotifier(c, acceptedLogLevels)
}

func newSlackNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {
	flags := c.PersistentFlags()

	hookURL, _ := flags.GetString("notification-slack-hook-url")
	userName, _ := flags.GetString("notification-slack-identifier")
	channel, _ := flags.GetString("notification-slack-channel")
	emoji, _ := flags.GetString("notification-slack-icon-emoji")
	iconURL, _ := flags.GetString("notification-slack-icon-url")
	n := &slackTypeNotifier{
		SlackrusHook: slackrus.SlackrusHook{
			HookURL:        hookURL,
			Username:       userName,
			Channel:        channel,
			IconEmoji:      emoji,
			IconURL:        iconURL,
			AcceptedLevels: acceptedLogLevels,
		},
	}
	return n
}

func (s *slackTypeNotifier) GetURL() string {
	rawTokens := strings.Replace(s.HookURL, "https://hooks.slack.com/services/", "", 1)
	tokens := strings.Split(rawTokens, "/")

	conf := &shoutrrrSlack.Config{
		BotName: s.Username,
		Token: shoutrrrSlack.Token{
			A: tokens[0],
			B: tokens[1],
			C: tokens[2],
		},
	}

	return conf.GetURL().String()
}

func (s *slackTypeNotifier) StartNotification() {
}

func (s *slackTypeNotifier) SendNotification() {}

func (s *slackTypeNotifier) Close() {}
