// Package lifecycle holds pre/post update lifecycle hook settings.
package lifecycle

// Lifecycle holds lifecycle hook configuration.
type Lifecycle struct {
	// Enabled turns on pre- and post-update lifecycle hooks.
	Enabled bool
	// UID is the default UID for lifecycle hook commands.
	UID int
	// GID is the default GID for lifecycle hook commands.
	GID int
}
