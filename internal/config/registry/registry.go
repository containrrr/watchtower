// Package registry holds the settings for registry requests.
package registry

// Registry holds the settings for registry requests.
type Registry struct {
	// CooldownPlatform selects the image platform whose creation time the
	// cooldown check reads from the registry.
	CooldownPlatform Platform
	// TLSMinVersion is the minimum TLS version for registry connections, such
	// as "TLS1.3". Empty means TLS 1.2.
	TLSMinVersion string
	// TLSSkip sends registry requests over plain HTTP and disables TLS
	// certificate verification.
	TLSSkip bool
}

// Platform names an image platform within a multi-platform image.
type Platform struct {
	// OS is the operating system, such as "linux". Empty means the runtime
	// operating system.
	OS string
	// Architecture is the CPU architecture, such as "arm64". Empty means the
	// runtime architecture.
	Architecture string
	// Variant is the CPU variant, such as "v7". Empty means no variant.
	Variant string
}
