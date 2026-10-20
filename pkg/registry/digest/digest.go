// Package digest fetches image digests from container registries and compares
// them with local image digests to decide whether a container's image is stale.
package digest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
	"github.com/nicholas-fedor/watchtower/pkg/registry/hosts"
	"github.com/nicholas-fedor/watchtower/pkg/registry/manifest"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// ContentDigestHeader is the HTTP header key used to retrieve the digest from a registry's response.
// This header, typically "Docker-Content-Digest", contains the digest value (e.g., "sha256:abc...") for an image manifest,
// allowing Watchtower to compare or fetch it without downloading the full manifest body.
const ContentDigestHeader = "Docker-Content-Digest"

// maxManifestSize is the maximum allowed size for a digest GET fallback body (1 MiB).
const maxManifestSize = 1 << 20

// Errors for digest retrieval operations.
var (
	// errMissingImageInfo indicates the container lacks image metadata, preventing digest operations.
	errMissingImageInfo = errors.New("container image info missing")
	// errInvalidRegistryResponse indicates the registry's HEAD response lacks a digest or is malformed.
	errInvalidRegistryResponse = errors.New("registry responded with invalid HEAD request")
	// errManifestNotFound indicates the registry returned 404 for the manifest request.
	errManifestNotFound = errors.New("manifest not found")
	// errFailedGetToken indicates a failure to obtain an authentication token from the registry.
	errFailedGetToken = errors.New("failed to get token")
	// errFailedBuildManifestURL indicates a failure to construct the manifest URL for the registry.
	errFailedBuildManifestURL = errors.New("failed to build manifest URL")
	// errFailedParseImageName indicates an image reference could not be parsed or retagged.
	errFailedParseImageName = errors.New("failed to parse image name")
	// errFailedCreateRequest indicates a failure to construct an HTTP request for digest retrieval.
	errFailedCreateRequest = errors.New("failed to create request")
	// errFailedExecuteRequest indicates a failure to execute an HTTP request to the registry.
	errFailedExecuteRequest = errors.New("failed to execute request")
	// errManifestTooLarge indicates the digest GET fallback body exceeded the size limit.
	errManifestTooLarge = errors.New("manifest response exceeds size limit")
	// errUnresolvedManifestRetry indicates the retried manifest request was sent to yet another host.
	errUnresolvedManifestRetry = errors.New("manifest request still needs another host after retrying")
)

// localOnlyImageCache records image name+ID pairs previously confirmed as local-only
// (domain-less Config.Image with a registry 404). Subsequent checks skip remote
// probes, avoiding Docker Hub rate-limit pressure that can starve concurrent
// staleness checks for other containers in the same update cycle.
//
// Keyed by "imageName|imageID".
// Entries live for process lifetime and a rebuild (new image ID) forces a fresh probe.
var localOnlyImageCache sync.Map

// localOnlyCacheKey builds a cache key for a container's image identity.
func localOnlyCacheKey(container types.Container) string {
	if !container.HasImageInfo() {
		return container.ImageName() + "|"
	}

	return container.ImageName() + "|" + container.ImageInfo().ID
}

// rememberLocalOnlyImage records that the container's image was confirmed local-only.
func rememberLocalOnlyImage(container types.Container) {
	localOnlyImageCache.Store(localOnlyCacheKey(container), true)
}

// isCachedLocalOnlyImage reports whether the container's image was previously
// confirmed local-only for this process.
func isCachedLocalOnlyImage(container types.Container) bool {
	_, ok := localOnlyImageCache.Load(localOnlyCacheKey(container))

	return ok
}

// NormalizeDigest standardizes a digest string for consistent comparison.
//
// It trims common prefixes (e.g., "sha256:") to return the raw digest value.
//
// Parameters:
//   - digest: Digest string (e.g., "sha256:abc123").
//
// Returns:
//   - string: Normalized digest (e.g., "abc123").
func NormalizeDigest(log *zerolog.Logger, digest string) string {
	// List of prefixes to strip from the digest.
	prefixes := []string{"sha256:"}
	for _, prefix := range prefixes {
		after, ok := strings.CutPrefix(digest, prefix)
		if ok {
			// Trim the prefix to get the raw digest value.
			normalized := after
			log.Debug().
				Str("original", digest).
				Str("normalized", normalized).
				Msg("Normalized digest by trimming prefix")

			return normalized
		}
	}

	// Return unchanged if no prefix matches.
	return digest
}

// CompareDigest checks whether a container's current image digest matches the
// latest from its registry.
//
// It is a convenience wrapper around CompareDigestWithRemote that discards the
// remote digest.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - container: Container whose digest is being compared.
//   - registryAuth: Base64-encoded auth string.
//   - settings: Registry request settings.
//   - limiter: Rate limiter shared by registry requests. Must not be nil.
//   - endpoints: Optional list of registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - bool: True if digests match (image is up-to-date), false otherwise.
//   - error: Non-nil if operation fails, nil on success.
func CompareDigest(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	settings registryConfig.Registry,
	limiter *ratelimit.Limiter,
	endpoints ...string,
) (bool, error) {
	match, _, err := CompareDigestWithRemote(log,
		ctx,
		container,
		registryAuth,
		settings,
		limiter,
		endpoints...,
	)

	return match, err
}

// isLocalImageNotFound reports whether the digest fetch failed because the
// registry returned 404 for a manifest request and the container's image looks
// local-only: domain-less Config.Image and no registry-qualified RepoDigests.
//
// Official Hub short names (for example nginx) often store Config.Image without
// a domain but still have docker.io/... RepoDigests. Those must not be cached as
// local-only after a transient 404, or updates are skipped for the process life.
//
// Parameters:
//   - container: Container whose image name is inspected.
//   - err: Error returned from digest fetch.
//
// Returns:
//   - bool: True if the image should be treated as local-only, false otherwise.
func isLocalImageNotFound(container types.Container, err error) bool {
	if !errors.Is(err, errManifestNotFound) {
		return false
	}

	if !container.HasImageInfo() ||
		container.ContainerInfo() == nil ||
		container.ContainerInfo().Config == nil {
		return false
	}

	// Evaluate only the repository name so tags/digests (e.g. my-app:1.0) do not
	// make a domain-less image look registry-qualified via "." in a semver tag.
	originalImage := container.ContainerInfo().Config.Image
	namePart, _, _ := strings.Cut(originalImage, "@")

	if i := strings.LastIndex(namePart, ":"); i >= 0 {
		if j := strings.LastIndex(namePart, "/"); j < i {
			namePart = namePart[:i]
		}
	}

	if strings.ContainsAny(namePart, "./") {
		return false
	}

	// Registry-qualified digests mean the image was pulled from a remote source.
	// a 404 is a probe failure, not proof the image is local-only. Official Hub
	// short names often keep a domain-less Config.Image while RepoDigests still
	// records docker.io/library/... .
	for _, digestRef := range container.ImageInfo().RepoDigests {
		repoPart, _, _ := strings.Cut(digestRef, "@")
		if repoPartHasRegistryHost(repoPart) {
			return false
		}
	}

	return true
}

// repoPartHasRegistryHost reports whether a RepoDigest repository path includes
// a registry host (hostname with a dot, or host:port).
func repoPartHasRegistryHost(repoPart string) bool {
	// Strip optional scheme leftovers. RepoDigests are host/path form.
	hostOrName, remainder, hasSlash := strings.Cut(repoPart, "/")
	if !hasSlash {
		// Bare name (e.g. "nginx") has no host segment.
		return false
	}

	// host:port/path or registry.example.com/path
	if strings.Contains(hostOrName, ":") || strings.Contains(hostOrName, ".") {
		return true
	}

	// Remainder unused. Keep the signature clear for future extension.
	_ = remainder

	return false
}

// CompareDigestWithRemote checks whether a container's current image digest matches
// the latest from its registry and returns the remote digest used for comparison.
//
// It first inspects the image to check if it's locally built (empty RepoDigests).
// For local images, digest comparison against a remote registry is not possible,
// so it returns true to indicate the image should not be updated. This avoids
// unnecessary HTTP requests and confusing error messages for locally built images.
//
// When endpoints are provided (registry mirror hosts), each is tried in order.
// An empty string in the endpoints list means use the canonical registry host.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - container: Container whose digest is being compared.
//   - registryAuth: Base64-encoded auth string.
//   - settings: Registry request settings.
//   - limiter: Rate limiter shared by registry requests. Must not be nil.
//   - endpoints: Optional list of registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - bool: True if digests match (image is up-to-date), false otherwise.
//   - string: Remote registry digest in "sha256:..." form (empty when unavailable).
//   - error: Non-nil if operation fails, nil on success.
func CompareDigestWithRemote(log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	settings registryConfig.Registry,
	limiter *ratelimit.Limiter,
	endpoints ...string,
) (bool, string, error) {
	fields := map[string]any{
		"container": container.Name(),
		"image":     container.ImageName(),
	}

	// Ensure the container has image metadata to proceed with digest comparison.
	if !container.HasImageInfo() {
		log.Debug().
			Fields(fields).
			Msg("Container image info missing")

		return false, "", errMissingImageInfo
	}

	// Check if the container's image has no RepoDigests, which indicates a locally
	// built image that has never been pushed or pulled. For such images, there is
	// no remote digest to compare against, so we treat them as up-to-date to avoid
	// unnecessary registry requests and confusing error messages.
	//
	// We check container.ImageInfo().RepoDigests rather than inspecting via the
	// Docker daemon because:
	// 1. The container was already populated with image info during initialization
	// 2. For locally built images, RepoDigests are either empty or registry-less
	// 3. This avoids an extra Docker daemon call
	if len(container.ImageInfo().RepoDigests) == 0 {
		log.Debug().
			Fields(fields).
			Msg("Image with no registry reference detected (empty RepoDigests) - skipping digest comparison")

		return true, "", nil
	}

	// Skip registry probes for images previously confirmed local-only (same name+ID).
	// This prevents repeated Docker Hub 404 traffic that can rate-limit concurrent
	// checks for other containers in the same update session.
	if isCachedLocalOnlyImage(container) {
		log.Debug().
			Fields(fields).
			Msg("Cached local-only image - skipping digest comparison")

		return true, "", nil
	}

	// Fetch the latest digest from the registry using a HEAD request for efficiency.
	remoteDigest, err := fetchDigest(log,
		ctx,
		container,
		registryAuth,
		settings,
		limiter,
		http.MethodHead,
		"",
		endpoints...,
	)
	if err != nil {
		// Domain-less image names that 404 are local-only builds. Treat them as up-to-date
		// with no error so callers skip the pull without logging a warning.
		if isLocalImageNotFound(container, err) {
			rememberLocalOnlyImage(container)
			log.Debug().
				Fields(fields).
				Msg("Image not found in registry - treating as local image")

			return true, "", nil
		}

		return false, "", err
	}

	// If HEAD request returned empty digest (due to missing Docker-Content-Digest header),
	// fall back to GET request.
	if remoteDigest == "" {
		log.Debug().
			Fields(fields).
			Msg("HEAD request returned empty digest - falling back to GET")

		remoteDigest, err = fetchDigestWithGet(log,
			ctx,
			container,
			registryAuth,
			settings,
			limiter,
			endpoints...,
		)
		if err != nil {
			return false, "", err
		}
	}

	// Compare the fetched remote digest with the container's local digests.
	matches := digestsMatch(log,
		container.ImageInfo().RepoDigests,
		remoteDigest,
	)
	log.Debug().
		Fields(fields).
		Bool("matches", matches).
		Msg("Completed digest comparison")

	return matches, formatDigest(remoteDigest), nil
}

// formatDigest ensures a digest string uses the "sha256:..." form.
//
// Empty input is returned unchanged. Digests that already include a known
// algorithm prefix are returned as-is, otherwise "sha256:" is prepended.
//
// Parameters:
//   - digest: Digest string (raw hash or "sha256:...").
//
// Returns:
//   - string: Digest with algorithm prefix when non-empty.
func formatDigest(digest string) string {
	if digest == "" {
		return ""
	}

	if strings.Contains(digest, ":") {
		return digest
	}

	return "sha256:" + digest
}

// fetchDigestWithGet retrieves the digest of an image from its registry using a GET request.
// It fetches the manifest to ensure the digest header is present, which may be necessary when
// HEAD requests are unsupported. The digest is extracted from the response headers and normalized for consistency.
//
// Parameters:
//   - ctx: The context controlling the request's lifecycle, enabling cancellation or timeouts.
//   - container: The container whose image digest is being fetched, providing the image name and reference.
//   - authToken: A base64-encoded authentication string for registry access.
//   - settings: Registry request settings.
//   - limiter: Rate limiter shared by registry requests.
//   - endpoints: Optional list of registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - string: The normalized digest (e.g., "abc..." without "sha256:") if successful.
//   - error: An error if the request fails or digest header is missing, nil if successful.
func fetchDigestWithGet(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	authToken string,
	settings registryConfig.Registry,
	limiter *ratelimit.Limiter,
	endpoints ...string,
) (string, error) {
	return fetchDigest(log, ctx, container, authToken, settings, limiter, http.MethodGet, "", endpoints...)
}

// FetchDigestForTag retrieves the manifest digest of a specific tag.
//
// It is used to confirm which release tag a maintainer actually published, so a
// changelog link can name the exact release rather than the release index. The
// request is a HEAD: a missing digest header is not retried with a GET, because
// the probe is optional and must stay cheap.
//
// Authentication is scoped to the container's repository, which is the same for
// every tag, so only the manifest specifier changes between candidates.
//
// Parameters:
//   - log: Logger.
//   - ctx: The context controlling the request's lifecycle.
//   - container: Container whose image supplies the repository and registry host.
//   - authToken: A base64-encoded authentication string for registry access.
//   - tag: Exact tag to fetch the digest for.
//   - settings: Registry request settings.
//   - limiter: Rate limiter shared by registry requests. Must not be nil.
//   - endpoints: Optional list of registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - string: The normalized digest, or empty when the tag does not exist.
//   - error: Non-nil if the request fails, nil on success.
func FetchDigestForTag(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	authToken string,
	tag string,
	settings registryConfig.Registry,
	limiter *ratelimit.Limiter,
	endpoints ...string,
) (string, error) {
	return fetchDigest(log, ctx, container, authToken, settings, limiter, http.MethodHead, tag, endpoints...)
}

// buildManifestURL constructs and validates a manifest URL for a container.
//
// It determines the scheme from the registry settings, builds the initial URL via
// manifest.BuildManifestURLForImage (which always uses the canonical host from the image reference),
// then overrides the host and scheme if hostOverride is a non-empty bare host or full endpoint
// URL (e.g. "https://mirror.example.com"). It returns the final URL, the original host before
// override, and the parsed URL object.
//
// The hostOverride parameter accepts either a bare host or a full URL. When a full URL is
// supplied, its scheme (if present) takes precedence over the registry settings.
//
// Parameters:
//   - container: Container whose manifest URL is being built.
//   - settings: Registry request settings.
//   - hostOverride: Optional host or endpoint URL to use instead of the canonical host.
//     An empty string uses the canonical host.
//
// Returns:
//   - string: The final manifest URL.
//   - string: The original host before applying hostOverride.
//   - *url.URL: The parsed URL object.
//   - error: Non-nil if construction or validation fails.
func buildManifestURL(log *zerolog.Logger,
	container types.Container,
	settings registryConfig.Registry,
	hostOverride string,
) (string, string, *url.URL, error) {
	return buildManifestURLForImage(log, container.ImageName(), container.Name(), settings, hostOverride)
}

// buildManifestURLForTag constructs a manifest URL for a specific tag of an
// image, leaving the repository and registry host unchanged.
//
// This is how a release tag is confirmed without pulling it: the manifest for
// the candidate tag is requested and its digest compared with the digest of the
// image that was actually pulled. Authentication scope is the repository, which
// is identical for every tag, so only the manifest specifier changes.
//
// Parameters:
//   - log: Logger.
//   - container: Container whose image supplies the repository and registry host.
//   - settings: Registry request settings.
//   - hostOverride: Optional host or endpoint URL to use instead of the canonical host.
//   - tag: Exact tag to target. An empty tag yields the container's own reference.
//
// Returns:
//   - string: The final manifest URL.
//   - string: The original host before applying hostOverride.
//   - *url.URL: The parsed URL object.
//   - error: Non-nil if construction or validation fails.
func buildManifestURLForTag(log *zerolog.Logger,
	container types.Container,
	settings registryConfig.Registry,
	hostOverride string,
	tag string,
) (string, string, *url.URL, error) {
	imageName, err := imageNameForTag(container.ImageName(), tag)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image", container.ImageName()).
			Str("tag", tag).
			Msg("Failed to resolve image name for tag")

		return "", "", nil, fmt.Errorf("%w: %w", errFailedBuildManifestURL, err)
	}

	return buildManifestURLForImage(log, imageName, container.Name(), settings, hostOverride)
}

// imageNameForTag returns the image reference with its tag replaced by tag.
//
// The repository and registry host are preserved. A reference that carries no
// usable tag gets one appended, so a bare repository name still resolves.
//
// Parameters:
//   - imageName: Original image reference.
//   - tag: Exact tag to apply, or empty to leave the reference unchanged.
//
// Returns:
//   - string: Image reference for the requested tag.
//   - error: Non-nil when the reference or the tag cannot be parsed.
func imageNameForTag(imageName, tag string) (string, error) {
	if tag == "" {
		return imageName, nil
	}

	named, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errFailedParseImageName, err)
	}

	tagged, err := reference.WithTag(reference.TrimNamed(named), tag)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errFailedParseImageName, err)
	}

	return tagged.String(), nil
}

// buildManifestURLForImage constructs and validates a manifest URL for an image
// reference.
//
// Parameters:
//   - log: Logger.
//   - imageName: Image reference supplying the registry host and repository.
//   - containerName: Container name used only for log context.
//   - settings: Registry request settings.
//   - hostOverride: Optional host or endpoint URL to use instead of the canonical host.
//
// Returns:
//   - string: The final manifest URL.
//   - string: The original host before applying hostOverride.
//   - *url.URL: The parsed URL object.
//   - error: Non-nil if construction or validation fails.
func buildManifestURLForImage(log *zerolog.Logger,
	imageName,
	containerName string,
	settings registryConfig.Registry,
	hostOverride string,
) (string, string, *url.URL, error) {
	fields := map[string]any{
		"container": containerName,
		"image":     imageName,
	}

	scheme := auth.Scheme(settings)

	// Capture the original registry host from the image reference before any
	// canonicalization or host remapping. For lscr.io images, preserve the
	// original host so the redirect-detection logic can distinguish lscr.io
	// from its ghcr.io target. For all other images use the canonical host.
	originalHost := ""

	normalizedRef, parseErr := reference.ParseNormalizedNamed(imageName)
	if parseErr == nil {
		rawDomain := reference.Domain(normalizedRef)

		canonicalHost, _ := auth.GetRegistryAddress(log, imageName)
		if rawDomain == hosts.LSCRRegistryDomain {
			originalHost = rawDomain
		} else if canonicalHost != "" {
			originalHost = canonicalHost
		}
	}

	// Build the canonical manifest URL.
	manifestURLStr, err := manifest.BuildManifestURLForImage(log, imageName, "", scheme)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to build manifest URL")

		return "", "", nil, fmt.Errorf("%w: %w", errFailedBuildManifestURL, err)
	}

	parsedURL, parseErr := url.Parse(manifestURLStr)
	if parseErr != nil {
		return "", "", nil, fmt.Errorf(
			"%w: failed to parse manifest URL: %w",
			errFailedBuildManifestURL,
			parseErr,
		)
	}

	// Special handling for lscr.io registry redirects:
	// lscr.io (LinuxServer.io) images are hosted on GitHub Container Registry (ghcr.io)
	// but the registry redirects manifest requests from lscr.io to ghcr.io.
	// However, the authentication challenge comes from ghcr.io, and when we try to
	// make manifest requests to lscr.io, we get 401 Unauthorized followed by 404 Not Found
	// because lscr.io doesn't actually host the manifests - it's just a redirect endpoint.
	//
	// To fix this, we intercept lscr.io URLs and change the host to ghcr.io for manifest requests,
	// while still using lscr.io for the initial authentication challenge to get the correct tokens.
	// This ensures HEAD requests succeed with 200 OK and we can extract digests without
	// falling back to expensive full image pulls.
	//
	// The authentication flow works as follows:
	// 1. Initial challenge request to lscr.io/v2/ gets redirected to ghcr.io
	// 2. Authentication tokens are obtained from ghcr.io using the redirected challenge
	// 3. Manifest requests are made directly to ghcr.io (not lscr.io) to avoid 401/404 errors
	// 4. Digest extraction succeeds from the 200 OK response
	if parsedURL.Host == hosts.LSCRRegistryDomain {
		parsedURL.Host = hosts.GitHubRegistryDomain
		manifestURLStr = parsedURL.String()
	}

	if parsedURL.Host == "" {
		return "", "", nil, fmt.Errorf(
			"%w: manifest URL has no host: %s",
			errFailedBuildManifestURL,
			manifestURLStr,
		)
	}

	if hostOverride != "" {
		// Resolve the override as the challenge request does, so a mirror given
		// as a URL or as a bare host reaches the same host for both requests.
		host, scheme, err := auth.ResolveEndpoint(hostOverride, parsedURL.Host, parsedURL.Scheme)
		if err != nil {
			return "", "", nil, fmt.Errorf("%w: %w", errFailedBuildManifestURL, err)
		}

		parsedURL.Host = host
		parsedURL.Scheme = scheme
		manifestURLStr = parsedURL.String()
	}

	return manifestURLStr, originalHost, parsedURL, nil
}

// fetchDigest retrieves an image digest using the specified HTTP method.
//
// When a tag is supplied the manifest request targets that tag of the
// container's repository instead of the container's own reference, so a
// candidate release tag can be confirmed without pulling it. Authentication
// stays scoped to the container's repository, which every tag shares.
//
// When endpoints are provided, each mirror host is tried in order. An empty string
// endpoint means use the canonical registry host. If all endpoints fail, the last
// error is returned.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - container: Container whose digest is being retrieved.
//   - registryAuth: Base64-encoded auth string.
//   - settings: Registry request settings.
//   - limiter: Rate limiter shared by registry requests.
//   - method: HTTP method ("HEAD" or "GET").
//   - tag: Optional tag override for the manifest specifier.
//   - endpoints: Optional list of registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - string: Normalized digest.
//   - error: Non-nil if operation fails, nil on success.
func fetchDigest(log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	settings registryConfig.Registry,
	limiter *ratelimit.Limiter,
	method string,
	tag string,
	endpoints ...string,
) (string, error) {
	fields := map[string]any{
		"container": container.Name(),
		"image":     container.ImageName(),
	}

	// Skip digest fetching for locally built images (empty RepoDigests).
	if container.HasImageInfo() && len(container.ImageInfo().RepoDigests) == 0 {
		log.Debug().
			Fields(fields).
			Msg("Skipping digest fetch for locally built image")

		return "", nil
	}

	// Transform the provided auth string into a usable format for registry authentication.
	registryAuth = auth.TransformAuth(log, registryAuth)

	// Create an authentication client for registry requests.
	client := auth.NewAuthClient(log, settings)

	// Build the canonical manifest URL and apply lscr.io/host-override handling.
	manifestURL, originalHost, _, err := buildManifestURL(log, container, settings, "")
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to build manifest URL")

		return "", fmt.Errorf("failed to build manifest URL: %w", err)
	}

	log.Debug().
		Fields(fields).
		Str("original_host", originalHost).
		Str("manifest_url", manifestURL).
		Msg("Built manifest URL for container")

	// If no endpoints specified, use a single empty endpoint (canonical host).
	if len(endpoints) == 0 {
		endpoints = []string{""}
	}

	limitHost, hostErr := auth.GetRegistryAddress(log, container.ImageName())
	if hostErr != nil || limitHost == "" {
		log.Debug().
			Err(hostErr).
			Fields(fields).
			Msg("Failed to resolve registry host for rate limiting")
	}

	limitKey, release, holdErr := limiter.HoldAnonymous(ctx, limitHost, registryAuth != "")
	if holdErr != nil {
		return "", fmt.Errorf("%w: %w", errFailedGetToken, holdErr)
	}
	defer release()

	var lastErr error

	for _, endpoint := range endpoints {
		var epFields map[string]any

		if endpoint != "" {
			sanitized := "<redacted>"

			u, err := url.Parse(endpoint)
			if err == nil && u.Host != "" {
				sanitized = u.Host
			}

			epFields = map[string]any{
				"registry_endpoint": sanitized,
			}
		}

		result, err := limiter.DoValue(ctx, log, limitKey, func() (auth.TokenResult, error) {
			return auth.GetToken(log,
				ctx,
				container,
				registryAuth,
				client,
				settings,
				endpoint,
			)
		})
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Fields(epFields).
				Msg("Failed to get token from endpoint")
			lastErr = fmt.Errorf("%w: %w", errFailedGetToken, err)

			if ratelimit.Is(err) {
				return "", lastErr
			}

			continue
		}

		token := result.Token
		challengeHost := result.ChallengeHost
		redirected := result.Redirected
		redirectHost := result.RedirectHost

		if token == "" {
			log.Debug().
				Fields(fields).
				Fields(epFields).
				Msg("No authentication required, proceeding with request")
		} else {
			log.Debug().
				Fields(fields).
				Fields(epFields).
				Str("challenge_host", challengeHost).
				Bool("redirected", redirected).
				Str("redirect_host", redirectHost).
				Msg("Received challenge host and redirect flag from GetToken")
		}

		// Decide which host to use for this manifest request attempt.
		// Priority:
		// 1. Registry redirect host (from auth challenge, e.g. lscr.io → ghcr.io)
		// 2. Current mirror endpoint (when using registry mirrors)
		// 3. Canonical host (empty string)
		hostForManifest := ""
		if redirectHost != "" && redirectHost != originalHost && redirected {
			hostForManifest = redirectHost
		} else if endpoint != "" {
			// Use the current registry mirror endpoint for the manifest request.
			hostForManifest = endpoint
		}

		var (
			manifestURL string
			parsedURL   *url.URL
		)

		manifestURL, _, parsedURL, err = buildManifestURLForTag(log,
			container,
			settings,
			hostForManifest,
			tag,
		)
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Fields(epFields).
				Msg("Failed to build manifest URL")
			lastErr = err

			continue
		}

		log.Debug().
			Fields(fields).
			Fields(epFields).
			Str("method", method).
			Str("url", manifestURL).
			Msg("Fetching digest")

		var (
			digest     string
			updatedURL string
			retry      bool
		)

		err = limiter.Do(ctx, log, limitKey, func() error {
			req, reqErr := makeManifestRequest(ctx, method, manifestURL, token)
			if reqErr != nil {
				return reqErr
			}

			resp, doErr := client.Do(req)
			if doErr != nil {
				return fmt.Errorf("%w: %w", errFailedExecuteRequest, doErr)
			}

			var handleErr error

			digest, updatedURL, retry, handleErr = handleManifestResponse(log,
				resp,
				method,
				originalHost,
				challengeHost,
				redirected,
				parsedURL,
				parsedURL.Host,
			)
			_ = resp.Body.Close()

			return handleErr
		})
		if err != nil {
			log.Debug().
				Err(err).
				Fields(fields).
				Fields(epFields).
				Msg("Failed to handle manifest response")
			lastErr = err

			if ratelimit.Is(err) {
				return "", lastErr
			}

			continue
		}

		if retry && updatedURL != "" {
			log.Debug().
				Fields(fields).
				Fields(epFields).
				Str("retry_url", updatedURL).
				Msg("Retrying manifest request with updated URL")

			digest, err = retryManifestRequest(log,
				ctx,
				method,
				updatedURL,
				token,
				originalHost,
				challengeHost,
				redirected,
				parsedURL,
				limitKey,
				client,
				limiter,
			)
			if err != nil {
				log.Debug().
					Err(err).
					Fields(fields).
					Fields(epFields).
					Str("retry_url", updatedURL).
					Msg("Failed to retry manifest request")
				lastErr = err

				if ratelimit.Is(err) {
					return "", lastErr
				}

				continue
			}
		}

		log.Debug().
			Fields(fields).
			Fields(epFields).
			Str("remote_digest", digest).
			Msg("Fetched remote digest")

		return digest, nil
	}

	return "", lastErr
}

// handleManifestResponse processes the HTTP response, handles redirects, and extracts the digest.
//
// It checks for redirects, updates the manifest URL if necessary, and extracts the digest
// from the response headers or body based on the request method.
//
// Parameters:
//   - resp: The HTTP response from the manifest request.
//   - method: The HTTP method used ("HEAD" or "GET").
//   - originalHost: The original host from the initial manifest URL.
//   - challengeHost: The challenge host from authentication (empty if not redirected).
//   - redirected: Whether authentication was redirected.
//   - parsedURL: Parsed URL for updating host.
//   - currentHost: The current host being used for the request.
//
// Returns:
//   - string: The extracted and normalized digest.
//   - string: Updated manifest URL if redirected, otherwise empty.
//   - bool: Whether a retry is needed.
//   - error: Non-nil if processing or extraction fails, nil on success.
func handleManifestResponse(log *zerolog.Logger,
	resp *http.Response,
	method, originalHost, challengeHost string,
	redirected bool,
	parsedURL *url.URL,
	currentHost string,
) (string, string, bool, error) {
	fields := map[string]any{
		"method":         method,
		"status_code":    resp.StatusCode,
		"status":         resp.Status,
		"original_host":  originalHost,
		"challenge_host": challengeHost,
		"redirected":     redirected,
		"request_host":   resp.Request.URL.Host,
		"current_host":   currentHost,
	}

	log.Debug().
		Fields(fields).
		Msg("Handling manifest response")

	if resp.StatusCode == http.StatusTooManyRequests {
		body := ratelimit.ReadBody(resp, ratelimit.DefaultBodyLimit)

		log.Debug().
			Fields(fields).
			Msg("Registry returned 429 for manifest request")

		return "", "", false, ratelimit.FromResponse(resp, body)
	}

	var manifestURL string

	// Handle non-success responses for HEAD requests by returning empty digest to trigger GET fallback.
	// Exclude 404 Not Found to avoid unnecessary GET requests when the manifest doesn't exist.
	headFallbackCondition := method == http.MethodHead &&
		(resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) &&
		resp.StatusCode != http.StatusNotFound
	log.Debug().
		Fields(fields).
		Bool("head_fallback_condition", headFallbackCondition).
		Msg("Checking HEAD fallback condition")

	if headFallbackCondition {
		// For non-redirected registries, try challenge host first before falling back to GET
		if !redirected && challengeHost != "" && currentHost == originalHost &&
			(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized) {
			log.Debug().
				Fields(fields).
				Bool("retry_on_challenge", true).
				Msg("HEAD request failed on original host for non-redirected registry, trying challenge host")

			parsedURL.Host = challengeHost
			manifestURL = parsedURL.String()

			log.Debug().
				Fields(fields).
				Str("retry_url", manifestURL).
				Msg("Setting retry due to HEAD failure on original host")

			return "", manifestURL, true, nil
		}

		log.Debug().
			Fields(fields).
			Msg("HEAD request failed, returning empty digest to trigger GET fallback")

		return "", "", false, nil // Return empty to trigger GET fallback in CompareDigest
	}

	// Check for redirect status codes (3xx)
	redirectCondition := resp.StatusCode >= http.StatusMultipleChoices &&
		resp.StatusCode < http.StatusBadRequest
	log.Debug().
		Fields(fields).
		Bool("redirect_condition", redirectCondition).
		Msg("Checking redirect condition")

	if redirectCondition {
		// Handle manifest request redirects by updating URL to redirected host
		location := resp.Header.Get("Location")
		if location != "" {
			redirectURL, err := url.Parse(location)
			if err == nil && redirectURL.Host != "" && redirectURL.Host != currentHost {
				log.Debug().
					Fields(fields).
					Str("redirect_location", location).
					Str("redirect_host", redirectURL.Host).
					Msg("Manifest request redirected, updating URL host")

				parsedURL.Host = redirectURL.Host
				manifestURL = parsedURL.String()

				log.Debug().
					Fields(fields).
					Str("retry_url", manifestURL).
					Msg("Setting retry due to redirect")

				return "", manifestURL, true, nil
			}
		}
	}

	// Check for successful status code (only for GET requests, since HEAD is handled above).
	successCondition := resp.StatusCode >= http.StatusOK &&
		resp.StatusCode < http.StatusMultipleChoices
	log.Debug().
		Fields(fields).
		Bool("success_condition", successCondition).
		Msg("Checking success status condition")

	if !successCondition {
		// For HEAD requests, do not retry on 404 to avoid unnecessary GET fallback
		if method == http.MethodHead && resp.StatusCode == http.StatusNotFound {
			log.Debug().
				Fields(fields).
				Str("error", "HEAD request returned 404, not retrying").
				Msg("Response status not successful")

			return "", "", false, fmt.Errorf(
				"%w: %w: status %s",
				errManifestNotFound,
				errInvalidRegistryResponse,
				resp.Status,
			)
		}

		// Handle 401/404 errors on redirected hosts by retrying on original host
		if (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized) &&
			currentHost != originalHost {
			log.Debug().
				Fields(fields).
				Bool("retry_on_original", true).
				Msg("401/404 on redirected host, retrying on original host")

			parsedURL.Host = originalHost
			manifestURL = parsedURL.String()

			log.Debug().
				Fields(fields).
				Str("retry_url", manifestURL).
				Msg("Setting retry due to 401/404 on redirected host")

			return "", manifestURL, true, nil
		}

		// If we're on original host and have challenge host, try challenge host
		if (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized) &&
			challengeHost != "" && currentHost == originalHost {
			log.Debug().
				Fields(fields).
				Bool("retry_on_challenge", true).
				Msg("401/404 on original host, trying challenge host")

			parsedURL.Host = challengeHost
			manifestURL = parsedURL.String()

			log.Debug().
				Fields(fields).
				Str("retry_url", manifestURL).
				Msg("Setting retry due to 401/404 on original host with challenge host")

			return "", manifestURL, true, nil
		}

		log.Debug().
			Fields(fields).
			Str("error", "invalid status code").
			Msg("Response status not successful")

		return "", "", false, fmt.Errorf("%w: status %s", errInvalidRegistryResponse, resp.Status)
	}

	// Extract the digest based on the request method (HEAD from headers, GET from body).
	var (
		digest string
		err    error
	)

	log.Debug().
		Fields(fields).
		Msg("Extracting digest")

	if method == http.MethodHead {
		digest, err = extractHeadDigest(log, resp)
	} else {
		digest, err = extractGetDigest(log, resp)
	}

	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to extract digest")

		return "", "", false, err
	}

	log.Debug().
		Fields(fields).
		Str("extracted_digest", digest).
		Msg("Successfully extracted digest")

	return digest, "", false, nil
}

// extractHeadDigest extracts the image digest from a HEAD response's headers.
//
// It retrieves the digest from the "Docker-Content-Digest" header, normalizing it for comparison,
// and validates its presence to ensure a valid response from the registry.
//
// Parameters:
//   - resp: The HTTP response from a HEAD request containing headers.
//
// A header with no hash after its algorithm, such as "sha256:", yields an
// empty digest and no error, which asks the caller to fall back to a GET
// request, as a non-success HEAD response does.
//
// Returns:
//   - string: The normalized digest (e.g., "abc..." without "sha256:"), or empty
//     when the header has no hash.
//   - error: An error if the digest header is missing, nil otherwise.
func extractHeadDigest(log *zerolog.Logger, resp *http.Response) (string, error) {
	// Retrieve the digest from the standard header.
	header := resp.Header.Get(ContentDigestHeader)
	if header == "" {
		// Log and return an error if the digest is missing, including auth details for debugging.
		wwwAuthHeader := resp.Header.Get("WWW-Authenticate")
		log.Debug().
			Str("status", resp.Status).
			Str("auth_header", wwwAuthHeader).
			Msg("Registry responded with invalid HEAD request")

		return "", fmt.Errorf(
			"%w: status %q, auth: %q",
			errInvalidRegistryResponse,
			resp.Status,
			wwwAuthHeader,
		)
	}

	// Normalize the digest (e.g., strip "sha256:") for consistency.
	digest := NormalizeDigest(log, header)
	if digest == "" {
		log.Debug().
			Str("status", resp.Status).
			Msg("HEAD response digest has no hash, falling back to GET")

		return "", nil
	}

	log.Debug().
		Str("digest", digest).
		Msg("Extracted digest from HEAD response")

	return digest, nil
}

// extractGetDigest extracts the image digest from a GET response's headers or body.
//
// It first tries to retrieve the digest from the "Docker-Content-Digest" header.
// If the header is missing, it falls back to parsing the response body as a JSON
// manifest or as a plain text digest for non-standard registries.
// The fallback body is limited to maxManifestSize.
// When attempting JSON parsing, the Content-Type header must contain "application/json",
// "application/vnd.oci", or "application/vnd.docker".
// The digest is normalized for consistency.
//
// Parameters:
//   - log: Logger for debug output.
//   - resp: The HTTP response from a GET request containing headers and body.
//
// Returns:
//   - string: The normalized digest (e.g., "abc..." without "sha256:") if present.
//   - error: An error if the digest cannot be extracted, nil if successful.
func extractGetDigest(log *zerolog.Logger, resp *http.Response) (string, error) {
	// First, try to retrieve the digest from the standard header, normalized
	// (e.g., "sha256:" stripped) for consistency. A header with no hash after
	// its algorithm carries no digest, so the body is tried as for a missing one.
	headerDigest := NormalizeDigest(log, resp.Header.Get(ContentDigestHeader))
	if headerDigest != "" {
		log.Debug().
			Str("digest", headerDigest).
			Msg("Extracted digest from GET response header")

		return headerDigest, nil
	}

	// Fallback: read the body with a size limit. The extra byte detects overflow.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestSize+1))
	if err != nil {
		log.Debug().
			Err(err).
			Str("status", resp.Status).
			Msg("Failed to read response body for digest extraction")

		return "", fmt.Errorf(
			"%w: failed to read response body: %w",
			errInvalidRegistryResponse,
			err,
		)
	}

	// Reject oversized bodies before parsing or converting to a string.
	if len(bodyBytes) > maxManifestSize {
		log.Debug().
			Str("status", resp.Status).
			Int("observed_size", len(bodyBytes)).
			Int("limit", maxManifestSize).
			Msg("Digest response body exceeds size limit")

		return "", fmt.Errorf(
			"%w: %d bytes exceeds limit of %d bytes",
			errManifestTooLarge,
			len(bodyBytes),
			maxManifestSize,
		)
	}

	bodyStr := strings.TrimSpace(string(bodyBytes))
	if bodyStr == "" {
		log.Debug().
			Str("status", resp.Status).
			Msg("Response body is empty, no digest found")

		return "", fmt.Errorf(
			"%w: missing digest header and empty body",
			errInvalidRegistryResponse,
		)
	}

	// Check if the response body starts with JSON indicators ('{' or '[') before attempting JSON unmarshaling.
	if strings.HasPrefix(bodyStr, "{") || strings.HasPrefix(bodyStr, "[") {
		// Check Content-Type for JSON parsing.
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") &&
			!strings.Contains(contentType, "application/vnd.oci") &&
			!strings.Contains(contentType, "application/vnd.docker") {
			return "", fmt.Errorf(
				"%w: unsupported content type for JSON parsing: %s",
				errInvalidRegistryResponse,
				contentType,
			)
		}
		// Try to parse as JSON manifest first (standard OCI/Docker format).
		// Define a struct to hold the expected JSON structure with a digest field.
		var manifest struct {
			Digest string `json:"digest"`
		}
		// Attempt to unmarshal the response body as JSON.
		jsonErr := json.Unmarshal(bodyBytes, &manifest)
		if jsonErr == nil {
			// JSON unmarshaling succeeded, check if digest field contains a value.
			// A digest field with no hash after its algorithm is as empty as a missing one.
			normalizedDigest := NormalizeDigest(log, manifest.Digest)
			if normalizedDigest != "" {
				log.Debug().
					Str("digest", normalizedDigest).
					Msg("Extracted digest from JSON manifest")

				return normalizedDigest, nil
			}
			// JSON parsed successfully, but the digest field is missing or has no hash.
			log.Debug().
				Str("status", resp.Status).
				Str("body", bodyStr).
				Msg("JSON manifest parsed but digest field is empty")

			return "", fmt.Errorf("%w: empty digest in JSON manifest", errInvalidRegistryResponse)
		}
		// JSON parsing failed, log metadata for debugging (avoid exposing potentially sensitive content).
		log.Debug().
			Err(jsonErr).
			Str("status", resp.Status).
			Int("body_length", len(bodyStr)).
			Str("content_type", resp.Header.Get("Content-Type")).
			Msg("Failed to parse response body as JSON manifest")
	}

	// Final fallback: Try to parse as plain text digest for non-standard registries.
	// Validate that the body looks like a digest (starts with sha256: prefix and has reasonable length).
	if !strings.HasPrefix(bodyStr, "sha256:") || len(bodyStr) < 20 {
		log.Debug().
			Str("status", resp.Status).
			Int("body_length", len(bodyStr)).
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("body", bodyStr).
			Msg("Response body does not appear to be a valid digest")

		return "", fmt.Errorf("%w: invalid digest format in body", errInvalidRegistryResponse)
	}

	// Normalize the digest from the plain text body.
	normalizedDigest := NormalizeDigest(log, bodyStr)
	log.Debug().
		Str("digest", normalizedDigest).
		Msg("Extracted digest from plain text body")

	return normalizedDigest, nil
}

// digestsMatch compares a list of local digests with a remote digest to determine if there's a match.
//
// It normalizes both the remote digest and each local digest, checking for equality to confirm
// whether the container's image is up-to-date with the registry's latest version.
//
// Parameters:
//   - localDigests: A slice of local digests from the container's image info (e.g., "sha256:abc...").
//   - remoteDigest: The digest fetched from the registry (e.g., "sha256:abc..." or raw hash).
//
// Returns:
//   - bool: True if any normalized local digest matches the normalized remote digest, false
//     otherwise, including when the remote digest is empty.
func digestsMatch(log *zerolog.Logger, localDigests []string, remoteDigest string) bool {
	// Normalize the remote digest once for efficiency.
	normalizedRemoteDigest := NormalizeDigest(log, remoteDigest)

	// An empty remote digest identifies no image, so it matches nothing.
	if normalizedRemoteDigest == "" {
		log.Debug().
			Strs("local_digests", localDigests).
			Msg("Remote digest is empty, so no local digest matches")

		return false
	}

	log.Debug().
		Strs("local_digests", localDigests).
		Str("remote_digest", normalizedRemoteDigest).
		Msg("Comparing digests")

	for _, digest := range localDigests {
		// Cut the digest into repo and hash parts (e.g., "repo@sha256:abc").
		repo, after, found := strings.Cut(digest, "@")

		// Skip malformed digests without @ separator.
		if !found {
			log.Debug().
				Str("digest", digest).
				Msg("Skipping malformed digest without @ separator")

			continue
		}

		// Handle case where digest starts with "@" (e.g., "@sha256:abc123")
		// This is a valid format that Docker may report in some contexts.
		if repo == "" {
			log.Debug().
				Str("digest", digest).
				Str("remote_digest", normalizedRemoteDigest).
				Msg("Local digest has empty repo prefix, comparing only digest part")
		}

		// Remove the sha256: prefix, if needed.
		normalizedLocalDigest := NormalizeDigest(log, after)

		// Return true on the first match.
		if normalizedLocalDigest == normalizedRemoteDigest {
			log.Debug().
				Str("local_digest", digest).
				Str("remote_digest", normalizedRemoteDigest).
				Msg("Found digest match")

			return true
		}
	}

	return false
}

// makeManifestRequest creates an HTTP request for fetching the manifest with proper headers and authentication.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - method: HTTP method ("HEAD" or "GET").
//   - manifestURL: The URL to request the manifest from.
//   - token: Authentication token (empty if not required).
//
// Returns:
//   - *http.Request: The constructed HTTP request.
//   - error: Non-nil if request creation fails, nil on success.
func makeManifestRequest(
	ctx context.Context,
	method, manifestURL, token string,
) (*http.Request, error) {
	// Construct the HTTP request with the appropriate method, headers, and context.
	req, err := http.NewRequestWithContext(ctx, method, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errFailedCreateRequest, err)
	}

	// Set headers only if a token is provided.
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	// Set Accept header for Docker Distribution API manifest requests, supporting v1, v2, OCI v1, and OCI index.
	req.Header.Set(
		"Accept",
		"application/vnd.docker.distribution.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json",
	)
	req.Header.Set("User-Agent", meta.UserAgent)

	return req, nil
}

// retryManifestRequest performs a retry request to the manifest URL with updated host and returns the digest.
//
// Parameters:
//   - ctx: Context for request lifecycle control.
//   - method: HTTP method ("HEAD" or "GET").
//   - updatedURL: The updated manifest URL to retry.
//   - token: Authentication token.
//   - originalHost: The original host.
//   - challengeHost: The challenge host.
//   - redirected: Whether authentication was redirected.
//   - parsedURL: Parsed URL object.
//   - limitKey: Rate-limit host key from [ratelimit.Scope]. Empty uses parsedURL.Host.
//   - client: The HTTP client to use for the request.
//   - limiter: Rate limiter shared by registry requests.
//
// Returns:
//   - string: The extracted digest, or empty for a HEAD request that should fall back to GET.
//   - error: Non-nil if the retry request fails, or a GET retry asks for yet another host.
func retryManifestRequest(
	log *zerolog.Logger,
	ctx context.Context,
	method, updatedURL, token string,
	originalHost, challengeHost string,
	redirected bool,
	parsedURL *url.URL,
	limitKey string,
	client auth.Client,
	limiter *ratelimit.Limiter,
) (string, error) {
	var digest string

	if limitKey == "" {
		limitKey = parsedURL.Host
	}

	err := limiter.Do(ctx, log, limitKey, func() error {
		req, reqErr := makeManifestRequest(ctx, method, updatedURL, token)
		if reqErr != nil {
			return reqErr
		}

		resp, doErr := client.Do(req)
		if doErr != nil {
			return fmt.Errorf("%w: %w", errFailedExecuteRequest, doErr)
		}

		defer func() { _ = resp.Body.Close() }()

		got, nextURL, retry, handleErr := handleManifestResponse(log,
			resp,
			method,
			originalHost,
			challengeHost,
			redirected,
			parsedURL,
			parsedURL.Host,
		)
		if handleErr != nil {
			return handleErr
		}

		// The request is retried only once. A HEAD request falls back to GET
		// with an empty digest. A GET request is the last attempt, and an
		// empty digest would mark the image as out of date.
		if retry && method != http.MethodHead {
			return fmt.Errorf("%w: %s", errUnresolvedManifestRetry, nextURL)
		}

		digest = got

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("manifest retry: %w", err)
	}

	return digest, nil
}
