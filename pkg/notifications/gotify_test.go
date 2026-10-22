package notifications

import (
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGotifyTypeNotifier_GetURL_TLSSkipVerify verifies that skipping TLS
// verification reaches the generated Gotify URL.
func TestGotifyTypeNotifier_GetURL_TLSSkipVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		skipVerify bool
		// wantSkip is the expected insecureskipverify value, empty when absent.
		wantSkip string
	}{
		{name: "verification on", skipVerify: false, wantSkip: ""},
		{name: "verification skipped", skipVerify: true, wantSkip: "Yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := zerolog.Nop()
			notifier := &gotifyTypeNotifier{
				gotifyURL:                "https://gotify.example.com",
				gotifyAppToken:           "Aaaaaaaaaaaaaaa",
				gotifyInsecureSkipVerify: tt.skipVerify,
				log:                      &log,
			}

			raw, err := notifier.GetURL(nil)
			require.NoError(t, err)

			parsed, err := url.Parse(raw)
			require.NoError(t, err)

			assert.Equal(t, tt.wantSkip, parsed.Query().Get("insecureskipverify"))
			assert.Equal(t, "gotify.example.com", parsed.Host)
		})
	}
}
