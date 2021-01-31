package types

// ConvertibleNotifier is a notifier capable of creating a shoutrrr URL
type ConvertibleNotifier interface {
	GetURL() (string, error)
}
