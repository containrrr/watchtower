package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/reference"
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
		log.Debugf("Loaded auth credentials for user %s on registry %s", auth.Username, ref)
		log.Tracef("Using auth password %s", auth.Password)
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
	if err != nil {
		log.Errorf("Unable to parse the image ref %s", err)
		return "", err
	}
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
	auth, _ := credStore.Get(server) // returns (types.AuthConfig{}) if server not in credStore

	if auth == (types.AuthConfig{}) {
		log.WithField("config_file", configFile.Filename).Debugf("No credentials for %s found", server)
		return "", nil
	}
	log.Debugf("Loaded auth credentials for user %s, on registry %s, from file %s", auth.Username, ref, configFile.Filename)
	log.Tracef("Using auth password %s", auth.Password)
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
func EncodeAuth(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}
