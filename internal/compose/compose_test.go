package compose

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

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

// newNopLogger returns a logger that discards every event, for Ginkgo specs,
// benchmarks, and fuzz targets.
//
// Returns:
//   - *zerolog.Logger: Logger that discards all events.
func newNopLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

var _ = ginkgo.Describe("Compose", func() {
	ginkgo.DescribeTable(
		"ParseDependsOnLabel",
		func(input string, expected []string) {
			result := ParseDependsOnLabel(newNopLogger(), input)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry("returns nil for empty label", "", nil),
		ginkgo.Entry("parses single service", "postgres", []string{"postgres"}),
		ginkgo.Entry("parses multiple services", "postgres,redis", []string{"postgres", "redis"}),
		ginkgo.Entry("trims whitespace", " postgres , redis ", []string{"postgres", "redis"}),
		ginkgo.Entry(
			"parses colon-separated format",
			"postgres:service_started:required,redis:service_healthy",
			[]string{"postgres", "redis"},
		),
		ginkgo.Entry("ignores empty parts", "postgres,,redis", []string{"postgres", "redis"}),
		ginkgo.Entry(
			"parses JSON format",
			`{"database":{"condition":"service_started"}}`,
			[]string{"database"},
		),
		ginkgo.Entry(
			"parses JSON format with multiple services",
			`{"database":{"condition":"service_started"},"cache":{"condition":"service_healthy"}}`,
			[]string{"cache", "database"}, // Sorted alphabetically
		),
	)

	ginkgo.DescribeTable(
		"GetServiceName",
		func(labels map[string]string, expected string) {
			result := GetServiceName(labels)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry("returns empty string for nil labels", nil, ""),
		ginkgo.Entry("returns empty string for empty labels", map[string]string{}, ""),
		ginkgo.Entry(
			"returns empty string when label not present",
			map[string]string{"other": "value"},
			"",
		),
		ginkgo.Entry(
			"returns service name when label present",
			map[string]string{ComposeServiceLabel: "web"},
			"web",
		),
	)

	ginkgo.DescribeTable(
		"GetProjectName",
		func(labels map[string]string, expected string) {
			result := GetProjectName(labels)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry("returns empty string for nil labels", nil, ""),
		ginkgo.Entry("returns empty string for empty labels", map[string]string{}, ""),
		ginkgo.Entry(
			"returns empty string when label not present",
			map[string]string{"other": "value"},
			"",
		),
		ginkgo.Entry(
			"returns project name when label present",
			map[string]string{ComposeProjectLabel: "myproject"},
			"myproject",
		),
	)

	ginkgo.DescribeTable(
		"GetContainerNumber",
		func(labels map[string]string, expected string) {
			result := GetContainerNumber(labels)
			gomega.Expect(result).To(gomega.Equal(expected))
		},
		ginkgo.Entry("returns empty string for nil labels", nil, ""),
		ginkgo.Entry("returns empty string for empty labels", map[string]string{}, ""),
		ginkgo.Entry(
			"returns empty string when label not present",
			map[string]string{"other": "value"},
			"",
		),
		ginkgo.Entry(
			"returns container number when label present",
			map[string]string{ComposeContainerNumber: "1"},
			"1",
		),
	)

	ginkgo.Describe("ImageLabelValue", func() {
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

		// multiPlatformNew is a new image holding arm64 and amd64 manifests
		// and an attestation.
		multiPlatformNew := &dockerImage.InspectResponse{
			ID: newImageID,
			Manifests: []dockerImage.ManifestSummary{
				imageManifest(newArm64, "arm64", true),
				imageManifest(newAmd64, "amd64", true),
				{ID: "sha256:new-attestation", Kind: dockerImage.ManifestKindAttestation, Available: true},
			},
		}

		ginkgo.DescribeTable("maps the label to the new image in the same form",
			func(label string, oldImage, newImage *dockerImage.InspectResponse, expected string, expectedOK bool) {
				value, ok := ImageLabelValue(label, oldImage, newImage)
				gomega.Expect(ok).To(gomega.Equal(expectedOK))
				gomega.Expect(value).To(gomega.Equal(expected))
			},
			ginkgo.Entry("maps an image ID to the new image ID",
				oldImageID, multiPlatformOld, multiPlatformNew, newImageID, true,
			),
			ginkgo.Entry("maps an image ID when inspected without manifests",
				oldImageID,
				&dockerImage.InspectResponse{ID: oldImageID},
				&dockerImage.InspectResponse{ID: newImageID},
				newImageID, true,
			),
			ginkgo.Entry("maps an arm64 manifest digest to the new arm64 manifest",
				oldArm64, multiPlatformOld, multiPlatformNew, newArm64, true,
			),
			ginkgo.Entry("maps an amd64 manifest digest to the new amd64 manifest",
				oldAmd64, multiPlatformOld, multiPlatformNew, newAmd64, true,
			),
			ginkgo.Entry("maps a manifest without platform data to a lone available manifest",
				oldAmd64,
				&dockerImage.InspectResponse{
					ID:        oldImageID,
					Manifests: []dockerImage.ManifestSummary{imageManifest(oldAmd64, "", true)},
				},
				&dockerImage.InspectResponse{
					ID: newImageID,
					Manifests: []dockerImage.ManifestSummary{
						imageManifest(newAmd64, "", true),
						imageManifest(newArm64, "", false),
					},
				},
				newAmd64, true,
			),
			ginkgo.Entry("finds no value for a manifest without platform data and several candidates",
				oldAmd64,
				&dockerImage.InspectResponse{
					ID:        oldImageID,
					Manifests: []dockerImage.ManifestSummary{imageManifest(oldAmd64, "", true)},
				},
				&dockerImage.InspectResponse{
					ID: newImageID,
					Manifests: []dockerImage.ManifestSummary{
						imageManifest(newAmd64, "", true),
						imageManifest(newArm64, "", true),
					},
				},
				"", false,
			),
			ginkgo.Entry("finds no value when the new image lacks the label's platform",
				oldArm64,
				multiPlatformOld,
				&dockerImage.InspectResponse{
					ID:        newImageID,
					Manifests: []dockerImage.ManifestSummary{imageManifest(newAmd64, "amd64", true)},
				},
				"", false,
			),
			ginkgo.Entry("finds no value when the new image's matching manifest is unavailable",
				oldArm64,
				multiPlatformOld,
				&dockerImage.InspectResponse{
					ID:        newImageID,
					Manifests: []dockerImage.ManifestSummary{imageManifest(newArm64, "arm64", false)},
				},
				"", false,
			),
			ginkgo.Entry("finds no value for a manifest digest when inspected without manifests",
				oldAmd64,
				&dockerImage.InspectResponse{ID: oldImageID},
				&dockerImage.InspectResponse{ID: newImageID},
				"", false,
			),
			ginkgo.Entry("finds no value for a label naming neither the old image nor its manifests",
				"sha256:unknown", multiPlatformOld, multiPlatformNew, "", false,
			),
			ginkgo.Entry("finds no value without old image data",
				oldImageID, nil, multiPlatformNew, "", false,
			),
			ginkgo.Entry("finds no value without new image data",
				oldImageID, multiPlatformOld, nil, "", false,
			),
		)
	})
})
