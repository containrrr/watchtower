package details

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// ContainerDetails holds detailed information about a single watched container.
type ContainerDetails struct {
	// Name is the container name.
	Name string `json:"name"`
	// Image is the image reference with tag.
	Image string `json:"image"`
	// ImageID is the local image config ID (sha256:...).
	ImageID string `json:"image_id"`
	// Digest is the registry manifest digest (sha256:...).
	Digest string `json:"digest"`
	// Running indicates whether the container is currently running.
	Running bool `json:"running"`
	// Watchtower indicates whether this is the Watchtower container itself.
	Watchtower bool `json:"watchtower"`
	// MonitorOnly indicates whether the container is in monitor-only mode.
	MonitorOnly bool `json:"monitor_only"`
	// NoPull indicates whether image pulling is disabled for this container.
	NoPull bool `json:"no_pull"`
	// Enabled indicates whether the container is enabled for watching.
	Enabled bool `json:"enabled"`
	// Stale indicates whether the container's image is outdated.
	Stale bool `json:"stale"`
	// Scope is the monitoring scope of the container.
	Scope string `json:"scope"`
}

// GetFunc returns detailed container status for all watched containers.
type GetFunc func(ctx context.Context, name, image string) ([]ContainerDetails, error)

// Handler serves the /v1/containers/details endpoint.
type Handler struct {
	log *zerolog.Logger

	getDetails GetFunc
	Path       string
}

// New creates a new container details handler backed by the given function.
//
// Parameters:
//   - getDetails: Function that returns detailed container information.
func New(log *zerolog.Logger, getDetails GetFunc) *Handler {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	return &Handler{
		log:        log,
		getDetails: getDetails,
		Path:       "/v1/containers/details",
	}
}

// Handle responds with detailed container information as JSON.
//
//	@Summary		Detailed container status
//	@Description	Returns detailed information about each watched container, including running state, image identity, and configuration flags. Optionally filter by container name or image name.
//	@Tags			containers
//	@Accept			json
//	@Produce		json
//	@Param			name	query		string					false	"Filter by container name (exact match)"
//	@Param			image	query		string					false	"Filter by image name (exact match)"
//	@Success		200		{object}	map[string]interface{}	"Container details with count and timestamp"
//	@Failure		500		{string}	string					"Failed to get container details"
//	@Failure		401		{string}	string					"Missing or invalid API token"
//	@Security		BearerAuth
//	@Router			/v1/containers/details [get]
func (h *Handler) Handle(c fiber.Ctx) error {
	h.log.Debug().
		Str("method", c.Method()).
		Str("path", c.Path()).
		Str("notify", "no").
		Msg("Received HTTP API container details request")

	nameFilter := c.Query("name")
	imageFilter := c.Query("image")

	details, err := h.getDetails(c.Context(), nameFilter, imageFilter)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("notify", "no").
			Msg("Failed to get container details for API")

		sendErr := c.Status(fiber.StatusInternalServerError).
			SendString("failed to get container details")
		if sendErr != nil {
			return fmt.Errorf("failed to send error response: %w", sendErr)
		}

		return nil
	}

	err = c.Status(fiber.StatusOK).JSON(fiber.Map{
		"containers":  details,
		"count":       len(details),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"api_version": "v1",
	})
	if err != nil {
		return fmt.Errorf("failed to send JSON response: %w", err)
	}

	return nil
}

// GetContainerDetails fetches detailed information for all watched containers,
// optionally filtered by container name or image name.
//
// Parameters:
//   - ctx: Context for the Docker API call.
//   - client: Docker client.
//   - filter: Container filter function.
//   - nameFilter: Exact container name to filter by, or empty for no filter.
//   - imageFilter: Exact image name to filter by, or empty for no filter.
//   - params: Update parameters for computing per-container effective flags.
//
// Returns:
//   - []ContainerDetails: Detailed information for each matching container.
//   - error: Non-nil if listing containers fails.
func GetContainerDetails(
	ctx context.Context,
	client container.Client,
	filter types.Filter,
	nameFilter,
	imageFilter string,
	params types.UpdateParams,
) ([]ContainerDetails, error) {
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

	details := make([]ContainerDetails, 0, len(list))

	for _, c := range list {
		if nameFilter != "" && c.Name() != nameFilter {
			continue
		}

		if imageFilter != "" && c.ImageName() != imageFilter {
			continue
		}

		rawEnabled, hasLabel := c.Enabled()

		var enabled bool
		if !hasLabel {
			enabled = !params.LabelEnable
		} else {
			enabled = rawEnabled
		}

		scope, _ := c.Scope()

		containerDetails := ContainerDetails{
			Name:        c.Name(),
			Image:       c.ImageName(),
			ImageID:     string(c.ImageID()),
			Running:     c.IsRunning(),
			Watchtower:  c.IsWatchtower(),
			MonitorOnly: c.IsMonitorOnly(params),
			NoPull:      c.IsNoPull(params),
			Enabled:     enabled,
			Stale:       c.IsStale(),
			Scope:       scope,
		}

		info := c.ImageInfo()
		if info != nil {
			containerDetails.Digest = container.ExtractImageDigest(info.RepoDigests, containerDetails.Image)
		}

		details = append(details, containerDetails)
	}

	return details, nil
}
