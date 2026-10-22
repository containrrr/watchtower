package containers

import (
	"context"
	"fmt"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Status describes a single watched container's current image identity.
type Status struct {
	// Name is the container name.
	Name string `json:"name"`
	// Image is the image reference with tag (e.g. ethpandaops/lighthouse:latest).
	Image string `json:"image"`
	// ImageID is the local image config ID (sha256:...).
	ImageID string `json:"image_id"`
	// Digest is the registry manifest digest the image was pulled from
	// (sha256:...), derived from the image's RepoDigests. It is directly
	// comparable to a registry's Docker-Content-Digest. Empty for locally-built
	// images with no registry reference.
	Digest string `json:"digest"`
}

// ListFunc returns the current status of all watched containers.
type ListFunc func(ctx context.Context) ([]Status, error)

// filterStatuses filters a slice of statuses by name and image query parameters.
func filterStatuses(statuses []Status, name, image string) []Status {
	// Start empty so a filter that matches nothing encodes as [] rather than null.
	filtered := make([]Status, 0, len(statuses))

	for _, status := range statuses {
		if name != "" && status.Name != name {
			continue
		}

		if image != "" && status.Image != image {
			continue
		}

		filtered = append(filtered, status)
	}

	return filtered
}

// ListContainerStatuses fetches all containers from the client and transforms
// them into API status objects with image identity information.
//
// Parameters:
//   - ctx: Context for the Docker API call.
//   - client: Docker client.
//   - filter: Container filter function.
//
// Returns:
//   - []Status: Status for each watched container.
//   - error: Non-nil if listing containers fails.
func ListContainerStatuses(ctx context.Context, client container.Client, filter types.Filter) ([]Status, error) {
	var list []types.Container

	var err error

	if filter != nil {
		list, err = client.ListContainers(ctx, filter)
	} else {
		list, err = client.ListContainers(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	statuses := make([]Status, 0, len(list))
	for _, c := range list {
		statuses = append(statuses, containerToStatus(c))
	}

	return statuses, nil
}

func containerToStatus(c types.Container) Status {
	status := Status{
		Name:    c.Name(),
		Image:   c.ImageName(),
		ImageID: string(c.ImageID()),
	}

	if info := c.ImageInfo(); info != nil {
		status.Digest = container.ExtractImageDigest(info.RepoDigests, c.ImageName())
	}

	return status
}
