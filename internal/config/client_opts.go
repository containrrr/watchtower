package config

import (
	"github.com/nicholas-fedor/watchtower/pkg/container"
)

// ClientOptions builds container.ClientOptions from the resolved Config.
//
// Parameters:
//   - none (receiver Config).
//
// Returns:
//   - container.ClientOptions: Options for container.NewClient.
func (c Config) ClientOptions() container.ClientOptions {
	return container.ClientOptions{
		IncludeStopped:          c.Client.IncludeStopped,
		IncludeRestarting:       c.Client.IncludeRestarting,
		ReviveStopped:           c.Client.ReviveStopped,
		RemoveVolumes:           c.Client.RemoveVolumes,
		DisableMemorySwappiness: c.Client.DisableMemorySwappiness || c.Compatibility.DisableMemorySwappiness,
		// Prefer Compatibility, then Client — same order as UpdateParams.
		CPUCopyMode:      firstNonEmpty(c.Compatibility.CPUCopyMode, c.Client.CPUCopyMode),
		WarnOnHeadFailed: container.WarningStrategy(c.Client.WarnOnHeadFailure),
		Registry:         c.Registry,
	}
}

// firstNonEmpty returns a if non-empty, otherwise b.
//
// Parameters:
//   - a: Preferred string.
//   - b: Fallback string.
//
// Returns:
//   - string: First non-empty value, or empty if both are empty.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}

	return b
}
