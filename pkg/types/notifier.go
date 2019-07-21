package types

type Notifier interface {
	StartNotification()
	SendNotification()
}
