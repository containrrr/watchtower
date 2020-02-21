package flags

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvConfig_Defaults(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)

	err := EnvConfig(cmd)
	require.NoError(t, err)

	assert.Equal(t, "unix:///var/run/docker.sock", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "", os.Getenv("DOCKER_TLS_VERIFY"))
	assert.Equal(t, DockerAPIMinVersion, os.Getenv("DOCKER_API_VERSION"))
}

func TestEnvConfig_Custom(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()
	RegisterDockerFlags(cmd)

	err := cmd.ParseFlags([]string{"--host", "some-custom-docker-host", "--tlsverify", "--api-version", "1.99"})
	require.NoError(t, err)

	err = EnvConfig(cmd)
	require.NoError(t, err)

	assert.Equal(t, "some-custom-docker-host", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "1", os.Getenv("DOCKER_TLS_VERIFY"))
	assert.Equal(t, "1.99", os.Getenv("DOCKER_API_VERSION"))
}

/*
func TestRegisterContainerMemoryFlags(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()

	err := cmd.ParseFlags([]string{"max-memory-per-container", "4g"})
	require.NoError(t, err)
	RegisterContainerMemoryFlags(cmd)

	m, _ := cmd.PersistentFlags().GetString("max-memory-per-container")

	assert.Equal(t, "4g", m)
}
*/
func TestRegisterContainerMemoryFlags_default(t *testing.T) {
	cmd := new(cobra.Command)
	SetDefaults()

	RegisterContainerMemoryFlags(cmd)

	m, _ := cmd.PersistentFlags().GetString("max-memory-per-container")

	assert.Equal(t, "2g", m)
}
