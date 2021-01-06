package types

import "github.com/docker/docker/api/types"

// Container is a docker container running an image
type Container interface {
	ContainerInfo() *types.ContainerJSON
	ID() string
	IsRunning() bool
	Name() string
	ImageID() string
	ImageName() string
	Enabled() (bool, bool)
	IsMonitorOnly() bool
	Scope() (string, bool)
	Links() []string
	ToRestart() bool
	IsWatchtower() bool
	StopSignal() string
	HasImageInfo() bool
	ImageInfo() *types.ImageInspect
	GetLifecyclePreCheckCommand() string
	GetLifecyclePostCheckCommand() string
	GetLifecyclePreUpdateCommand() string
	GetLifecyclePostUpdateCommand() string
}
