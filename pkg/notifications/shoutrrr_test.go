package notifications

import (
	"github.com/containrrr/shoutrrr/pkg/types"
	"testing"
	"text/template"

	"github.com/containrrr/watchtower/internal/flags"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestShoutrrrDefaultTemplate(t *testing.T) {
	cmd := new(cobra.Command)

	shoutrrr := &shoutrrrTypeNotifier{
		template:       getShoutrrrTemplate(cmd, true),
		legacyTemplate: true,
	}

	entries := []*log.Entry{
		{
			Message: "foo bar",
		},
	}

	s := shoutrrr.buildMessage(Data{Entries: entries})

	require.Equal(t, "foo bar\n", s)
}

func TestShoutrrrTemplate(t *testing.T) {
	cmd := new(cobra.Command)
	flags.RegisterNotificationFlags(cmd)
	err := cmd.ParseFlags([]string{"--notification-template={{range .}}{{.Level}}: {{.Message}}{{println}}{{end}}"})

	require.NoError(t, err)

	shoutrrr := &shoutrrrTypeNotifier{
		template:       getShoutrrrTemplate(cmd, true),
		legacyTemplate: true,
	}

	entries := []*log.Entry{
		{
			Level:   log.InfoLevel,
			Message: "foo bar",
		},
	}

	s := shoutrrr.buildMessage(Data{Entries: entries})

	require.Equal(t, "info: foo bar\n", s)
}

func TestShoutrrrStringFunctions(t *testing.T) {
	cmd := new(cobra.Command)
	flags.RegisterNotificationFlags(cmd)
	err := cmd.ParseFlags([]string{"--notification-template={{range .}}{{.Level | printf \"%v\" | ToUpper }}: {{.Message | ToLower }} {{.Message | Title }}{{println}}{{end}}"})

	require.NoError(t, err)

	shoutrrr := &shoutrrrTypeNotifier{
		template:       getShoutrrrTemplate(cmd, true),
		legacyTemplate: true,
	}

	entries := []*log.Entry{
		{
			Level:   log.InfoLevel,
			Message: "foo Bar",
		},
	}

	s := shoutrrr.buildMessage(Data{Entries: entries})

	require.Equal(t, "INFO: foo bar Foo Bar\n", s)
}

func TestShoutrrrInvalidTemplateUsesTemplate(t *testing.T) {
	cmd := new(cobra.Command)

	flags.RegisterNotificationFlags(cmd)
	err := cmd.ParseFlags([]string{"--notification-template={{"})

	require.NoError(t, err)

	shoutrrr := &shoutrrrTypeNotifier{
		template: getShoutrrrTemplate(cmd, true),
	}

	shoutrrrDefault := &shoutrrrTypeNotifier{
		template: template.Must(template.New("").Parse(shoutrrrDefaultLegacyTemplate)),
	}

	entries := []*log.Entry{
		{
			Message: "foo bar",
		},
	}
	data := Data{Entries: entries}

	s := shoutrrr.buildMessage(data)
	sd := shoutrrrDefault.buildMessage(data)

	require.Equal(t, sd, s)
}

type blockingRouter struct {
	unlock chan bool
	sent   chan bool
}

func (b blockingRouter) Send(_ string, _ *types.Params) []error {
	_ = <-b.unlock
	b.sent <- true
	return nil
}

func TestSlowNotificationNotSent(t *testing.T) {
	_, blockingRouter := sendNotificationsWithBlockingRouter()

	notifSent := false
	select {
	case notifSent = <-blockingRouter.sent:
	default:
	}

	require.Equal(t, false, notifSent)
}

func TestSlowNotificationSent(t *testing.T) {
	shoutrrr, blockingRouter := sendNotificationsWithBlockingRouter()

	blockingRouter.unlock <- true
	shoutrrr.Close()

	notifSent := false
	select {
	case notifSent = <-blockingRouter.sent:
	default:
	}
	require.Equal(t, true, notifSent)
}

func sendNotificationsWithBlockingRouter() (*shoutrrrTypeNotifier, *blockingRouter) {
	cmd := new(cobra.Command)

	router := &blockingRouter{
		unlock: make(chan bool, 1),
		sent:   make(chan bool, 1),
	}

	shoutrrr := &shoutrrrTypeNotifier{
		template: getShoutrrrTemplate(cmd, true),
		messages: make(chan string, 1),
		done:     make(chan bool),
		Router:   router,
	}

	entry := &log.Entry{
		Message: "foo bar",
	}

	go sendNotifications(shoutrrr)

	shoutrrr.StartNotification()
	_ = shoutrrr.Fire(entry)

	shoutrrr.SendNotification(nil)

	return shoutrrr, router
}
