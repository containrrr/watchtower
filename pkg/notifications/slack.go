package notifications

import (
	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	slackType = "slack"
)

type slackTypeNotifier struct {
	slackrus.SlackrusHook
}

func newSlackNotifier(_ *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {

	hookURL := viper.GetString("notification-slack-hook-url")
	userName := viper.GetString("notification-slack-identifier")
	channel := viper.GetString("notification-slack-channel")
	emoji := viper.GetString("notification-slack-icon-emoji")
	iconURL := viper.GetString("notification-slack-icon-url")

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

	log.AddHook(n)
	return n
}

func (s *slackTypeNotifier) StartNotification() {}

func (s *slackTypeNotifier) SendNotification() {}

func (s *slackTypeNotifier) Close() {}
