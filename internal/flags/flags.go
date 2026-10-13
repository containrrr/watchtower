package flags

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/nicholas-fedor/watchtower/internal/flags/api"
	"github.com/nicholas-fedor/watchtower/internal/flags/client"
	"github.com/nicholas-fedor/watchtower/internal/flags/compat"
	"github.com/nicholas-fedor/watchtower/internal/flags/docker"
	"github.com/nicholas-fedor/watchtower/internal/flags/filter"
	"github.com/nicholas-fedor/watchtower/internal/flags/git"
	"github.com/nicholas-fedor/watchtower/internal/flags/lifecycle"
	flagslogging "github.com/nicholas-fedor/watchtower/internal/flags/logging"
	"github.com/nicholas-fedor/watchtower/internal/flags/mode"
	"github.com/nicholas-fedor/watchtower/internal/flags/notify"
	"github.com/nicholas-fedor/watchtower/internal/flags/registry"
	"github.com/nicholas-fedor/watchtower/internal/flags/schedule"
	"github.com/nicholas-fedor/watchtower/internal/flags/update"
	"github.com/nicholas-fedor/watchtower/internal/flags/utils"
	"github.com/nicholas-fedor/watchtower/internal/logging"
)

// DockerAPIMinVersion sets the minimum Docker API version supported by Watchtower.
const DockerAPIMinVersion string = "1.24"

// Errors for flag and environment configuration.
var (
	// errInvalidLogFormat indicates an invalid log format was specified in configuration.
	errInvalidLogFormat = errors.New("invalid log format specified")
	// errInvalidLogLevel indicates an invalid log level was specified in configuration.
	errInvalidLogLevel = errors.New("invalid log level specified")
	// errSetEnvFailed indicates a failure to set an environment variable during configuration.
	errSetEnvFailed = errors.New("failed to set environment variable")
	// errOpenFileFailed indicates a failure to open a file when reading secrets.
	errOpenFileFailed = errors.New("failed to open secret file")
	// errReplaceSliceFailed indicates a failure to replace a slice value in a flag.
	errReplaceSliceFailed = errors.New("failed to replace slice value in flag")
	// errReadFileFailed indicates a failure to read a file's contents for secrets.
	errReadFileFailed = errors.New("failed to read secret file")
	// errInvalidSecretURL indicates an invalid URL was found in a secret file.
	errInvalidSecretURL = errors.New("invalid notification URL in secret file")
	// errSetFlagFailed indicates a failure to set a flag's value during configuration.
	errSetFlagFailed = errors.New("failed to set flag value")
	// errInvalidFlagName indicates an invalid flag name was provided for modification.
	errInvalidFlagName = errors.New("invalid flag name provided")
	// errNotSliceValue indicates a flag does not support slice values for appending.
	errNotSliceValue = errors.New("flag does not support slice values")
)

// RegisterDockerFlags adds Docker API client flags to the root command.
//
// Prefer RegisterAll when registering the full flag set.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func RegisterDockerFlags(rootCmd *cobra.Command) {
	docker.Register(rootCmd)
}

// RegisterSystemFlags registers non-Docker, non-notification domain flags.
//
// Prefer RegisterAll. Kept for tests that call domain groups separately.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func RegisterSystemFlags(rootCmd *cobra.Command) {
	client.Register(rootCmd)
	schedule.Register(rootCmd)
	mode.Register(rootCmd)
	update.Register(rootCmd)
	lifecycle.Register(rootCmd)
	filter.Register(rootCmd)
	registry.Register(rootCmd)
	compat.Register(rootCmd)
	api.Register(rootCmd)
	flagslogging.Register(rootCmd)
	git.Register(rootCmd)
}

// RegisterNotificationFlags adds notification flags to the root command.
//
// Prefer RegisterAll when registering the full flag set.
//
// Parameters:
//   - rootCmd: Root Cobra command.
func RegisterNotificationFlags(rootCmd *cobra.Command) {
	notify.Register(rootCmd)
}

// filterEmptyStrings is a package-local alias for utils.FilterEmptyStrings (tests).
func filterEmptyStrings(values []string) []string {
	return utils.FilterEmptyStrings(values)
}

// isPureNumeric reports whether str is a bare number (tests and env duration helpers).
func isPureNumeric(str string) bool {
	return utils.IsPureNumeric(str)
}

// SetDefaults enables automatic environment lookup on the global Viper instance.
//
// Flag static defaults and process Load bind live on FlagSpec rows. This remains
// for tests and helpers that still touch the global Viper (for example EnvDuration).
func SetDefaults() {
	viper.AutomaticEnv()
}

// EnvConfig sets Docker environment variables from flags.
//
// Parameters:
//   - log: Logger for configuration diagnostics.
//   - cmd: Cobra command with flags.
//
// Returns:
//   - error: Non-nil if flag retrieval fails, nil on success.
func EnvConfig(log *zerolog.Logger, cmd *cobra.Command) error {
	flagSet := cmd.PersistentFlags()

	// Resolve Docker settings via Viper (flag > env > static default) after BindAll.
	vCfg := viper.New()

	err := BindAll(vCfg, flagSet, docker.Specs())
	if err != nil {
		return fmt.Errorf("bind docker flags: %w", err)
	}

	host := vCfg.GetString("host")
	tls := vCfg.GetBool("tlsverify")
	version := strings.Trim(vCfg.GetString("api-version"), "\"")
	certPath := vCfg.GetString("cert-path")

	// Convert tcp:// to https:// when TLS is enabled.
	if tls && strings.HasPrefix(host, "tcp://") {
		host = strings.Replace(host, "tcp://", "https://", 1)
	}

	// Warn about mismatched TLS settings.
	if tls {
		if strings.HasPrefix(host, "http://") {
			log.Warn().
				Msg("TLS verification is enabled but DOCKER_HOST uses insecure scheme 'http://'. Consider using 'https://' or disable TLS verification.")
		} else if strings.HasPrefix(host, "unix://") {
			log.Warn().
				Msg("TLS verification is enabled but DOCKER_HOST uses local socket 'unix://'. TLS is not applicable for local sockets. Consider disabling TLS verification.")
		}
	}

	// Set environment variables.
	err = setEnvOptStr(log, "DOCKER_HOST", host)
	if err != nil {
		return err
	}

	err = setEnvOptBool(log, "DOCKER_TLS_VERIFY", tls)
	if err != nil {
		return err
	}

	err = setEnvOptStr(log, "DOCKER_API_VERSION", version)
	if err != nil {
		return err
	}

	err = setEnvOptStr(log, "DOCKER_CERT_PATH", certPath)
	if err != nil {
		return err
	}

	log.Debug().
		Str("host", host).
		Bool("tls", tls).
		Str("version", version).
		Str("certPath", certPath).
		Msg("Configured Docker environment variables")

	return nil
}

// setEnvOptStr sets an environment variable if needed.
//
// Parameters:
//   - log: Logger for configuration diagnostics.
//   - env: Environment variable name.
//   - opt: Value to set.
//
// Returns:
//   - error: Non-nil if set fails, nil if skipped or successful.
func setEnvOptStr(log *zerolog.Logger, env, opt string) error {
	if opt == "" || opt == os.Getenv(env) {
		return nil
	}

	err := os.Setenv(env, opt)
	if err != nil {
		log.Debug().
			Err(err).
			Str("env", env).
			Str("value", opt).
			Msg("Failed to set environment variable")

		return fmt.Errorf("%w: %s: %w", errSetEnvFailed, env, err)
	}

	log.Debug().
		Str("env", env).
		Str("value", opt).
		Msg("Set environment variable")

	return nil
}

// setEnvOptBool sets an environment variable to "1" if true.
//
// Parameters:
//   - log: Logger for configuration diagnostics.
//   - env: Environment variable name.
//   - opt: Boolean value.
//
// Returns:
//   - error: Non-nil if set fails, nil otherwise.
func setEnvOptBool(log *zerolog.Logger, env string, opt bool) error {
	if opt {
		return setEnvOptStr(log, env, "1")
	}

	return nil
}

// GetSecretsFromFiles updates flags with file contents for secrets.
//
// Parameters:
//   - log: Logger for secret-loading diagnostics and fatal failures.
//   - rootCmd: Root Cobra command.
//
//nolint:godox
func GetSecretsFromFiles(log *zerolog.Logger, rootCmd *cobra.Command) {
	flags := rootCmd.PersistentFlags()
	secrets := []string{
		// TODO: Remove just before v2 Release.
		"notification-email-server-password",
		// TODO: Remove just before v2 Release.
		"notification-slack-hook-url",
		// TODO: Remove just before v2 Release.
		"notification-msteams-hook",
		// TODO: Remove just before v2 Release.
		"notification-gotify-token",
		"notification-url",
		"http-api-token",
		"http-api-events-token",
		"git-auth-token",
		"git-password",
	}

	// Process each secret flag.
	for _, secret := range secrets {
		err := getSecretFromFile(log, flags, secret)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("flag", secret).
				Msg("Failed to load secret from file")
		}
	}
}

// getSecretFromFile reads file contents into a flag if applicable.
//
// Parameters:
//   - log: Logger for secret-loading diagnostics.
//   - flags: Flag set.
//   - secret: Flag name.
//
// Returns:
//   - error: Non-nil if file ops fail, nil on success or skip.
func getSecretFromFile(log *zerolog.Logger, flags *pflag.FlagSet, secret string) error {
	flag := flags.Lookup(secret)

	// Handle slice flags.
	if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
		oldValues := sliceValue.GetSlice()
		values := make([]string, 0, len(oldValues))

		for _, value := range oldValues {
			if value != "" && isFilePath(value) {
				lines, err := readSecretFileLines(log, secret, value)
				if err != nil {
					return err
				}

				values = append(values, lines...)
			} else {
				values = append(values, value)
			}
		}

		err := sliceValue.Replace(values)
		if err != nil {
			log.Debug().
				Err(err).
				Str("flag", secret).
				Msg("Failed to replace slice value in flag")

			return fmt.Errorf("%w: %w", errReplaceSliceFailed, err)
		}

		// Mark the flag as explicitly set so downstream consumers read the expanded
		// value from the flag rather than re-deriving from raw os.Getenv.
		flag.Changed = true

		return nil
	}

	// Handle string flags.
	value := flag.Value.String()
	if value != "" && isFilePath(value) {
		content, err := os.ReadFile(value)
		if err != nil {
			log.Debug().
				Err(err).
				Str("flag", secret).
				Str("file", value).
				Msg("Failed to read secret file")

			return fmt.Errorf("%w: %w", errReadFileFailed, err)
		}

		err = flags.Set(secret, strings.TrimSpace(string(content)))
		if err != nil {
			log.Debug().
				Err(err).
				Str("flag", secret).
				Msg("Failed to set flag from file contents")

			return fmt.Errorf("%w: %w", errSetFlagFailed, err)
		}

		log.Debug().
			Str("flag", secret).
			Str("file", value).
			Msg("Set flag from file contents")
	}

	return nil
}

// readSecretFileLines reads the values of a list secret from a file.
//
// Blank lines and lines starting with "#" are skipped. The file is closed
// before returning, so a list secret holds at most one file open at a time.
//
// Parameters:
//   - log: Logger for secret-loading diagnostics.
//   - secret: Flag name. Lines of the notification-url secret must be valid notification URLs.
//   - path: Path of the secret file.
//
// Returns:
//   - []string: Values read from the file.
//   - error: Non-nil if the file cannot be opened or read, or holds an invalid notification URL.
func readSecretFileLines(log *zerolog.Logger, secret, path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", secret).
			Str("file", path).
			Msg("Failed to open secret file")

		return nil, fmt.Errorf("%w: %w", errOpenFileFailed, err)
	}

	defer func() { _ = file.Close() }()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if secret == "notification-url" {
			if !strings.Contains(line, "://") {
				return nil, errInvalidSecretURL
			}

			parsedURL, err := url.Parse(line)
			if err != nil || parsedURL.Scheme == "" {
				return nil, errInvalidSecretURL
			}

			if parsedURL.Opaque == "" && parsedURL.Host == "" && parsedURL.Path == "" {
				if parsedURL.Scheme != "logger" && parsedURL.Scheme != "mock" {
					return nil, errInvalidSecretURL
				}
			}
		}

		lines = append(lines, line)
	}

	err = scanner.Err()
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", secret).
			Str("file", path).
			Msg("Failed to read secret file")

		return nil, fmt.Errorf("%w: %w", errReadFileFailed, err)
	}

	log.Debug().
		Str("flag", secret).
		Str("file", path).
		Msg("Read secret from file into slice")

	return lines, nil
}

// isFilePath checks if a string is likely a file path.
//
// Parameters:
//   - path: String to check.
//
// Returns:
//   - bool: True if likely a file path, false otherwise.
func isFilePath(path string) bool {
	firstColon := strings.IndexRune(path, ':')
	if firstColon != 1 && firstColon != -1 {
		// If ':' exists but isn't the second character, it's likely not a file path (e.g., URLs).
		return false
	}

	//nolint:gosec // G703: Path traversal via taint analysis - validating user-provided path exists
	_, err := os.Stat(path)

	return !errors.Is(err, os.ErrNotExist)
}

// ProcessFlagAliases applies environment values then syncs flag aliases.
//
// It bridges env onto unset flags, then applies porcelain mode, interval versus
// schedule conflicts, and debug/trace log-level forcing.
//
// Intended production order (see cmd preRun / notify-upgrade):
//
//  1. ApplyEnvToFlags (or rely on the bridge inside this function)
//  2. SetupLogging — apply --log-format (and current level) so Fatal paths here
//     use the user-selected format rather than zerolog's default JSON encoding
//  3. ProcessFlagAliases — may force log-level to debug/trace and may Fatal
//  4. SetupLogging again — re-apply level after alias mutations
//  5. GetSecretsFromFiles / EnvConfig / config.Load
//
// Call after Cobra parse. Do not call only after a single post-alias SetupLogging
// if format-before-fatal matters. Sandwich this between the two SetupLogging calls.
//
// Parameters:
//   - log: Logger for alias diagnostics and fatal configuration errors.
//   - flags: Parsed persistent flag set.
func ProcessFlagAliases(log *zerolog.Logger, flags *pflag.FlagSet) {
	// Ensure env-sourced values are visible to alias logic via flag Gets.
	err := ApplyEnvToFlags(flags, AllSpecs())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to apply environment configuration")
	}

	// Handle porcelain mode.
	porcelain, err := flags.GetString("porcelain")
	if err != nil {
		log.Fatal().
			Err(err).
			Str("flag", "porcelain").
			Msg("Failed to get porcelain flag")
	}

	if porcelain != "" {
		switch porcelain {
		case "v1", "json":
		default:
			log.Fatal().
				Str("version", porcelain).
				Msg("Unknown porcelain version, supported: v1, json")
		}

		err := appendFlagValue(log, flags, "notification-url", "logger://")
		if err != nil {
			log.Debug().Err(err).Msg("Failed to append notification-url")
		}

		setFlagIfDefault(log, flags, "notification-log-stdout", "true")
		setFlagIfDefault(log, flags, "notification-report", "true")

		var tpl string
		if porcelain == "v1" {
			tpl = "porcelain.v1.summary-no-log"
		} else {
			tpl = "porcelain.json"
		}

		setFlagIfDefault(log, flags, "notification-template", tpl)
		log.Debug().
			Str("porcelain", porcelain).
			Msg("Configured porcelain mode")
	}

	// Handle interval vs. schedule conflicts.
	scheduleChanged := flags.Changed("schedule")
	intervalChanged := flags.Changed("interval")

	if val, _ := flags.GetString("schedule"); val != "" {
		scheduleChanged = true
	}

	if val, _ := flags.GetInt("interval"); val != schedule.DefaultPollIntervalSeconds {
		intervalChanged = true
	}

	if intervalChanged && scheduleChanged {
		log.Fatal().
			Bool("interval", intervalChanged).
			Bool("schedule", scheduleChanged).
			Msg("Cannot define both interval and schedule")
	}

	// Update schedule to match interval or default if needed.
	if intervalChanged || !scheduleChanged {
		interval, _ := flags.GetInt("interval")

		scheduleValue := fmt.Sprintf("@every %ds", interval)

		err := flags.Set("schedule", scheduleValue)
		if err != nil {
			log.Debug().
				Err(err).
				Int("interval", interval).
				Msg("Failed to set schedule from interval")
		} else {
			log.Debug().
				Int("interval", interval).
				Str("schedule", scheduleValue).
				Msg("Set default schedule from interval")
		}
	}

	// Adjust log level for debug/trace.
	if flagIsEnabled(log, flags, "debug") {
		err := flags.Set("log-level", "debug")
		if err != nil {
			log.Debug().Err(err).Msg("Failed to set debug log level")
		}
	}

	if flagIsEnabled(log, flags, "trace") {
		err := flags.Set("log-level", "trace")
		if err != nil {
			log.Debug().Err(err).Msg("Failed to set trace log level")
		}
	}
}

// SetupLogging configures format and level on the provided logger.
//
// Parameters:
//   - log: Logger to reconfigure (typically from logging.New at process start).
//   - flags: Flag set.
//
// Returns:
//   - *zerolog.Logger: Logger with format writer and level applied.
//   - error: Non-nil if config fails, nil on success.
func SetupLogging(log *zerolog.Logger, flags *pflag.FlagSet) (*zerolog.Logger, error) {
	logFormat, err := flags.GetString("log-format")
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", "log-format").
			Msg("Failed to get log-format flag")

		return log, fmt.Errorf("%w: %w", errSetFlagFailed, err)
	}

	// Default to "auto" when neither the flag nor WATCHTOWER_LOG_FORMAT is set.
	// This prevents ConfigureWriter from failing on empty strings, which is the
	// case when running the ephemeral orchestrator container without
	// WATCHTOWER_LOG_FORMAT in its environment.
	if logFormat == "" {
		logFormat = "auto"
	}

	noColor, err := flags.GetBool("no-color")
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", "no-color").
			Msg("Failed to get no-color flag")

		return log, fmt.Errorf("%w: %w", errSetFlagFailed, err)
	}

	writer, err := logging.ConfigureWriter(logFormat, noColor)
	if err != nil {
		log.Debug().
			Err(err).
			Str("format", logFormat).
			Msg("Invalid log format specified")

		return log, fmt.Errorf("%w: %w", errInvalidLogFormat, err)
	}

	// Rebuild with the format writer, preserving the current level until
	// --log-level (including debug/trace aliases) is applied below.
	rebuilt := zerolog.New(writer).Level(log.GetLevel()).With().Timestamp().Logger()
	log = &rebuilt

	// Set log level only when explicitly specified.
	rawLogLevel, err := flags.GetString("log-level")
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", "log-level").
			Msg("Failed to get log-level flag")

		return log, fmt.Errorf("%w: %w", errSetFlagFailed, err)
	}

	// Parse and apply log level when non-empty.
	// Under normal registration the flag default is "info" (see flags/logging Specs),
	// so GetString is rarely empty. The empty branch is defensive for tests or
	// partial flag sets (for example orchestrator paths that never registered logging
	// flags) and preserves the level already on log (typically InfoLevel from main).
	// ProcessFlagAliases may have already forced debug/trace onto the log-level flag.
	// Invalid levels fail fast (same contract as the previous SetupLogging behavior).
	if rawLogLevel != "" {
		level, parseErr := logging.ParseLevel(rawLogLevel)
		if parseErr != nil {
			log.Debug().
				Err(parseErr).
				Str("log_level", rawLogLevel).
				Msg("Invalid log level specified")

			return log, fmt.Errorf("%w: %w", errInvalidLogLevel, parseErr)
		}

		leveled := log.Level(level)
		log = &leveled
	}

	log.Debug().
		Str("format", logFormat).
		Str("log_level", log.GetLevel().String()).
		Bool("no_color", noColor).
		Msg("Configured logging settings")

	return log, nil
}

// flagIsEnabled checks if a boolean flag is true.
//
// Parameters:
//   - log: Logger for fatal flag retrieval failures.
//   - flags: Flag set.
//   - name: Flag name.
//
// Returns:
//   - bool: True if enabled.
func flagIsEnabled(log *zerolog.Logger, flags *pflag.FlagSet, name string) bool {
	value, err := flags.GetBool(name)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("flag", name).
			Msg("Failed to check flag status")
	}

	return value
}

// appendFlagValue appends values to a slice flag.
//
// Parameters:
//   - log: Logger for append diagnostics.
//   - flags: Flag set.
//   - name: Flag name.
//   - values: Values to append.
//
// Returns:
//   - error: Non-nil if append fails, nil on success.
func appendFlagValue(log *zerolog.Logger, flags *pflag.FlagSet, name string, values ...string) error {
	flag := flags.Lookup(name)
	if flag == nil {
		log.Debug().
			Str("flag", name).
			Msg("Invalid flag name provided")

		return fmt.Errorf("%w: %q", errInvalidFlagName, name)
	}

	if flagValues, ok := flag.Value.(pflag.SliceValue); ok {
		for _, value := range values {
			err := flagValues.Append(value)
			if err != nil {
				log.Debug().
					Err(err).
					Str("flag", name).
					Str("value", value).
					Msg("Failed to append value to flag")
			}
		}
	} else {
		log.Debug().
			Str("flag", name).
			Msg("Flag does not support slice values")

		return fmt.Errorf("%w: %q", errNotSliceValue, name)
	}

	return nil
}

// setFlagIfDefault sets a flag's value unless it was set by a flag or by an
// environment variable. ApplyEnvToFlags leaves environment values unmarked, so
// they are checked separately.
//
// Parameters:
//   - log: Logger for set diagnostics.
//   - flags: Flag set.
//   - name: Flag name.
//   - value: Value to set.
func setFlagIfDefault(log *zerolog.Logger, flags *pflag.FlagSet, name, value string) {
	if flags.Changed(name) || setByEnv(name) {
		return
	}

	err := flags.Set(name, value)
	if err != nil {
		log.Debug().
			Err(err).
			Str("flag", name).
			Str("value", value).
			Msg("Failed to set default flag value")
	} else {
		log.Debug().
			Str("flag", name).
			Str("value", value).
			Msg("Set default flag value")
	}
}

// setByEnv reports whether an environment variable supplies the flag's value,
// using the same rule as ApplyEnvToFlags.
//
// Parameters:
//   - name: Flag name.
//
// Returns:
//   - bool: True if one of the flag's environment variables is set.
func setByEnv(name string) bool {
	for _, flagSpec := range AllSpecs() {
		if flagSpec.Name == name {
			_, ok := firstEnv(flagSpec.EnvKeys)

			return ok
		}
	}

	return false
}
