package session

import "strings"

type Trigger int

const (
	SchedulerTrigger Trigger = iota
	APITrigger
	StartupTrigger
)

// String returns a string representation of the Trigger
func (trigger Trigger) String() string {
	switch trigger {
	case SchedulerTrigger:
		return "Scheduler"
	case APITrigger:
		return "API"
	case StartupTrigger:
		return "Startup"
	default:
		return "Unknown"
	}
}

// MarshalJSON marshals Trigger as a quoted string
func (trigger Trigger) MarshalJSON() ([]byte, error) {
	sb := strings.Builder{}
	sb.WriteString(`"`)
	sb.WriteString(trigger.String())
	sb.WriteString(`"`)
	return []byte(sb.String()), nil
}
