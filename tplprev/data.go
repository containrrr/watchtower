package main

import (
	"time"

	"github.com/containrrr/watchtower/pkg/types"
)

type Data struct {
	Entries    []*LogEntry
	StaticData StaticData
	Report     types.Report
}

type StaticData struct {
	Title string
	Host  string
}

type LogEntry struct {
	Message string
	Data    map[string]any
	Time    time.Time
	Level   LogLevel
}

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

func (level LogLevel) String() string {
	return string(level)
}
