// Package logging provides zerolog construction and configuration helpers for Watchtower.
//
// This package holds no logger state. It does not expose a global logger, Global
// accessor, InitLogger, or init-based setup. Callers construct a *zerolog.Logger
// via New and related helpers and pass it explicitly to subsystems.
package logging

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog"
)

// ErrUnknownLogLevel is returned by ParseLevel for unrecognized level strings.
var ErrUnknownLogLevel = errors.New("unknown log level")

// Level maps to zerolog levels.
type Level uint8

// Logging levels ordered from most to least verbose.
const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// toZerologLevel converts a logging.Level to zerolog.Level.
//
// Parameters:
//   - level: Application logging level constant.
//
// Returns:
//   - zerolog.Level: Corresponding zerolog level, or InfoLevel for unknown values.
func toZerologLevel(level Level) zerolog.Level {
	switch level {
	case TraceLevel:
		return zerolog.TraceLevel
	case DebugLevel:
		return zerolog.DebugLevel
	case InfoLevel:
		return zerolog.InfoLevel
	case WarnLevel:
		return zerolog.WarnLevel
	case ErrorLevel:
		return zerolog.ErrorLevel
	case FatalLevel:
		return zerolog.FatalLevel
	case PanicLevel:
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// New creates a *zerolog.Logger configured with the given writer and level.
//
// The logger always includes timestamps on events.
//
// Parameters:
//   - w: Destination for log output.
//   - level: Minimum level of events to emit.
//
// Returns:
//   - *zerolog.Logger: Configured logger instance.
func New(w io.Writer, level Level) *zerolog.Logger {
	l := zerolog.New(w).Level(toZerologLevel(level)).With().Timestamp().Logger()

	return &l
}

// ParseLevel maps a CLI level string to zerolog.Level.
//
// Accepted values (case-insensitive) are panic, fatal, error, warn, warning,
// info, debug, and trace.
//
// Parameters:
//   - level: Level name string (may include surrounding whitespace).
//
// Returns:
//   - zerolog.Level: Parsed level, or NoLevel when invalid.
//   - error: Non-nil when the level string is not recognized.
func ParseLevel(level string) (zerolog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "panic":
		return zerolog.PanicLevel, nil
	case "fatal":
		return zerolog.FatalLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "debug":
		return zerolog.DebugLevel, nil
	case "trace":
		return zerolog.TraceLevel, nil
	default:
		return zerolog.NoLevel, fmt.Errorf("%w: %q", ErrUnknownLogLevel, level)
	}
}
