// Package scheduling provides functionality for scheduling and executing container updates in Watchtower.
// It handles periodic scheduling using cron specifications, manages update concurrency, and ensures
// graceful shutdown of scheduled operations.
package scheduling

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// updateWaitTimeout bounds how long shutdown waits for an in-flight update.
const updateWaitTimeout = 60 * time.Second

// WaitForRunningUpdate waits for a running update to finish before shutdown
// and then keeps the update lock, so no further update starts during shutdown.
//
// The wait ends after updateWaitTimeout, or when ctx is done, whichever comes
// first. The running update then keeps the lock.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil. A nil logger panics on the first log call.
//   - ctx: The context for cancellation, allowing early shutdown on context timeout.
//   - lock: The update lock shared by every caller that starts updates.
func WaitForRunningUpdate(log *zerolog.Logger, ctx context.Context, lock *UpdateLock) {
	log.Debug().Msg("Checking lock status before shutdown.")

	if lock.TryLock() {
		log.Debug().Msg("No update running, lock available.")
	} else {
		waitCtx, cancel := context.WithTimeout(ctx, updateWaitTimeout)
		defer cancel()

		err := lock.Lock(waitCtx)

		switch {
		case err == nil:
			log.Debug().Msg("Lock acquired, update finished.")
		case errors.Is(err, context.DeadlineExceeded):
			log.Warn().Msg("Timeout waiting for running update to finish, proceeding with shutdown.")
		default:
			log.Warn().Msg("Context canceled while waiting for running update.")
		}
	}

	log.Debug().Msg("Lock check completed.")
}

// ScheduleDeps holds dependencies for scheduled update runs.
//
// BaseParams must be a complete types.UpdateParams snapshot from config.UpdateParams
// (or an equivalent full construction).
// Each tick copies BaseParams and applies only per-run fields such as SkipSelfUpdate.
// A tick can disable self-updates, but never enables them when BaseParams disables them.
type ScheduleDeps struct {
	// Logger is the process logger for scheduled runs. Required and must be non-nil.
	Logger *zerolog.Logger
	// Filter determines which containers are updated.
	Filter types.Filter
	// Lock ensures only one update runs at a time. Share it with every other
	// caller that starts updates, such as the HTTP API. Nil gives the scheduler
	// a lock of its own.
	Lock *UpdateLock
	// ScheduleSpec is the cron-formatted schedule string for periodic updates.
	ScheduleSpec string
	// Startup holds resolved values for startup messaging (no flag reads).
	// Callers must set Filtering, Scope, Client, Notifier, and Version on Startup.
	// RunUpgradesOnSchedule only applies Sched and UpdateOnStart at send time.
	Startup logging.StartupParams
	// WriteStartupMessage writes the startup message with scheduling information.
	WriteStartupMessage func(logging.StartupParams)
	// RunUpdate performs container updates and sends notifications.
	RunUpdate func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric
	// Notifier is closed on schedule shutdown. Prefer Startup.Notifier for messaging.
	Notifier types.Notifier
	// UpdateOnStart triggers an immediate update before the scheduler starts.
	UpdateOnStart bool
	// SkipFirstRun skips Watchtower self-update on the first scheduled run
	// (useful after self-update cleanup of old instances).
	SkipFirstRun bool
	// CurrentWatchtowerContainer is the running Watchtower container for parent checking.
	CurrentWatchtowerContainer types.Container
	// StartupMessageSent is true when the startup message was already sent
	// (for example by the HTTP API in blocking mode).
	StartupMessageSent bool
	// BaseParams is the complete update policy snapshot for every scheduled tick.
	// Must include Cleanup, MonitorOnly, UseComposeDependsOn, ReviveStopped, and
	// all other process-wide UpdateParams fields.
	BaseParams types.UpdateParams
}

// RunUpgradesOnSchedule schedules and executes periodic container updates according to the cron specification.
//
// It sets up a cron scheduler, runs updates at specified intervals, and shuts down gracefully when ctx is
// canceled, handling concurrency with an update lock. The caller cancels ctx on shutdown signals.
// If update-on-start is enabled, it triggers the first update immediately before starting the scheduler.
// If SkipFirstRun is true, it skips Watchtower self-update on the first scheduled run (useful after self-update cleanup).
//
// Parameters:
//   - ctx: The context controlling the scheduler's lifecycle. Canceling it, for example on SIGINT or SIGTERM, shuts
//     the scheduler down.
//   - deps: Schedule dependencies including a complete BaseParams policy snapshot.
//     deps.Logger is required and must be non-nil (nil panics on first log call).
//
// Returns:
//   - error: An error if scheduling fails (e.g., invalid cron spec), nil on successful shutdown.
func RunUpgradesOnSchedule(ctx context.Context, deps ScheduleDeps) error {
	log := deps.Logger
	// Initialize lock if not provided, ensuring single-update concurrency.
	lock := deps.Lock
	if lock == nil {
		lock = NewUpdateLock()
	}

	// Create a new cron scheduler for managing periodic updates.
	// Configured with optional seconds, skip overlapping runs, and panic recovery.
	scheduler := cron.New(
		cron.WithParser(
			cron.NewParser(
				cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
			),
		),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)

	// Determine if self-update should be skipped due to exposed port conflicts.
	// When a port is configured (e.g., HTTP API), the old container holds the port
	// while the new container tries to bind it, resulting in both containers being stopped.
	// Ephemeral self-updates are exempt from this restriction because they remove
	// the old container before creating the new one, avoiding port conflicts.
	skipSelfUpdateForPorts := deps.CurrentWatchtowerContainer != nil &&
		deps.CurrentWatchtowerContainer.HasExposedPorts() &&
		!deps.BaseParams.EphemeralSelfUpdate

	// Define the update function to be used both for scheduled runs and immediate execution.
	// skipWatchtowerSelfUpdate: whether to skip updating the Watchtower container itself
	// blocking: whether to wait for the lock (true for scheduled runs, false for immediate runs)
	updateFunc := func(skipWatchtowerSelfUpdate, blocking bool) {
		// Skip self-update if the container has exposed ports to prevent port conflicts.
		// This takes precedence over the skipWatchtowerSelfUpdate parameter.
		if skipSelfUpdateForPorts {
			skipWatchtowerSelfUpdate = true

			log.Debug().Msg("Published ports detected - self-update skipped.")
		}

		// Skip update if this is a Watchtower parent container (from self-update chain)
		if deps.CurrentWatchtowerContainer != nil {
			chain, _ := deps.CurrentWatchtowerContainer.GetContainerChain()

			if container.IsWatchtowerParent(deps.CurrentWatchtowerContainer.ID(), chain) {
				log.Debug().Msg("Skipping scheduled update for Watchtower parent container")

				nextRuns := scheduler.Entries()
				if len(nextRuns) > 0 {
					log.Debug().Msg(
						"Scheduled next run: " + nextRuns[0].Schedule.Next(time.Now()).String(),
					)
				}

				return
			}
		}

		// Acquire the update lock: blocking waits until shutdown, non-blocking returns if unavailable
		if blocking {
			err := lock.Lock(ctx)
			if err != nil {
				log.Debug().Err(err).Msg("Update skipped: shutting down while waiting for the update lock")

				return
			}
		} else if !lock.TryLock() {
			log.Debug().Msg("Update skipped: another update is currently running")

			return
		}

		defer lock.Unlock()

		// Shutdown can begin just after the lock is taken. A run started then
		// would only be canceled and reported as a failed scan.
		if ctx.Err() != nil {
			log.Debug().Err(ctx.Err()).Msg("Update skipped: shutting down")

			return
		}

		params := deps.BaseParams
		params.RunOnce = false
		params.SkipSelfUpdate = skipWatchtowerSelfUpdate || deps.BaseParams.SkipSelfUpdate

		// One filter for this tick: schedule filter when set, else BaseParams.
		// Keep params.Filter and the positional argument identical so
		// runUpdatesWithNotifications cannot prefer a divergent source.
		updateFilter := deps.Filter
		if updateFilter == nil {
			updateFilter = params.Filter
		}

		params.Filter = updateFilter

		if deps.RunUpdate == nil {
			log.Debug().Msg("Update skipped: RunUpdate hook is not configured")

			return
		}

		metric := deps.RunUpdate(ctx, updateFilter, params)
		if metric != nil {
			metrics.Default().RegisterScan(metric)
		}

		log.Debug().Msg("Update operation completed")

		nextRuns := scheduler.Entries()
		if len(nextRuns) > 0 {
			log.Debug().Msg("Scheduled next run: " + nextRuns[0].Schedule.Next(time.Now()).String())
		}
	}

	// Wrapper function that can skip Watchtower self-update on the first run if needed
	var scheduledUpdateFunc func()

	// If Watchtower has performed a self-cleanup, then prevent Watchtower
	// from self-updating during the first update cycle.
	if deps.SkipFirstRun {
		var firstRun atomic.Uint32 // atomic flag to track if this is the first run

		scheduledUpdateFunc = func() {
			// Atomically check and set firstRun to ensure only the first execution skips self-update
			skipWatchtowerSelfUpdate := firstRun.CompareAndSwap(0, 1)
			if skipWatchtowerSelfUpdate {
				log.Debug().Msg(
					"Skipping Watchtower self-update on first scheduled run due to cleanup",
				)
			}

			updateFunc(skipWatchtowerSelfUpdate, true)
		}
	} else {
		scheduledUpdateFunc = func() { updateFunc(false, true) }
	}

	// Add the update function to the cron schedule, handling concurrency and metrics.
	scheduleSpec := strings.Trim(deps.ScheduleSpec, `"'`)
	if scheduleSpec != "" {
		_, err := scheduler.AddFunc(
			scheduleSpec,
			scheduledUpdateFunc,
		)
		if err != nil {
			return fmt.Errorf("failed to schedule updates: %w", err)
		}
	}

	// Log startup message with the first scheduled run time.
	// Skip if the startup message was already sent (e.g., by the HTTP API in blocking mode).
	var nextRun time.Time
	if len(scheduler.Entries()) > 0 {
		nextRun = scheduler.Entries()[0].Schedule.Next(time.Now())
	}

	// Log startup message with the first scheduled run time.
	// Skip if the startup message was already sent (for example by the HTTP API in blocking mode).
	// Startup is the single source of truth for messaging fields (Filtering, Scope, Client,
	// Notifier, Version). Only apply scheduler-owned runtime values here.
	if !deps.StartupMessageSent && deps.WriteStartupMessage != nil {
		startup := deps.Startup
		startup.Sched = nextRun
		startup.UpdateOnStart = &deps.UpdateOnStart
		deps.WriteStartupMessage(startup)
	}

	// Check if update-on-start is enabled and trigger immediate update if so.
	// Skip it when the process is already stopping, so a canceled update is not
	// reported as a failure.
	if deps.UpdateOnStart && ctx.Err() == nil {
		updateFunc(false, false)
	}

	// Start the scheduler to begin periodic execution if scheduling is enabled.
	// Only start if a schedule spec was provided (empty string means no scheduling).
	if scheduleSpec != "" {
		scheduler.Start()
	}

	// Wait for cancellation. The caller's context carries shutdown signals.
	<-ctx.Done()
	log.Debug().Msg("Context canceled, stopping scheduler...")

	// Stop the scheduler and wait for any running update to complete.
	scheduler.Stop()
	log.Debug().Msg("Waiting for running update to be finished...")

	// Original ctx is often already canceled at shutdown. Detach cancel and
	// apply a bound so an active RunUpdate can finish before Notifier.Close().
	waitCtx, waitCancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		updateWaitTimeout,
	)
	defer waitCancel()

	WaitForRunningUpdate(log, waitCtx, lock)

	// Close the notification system to clean up resources during shutdown.
	if deps.Notifier != nil {
		deps.Notifier.Close()
	}

	log.Debug().Msg("Scheduler stopped and update completed.")

	return nil
}

// ShouldExitDueToInvalidRestart determines if the program should exit due to an invalid restart of an old Watchtower container.
//
// This function checks two conditions:
//  1. The current container's name matches the watchtower-old-* prefix, indicating it is
//     a predecessor renamed during self-update that should not run.
//  2. The current container is present in the container chain label, indicating it is
//     an ancestor in the self-update lineage.
//
// If either condition is true and runOnce is false, the program should exit
// to prevent an old Watchtower container from running.
//
// Parameters:
//   - c: The current Watchtower container to check.
//   - runOnce: Whether the process is in run-once mode.
//
// Returns:
//   - bool: True if the program should exit due to an invalid restart, false otherwise.
func ShouldExitDueToInvalidRestart(c types.Container, runOnce bool) bool {
	if c == nil {
		return false
	}

	if container.IsOldContainer(c.Name()) && !runOnce {
		return true
	}

	chain, present := c.GetContainerChain()
	if !present {
		return false
	}

	return container.IsWatchtowerParent(c.ID(), chain) && !runOnce
}
