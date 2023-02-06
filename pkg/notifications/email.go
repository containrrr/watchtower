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
	From, To               string
	Server, User, Password string
	Port                   int
	tlsSkipVerify          bool
	entries                []*log.Entry
	delay                  time.Duration
}

func newEmailNotifier(c *cobra.Command) t.ConvertibleNotifier {
	flags := c.Flags()

	from, _ := flags.GetString("notification-email-from")
	to, _ := flags.GetString("notification-email-to")
	server, _ := flags.GetString("notification-email-server")
	user, _ := flags.GetString("notification-email-server-user")
	password, _ := flags.GetString("notification-email-server-password")
	port, _ := flags.GetInt("notification-email-server-port")
	tlsSkipVerify, _ := flags.GetBool("notification-email-server-tls-skip-verify")
	delay, _ := flags.GetInt("notification-email-delay")

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
		Username:    e.User,
		Password:    e.Password,
		UseStartTLS: !e.tlsSkipVerify,
		UseHTML:     false,
		Encryption:  shoutrrrSmtp.EncMethods.Auto,
		Auth:        shoutrrrSmtp.AuthTypes.None,
		ClientHost:  "localhost",
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
