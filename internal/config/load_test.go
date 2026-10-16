package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/config/git"
	"github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/config/update"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// nopLog returns a discarded logger for Load and validate tests.
//
// Returns:
//   - *zerolog.Logger: Nop logger.
func nopLog() *zerolog.Logger {
	log := zerolog.Nop()

	return &log
}

// bindLoad registers flags, parses args, and binds a local Viper instance.
//
// Parameters:
//   - t: Test handle.
//   - args: CLI arguments after the command name.
//
// Returns:
//   - *viper.Viper: Bound configuration.
//   - *pflag.FlagSet: Persistent flags after ParseFlags.
//   - *cobra.Command: Command passed to Load.
func bindLoad(t *testing.T, args ...string) (*viper.Viper, *pflag.FlagSet, *cobra.Command) {
	t.Helper()

	flags.SetDefaults()

	cmd := &cobra.Command{Use: "watchtower"}
	flags.RegisterAll(cmd)
	require.NoError(t, cmd.ParseFlags(args))

	flagSet := cmd.PersistentFlags()
	vCfg := viper.New()
	require.NoError(t, flags.BindAll(vCfg, flagSet, flags.AllSpecs()))

	return vCfg, flagSet, cmd
}

func TestLoad_DefaultsAndAlignment(t *testing.T) {
	_, _, cmd := bindLoad(t)

	cfg, err := Load(nopLog(), cmd, []string{"/Foo", "Bar"})
	require.NoError(t, err)

	assert.Equal(t, "unix:///var/run/docker.sock", cfg.Docker.Host)
	assert.False(t, cfg.Docker.TLSVerify)
	assert.Equal(t, "auto", cfg.Client.CPUCopyMode)
	assert.False(t, cfg.Client.DisableMemorySwappiness)
	assert.Equal(t, "auto", cfg.Compatibility.CPUCopyMode)
	assert.Equal(t, 30*time.Second, cfg.Update.StopTimeout)
	assert.Equal(t, []string{"Foo", "Bar"}, cfg.Filter.Names)
	assert.NotNil(t, cfg.Filter.Predicate)
	assert.Equal(t, "auto", cfg.Logging.Format)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.False(t, cfg.Git.Enable)
	assert.Equal(t, git.DefaultRef, cfg.Git.DefaultRef)
	assert.Equal(t, git.DefaultPolicy, cfg.Git.SemverPolicy)
}

func TestLoad_BindError(t *testing.T) {
	cmd := &cobra.Command{Use: "watchtower"}

	_, err := Load(nopLog(), cmd, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "bind configuration:")
}

func TestLoad_APIVersionQuotesStripped(t *testing.T) {
	_, _, cmd := bindLoad(t, "--api-version", `"1.44"`)

	cfg, err := Load(nopLog(), cmd, nil)
	require.NoError(t, err)
	assert.Equal(t, "1.44", cfg.Docker.APIVersion)
}

func TestLoad_CompatCopiedOntoClient(t *testing.T) {
	_, _, cmd := bindLoad(t, "--cpu-copy-mode", "full", "--disable-memory-swappiness")

	cfg, err := Load(nopLog(), cmd, nil)
	require.NoError(t, err)
	assert.Equal(t, "full", cfg.Compatibility.CPUCopyMode)
	assert.Equal(t, "full", cfg.Client.CPUCopyMode)
	assert.True(t, cfg.Compatibility.DisableMemorySwappiness)
	assert.True(t, cfg.Client.DisableMemorySwappiness)
}

func TestLoad_RollingRestartWithMonitorOnly(t *testing.T) {
	_, _, cmd := bindLoad(t, "--rolling-restart", "--monitor-only")

	_, err := Load(nopLog(), cmd, nil)
	require.ErrorIs(t, err, ErrRollingRestartWithMonitorOnly)
}

func TestLoad_MonitorOnlyWithNoPullSucceeds(t *testing.T) {
	_, _, cmd := bindLoad(t, "--monitor-only", "--no-pull")

	cfg, err := Load(nopLog(), cmd, nil)
	require.NoError(t, err)
	assert.True(t, cfg.Update.MonitorOnly)
	assert.True(t, cfg.Update.NoPull)
}

func TestLoad_GitAndComposeErrors(t *testing.T) {
	t.Run("invalid git-image", func(t *testing.T) {
		_, _, cmd := bindLoad(t, "--git-image", "not-a-mapping")
		_, err := Load(nopLog(), cmd, nil)
		require.ErrorIs(t, err, git.ErrInvalidGitImage)
	})

	t.Run("invalid compose-project", func(t *testing.T) {
		_, _, cmd := bindLoad(t, "--compose-project", "nopair")
		_, err := Load(nopLog(), cmd, nil)
		require.ErrorIs(t, err, git.ErrInvalidComposeProject)
	})

	t.Run("missing ca bundle", func(t *testing.T) {
		_, _, cmd := bindLoad(t, "--git-ca-bundle", filepath.Join(t.TempDir(), "missing.pem"))
		_, err := Load(nopLog(), cmd, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "git-ca-bundle:")
	})
}

func TestLoad_InvalidFilterLabel(t *testing.T) {
	_, _, cmd := bindLoad(t, "--enable-containers-by-label", "not-a-pair")

	_, err := Load(nopLog(), cmd, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "build filter:")
}

func TestLoad_NegativeStopTimeout(t *testing.T) {
	_, _, cmd := bindLoad(t, "--stop-timeout", "-1s")

	_, err := Load(nopLog(), cmd, nil)
	require.ErrorIs(t, err, ErrNegativeStopTimeout)
}

func TestLoad_InvalidAndNegativeCooldown(t *testing.T) {
	t.Run("invalid duration", func(t *testing.T) {
		_, _, cmd := bindLoad(t, "--cooldown-delay", "not-a-duration")
		_, err := Load(nopLog(), cmd, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cooldown-delay:")
	})

	t.Run("negative duration", func(t *testing.T) {
		_, _, cmd := bindLoad(t, "--cooldown-delay", "-1h")
		_, err := Load(nopLog(), cmd, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cooldown-delay:")
	})
}

func TestLoadDocker(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "-H", "tcp://127.0.0.1:2375", "--tlsverify", "--api-version", `"1.43"`, "--cert-path", "/certs")

	got := loadDocker(vCfg)
	assert.Equal(t, "tcp://127.0.0.1:2375", got.Host)
	assert.True(t, got.TLSVerify)
	assert.Equal(t, "1.43", got.APIVersion)
	assert.Equal(t, "/certs", got.CertPath)
}

func TestLoadClient(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "--include-stopped", "--include-restarting", "--revive-stopped", "--remove-volumes", "--warn-on-head-failure", "always")

	got := loadClient(vCfg)
	assert.True(t, got.IncludeStopped)
	assert.True(t, got.IncludeRestarting)
	assert.True(t, got.ReviveStopped)
	assert.True(t, got.RemoveVolumes)
	assert.Equal(t, "always", got.WarnOnHeadFailure)
	assert.Empty(t, got.CPUCopyMode)
}

func TestLoadCompat(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "--disable-memory-swappiness", "--cpu-copy-mode", "none")

	got := loadCompat(vCfg)
	assert.True(t, got.DisableMemorySwappiness)
	assert.Equal(t, "none", got.CPUCopyMode)
}

func TestLoadSchedule(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "--interval", "120", "--schedule", "0 0 * * *", "--update-on-start")

	got := loadSchedule(vCfg)
	assert.Equal(t, 120, got.IntervalSeconds)
	assert.Equal(t, "0 0 * * *", got.Spec)
	assert.True(t, got.UpdateOnStart)
}

func TestLoadMode(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "--run-once", "--health-check", "--porcelain", "v1", "--self-update-orchestrator", "--no-startup-message")

	got := loadMode(vCfg)
	assert.True(t, got.RunOnce)
	assert.True(t, got.HealthCheck)
	assert.Equal(t, "v1", got.Porcelain)
	assert.True(t, got.SelfUpdateOrchestrator)
	assert.True(t, got.NoStartupMessage)
}

func TestLoadUpdate(t *testing.T) {
	t.Run("flags", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t,
			"--cleanup", "--no-pull", "--no-restart", "--monitor-only", "--rolling-restart",
			"--stop-timeout", "45s", "--cooldown-delay", "2h",
			"--use-compose-depends-on", "--label-take-precedence", "--ephemeral-self-update",
		)

		got, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.NoError(t, err)
		assert.True(t, got.Cleanup)
		assert.True(t, got.NoPull)
		assert.True(t, got.NoRestart)
		assert.True(t, got.MonitorOnly)
		assert.True(t, got.RollingRestart)
		assert.Equal(t, 45*time.Second, got.StopTimeout)
		assert.Equal(t, 2*time.Hour, got.CooldownDelay)
		assert.True(t, got.UseComposeDependsOn)
		assert.True(t, got.LabelPrecedence)
		assert.True(t, got.EphemeralSelfUpdate)
	})

	t.Run("empty cooldown", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t)
		got, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), got.CooldownDelay)
	})

	t.Run("subsecond timeout", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--stop-timeout", "500ms")
		got, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.NoError(t, err)
		assert.Equal(t, 500*time.Millisecond, got.StopTimeout)
	})

	t.Run("negative timeout", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--stop-timeout", "-5s")
		_, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.ErrorIs(t, err, ErrNegativeStopTimeout)
	})

	t.Run("invalid cooldown", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--cooldown-delay", "nope")
		_, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cooldown-delay:")
	})

	t.Run("negative cooldown", func(t *testing.T) {
		// util.ParseDuration rejects a leading '-' before loadUpdate can return
		// ErrNegativeCooldownDelay.
		vCfg, flagSet, _ := bindLoad(t, "--cooldown-delay", "-2d")
		_, err := loadUpdate(nopLog(), vCfg, flagSet)
		require.Error(t, err)
		assert.ErrorContains(t, err, "cooldown-delay:")
	})
}

func TestLoadLifecycle(t *testing.T) {
	vCfg, _, _ := bindLoad(t, "--enable-lifecycle-hooks", "--lifecycle-uid", "1000", "--lifecycle-gid", "1001")

	got := loadLifecycle(vCfg)
	assert.True(t, got.Enabled)
	assert.Equal(t, 1000, got.UID)
	assert.Equal(t, 1001, got.GID)
}

func TestNormalizedStringSlice(t *testing.T) {
	t.Run("lowercases values", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--disable-containers", "/Foo,Bar")

		got := normalizedStringSlice(
			vCfg, flagSet, "disable-containers",
			[]string{"WATCHTOWER_DISABLE_CONTAINERS"},
			strings.ToLower,
		)
		assert.Equal(t, []string{"/foo", "bar"}, got)
	})

	t.Run("empty when unset", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t)

		got := normalizedStringSlice(
			vCfg, flagSet, "disable-containers",
			[]string{"WATCHTOWER_DISABLE_CONTAINERS"},
			strings.ToLower,
		)
		assert.Empty(t, got)
	})
}

func TestLoadFilter(t *testing.T) {
	t.Run("names and lists", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t,
			"--label-enable",
			"--disable-containers", "/SkipMe",
			"--monitor-image-names", "nginx*",
			"--skip-image-names", "redis*",
			"--enable-containers-by-label", "env=prod",
			"--disable-containers-by-label", "skip=true",
			"--scope", "prod",
		)

		got, err := loadFilter(nopLog(), vCfg, flagSet, []string{"/App"})
		require.NoError(t, err)
		assert.True(t, got.LabelEnable)
		assert.Equal(t, []string{"SkipMe"}, got.DisableContainers)
		assert.Equal(t, []string{"nginx*"}, got.MonitorImageNames)
		assert.Equal(t, []string{"redis*"}, got.SkipImageNames)
		assert.Equal(t, []string{"env=prod"}, got.EnableContainersByLabel)
		assert.Equal(t, []string{"skip=true"}, got.DisableContainersByLabel)
		assert.Equal(t, "prod", got.Scope)
		assert.Equal(t, []string{"App"}, got.Names)
		assert.NotNil(t, got.Predicate)
		assert.NotEmpty(t, got.Desc)
	})

	t.Run("invalid label pair", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--disable-containers-by-label", "broken")
		_, err := loadFilter(nopLog(), vCfg, flagSet, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "build filter:")
	})
}

// TestLoadRegistry verifies that loadRegistry reads the TLS settings from
// their flags and the cooldown platform from its environment variables, and
// leaves an unset platform empty so the registry client uses the runtime
// platform.
func TestLoadRegistry(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("WATCHTOWER_COOLDOWN_PLATFORM_OS", "windows")
		t.Setenv("WATCHTOWER_COOLDOWN_PLATFORM_ARCH", "arm")
		t.Setenv("WATCHTOWER_COOLDOWN_PLATFORM_VARIANT", "v7")

		vCfg, _, _ := bindLoad(t, "--registry-tls-skip", "--registry-tls-min-version", "1.3")

		assert.Equal(t, registry.Registry{
			CooldownPlatform: registry.Platform{OS: "windows", Architecture: "arm", Variant: "v7"},
			TLSMinVersion:    "1.3",
			TLSSkip:          true,
		}, loadRegistry(vCfg))
	})

	t.Run("unset", func(t *testing.T) {
		for _, key := range []string{
			"WATCHTOWER_COOLDOWN_PLATFORM_OS",
			"WATCHTOWER_COOLDOWN_PLATFORM_ARCH",
			"WATCHTOWER_COOLDOWN_PLATFORM_VARIANT",
		} {
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}

		vCfg, _, _ := bindLoad(t)

		assert.Equal(t, registry.Registry{TLSMinVersion: "TLS1.2"}, loadRegistry(vCfg))
	})
}

func TestLoadAPI(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t,
			"--http-api-endpoints", "health,update",
			"--http-api-update", "--http-api-metrics", "--http-api-containers",
			"--http-api-host", "127.0.0.1", "--http-api-port", "8080",
			"--http-api-token", "tok", "--http-api-events-token", "ev",
			"--http-api-periodic-polls", "--http-api-rate-limit", "10",
			"--http-api-tls-cert", "cert.pem", "--http-api-tls-key", "key.pem",
			"--http-api-trusted-proxies", "10.0.0.0/8,172.16.0.0/12",
			"--http-api-proxy-header", "X-Real-IP",
			"--http-api-cors-origins", "https://app.example.com",
			"--http-api-check-timeout", "5s", "--http-api-update-timeout", "10s",
		)

		got := loadAPI(vCfg, flagSet)
		assert.Equal(t, []string{"health", "update"}, got.Endpoints)
		assert.True(t, got.LegacyUpdate)
		assert.True(t, got.LegacyMetrics)
		assert.True(t, got.LegacyContainers)
		assert.Equal(t, "127.0.0.1", got.Host)
		assert.True(t, got.HostChanged)
		assert.Equal(t, "8080", got.Port)
		assert.True(t, got.PortChanged)
		assert.Equal(t, "tok", got.Token)
		assert.Equal(t, "ev", got.EventsToken)
		assert.True(t, got.PeriodicPolls)
		assert.Equal(t, 10, got.RateLimit)
		assert.True(t, got.RateLimitChanged)
		assert.Equal(t, "cert.pem", got.TLSCert)
		assert.Equal(t, "key.pem", got.TLSKey)
		assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, got.TrustedProxies)
		assert.Equal(t, "X-Real-IP", got.ProxyHeader)
		assert.Equal(t, []string{"https://app.example.com"}, got.CORSOrigins)
		assert.Equal(t, 5*time.Second, got.CheckTimeout)
		assert.True(t, got.CheckTimeoutChanged)
		assert.Equal(t, 10*time.Second, got.UpdateTimeout)
		assert.True(t, got.UpdateTimeoutChanged)
	})

	t.Run("unchanged flags", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t)

		got := loadAPI(vCfg, flagSet)
		assert.False(t, got.HostChanged)
		assert.False(t, got.PortChanged)
		assert.False(t, got.RateLimitChanged)
		assert.False(t, got.CheckTimeoutChanged)
		assert.False(t, got.UpdateTimeoutChanged)
		assert.Empty(t, got.Endpoints)
		assert.Empty(t, got.TrustedProxies)
		assert.Empty(t, got.CORSOrigins)
	})
}

func TestLoadNotify(t *testing.T) {
	vCfg, flagSet, _ := bindLoad(t,
		"--notification-url", "gotify://example/token",
		"--notifications", "email,slack",
		"--notifications-level", "debug",
		"--notification-template", "{{.Title}}",
		"--notification-template-file", "/tpl.tmpl",
		"--notification-report",
		"--notification-split-by-container",
		"--notification-skip-title",
		"--notification-log-stdout",
		"--notifications-delay", "7",
		"--notifications-hostname", "box",
		"--notification-title-tag", "prod",
		"--notification-email-subjecttag", "mail",
		"--notification-email-from", "from@example.com",
		"--notification-email-to", "to@example.com",
		"--notification-email-server", "smtp.example.com",
		"--notification-email-server-user", "user",
		"--notification-email-server-password", "pw",
		"--notification-email-server-port", "587",
		"--notification-email-server-tls-skip-verify",
		"--notification-email-delay", "3",
		"--notification-slack-hook-url", "https://hooks.slack.example/x",
		"--notification-slack-identifier", "wt",
		"--notification-slack-channel", "#ops",
		"--notification-slack-icon-emoji", ":bell:",
		"--notification-slack-icon-url", "https://example.com/icon.png",
		"--notification-msteams-hook", "https://teams.example/hook",
		"--notification-gotify-url", "https://gotify.example",
		"--notification-gotify-token", "g-tok",
		"--notification-gotify-tls-skip-verify",
	)

	got := loadNotify(vCfg, flagSet)
	assert.Equal(t, []string{"gotify://example/token"}, got.URLs)
	assert.Equal(t, []string{"email", "slack"}, got.LegacyTypes)
	assert.Equal(t, "debug", got.Level)
	assert.Equal(t, "{{.Title}}", got.Template)
	assert.Equal(t, "/tpl.tmpl", got.TemplateFile)
	assert.True(t, got.Report)
	assert.True(t, got.SplitByContainer)
	assert.True(t, got.SkipTitle)
	assert.True(t, got.LogStdout)
	assert.Equal(t, 7, got.DelaySeconds)
	assert.Equal(t, "box", got.Hostname)
	assert.Equal(t, "prod", got.TitleTag)
	assert.Equal(t, "mail", got.EmailSubjectTag)
	assert.Equal(t, "from@example.com", got.Legacy.EmailFrom)
	assert.Equal(t, "to@example.com", got.Legacy.EmailTo)
	assert.Equal(t, "smtp.example.com", got.Legacy.EmailServer)
	assert.Equal(t, "user", got.Legacy.EmailUser)
	assert.Equal(t, "pw", got.Legacy.EmailPassword)
	assert.Equal(t, 587, got.Legacy.EmailPort)
	assert.True(t, got.Legacy.EmailTLSSkipVerify)
	assert.Equal(t, 3, got.Legacy.EmailDelay)
	assert.Equal(t, "https://hooks.slack.example/x", got.Legacy.SlackHookURL)
	assert.Equal(t, "wt", got.Legacy.SlackIdentifier)
	assert.Equal(t, "#ops", got.Legacy.SlackChannel)
	assert.Equal(t, ":bell:", got.Legacy.SlackIconEmoji)
	assert.Equal(t, "https://example.com/icon.png", got.Legacy.SlackIconURL)
	assert.Equal(t, "https://teams.example/hook", got.Legacy.MSTeamsHook)
	assert.Equal(t, "https://gotify.example", got.Legacy.GotifyURL)
	assert.Equal(t, "g-tok", got.Legacy.GotifyToken)
	assert.True(t, got.Legacy.GotifyTLSSkipVerify)
}

func TestLoadLogging(t *testing.T) {
	t.Run("explicit format", func(t *testing.T) {
		vCfg, _, _ := bindLoad(t, "--log-format", "json", "--log-level", "debug", "--debug", "--trace", "--no-color")
		got := loadLogging(vCfg)
		assert.Equal(t, "json", got.Format)
		assert.Equal(t, "debug", got.Level)
		assert.True(t, got.Debug)
		assert.True(t, got.Trace)
		assert.True(t, got.NoColor)
	})

	t.Run("empty format becomes auto", func(t *testing.T) {
		vCfg := viper.New()
		vCfg.Set("log-format", "")
		vCfg.Set("log-level", "warn")
		got := loadLogging(vCfg)
		assert.Equal(t, "auto", got.Format)
		assert.Equal(t, "warn", got.Level)
	})
}

func TestLoadGit(t *testing.T) {
	t.Run("flags", func(t *testing.T) {
		dir := t.TempDir()
		bundle := filepath.Join(dir, "ca.pem")
		require.NoError(t, os.WriteFile(bundle, []byte("pem"), 0o600))

		vCfg, flagSet, _ := bindLoad(t,
			"--git-enable",
			"--git-image", "myapp:latest=https://github.com/org/app.git#main@minor",
			"--git-timeout", "45s",
			"--git-auth-token", "tok",
			"--git-username", "user",
			"--git-password", "pw",
			"--git-ssh-key-path", "/key",
			"--git-ssh-known-hosts", "/known",
			"--git-ca-bundle", bundle,
			"--git-insecure-skip-tls",
			"--git-dockerfile", "  build/docker/Dockerfile  ",
			"--git-context", "  .  ",
			"--git-compose-stash",
			"--compose-project", "web=/srv/web",
		)

		got, err := loadGit(vCfg, flagSet)
		require.NoError(t, err)
		assert.True(t, got.Enable)
		assert.Equal(t, git.DefaultRef, got.DefaultRef)
		assert.Equal(t, git.DefaultPolicy, got.SemverPolicy)
		assert.Equal(t, 45*time.Second, got.Timeout)
		assert.Equal(t, "tok", got.Token)
		assert.Equal(t, "user", got.Username)
		assert.Equal(t, "pw", got.Password)
		assert.Equal(t, "/key", got.SSHKeyPath)
		assert.Equal(t, "/known", got.SSHKnownHosts)
		assert.Equal(t, []byte("pem"), got.CABundle)
		assert.True(t, got.InsecureSkipTLS)
		assert.Equal(t, "build/docker/Dockerfile", got.Dockerfile)
		assert.Equal(t, ".", got.Context)
		assert.Equal(t, types.GitPolicyMinor, got.Images["myapp:latest"].Policy)
		assert.True(t, got.ComposeStash)
		assert.Equal(t, "/srv/web", got.ComposeProjects["web"])
	})

	t.Run("defaults", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t)
		got, err := loadGit(vCfg, flagSet)
		require.NoError(t, err)
		assert.False(t, got.Enable)
		assert.Equal(t, git.DefaultRef, got.DefaultRef)
		assert.Equal(t, git.DefaultPolicy, got.SemverPolicy)
		assert.Empty(t, got.Images)
		assert.Empty(t, got.ComposeProjects)
		assert.Nil(t, got.CABundle)
	})

	t.Run("invalid image", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--git-image", "nope")
		_, err := loadGit(vCfg, flagSet)
		require.ErrorIs(t, err, git.ErrInvalidGitImage)
		assert.ErrorContains(t, err, "git-image:")
	})

	t.Run("invalid compose project", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--compose-project", "nopair")
		_, err := loadGit(vCfg, flagSet)
		require.ErrorIs(t, err, git.ErrInvalidComposeProject)
		assert.ErrorContains(t, err, "compose-project:")
	})

	t.Run("missing ca bundle", func(t *testing.T) {
		vCfg, flagSet, _ := bindLoad(t, "--git-ca-bundle", filepath.Join(t.TempDir(), "missing.pem"))
		_, err := loadGit(vCfg, flagSet)
		require.Error(t, err)
		assert.ErrorContains(t, err, "git-ca-bundle:")
	})
}

func TestReadOptionalFile(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := readOptionalFile("  ")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("reads file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ca.pem")
		require.NoError(t, os.WriteFile(path, []byte("cert"), 0o600))
		got, err := readOptionalFile(path)
		require.NoError(t, err)
		assert.Equal(t, []byte("cert"), got)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := readOptionalFile(filepath.Join(t.TempDir(), "missing"))
		require.Error(t, err)
	})
}

func TestValidate(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		require.NoError(t, validate(nopLog(), Config{}))
	})

	t.Run("rolling and monitor-only", func(t *testing.T) {
		err := validate(nopLog(), Config{
			Update: update.Update{
				RollingRestart: true,
				MonitorOnly:    true,
			},
		})
		require.ErrorIs(t, err, ErrRollingRestartWithMonitorOnly)
	})

	t.Run("monitor-only and no-pull warns", func(t *testing.T) {
		err := validate(nopLog(), Config{
			Update: update.Update{
				MonitorOnly: true,
				NoPull:      true,
			},
		})
		require.NoError(t, err)
	})
}

func TestFlagChanged(t *testing.T) {
	t.Run("missing flag", func(t *testing.T) {
		flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
		assert.False(t, flagChanged(flagSet, "http-api-host"))
	})

	t.Run("unchanged", func(t *testing.T) {
		_, flagSet, _ := bindLoad(t)
		assert.False(t, flagChanged(flagSet, "http-api-host"))
	})

	t.Run("changed", func(t *testing.T) {
		_, flagSet, _ := bindLoad(t, "--http-api-host", "127.0.0.1")
		assert.True(t, flagChanged(flagSet, "http-api-host"))
	})
}
