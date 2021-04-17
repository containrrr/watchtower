package types

type Report interface {
	Scanned() []ContainerReport
	Updated() []ContainerReport
	Failed() []ContainerReport
	Skipped() []ContainerReport
	Stale() []ContainerReport
	Fresh() []ContainerReport
}

type ContainerReport interface {
	ID() string
	Name() string
	OldImageID() string
	NewImageID() string
	ImageName() string
	Error() string
	State() string
}
