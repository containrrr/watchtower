package notifications

import (
	"strings"

	shoutrrrDisco "github.com/containrrr/shoutrrr/pkg/services/discord"
	shoutrrrSlack "github.com/containrrr/shoutrrr/pkg/services/slack"
	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	slackType = "slack"
)

type slackTypeNotifier struct {
	slackrus.SlackrusHook
}

// NewSlackNotifier is a factory function used to generate new instance of the slack notifier type
func NewSlackNotifier() t.ConvertibleNotifier {
	return newSlackNotifier()
}

func newSlackNotifier() t.ConvertibleNotifier {

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
		},
	}
	return n
}

func (s *slackTypeNotifier) GetURL() (string, error) {
	trimmedURL := strings.TrimRight(s.HookURL, "/")
	trimmedURL = strings.TrimLeft(trimmedURL, "https://")
	parts := strings.Split(trimmedURL, "/")

	if parts[0] == "discord.com" || parts[0] == "discordapp.com" {
		log.Debug("Detected a discord slack wrapper URL, using shoutrrr discord service")
		conf := &shoutrrrDisco.Config{
			Channel:    parts[len(parts)-3],
			Token:      parts[len(parts)-2],
			Color:      ColorInt,
			Title:      GetTitle(),
			SplitLines: true,
			Username:   s.Username,
		}
		return conf.GetURL().String(), nil
	}

	rawTokens := strings.Replace(s.HookURL, "https://hooks.slack.com/services/", "", 1)
	tokens := strings.Split(rawTokens, "/")

	conf := &shoutrrrSlack.Config{
		BotName: s.Username,
		Token:   tokens,
		Color:   ColorHex,
		Title:   GetTitle(),
	}

	return conf.GetURL().String(), nil
}
