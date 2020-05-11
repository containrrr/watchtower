package notifications

import (
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
		template: getShoutrrrTemplate(cmd),
	}

	entries := []*log.Entry{
		{
			Message: "foo bar",
		},
	}

	s := shoutrrr.buildMessage(entries)

	require.Equal(t, "foo bar\n", s)
}

func TestShoutrrrTemplate(t *testing.T) {
	cmd := new(cobra.Command)
	flags.RegisterNotificationFlags(cmd)
	err := cmd.ParseFlags([]string{"--notification-template={{range .}}{{.Level}}: {{.Message}}{{println}}{{end}}"})

	require.NoError(t, err)

	shoutrrr := &shoutrrrTypeNotifier{
		template: getShoutrrrTemplate(cmd),
	}

	entries := []*log.Entry{
		{
			Level:   log.InfoLevel,
			Message: "foo bar",
		},
	}

	s := shoutrrr.buildMessage(entries)

	require.Equal(t, "info: foo bar\n", s)
}

func TestShoutrrrInvalidTemplateUsesTemplate(t *testing.T) {
	cmd := new(cobra.Command)

	flags.RegisterNotificationFlags(cmd)
	err := cmd.ParseFlags([]string{"--notification-template={{"})

	require.NoError(t, err)

	shoutrrr := &shoutrrrTypeNotifier{
		template: getShoutrrrTemplate(cmd),
	}

	shoutrrrDefault := &shoutrrrTypeNotifier{
		template: template.Must(template.New("").Parse(shoutrrrDefaultTemplate)),
	}

	entries := []*log.Entry{
		{
			Message: "foo bar",
		},
	}

	s := shoutrrr.buildMessage(entries)
	sd := shoutrrrDefault.buildMessage(entries)

	require.Equal(t, sd, s)
}
