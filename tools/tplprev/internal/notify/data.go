package notify

import (
	"time"

	"github.com/nicholas-fedor/tplprev/internal/report"
)

// StaticData is the part of the notification template data model set upon initialization.
type StaticData struct {
	Title string
	Host  string
}

// Entry is a snapshot of a log event for notification templates.
// Field names match the shape templates expect (Message, Data, Level, Time).
type Entry struct {
	Message string
	Data    map[string]any
	Time    time.Time
	Level   string
}

// Data is the notification template data model.
type Data struct {
	StaticData

	Entries []*Entry
	Report  report.Report
}
