package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvConfig_Defaults(t *testing.T) {
	cmd := new(cobra.Command)
	RegisterDockerOptions(cmd)
	BindViperFlags(cmd)

	err := EnvConfig()
	require.NoError(t, err)

	assert.Equal(t, "unix:///var/run/docker.sock", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "", os.Getenv("DOCKER_TLS_VERIFY"))
	// Re-enable this test when we've moved to github actions.
	// assert.Equal(t, DockerAPIMinVersion, os.Getenv("DOCKER_API_VERSION"))
}

func TestEnvConfig_Custom(t *testing.T) {
	cmd := new(cobra.Command)
	RegisterDockerOptions(cmd)
	BindViperFlags(cmd)

	err := cmd.ParseFlags([]string{"--host", "some-custom-docker-host", "--tlsverify", "--api-version", "1.99"})
	require.NoError(t, err)

	err = EnvConfig()
	require.NoError(t, err)

	assert.Equal(t, "some-custom-docker-host", os.Getenv("DOCKER_HOST"))
	assert.Equal(t, "1", os.Getenv("DOCKER_TLS_VERIFY"))
	// Re-enable this test when we've moved to github actions.
	// assert.Equal(t, "1.99", os.Getenv("DOCKER_API_VERSION"))
}

func TestGetSecretsFromFilesWithString(t *testing.T) {
	value := "supersecretstring"

	err := os.Setenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD", value)
	require.NoError(t, err)

	testGetSecretsFromFiles(t, "notification-email-server-password", value)
}

func TestGetSecretsFromFilesWithFile(t *testing.T) {
	value := "megasecretstring"

	// Create the temporary file which will contain a secret.
	file, err := ioutil.TempFile(os.TempDir(), "watchtower-")
	require.NoError(t, err)
	defer func() {
		// Make sure to remove the temporary file later.
		_ = os.Remove(file.Name())
	}()

	// Write the secret to the temporary file.
	secret := []byte(value)
	_, err = file.Write(secret)
	require.NoError(t, err)

	err = os.Setenv("WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD", file.Name())
	require.NoError(t, err)

	testGetSecretsFromFiles(t, "notification-email-server-password", value)
}

func testGetSecretsFromFiles(t *testing.T, flagName string, expected string) {
	cmd := new(cobra.Command)
	RegisterNotificationOptions(cmd)
	BindViperFlags(cmd)

	GetSecretsFromFiles()
	value := viper.GetString(flagName)

	assert.Equal(t, expected, value)
}

func TestHTTPAPIPeriodicPollsFlag(t *testing.T) {
	cmd := new(cobra.Command)

	RegisterDockerOptions(cmd)
	RegisterSystemOptions(cmd)

	err := cmd.ParseFlags([]string{"--http-api-periodic-polls"})
	require.NoError(t, err)

	periodicPolls, err := cmd.PersistentFlags().GetBool("http-api-periodic-polls")
	require.NoError(t, err)

	assert.Equal(t, true, periodicPolls)
}

func TestEnvVariablesMapToFlags(t *testing.T) {

	viper.Reset()
	cmd := new(cobra.Command)

	RegisterDockerOptions(cmd)
	RegisterSystemOptions(cmd)
	RegisterNotificationOptions(cmd)
	BindViperFlags(cmd)

	//for _, opt := range stringConfOpts {
	//	value := opt.key
	//	assert.Nil(t, os.Setenv(opt.env, value))
	//	assert.Equal(t, value, viper.GetString(opt.key))
	//}
	//
	//for _, opt := range intConfOpts {
	//	value := len(opt.key)
	//	assert.Nil(t, os.Setenv(opt.env, fmt.Sprint(value)))
	//	assert.Equal(t, value, viper.GetInt(opt.key))
	//}
	//
	//for _, opt := range boolConfOpts {
	//	assert.Nil(t, os.Setenv(opt.env, fmt.Sprint(true)))
	//	assert.True(t, viper.GetBool(opt.key))
	//}

}
