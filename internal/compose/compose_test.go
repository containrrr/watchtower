package compose

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	dockerImage "github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// imageManifest builds an image-kind manifest summary for a Linux platform.
// An empty architecture leaves the manifest without platform data.
//
// Parameters:
//   - id: Manifest digest.
//   - architecture: CPU architecture, or empty for no platform data.
//   - available: Whether the manifest's content is available locally.
//
// Returns:
//   - dockerImage.ManifestSummary: Manifest summary.
func imageManifest(id, architecture string, available bool) dockerImage.ManifestSummary {
	manifest := dockerImage.ManifestSummary{
		ID:        id,
		Kind:      dockerImage.ManifestKindImage,
		Available: available,
	}
	if architecture != "" {
		manifest.ImageData = &dockerImage.ImageProperties{
			Platform: ocispec.Platform{OS: "linux", Architecture: architecture},
		}
	}

	return manifest
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

// TestParseDependsOnLabel verifies that a Compose depends_on label yields its
// service names, from the comma-separated form with or without conditions and
// from the JSON form, sorted, trimmed, and without empty entries.
func TestParseDependsOnLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "returns nil for empty label", input: "", want: nil},
		{name: "parses single service", input: "postgres", want: []string{"postgres"}},
		{name: "parses multiple services", input: "postgres,redis", want: []string{"postgres", "redis"}},
		{name: "trims whitespace", input: " postgres , redis ", want: []string{"postgres", "redis"}},
		{
			name:  "parses colon-separated format",
			input: "postgres:service_started:required,redis:service_healthy",
			want:  []string{"postgres", "redis"},
		},
		{name: "ignores empty parts", input: "postgres,,redis", want: []string{"postgres", "redis"}},
		{
			name:  "parses JSON format",
			input: `{"database":{"condition":"service_started"}}`,
			want:  []string{"database"},
		},
		{
			name:  "parses JSON format with multiple services",
			input: `{"database":{"condition":"service_started"},"cache":{"condition":"service_healthy"}}`,
			want:  []string{"cache", "database"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ParseDependsOnLabel(newTestLogger(t), tt.input))
		})
	}
}

// TestComposeLabelGetters verifies that the service name, project name, and
// container number are read from their Compose labels, and are empty when the
// labels are nil, empty, or lack the label.
func TestComposeLabelGetters(t *testing.T) {
	t.Parallel()

	getters := []struct {
		name  string
		get   func(map[string]string) string
		label string
		value string
		// presentName names the case where the label is present.
		presentName string
	}{
		{name: "GetServiceName", get: GetServiceName, label: ComposeServiceLabel, value: "web", presentName: "returns service name when label present"},
		{name: "GetProjectName", get: GetProjectName, label: ComposeProjectLabel, value: "myproject", presentName: "returns project name when label present"},
		{name: "GetContainerNumber", get: GetContainerNumber, label: ComposeContainerNumber, value: "1", presentName: "returns container number when label present"},
	}

	for _, getter := range getters {
		tests := []struct {
			name   string
			labels map[string]string
			want   string
		}{
			{name: "returns empty string for nil labels", labels: nil, want: ""},
			{name: "returns empty string for empty labels", labels: map[string]string{}, want: ""},
			{name: "returns empty string when label not present", labels: map[string]string{"other": "value"}, want: ""},
			{name: getter.presentName, labels: map[string]string{getter.label: getter.value}, want: getter.value},
		}

		t.Run(getter.name, func(t *testing.T) {
			t.Parallel()

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.want, getter.get(tt.labels))
				})
			}
		})
	}
}

// TestImageLabelValue verifies that a com.docker.compose.image label naming the
// old image ID maps to the new image ID, and one naming a platform manifest maps
// to the new image's available manifest for the same platform, and that no
// value is found when the mapping is ambiguous, unavailable, or lacks data.
func TestImageLabelValue(t *testing.T) {
	t.Parallel()

	const (
		oldImageID = "sha256:old-index"
		newImageID = "sha256:new-index"
		oldAmd64   = "sha256:old-amd64"
		oldArm64   = "sha256:old-arm64"
		newAmd64   = "sha256:new-amd64"
		newArm64   = "sha256:new-arm64"
	)

	// multiPlatformOld is an old image holding amd64 and arm64 manifests.
	multiPlatformOld := &dockerImage.InspectResponse{
		ID: oldImageID,
		Manifests: []dockerImage.ManifestSummary{
			imageManifest(oldAmd64, "amd64", true),
			imageManifest(oldArm64, "arm64", true),
		},
	}

	// multiPlatformNew is a new image holding arm64 and amd64 manifests and an
	// attestation.
	multiPlatformNew := &dockerImage.InspectResponse{
		ID: newImageID,
		Manifests: []dockerImage.ManifestSummary{
			imageManifest(newArm64, "arm64", true),
			imageManifest(newAmd64, "amd64", true),
			{ID: "sha256:new-attestation", Kind: dockerImage.ManifestKindAttestation, Available: true},
		},
	}

	tests := []struct {
		name     string
		label    string
		oldImage *dockerImage.InspectResponse
		newImage *dockerImage.InspectResponse
		want     string
		wantOK   bool
	}{
		{
			name:     "maps an image ID to the new image ID",
			label:    oldImageID,
			oldImage: multiPlatformOld,
			newImage: multiPlatformNew,
			want:     newImageID,
			wantOK:   true,
		},
		{
			name:     "maps an image ID when inspected without manifests",
			label:    oldImageID,
			oldImage: &dockerImage.InspectResponse{ID: oldImageID},
			newImage: &dockerImage.InspectResponse{ID: newImageID},
			want:     newImageID,
			wantOK:   true,
		},
		{
			name:     "maps an arm64 manifest digest to the new arm64 manifest",
			label:    oldArm64,
			oldImage: multiPlatformOld,
			newImage: multiPlatformNew,
			want:     newArm64,
			wantOK:   true,
		},
		{
			name:     "maps an amd64 manifest digest to the new amd64 manifest",
			label:    oldAmd64,
			oldImage: multiPlatformOld,
			newImage: multiPlatformNew,
			want:     newAmd64,
			wantOK:   true,
		},
		{
			name:  "maps a manifest without platform data to a lone available manifest",
			label: oldAmd64,
			oldImage: &dockerImage.InspectResponse{
				ID:        oldImageID,
				Manifests: []dockerImage.ManifestSummary{imageManifest(oldAmd64, "", true)},
			},
			newImage: &dockerImage.InspectResponse{
				ID: newImageID,
				Manifests: []dockerImage.ManifestSummary{
					imageManifest(newAmd64, "", true),
					imageManifest(newArm64, "", false),
				},
			},
			want:   newAmd64,
			wantOK: true,
		},
		{
			name:  "finds no value for a manifest without platform data and several candidates",
			label: oldAmd64,
			oldImage: &dockerImage.InspectResponse{
				ID:        oldImageID,
				Manifests: []dockerImage.ManifestSummary{imageManifest(oldAmd64, "", true)},
			},
			newImage: &dockerImage.InspectResponse{
				ID: newImageID,
				Manifests: []dockerImage.ManifestSummary{
					imageManifest(newAmd64, "", true),
					imageManifest(newArm64, "", true),
				},
			},
		},
		{
			name:     "finds no value when the new image lacks the label's platform",
			label:    oldArm64,
			oldImage: multiPlatformOld,
			newImage: &dockerImage.InspectResponse{
				ID:        newImageID,
				Manifests: []dockerImage.ManifestSummary{imageManifest(newAmd64, "amd64", true)},
			},
		},
		{
			name:     "finds no value when the new image's matching manifest is unavailable",
			label:    oldArm64,
			oldImage: multiPlatformOld,
			newImage: &dockerImage.InspectResponse{
				ID:        newImageID,
				Manifests: []dockerImage.ManifestSummary{imageManifest(newArm64, "arm64", false)},
			},
		},
		{
			name:     "finds no value for a manifest digest when inspected without manifests",
			label:    oldAmd64,
			oldImage: &dockerImage.InspectResponse{ID: oldImageID},
			newImage: &dockerImage.InspectResponse{ID: newImageID},
		},
		{
			name:     "finds no value for a label naming neither the old image nor its manifests",
			label:    "sha256:unknown",
			oldImage: multiPlatformOld,
			newImage: multiPlatformNew,
		},
		{
			name:     "finds no value without old image data",
			label:    oldImageID,
			oldImage: nil,
			newImage: multiPlatformNew,
		},
		{
			name:     "finds no value without new image data",
			label:    oldImageID,
			oldImage: multiPlatformOld,
			newImage: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ImageLabelValue(tt.label, tt.oldImage, tt.newImage)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
