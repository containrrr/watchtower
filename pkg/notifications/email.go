package notifications

import (
	"time"

	"github.com/spf13/cobra"

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
func NewEmailNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertibleNotifier {
	return newEmailNotifier(c, acceptedLogLevels)
}

func newEmailNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertibleNotifier {
	flags := c.PersistentFlags()

	from, _ := flags.GetString("notification-email-from")
	to, _ := flags.GetString("notification-email-to")
	server, _ := flags.GetString("notification-email-server")
	user, _ := flags.GetString("notification-email-server-user")
	password, _ := flags.GetString("notification-email-server-password")
	port, _ := flags.GetInt("notification-email-server-port")
	tlsSkipVerify, _ := flags.GetBool("notification-email-server-tls-skip-verify")
	delay, _ := flags.GetInt("notification-email-delay")
	subjecttag, _ := flags.GetString("notification-email-subjecttag")

	n := &emailTypeNotifier{
		entries:       []*log.Entry{},
		From:          from,
		To:            to,
		Server:        server,
		User:          user,
		Password:      password,
		Port:          port,
		tlsSkipVerify: tlsSkipVerify,
		logLevels:     acceptedLogLevels,
		delay:         time.Duration(delay) * time.Second,
		SubjectTag:    subjecttag,
	}

	return n
}

func (e *emailTypeNotifier) GetURL(c *cobra.Command) (string, error) {
	conf := &shoutrrrSmtp.Config{
		FromAddress: e.From,
		FromName:    "Watchtower",
		ToAddresses: []string{e.To},
		Port:        uint16(e.Port),
		Host:        e.Server,
		Subject:     e.getSubject(c),
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

func (e *emailTypeNotifier) getSubject(c *cobra.Command) string {
	subject := GetTitle(c)

	if e.SubjectTag != "" {
		subject = e.SubjectTag + " " + subject
	}

	return subject
}
