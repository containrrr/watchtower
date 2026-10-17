package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/config/client"
	"github.com/nicholas-fedor/watchtower/internal/config/compatibility"
	"github.com/nicholas-fedor/watchtower/internal/config/filter"
	"github.com/nicholas-fedor/watchtower/internal/config/git"
	"github.com/nicholas-fedor/watchtower/internal/config/lifecycle"
	"github.com/nicholas-fedor/watchtower/internal/config/mode"
	"github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/config/update"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateParamsAssignsEveryField ensures UpdateParams sets every exported
// field on types.UpdateParams so scheduled and run-once paths cannot omit policy.
func TestUpdateParamsAssignsEveryField(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Client: client.Client{
			ReviveStopped: true,
			CPUCopyMode:   "full",
		},
		Compatibility: compatibility.Compatibility{
			CPUCopyMode: "auto",
		},
		Mode: mode.Mode{
			RunOnce: false,
		},
		Update: update.Update{
			Cleanup:             true,
			NoPull:              true,
			NoRestart:           true,
			MonitorOnly:         true,
			RollingRestart:      false,
			StopTimeout:         30 * time.Second,
			CooldownDelay:       24 * time.Hour,
			UseComposeDependsOn: true,
			LabelPrecedence:     true,
			EnableChangelog:     true,
			EphemeralSelfUpdate: true,
			PullFailureDelay:    5 * time.Second,
			DiskSpaceMaxBytes:   40_000_000_000,
			DiskSpaceWarnBytes:  32_000_000_000,
		},
		Lifecycle: lifecycle.Lifecycle{
			Enabled: true,
			UID:     1000,
			GID:     1000,
		},
		Filter: filter.Filter{
			Predicate:   filters.NoFilter,
			Desc:        "all",
			LabelEnable: true,
		},
		Git: git.Git{
			Enable:          true,
			DefaultRef:      git.DefaultRef,
			SemverPolicy:    git.DefaultPolicy,
			Timeout:         30 * time.Second,
			Dockerfile:      "build/docker/Dockerfile",
			Context:         ".",
			ComposeStash:    true,
			ComposeProjects: map[string]string{"demo": "/compose/demo"},
			Images: map[string]types.GitImage{
				"myapp:latest": {
					Repo:   "https://github.com/org/app.git",
					Ref:    "main",
					Policy: types.GitPolicyMinor,
				},
			},
		},
		Registry: registry.Registry{
			CooldownPlatform: registry.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			TLSMinVersion:    "TLS1.3",
			TLSSkip:          true,
		},
	}

	ov := RunOverrides{
		RunOnce:            true,
		SkipSelfUpdate:     true,
		CurrentContainerID: types.ContainerID("abc123"),
	}

	params := cfg.UpdateParams(ov)

	assert.NotNil(t, params.Filter)
	assert.True(t, params.Cleanup)
	assert.True(t, params.NoRestart)
	assert.True(t, params.ReviveStopped)
	assert.Equal(t, 30*time.Second, params.Timeout)
	assert.True(t, params.MonitorOnly)
	assert.True(t, params.NoPull)
	assert.True(t, params.LifecycleHooks)
	assert.False(t, params.RollingRestart)
	assert.True(t, params.LabelPrecedence)
	assert.True(t, params.EnableChangelog)
	assert.Equal(t, 5*time.Second, params.PullFailureDelay)
	assert.Equal(t, 1000, params.LifecycleUID)
	assert.Equal(t, 1000, params.LifecycleGID)
	assert.True(t, params.RunOnce)
	assert.Equal(t, types.ContainerID("abc123"), params.CurrentContainerID)
	assert.True(t, params.UseComposeDependsOn)
	assert.True(t, params.SkipSelfUpdate)
	assert.True(t, params.EphemeralSelfUpdate)
	assert.Equal(t, 24*time.Hour, params.CooldownDelay)
	assert.Equal(t, int64(40_000_000_000), params.DiskSpaceMax)
	assert.Equal(t, int64(32_000_000_000), params.DiskSpaceWarn)
	assert.True(t, params.EnableGitMonitoring)
	assert.Equal(t, git.DefaultRef, params.GitDefaultRef)
	assert.Equal(t, git.DefaultPolicy, params.GitSemverPolicy)
	assert.Equal(t, 30*time.Second, params.GitTimeout)
	assert.Equal(t, types.GitImage{
		Repo:   "https://github.com/org/app.git",
		Ref:    "main",
		Policy: types.GitPolicyMinor,
	}, params.GitImages["myapp:latest"])
	assert.Equal(t, "build/docker/Dockerfile", params.GitDockerfile)
	assert.Equal(t, ".", params.GitContext)
	assert.Equal(t, cfg.Registry, params.Registry)
	assert.True(t, params.GitComposeStash)
	assert.Equal(t, "/compose/demo", params.ComposeProjects["demo"])

	// Exhaustiveness: every exported field must be non-zero in this fixture
	// (Filter is a func; RunOnce and SkipSelfUpdate come from overrides).
	val := reflect.ValueOf(params)
	typ := val.Type()

	for i := range val.NumField() {
		field := typ.Field(i)
		fv := val.Field(i)

		switch field.Name {
		case "RollingRestart":
			// Intentionally false in fixture to prove false is set, not omitted.
			assert.False(t, fv.Bool())
		case "Filter":
			assert.False(t, fv.IsNil(), "Filter must be assigned")
		default:
			assert.False(t, fv.IsZero(), "field %s must be assigned by UpdateParams", field.Name)
		}
	}
}

// TestUpdateParamsOverrideFilter replaces the config filter when provided.
func TestUpdateParamsOverrideFilter(t *testing.T) {
	t.Parallel()

	override := func(types.FilterableContainer) bool { return false }

	cfg := Config{
		Filter:        filter.Filter{Predicate: filters.NoFilter},
		Compatibility: compatibility.Compatibility{CPUCopyMode: "none"},
	}

	params := cfg.UpdateParams(RunOverrides{Filter: override})
	require.NotNil(t, params.Filter)
	assert.False(t, params.Filter(nil))
}

// TestClientOptionsMapsClientAndCompat projects client construction options.
func TestClientOptionsMapsClientAndCompat(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Client: client.Client{
			IncludeStopped:    true,
			IncludeRestarting: true,
			ReviveStopped:     true,
			RemoveVolumes:     true,
			WarnOnHeadFailure: "always",
			CPUCopyMode:       "full",
		},
		Compatibility: compatibility.Compatibility{
			DisableMemorySwappiness: true,
			CPUCopyMode:             "auto",
		},
		Registry: registry.Registry{TLSMinVersion: "TLS1.3", TLSSkip: true},
	}

	opts := cfg.ClientOptions()
	assert.True(t, opts.IncludeStopped)
	assert.True(t, opts.IncludeRestarting)
	assert.True(t, opts.ReviveStopped)
	assert.True(t, opts.RemoveVolumes)
	assert.True(t, opts.DisableMemorySwappiness)
	// Compatibility wins over Client when both are non-empty.
	assert.Equal(t, "auto", opts.CPUCopyMode)
	assert.Equal(t, "always", string(opts.WarnOnHeadFailed))
	assert.Equal(t, cfg.Registry, opts.Registry)

	// Client is used when Compatibility is empty.
	cfg.Compatibility.CPUCopyMode = ""
	assert.Equal(t, "full", cfg.ClientOptions().CPUCopyMode)
}
