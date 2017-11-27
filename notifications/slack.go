package notifications

import (
	"github.com/johntdyer/slackrus"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	slackType = "slack"
)

type slackTypeNotifier struct {
	slackrus.SlackrusHook
}

func newSlackNotifier(c *cli.Context) typeNotifier {
	logLevel, err := log.ParseLevel(c.GlobalString("notification-slack-level"))
	if err != nil {
		log.Fatalf("Slack notifications: %s", err.Error())
	}

	n := &slackTypeNotifier{
		SlackrusHook: slackrus.SlackrusHook{
			HookURL:        c.GlobalString("notification-slack-hook-url"),
			Username:       c.GlobalString("notification-slack-identifier"),
			AcceptedLevels: slackrus.LevelThreshold(logLevel),
		},
	}

	log.AddHook(n)

	return n
}

func (s *slackTypeNotifier) StartNotification() {}

func (s *slackTypeNotifier) SendNotification() {}
