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

// mockDigested is a test implementation of reference.Digested with an invalid String() method.
type mockDigested struct {
	name   string
	digest digest.Digest
}

func (m *mockDigested) String() string {
	return m.name
}

func (m *mockDigested) Digest() digest.Digest {
	return m.digest
}

func testLog() *zerolog.Logger {
	return new(zerolog.Nop())
}

func TestParseImageRef(t *testing.T) {
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
			_, err := parseImageRef(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveRegistryHost(t *testing.T) {
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
			result := resolveRegistryHost(testLog(), tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManifestURLPath(t *testing.T) {
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
			result := manifestURLPath(tt.imagePath, tt.specifier)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManifestURLString(t *testing.T) {
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
			result := manifestURLString(tt.host, tt.scheme, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTagComponents(t *testing.T) {
	ref, err := reference.ParseDockerRef("registry.example.com/org/image:v1.0.0")
	require.NoError(t, err)

	tagged, ok := ref.(reference.NamedTagged)
	require.True(t, ok, "Reference should be NamedTagged")

	components := tagComponents(tagged)

	assert.Equal(t, "org/image", components.imagePath)
	assert.Equal(t, "v1.0.0", components.specifier)
}

func TestDigestComponents(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantErr       bool
		expectedImage string
		expectedSpec  string
	}{
		{
			name:          "valid digest reference",
			input:         "registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
			wantErr:       false,
			expectedImage: "org/image",
			expectedSpec:  "sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := reference.ParseDockerRef(tt.input)
			if err != nil {
				t.Skipf("Skipping: parse error: %v", err)

				return
			}

			digested, ok := ref.(reference.Digested)
			if !ok {
				t.Skip("Reference is not Digested")

				return
			}

			components, err := digestComponents(digested)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedImage, components.imagePath)
			assert.Equal(t, tt.expectedSpec, components.specifier)
		})
	}
}

func TestDigestComponentsInvalidString(t *testing.T) {
	// Test the error path in digestComponents when String() returns an invalid reference.
	digestVal, err := digest.Parse("sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)

	// Use empty string to trigger reference.WithName error.
	mock := &mockDigested{
		name:   "",
		digest: digestVal,
	}

	components, err := digestComponents(mock)
	require.Error(t, err)
	assert.Empty(t, components)
}

func TestBuildTaggedManifestURL(t *testing.T) {
	ref, err := reference.ParseDockerRef("ghcr.io/org/image:mytag")
	require.NoError(t, err)

	tagged, ok := ref.(reference.NamedTagged)
	require.True(t, ok, "Reference should be NamedTagged")

	fields := map[string]any{
		"container": "test-container",
		"image":     "ghcr.io/org/image:mytag",
	}

	url, err := buildTaggedManifestURL(testLog(), fields, tagged, "https")
	require.NoError(t, err)

	expected := "https://ghcr.io/v2/org/image/manifests/mytag"
	assert.Equal(t, expected, url)
}

func TestBuildDigestedManifestURL(t *testing.T) {
	ref, err := reference.ParseDockerRef("registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7")
	require.NoError(t, err)

	digested, ok := ref.(reference.Digested)
	require.True(t, ok, "Reference should be Digested")

	fields := map[string]any{
		"container": "test-container",
		"image":     "registry.example.com/org/image@sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7",
	}

	url, err := buildDigestedManifestURL(testLog(), fields, digested, "https")
	require.NoError(t, err)

	expected := "https://registry.example.com/v2/org/image/manifests/sha256:daf7034c5c89775afe3008393ae033529913548243b84926931d7c84398ecda7"
	assert.Equal(t, expected, url)
}

func TestBuildDigestedManifestURLInvalidComponents(t *testing.T) {
	// Test the error path in buildDigestedManifestURL when digestComponents fails.
	digestVal, err := digest.Parse("sha256:" + strings.Repeat("a", 64))
	require.NoError(t, err)

	mock := &mockDigested{
		name:   "",
		digest: digestVal,
	}

	fields := map[string]any{
		"container": "test-container",
		"image":     "invalid",
	}

	url, err := buildDigestedManifestURL(testLog(), fields, mock, "https")
	require.Error(t, err)
	assert.Empty(t, url)
}
