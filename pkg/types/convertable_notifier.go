package types

// ConvertableNotifier is a notifier capable of creating a shoutrrr URL
type ConvertableNotifier interface {
	Notifier
	GetURL() string
}
