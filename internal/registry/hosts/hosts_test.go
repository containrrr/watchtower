package hosts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsGitHubRegistry(t *testing.T) {
	t.Parallel()

	assert.True(t, IsGitHubRegistry(GitHubRegistryDomain))
	assert.True(t, IsGitHubRegistry(LSCRRegistryDomain))
	assert.False(t, IsGitHubRegistry(DockerRegistryHost))
	assert.False(t, IsGitHubRegistry(DockerRegistryDomain))
	assert.False(t, IsGitHubRegistry(""))
}
