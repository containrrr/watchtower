package container

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodedEnvAuth_ShouldReturnAnErrorIfRepoEnvsAreUnset(t *testing.T) {
	os.Unsetenv("REPO_USER")
	os.Unsetenv("REPO_PASS")
	_, err := EncodedEnvAuth("")
	assert.Error(t, err)
}
func TestEncodedEnvAuth_ShouldReturnAuthHashIfRepoEnvsAreSet(t *testing.T) {
	expectedHash := "eyJ1c2VybmFtZSI6ImNvbnRhaW5ycnItdXNlciIsInBhc3N3b3JkIjoiY29udGFpbnJyci1wYXNzIn0="

	os.Setenv("REPO_USER", "containrrr-user")
	os.Setenv("REPO_PASS", "containrrr-pass")
	config, _ := EncodedEnvAuth("")

	assert.Equal(t, config, expectedHash)
}

func TestEncodedConfigAuth_ShouldReturnAnErrorIfFileIsNotPresent(t *testing.T) {
	os.Setenv("DOCKER_CONFIG", "/dev/null/should-fail")
	_, err := EncodedConfigAuth("")
	assert.Error(t, err)
}

/*
 * TODO:
 * This part only confirms that it still works in the same way as it did
 * with the old version of the docker api client sdk. I'd say that
 * ParseServerAddress likely needs to be elaborated a bit to default to
 * dockerhub in case no server address was provided.
 *
 * ++ @simskij, 2019-04-04
 */

func TestParseServerAddress_ShouldReturnErrorIfPassedEmptyString(t *testing.T) {
	_, err := ParseServerAddress("")
	assert.Error(t, err)
}

func TestParseServerAddress_ShouldReturnTheRepoNameIfPassedAFullyQualifiedImageName(t *testing.T) {
	val, _ := ParseServerAddress("github.com/containrrrr/config")
	assert.Equal(t, val, "github.com")
}

func TestParseServerAddress_ShouldReturnTheOrganizationPartIfPassedAnImageNameMissingServerName(t *testing.T) {
	val, _ := ParseServerAddress("containrrr/config")
	assert.Equal(t, val, "containrrr")
}

func TestParseServerAddress_ShouldReturnTheServerNameIfPassedAFullyQualifiedImageName(t *testing.T) {
	val, _ := ParseServerAddress("github.com/containrrrr/config")
	assert.Equal(t, val, "github.com")
}
