package container

type Interface interface {
	ID() string
	Name() string
	SafeImageID() string
	ImageName() string
}
