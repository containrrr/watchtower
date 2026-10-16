// Package flags manages command-line flags and environment variables for Watchtower configuration.
// It configures Docker connections, operational behavior, and notifications via Cobra and Viper.
//
// Layout:
//   - internal/flags/spec — FlagSpec metadata (static defaults, env keys, list parse kind)
//   - internal/flags/<domain> — Specs() and/or Register per subsystem
//   - internal/flags/utils — list parsers, env helpers, deprecation
//   - RegisterAll / BindAll / AllSpecs — parent façade
//
// Domain packages match the config taxonomy:
// docker, client, schedule, mode, update, lifecycle, filter, registry, compat,
// api, notify, logging, git.
//
// Key components:
//   - RegisterAll: Registers every domain's flags on the root command.
//   - BindAll: Applies Viper defaults, BindPFlag, and BindEnv from FlagSpec rows.
//   - SetupLogging: Reconfigures a *zerolog.Logger (format writer + level) from flags and returns the updated logger.
//   - ProcessFlagAliases / GetSecretsFromFiles: Pre-load transforms (porcelain, interval→schedule, secrets).
//   - EnvConfig: Maps Docker client flags into DOCKER_* process environment variables.
//
// Every domain exposes FlagSpec with static pflag defaults. BindAll and Load
// resolve values as flag > env > default. ApplyEnvToFlags bridges env onto unset
// flags for pre-load helpers without baking env into registration defaults.
//
// Usage example:
//
//	log := logging.New(os.Stderr, logging.InfoLevel)
//	cmd := &cobra.Command{}
//	flags.RegisterAll(cmd)
//	log, err := flags.SetupLogging(log, cmd.PersistentFlags())
//	if err != nil {
//	    log.Fatal().Err(err).Msg("Logging setup failed")
//	}
//
// Resolved process policy belongs in internal/config via config.Load. This package
// only declares and binds flags. It does not own update policy DTOs.
//
// Logging helpers accept and return *zerolog.Logger explicitly (no global logger).
// Format/level configuration uses internal/logging (ConfigureWriter, ParseLevel).
package flags
