package notifications

import (
	"strings"

	shoutrrrGotify "github.com/containrrr/shoutrrr/pkg/services/gotify"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	gotifyType = "gotify"
)

type gotifyTypeNotifier struct {
	gotifyURL                string
	gotifyAppToken           string
	gotifyInsecureSkipVerify bool
	logLevels                []log.Level
}

// NewGotifyNotifier is a factory method creating a new gotify notifier instance
func NewGotifyNotifier(c *cobra.Command, levels []log.Level) t.ConvertableNotifier {
	return newGotifyNotifier(c, levels)
}

func newGotifyNotifier(c *cobra.Command, levels []log.Level) t.ConvertableNotifier {
	flags := c.PersistentFlags()

	url := getGotifyURL(flags)
	token := getGotifyToken(flags)

	skipVerify, _ := flags.GetBool("notification-gotify-tls-skip-verify")

	n := &gotifyTypeNotifier{
		gotifyURL:                url,
		gotifyAppToken:           token,
		gotifyInsecureSkipVerify: skipVerify,
		logLevels:                levels,
	}

	return n
}

func getGotifyToken(flags *pflag.FlagSet) string {
	gotifyToken, _ := flags.GetString("notification-gotify-token")
	if len(gotifyToken) < 1 {
		log.Fatal("Required argument --notification-gotify-token(cli) or WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN(env) is empty.")
	}
	return gotifyToken
}

func getGotifyURL(flags *pflag.FlagSet) string {
	gotifyURL, _ := flags.GetString("notification-gotify-url")

	if len(gotifyURL) < 1 {
		log.Fatal("Required argument --notification-gotify-url(cli) or WATCHTOWER_NOTIFICATION_GOTIFY_URL(env) is empty.")
	} else if !(strings.HasPrefix(gotifyURL, "http://") || strings.HasPrefix(gotifyURL, "https://")) {
		log.Fatal("Gotify URL must start with \"http://\" or \"https://\"")
	} else if strings.HasPrefix(gotifyURL, "http://") {
		log.Warn("Using an HTTP url for Gotify is insecure")
	}

	return gotifyURL
}

func (n *gotifyTypeNotifier) GetURL() string {
	url := n.gotifyURL

	if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	} else {
		url = strings.TrimPrefix(url, "http://")
	}

	url = strings.TrimSuffix(url, "/")

	config := &shoutrrrGotify.Config{
		Host:  url,
		Token: n.gotifyAppToken,
	}

	return config.GetURL().String()
}

func (n *gotifyTypeNotifier) StartNotification()  {}
func (n *gotifyTypeNotifier) SendNotification()   {}
func (n *gotifyTypeNotifier) Close() {}
func (n *gotifyTypeNotifier) Levels() []log.Level { return nil }
