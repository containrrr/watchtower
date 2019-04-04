package container

import (
	"errors"
	"os"
	"strings"

	"github.com/docker/cli/cli/command"
	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

// EncodedAuth returns an encoded auth config for the given registry
// loaded from environment variables or docker config
// as available in that order
func EncodedAuth(ref string) (string, error) {
	auth, err := EncodedEnvAuth(ref)
	if err != nil {
		auth, err = EncodedConfigAuth(ref)
	}
	return auth, err
}

// EncodedEnvAuth returns an encoded auth config for the given registry
// loaded from environment variables
// Returns an error if authentication environment variables have not been set
func EncodedEnvAuth(ref string) (string, error) {
	username := os.Getenv("REPO_USER")
	password := os.Getenv("REPO_PASS")
	if username != "" && password != "" {
		auth := types.AuthConfig{
			Username: username,
			Password: password,
		}
		log.Debugf("Loaded auth credentials %s for %s", auth, ref)
		return EncodeAuth(auth)
	}
	return "", errors.New("Registry auth environment variables (REPO_USER, REPO_PASS) not set")
}

// EncodedConfigAuth returns an encoded auth config for the given registry
// loaded from the docker config
// Returns an empty string if credentials cannot be found for the referenced server
// The docker config must be mounted on the container
func EncodedConfigAuth(ref string) (string, error) {
	server, err := ParseServerAddress(ref)
	configDir := os.Getenv("DOCKER_CONFIG")
	if configDir == "" {
		configDir = "/"
	}
	configFile, err := cliconfig.Load(configDir)
	if err != nil {
		log.Errorf("Unable to find default config file %s", err)
		return "", err
	}
	credStore := CredentialsStore(*configFile)
	auth, err := credStore.Get(server) // returns (types.AuthConfig{}) if server not in credStore
	if auth == (types.AuthConfig{}) {
		log.Debugf("No credentials for %s in %s", server, configFile.Filename)
		return "", nil
	}
	log.Debugf("Loaded auth credentials %s from %s", auth, configFile.Filename)
	return EncodeAuth(auth)
}

// ParseServerAddress extracts the server part from a container image ref
func ParseServerAddress(ref string) (string, error) {

	parsedRef, err := reference.Parse(ref)
	if err != nil {
		return ref, err
	}

	parts := strings.Split(parsedRef.String(), "/")
	return parts[0], nil
}

// CredentialsStore returns a new credentials store based
// on the settings provided in the configuration file.
func CredentialsStore(configFile configfile.ConfigFile) credentials.Store {
	if configFile.CredentialsStore != "" {
		return credentials.NewNativeStore(&configFile, configFile.CredentialsStore)
	}
	return credentials.NewFileStore(&configFile)
}

// EncodeAuth Base64 encode an AuthConfig struct for transmission over HTTP
func EncodeAuth(auth types.AuthConfig) (string, error) {
	return command.EncodeAuthToBase64(auth)
}

// DefaultAuthHandler will be invoked if an AuthConfig is rejected
// It could be used to return a new value for the "X-Registry-Auth" authentication header,
// but there's no point trying again with the same value as used in AuthConfig
func DefaultAuthHandler() (string, error) {
	log.Debug("Authentication request was rejected. Trying again without authentication")
	return "", nil
}
