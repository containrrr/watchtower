// Package mocks provides mock implementations for container interfaces used in testing.
package mocks

import (
	"strings"
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// SimpleContainer implements a minimal Container interface for benchmarking.
type SimpleContainer struct {
	ContainerName      string
	ContainerID        types.ContainerID
	ContainerLinks     []string
	ContainerInfoField *dockerContainer.InspectResponse
}

func (c *SimpleContainer) Name() string {
	return util.NormalizeContainerName(c.ContainerName)
}

func (c *SimpleContainer) ID() types.ContainerID {
	return c.ContainerID
}

func (c *SimpleContainer) Links(useComposeDependsOn bool) []string {
	return c.ContainerLinks
}

func (c *SimpleContainer) IsWatchtower() bool {
	return false
}

func (c *SimpleContainer) ContainerInfo() *dockerContainer.InspectResponse {
	if c.ContainerInfoField != nil {
		return c.ContainerInfoField
	}

	name := c.ContainerName
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	// Ensure Name includes leading "/" to match Docker API behavior
	return &dockerContainer.InspectResponse{
		Name:   name,
		Config: &dockerContainer.Config{Labels: map[string]string{}},
	}
}

// IsRunning returns true for mock containers.
func (c *SimpleContainer) IsRunning() bool { return true }

func (c *SimpleContainer) ImageID() types.ImageID {
	if c.ContainerInfoField != nil {
		image := string(c.ContainerInfoField.Image)
		if strings.HasPrefix(image, "sha256:") {
			return types.ImageID(image)
		} else {
			return types.ImageID("sha256:" + image)
		}
	}

	return types.ImageID("sha256:" + string(c.ContainerID))
}

func (c *SimpleContainer) ImageName() string {
	if c.ContainerInfoField != nil {
		return c.ContainerInfoField.Config.Image
	}

	return "test-image"
}

func (c *SimpleContainer) Enabled() (bool, bool)                   { return true, true }
func (c *SimpleContainer) IsMonitorOnly(_ types.UpdateParams) bool { return false }

func (c *SimpleContainer) Scope() (string, bool) { return "", false }
func (c *SimpleContainer) GetLabel(key string) (string, bool) {
	if c.ContainerInfoField != nil && c.ContainerInfoField.Config != nil && c.ContainerInfoField.Config.Labels != nil {
		val, ok := c.ContainerInfoField.Config.Labels[key]

		return val, ok
	}

	return "", false
}
func (c *SimpleContainer) ToRestart() bool { return false }

func (c *SimpleContainer) StopSignal() string {
	if c.ContainerInfoField != nil {
		return c.ContainerInfoField.Config.StopSignal
	}

	return "SIGTERM"
}

// StopTimeout returns the container's configured stop timeout in seconds from ContainerInfoField.
func (c *SimpleContainer) StopTimeout() *int {
	if c.ContainerInfoField != nil && c.ContainerInfoField.Config != nil {
		return c.ContainerInfoField.Config.StopTimeout
	}

	return nil
}

func (c *SimpleContainer) HasImageInfo() bool                               { return false }
func (c *SimpleContainer) ImageInfo() *dockerImage.InspectResponse          { return nil }
func (c *SimpleContainer) GetLifecyclePreCheckCommand() string              { return "" }
func (c *SimpleContainer) GetLifecyclePostCheckCommand() string             { return "" }
func (c *SimpleContainer) GetLifecyclePreUpdateCommand() string             { return "" }
func (c *SimpleContainer) GetLifecyclePostUpdateCommand() string            { return "" }
func (c *SimpleContainer) GetLifecycleUID() (int, bool)                     { return 0, false }
func (c *SimpleContainer) GetLifecycleGID() (int, bool)                     { return 0, false }
func (c *SimpleContainer) GetContainerChain() (string, bool)                { return "", false }
func (c *SimpleContainer) VerifyConfiguration() error                       { return nil }
func (c *SimpleContainer) SetStale(_ bool)                                  {}
func (c *SimpleContainer) IsStale() bool                                    { return false }
func (c *SimpleContainer) IsNoPull(_ types.UpdateParams) bool               { return false }
func (c *SimpleContainer) CooldownDelay(_ types.UpdateParams) time.Duration { return 0 }
func (c *SimpleContainer) SetLinkedToRestarting(_ bool)                     {}
func (c *SimpleContainer) IsLinkedToRestarting() bool                       { return false }
func (c *SimpleContainer) PreUpdateTimeout() int {
	if c.ContainerInfoField != nil {
		return 0 // Return 0 to disable timeout when using custom ContainerInfo
	}

	return 30
}

func (c *SimpleContainer) PostUpdateTimeout() int {
	if c.ContainerInfoField != nil {
		return 0
	}

	return 30
}
func (c *SimpleContainer) IsRestarting() bool                               { return false }
func (c *SimpleContainer) IsCreated() bool                                  { return false }
func (c *SimpleContainer) GetCreateConfig() *dockerContainer.Config         { return nil }
func (c *SimpleContainer) GetCreateHostConfig() *dockerContainer.HostConfig { return nil }
func (c *SimpleContainer) HasExposedPorts() bool                            { return false }
