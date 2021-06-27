package types

import (
	"github.com/docker/docker/api/types"
	"strings"
)

// ImageID is a hash string representing a container image
type ImageID string

// ContainerID is a hash string representing a container instance
type ContainerID string

// ShortID returns the 12-character (hex) short version of an image ID hash, removing any "sha256:" prefix if present
func (id ImageID) ShortID() (short string) {
	return shortID(string(id))
}

// ShortID returns the 12-character (hex) short version of a container ID hash, removing any "sha256:" prefix if present
func (id ContainerID) ShortID() (short string) {
	return shortID(string(id))
}

func shortID(longID string) string {
	prefixSep := strings.IndexRune(longID, ':')
	offset := 0
	length := 12
	if prefixSep >= 0 {
		if longID[0:prefixSep] == "sha256" {
			offset = prefixSep + 1
		} else {
			length += prefixSep + 1
		}
	}

	if len(longID) >= offset+length {
		return longID[offset : offset+length]
	}

	return longID
}

// Container is a docker container running an image
type Container interface {
	ContainerInfo() *types.ContainerJSON
	ID() ContainerID
	IsRunning() bool
	Name() string
	ImageID() ImageID
	SafeImageID() ImageID
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
