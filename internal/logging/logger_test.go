package logging

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew verifies that a new logger writes JSON events with a level, message,
// and time, and drops events below its level.
func TestNew(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	log := New(buf, InfoLevel)
	require.NotNil(t, log)

	log.Info().Msg("hello")
	log.Debug().Msg("hidden")

	var event map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &event))
	assert.Equal(t, "info", event["level"])
	assert.Equal(t, "hello", event["message"])
	assert.Contains(t, event, "time")
	assert.NotContains(t, buf.String(), "hidden")
}

// TestNew_LevelFilter verifies that a logger at warn level drops info events.
func TestNew_LevelFilter(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	log := New(buf, WarnLevel)

	log.Info().Msg("info")
	log.Warn().Msg("warn")

	assert.NotContains(t, buf.String(), `"info"`)
	assert.Contains(t, buf.String(), "warn")
}

// TestParseLevel verifies that level names are parsed case-insensitively and
// trimmed, with warning accepted for warn, and that any other name is an error.
func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    zerolog.Level
		wantErr bool
	}{
		{"panic", zerolog.PanicLevel, false},
		{"fatal", zerolog.FatalLevel, false},
		{"error", zerolog.ErrorLevel, false},
		{"warn", zerolog.WarnLevel, false},
		{"warning", zerolog.WarnLevel, false},
		{"WARNING", zerolog.WarnLevel, false},
		{"info", zerolog.InfoLevel, false},
		{"debug", zerolog.DebugLevel, false},
		{"trace", zerolog.TraceLevel, false},
		{" Trace ", zerolog.TraceLevel, false},
		{"", zerolog.NoLevel, true},
		{"verbose", zerolog.NoLevel, true},
		{"invalid", zerolog.NoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := ParseLevel(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, errUnknownLogLevel)
				assert.Equal(t, tt.want, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
