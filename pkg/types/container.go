package types

import (
	"strings"
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
)

// WatchtowerOldPrefix is the prefix used when renaming Watchtower containers
// during self-update. It is the single source of truth for both rename
// generation and old-name detection to prevent cross-file protocol drift.
const WatchtowerOldPrefix = "watchtower-old-"

// Container defines a docker container's interface in Watchtower.
type Container interface {
	ContainerInfo() *dockerContainer.InspectResponse  // Container metadata.
	ID() ContainerID                                  // Container ID.
	IsRunning() bool                                  // Check if running.
	Name() string                                     // Container name.
	ImageID() ImageID                                 // Current image ID.
	ImageName() string                                // Image name with tag.
	Enabled() (bool, bool)                            // Enabled status and presence.
	IsMonitorOnly(params UpdateParams) bool           // Monitor-only check.
	Scope() (string, bool)                            // Scope value and presence.
	Links(useComposeDependsOn bool) []string          // Dependency links.
	GetLabel(key string) (string, bool)               // Arbitrary label value lookup.
	ToRestart() bool                                  // Needs restart check.
	IsWatchtower() bool                               // Watchtower instance check.
	StopSignal() string                               // Custom stop signal.
	StopTimeout() *int                                // Custom stop timeout in seconds.
	HasImageInfo() bool                               // Image metadata presence.
	ImageInfo() *dockerImage.InspectResponse          // Image metadata.
	GetLifecyclePreCheckCommand() string              // Pre-check command.
	GetLifecyclePostCheckCommand() string             // Post-check command.
	GetLifecyclePreUpdateCommand() string             // Pre-update command.
	GetLifecyclePostUpdateCommand() string            // Post-update command.
	GetLifecycleUID() (int, bool)                     // UID for lifecycle hooks, with presence.
	GetLifecycleGID() (int, bool)                     // GID for lifecycle hooks, with presence.
	VerifyConfiguration() error                       // Config validation.
	SetStale(status bool)                             // Set stale status.
	IsStale() bool                                    // Stale status check.
	IsNoPull(params UpdateParams) bool                // No-pull check.
	CooldownDelay(params UpdateParams) time.Duration  // Effective cooldown delay.
	SetLinkedToRestarting(status bool)                // Set linked-to-restarting status.
	IsLinkedToRestarting() bool                       // Linked-to-restarting check.
	PreUpdateTimeout() int                            // Pre-update timeout.
	PostUpdateTimeout() int                           // Post-update timeout.
	IsRestarting() bool                               // Restarting status check.
	IsCreated() bool                                  // Created-state check.
	GetCreateConfig() *dockerContainer.Config         // Creation config.
	GetCreateHostConfig() *dockerContainer.HostConfig // Host creation config.
	GetContainerChain() (string, bool)                // Container chain label value and presence.
	HasExposedPorts() bool                            // Exposed ports presence check.
}

// ImageID is a hash string for a container image.
type ImageID string

// ContainerID is a hash string for a container instance.
type ContainerID string

// ShortID returns the 12-character short version of an image ID.
//
// Returns:
//   - string: Shortened ID without "sha256:" prefix.
func (id ImageID) ShortID() string {
	return shortID(string(id))
}

// ShortID returns the 12-character short version of a container ID.
//
// Returns:
//   - string: Shortened ID without "sha256:" prefix.
func (id ContainerID) ShortID() string {
	return shortID(string(id))
}

// shortID shortens a hash string to 12 characters.
//
// Parameters:
//   - longID: Full hash string.
//
// Returns:
//   - string: Shortened ID, adjusted for "sha256:" prefix.
func shortID(longID string) string {
	prefixSep := strings.IndexRune(longID, ':')
	offset := 0
	length := 12

	// Adjust offset for "sha256:" prefix.
	if prefixSep >= 0 {
		if longID[0:prefixSep] == "sha256" {
			offset = prefixSep + 1
		} else {
			length += prefixSep + 1
		}
	}

	// Return shortened ID or full string if too short.
	if len(longID) >= offset+length {
		return longID[offset : offset+length]
	}

	return longID
}
