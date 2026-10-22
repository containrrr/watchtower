// Package registry registers registry TLS flags.
package registry

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns registry domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Registry flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:    "registry-tls-skip",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_REGISTRY_TLS_SKIP"},
			Help:    "Disable TLS verification for registry connections; allows HTTP or insecure TLS registries (use with caution)",
		},
		{
			Name:    "registry-tls-min-version",
			Kind:    spec.KindString,
			Default: "TLS1.2",
			EnvKeys: []string{"WATCHTOWER_REGISTRY_TLS_MIN_VERSION"},
			Help:    "Minimum TLS version for registry connections (e.g., TLS1.0, TLS1.1, TLS1.2, TLS1.3); default is TLS1.2",
		},
	}
}

// Register adds registry domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
