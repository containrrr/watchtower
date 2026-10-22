// Package client registers Docker client construction flags.
package client

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns client domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Client flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "include-stopped",
			Shorthand: "S",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_INCLUDE_STOPPED"},
			Help:      "Will also include created and exited containers",
		},
		{
			Name:    "include-restarting",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_INCLUDE_RESTARTING"},
			Help:    "Will also include restarting containers",
		},
		{
			Name:    "revive-stopped",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_REVIVE_STOPPED"},
			Help:    "Will also start stopped containers that were updated, if include-stopped is active",
		},
		{
			Name:    "remove-volumes",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_REMOVE_VOLUMES"},
			Help:    "Remove attached volumes before updating",
		},
		{
			Name:    "warn-on-head-failure",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_WARN_ON_HEAD_FAILURE"},
			Help:    "When to warn about HEAD pull requests failing. Possible values: always, auto or never",
		},
	}
}

// Register adds client domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
