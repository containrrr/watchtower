package config

import (
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// UpdateParams builds a complete types.UpdateParams from Config and per-run overrides.
//
// This is the only production constructor for update policy DTOs. Every field on
// types.UpdateParams is assigned here so scheduled and run-once paths cannot omit policy.
//
// Parameters:
//   - overrides: Per-invocation overrides (filter, run-once, skip self-update, current container ID).
//
// Returns:
//   - types.UpdateParams: Complete update policy for the actions pipeline.
func (c Config) UpdateParams(overrides RunOverrides) types.UpdateParams {
	filterFn := c.Filter.Predicate
	if overrides.Filter != nil {
		filterFn = overrides.Filter
	}

	return types.UpdateParams{
		Filter:              filterFn,
		Cleanup:             c.Update.Cleanup,
		NoRestart:           c.Update.NoRestart,
		ReviveStopped:       c.Client.ReviveStopped,
		Timeout:             c.Update.StopTimeout,
		MonitorOnly:         c.Update.MonitorOnly,
		NoPull:              c.Update.NoPull,
		EnableChangelog:     c.Update.EnableChangelog,
		LifecycleHooks:      c.Lifecycle.Enabled,
		RollingRestart:      c.Update.RollingRestart,
		LabelPrecedence:     c.Update.LabelPrecedence,
		PullFailureDelay:    c.Update.PullFailureDelay,
		LifecycleUID:        c.Lifecycle.UID,
		LifecycleGID:        c.Lifecycle.GID,
		RunOnce:             overrides.RunOnce || c.Mode.RunOnce,
		CurrentContainerID:  overrides.CurrentContainerID,
		UseComposeDependsOn: c.Update.UseComposeDependsOn,
		SkipSelfUpdate:      overrides.SkipSelfUpdate,
		EphemeralSelfUpdate: c.Update.EphemeralSelfUpdate,
		CooldownDelay:       c.Update.CooldownDelay,
		LabelEnable:         c.Filter.LabelEnable,
		DiskSpaceMax:        c.Update.DiskSpaceMaxBytes,
		DiskSpaceWarn:       c.Update.DiskSpaceWarnBytes,
		EnableGitMonitoring: c.Git.Enable,
		GitDefaultRef:       c.Git.DefaultRef,
		GitSemverPolicy:     c.Git.SemverPolicy,
		GitTimeout:          c.Git.Timeout,
		GitImages:           c.Git.Images,
		GitDockerfile:       c.Git.Dockerfile,
		GitContext:          c.Git.Context,
		GitComposeStash:     c.Git.ComposeStash,
		ComposeProjects:     c.Git.ComposeProjects,
		Registry:            c.Registry,
	}
}
