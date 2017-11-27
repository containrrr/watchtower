package notifications

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	emailType = "email"
)

// Implements typeNotifier, logrus.Hook
// The default logrus email integration would have several issues:
// - It would send one email per log output
// - It would only send errors
// We work around that by holding on to log entries until the update cycle is done.
type emailTypeNotifier struct {
	From, To               string
	Server, User, Password string
	entries                []*log.Entry
}

func newEmailNotifier(c *cli.Context) typeNotifier {
	n := &emailTypeNotifier{
		From:     c.GlobalString("notification-email-from"),
		To:       c.GlobalString("notification-email-to"),
		Server:   c.GlobalString("notification-email-server"),
		User:     c.GlobalString("notification-email-server-user"),
		Password: c.GlobalString("notification-email-server-password"),
	}

	log.AddHook(n)

	return n
}

func (e *emailTypeNotifier) buildMessage(entries []*log.Entry) []byte {
	emailSubject := "Watchtower updates"
	if hostname, err := os.Hostname(); err == nil {
		emailSubject += " on " + hostname
	}
	body := ""
	for _, entry := range entries {
		body += entry.Time.Format("2006-01-02 15:04:05") + " (" + entry.Level.String() + "): " + entry.Message + "\r\n"
		// We don't use fields in watchtower, so don't bother sending them.
	}

	header := make(map[string]string)
	header["From"] = e.From
	header["To"] = e.To
	header["Subject"] = emailSubject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	return []byte(message)
}

func (e *emailTypeNotifier) sendEntries(entries []*log.Entry) {
	// Do the sending in a separate goroutine so we don't block the main process.
	msg := e.buildMessage(entries)
	go func() {
		auth := smtp.PlainAuth("", e.User, e.Password, e.Server)
		err := smtp.SendMail(e.Server+":25", auth, e.From, []string{e.To}, msg)
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
	if e.entries != nil {
		e.sendEntries(e.entries)
		e.entries = nil
	}
}

func (e *emailTypeNotifier) Levels() []log.Level {
	// TODO: Make this configurable.
	return []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	}
}

func (e *emailTypeNotifier) Fire(entry *log.Entry) error {
	if e.entries != nil {
		e.entries = append(e.entries, entry)
	} else {
		// Log output generated outside a cycle is sent immediately.
		e.sendEntries([]*log.Entry{entry})
	}
	return nil
}
