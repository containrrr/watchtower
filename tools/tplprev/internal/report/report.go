package report

// Report defines container session results.
type Report interface {
	Scanned() []ContainerReport
	Updated() []ContainerReport
	Failed() []ContainerReport
	Skipped() []ContainerReport
	Stale() []ContainerReport
	Fresh() []ContainerReport
	Restarted() []ContainerReport
	All() []ContainerReport
}

// ContainerReport defines a container's session status.
type ContainerReport interface {
	ID() ContainerID              // Container ID.
	Name() string                 // Container name.
	CurrentImageID() ImageID      // Original image ID.
	LatestImageID() ImageID       // Latest image ID.
	ImageName() string            // Image name with tag.
	Error() string                // Error message, if any.
	State() string                // Human-readable state.
	IsMonitorOnly() bool          // Monitor-only status.
	NewContainerID() ContainerID  // New container ID after update.
	GitRepo() string              // Resolved Git repository URL.
	GitRef() string               // Resolved Git ref, empty when not Git-associated.
	Changelog() string            // Changelog or releases URL for the latest known image.
	Source() string               // OCI org.opencontainers.image.source.
	ImageURL() string             // OCI org.opencontainers.image.url.
	Documentation() string        // OCI org.opencontainers.image.documentation.
	CurrentImageVersion() string  // OCI org.opencontainers.image.version of the running image.
	LatestImageVersion() string   // OCI org.opencontainers.image.version of a newer image.
	CurrentImageRevision() string // OCI org.opencontainers.image.revision of the running image.
	LatestImageRevision() string  // OCI org.opencontainers.image.revision of a newer image.
}
