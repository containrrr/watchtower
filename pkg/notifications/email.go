package notifications

import (
	"time"

	"github.com/spf13/viper"

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

// NewEmailNotifier is a factory method creating a new email notifier instance
func NewEmailNotifier() t.ConvertibleNotifier {
	return newEmailNotifier()
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

func (e *emailTypeNotifier) GetURL() (string, error) {
	conf := &shoutrrrSmtp.Config{
		FromAddress: e.From,
		FromName:    "Watchtower",
		ToAddresses: []string{e.To},
		Port:        uint16(e.Port),
		Host:        e.Server,
		Subject:     e.getSubject(),
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

func (e *emailTypeNotifier) getSubject() string {
	subject := GetTitle()

	if e.SubjectTag != "" {
		subject = e.SubjectTag + " " + subject
	}

	return subject
}
