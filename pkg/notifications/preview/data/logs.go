package data

import (
	"time"
)

type logEntry struct {
	Message string
	Data    map[string]any
	Time    time.Time
	Level   LogLevel
}

// LogLevel is the analog of logrus.Level
type LogLevel string

const (
	TraceLevel LogLevel = "trace"
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warning"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
	PanicLevel LogLevel = "panic"
)

// LevelsFromString parses a string of level characters and returns a slice of the corresponding log levels
func LevelsFromString(str string) []LogLevel {
	levels := make([]LogLevel, 0, len(str))
	for _, c := range str {
		switch c {
		case 'p':
			levels = append(levels, PanicLevel)
		case 'f':
			levels = append(levels, FatalLevel)
		case 'e':
			levels = append(levels, ErrorLevel)
		case 'w':
			levels = append(levels, WarnLevel)
		case 'i':
			levels = append(levels, InfoLevel)
		case 'd':
			levels = append(levels, DebugLevel)
		case 't':
			levels = append(levels, TraceLevel)
		default:
			continue
		}
	}
	return levels
}

// String returns the log level as a string
func (level LogLevel) String() string {
	return string(level)
}
