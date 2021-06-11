package container

// Interface is the minimum common Container interface
type Interface interface {
	ID() string
	Name() string
	SafeImageID() string
	ImageName() string
}
