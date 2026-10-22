// Package compatibility holds runtime compatibility settings (for example Podman).
package compatibility

// Compatibility holds container recreate compatibility options.
type Compatibility struct {
	// DisableMemorySwappiness clears memory swappiness when recreating containers.
	DisableMemorySwappiness bool
	// CPUCopyMode controls CPU settings when recreating containers.
	CPUCopyMode string
}
