// Package client holds Docker client construction options derived from flags.
package client

// Client holds options passed to container.NewClient.
type Client struct {
	// IncludeStopped includes created and exited containers in updates
	// (--include-stopped / WATCHTOWER_INCLUDE_STOPPED).
	IncludeStopped bool
	// IncludeRestarting includes containers that are restarting
	// (--include-restarting / WATCHTOWER_INCLUDE_RESTARTING).
	IncludeRestarting bool
	// ReviveStopped starts stopped containers after update when IncludeStopped is active
	// (--revive-stopped / WATCHTOWER_REVIVE_STOPPED).
	ReviveStopped bool
	// RemoveVolumes removes attached volumes before updating
	// (--remove-volumes / WATCHTOWER_REMOVE_VOLUMES).
	RemoveVolumes bool
	// DisableMemorySwappiness clears memory swappiness when recreating containers
	// for Podman compatibility (--disable-memory-swappiness).
	DisableMemorySwappiness bool
	// CPUCopyMode controls CPU settings when recreating containers
	// (--cpu-copy-mode / WATCHTOWER_CPU_COPY_MODE: auto, full, none).
	CPUCopyMode string
	// WarnOnHeadFailure is the HEAD pull failure warning strategy
	// (--warn-on-head-failure / WATCHTOWER_WARN_ON_HEAD_FAILURE: always, auto, never).
	WarnOnHeadFailure string
}
