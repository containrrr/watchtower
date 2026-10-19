package manifest

import (
	"strings"
	"testing"

	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unnamedDigested is a [reference.Digested] whose String returns an empty
// name, which no parsed reference can produce, to reach the error paths for
// invalid names.
type unnamedDigested struct {
	// digest is the reference's digest.
	digest digest.Digest
}

// String returns the empty reference name.
//
// Returns:
//   - string: The empty string.
func (u unnamedDigested) String() string {
	return ""
}

// Digest returns the reference's digest.
//
// Returns:
//   - digest.Digest: The digest the stub was built with.
func (u unnamedDigested) Digest() digest.Digest {
	return u.digest
}

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

// TestParseImageRef verifies which image references parse and which are rejected.
func TestParseImageRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid tagged reference",
			input:   "nginx:latest",
			wantErr: false,
		},
		{
			name:    "valid digested reference",
			input:   "registry.example.com/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
			wantErr: false,
		},
		{
			name:    "invalid reference format",
			input:   "invalid:image:ref:!!",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only colons",
			input:   ":::",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseImageRef(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestResolveRegistryHost verifies the registry host resolved for references
// with and without an explicit registry.
func TestResolveRegistryHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "fully qualified registry",
			input:    "ghcr.io/org/image",
			expected: "ghcr.io",
		},
		{
			name:     "registry with port",
			input:    "localhost:5000/image",
			expected: "localhost:5000",
		},
		{
			name:     "Docker Hub implicit",
			input:    "library/nginx",
			expected: "index.docker.io",
		},
		{
			name:     "subdomain registry",
			input:    "registry.example.com/image",
			expected: "registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := resolveRegistryHost(newTestLogger(t), tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestManifestURLPath verifies the manifest path built from an image path and
// a tag or digest specifier.
func TestManifestURLPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		imagePath string
		specifier string
		expected  string
	}{
		{
			name:      "simple image path",
			imagePath: "nginx",
			specifier: "latest",
			expected:  "/v2/nginx/manifests/latest",
		},
		{
			name:      "namespaced image",
			imagePath: "org/image",
			specifier: "v1.0.0",
			expected:  "/v2/org/image/manifests/v1.0.0",
		},
		{
			name:      "digest specifier",
			imagePath: "image",
			specifier: "sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
			expected:  "/v2/image/manifests/sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := manifestURLPath(tt.imagePath, tt.specifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestManifestURLString verifies the manifest URL built from a host, scheme,
// and path.
func TestManifestURLString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		scheme   string
		path     string
		expected string
	}{
		{
			name:     "HTTPS URL",
			host:     "ghcr.io",
			scheme:   "https",
			path:     "/v2/org/image/manifests/latest",
			expected: "https://ghcr.io/v2/org/image/manifests/latest",
		},
		{
			name:     "HTTP URL",
			host:     "localhost:5000",
			scheme:   "http",
			path:     "/v2/image/manifests/tag",
			expected: "http://localhost:5000/v2/image/manifests/tag",
		},
		{
			name:     "URL with subdomain",
			host:     "registry.example.com",
			scheme:   "https",
			path:     "/v2/org/image/manifests/sha256:abc123",
			expected: "https://registry.example.com/v2/org/image/manifests/sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := manifestURLString(tt.host, tt.scheme, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTagComponents verifies the image path and tag taken from a tagged reference.
func TestTagComponents(t *testing.T) {
	t.Parallel()

	ref, err := reference.ParseDockerRef("registry.example.com/org/image:v1.0.0")
	require.NoError(t, err)

	tagged, ok := ref.(reference.NamedTagged)
	require.True(t, ok, "Reference should be NamedTagged")

	components := tagComponents(tagged)

	assert.Equal(t, "org/image", components.imagePath)
	assert.Equal(t, "v1.0.0", components.specifier)
}

// TestDigestComponents verifies the image path and digest taken from a
// digested reference.
func TestDigestComponents(t *testing.T) {
	t.Parallel()

	ref, err := reference.ParseDockerRef(
		"registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
	)
	require.NoError(t, err)

	digested, ok := ref.(reference.Digested)
	require.True(t, ok, "Reference should be Digested")

	components, err := digestComponents(digested)
	require.NoError(t, err)
	assert.Equal(t, "org/image", components.imagePath)
	assert.Equal(t, "sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7", components.specifier)
}

// TestDigestComponentsInvalidString verifies that a digested reference whose
// name cannot be parsed is an error.
func TestDigestComponentsInvalidString(t *testing.T) {
	t.Parallel()

	digestVal, err := digest.Parse("sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)

	components, err := digestComponents(unnamedDigested{digest: digestVal})
	require.Error(t, err)
	assert.Empty(t, components)
}

// TestBuildTaggedManifestURL verifies the manifest URL built for a tagged reference.
func TestBuildTaggedManifestURL(t *testing.T) {
	t.Parallel()

	ref, err := reference.ParseDockerRef("ghcr.io/org/image:mytag")
	require.NoError(t, err)

	tagged, ok := ref.(reference.NamedTagged)
	require.True(t, ok, "Reference should be NamedTagged")

	fields := map[string]any{
		"container": "test-container",
		"image":     "ghcr.io/org/image:mytag",
	}

	url, err := buildTaggedManifestURL(newTestLogger(t), fields, tagged, "https")
	require.NoError(t, err)

	expected := "https://ghcr.io/v2/org/image/manifests/mytag"
	assert.Equal(t, expected, url)
}

// TestBuildDigestedManifestURL verifies the manifest URL built for a digested reference.
func TestBuildDigestedManifestURL(t *testing.T) {
	t.Parallel()

	ref, err := reference.ParseDockerRef("registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7")
	require.NoError(t, err)

	digested, ok := ref.(reference.Digested)
	require.True(t, ok, "Reference should be Digested")

	fields := map[string]any{
		"container": "test-container",
		"image":     "registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
	}

	url, err := buildDigestedManifestURL(newTestLogger(t), fields, digested, "https")
	require.NoError(t, err)

	expected := "https://registry.example.com/v2/org/image/manifests/sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7"
	assert.Equal(t, expected, url)
}

// TestBuildDigestedManifestURLInvalidComponents verifies that a digested
// reference whose name cannot be parsed yields no URL.
func TestBuildDigestedManifestURLInvalidComponents(t *testing.T) {
	t.Parallel()

	digestVal, err := digest.Parse("sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)

	fields := map[string]any{
		"container": "test-container",
		"image":     "invalid",
	}

	url, err := buildDigestedManifestURL(newTestLogger(t), fields, unnamedDigested{digest: digestVal}, "https")
	require.Error(t, err)
	assert.Empty(t, url)
}

// TestBuildManifestURLForImage verifies the manifest URL built for image
// references across registries, tags, digests, and schemes, and the references
// it rejects.
func TestBuildManifestURLForImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		imageRef string
		scheme   string
		want     string
		wantErr  bool
	}{
		{
			name:     "should return a valid url given a fully qualified image",
			imageRef: "ghcr.io/nicholas-fedor/watchtower:mytag",
			scheme:   "https",
			want:     "https://ghcr.io/v2/nicholas-fedor/watchtower/manifests/mytag",
		},
		{
			name:     "should assume Docker Hub for image refs with no explicit registry",
			imageRef: "nickfedor/watchtower:latest",
			scheme:   "https",
			want:     "https://index.docker.io/v2/nickfedor/watchtower/manifests/latest",
		},
		{
			name:     "should assume latest for image refs with no explicit tag",
			imageRef: "nickfedor/watchtower",
			scheme:   "https",
			want:     "https://index.docker.io/v2/nickfedor/watchtower/manifests/latest",
		},
		{
			name:     "should not prepend library/ for single-part container names in registries other than Docker Hub",
			imageRef: "docker-registry.domain/imagename:latest",
			scheme:   "https",
			want:     "https://docker-registry.domain/v2/imagename/manifests/latest",
		},
		{
			name:     "should return a valid URL for pinned images with digest",
			imageRef: "docker-registry.domain/imagename@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
			scheme:   "https",
			want:     "https://docker-registry.domain/v2/imagename/manifests/sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
		},
		{
			name:     "should return an error for invalid image references",
			imageRef: "invalid:image:ref:!!",
			scheme:   "https",
			wantErr:  true,
		},
		{
			name:     "should prepend library/ for Docker Hub official images",
			imageRef: "nginx:latest",
			scheme:   "https",
			want:     "https://index.docker.io/v2/library/nginx/manifests/latest",
		},
		{
			name:     "should handle hosts with ports correctly",
			imageRef: "localhost:5000/repo/image:tag",
			scheme:   "https",
			want:     "https://localhost:5000/v2/repo/image/manifests/tag",
		},
		{
			name:     "should use the HTTP scheme when requested",
			imageRef: "ghcr.io/nicholas-fedor/watchtower:mytag",
			scheme:   "http",
			want:     "http://ghcr.io/v2/nicholas-fedor/watchtower/manifests/mytag",
		},
		{
			name:     "should handle tags with hyphens and numbers",
			imageRef: "registry.example.com/my-project/my-app:v1.2.3-rc1",
			scheme:   "https",
			want:     "https://registry.example.com/v2/my-project/my-app/manifests/v1.2.3-rc1",
		},
		{
			name:     "should handle multi-level image paths",
			imageRef: "registry.example.com/org/team/project/image:tag",
			scheme:   "https",
			want:     "https://registry.example.com/v2/org/team/project/image/manifests/tag",
		},
		{
			name:     "should return an error for uppercase registry names",
			imageRef: "REGISTRY.EXAMPLE.COM/IMAGE:TAG",
			scheme:   "https",
			wantErr:  true,
		},
		{
			name:     "should handle image names with dots and dashes",
			imageRef: "registry.example.com/my.project/my-app:latest",
			scheme:   "https",
			want:     "https://registry.example.com/v2/my.project/my-app/manifests/latest",
		},
		{
			name:     "should handle Docker Hub official images with namespace",
			imageRef: "docker.io/library/nginx:latest",
			scheme:   "https",
			want:     "https://index.docker.io/v2/library/nginx/manifests/latest",
		},
		{
			name:     "should return an error for references with invalid digest format",
			imageRef: "registry.example.com/image@sha256:invalid-digest",
			scheme:   "https",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildManifestURLForImage(newTestLogger(t), tt.imageRef, "mock-container", tt.scheme)
			if tt.wantErr {
				require.ErrorIs(t, err, errFailedParseImageName)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
