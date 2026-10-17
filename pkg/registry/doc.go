// Package registry provides functionality for interacting with container registries in Watchtower.
// It handles authentication, digest retrieval, and image pull options for registry operations.
//
// Key components:
//   - age: Fetches image creation time from registry config blobs for cooldown support.
//   - auth: Manages registry authentication (token fetching, challenge handling).
//   - digest: Retrieves and compares image digests via HTTP requests.
//   - ratelimit: Parses registry 429 responses and retries them with backoff.
//   - hosts: Canonical registry domain names (GHCR, LSCR, Docker Hub).
//   - helpers: Utilities for registry address parsing and digest normalization.
//   - manifest: Constructs manifest URLs for digest fetching.
//   - registry: Configures pull options, API consumption checks, and image age fetching.
//
// Usage example:
//
//	opts, err := registry.GetPullOptions(log, "docker.io/library/alpine")
//	if err != nil {
//	    log.Error().Err(err).Msg("Failed to get pull options")
//	}
//	match, err := digest.CompareDigest(log, ctx, container, opts.RegistryAuth, settings)
//
// The package integrates with Docker's registry API, supports credential fetching from config files
// or environment variables, and uses zerolog for logging operations.
package registry
