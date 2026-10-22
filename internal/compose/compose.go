package compose

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/containerd/platforms"
	"github.com/rs/zerolog"

	dockerImage "github.com/moby/moby/api/types/image"
)

// Docker Compose labels.
const (
	// ComposeDependsOnLabel lists container names this container depends on from Docker Compose, comma-separated.
	ComposeDependsOnLabel = "com.docker.compose.depends_on"
	// ComposeProjectLabel specifies the project name of the container in Docker Compose.
	ComposeProjectLabel = "com.docker.compose.project"
	// ComposeServiceLabel specifies the service name of the container in Docker Compose.
	ComposeServiceLabel = "com.docker.compose.service"
	// ComposeContainerNumber specifies the container number of the container in Docker Compose.
	ComposeContainerNumber = "com.docker.compose.container-number"
	// ComposeImageLabel identifies the image Compose created the container from.
	ComposeImageLabel = "com.docker.compose.image"
	// ComposeWorkingDirLabel is the host working directory Compose recorded on the container.
	ComposeWorkingDirLabel = "com.docker.compose.project.working_dir"
	// ComposeConfigFilesLabel lists compose files Compose used, comma-separated.
	ComposeConfigFilesLabel = "com.docker.compose.project.config_files"
	// WatchtowerComposeDirLabel is a path inside Watchtower to the Compose project directory.
	WatchtowerComposeDirLabel = "com.centurylinklabs.watchtower.compose-dir"
)

// ParseDependsOnLabel parses the Docker Compose depends_on label value.
//
// It handles both JSON format (Docker Compose v2+) and comma-separated string format.
// Returns a slice of service names.
//
// Parameters:
//   - labelValue: The raw label value from com.docker.compose.depends_on.
//
// Returns:
//   - []string: List of service names.
func ParseDependsOnLabel(log *zerolog.Logger, labelValue string) []string {
	if labelValue == "" {
		return nil
	}

	if strings.HasPrefix(strings.TrimSpace(labelValue), "{") {
		var dependsOn map[string]json.RawMessage

		err := json.Unmarshal([]byte(labelValue), &dependsOn)
		if err != nil {
			log.Debug().
				Err(err).
				Str("label_value", labelValue).
				Msg("Failed to parse as JSON, falling back to string parsing")
		} else {
			return slices.Sorted(maps.Keys(dependsOn))
		}
	}

	deps := strings.Split(labelValue, ",")
	services := make([]string, 0, len(deps))

	for _, dep := range deps {
		serviceName, _, _ := strings.Cut(strings.TrimSpace(dep), ":")

		serviceName = strings.TrimSpace(serviceName)
		if serviceName != "" {
			services = append(services, serviceName)
		}
	}

	return services
}

// GetProjectName extracts the project name from Docker Compose labels.
//
// If the com.docker.compose.project label is present, returns its value.
// Otherwise, returns an empty string.
//
// Parameters:
//   - labels: Map of container labels.
//
// Returns:
//   - string: Project name if present, empty string otherwise.
func GetProjectName(labels map[string]string) string {
	return labels[ComposeProjectLabel]
}

// GetServiceName extracts the service name from Docker Compose labels.
//
// If the com.docker.compose.service label is present, returns its value.
// Otherwise, returns an empty string.
//
// Parameters:
//   - labels: Map of container labels.
//
// Returns:
//   - string: Service name if present, empty string otherwise.
func GetServiceName(labels map[string]string) string {
	return labels[ComposeServiceLabel]
}

// GetContainerNumber extracts the container number from the Docker Compose labels.
//
// If the ComposeContainerNumber label is present, returns its value.
// Otherwise, returns an empty string.
//
// Parameters:
//   - labels: Map of container labels.
//
// Returns:
//   - string: Container replica number if present, empty string otherwise.
func GetContainerNumber(labels map[string]string) string {
	return labels[ComposeContainerNumber]
}

// ImageLabelValue returns the com.docker.compose.image value that identifies
// newImage in the same form as label, which Compose recorded for oldImage.
//
// Compose records either the image ID or the digest of one platform's image
// manifest, depending on its version and the image store. A label holding the
// old image ID maps to the new image ID. A label holding the digest of one of
// the old image's manifests maps to the digest of the new image's manifest for
// the same platform. Manifests are only present when the image was inspected
// with them.
//
// Parameters:
//   - label: Label value Compose recorded for oldImage.
//   - oldImage: Inspect data of the image the label was recorded for.
//   - newImage: Inspect data of the image the label should identify.
//
// Returns:
//   - string: Label value for newImage.
//   - bool: False when label names neither oldImage nor one of its image
//     manifests, or when newImage has no manifest for that platform.
func ImageLabelValue(label string, oldImage, newImage *dockerImage.InspectResponse) (string, bool) {
	if oldImage == nil || newImage == nil {
		return "", false
	}

	if label == oldImage.ID {
		return newImage.ID, true
	}

	oldManifest, ok := findImageManifest(oldImage.Manifests, label)
	if !ok {
		return "", false
	}

	newManifest, ok := selectPlatformManifest(newImage.Manifests, oldManifest.ImageData)
	if !ok {
		return "", false
	}

	return newManifest.ID, true
}

// findImageManifest returns the image-kind manifest with the given digest.
//
// Parameters:
//   - manifests: Manifests of an image.
//   - digest: Manifest digest to find.
//
// Returns:
//   - dockerImage.ManifestSummary: Matching manifest.
//   - bool: False when no image-kind manifest has the digest.
func findImageManifest(manifests []dockerImage.ManifestSummary, digest string) (dockerImage.ManifestSummary, bool) {
	for _, manifest := range manifests {
		if manifest.Kind == dockerImage.ManifestKindImage && manifest.ID == digest {
			return manifest, true
		}
	}

	return dockerImage.ManifestSummary{}, false
}

// selectPlatformManifest selects the available image-kind manifest for the
// platform in imageData, following the selection Compose uses when it records
// the image label.
//
// Without platform data, a lone available image manifest is selected.
//
// Parameters:
//   - manifests: Manifests of an image.
//   - imageData: Image properties holding the platform to select, or nil.
//
// Returns:
//   - dockerImage.ManifestSummary: Selected manifest.
//   - bool: False when no single manifest matches.
func selectPlatformManifest(
	manifests []dockerImage.ManifestSummary,
	imageData *dockerImage.ImageProperties,
) (dockerImage.ManifestSummary, bool) {
	available := slices.DeleteFunc(slices.Clone(manifests), func(manifest dockerImage.ManifestSummary) bool {
		return manifest.Kind != dockerImage.ManifestKindImage || !manifest.Available
	})

	if imageData == nil {
		if len(available) == 1 {
			return available[0], true
		}

		return dockerImage.ManifestSummary{}, false
	}

	matcher := platforms.NewMatcher(imageData.Platform)

	for _, manifest := range available {
		if manifest.ImageData != nil && matcher.Match(manifest.ImageData.Platform) {
			return manifest, true
		}
	}

	return dockerImage.ManifestSummary{}, false
}
