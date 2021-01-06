package notifications

import (
	"os"
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
func NewEmailNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {
	return newEmailNotifier(c, acceptedLogLevels)
}

func newEmailNotifier(c *cobra.Command, acceptedLogLevels []log.Level) t.ConvertableNotifier {
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

func (e *emailTypeNotifier) GetURL() string {
	conf := &shoutrrrSmtp.Config{
		FromAddress: e.From,
		FromName:    "Watchtower",
		ToAddresses: []string{e.To},
		Port:        uint16(e.Port),
		Host:        e.Server,
		Subject:     e.getSubject(),
		Username:    e.User,
		Password:    e.Password,
		UseStartTLS: true,
		UseHTML:     false,
	}

	if len(e.User) > 0 {
		conf.Set("auth", "Plain")
	} else {
		conf.Set("auth", "None")
	}

	return conf.GetURL().String()
}

func (e *emailTypeNotifier) getSubject() string {
	var emailSubject string

	if e.SubjectTag == "" {
		emailSubject = "Watchtower updates"
	} else {
		emailSubject = e.SubjectTag + " Watchtower updates"
	}
	if hostname, err := os.Hostname(); err == nil {
		emailSubject += " on " + hostname
	}
	return emailSubject
}

// TODO: Delete these once all notifiers have been converted to shoutrrr
func (e *emailTypeNotifier) StartNotification()          {}
func (e *emailTypeNotifier) SendNotification()           {}
func (e *emailTypeNotifier) Levels() []log.Level         { return nil }
func (e *emailTypeNotifier) Fire(entry *log.Entry) error { return nil }

func (e *emailTypeNotifier) Close() {}
