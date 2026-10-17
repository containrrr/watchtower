package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

// errInvalidLogFormat is returned by ConfigureWriter for unrecognized format names.
var errInvalidLogFormat = errors.New("invalid log format")

// ConfigureWriter returns an io.Writer for the requested log format.
//
// Supported formats (case-insensitive):
//   - json: raw os.Stderr (zerolog default JSON encoding)
//   - pretty: zerolog.ConsoleWriter with colors unless noColor is true
//   - logfmt: ConsoleWriter with NoColor and key=value-style formatting
//   - auto: pretty when stderr is a TTY, NO_COLOR is not present in the
//     environment (presence includes empty value), and noColor is false.
//     Otherwise logfmt is used.
//
// Parameters:
//   - format: Requested format name (json, pretty, logfmt, or auto).
//   - noColor: When true, disables colorized pretty output.
//
// Returns:
//   - io.Writer: Writer suitable for zerolog.New.
//   - error: Non-nil when format is not recognized.
func ConfigureWriter(format string, noColor bool) (io.Writer, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return os.Stderr, nil
	case "pretty":
		return newPrettyWriter(noColor), nil
	case "logfmt":
		return newLogfmtWriter(), nil
	case "auto":
		if usePrettyAuto(noColor) {
			return newPrettyWriter(false), nil
		}

		return newLogfmtWriter(), nil
	default:
		return nil, fmt.Errorf("%w: %q", errInvalidLogFormat, format)
	}
}

// usePrettyAuto reports whether auto format should use colorized pretty output.
//
// NO_COLOR follows this repo's presence semantics (any set value, including empty
// string), matching internal/flags ApplyEnvToFlags rather than only non-empty values.
//
// Parameters:
//   - noColor: Explicit --no-color flag value.
//
// Returns:
//   - bool: True when pretty colored output should be used for auto format.
func usePrettyAuto(noColor bool) bool {
	if noColor {
		return false
	}

	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	fd := os.Stderr.Fd()

	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// newPrettyWriter builds a human-friendly ConsoleWriter.
//
// Parameters:
//   - noColor: When true, disables ANSI colors.
//
// Returns:
//   - zerolog.ConsoleWriter: Pretty console writer targeting stderr.
func newPrettyWriter(noColor bool) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    noColor,
		TimeFormat: time.Kitchen,
	}
}

// newLogfmtWriter builds a ConsoleWriter that emits key=value logfmt-style lines.
//
// Returns:
//   - zerolog.ConsoleWriter: Logfmt-style console writer targeting stderr.
func newLogfmtWriter() zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    true,
		TimeFormat: time.RFC3339,
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			zerolog.MessageFieldName,
		},
		FormatTimestamp: func(i any) string {
			if i == nil {
				return "time="
			}

			return "time=" + fmt.Sprint(i)
		},
		FormatLevel: func(i any) string {
			if i == nil {
				return "level="
			}

			return "level=" + strings.ToLower(fmt.Sprint(i))
		},
		FormatMessage: func(i any) string {
			if i == nil {
				return "msg="
			}

			s := fmt.Sprint(i)
			if needsQuote(s) {
				return "msg=" + fmt.Sprintf("%q", s)
			}

			return "msg=" + s
		},
		FormatFieldName: func(i any) string {
			return fmt.Sprint(i) + "="
		},
		// FormatFieldValue must preserve ConsoleWriter pre-formatting. Strings may
		// already be strconv.Quoted when they need quotes. Non-string values (bools,
		// slices, maps) are JSON-marshaled to []byte before this runs. See zerolog
		// ConsoleWriter.writeFields. Render []byte as text so true prints as true
		// rather than as a decimal byte slice.
		FormatFieldValue: formatLogfmtFieldValue,
	}
}

// formatLogfmtFieldValue renders a ConsoleWriter field value for logfmt output.
//
// Parameters:
//   - i: Field value from ConsoleWriter (string, json.Number, or JSON []byte).
//
// Returns:
//   - string: Printable field value without additional quoting.
func formatLogfmtFieldValue(i any) string {
	switch v := i.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		// Match zerolog consoleDefaultFormatFieldValue for remaining types.
		return fmt.Sprintf("%s", i)
	}
}

// needsQuote reports whether a logfmt value should be quoted.
//
// Parameters:
//   - s: Field or message value.
//
// Returns:
//   - bool: True when the value contains spaces, equals signs, quotes, is empty, or contains Unicode whitespace or control characters.
func needsQuote(s string) bool {
	if s == "" {
		return true
	}

	for _, r := range s {
		if r == ' ' || r == '=' || r == '"' || unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}

	return false
}
