package notifications

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// StaticData is the part of the notification template data model set upon initialization.
type StaticData struct {
	Title string
	Host  string
}

// notificationEntry is a snapshot of a log event for notification templates.
// Field names match the shape templates expect (Message, Data, Level, Time).
type notificationEntry struct {
	Message string
	Data    map[string]any
	Time    time.Time
	Level   string
}

// notificationData is the notification template data model.
type notificationData struct {
	StaticData

	Entries []*notificationEntry
	Report  types.Report
}

// levelToString converts a zerolog level to the legacy template string.
// WarnLevel maps to "warning" for backwards compatibility with templates
// that compare .Level against "warning". All other levels use zerolog's
// standard string representation.
func levelToString(level zerolog.Level) string {
	if level == zerolog.WarnLevel {
		return "warning"
	}

	return level.String()
}
