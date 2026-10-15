package notifications

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSlackTypeNotifier_GetURL_Channel verifies that a legacy Slack channel
// given as a Slack channel ID reaches the generated URL, and that any other
// channel keeps the webhook's own channel and is reported with a warning,
// since a shoutrrr Slack URL can only hold a channel ID.
func TestSlackTypeNotifier_GetURL_Channel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		channel string
		// wantChannel is the channel in the generated URL.
		wantChannel string
		wantWarning bool
	}{
		{name: "no channel", wantChannel: "webhook"},
		{name: "public channel ID", channel: "C8UJ12P4P", wantChannel: "C8UJ12P4P"},
		{name: "private channel ID", channel: "G0987654321", wantChannel: "G0987654321"},
		{name: "direct message ID", channel: "D024BE91L", wantChannel: "D024BE91L"},
		{name: "channel name", channel: "#ops", wantChannel: "webhook", wantWarning: true},
		{name: "bare channel name", channel: "ops", wantChannel: "webhook", wantWarning: true},
		{name: "user", channel: "@alice", wantChannel: "webhook", wantWarning: true},
		{name: "short uppercase word", channel: "DEV", wantChannel: "webhook", wantWarning: true},
		{name: "lowercase ID-like name", channel: "c8uj12p4p", wantChannel: "webhook", wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			log := zerolog.New(&out).Level(zerolog.WarnLevel)
			notifier := &slackTypeNotifier{
				HookURL: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX",
				Channel: tt.channel,
				log:     &log,
			}

			raw, err := notifier.GetURL(nil)
			require.NoError(t, err)

			parsed, err := url.Parse(raw)
			require.NoError(t, err)

			assert.Equal(t, tt.wantChannel, parsed.Host)

			if tt.wantWarning {
				assert.Contains(t, out.String(), slackChannelNotConvertedMessage)
				assert.Contains(t, out.String(), tt.channel)
			} else {
				assert.Empty(t, out.String())
			}
		})
	}
}
