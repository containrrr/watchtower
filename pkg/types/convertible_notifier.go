package types

import (
	"time"
)

// ConvertibleNotifier is a notifier capable of creating a shoutrrr URL
type ConvertibleNotifier interface {
	GetURL(title string) (string, error)
}

// DelayNotifier is a notifier that might need to be delayed before sending notifications
type DelayNotifier interface {
	GetDelay() time.Duration
}
