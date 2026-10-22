package actions

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// ValidateRollingRestartDependencies validates the environment for rolling restart updates.
//
// It iterates through the filtered containers and returns an error if any
// container has a linked dependency, which is incompatible with the use of
// a rolling restart update policy.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: Context for cancellation and timeouts.
//   - client: Container client for Docker operations.
//   - filter: Container filter to select relevant containers.
//   - useComposeDependsOn: Whether to include Docker Compose depends_on label in dependency resolution.
//
// Returns:
//   - error: Non-nil if dependencies conflict with rolling restarts, nil otherwise.
func ValidateRollingRestartDependencies(
	log *zerolog.Logger,
	ctx context.Context,
	client container.Client,
	filter types.Filter,
	useComposeDependsOn bool,
) error {
	log.Debug().Msg("Performing pre-update rolling restart dependency validation")

	// Obtain the list of filtered containers.
	containers, err := client.ListContainers(ctx, filter)
	// Handle errors obtaining the list of containers.
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to list containers")

		return fmt.Errorf("%w: %w", errListContainersFailed, err)
	}

	// If there's no containers, then log and return nil.
	if len(containers) == 0 {
		log.Debug().Msg("No containers found")

		return nil
	}

	// Check each container for links.
	for _, c := range containers {
		// If a container has any links, then return an error.
		links := c.Links(useComposeDependsOn)
		if len(links) > 0 {
			log.Debug().
				Str("container", c.Name()).
				Strs("links", links).
				Msg("Found dependencies incompatible with rolling restarts")

			return fmt.Errorf("%w: %q depends on %v", errRollingRestartDependency, c.Name(), links)
		}
	}

	log.Debug().
		Int("container_count", len(containers)).
		Msg("Rolling restart dependency validation passed - no dependencies found")

	return nil
}
