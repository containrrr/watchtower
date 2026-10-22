package notifications

import (
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmailTypeNotifier_GetURL_TLSSkipVerify verifies that skipping TLS
// verification keeps the connection encrypted and only skips the certificate
// check.
func TestEmailTypeNotifier_GetURL_TLSSkipVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tlsSkipVerify bool
		// wantSkip is the expected skiptlsverify value, empty when absent.
		wantSkip string
	}{
		{name: "verification on", tlsSkipVerify: false, wantSkip: ""},
		{name: "verification skipped", tlsSkipVerify: true, wantSkip: "Yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := zerolog.Nop()
			notifier := &emailTypeNotifier{
				From:          "watchtower@example.com",
				To:            "admin@example.com",
				Server:        "smtp.example.com",
				Port:          587,
				tlsSkipVerify: tt.tlsSkipVerify,
				log:           &log,
			}

			raw, err := notifier.GetURL(nil)
			require.NoError(t, err)

			parsed, err := url.Parse(raw)
			require.NoError(t, err)

			query := parsed.Query()
			assert.Equal(t, "Auto", query.Get("encryption"))
			assert.Equal(t, "Yes", query.Get("usestarttls"))
			assert.Equal(t, tt.wantSkip, query.Get("skiptlsverify"))
		})
	}
}
