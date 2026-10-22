package release

import (
	"errors"
	"fmt"

	"github.com/distribution/reference"
)

// errInvalidImageRef indicates an image reference could not be parsed.
var errInvalidImageRef = errors.New("invalid image reference")

// splitReference separates an image reference into its registry host and
// repository path.
//
// A reference without an explicit registry uses the Docker Hub default, which
// matches how the digest package builds manifest URLs.
//
// Parameters:
//   - imageName: Image reference to split.
//
// Returns:
//   - string: Registry host, or empty when the reference cannot be parsed.
//   - string: Repository path, or empty when the reference cannot be parsed.
//   - error: Non-nil when the reference cannot be parsed.
func splitReference(imageName string) (string, string, error) {
	named, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", errInvalidImageRef, err)
	}

	host := reference.Domain(named)
	path := reference.Path(named)

	return host, path, nil
}

// isDigestPinned reports whether a reference names a digest rather than a tag.
//
// A digest-pinned image cannot be re-tagged, so no release tag would match it
// and the lookup would always miss.
//
// Parameters:
//   - imageName: Image reference to inspect.
//
// Returns:
//   - bool: True when the reference is pinned to a digest.
//   - error: Non-nil when the reference cannot be parsed.
func isDigestPinned(imageName string) (bool, error) {
	named, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return false, fmt.Errorf("%w: %w", errInvalidImageRef, err)
	}

	_, ok := named.(reference.Canonical)

	return ok, nil
}
