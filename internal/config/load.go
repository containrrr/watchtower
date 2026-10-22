package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/nicholas-fedor/watchtower/internal/config/api"
	"github.com/nicholas-fedor/watchtower/internal/config/client"
	"github.com/nicholas-fedor/watchtower/internal/config/compatibility"
	"github.com/nicholas-fedor/watchtower/internal/config/docker"
	"github.com/nicholas-fedor/watchtower/internal/config/filter"
	gitConfig "github.com/nicholas-fedor/watchtower/internal/config/git"
	"github.com/nicholas-fedor/watchtower/internal/config/lifecycle"
	"github.com/nicholas-fedor/watchtower/internal/config/logging"
	"github.com/nicholas-fedor/watchtower/internal/config/mode"
	"github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/config/schedule"
	"github.com/nicholas-fedor/watchtower/internal/config/update"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/internal/flags/spec"
	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
)

// diskSpacePercentBase is the divisor used to convert a 0-100 percentage into a fraction of max.
const diskSpacePercentBase = 100

var (
	// errNegativeStopTimeout indicates stop-timeout was set to a negative duration.
	errNegativeStopTimeout = errors.New("stop-timeout must be non-negative")
	// errNegativeCooldownDelay indicates cooldown-delay was set to a negative duration.
	errNegativeCooldownDelay = errors.New("cooldown-delay must be non-negative")
	// errRollingRestartWithMonitorOnly indicates incompatible rolling-restart and monitor-only flags.
	errRollingRestartWithMonitorOnly = errors.New(
		"rolling-restart and monitor-only cannot both be enabled",
	)
	// errDiskSpaceMaxPercent indicates disk-space-max was given as a percentage.
	errDiskSpaceMaxPercent = errors.New("disk-space-max cannot be a percentage")
	// errDiskSpacePercentWithoutMax indicates a percentage warn value without disk-space-max.
	errDiskSpacePercentWithoutMax = errors.New(
		"disk-space-warn percentage requires disk-space-max",
	)
	// errDiskSpacePercentOutOfRange indicates a percentage outside (0, 100].
	errDiskSpacePercentOutOfRange = errors.New(
		"disk-space percentage must be greater than 0 and at most 100",
	)
	// errDiskSpaceWarnNotBelowMax indicates the warn threshold is not below the max.
	errDiskSpaceWarnNotBelowMax = errors.New(
		"disk-space-warn must be below disk-space-max",
	)
)

// Load reads resolved settings from a parsed Cobra command into Config.
//
// Callers must run flag registration, ProcessFlagAliases, SetupLogging, and
// GetSecretsFromFiles before Load. Load does not re-parse CLI arguments.
//
// Values are resolved through a process-local Viper instance (flag > env >
// static default) using FlagSpec metadata from every domain.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - cmd: Parsed root command with persistent flags populated.
//   - args: Positional container name arguments for filtering.
//
// Returns:
//   - Config: Immutable configuration snapshot.
//   - error: Non-nil when required flags are missing or values are invalid.
func Load(log *zerolog.Logger, cmd *cobra.Command, args []string) (Config, error) {
	flagSet := cmd.PersistentFlags()

	vCfg := viper.New()

	err := flags.BindAll(vCfg, flagSet, flags.AllSpecs())
	if err != nil {
		return Config{}, fmt.Errorf("bind configuration: %w", err)
	}

	cfg := Config{}

	cfg.Docker = loadDocker(vCfg)
	cfg.Client = loadClient(vCfg)
	cfg.Compatibility = loadCompat(vCfg)

	// Keep client and compat CPU/memory fields aligned for projections.
	if cfg.Client.CPUCopyMode == "" {
		cfg.Client.CPUCopyMode = cfg.Compatibility.CPUCopyMode
	}

	if !cfg.Client.DisableMemorySwappiness {
		cfg.Client.DisableMemorySwappiness = cfg.Compatibility.DisableMemorySwappiness
	}

	cfg.Schedule = loadSchedule(vCfg)
	cfg.Mode = loadMode(vCfg)

	cfg.Update, err = loadUpdate(log, vCfg, flagSet)
	if err != nil {
		return Config{}, err
	}

	cfg.Lifecycle = loadLifecycle(vCfg)

	cfg.Filter, err = loadFilter(log, vCfg, flagSet, args)
	if err != nil {
		return Config{}, err
	}

	cfg.Registry = loadRegistry(vCfg)
	cfg.API = loadAPI(vCfg, flagSet)
	cfg.Notify = loadNotify(vCfg, flagSet)
	cfg.Logging = loadLogging(vCfg)

	cfg.Git, err = loadGit(vCfg, flagSet)
	if err != nil {
		return Config{}, err
	}

	err = validate(log, cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// loadDocker reads Docker connection settings from Viper after BindAll.
func loadDocker(vCfg *viper.Viper) docker.Docker {
	return docker.Docker{
		Host:       vCfg.GetString("host"),
		TLSVerify:  vCfg.GetBool("tlsverify"),
		APIVersion: strings.Trim(vCfg.GetString("api-version"), "\""),
		CertPath:   vCfg.GetString("cert-path"),
	}
}

// loadClient reads Docker client construction settings from Viper.
func loadClient(vCfg *viper.Viper) client.Client {
	return client.Client{
		IncludeStopped:    vCfg.GetBool("include-stopped"),
		IncludeRestarting: vCfg.GetBool("include-restarting"),
		ReviveStopped:     vCfg.GetBool("revive-stopped"),
		RemoveVolumes:     vCfg.GetBool("remove-volumes"),
		WarnOnHeadFailure: vCfg.GetString("warn-on-head-failure"),
	}
}

// loadCompat reads runtime compatibility settings from Viper.
func loadCompat(vCfg *viper.Viper) compatibility.Compatibility {
	return compatibility.Compatibility{
		DisableMemorySwappiness: vCfg.GetBool("disable-memory-swappiness"),
		CPUCopyMode:             vCfg.GetString("cpu-copy-mode"),
	}
}

// loadSchedule reads schedule settings from Viper.
func loadSchedule(vCfg *viper.Viper) schedule.Schedule {
	return schedule.Schedule{
		IntervalSeconds: vCfg.GetInt("interval"),
		Spec:            vCfg.GetString("schedule"),
		UpdateOnStart:   vCfg.GetBool("update-on-start"),
	}
}

// loadMode reads process mode settings from Viper.
func loadMode(vCfg *viper.Viper) mode.Mode {
	return mode.Mode{
		RunOnce:                vCfg.GetBool("run-once"),
		HealthCheck:            vCfg.GetBool("health-check"),
		Porcelain:              vCfg.GetString("porcelain"),
		SelfUpdateOrchestrator: vCfg.GetBool("self-update-orchestrator"),
		NoStartupMessage:       vCfg.GetBool("no-startup-message"),
	}
}

// loadUpdate reads update policy settings from Viper.
func loadUpdate(log *zerolog.Logger, vCfg *viper.Viper, flagSet *pflag.FlagSet) (update.Update, error) {
	stopTimeout := durationValue(vCfg, flagSet, "stop-timeout", []string{"WATCHTOWER_TIMEOUT"})

	if stopTimeout < 0 {
		return update.Update{}, errNegativeStopTimeout
	}

	if stopTimeout > 0 && stopTimeout < time.Second {
		log.Warn().
			Dur("timeout", stopTimeout).
			Msg("WATCHTOWER_TIMEOUT is less than 1 second")
	}

	cooldownStr := vCfg.GetString("cooldown-delay")

	var cooldown time.Duration

	if cooldownStr != "" {
		parsed, err := util.ParseDuration(cooldownStr)
		if err != nil {
			return update.Update{}, fmt.Errorf("cooldown-delay: %w", err)
		}

		if parsed < 0 {
			return update.Update{}, errNegativeCooldownDelay
		}

		cooldown = parsed
	}

	maxRaw := vCfg.GetString("disk-space-max")
	warnRaw := vCfg.GetString("disk-space-warn")

	maxBytes, warnBytes, err := resolveDiskSpaceThresholds(maxRaw, warnRaw)
	if err != nil {
		return update.Update{}, err
	}

	return update.Update{
		Cleanup:             vCfg.GetBool("cleanup"),
		NoPull:              vCfg.GetBool("no-pull"),
		EnableChangelog:     vCfg.GetBool("enable-changelog"),
		NoRestart:           vCfg.GetBool("no-restart"),
		MonitorOnly:         vCfg.GetBool("monitor-only"),
		RollingRestart:      vCfg.GetBool("rolling-restart"),
		StopTimeout:         stopTimeout,
		CooldownDelay:       cooldown,
		UseComposeDependsOn: vCfg.GetBool("use-compose-depends-on"),
		LabelPrecedence:     vCfg.GetBool("label-take-precedence"),
		EphemeralSelfUpdate: vCfg.GetBool("ephemeral-self-update"),
		DiskSpaceMax:        maxRaw,
		DiskSpaceWarn:       warnRaw,
		DiskSpaceMaxBytes:   maxBytes,
		DiskSpaceWarnBytes:  warnBytes,
	}, nil
}

// resolveDiskSpaceThresholds parses disk-space-max and disk-space-warn into bytes.
//
// Max must be an absolute size. Warn may be absolute or a percent of max.
// When both are set, warn must be strictly below max.
//
// Parameters:
//   - maxRaw: Raw disk-space-max value.
//   - warnRaw: Raw disk-space-warn value.
//
// Returns:
//   - int64: Parsed max in bytes, or 0 when unset.
//   - int64: Parsed warn in bytes, or 0 when unset.
//   - error: Non-nil when values are invalid or inconsistent.
func resolveDiskSpaceThresholds(maxRaw, warnRaw string) (int64, int64, error) {
	maxRaw = strings.TrimSpace(maxRaw)
	warnRaw = strings.TrimSpace(warnRaw)

	if strings.HasSuffix(maxRaw, "%") {
		return 0, 0, errDiskSpaceMaxPercent
	}

	maxBytes, err := util.ParseDiskSpace(maxRaw)
	if err != nil {
		return 0, 0, fmt.Errorf("disk-space-max: %w", err)
	}

	warnBytes, err := parseDiskSpaceWarn(warnRaw, maxBytes)
	if err != nil {
		return 0, 0, err
	}

	if maxBytes > 0 && warnBytes > 0 && warnBytes >= maxBytes {
		return 0, 0, errDiskSpaceWarnNotBelowMax
	}

	return maxBytes, warnBytes, nil
}

// parseDiskSpaceWarn parses an absolute or percentage warning threshold.
//
// Parameters:
//   - raw: Raw disk-space-warn value.
//   - maxBytes: Parsed disk-space-max in bytes; required when raw is a percentage.
//
// Returns:
//   - int64: Parsed warn threshold in bytes, or 0 when unset.
//   - error: Non-nil when the value is invalid.
func parseDiskSpaceWarn(raw string, maxBytes int64) (int64, error) {
	if raw == "" || raw == "0" {
		return 0, nil
	}

	if strings.HasSuffix(raw, "%") {
		if maxBytes <= 0 {
			return 0, errDiskSpacePercentWithoutMax
		}

		pctStr := strings.TrimSpace(strings.TrimSuffix(raw, "%"))

		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return 0, fmt.Errorf("disk-space-warn: invalid percentage %q: %w", raw, err)
		}

		if pct <= 0 || pct > diskSpacePercentBase {
			return 0, errDiskSpacePercentOutOfRange
		}

		return int64(float64(maxBytes) * pct / diskSpacePercentBase), nil
	}

	warnBytes, err := util.ParseDiskSpace(raw)
	if err != nil {
		return 0, fmt.Errorf("disk-space-warn: %w", err)
	}

	return warnBytes, nil
}

// loadLifecycle reads lifecycle hook settings from Viper.
func loadLifecycle(vCfg *viper.Viper) lifecycle.Lifecycle {
	return lifecycle.Lifecycle{
		Enabled: vCfg.GetBool("enable-lifecycle-hooks"),
		UID:     vCfg.GetInt("lifecycle-uid"),
		GID:     vCfg.GetInt("lifecycle-gid"),
	}
}

// normalizedStringSlice loads a comma- or space-separated list flag/env value
// and applies normalize to each element.
//
// Parameters:
//   - vCfg: Bound Viper instance.
//   - flagSet: Parsed flag set.
//   - name: Flag name.
//   - envKeys: Environment variable aliases.
//   - normalize: Per-element transform (for example TrimSpace or NormalizeContainerName).
//
// Returns:
//   - []string: Normalized list (empty when unset).
func normalizedStringSlice(
	vCfg *viper.Viper,
	flagSet *pflag.FlagSet,
	name string,
	envKeys []string,
	normalize func(string) string,
) []string {
	values := stringSliceValue(vCfg, flagSet, name, envKeys, spec.ListCommaOrSpace)
	for i := range values {
		values[i] = normalize(values[i])
	}

	return values
}

// loadFilter reads filter settings, normalizes names, and builds the predicate.
func loadFilter(log *zerolog.Logger, vCfg *viper.Viper, flagSet *pflag.FlagSet, args []string) (filter.Filter, error) {
	labelEnable := vCfg.GetBool("label-enable")

	disableContainers := normalizedStringSlice(
		vCfg, flagSet, "disable-containers",
		[]string{"WATCHTOWER_DISABLE_CONTAINERS"},
		util.NormalizeContainerName,
	)

	monitorImages := normalizedStringSlice(
		vCfg, flagSet, "monitor-image-names",
		[]string{"WATCHTOWER_MONITOR_IMAGE_NAMES"},
		strings.TrimSpace,
	)

	skipImages := normalizedStringSlice(
		vCfg, flagSet, "skip-image-names",
		[]string{"WATCHTOWER_SKIP_IMAGE_NAMES"},
		strings.TrimSpace,
	)

	enableByLabel := stringSliceValue(
		vCfg, flagSet, "enable-containers-by-label",
		[]string{"WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL"},
		spec.ListCommaOnly,
	)

	disableByLabel := stringSliceValue(
		vCfg, flagSet, "disable-containers-by-label",
		[]string{"WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL"},
		spec.ListCommaOnly,
	)

	scope := vCfg.GetString("scope")

	names := make([]string, len(args))
	for i, name := range args {
		names[i] = util.NormalizeContainerName(name)
	}

	predicate, desc, err := filters.BuildFilter(
		log,
		names,
		disableContainers,
		monitorImages,
		skipImages,
		enableByLabel,
		disableByLabel,
		labelEnable,
		scope,
	)
	if err != nil {
		return filter.Filter{}, fmt.Errorf("build filter: %w", err)
	}

	return filter.Filter{
		LabelEnable:              labelEnable,
		DisableContainers:        disableContainers,
		MonitorImageNames:        monitorImages,
		SkipImageNames:           skipImages,
		EnableContainersByLabel:  enableByLabel,
		DisableContainersByLabel: disableByLabel,
		Scope:                    scope,
		Names:                    names,
		Predicate:                predicate,
		Desc:                     desc,
	}, nil
}

// loadRegistry reads the registry request settings.
//
// The TLS settings come from Viper. The cooldown platform has no flags and
// comes from the environment only.
//
// Parameters:
//   - vCfg: Bound Viper instance.
//
// Returns:
//   - registry.Registry: Registry request settings.
func loadRegistry(vCfg *viper.Viper) registry.Registry {
	return registry.Registry{
		CooldownPlatform: registry.Platform{
			OS:           os.Getenv("WATCHTOWER_COOLDOWN_PLATFORM_OS"),
			Architecture: os.Getenv("WATCHTOWER_COOLDOWN_PLATFORM_ARCH"),
			Variant:      os.Getenv("WATCHTOWER_COOLDOWN_PLATFORM_VARIANT"),
		},
		TLSMinVersion: vCfg.GetString("registry-tls-min-version"),
		TLSSkip:       vCfg.GetBool("registry-tls-skip"),
	}
}

// loadAPI reads HTTP API settings from Viper.
func loadAPI(vCfg *viper.Viper, flagSet *pflag.FlagSet) api.API {
	return api.API{
		Endpoints: stringSliceValue(
			vCfg, flagSet, "http-api-endpoints",
			[]string{"WATCHTOWER_HTTP_API_ENDPOINTS"},
			spec.ListCommaOrSpace,
		),
		LegacyUpdate:     vCfg.GetBool("http-api-update"),
		LegacyMetrics:    vCfg.GetBool("http-api-metrics"),
		LegacyContainers: vCfg.GetBool("http-api-containers"),
		Host:             vCfg.GetString("http-api-host"),
		HostChanged:      flagChanged(flagSet, "http-api-host"),
		Port:             vCfg.GetString("http-api-port"),
		PortChanged:      flagChanged(flagSet, "http-api-port"),
		Token:            vCfg.GetString("http-api-token"),
		EventsToken:      vCfg.GetString("http-api-events-token"),
		PeriodicPolls:    vCfg.GetBool("http-api-periodic-polls"),
		RateLimit:        vCfg.GetInt("http-api-rate-limit"),
		RateLimitChanged: flagChanged(flagSet, "http-api-rate-limit"),
		TLSCert:          vCfg.GetString("http-api-tls-cert"),
		TLSKey:           vCfg.GetString("http-api-tls-key"),
		TrustedProxies: stringSliceValue(
			vCfg, flagSet, "http-api-trusted-proxies",
			[]string{"WATCHTOWER_HTTP_API_TRUSTED_PROXIES"},
			spec.ListCommaOrSpace,
		),
		ProxyHeader: vCfg.GetString("http-api-proxy-header"),
		CORSOrigins: stringSliceValue(
			vCfg, flagSet, "http-api-cors-origins",
			[]string{"WATCHTOWER_HTTP_API_CORS_ORIGINS"},
			spec.ListCommaOrSpace,
		),
		CheckTimeout: durationValue(
			vCfg, flagSet, "http-api-check-timeout",
			[]string{"WATCHTOWER_HTTP_API_CHECK_TIMEOUT"},
		),
		CheckTimeoutChanged: flagChanged(flagSet, "http-api-check-timeout"),
		UpdateTimeout: durationValue(
			vCfg, flagSet, "http-api-update-timeout",
			[]string{"WATCHTOWER_HTTP_API_UPDATE_TIMEOUT"},
		),
		UpdateTimeoutChanged: flagChanged(flagSet, "http-api-update-timeout"),
	}
}

// loadNotify reads notification settings from Viper.
func loadNotify(vCfg *viper.Viper, flagSet *pflag.FlagSet) notify.Notify {
	return notify.Notify{
		URLs: stringSliceValue(
			vCfg, flagSet, "notification-url",
			[]string{"WATCHTOWER_NOTIFICATION_URL"},
			spec.ListNotificationURLs,
		),
		LegacyTypes: stringSliceValue(
			vCfg, flagSet, "notifications",
			[]string{"WATCHTOWER_NOTIFICATIONS"},
			spec.ListCommaOrSpace,
		),
		Level:            vCfg.GetString("notifications-level"),
		Template:         vCfg.GetString("notification-template"),
		TemplateFile:     vCfg.GetString("notification-template-file"),
		Report:           vCfg.GetBool("notification-report"),
		SplitByContainer: vCfg.GetBool("notification-split-by-container"),
		SkipTitle:        vCfg.GetBool("notification-skip-title"),
		LogStdout:        vCfg.GetBool("notification-log-stdout"),
		DelaySeconds:     vCfg.GetInt("notifications-delay"),
		Hostname:         vCfg.GetString("notifications-hostname"),
		TitleTag:         vCfg.GetString("notification-title-tag"),
		EmailSubjectTag:  vCfg.GetString("notification-email-subjecttag"),
		Legacy: notify.Legacy{
			EmailFrom:           vCfg.GetString("notification-email-from"),
			EmailTo:             vCfg.GetString("notification-email-to"),
			EmailServer:         vCfg.GetString("notification-email-server"),
			EmailUser:           vCfg.GetString("notification-email-server-user"),
			EmailPassword:       vCfg.GetString("notification-email-server-password"),
			EmailPort:           vCfg.GetInt("notification-email-server-port"),
			EmailTLSSkipVerify:  vCfg.GetBool("notification-email-server-tls-skip-verify"),
			EmailDelay:          vCfg.GetInt("notification-email-delay"),
			SlackHookURL:        vCfg.GetString("notification-slack-hook-url"),
			SlackIdentifier:     vCfg.GetString("notification-slack-identifier"),
			SlackChannel:        vCfg.GetString("notification-slack-channel"),
			SlackIconEmoji:      vCfg.GetString("notification-slack-icon-emoji"),
			SlackIconURL:        vCfg.GetString("notification-slack-icon-url"),
			MSTeamsHook:         vCfg.GetString("notification-msteams-hook"),
			GotifyURL:           vCfg.GetString("notification-gotify-url"),
			GotifyToken:         vCfg.GetString("notification-gotify-token"),
			GotifyTLSSkipVerify: vCfg.GetBool("notification-gotify-tls-skip-verify"),
		},
	}
}

// loadLogging reads logging settings from Viper.
func loadLogging(vCfg *viper.Viper) logging.Logging {
	format := vCfg.GetString("log-format")
	if format == "" {
		format = "auto"
	}

	return logging.Logging{
		Level:   vCfg.GetString("log-level"),
		Format:  format,
		Debug:   vCfg.GetBool("debug"),
		Trace:   vCfg.GetBool("trace"),
		NoColor: vCfg.GetBool("no-color"),
	}
}

// loadGit reads Git association and watcher settings from Viper.
//
// Parameters:
//   - vCfg: Bound configuration.
//   - flagSet: Command flag set used for list-valued flags.
//
// Returns:
//   - cfggit.Git: Parsed Git settings.
//   - error: Non-nil when a mapping, host, or CA bundle is invalid.
func loadGit(vCfg *viper.Viper, flagSet *pflag.FlagSet) (gitConfig.Git, error) {
	rawImages := stringSliceValue(
		vCfg, flagSet, "git-image",
		[]string{"WATCHTOWER_GIT_IMAGE"},
		spec.ListNewline,
	)

	images, err := gitConfig.ParseImageMappings(rawImages)
	if err != nil {
		return gitConfig.Git{}, fmt.Errorf("git-image: %w", err)
	}

	timeout := durationValue(
		vCfg, flagSet, "git-timeout",
		[]string{"WATCHTOWER_GIT_TIMEOUT"},
	)

	caBundle, err := readOptionalFile(vCfg.GetString("git-ca-bundle"))
	if err != nil {
		return gitConfig.Git{}, fmt.Errorf("git-ca-bundle: %w", err)
	}

	rawProjects := stringSliceValue(
		vCfg, flagSet, "compose-project",
		[]string{"WATCHTOWER_COMPOSE_PROJECT"},
		spec.ListNewline,
	)

	projects, err := gitConfig.ParseComposeProjects(rawProjects)
	if err != nil {
		return gitConfig.Git{}, fmt.Errorf("compose-project: %w", err)
	}

	return gitConfig.Git{
		Enable:          vCfg.GetBool("git-enable"),
		DefaultRef:      gitConfig.DefaultRef,
		SemverPolicy:    gitConfig.DefaultPolicy,
		Timeout:         timeout,
		Token:           vCfg.GetString("git-auth-token"),
		Username:        vCfg.GetString("git-username"),
		Password:        vCfg.GetString("git-password"),
		SSHKeyPath:      vCfg.GetString("git-ssh-key-path"),
		SSHKnownHosts:   vCfg.GetString("git-ssh-known-hosts"),
		CABundle:        caBundle,
		InsecureSkipTLS: vCfg.GetBool("git-insecure-skip-tls"),
		Dockerfile:      strings.TrimSpace(vCfg.GetString("git-dockerfile")),
		Context:         strings.TrimSpace(vCfg.GetString("git-context")),
		Images:          images,
		ComposeStash:    vCfg.GetBool("git-compose-stash"),
		ComposeProjects: projects,
	}, nil
}

// readOptionalFile reads path when it is non-empty.
//
// Parameters:
//   - path: Filesystem path, or empty.
//
// Returns:
//   - []byte: File contents, or nil when path is empty.
//   - error: Non-nil when the file cannot be read.
func readOptionalFile(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return data, nil
}

// validate checks cross-flag constraints that Load can enforce without side effects.
func validate(log *zerolog.Logger, cfg Config) error {
	if cfg.Update.RollingRestart && cfg.Update.MonitorOnly {
		return errRollingRestartWithMonitorOnly
	}

	if cfg.Update.MonitorOnly && cfg.Update.NoPull {
		log.Warn().
			Bool("monitor_only", cfg.Update.MonitorOnly).
			Bool("no_pull", cfg.Update.NoPull).
			Msg("Combining monitor-only and no-pull might result in no updates")
	}

	return nil
}

// flagChanged reports whether a flag was set on the command line.
func flagChanged(flagSet *pflag.FlagSet, name string) bool {
	flag := flagSet.Lookup(name)
	if flag == nil {
		return false
	}

	return flag.Changed
}
