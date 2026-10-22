// Package compat registers runtime compatibility flags.
package compat

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns compat domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Compat flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:    "disable-memory-swappiness",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_DISABLE_MEMORY_SWAPPINESS"},
			Help:    "Label used for setting memory swappiness as nil when recreating the container, used for compatibility with podman",
		},
		{
			Name:    "cpu-copy-mode",
			Kind:    spec.KindString,
			Default: "auto",
			EnvKeys: []string{"WATCHTOWER_CPU_COPY_MODE"},
			Help:    "CPU copy mode for container recreation, used for compatibility with Podman. Options: auto, full, none",
		},
	}
}

// Register adds compatibility domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
