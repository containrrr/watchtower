// Package schedule registers poll interval and cron schedule flags.
package schedule

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// DefaultPollIntervalSeconds is the static default poll interval (24 hours).
const DefaultPollIntervalSeconds = 86400

// Specs returns schedule domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Schedule flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "interval",
			Shorthand: "i",
			Kind:      spec.KindInt,
			Default:   DefaultPollIntervalSeconds,
			EnvKeys:   []string{"WATCHTOWER_POLL_INTERVAL"},
			Help:      "Poll interval (in seconds)",
		},
		{
			Name:      "schedule",
			Shorthand: "s",
			Kind:      spec.KindString,
			Default:   "",
			EnvKeys:   []string{"WATCHTOWER_SCHEDULE"},
			Help:      "The cron expression which defines when to update",
		},
		{
			Name:    "update-on-start",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_UPDATE_ON_START"},
			Help:    "Perform an update check on startup, then continue with periodic updates",
		},
	}
}

// Register adds schedule domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
