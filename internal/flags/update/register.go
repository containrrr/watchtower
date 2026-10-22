// Package update registers container update policy flags.
package update

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
)

// defaultStopTimeout is the static default container stop timeout.
const defaultStopTimeout = 30 * time.Second

// Specs returns update domain flag metadata with static defaults.
//
// Returns:
//   - []spec.FlagSpec: Update flag specifications.
func Specs() []spec.FlagSpec {
	return []spec.FlagSpec{
		{
			Name:      "cleanup",
			Shorthand: "c",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_CLEANUP"},
			Help:      "Remove previously used images after updating",
		},
		{
			Name:    "no-pull",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NO_PULL"},
			Help:    "Do not pull any new images",
		},
		{
			Name:    "enable-changelog",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_ENABLE_CHANGELOG"},
			Help:    "Include a release-notes link in notifications. Resolving the exact release tag adds one or two extra registry requests per updated container",
		},
		{
			Name:    "no-restart",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_NO_RESTART"},
			Help:    "Do not restart any containers",
		},
		{
			Name:      "monitor-only",
			Shorthand: "m",
			Kind:      spec.KindBool,
			Default:   false,
			EnvKeys:   []string{"WATCHTOWER_MONITOR_ONLY"},
			Help:      "Will only monitor for new images, not update the containers",
		},
		{
			Name:    "rolling-restart",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_ROLLING_RESTART"},
			Help:    "Restart containers one at a time",
		},
		{
			Name:      "stop-timeout",
			Shorthand: "t",
			Kind:      spec.KindDuration,
			Default:   defaultStopTimeout,
			EnvKeys:   []string{"WATCHTOWER_TIMEOUT"},
			Help:      "Timeout before a container is forcefully stopped (e.g., 30s, 1m, 5m)",
		},
		{
			Name:    "cooldown-delay",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_COOLDOWN_DELAY"},
			Help:    "Minimum time since image creation before allowing updates. Supports h, m, s, d (days), w (weeks), M (months) (e.g., 24h, 3d, 1w, 1M)",
		},
		{
			Name:    "use-compose-depends-on",
			Kind:    spec.KindBool,
			Default: true,
			EnvKeys: []string{"WATCHTOWER_USE_COMPOSE_DEPENDS_ON"},
			Help:    "Include Docker Compose depends_on label when determining container update order",
		},
		{
			Name:    "label-take-precedence",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_LABEL_TAKE_PRECEDENCE"},
			Help:    "Label applied to containers take precedence over arguments",
		},
		{
			Name:    "ephemeral-self-update",
			Kind:    spec.KindBool,
			Default: false,
			EnvKeys: []string{"WATCHTOWER_EPHEMERAL_SELF_UPDATE"},
			Help:    "Use an ephemeral container to orchestrate Watchtower self-updates (experimental)",
		},
		{
			Name:    "disk-space-max",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_DISK_SPACE_MAX"},
			Help:    "Block the update session when Docker image usage reaches this size (e.g., 40GB, 20GiB)",
		},
		{
			Name:    "disk-space-warn",
			Kind:    spec.KindString,
			Default: "",
			EnvKeys: []string{"WATCHTOWER_DISK_SPACE_WARN"},
			Help:    "Warn when Docker image usage reaches this size or percent of --disk-space-max (e.g., 30GB, 80%)",
		},
	}
}

// Register adds update policy flags to the root command.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func Register(rootCmd *cobra.Command) {
	spec.MustRegister(rootCmd.PersistentFlags(), Specs())
}
