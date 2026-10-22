// Package logging registers console logging flags.
package logging

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns logging domain flag metadata with static defaults.
//
// NO_COLOR is listed in EnvKeys so ApplyEnvToFlags can apply presence-based
// semantics (https://no-color.org/). The pflag default stays false. Env is not
// baked into registration-time defaults. BindAll skips BindEnv for NO_COLOR so
// Viper does not re-parse "0"/"false" as false after presence apply.
//
// Returns:
//   - []spec.FlagSpec: Logging flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "log-format",
			Shorthand: "l",
			Kind:      spec.KindString,
			Default:   "auto",
			EnvKeys:   []string{"WATCHTOWER_LOG_FORMAT"},
			Help:      "Sets what logging format to use for console output. Possible values: Auto, LogFmt, Pretty, JSON",
		},
		{
			Name:    "log-level",
			Kind:    spec.KindString,
			Default: "info",
			EnvKeys: []string{"WATCHTOWER_LOG_LEVEL"},
			Help:    "The maximum log level that will be written to STDERR. Possible values: panic, fatal, error, warn, info, debug or trace",
		},
		{
			Name:      "debug",
			Shorthand: "d",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_DEBUG"},
			Help:      "Enable debug mode with verbose logging",
		},
		{
			Name:    "trace",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_TRACE"},
			Help:    "Enable trace mode with very verbose logging - caution, exposes credentials",
		},
		{
			Name:    "no-color",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"NO_COLOR"},
			Help:    "Disable ANSI color escape codes in log output",
		},
	}
}

// Register adds logging domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
