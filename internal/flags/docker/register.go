// Package docker registers Docker API client flags.
package docker

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns Docker domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Docker flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "host",
			Shorthand: "H",
			Kind:      spec.KindString,
			Default:   "unix:///var/run/docker.sock",
			EnvKeys:   []string{"DOCKER_HOST"},
			Help:      "daemon socket to connect to",
		},
		{
			Name:      "tlsverify",
			Shorthand: "v",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"DOCKER_TLS_VERIFY"},
			Help:      "use TLS and verify the remote",
		},
		{
			Name:      "api-version",
			Shorthand: "a",
			Kind:      spec.KindString,
			Default:   "",
			EnvKeys:   []string{"DOCKER_API_VERSION"},
			Help:      "api version to use by docker client (omit for autonegotiation)",
		},
		{
			Name:    "cert-path",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"DOCKER_CERT_PATH"},
			Help:    "Path to TLS certificates",
		},
	}
}

// Register adds Docker API client flags using static defaults from Specs.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	registerOn(rootCmd.PersistentFlags())
}

// registerOn registers Docker flags onto an arbitrary flag set.
//
// Parameters:
//   - flagSet: Target flag set.
func registerOn(flagSet *pflag.FlagSet) {
	spec.MustRegister(flagSet, Specs())
}
