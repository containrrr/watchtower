// Package lifecycle registers pre/post update lifecycle hook flags.
package lifecycle

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns lifecycle domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Lifecycle flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:    "enable-lifecycle-hooks",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_LIFECYCLE_HOOKS"},
			Help:    "Enable the execution of commands triggered by pre- and post-update lifecycle hooks",
		},
		{
			Name:    "lifecycle-uid",
			Kind:    spec.KindInt,
			Default: 0,
			EnvKeys: []string{"WATCHTOWER_LIFECYCLE_UID"},
			Help:    "Default UID to run lifecycle hooks as (can be overridden by container labels)",
		},
		{
			Name:    "lifecycle-gid",
			Kind:    spec.KindInt,
			Default: 0,
			EnvKeys: []string{"WATCHTOWER_LIFECYCLE_GID"},
			Help:    "Default GID to run lifecycle hooks as (can be overridden by container labels)",
		},
	}
}

// Register adds lifecycle domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
