package notifications

import (
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

func newSlackNotifier(c *cobra.Command, acceptedLogLevels []log.Level) typeNotifier {
	flags := c.PersistentFlags()

	hookUrl,  _ := flags.GetString("notification-slack-hook-url")
	userName, _ := flags.GetString("notification-slack-identifier")
	channel,  _ := flags.GetString("notification-slack-channel")
	emoji,    _ := flags.GetString("notification-slack-icon-emoji")
	iconUrl,  _ := flags.GetString("notification-slack-icon-url")

	n := &slackTypeNotifier{
		SlackrusHook: slackrus.SlackrusHook{
			HookURL:        hookUrl,
			Username:       userName,
			Channel:        channel,
			IconEmoji:      emoji,
			IconURL:        iconUrl,
			AcceptedLevels: acceptedLogLevels,
		},
	}

	log.AddHook(n)
	return n
}

func (s *slackTypeNotifier) StartNotification() {}

func (s *slackTypeNotifier) SendNotification() {}
