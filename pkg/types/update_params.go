package types

import (
	"time"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
)

// UpdateParams defines options for the Update function.
type UpdateParams struct {
	Filter              Filter                  `json:"-"`                      // Container filter.
	Cleanup             bool                    `json:"cleanup"`                // Remove old images if true.
	NoRestart           bool                    `json:"no_restart"`             // Skip restarts if true.
	ReviveStopped       bool                    `json:"revive_stopped"`         // Start stopped containers after update if true.
	Timeout             time.Duration           `json:"timeout"`                // Update timeout.
	MonitorOnly         bool                    `json:"monitor_only"`           // Monitor without updating if true.
	NoPull              bool                    `json:"no_pull"`                // Skip image pulls if true.
	EnableChangelog     bool                    `json:"enable_changelog"`       // Resolve a release-notes link in notifications if true.
	LifecycleHooks      bool                    `json:"lifecycle_hooks"`        // Enable lifecycle hooks if true.
	RollingRestart      bool                    `json:"rolling_restart"`        // Use rolling restart if true.
	LabelPrecedence     bool                    `json:"label_precedence"`       // Prioritize labels if true.
	PullFailureDelay    time.Duration           `json:"pull_failure_delay"`     // Delay after a failed self-update pull in run-once mode.
	LifecycleUID        int                     `json:"lifecycle_uid"`          // Default UID for lifecycle hooks.
	LifecycleGID        int                     `json:"lifecycle_gid"`          // Default GID for lifecycle hooks.
	RunOnce             bool                    `json:"run_once"`               // Run once mode if true.
	CurrentContainerID  ContainerID             `json:"current_container_id"`   // ID of the current container being updated.
	UseComposeDependsOn bool                    `json:"use_compose_depends_on"` // Enable Docker Compose depends_on label processing.
	SkipSelfUpdate      bool                    `json:"skip_self_update"`       // Skip Watchtower self-update if true.
	EphemeralSelfUpdate bool                    `json:"ephemeral_self_update"`  // Use ephemeral container for self-update if true.
	CooldownDelay       time.Duration           `json:"cooldown_delay"`         // Minimum time since image creation before allowing updates.
	LabelEnable         bool                    `json:"label_enable"`           // Require enable label for monitoring.
	DiskSpaceMax        int64                   `json:"disk_space_max"`         // Block session when Docker image usage reaches this many bytes. Zero disables the block gate.
	DiskSpaceWarn       int64                   `json:"disk_space_warn"`        // Warn when Docker image usage reaches this many bytes. Zero disables the warning.
	EnableGitMonitoring bool                    `json:"enable_git_monitoring"`  // Process-wide Git watcher default.
	GitDefaultRef       string                  `json:"git_default_ref"`        // Ref used when a mapping or label omits one.
	GitSemverPolicy     string                  `json:"git_semver_policy"`      // Semver policy used when a mapping or label omits one.
	GitTimeout          time.Duration           `json:"git_timeout"`            // Git network timeout.
	GitImages           map[string]GitImage     `json:"git_images"`             // Per-image Git associations keyed by ImageName().
	GitDockerfile       string                  `json:"git_dockerfile"`         // Default Dockerfile path relative to the build context.
	GitContext          string                  `json:"git_context"`            // Default subdirectory of a Git URL context used as the Docker build context.
	GitComposeStash     bool                    `json:"git_compose_stash"`      // Save and restore local Compose project files around checkout.
	ComposeProjects     map[string]string       `json:"compose_projects"`       // Compose project name to directory inside Watchtower.
	Registry            registryConfig.Registry `json:"-"`                      // Settings for registry requests.
}
