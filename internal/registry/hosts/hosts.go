package hosts

const (
	// DockerRegistryDomain is the primary domain for Docker Hub image references.
	DockerRegistryDomain = "docker.io"
	// DockerRegistryHost is the current Docker Hub registry API endpoint.
	DockerRegistryHost = "index.docker.io"
	// GitHubRegistryDomain is the canonical domain for GitHub Container Registry.
	GitHubRegistryDomain = "ghcr.io"
	// LSCRRegistryDomain is LinuxServer's vanity domain. Images are hosted on ghcr.io.
	LSCRRegistryDomain = "lscr.io"
)

// IsGitHubRegistry reports whether host is GHCR or an LSCR vanity front for it.
//
// Parameters:
//   - host: Registry host from an image reference or URL.
//
// Returns:
//   - bool: True for [GitHubRegistryDomain] and [LSCRRegistryDomain].
func IsGitHubRegistry(host string) bool {
	return host == GitHubRegistryDomain || host == LSCRRegistryDomain
}
