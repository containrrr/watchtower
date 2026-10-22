// Package filter registers container selection flags.
package filter

import (
	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// Specs returns filter domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Filter flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "label-enable",
			Shorthand: "e",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_LABEL_ENABLE"},
			Help:      "Watch containers where the com.centurylinklabs.watchtower.enable label is true",
		},
		{
			Name:      "disable-containers",
			Shorthand: "x",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_DISABLE_CONTAINERS"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "Comma-separated list of containers to explicitly exclude from watching.",
		},
		{
			Name:      "monitor-image-names",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_MONITOR_IMAGE_NAMES"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "Comma-separated list of image names to monitor.",
		},
		{
			Name:      "skip-image-names",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_SKIP_IMAGE_NAMES"},
			ListParse: spec.ListCommaOrSpace,
			Help:      "Comma-separated list of image names to explicitly exclude from monitoring.",
		},
		{
			Name:      "enable-containers-by-label",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL"},
			ListParse: spec.ListCommaOnly,
			Help:      "Comma-separated list of key=value label pairs to restrict monitoring to matching containers.",
		},
		{
			Name:      "disable-containers-by-label",
			Kind:      spec.KindStringSlice,
			Default:   []string{},
			EnvKeys:   []string{"WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL"},
			ListParse: spec.ListCommaOnly,
			Help:      "Comma-separated list of key=value label pairs to exclude matching containers from monitoring.",
		},
		{
			Name:    "scope",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_SCOPE"},
			Help:    "Defines a monitoring scope for the Watchtower instance.",
		},
	}
}

// Register adds filter domain flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
