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

type LogLevel int

const (
	PanicLevel LogLevel = iota
	FatalLevel
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

func (level LogLevel) String() string {
	switch level {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warning"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case PanicLevel:
		return "panic"
	}
	return ""
}
