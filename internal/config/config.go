package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"strings"
)

// BindViperFlags binds the cmd PFlags to the viper configuration
func BindViperFlags(cmd *cobra.Command) {
	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind flags: %v", err)
	}
}

// EnvConfig translates the command-line options into environment variables
// that will initialize the api client
func EnvConfig() error {
	var err error

	host := GetString(DockerHost)
	tls := GetBool(DockerTlSVerify)
	version := GetString(DockerApiVersion)
	if err = setEnvOptStr("DOCKER_HOST", host); err != nil {
		return err
	}
	if err = setEnvOptBool("DOCKER_TLS_VERIFY", tls); err != nil {
		return err
	}
	if err = setEnvOptStr("DOCKER_API_VERSION", version); err != nil {
		return err
	}
	return nil
}

func setEnvOptStr(env string, opt string) error {
	if opt == "" || opt == os.Getenv(env) {
		return nil
	}
	err := os.Setenv(env, opt)
	if err != nil {
		return err
	}
	return nil
}

func setEnvOptBool(env string, opt bool) error {
	if opt {
		return setEnvOptStr(env, "1")
	}
	return nil
}

// GetSecretsFromFiles checks if passwords/tokens/webhooks have been passed as a file instead of plaintext.
// If so, the value of the flag will be replaced with the contents of the file.
func GetSecretsFromFiles() {
	secrets := []string{
		string(NotificationEmailServerPassword),
		string(NotificationSlackHookUrl),
		string(NotificationMsteamsHook),
		string(NotificationGotifyToken),
	}
	for _, secret := range secrets {
		getSecretFromFile(secret)
	}
}

// getSecretFromFile will check if the flag contains a reference to a file; if it does, replaces the value of the flag with the contents of the file.
func getSecretFromFile(secret string) {
	value := viper.GetString(secret)
	if value != "" && isFile(value) {
		file, err := ioutil.ReadFile(value)
		if err != nil {
			log.Fatal(err)
		}
		viper.Set(secret, strings.TrimSpace(string(file)))
	}
}

func isFile(s string) bool {
	_, err := os.Stat(s)
	if os.IsNotExist(err) {
		return false
	}
	return true
}
