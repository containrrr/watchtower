package notifications

import (
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net/smtp"
	"os"
	"strings"
	"time"

	t "github.com/containrrr/watchtower/pkg/types"
	log "github.com/sirupsen/logrus"
	"strconv"
)

const (
	emailType = "email"
)

// Implements Notifier, logrus.Hook
// The default logrus email integration would have several issues:
// - It would send one email per log output
// - It would only send errors
// We work around that by holding on to log entries until the update cycle is done.
type emailTypeNotifier struct {
	From, To                           string
	Server, User, Password, SubjectTag string
	Port                               int
	tlsSkipVerify                      bool
	entries                            []*log.Entry
	logLevels                          []log.Level
	delay                              time.Duration
}

func newEmailNotifier(_ *cobra.Command, acceptedLogLevels []log.Level) t.Notifier {

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
		logLevels:     acceptedLogLevels,
		delay:         time.Duration(delay) * time.Second,
		SubjectTag:    subjecttag,
	}

	log.AddHook(n)

	return n
}

func (e *emailTypeNotifier) buildMessage(entries []*log.Entry) []byte {
	var emailSubject string

	if e.SubjectTag == "" {
		emailSubject = "Watchtower updates"
	} else {
		emailSubject = e.SubjectTag + " Watchtower updates"
	}
	if hostname, err := os.Hostname(); err == nil {
		emailSubject += " on " + hostname
	}
	body := ""
	for _, entry := range entries {
		body += entry.Time.Format("2006-01-02 15:04:05") + " (" + entry.Level.String() + "): " + entry.Message + "\r\n"
		// We don't use fields in watchtower, so don't bother sending them.
	}

	now := time.Now()

	header := make(map[string]string)
	header["From"] = e.From
	header["To"] = e.To
	header["Subject"] = emailSubject
	header["Date"] = now.Format(time.RFC1123Z)
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}

	encodedBody := base64.StdEncoding.EncodeToString([]byte(body))
	//RFC 2045 base64 encoding demands line no longer than 76 characters.
	for _, line := range SplitSubN(encodedBody, 76) {
		message += "\r\n" + line
	}

	return []byte(message)
}

func (e *emailTypeNotifier) sendEntries(entries []*log.Entry) {
	// Do the sending in a separate goroutine so we don't block the main process.
	msg := e.buildMessage(entries)
	go func() {
		if e.delay > 0 {
			time.Sleep(e.delay)
		}

		var auth smtp.Auth
		if e.User != "" {
			auth = smtp.PlainAuth("", e.User, e.Password, e.Server)
		}
		err := SendMail(e.Server+":"+strconv.Itoa(e.Port), e.tlsSkipVerify, auth, e.From, strings.Split(e.To, ","), msg)
		if err != nil {
			// Use fmt so it doesn't trigger another email.
			fmt.Println("Failed to send notification email: ", err)
		}
	}()
}

func (e *emailTypeNotifier) StartNotification() {
	if e.entries == nil {
		e.entries = make([]*log.Entry, 0, 10)
	}
}

func (e *emailTypeNotifier) SendNotification() {
	if e.entries == nil || len(e.entries) <= 0 {
		return
	}

	e.sendEntries(e.entries)
	e.entries = nil
}

func (e *emailTypeNotifier) Levels() []log.Level {
	return e.logLevels
}

func (e *emailTypeNotifier) Fire(entry *log.Entry) error {
	if e.entries != nil {
		e.entries = append(e.entries, entry)
	} else {
		e.sendEntries([]*log.Entry{entry})
	}
	return nil
}

func (e *emailTypeNotifier) Close() {}
