package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/internal/registry/auth"
	"github.com/nicholas-fedor/watchtower/internal/registry/hosts"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Errors for registry operations.
var (
	// errFailedGetAuth indicates a failure to retrieve authentication credentials for an image.
	errFailedGetAuth = errors.New("failed to get authentication credentials")
)

// GetPullOptions creates a struct with all options needed for pulling images from a registry.
//
// It retrieves encoded authentication credentials and configures pull options with a privilege function.
//
// Parameters:
//   - imageName: Name of the image to pull (e.g., "docker.io/library/alpine").
//
// Returns:
//   - image.PullOptions: Configured pull options if successful.
//   - error: Non-nil if auth retrieval fails, nil on success.
func GetPullOptions(log *zerolog.Logger, imageName string) (dockerClient.ImagePullOptions, error) {
	clogVal := log.With().Str("image", imageName).Logger()
	clog := &clogVal

	clog.Debug().Msg("Retrieving pull options")

	registryCredentials, err := encodedAuth(log, imageName)
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to get authentication credentials")

		return dockerClient.ImagePullOptions{}, fmt.Errorf("%w: %w", errFailedGetAuth, err)
	}

	if registryCredentials == "" {
		clog.Debug().Msg("No authentication credentials retrieved")

		return dockerClient.ImagePullOptions{}, nil
	}

	if clog.GetLevel() == zerolog.TraceLevel {
		clog.Trace().
			Bool("has_credentials", true).
			Msg("Retrieved authentication credentials")
	}

	pullOptions := dockerClient.ImagePullOptions{
		RegistryAuth: registryCredentials,
		PrivilegeFunc: func(ctx context.Context) (string, error) {
			return defaultAuthHandler(log, ctx)
		},
	}

	clog.Debug().Msg("Configured pull options")

	return pullOptions, nil
}

// defaultAuthHandler is a privilege function called when initial authentication fails.
//
// It retries the request without credentials, as reusing the same auth is unlikely to succeed.
//
// Parameters:
//   - ctx: Context for request lifecycle control (unused here).
//
// Returns:
//   - string: Empty string to indicate no new credentials.
//   - error: Always nil, as no further action is taken.
func defaultAuthHandler(log *zerolog.Logger, _ context.Context) (string, error) {
	// Log the auth rejection and proceed without credentials.
	log.Debug().Msg("Authentication rejected, retrying without credentials")

	return "", nil
}

// WarnOnAPIConsumption determines whether to warn about API consumption for a container's registry.
//
// It returns true for registries supporting HEAD requests (e.g., Docker Hub, GHCR) or if parsing fails.
//
// Parameters:
//   - container: Container with image info for registry check.
//
// Returns:
//   - bool: True if a warning is warranted, false otherwise.
func WarnOnAPIConsumption(log *zerolog.Logger, container types.Container) bool {
	clogVal := log.With().
		Str("container", container.Name()).
		Str("image", container.ImageName()).
		Logger()
	clog := &clogVal

	normalizedRef, err := reference.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to parse image reference, assuming API consumption")

		return true
	}

	containerHost, err := auth.GetRegistryAddress(log, normalizedRef.Name())
	if err != nil {
		clog.Debug().
			Err(err).
			Msg("Failed to get registry address, assuming API consumption")

		return true
	}

	if containerHost == hosts.DockerRegistryHost || containerHost == hosts.GitHubRegistryDomain {
		clog.Debug().
			Str("host", containerHost).
			Msg("Registry supports HEAD requests, warning on API consumption")

		return true
	}

	clog.Debug().
		Str("host", containerHost).
		Msg("Registry behavior unknown, no API consumption warning")

	return false
}
