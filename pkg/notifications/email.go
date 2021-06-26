package notifications

import (
	"github.com/spf13/viper"
	"time"

	shoutrrrSmtp "github.com/containrrr/shoutrrr/pkg/services/smtp"
	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
)

const (
	emailType = "email"
)

type emailTypeNotifier struct {
	url                                string
	From, To                           string
	Server, User, Password, SubjectTag string
	Port                               int
	tlsSkipVerify                      bool
	entries                            []*log.Entry
	logLevels                          []log.Level
	delay                              time.Duration
}

func newEmailNotifier() t.ConvertibleNotifier {
	from := viper.GetString("notification-email-from")
	to := viper.GetString("notification-email-to")
	server := viper.GetString("notification-email-server")
	user := viper.GetString("notification-email-server-user")
	password := viper.GetString("notification-email-server-password")
	port := viper.GetInt("notification-email-server-port")
	tlsSkipVerify := viper.GetBool("notification-email-server-tls-skip-verify")
	delay := viper.GetInt("notification-email-delay")
	subjecttag := viper.GetString("notification-email-subjecttag")

	n := &emailTypeNotifier{
		entries:       []*log.Entry{},
		From:          from,
		To:            to,
		Server:        server,
		User:          user,
		Password:      password,
		Port:          port,
		tlsSkipVerify: tlsSkipVerify,
		delay:         time.Duration(delay) * time.Second,
		SubjectTag:    subjecttag,
	}

	return n
}

func (e *emailTypeNotifier) GetURL(title string) (string, error) {
	conf := &shoutrrrSmtp.Config{
		FromAddress: e.From,
		FromName:    "Watchtower",
		ToAddresses: []string{e.To},
		Port:        uint16(e.Port),
		Host:        e.Server,
		Subject:     e.getSubject(title),
		Username:    e.User,
		Password:    e.Password,
		UseStartTLS: !e.tlsSkipVerify,
		UseHTML:     false,
		Encryption:  shoutrrrSmtp.EncMethods.Auto,
		Auth:        shoutrrrSmtp.AuthTypes.None,
	}

	if len(e.User) > 0 {
		conf.Auth = shoutrrrSmtp.AuthTypes.Plain
	}

	if e.tlsSkipVerify {
		conf.Encryption = shoutrrrSmtp.EncMethods.None
	}

	return conf.GetURL().String(), nil
}

func (e *emailTypeNotifier) GetDelay() time.Duration {
	return e.delay
}

func (e *emailTypeNotifier) getSubject(title string) string {
	if e.SubjectTag != "" {
		return e.SubjectTag + " " + title
	}

	return title
}
