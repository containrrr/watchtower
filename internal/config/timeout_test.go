package config

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/flags"
)

// TestStopTimeout_LegacyBareNumberAsSeconds verifies bare numeric WATCHTOWER_TIMEOUT
// values are interpreted as seconds at Load time.
func TestStopTimeout_LegacyBareNumberAsSeconds(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		expected  time.Duration
		wantError bool
	}{
		{name: "default (no env) is 30s", envValue: "", expected: 30 * time.Second},
		{name: "bare integer as seconds", envValue: "60", expected: 60 * time.Second},
		{name: "larger bare integer", envValue: "300", expected: 300 * time.Second},
		{name: "bare float seconds", envValue: "1.5", expected: 1500 * time.Millisecond},
		{name: "with explicit unit s", envValue: "45s", expected: 45 * time.Second},
		{name: "with unit m", envValue: "2m", expected: 2 * time.Minute},
		{name: "zero value", envValue: "0", expected: 0},
		{name: "negative rejected", envValue: "-10", wantError: true},
		{name: "invalid non-numeric falls back to default", envValue: "abc", expected: 30 * time.Second},
		{name: "invalid multi-decimal falls back to default", envValue: "12.34.56", expected: 30 * time.Second},
		{name: "positive sign prefix", envValue: "+10", expected: 10 * time.Second},
		{name: "whitespace trimmed", envValue: "  30  ", expected: 30 * time.Second},
		{
			name:     "unparseable huge integer falls back to default",
			envValue: "1" + strings.Repeat("0", 1000),
			expected: 30 * time.Second,
		},
		{
			name:     "overflow clamps to max int64",
			envValue: "9223372038",
			expected: time.Duration(math.MaxInt64),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				t.Setenv("WATCHTOWER_TIMEOUT", tc.envValue)
			} else {
				_ = os.Unsetenv("WATCHTOWER_TIMEOUT")
			}

			cmd := &cobra.Command{}
			flags.RegisterAll(cmd)
			require.NoError(t, cmd.ParseFlags([]string{}))

			cfg, err := Load(newTestLogger(t), cmd, nil)
			if tc.wantError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.Update.StopTimeout)
		})
	}
}
