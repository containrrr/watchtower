package container

import (
	"errors"
	"os"
	"strings"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/reference"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/cliconfig/credentials"
)


/*
 * Return an encoded auth config for the given registry
 * hardcoded for a test environment
 */
func EncodedTestAuth(ref string) (string, error) {
	auth := types.AuthConfig {
		Username: "testuser",
		Password: "testpassword",
	}
	return EncodeAuth(auth)
}

/*
 * Return an encoded auth config for the given registry
 * loaded from environment variables
 */
func EncodedEnvAuth(ref string) (string, error) {
	username := os.Getenv("REPO_USER")
	password := os.Getenv("REPO_PASS")
	if username != "" && password != "" {
		auth := types.AuthConfig {
			Username: username,
			Password: password,
		}
		log.Debugf("Loaded auth credentials %s from environment for %s", auth, ref)
		return EncodeAuth(auth)
	} else {
		return "", errors.New("Registry auth environment variables (REPO_USER, REPO_PASS) not set")
	}
}

/*
 * Return an encoded auth config for the given registry
 * loaded from the docker config
 */
func EncodedConfigAuth(ref string) (string, error) {
	server, err := ParseServerAddress(ref)
	configFile := command.LoadDefaultConfigFile(log.StandardLogger().Out)
	credStore := CredentialsStore(*configFile)
	auth, err := credStore.Get(server)
	if err != nil {
		return "", err
	}
	log.Debugf("Loaded auth credentials %s from Docker config for reference %s", auth, ref)
	return EncodeAuth(auth)
}

func ParseServerAddress(ref string) (string, error) {
	repository, _, err := reference.Parse(ref)
	if err != nil {
		return ref, err
	}
	parts := strings.Split(repository, "/")
	return parts[0], nil
	
}

// CredentialsStore returns a new credentials store based
// on the settings provided in the configuration file.
func CredentialsStore(configFile configfile.ConfigFile) credentials.Store {
	if configFile.CredentialsStore != "" {
		return credentials.NewNativeStore(&configFile)
	}
	return credentials.NewFileStore(&configFile)
}

/*
 * Base64 encode an AuthConfig struct for transmission over HTTP
 */
func EncodeAuth(auth types.AuthConfig) (string, error) {
	return command.EncodeAuthToBase64(auth)
}

func DefaultAuthHandler() (string, error) {
	log.Error("Authentication requested")
	return "", fmt.Errorf("Error requesting privilege")
}
