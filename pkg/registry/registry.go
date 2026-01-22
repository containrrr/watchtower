package registry

import (
	"crypto/x509"
	"io/ioutil"

	"github.com/containrrr/watchtower/pkg/registry/helpers"
	watchtowerTypes "github.com/containrrr/watchtower/pkg/types"
	ref "github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

// InsecureSkipVerify controls whether registry HTTPS connections used for
// manifest HEAD/token requests disable certificate verification. Default is false.
// This is exposed so callers (e.g. CLI flag handling) can toggle it.
var InsecureSkipVerify = false

// RegistryCABundle is an optional filesystem path to a PEM bundle that will be
// used as additional trusted CAs when validating registry TLS certificates.
var RegistryCABundle string

// registryCertPool caches the loaded cert pool when RegistryCABundle is set
var registryCertPool *x509.CertPool

// GetPullOptions creates a struct with all options needed for pulling images from a registry
func GetPullOptions(imageName string) (types.ImagePullOptions, error) {
	auth, err := EncodedAuth(imageName)
	log.Debugf("Got image name: %s", imageName)
	if err != nil {
		return types.ImagePullOptions{}, err
	}

	if auth == "" {
		return types.ImagePullOptions{}, nil
	}

	// CREDENTIAL: Uncomment to log docker config auth
	// log.Tracef("Got auth value: %s", auth)

	return types.ImagePullOptions{
		RegistryAuth:  auth,
		PrivilegeFunc: DefaultAuthHandler,
	}, nil
}

// DefaultAuthHandler will be invoked if an AuthConfig is rejected
// It could be used to return a new value for the "X-Registry-Auth" authentication header,
// but there's no point trying again with the same value as used in AuthConfig
func DefaultAuthHandler() (string, error) {
	log.Debug("Authentication request was rejected. Trying again without authentication")
	return "", nil
}

// WarnOnAPIConsumption will return true if the registry is known-expected
// to respond well to HTTP HEAD in checking the container digest -- or if there
// are problems parsing the container hostname.
// Will return false if behavior for container is unknown.
func WarnOnAPIConsumption(container watchtowerTypes.Container) bool {

	normalizedRef, err := ref.ParseNormalizedNamed(container.ImageName())
	if err != nil {
		return true
	}

	containerHost, err := helpers.GetRegistryAddress(normalizedRef.Name())
	if err != nil {
		return true
	}

	if containerHost == helpers.DefaultRegistryHost || containerHost == "ghcr.io" {
		return true
	}

	return false
}

// GetRegistryCertPool returns a cert pool that includes system roots plus any
// additional CAs provided via RegistryCABundle. The resulting pool is cached.
func GetRegistryCertPool() *x509.CertPool {
	if RegistryCABundle == "" {
		return nil
	}
	if registryCertPool != nil {
		return registryCertPool
	}
	// Try to load file
	data, err := ioutil.ReadFile(RegistryCABundle)
	if err != nil {
		log.WithField("path", RegistryCABundle).Errorf("Failed to load registry CA bundle: %v", err)
		return nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(data); !ok {
		log.WithField("path", RegistryCABundle).Warn("No certs appended from registry CA bundle; file may be empty or invalid PEM")
	}
	registryCertPool = pool
	return registryCertPool
}
