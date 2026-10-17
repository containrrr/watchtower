// Package manifest provides functionality for constructing URLs to access container
// image manifests in Watchtower. It handles parsing image references and building
// registry-specific manifest URLs for digest retrieval and other operations.
package manifest

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
)

// Errors for manifest operations.
var (
	// errMissingTag indicates the parsed image reference lacks both a tag and a digest.
	errMissingTag = errors.New("parsed container image reference has no tag")
	// errFailedParseImageName indicates a failure to parse the container's image name.
	errFailedParseImageName = errors.New("failed to parse image name")
)

const manifestRefSplitParts = 2

// parseImageRef parses the container image name into a normalized reference.
//
// It uses reference.ParseDockerRef to normalize the image name, handling Docker Hub
// shorthand and validating the reference format.
//
// Parameters:
//   - imageName: Container image name string (e.g., "nginx:latest", "registry.example.com/org/image@sha256:abc...").
//
// Returns:
//   - reference.Reference: Parsed and normalized image reference.
//   - error: Non-nil if parsing fails, nil on success.
func parseImageRef(imageName string) (reference.Reference, error) {
	normalizedRef, err := reference.ParseDockerRef(imageName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errFailedParseImageName, err)
	}

	return normalizedRef, nil
}

// resolveRegistryHost extracts the registry host from a parsed image reference name.
//
// It delegates to auth.GetRegistryAddress to determine the registry host, handling
// Docker Hub canonicalization and custom registry extraction.
//
// Parameters:
//   - name: The image reference name portion (e.g., "registry.example.com/org/image").
//
// Returns:
//   - string: The registry host (e.g., "registry.example.com" or "index.docker.io").
func resolveRegistryHost(log *zerolog.Logger, name string) string {
	host, _ := auth.GetRegistryAddress(log, name)

	return host
}

// manifestPathComponents holds the extracted path and specifier (tag or digest)
// for a manifest URL.
type manifestPathComponents struct {
	imagePath string
	specifier string
}

// tagComponents extracts the image path and tag from a tag-based image reference.
//
// It retrieves the repository path and tag from a reference.NamedTagged, returning
// the components needed to construct a manifest URL.
//
// Parameters:
//   - tagged: Parsed image reference that implements reference.NamedTagged.
//
// Returns:
//   - manifestPathComponents: Struct containing imagePath and specifier (tag).
func tagComponents(tagged reference.NamedTagged) manifestPathComponents {
	return manifestPathComponents{
		imagePath: reference.Path(tagged),
		specifier: tagged.Tag(),
	}
}

// digestComponents extracts the image path and digest from a digest-only image reference.
//
// It splits the reference string on "@" to separate the image name from the digest,
// then validates the name portion and returns the normalized path and digest specifier.
//
// Parameters:
//   - digested: Parsed image reference that implements reference.Digested.
//
// Returns:
//   - manifestPathComponents: Struct containing imagePath and specifier (digest).
//   - error: Non-nil if the name portion fails to parse, nil on success.
func digestComponents(digested reference.Digested) (manifestPathComponents, error) {
	refStr := digested.String()
	parts := strings.SplitN(refStr, "@", manifestRefSplitParts)
	namePart := parts[0]
	digestStr := digested.Digest().String()

	namedRef, nameErr := reference.WithName(namePart)
	if nameErr != nil {
		return manifestPathComponents{}, fmt.Errorf("%w: %w", errFailedParseImageName, nameErr)
	}

	return manifestPathComponents{
		imagePath: reference.Path(namedRef),
		specifier: digestStr,
	}, nil
}

// setManifestURLFields logs the common fields for a parsed image reference.
//
// It populates a map[string]any map with host, image_path, scheme, and optionally
// tag or digest, then emits a debug log entry for tracing URL construction.
//
// Parameters:
//   - fields: Base log fields to extend (typically includes container and image).
//   - host: Registry host for the manifest request.
//   - imagePath: Repository path within the registry (e.g., "library/nginx").
//   - scheme: URL scheme ("https" or "http").
//   - tag: Tag specifier if present, empty string otherwise.
//   - digest: Digest specifier if present, empty string otherwise.
func setManifestURLFields(log *zerolog.Logger, fields map[string]any, host, imagePath, scheme, tag, digest string) {
	urlFields := map[string]any{
		"host":       host,
		"image_path": imagePath,
		"scheme":     scheme,
	}

	if tag != "" {
		urlFields["tag"] = tag
	}

	if digest != "" {
		urlFields["digest"] = digest
	}

	log.Debug().
		Fields(fields).
		Fields(urlFields).
		Msg("Constructed manifest URL components")
}

// manifestURLPath returns the v2 manifest API path for a given image path and specifier.
//
// It formats the path according to the Docker Registry API v2 specification:
// "/v2/<imagePath>/manifests/<specifier>".
//
// Parameters:
//   - imagePath: Repository path within the registry (e.g., "library/nginx").
//   - specifier: Tag or digest value to identify the manifest.
//
// Returns:
//   - string: The formatted manifest path (e.g., "/v2/library/nginx/manifests/latest").
func manifestURLPath(imagePath, specifier string) string {
	return fmt.Sprintf("/v2/%s/manifests/%s", imagePath, specifier)
}

// BuildManifestURLForImage constructs a manifest URL for an image reference.
//
// It accepts both tag-based references (for example "nginx:latest") and digest-only
// references (for example "registry.example.com/org/image@sha256:abc..."). For tag-based
// references, the tag becomes the manifest path specifier. For digest-only references,
// the digest becomes the manifest path specifier. The registry host is derived from the
// image reference itself.
//
// The function does not support host overrides. Callers that need to target registry mirrors
// apply overrides after obtaining the canonical URL, as the digest package does.
//
// Parameters:
//   - log: Logger.
//   - imageName: Image reference supplying the registry host, repository, and specifier.
//   - containerName: Container name used only for log context.
//   - scheme: The scheme to use for the URL (for example, "https" or "http").
//
// Returns:
//   - string: Manifest URL using the canonical registry host from the image reference.
//   - error: Non-nil if parsing or tagging fails, nil on success.
func BuildManifestURLForImage(
	log *zerolog.Logger,
	imageName,
	containerName,
	scheme string,
) (string, error) {
	// Set up logging fields for consistent tracking.
	fields := map[string]any{
		"container": containerName,
		"image":     imageName,
	}

	// Parse the image name into a normalized reference for reliable processing.
	normalizedRef, err := parseImageRef(imageName)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to parse image name")

		return "", err
	}

	tagged, ok := normalizedRef.(reference.NamedTagged)
	if ok {
		return buildTaggedManifestURL(log, fields, tagged, scheme)
	}

	digested, ok := normalizedRef.(reference.Digested)
	if ok {
		return buildDigestedManifestURL(log, fields, digested, scheme)
	}

	log.Debug().
		Fields(fields).
		Str("ref", normalizedRef.String()).
		Msg("Missing tag/digest in image reference")

	return "", fmt.Errorf("%w: %s", errMissingTag, normalizedRef.String())
}

// buildTaggedManifestURL constructs a manifest URL for a tag-based image reference.
//
// It resolves the registry host, extracts the image path and tag, logs the components,
// and returns the fully qualified manifest URL string.
//
// Parameters:
//   - fields: Log fields for tracing the URL construction.
//   - tagged: Parsed image reference implementing reference.NamedTagged.
//   - scheme: URL scheme ("https" or "http").
//
// Returns:
//   - string: Fully qualified manifest URL (e.g., "https://registry.example.com/v2/org/image/manifests/latest").
//   - error: Non-nil if URL construction fails, nil on success.
func buildTaggedManifestURL(log *zerolog.Logger, fields map[string]any, tagged reference.NamedTagged, scheme string) (string, error) {
	host := resolveRegistryHost(log, tagged.Name())
	components := tagComponents(tagged)
	setManifestURLFields(log, fields, host, components.imagePath, scheme, components.specifier, "")

	return manifestURLString(host, scheme, manifestURLPath(components.imagePath, components.specifier)), nil
}

// buildDigestedManifestURL constructs a manifest URL for a digest-only image reference.
//
// It extracts the image path and digest from the reference, resolves the registry host
// from the name portion (before "@"), logs the components, and returns the fully
// qualified manifest URL string.
//
// Parameters:
//   - fields: Log fields for tracing the URL construction.
//   - digested: Parsed image reference implementing reference.Digested.
//   - scheme: URL scheme ("https" or "http").
//
// Returns:
//   - string: Fully qualified manifest URL (e.g., "https://registry.example.com/v2/org/image/manifests/sha256:abc...").
//   - error: Non-nil if component extraction fails, nil on success.
func buildDigestedManifestURL(log *zerolog.Logger, fields map[string]any, digested reference.Digested, scheme string) (string, error) {
	components, err := digestComponents(digested)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to extract digest components")

		return "", err
	}

	namePart, _, _ := strings.Cut(digested.String(), "@")
	host := resolveRegistryHost(log, namePart)
	setManifestURLFields(log, fields, host, components.imagePath, scheme, "", components.specifier)

	return manifestURLString(host, scheme, manifestURLPath(components.imagePath, components.specifier)), nil
}

// manifestURLString builds the manifest URL string from host, scheme, and path.
//
// It constructs a url.URL and returns its string representation.
//
// Parameters:
//   - host: Registry host (e.g., "registry.example.com").
//   - scheme: URL scheme ("https" or "http").
//   - path: Manifest API path (e.g., "/v2/library/nginx/manifests/latest").
//
// Returns:
//   - string: Fully qualified manifest URL.
func manifestURLString(host, scheme, path string) string {
	manifestURL := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   path,
	}

	return manifestURL.String()
}
