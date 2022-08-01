package types

// Report contains reports for all the containers processed during a session
type Report interface {
	Scanned() []ContainerReport
	Updated() []ContainerReport
	Failed() []ContainerReport
	Skipped() []ContainerReport
	Stale() []ContainerReport
	Fresh() []ContainerReport
	All() []ContainerReport
}

// ContainerReport represents a container that was included in watchtower session
type ContainerReport interface {
	ID() ContainerID
	Name() string
	CurrentImageID() ImageID
	LatestImageID() ImageID
	ImageName() string
	Error() string
	State() string
}
