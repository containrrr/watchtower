package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	"github.com/nicholas-fedor/watchtower/internal/api"
	"github.com/nicholas-fedor/watchtower/internal/api/config"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	appConfig "github.com/nicholas-fedor/watchtower/internal/config"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/internal/git"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/meta"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/notifications"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// restartPolicyTimeout is the maximum duration allowed for restart-policy
	// update operations.
	//
	// It bounds the Docker API call that sets the current Watchtower
	// container's restart policy to "no", so a slow or unresponsive
	// daemon cannot delay shutdown paths.
	restartPolicyTimeout = 5 * time.Second

	// containerLookupTimeout is the maximum duration allowed for current
	// container ID lookups.
	//
	// It bounds the Docker API / hostname / mountinfo detection sequence,
	// so startup cannot hang indefinitely if the daemon or container runtime
	// metadata is unreachable.
	containerLookupTimeout = 5 * time.Second
)

var (
	// appCfg is the resolved process configuration from appconfig.Load.
	//
	// It is the single source of operational policy for run-once, schedule, and API paths.
	// Values originate from CLI flags and environment variables registered in internal/flags
	// and are resolved once at startup (and again in run when positional container names apply).
	appCfg appConfig.Config

	// client is the Docker client instance used to interact with container operations in Watchtower.
	//
	// It provides an interface for listing, stopping, starting, and managing containers, initialized during
	// the preRun phase with options derived from appCfg.ClientOptions() (DOCKER_HOST, TLS, API version, and
	// related client flags/environment variables).
	client container.Client

	// notifier is the notification system instance responsible for sending update status messages to configured channels.
	//
	// It is initialized in preRun from appCfg.Notify via notifications.NewNotifier, supporting
	// Shoutrrr URLs and (deprecated) legacy types such as email, Slack, or MSTeams.
	notifier types.Notifier

	// currentWatchtowerContainerID stores the current Watchtower container ID.
	//
	// It is initialized once in preRun after the client is set up, and used throughout the application
	// to avoid repeated calls to GetCurrentContainerID. If retrieval fails, it is set to an empty string.
	currentWatchtowerContainerID types.ContainerID

	// currentWatchtowerContainerUnknown is true when Watchtower runs in a container
	// whose ID could not be determined. Self-updates are then disabled.
	currentWatchtowerContainerUnknown bool

	// currentWatchtowerContainer holds the current Watchtower container instance.
	//
	// It is initialized in preRun by retrieving the container object using the currentWatchtowerContainerID,
	// remains nil if retrieval fails or yields an unexpected type, and is used for operations like updating
	// restart policy, validating restarts, and cleaning up excess instances.
	currentWatchtowerContainer types.Container

	// sleepFunc is a function variable for time.Sleep, allowing it to be overridden in tests.
	//
	// It is initialized to time.Sleep by default, providing a way to mock sleep behavior during testing
	// to avoid delays in unit tests or control timing in integration tests.
	sleepFunc = time.Sleep

	// createSignalContext is a function variable for creating a signal-aware context.
	//
	// It wraps signal.NotifyContext to allow overriding in tests for testing signal handling behavior.
	// The function creates a context that is canceled when the specified signals (SIGINT, SIGTERM) are received.
	createSignalContext = signal.NotifyContext

	// newClient is a function variable for creating the Docker client, allowing it to be overridden in tests.
	//
	// It is initialized to container.NewClient by default. preRun calls it with the resolved client options,
	// so tests can substitute a mock client without a Docker daemon.
	newClient = container.NewClient

	// processFs is the filesystem preRun checks for container marker files.
	//
	// Tests replace it to control whether Watchtower appears to run in a container.
	processFs afero.Fs = afero.NewOsFs()

	// runUpdatesWithNotifications is a function variable for performing container updates and sending notifications.
	//
	// It is initialized inside runMain with a closure that executes actions.RunUpdatesWithNotifications,
	// allowing it to be overridden in tests to mock the update process. It takes a context, filter, and update params,
	// and returns a metric summarizing the update session.
	runUpdatesWithNotifications func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric

	// rootCmd represents the root command for the Watchtower CLI, serving as the entry point for all subcommands.
	//
	// It defines the base usage string, short and long descriptions, and assigns lifecycle hooks (PreRun and Run)
	// to manage setup and execution, initialized with default behavior and configured via flags during runtime.
	rootCmd = NewRootCommand()
)

// process holds composition-root dependencies for a single CLI invocation.
//
// The logger is constructed in main and reconfigured in preRun after flags are
// parsed. It is never stored as a package-level global.
type process struct {
	log *zerolog.Logger
}

// init registers command-line flags for the root command during package initialization.
//
// It registers every domain's flags, such as Docker configuration (e.g., --host), system behavior
// (e.g., --interval), and notifications (e.g., --notifications), establishing the CLI's configurable
// parameters before execution begins.
func init() {
	flags.RegisterAll(rootCmd)
}

// NewRootCommand creates and configures the root command for the Watchtower CLI.
//
// It establishes the base usage string ("watchtower"), a short description summarizing its purpose,
// and a long description with additional context and a project URL.
//
// PreRun and Run are wired in Execute with a logger-capturing process instance so the
// composition root can pass *zerolog.Logger without a package-level global.
//
// Returns:
//   - *cobra.Command: A pointer to the fully configured root command, ready for flag registration and execution.
func NewRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watchtower",
		Short: "Automatically updates running Docker containers",
		Long:  "\nWatchtower automatically updates running Docker containers whenever a new image is released.\nMore information available at https://github.com/nicholas-fedor/watchtower/.",
		Args:  cobra.ArbitraryArgs, // Permits any number of positional arguments, processed as container names later.
	}
}

// Execute runs the root command and manages any errors encountered during its execution.
//
// It serves as the primary entry point for the Watchtower CLI, called from main.go, and ensures that any
// fatal errors are logged and terminate the program with an appropriate exit status, providing a clean
// interface between the CLI and the operating system.
//
// Parameters:
//   - log: Process logger from main (reconfigured in preRun from --log-format / --log-level).
func Execute(log *zerolog.Logger) {
	proc := &process{log: log}
	rootCmd.PreRun = proc.preRun
	rootCmd.Run = proc.run

	err := rootCmd.Execute()
	if err != nil {
		// Prefer proc.log so post-SetupLogging errors use the configured format/level.
		proc.log.Fatal().Err(err).Msg("Failed to execute root command")
	}
}

// preRun prepares the environment and configuration before the main command execution begins.
//
// It processes command-line flag aliases, configures logging based on verbosity settings,
// expands secrets from files, maps Docker flags into the process environment, loads the
// immutable appconfig snapshot, initializes the Docker client and notification client, and
// handles early-exit paths (ephemeral self-update orchestrator and invalid old-container restarts).
//
// Parameters:
//   - cmd: The cobra.Command instance being executed, providing access to parsed flags.
//   - _: A slice of string arguments (unused here, as container names are applied in run when
//     reloading configuration for filtering).
func (p *process) preRun(cmd *cobra.Command, _ []string) {
	flagsSet := cmd.PersistentFlags()

	// Bridge environment values onto unset flags so aliases and logging still see them.
	err := flags.ApplyEnvToFlags(flagsSet, flags.AllSpecs())
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to apply environment configuration")
	}

	// Apply format (and pre-alias level) before ProcessFlagAliases so Fatal paths
	// from porcelain/interval conflicts use the user-selected --log-format rather
	// than zerolog's default JSON encoding on stderr.
	p.log, err = flags.SetupLogging(p.log, flagsSet)
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to initialize logging")
	}

	// Apply porcelain, interval→schedule, and debug/trace log-level aliases.
	flags.ProcessFlagAliases(p.log, flagsSet)

	// Re-apply after aliases so debug/trace forced log-level values take effect.
	p.log, err = flags.SetupLogging(p.log, flagsSet)
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to initialize logging")
	}

	// Expand secrets from files (for example notification URLs and API tokens).
	flags.GetSecretsFromFiles(p.log, cmd)

	// Map Docker connection flags into the process environment for the client stack.
	err = flags.EnvConfig(p.log, cmd)
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to configure Docker environment")
	}

	// Load without positional names. run reloads with args for the final filter.
	appCfg, err = appConfig.Load(p.log, cmd, nil)
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	p.log.Debug().
		Str("scheduleSpec", appCfg.Schedule.Spec).
		Msg("Retrieved cron schedule specification from configuration")

	// Log the scope if specified, aiding debugging by confirming the operational boundary.
	if appCfg.Filter.Scope != "" {
		p.log.Debug().
			Str("scope", appCfg.Filter.Scope).
			Msg("Configured operational scope")
	}

	// Initialize the Docker client from the resolved ClientOptions projection.
	client = newClient(p.log, appCfg.ClientOptions())

	// Check for orchestrator mode early. This is an internal mode where Watchtower
	// runs as a one-shot orchestrator for self-update.
	if appCfg.Mode.SelfUpdateOrchestrator {
		p.log.Info().Msg("Running in ephemeral self-update orchestrator mode")

		actions.RunOrchestrator(p.log, context.Background(), client)

		currentWatchtowerContainer = resolveCurrentWatchtowerContainerForFallback(p.log,
			context.Background(),
			client,
		)

		setNoRestartPolicyCtx, cancel := context.WithTimeout(
			context.Background(),
			restartPolicyTimeout,
		)
		defer cancel()

		client.SetRestartPolicy(
			setNoRestartPolicyCtx,
			currentWatchtowerContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)

		p.log.Fatal().
			Str("flag", "self-update-orchestrator").
			Msg("RunOrchestrator returned unexpectedly. Exiting to prevent unintended execution")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		containerLookupTimeout,
	)
	defer cancel()

	// Retrieve and store the current container ID for use throughout the application.
	currentWatchtowerContainerID, err = container.GetCurrentContainerID(p.log, ctx, client)
	if err != nil {
		p.log.Debug().Err(err).Msg("Failed to get current container ID")

		currentWatchtowerContainerID = ""
	}

	// Retrieve the current Watchtower container.
	if currentWatchtowerContainerID != "" {
		currentWatchtowerContainer, err = client.GetCurrentWatchtowerContainer(
			ctx,
			currentWatchtowerContainerID,
		)
		if err != nil {
			p.log.Debug().Err(err).Msg("Failed to get the current Watchtower Container")

			// Handle context deadline exceeded or cancellation
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				currentWatchtowerContainerID = ""
			}

			currentWatchtowerContainer = nil
		}
	}

	// Without its own ID, Watchtower cannot tell itself apart from other instances.
	// A host binary has no container ID, so only a containerized run is unknown.
	currentWatchtowerContainerUnknown = currentWatchtowerContainerID == "" &&
		container.InContainer(processFs)

	// Check if this is an old Watchtower container that should not run continuously.
	// exitInvalidWatchtowerRestart calls os.Exit. Keep it in a helper so preRun
	// defers (for example cancel) are not paired with os.Exit in this function
	// (gocritic exitAfterDefer).
	if scheduling.ShouldExitDueToInvalidRestart(
		currentWatchtowerContainer,
		appCfg.Mode.RunOnce,
	) {
		// Cancel preRun lookup context before process exit.
		cancel()
		exitInvalidWatchtowerRestart(p.log, client, currentWatchtowerContainer)
	}

	// Set up the notification client from loaded process config (appCfg.Notify).
	// RegisterHook attaches the notifier to p.log and updates p.log in place to the
	// hooked logger so subsequent application logging is captured for notifications.
	notifier = notifications.NewNotifier(p.log, appCfg.Notify)
	notifier.RegisterHook(p.log)

	if currentWatchtowerContainerUnknown {
		p.log.Warn().Msg(
			"Could not identify Watchtower's own container. " +
				"Self-updates are disabled and old instances will not be removed",
		)
	}

	// Log deprecated notification configuration options, if set.
	notifications.LogLegacyDeprecationWarnings(p.log, appCfg.Notify.LegacyTypes)
}

// run executes the main Watchtower logic based on parsed command-line flags.
//
// It reloads process configuration with positional container names, derives the effective
// operational scope (including scope persistence across self-updates), handles health-check
// early exit, builds the HTTP API RunConfig from appCfg, and delegates to runMain for core
// execution, exiting with a status code based on the outcome (0 for success, non-zero for failure).
//
// exitInvalidWatchtowerRestart recovers any orphaned instance, sets restart policy to "no",
// and terminates the process.
//
// Kept separate from preRun so os.Exit is not paired with preRun's deferred context
// cancels (gocritic exitAfterDefer).
//
// Parameters:
//   - log: Process logger.
//   - dockerClient: Docker client used for recovery and restart-policy updates.
//   - watchtowerContainer: Current Watchtower container instance, or nil.
func exitInvalidWatchtowerRestart(
	log *zerolog.Logger,
	dockerClient container.Client,
	watchtowerContainer types.Container,
) {
	log.Info().
		Msg("Detected invalid restart of old Watchtower container, stopping Watchtower container now")

	exitCtx, exitCancel := context.WithTimeout(
		context.Background(),
		containerLookupTimeout,
	)

	// Attempt to recover an orphaned Watchtower container that is stuck
	// in the created state before exiting. If recovery succeeds, the
	// old container still exits with restart policy set to "no".
	recoverCtx, recoverCancel := context.WithTimeout(
		context.Background(),
		restartPolicyTimeout,
	)

	recoveredContainer, recovered := actions.TryRecoverOrphanedContainer(
		log,
		recoverCtx,
		dockerClient,
		watchtowerContainer,
	)
	if recovered {
		log.Info().
			Str("container", recoveredContainer.Name()).
			Msg("Recovered orphaned Watchtower container, exiting old instance")
	}

	// Prevent the old container from being restarted by the runtime after exit.
	dockerClient.SetRestartPolicy(
		exitCtx,
		watchtowerContainer,
		dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
	)

	recoverCancel()
	exitCancel()
	os.Exit(0)
}

// This function bridges configuration loading and the application's primary workflow.
//
// Parameters:
//   - command: The cobra.Command instance being executed, providing access to parsed flags.
//   - args: A slice of container names provided as positional arguments, used for filtering.
func (p *process) run(command *cobra.Command, args []string) {
	p.log.Debug().
		Strs("positional_args", args).
		Msg("Received positional arguments for container filtering")

	// Reload configuration with positional names so the filter includes them.
	loaded, err := appConfig.Load(p.log, command, args)
	if err != nil {
		if currentWatchtowerContainer != nil {
			setNoRestartPolicyCtx, cancel := context.WithTimeout(
				context.Background(),
				restartPolicyTimeout,
			)
			client.SetRestartPolicy(
				setNoRestartPolicyCtx,
				currentWatchtowerContainer,
				dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
			)

			cancel()
		}

		p.log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	appCfg = loaded

	normalizedContainerNames := append([]string(nil), appCfg.Filter.Names...)

	// Prefer explicit scope, then scope derived from the container label (self-update persistence).
	effectiveScope, scopeErr := container.GetEffectiveScope(
		currentWatchtowerContainer,
		appCfg.Filter.Scope,
	)
	if scopeErr != nil {
		p.log.Debug().
			Err(scopeErr).
			Msg("Scope derivation failed, continuing with current scope")
	} else if effectiveScope != appCfg.Filter.Scope {
		appCfg.Filter.Scope = effectiveScope

		// Rebuild the filter predicate with the effective scope.
		predicate, desc, filterErr := filters.BuildFilter(
			p.log,
			appCfg.Filter.Names,
			appCfg.Filter.DisableContainers,
			appCfg.Filter.MonitorImageNames,
			appCfg.Filter.SkipImageNames,
			appCfg.Filter.EnableContainersByLabel,
			appCfg.Filter.DisableContainersByLabel,
			appCfg.Filter.LabelEnable,
			appCfg.Filter.Scope,
		)
		if filterErr != nil {
			if currentWatchtowerContainer != nil {
				setNoRestartPolicyCtx, cancel := context.WithTimeout(
					context.Background(),
					restartPolicyTimeout,
				)
				client.SetRestartPolicy(
					setNoRestartPolicyCtx,
					currentWatchtowerContainer,
					dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
				)

				cancel()
			}

			p.log.Fatal().Err(filterErr).Msg("Failed to build container filter")
		}

		appCfg.Filter.Predicate = predicate
		appCfg.Filter.Desc = desc
	}

	if appCfg.Mode.HealthCheck {
		if os.Getpid() == 1 {
			time.Sleep(1 * time.Second)
			p.log.Fatal().
				Msg("The health check flag should never be passed to the main watchtower container process")
		}

		return
	}

	cfg, err := appCfg.BuildRunConfig(appConfig.RunConfigInput{
		Command: command,
		Names:   normalizedContainerNames,
	})
	if err != nil {
		p.log.Fatal().Err(err).Msg("Failed to build run configuration")
	}

	// Warn if HTTP API configuration options are set without an endpoint enabled.
	if !appConfig.HTTPAPIEndpointsEnabled(cfg) && appConfig.AnyHTTPAPIConfig(cfg) {
		p.log.Warn().
			Msg("HTTP API configuration options are set, but no endpoints are enabled.")
	}

	// Execute core logic and exit with the returned status code (0 for success, 1 for failure).
	exitCode := p.runMain(cfg)
	if exitCode != 0 {
		p.log.Debug().
			Int("exit_code", exitCode).
			Msg("Exiting with non-zero status")
		os.Exit(exitCode)
	}
}

// runMain contains the core Watchtower logic after early exits are handled.
//
// It validates rolling-restart compatibility, performs one-time updates when run-once is set,
// cleans up excess Watchtower instances, sets up the HTTP API when endpoints are enabled,
// and schedules periodic updates while managing context and concurrency for graceful shutdown.
// Update policy is taken from appCfg.UpdateParams so run-once, schedule, and API paths share
// a complete types.UpdateParams snapshot.
//
// Parameters:
//   - cfg: The RunConfig struct containing filter, API, and mode parameters for this execution.
//
// Returns:
//   - int: An exit code (0 for success, 1 for failure) used to terminate the program.
func (p *process) runMain(cfg types.RunConfig) int {
	// Log the container names being processed for debugging visibility.
	p.log.Debug().
		Strs("container_names", cfg.Names).
		Msg("Processing specified containers")

	// Validate flag compatibility to prevent conflicting operational modes.
	if appCfg.Update.RollingRestart && appCfg.Update.MonitorOnly {
		setNoRestartPolicyCtx, cancel := context.WithTimeout(
			context.Background(),
			restartPolicyTimeout,
		)
		defer cancel()

		client.SetRestartPolicy(
			setNoRestartPolicyCtx,
			currentWatchtowerContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)

		p.log.Fatal().
			Bool("rolling_restart", appCfg.Update.RollingRestart).
			Bool("monitor_only", appCfg.Update.MonitorOnly).
			Msg("Incompatible flags: rolling restarts and monitor-only")
	}

	// Ensure the Docker client is fully initialized before proceeding.
	awaitDockerClient(p.log)

	// Initialize the event broadcaster for SSE subscribers.
	// Declared before runUpdatesWithNotifications so the closure can capture it.
	eventsBroadcaster := events.NewBroadcaster()

	// runUpdatesWithNotifications performs container updates and sends notifications about the results.
	//
	// It executes the update action with configured parameters, batches notifications, and returns a metric
	// summarizing the session for monitoring purposes, ensuring users are informed of update outcomes.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts.
	//   - filter: The types.Filter determining which containers are targeted for updates.
	//   - params: The types.UpdateParams struct containing update configuration parameters.
	//
	// Returns:
	//   - *metrics.Metric: A pointer to a metric object summarizing the update session (scanned, updated, failed counts).
	gitClient := git.New(p.log, git.Options{
		Token:           appCfg.Git.Token,
		Username:        appCfg.Git.Username,
		Password:        appCfg.Git.Password,
		SSHKeyPath:      appCfg.Git.SSHKeyPath,
		SSHKnownHosts:   appCfg.Git.SSHKnownHosts,
		Timeout:         appCfg.Git.Timeout,
		CABundle:        appCfg.Git.CABundle,
		InsecureSkipTLS: appCfg.Git.InsecureSkipTLS,
	})

	ctx, stop := createSignalContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	runUpdatesWithNotifications = func(ctx context.Context, filter types.Filter, params types.UpdateParams) *metrics.Metric {
		update := params
		if filter != nil {
			update.Filter = filter
		}

		if update.CurrentContainerID == "" {
			update.CurrentContainerID = currentWatchtowerContainerID
		}

		return actions.RunUpdatesWithNotifications(ctx, actions.RunUpdatesWithNotificationsParams{
			Logger:                       p.log,
			Client:                       client,
			GitClient:                    gitClient,
			Notifier:                     notifier,
			NotificationSplitByContainer: appCfg.Notify.SplitByContainer,
			NotificationReport:           appCfg.Notify.Report,
			EventBroadcaster:             eventsBroadcaster,
			Update:                       update,
			// Stop the process, as a signal would, when this is an old instance.
			OnOldSelfDetected: stop,
		})
	}

	// If rolling restarts are enabled, validate that the containers being monitored for
	// updates do not have linked dependencies.
	if appCfg.Update.RollingRestart {
		err := actions.ValidateRollingRestartDependencies(
			p.log,
			ctx,
			client,
			cfg.Filter,
			appCfg.Update.UseComposeDependsOn,
		)
		if err != nil {
			p.logNotify("Rolling restart compatibility validation failed", err)

			// Update current Watchtower container's restart policy to "no" to prevent unwanted restarts
			setNoRestartPolicyCtx, cancel := context.WithTimeout(
				context.Background(),
				restartPolicyTimeout,
			)
			defer cancel()

			client.SetRestartPolicy(
				setNoRestartPolicyCtx,
				currentWatchtowerContainer,
				dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
			)

			return 1 // Exit immediately after logging failure
		}
	}

	// Share one update lock between the scheduler and the HTTP API, so their
	// updates never run at the same time.
	updateLock := scheduling.NewUpdateLock()

	baseParams := appCfg.UpdateParams(appConfig.RunOverrides{
		Filter:             cfg.Filter,
		SkipSelfUpdate:     currentWatchtowerContainerUnknown,
		CurrentContainerID: currentWatchtowerContainerID,
	})

	// Handle one-time update mode, executing updates and registering metrics.
	if cfg.RunOnce {
		// Write startup message from resolved config (no CLI flag reads).
		startup := appCfg.StartupParams(cfg)
		startup.Sched = time.Time{}
		startup.Filtering = cfg.FilterDesc
		startup.Scope = appCfg.Filter.Scope
		startup.Client = client
		startup.Notifier = notifier
		startup.Version = meta.Version
		startup.Logger = p.log
		logging.WriteStartupMessage(startup)

		params := baseParams
		params.RunOnce = true

		metric := runUpdatesWithNotifications(ctx, cfg.Filter, params)
		metrics.Default().RegisterScan(metric)
		notifier.Close()

		// Update current Watchtower container's restart policy to "no" to prevent unwanted restarts.
		setNoRestartPolicyCtx, cancel := context.WithTimeout(
			context.Background(),
			restartPolicyTimeout,
		)
		defer cancel()

		client.SetRestartPolicy(
			setNoRestartPolicyCtx,
			currentWatchtowerContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)

		return 0 // Exit after successful execution.
	}

	// Retrieve the current Watchtower container for cleanup operations.
	if currentWatchtowerContainer == nil && currentWatchtowerContainerID != "" {
		p.log.Warn().Msg("Current container not cached for cleanup")
	}

	// Check for and cleanup old Watchtower containers within scope.
	totalRemovedInstances, err := actions.RemoveExcessWatchtowerInstances(
		p.log,
		ctx,
		client,
		appCfg.Update.Cleanup,
		appCfg.Filter.Scope,
		nil, // Remove the images of stopped instances right away.
		currentWatchtowerContainer,
	)
	if err != nil {
		// Cleanup failure is non-fatal — log a warning and continue.
		// The old container may still be stopping. Forcing exit would leave
		// no Watchtower running. Continuing ensures the new instance operates
		// even if the old container could not be fully cleaned up.
		p.log.Warn().
			Err(err).
			Msg("Failed to clean up old Watchtower containers, continuing anyway")
	}

	// Check for and cleanup orphaned ephemeral orchestrator containers.
	// These may persist if the orchestrator crashed or was killed unexpectedly.
	// With AutoRemove: true, this is a safety net for edge cases.
	removedOrchestratorCount, orchestratorErr := container.RemoveOrphanedOrchestrators(p.log, ctx, client)
	if orchestratorErr != nil {
		p.log.Warn().
			Err(orchestratorErr).
			Int("removed_orchestrators", removedOrchestratorCount).
			Msg("Failed to clean up orphaned orchestrator containers, continuing anyway")
	} else if removedOrchestratorCount > 0 {
		p.log.Debug().
			Int("removed_orchestrators", removedOrchestratorCount).
			Msg("Cleaned up orphaned orchestrator containers")
	}

	// Track whether cleanup occurred to prevent redundant updates after self-update.
	cleanupOccurred := totalRemovedInstances > 0
	// Disable update-on-start if cleanup occurred to prevent redundant updates after self-update.
	if cleanupOccurred {
		cfg.UpdateOnStart = false

		p.log.Debug().Msg("Disabled update-on-start due to cleanup of old Watchtower containers")
	}

	// Determine whether self-update should be skipped because the running
	// Watchtower container has published host ports. Docker cannot rebind
	// an occupied port during container replacement. Ephemeral self-updates
	// are exempt, because they remove the old container before creating the new
	// one, so no port conflict occurs.
	//
	// Perform this check here rather than inside SetupAndStartAPI so the
	// warning always appears, even when no HTTP API endpoints are enabled
	// and SetupAndStartAPI returns early.
	skipSelfUpdate := currentWatchtowerContainer != nil &&
		currentWatchtowerContainer.HasExposedPorts() &&
		!appCfg.Update.EphemeralSelfUpdate
	if skipSelfUpdate {
		p.log.Warn().Msg("Published port detected - self-updates disabled.")
	}

	// One UpdateParams snapshot for HTTP API and schedule paths.
	sharedBase := baseParams
	if skipSelfUpdate {
		sharedBase.SkipSelfUpdate = true
	}

	// Startup messaging snapshot. Sched and UpdateOnStart are filled by schedule or API
	// callers. Populate the rest here so scheduling does not re-derive them from scalar deps.
	startupBase := appCfg.StartupParams(cfg)
	startupBase.Filtering = cfg.FilterDesc
	startupBase.Scope = appCfg.Filter.Scope
	startupBase.Client = client
	startupBase.Notifier = notifier
	startupBase.Version = meta.Version
	startupBase.Logger = p.log

	// API request/auth logs must not fan into notification hooks.
	apiLog := p.log.With().Str("notify", "no").Logger()

	err = api.SetupAndStartAPI(
		ctx,
		config.Options{
			Logger:                       &apiLog,
			Host:                         cfg.APIHost,
			Port:                         cfg.APIPort,
			Token:                        cfg.APIToken,
			EventsToken:                  cfg.APIEventsToken,
			RateLimit:                    cfg.APIRateLimit,
			EnableCheckAPI:               cfg.EnableCheckAPI,
			EnableConfigAPI:              cfg.EnableConfigAPI,
			EnableContainersAPI:          cfg.EnableContainersAPI,
			EnableEventsAPI:              cfg.EnableEventsAPI,
			EnableHealthAPI:              cfg.EnableHealthAPI,
			EnableHistoryAPI:             cfg.EnableHistoryAPI,
			EnableImagesAPI:              cfg.EnableImagesAPI,
			EnableMetricsAPI:             cfg.EnableMetricsAPI,
			EnableSwaggerAPI:             cfg.EnableSwaggerAPI,
			EnableUpdateAPI:              cfg.EnableUpdateAPI,
			CheckTimeout:                 cfg.CheckAPITimeout,
			UpdateTimeout:                cfg.UpdateAPITimeout,
			TLSCertPath:                  cfg.TLSCertPath,
			TLSKeyPath:                   cfg.TLSKeyPath,
			CORSAllowedOrigins:           cfg.CORSAllowedOrigins,
			TrustedProxies:               cfg.TrustedProxies,
			ProxyHeader:                  cfg.ProxyHeader,
			UnblockHTTPAPI:               cfg.UnblockHTTPAPI,
			NoStartupMessage:             cfg.NoStartupMessage,
			Filter:                       cfg.Filter,
			FilterDesc:                   cfg.FilterDesc,
			UpdateLock:                   updateLock,
			BaseParams:                   sharedBase,
			IncludeStopped:               appCfg.Client.IncludeStopped,
			IncludeRestarting:            appCfg.Client.IncludeRestarting,
			LabelEnable:                  appCfg.Filter.LabelEnable,
			Client:                       client,
			GitClient:                    gitClient,
			Notifier:                     notifier,
			NotificationSplitByContainer: appCfg.Notify.SplitByContainer,
			Scope:                        appCfg.Filter.Scope,
			Version:                      meta.Version,
			Startup:                      startupBase,
			RunUpdatesWithNotifications:  runUpdatesWithNotifications,
			FilterByImage: func(images []string, base types.Filter) types.Filter {
				return filters.FilterByImage(p.log, images, base)
			},
			DefaultMetrics:      metrics.Default,
			WriteStartupMessage: logging.WriteStartupMessage,
			EventBroadcaster:    eventsBroadcaster,
			OnUnexpectedServerStop: func(listenErr error) {
				p.log.Error().
					Err(listenErr).
					Msg("Canceling process context after unexpected HTTP server stop")
				stop()
			},
		},
	)
	if err != nil {
		p.logNotify("API setup failed", err)

		// Update current Watchtower container's restart policy to "no" to prevent unwanted restarts
		setNoRestartPolicyCtx, cancel := context.WithTimeout(
			context.Background(),
			restartPolicyTimeout,
		)
		defer cancel()

		client.SetRestartPolicy(
			setNoRestartPolicyCtx,
			currentWatchtowerContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)

		return 1 // Exit while indicating failure.
	}

	// Schedule and execute periodic updates, handling errors or shutdown.
	// The startup message is skipped here if it was already sent by the HTTP API in blocking mode.
	startupMessageSent := cfg.EnableUpdateAPI && !cfg.UnblockHTTPAPI

	err = scheduling.RunUpgradesOnSchedule(ctx, scheduling.ScheduleDeps{
		Logger:                     p.log,
		Filter:                     cfg.Filter,
		Lock:                       updateLock,
		ScheduleSpec:               appCfg.Schedule.Spec,
		Startup:                    startupBase,
		WriteStartupMessage:        logging.WriteStartupMessage,
		RunUpdate:                  runUpdatesWithNotifications,
		Notifier:                   notifier,
		UpdateOnStart:              cfg.UpdateOnStart,
		SkipFirstRun:               cleanupOccurred,
		CurrentWatchtowerContainer: currentWatchtowerContainer,
		StartupMessageSent:         startupMessageSent,
		BaseParams:                 sharedBase,
	})
	if err != nil {
		p.logNotify("Scheduled upgrades failed", err)

		// Update current Watchtower container's restart policy to "no" to prevent unwanted restarts
		setNoRestartPolicyCtx, cancel := context.WithTimeout(
			context.Background(),
			restartPolicyTimeout,
		)
		defer cancel()

		client.SetRestartPolicy(
			setNoRestartPolicyCtx,
			currentWatchtowerContainer,
			dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
		)

		return 1 // Exit while indicating failure.
	}

	return 0 // Default to success if execution completes without errors.
}

// logNotify logs an error message and ensures notifications are sent before returning control.
//
// It uses a specific message if provided, falling back to a generic one, and includes the error in fields.
//
// Parameters:
//   - msg: A string specifying the error context (e.g., "Sanity check failed"), optional.
//   - err: The error to log and include in notifications.
func (p *process) logNotify(msg string, err error) {
	if msg == "" {
		msg = "Operation failed"
	}

	p.log.Error().Err(err).Msg(msg)
	notifier.StartNotification(false)
	notifier.SendNotification(nil)
	notifier.Close()
}

// awaitDockerClient introduces a brief delay to ensure the Docker client is fully initialized.
//
// It pauses execution for one second to mitigate potential race conditions during startup,
// giving the Docker API time to stabilize before Watchtower begins interacting with containers.
func awaitDockerClient(log *zerolog.Logger) {
	log.Debug().
		Msg("Sleeping for a second to ensure the docker api client has been properly initialized.")
	sleepFunc(1 * time.Second)
}

// resolveCurrentWatchtowerContainerForFallback resolves the current Watchtower container
// for use in the orchestrator fallback path.
//
// It attempts to detect the current container ID and retrieve the container object,
// returning nil if any step fails.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts.
//   - c: Container client for Docker API operations.
//
// Returns:
//   - types.Container: The resolved Watchtower container, or nil if detection fails.
func resolveCurrentWatchtowerContainerForFallback(log *zerolog.Logger, ctx context.Context, c container.Client) types.Container {
	lookupCtx, cancel := context.WithTimeout(ctx, containerLookupTimeout)
	defer cancel()

	containerID, err := container.GetCurrentContainerID(log, lookupCtx, c)
	if err == nil && containerID != "" {
		resolvedContainer, _ := c.GetCurrentWatchtowerContainer(lookupCtx, containerID)

		return resolvedContainer
	}

	return nil
}
