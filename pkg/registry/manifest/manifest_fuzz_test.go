package manifest

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzBuildManifestURLForImage checks that a manifest URL built for an image
// reference parses as a /v2/ manifest URL with the requested scheme and no
// query or fragment, and that a reference that cannot be used is a parse or
// missing-tag error.
//
// The seeds cover valid references on Docker Hub, GHCR, and registries with
// ports, digest references, malformed and empty strings, whitespace and
// control characters, and very long or deep references.
func FuzzBuildManifestURLForImage(f *testing.F) {
	// Seed with valid image references
	f.Add("ghcr.io/nicholas-fedor/watchtower:mytag")
	f.Add("nickfedor/watchtower:latest")
	f.Add("nginx:latest")
	f.Add("localhost:5000/repo/image:tag")
	f.Add("docker.io/library/alpine:v3.14")
	f.Add("registry.example.com/user/repo:1.0.0")

	// Seed with malformed strings
	f.Add("invalid:image:ref:!!")
	f.Add(
		"docker-registry.domain/imagename@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
	)
	f.Add("")
	f.Add(":::")
	f.Add("image@digest")
	f.Add("registry.com:port/image")

	// Seed with edge cases
	f.Add("image:tag")              // No registry
	f.Add("REGISTRY.COM/IMAGE:TAG") // Uppercase
	f.Add("very-long-registry-name.with.many.subdomains.com/repo/image:tag")
	f.Add("localhost/image")                                    // No tag, assumes latest
	f.Add("image")                                              // Just image name
	f.Add("registry.com:5000/image:tag")                        // Port number
	f.Add("192.168.1.1:5000/repo/image:v1.0")                   // IP address
	f.Add("registry.com/image:tag:with:colons")                 // Multiple colons in tag
	f.Add("registry.com/image@sha256:abc123")                   // Digest instead of tag
	f.Add(strings.Repeat("a", 1000) + ":tag")                   // Very long image name
	f.Add("registry.com/" + strings.Repeat("a", 1000) + ":tag") // Very long repo path

	// Seed with additional edge cases for robustness
	f.Add("")                                                      // Empty string
	f.Add("@")                                                     // Single @
	f.Add("image@")                                                // Trailing @
	f.Add("image@sha256:")                                         // Empty digest
	f.Add("registry/image@sha256:")                                // Empty digest with registry
	f.Add("image@@sha256:abc")                                     // Multiple @
	f.Add("image@sha256:abc@def")                                  // Multiple @ in digest
	f.Add("registry.com/image@sha256:" + strings.Repeat("a", 64))  // Long digest
	f.Add("registry.com/image@sha512:" + strings.Repeat("a", 128)) // SHA512 digest
	f.Add("\t")                                                    // Tab character
	f.Add("\n")                                                    // Newline character
	f.Add(" ")                                                     // Space
	f.Add("image:tag\nwith:newline")                               // Newline in tag
	f.Add("image/with/ multiple /spaces")                          // Spaces in path
	f.Add("registry.com:5000")                                     // Registry with port, no image
	f.Add("registry.com:5000/")                                    // Registry with port and trailing slash
	f.Add("docker.io/library")                                     // Docker Hub library without image
	f.Add(strings.Repeat("a", 10000))                              // Very long single word
	f.Add("a" + strings.Repeat("/a", 100) + ":tag")                // Very deep path
	f.Add("registry.com/image:" + strings.Repeat("v", 100))        // Very long tag
	f.Add("image:tag-with-many-dashes-and_underscores-123")        // Complex tag
	f.Add("registry.com/image-name.with.dots:tag-name.with.dots")  // Dots in name and tag

	f.Fuzz(func(t *testing.T, imageRef string) {
		got, err := BuildManifestURLForImage(new(zerolog.Nop()), imageRef, "fuzz-container", "https")
		if err != nil {
			assert.True(t,
				errors.Is(err, errFailedParseImageName) || errors.Is(err, errMissingTag),
				"unexpected error: %v", err)
			assert.Empty(t, got)

			return
		}

		u, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, "https", u.Scheme)
		assert.True(t, strings.HasPrefix(u.Path, "/v2/"), "path %q is not under /v2/", u.Path)
		assert.Contains(t, u.Path, "/manifests/")
		assert.Empty(t, u.RawQuery)
		assert.Empty(t, u.Fragment)
	})
}
