package logging

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigureWriter_JSON verifies that the json format writes events to
// stderr unchanged.
func TestConfigureWriter_JSON(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("json", false)
	require.NoError(t, err)
	assert.Equal(t, os.Stderr, w)
}

// TestConfigureWriter_Pretty verifies that the pretty format writes colored
// console output to stderr unless color is turned off.
func TestConfigureWriter_Pretty(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("pretty", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)
	assert.False(t, cw.NoColor)
	assert.Equal(t, os.Stderr, cw.Out)

	wNoColor, err := ConfigureWriter("pretty", true)
	require.NoError(t, err)

	cwNoColor, ok := wNoColor.(zerolog.ConsoleWriter)
	require.True(t, ok)
	assert.True(t, cwNoColor.NoColor)
}

// TestConfigureWriter_Logfmt verifies that the logfmt format writes uncolored
// key=value output to stderr, with the time, level, and message first.
func TestConfigureWriter_Logfmt(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("logfmt", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)
	assert.True(t, cw.NoColor)
	assert.Equal(t, os.Stderr, cw.Out)
	assert.NotNil(t, cw.FormatTimestamp)
	assert.NotNil(t, cw.FormatLevel)
	assert.NotNil(t, cw.FormatMessage)
	assert.NotNil(t, cw.FormatFieldName)
	assert.Equal(t, []string{
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		zerolog.MessageFieldName,
	}, cw.PartsOrder)

	// Custom formatters emit key=value style fragments.
	assert.Contains(t, cw.FormatLevel("info"), "level=")
	assert.Contains(t, cw.FormatMessage("hello world"), "msg=")
	assert.Contains(t, cw.FormatFieldName("container"), "container=")
}

// TestConfigureWriter_LogfmtWriteThrough writes a real event through the logfmt
// ConsoleWriter and asserts single-layer quoting for spaced messages and fields.
func TestConfigureWriter_LogfmtWriteThrough(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("logfmt", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)

	buf := &bytes.Buffer{}
	cw.Out = buf

	log := zerolog.New(cw).Level(zerolog.InfoLevel).With().Timestamp().Logger()
	log.Info().
		Str("container", "nginx with spaces").
		Msg("hello world")

	line := strings.TrimSpace(buf.String())
	t.Logf("logfmt line: %s", line)

	assert.Contains(t, line, "level=info")
	// Message with spaces: single pair of quotes via FormatMessage.
	assert.Contains(t, line, `msg="hello world"`)
	// Field with spaces: ConsoleWriter pre-quotes once. FormatFieldValue must not re-quote.
	assert.Contains(t, line, `container="nginx with spaces"`)
	assert.NotContains(t, line, `\"nginx with spaces\"`)
	assert.NotContains(t, line, `container="\"nginx`)
}

// TestConfigureWriter_LogfmtTypedFields verifies that bools and slices render as
// readable text instead of Go's default decimal byte-slice form.
func TestConfigureWriter_LogfmtTypedFields(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("logfmt", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)

	buf := &bytes.Buffer{}
	cw.Out = buf

	log := zerolog.New(cw).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	log.Debug().
		Bool("legacy_template", true).
		Bool("enable_label", false).
		Strs("names", []string{"watchtower-test"}).
		Strs("empty_names", nil).
		Int("entries_count", 3).
		Msg("typed fields")

	line := strings.TrimSpace(buf.String())
	t.Logf("logfmt line: %s", line)

	assert.Contains(t, line, "legacy_template=true")
	assert.Contains(t, line, "enable_label=false")
	assert.Contains(t, line, `names=["watchtower-test"]`)
	assert.Contains(t, line, "entries_count=3")
	// fmt.Sprint on JSON []byte used to emit decimal slices like [116 114 117 101].
	assert.NotContains(t, line, "[116 114 117 101]")
	assert.NotContains(t, line, "[102 97 108 115 101]")
	assert.NotContains(t, line, "[91 34")
}

// TestConfigureWriter_Auto_NoColorEnv verifies that the auto format falls back
// to logfmt when NO_COLOR is set.
//
// The test sets an environment variable, so it does not run in parallel.
func TestConfigureWriter_Auto_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	w, err := ConfigureWriter("auto", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)
	// auto falls back to logfmt when NO_COLOR is set.
	assert.True(t, cw.NoColor)
	assert.NotNil(t, cw.FormatTimestamp)
}

// TestConfigureWriter_Auto_EmptyNoColorEnv verifies that the auto format falls
// back to logfmt when NO_COLOR is present, even when it is empty.
//
// The test sets an environment variable, so it does not run in parallel.
func TestConfigureWriter_Auto_EmptyNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	w, err := ConfigureWriter("auto", false)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)
	assert.True(t, cw.NoColor)
	assert.NotNil(t, cw.FormatTimestamp)
}

// TestConfigureWriter_Auto_NoColorFlag verifies that the auto format writes
// uncolored output when color is turned off.
func TestConfigureWriter_Auto_NoColorFlag(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("auto", true)
	require.NoError(t, err)

	cw, ok := w.(zerolog.ConsoleWriter)
	require.True(t, ok)
	assert.True(t, cw.NoColor)
}

// TestConfigureWriter_CaseInsensitive verifies that format names are matched
// case-insensitively.
func TestConfigureWriter_CaseInsensitive(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("JSON", false)
	require.NoError(t, err)
	assert.Equal(t, os.Stderr, w)
}

// TestConfigureWriter_Invalid verifies that an unknown format is an invalid
// log format error.
func TestConfigureWriter_Invalid(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("cowsay", false)
	require.ErrorIs(t, err, errInvalidLogFormat)
	assert.Nil(t, w)
	assert.Contains(t, err.Error(), `"cowsay"`)
}

// TestConfigureWriter_Empty verifies that an empty format is an invalid log
// format error.
func TestConfigureWriter_Empty(t *testing.T) {
	t.Parallel()

	w, err := ConfigureWriter("", false)
	require.ErrorIs(t, err, errInvalidLogFormat)
	assert.Nil(t, w)
	assert.Contains(t, err.Error(), `""`)
}
