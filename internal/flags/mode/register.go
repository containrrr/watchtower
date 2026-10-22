// Package mode registers process entry-shape flags.
package mode

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns mode domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Mode flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "run-once",
			Shorthand: "R",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_RUN_ONCE"},
			Help:      "Run once now and exit",
		},
		{
			Name:    "health-check",
			Kind:    spec.KindBool,
			Default: false,
			Help:    "Do health check and exit",
		},
		{
			Name:      "porcelain",
			Shorthand: "P",
			Kind:      spec.KindString,
			Default:   "",
			EnvKeys:   []string{"WATCHTOWER_PORCELAIN"},
			Help:      `Write session results to stdout using a stable versioned format. Supported values: "v1", "json"`,
		},
		{
			Name:    "no-startup-message",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NO_STARTUP_MESSAGE"},
			Help:    "Prevents watchtower from sending a startup message",
		},
		{
			Name:    "self-update-orchestrator",
			Kind:    spec.KindBool,
			Default: false,
			Hidden:  true,
			Help:    "Internal: Run as ephemeral orchestrator for self-update (not for direct use)",
		},
	}
}

// Register adds mode domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
