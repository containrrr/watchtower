package config

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// testLogger returns a discarded zerolog logger for tests that do not assert on logs.
func testLogger() *zerolog.Logger {
	nop := zerolog.Nop()

	return &nop
}

// newLoadedCommand registers flags, applies env, parses args, and loads config.
func newLoadedCommand(t *testing.T, env map[string]string, args ...string) Config {
	t.Helper()

	for k, v := range env {
		t.Setenv(k, v)
	}

	cmd := &cobra.Command{Use: "watchtower"}

	flags.RegisterAll(cmd)
	require.NoError(t, cmd.ParseFlags(args))

	flagSet := cmd.PersistentFlags()
	require.NoError(t, flags.ApplyEnvToFlags(flagSet, flags.AllSpecs()))

	log := logging.New(io.Discard, logging.InfoLevel)
	flags.ProcessFlagAliases(log, flagSet)
	flags.GetSecretsFromFiles(log, cmd)

	cfg, err := Load(testLogger(), cmd, nil)
	require.NoError(t, err)

	return cfg
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	cfg := newLoadedCommand(t,
		map[string]string{
			"WATCHTOWER_CLEANUP":      "false",
			"WATCHTOWER_MONITOR_ONLY": "true",
			"WATCHTOWER_TIMEOUT":      "30",
		},
		"--cleanup",
		"--monitor-only=false",
		"--stop-timeout", "10s",
	)

	assert.True(t, cfg.Update.Cleanup, "flag must override env for cleanup")
	assert.False(t, cfg.Update.MonitorOnly, "flag must override env for monitor-only")
	assert.Equal(t, 10*time.Second, cfg.Update.StopTimeout, "flag must override env for timeout")
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_CLEANUP":       "true",
		"WATCHTOWER_NO_PULL":       "true",
		"WATCHTOWER_TIMEOUT":       "45",
		"WATCHTOWER_POLL_INTERVAL": "120",
	})

	assert.True(t, cfg.Update.Cleanup)
	assert.True(t, cfg.Update.NoPull)
	assert.Equal(t, 45*time.Second, cfg.Update.StopTimeout, "bare-second env duration")
	assert.Equal(t, 120, cfg.Schedule.IntervalSeconds)
}

func TestLoad_DefaultWhenUnset(t *testing.T) {
	cfg := newLoadedCommand(t, nil)

	assert.False(t, cfg.Update.Cleanup)
	assert.False(t, cfg.Update.MonitorOnly)
	assert.False(t, cfg.Update.NoPull)
	assert.Equal(t, 30*time.Second, cfg.Update.StopTimeout)
}

func TestLoad_ListParseStrategies(t *testing.T) {
	t.Run("comma or space disable-containers", func(t *testing.T) {
		cfg := newLoadedCommand(t, map[string]string{
			"WATCHTOWER_DISABLE_CONTAINERS": "a,b c",
		})
		assert.ElementsMatch(t, []string{"a", "b", "c"}, cfg.Filter.DisableContainers)
	})

	t.Run("comma only label values keep spaces", func(t *testing.T) {
		cfg := newLoadedCommand(t, map[string]string{
			"WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL": "env=prod east,env=prod west",
		})
		assert.Equal(t, []string{"env=prod east", "env=prod west"}, cfg.Filter.EnableContainersByLabel)
	})

	t.Run("notification urls preserve embedded commas", func(t *testing.T) {
		cfg := newLoadedCommand(t, map[string]string{
			"WATCHTOWER_NOTIFICATION_URL": "teams://token@tenant/group/id?host=host,title=Hello discord://token",
		})
		require.Len(t, cfg.Notify.URLs, 2)
		assert.Contains(t, cfg.Notify.URLs[0], "host,title=Hello")
		assert.Contains(t, cfg.Notify.URLs[1], "discord://")
	})
}

func TestLoad_CooldownExtendedUnits(t *testing.T) {
	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_COOLDOWN_DELAY": "3d",
	})
	assert.Equal(t, 72*time.Hour, cfg.Update.CooldownDelay)
}

func TestUpdateParams_CompleteSnapshot(t *testing.T) {
	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_CLEANUP":                "true",
		"WATCHTOWER_REVIVE_STOPPED":         "true",
		"WATCHTOWER_USE_COMPOSE_DEPENDS_ON": "true",
		"WATCHTOWER_LIFECYCLE_HOOKS":        "true",
		"WATCHTOWER_LABEL_TAKE_PRECEDENCE":  "true",
	})

	params := cfg.UpdateParams(RunOverrides{})
	assert.True(t, params.Cleanup)
	assert.True(t, params.ReviveStopped, "ReviveStopped projects from Client domain")
	assert.True(t, params.UseComposeDependsOn)
	assert.True(t, params.LifecycleHooks)
	assert.True(t, params.LabelPrecedence)
	assert.NotNil(t, params.Filter)
	assert.True(t, cfg.Client.ReviveStopped)
}

// TestLoad_APIChangedReflectsCLIOnly verifies env-only API transport settings do not
// set *Changed fields (avoids false "API config set without endpoints" warnings).
func TestLoad_APIChangedReflectsCLIOnly(t *testing.T) {
	t.Run("env only leaves Changed false", func(t *testing.T) {
		cfg := newLoadedCommand(t, map[string]string{
			"WATCHTOWER_HTTP_API_HOST":           "127.0.0.1",
			"WATCHTOWER_HTTP_API_PORT":           "9090",
			"WATCHTOWER_HTTP_API_RATE_LIMIT":     "30",
			"WATCHTOWER_HTTP_API_CHECK_TIMEOUT":  "2m",
			"WATCHTOWER_HTTP_API_UPDATE_TIMEOUT": "3m",
		})

		assert.Equal(t, "127.0.0.1", cfg.API.Host)
		assert.Equal(t, "9090", cfg.API.Port)
		assert.Equal(t, 30, cfg.API.RateLimit)
		assert.Equal(t, 2*time.Minute, cfg.API.CheckTimeout)
		assert.Equal(t, 3*time.Minute, cfg.API.UpdateTimeout)

		assert.False(t, cfg.API.HostChanged)
		assert.False(t, cfg.API.PortChanged)
		assert.False(t, cfg.API.RateLimitChanged)
		assert.False(t, cfg.API.CheckTimeoutChanged)
		assert.False(t, cfg.API.UpdateTimeoutChanged)

		run, err := cfg.BuildRunConfig(RunConfigInput{})
		require.NoError(t, err)
		assert.False(t, AnyHTTPAPIConfig(run),
			"env-only host/port/rate/timeouts must not trip the no-endpoint warning")
	})

	t.Run("CLI sets Changed true", func(t *testing.T) {
		cfg := newLoadedCommand(t, nil,
			"--http-api-host", "127.0.0.1",
			"--http-api-port", "9090",
			"--http-api-rate-limit", "30",
		)

		assert.True(t, cfg.API.HostChanged)
		assert.True(t, cfg.API.PortChanged)
		assert.True(t, cfg.API.RateLimitChanged)

		run, err := cfg.BuildRunConfig(RunConfigInput{})
		require.NoError(t, err)
		assert.True(t, AnyHTTPAPIConfig(run),
			"CLI host/port/rate-limit should still trip the no-endpoint warning")
	})
}

// TestLoad_NotificationURLFromSecretFile_ResolvesContentNotPath verifies that
// a notification-url env pointing to a file resolves to the file content,
// not the file path, in Config.Notify.URLs.
func TestLoad_NotificationURLFromSecretFile_ResolvesContentNotPath(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "watchtower-")
	require.NoError(t, err)
	_, err = file.WriteString("gotify://gotify.example.com/token123")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_NOTIFICATION_URL": file.Name(),
	})

	require.Len(t, cfg.Notify.URLs, 1)
	assert.Equal(t, "gotify://gotify.example.com/token123", cfg.Notify.URLs[0])
	assert.NotContains(t, cfg.Notify.URLs[0], "/run/secrets/")
}

// TestLoad_KindStringSecretFromFiles_ResolveContent verifies that KindString
// notification secrets resolve their file contents through config.Load.
func TestLoad_KindStringSecretFromFiles_ResolveContent(t *testing.T) {
	emailFile, err := os.CreateTemp(t.TempDir(), "watchtower-")
	require.NoError(t, err)
	_, err = emailFile.WriteString("supersecretstring")
	require.NoError(t, err)
	require.NoError(t, emailFile.Close())

	slackFile, err := os.CreateTemp(t.TempDir(), "watchtower-")
	require.NoError(t, err)
	_, err = slackFile.WriteString("slack://token@channel")
	require.NoError(t, err)
	require.NoError(t, slackFile.Close())

	msteamsFile, err := os.CreateTemp(t.TempDir(), "watchtower-")
	require.NoError(t, err)
	_, err = msteamsFile.WriteString("https://outlook.office.com/webhook/abc123")
	require.NoError(t, err)
	require.NoError(t, msteamsFile.Close())

	gotifyFile, err := os.CreateTemp(t.TempDir(), "watchtower-")
	require.NoError(t, err)
	_, err = gotifyFile.WriteString("gotify_token_abc")
	require.NoError(t, err)
	require.NoError(t, gotifyFile.Close())

	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD": emailFile.Name(),
		"WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL":        slackFile.Name(),
		"WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL":      msteamsFile.Name(),
		"WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN":          gotifyFile.Name(),
	})

	assert.Equal(t, "supersecretstring", cfg.Notify.Legacy.EmailPassword)
	assert.Equal(t, "slack://token@channel", cfg.Notify.Legacy.SlackHookURL)
	assert.Equal(t, "https://outlook.office.com/webhook/abc123", cfg.Notify.Legacy.MSTeamsHook)
	assert.Equal(t, "gotify_token_abc", cfg.Notify.Legacy.GotifyToken)
}

func TestLoad_GitEnvImageAndEnable(t *testing.T) {
	cfg := newLoadedCommand(t, map[string]string{
		"WATCHTOWER_GIT_IMAGE":  "myapp:latest=https://github.com/org/app.git#develop@patch",
		"WATCHTOWER_GIT_ENABLE": "true",
	})

	assert.True(t, cfg.Git.Enable)
	assert.Equal(t, "develop", cfg.Git.Images["myapp:latest"].Ref)
	assert.Equal(t, types.GitPolicyPatch, cfg.Git.Images["myapp:latest"].Policy)
}
