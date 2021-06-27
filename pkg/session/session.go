package session

import (
	"time"
)

type Session struct {
	Trigger  Trigger
	Started  time.Time
	Progress Progress
}

func New(trigger Trigger) *Session {
	return &Session{
		Started:  time.Now().UTC(),
		Trigger:  trigger,
		Progress: Progress{},
	}
}

// Report creates a new Report from a Session instance
func (s Session) Report() *Report {
	return NewReport(s.Started, time.Now().UTC(), s.Trigger, s.Progress)
}
