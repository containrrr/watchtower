package types

// Notifier is the interface that all notification services have in common
type Notifier interface {
	StartNotification()
	SendNotification(Report)
	AddLogHook()
	GetNames() []string
	Close()
}
