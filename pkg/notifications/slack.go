package notifications

import (
	"strings"

	shoutrrrDisco "github.com/containrrr/shoutrrr/pkg/services/discord"
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

func newSlackNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertibleNotifier {
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

func (s *slackTypeNotifier) GetURL(c *cobra.Command, title string) (string, error) {
	trimmedURL := strings.TrimRight(s.HookURL, "/")
	trimmedURL = strings.TrimLeft(trimmedURL, "https://")
	parts := strings.Split(trimmedURL, "/")

	if parts[0] == "discord.com" || parts[0] == "discordapp.com" {
		log.Debug("Detected a discord slack wrapper URL, using shoutrrr discord service")
		conf := &shoutrrrDisco.Config{
			WebhookID:  parts[len(parts)-3],
			Token:      parts[len(parts)-2],
			Color:      ColorInt,
			Title:      title,
			SplitLines: true,
			Username:   s.Username,
		}
		return conf.GetURL().String(), nil
	}

	webhookToken := strings.Replace(s.HookURL, "https://hooks.slack.com/services/", "", 1)

	conf := &shoutrrrSlack.Config{
		BotName: s.Username,
		Color:   ColorHex,
		Channel: "webhook",
		Title:   title,
	}

	if s.IconURL != "" {
		conf.Icon = s.IconURL
	} else if s.IconEmoji != "" {
		conf.Icon = s.IconEmoji
	}

	if err := conf.Token.SetFromProp(webhookToken); err != nil {
		return "", err
	}

	return conf.GetURL().String(), nil
}
