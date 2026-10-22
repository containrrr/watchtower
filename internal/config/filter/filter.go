// Package filter holds container selection inputs and the resolved filter.
package filter

import "github.com/nicholas-fedor/watchtower/pkg/types"

// Filter holds raw filter inputs and the resolved predicate.
type Filter struct {
	// LabelEnable restricts watching to containers with the enable label.
	LabelEnable bool
	// DisableContainers lists container names to exclude.
	DisableContainers []string
	// MonitorImageNames lists image name patterns to include.
	MonitorImageNames []string
	// SkipImageNames lists image name patterns to exclude.
	SkipImageNames []string
	// EnableContainersByLabel lists key=value pairs that must match.
	EnableContainersByLabel []string
	// DisableContainersByLabel lists key=value pairs that exclude matches.
	DisableContainersByLabel []string
	// Scope is the Watchtower monitoring scope label value.
	Scope string
	// Names are positional container name arguments.
	Names []string
	// Predicate is the resolved container filter function.
	Predicate types.Filter
	// Desc is a human-readable description of the filter.
	Desc string
}
