package helpers

import (
	"github.com/docker/distribution/reference"
)

// domains for Docker Hub, the default registry
const (
	DefaultRegistryDomain       = "docker.io"
	DefaultRegistryHost         = "index.docker.io"
	LegacyDefaultRegistryDomain = "index.docker.io"
)

// GetRegistryAddress parses an image name
// and returns the address of the specified registry
func GetRegistryAddress(imageRef string) (string, error) {
	normalizedRef, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", err
	}

	address := reference.Domain(normalizedRef)

	if address == DefaultRegistryDomain {
		address = DefaultRegistryHost
	}
	return address, nil
}
