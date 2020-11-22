package notifications

import (
	"net/url"
	"strings"

	shoutrrrGotify "github.com/containrrr/shoutrrr/pkg/services/gotify"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
func NewGotifyNotifier() t.ConvertibleNotifier {
	return newGotifyNotifier()
}

func newGotifyNotifier() t.ConvertibleNotifier {
	apiURL := getGotifyURL()
	token := getGotifyToken()

	skipVerify := viper.GetBool("notification-gotify-tls-skip-verify")

	n := &gotifyTypeNotifier{
		gotifyURL:                apiURL,
		gotifyAppToken:           token,
		gotifyInsecureSkipVerify: skipVerify,
	}

	return n
}

func getGotifyToken() string {
	gotifyToken := viper.GetString("notification-gotify-token")
	if len(gotifyToken) < 1 {
		log.Fatal("Required argument --notification-gotify-token(cli) or WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN(env) is empty.")
	}
	return gotifyToken
}

func getGotifyURL() string {
	gotifyURL := viper.GetString("notification-gotify-url")

	if len(gotifyURL) < 1 {
		log.Fatal("Required argument --notification-gotify-url(cli) or WATCHTOWER_NOTIFICATION_GOTIFY_URL(env) is empty.")
	} else if !(strings.HasPrefix(gotifyURL, "http://") || strings.HasPrefix(gotifyURL, "https://")) {
		log.Fatal("Gotify URL must start with \"http://\" or \"https://\"")
	} else if strings.HasPrefix(gotifyURL, "http://") {
		log.Warn("Using an HTTP url for Gotify is insecure")
	}

	return gotifyURL
}

func (n *gotifyTypeNotifier) GetURL() (string, error) {
	apiURL, err := url.Parse(n.gotifyURL)
	if err != nil {
		return "", err
	}

	config := &shoutrrrGotify.Config{
		Host:       apiURL.Host,
		Path:       apiURL.Path,
		DisableTLS: apiURL.Scheme == "http",
		Title:      GetTitle(),
		Token:      n.gotifyAppToken,
	}

	return config.GetURL().String(), nil
}
