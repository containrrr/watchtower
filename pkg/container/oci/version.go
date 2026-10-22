package oci

import (
	"strings"

	"golang.org/x/mod/semver"
)

// LooksLikeTag reports whether version looks like a semver tag.
//
// Parameters:
//   - version: Raw OCI image version string.
//
// Returns:
//   - bool: True when version is valid semver with or without a v prefix.
func LooksLikeTag(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}

	// Accept both v1.2.3 and 1.2.3. Operators stamp either form.
	return semver.IsValid(version) || semver.IsValid("v"+version)
}
