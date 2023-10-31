package cmd

import (
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/internal/meta"
	"github.com/containrrr/watchtower/internal/util"
	"github.com/containrrr/watchtower/pkg/api"
	"github.com/containrrr/watchtower/pkg/api/updates"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/notifications"
	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (
	client            container.Client
	scheduleSpec      string
	enableLabel       bool
	disableContainers []string
	notifier          t.Notifier
	scope             string

	up = t.UpdateParams{}
)

var rootCmd = NewRootCommand()
var localLog = notifications.LocalLog

// NewRootCommand creates the root command for watchtower
func NewRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watchtower",
		Short: "Automatically updates running Docker containers",
		Long: `
	Watchtower automatically updates running Docker containers whenever a new image is released.
	More information available at https://github.com/containrrr/watchtower/.
	`,
		Run:    Run,
		PreRun: PreRun,
		Args:   cobra.ArbitraryArgs,
	}
}

func init() {
	flags.SetDefaults()
	flags.RegisterDockerFlags(rootCmd)
	flags.RegisterSystemFlags(rootCmd)
	flags.RegisterNotificationFlags(rootCmd)
}

// Execute the root func and exit in case of errors
func Execute() {
	rootCmd.AddCommand(notifyUpgradeCommand)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// PreRun is a lifecycle hook that runs before the command is executed.
func PreRun(cmd *cobra.Command, _ []string) {
	f := cmd.PersistentFlags()
	flags.ProcessFlagAliases(f)
	if err := flags.SetupLogging(f); err != nil {
		log.Fatalf("Failed to initialize logging: %s", err.Error())
	}

	scheduleSpec, _ = f.GetString("schedule")

	flags.GetSecretsFromFiles(cmd)
	up.Cleanup, up.NoRestart, up.MonitorOnly, up.Timeout = flags.ReadFlags(cmd)

	if up.Timeout < 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}

	enableLabel, _ = f.GetBool("label-enable")
	disableContainers, _ = f.GetStringSlice("disable-containers")
	up.LifecycleHooks, _ = f.GetBool("enable-lifecycle-hooks")
	up.RollingRestart, _ = f.GetBool("rolling-restart")
	scope, _ = f.GetString("scope")
	up.LabelPrecedence, _ = f.GetBool("label-take-precedence")

	if scope != "" {
		log.Debugf(`Using scope %q`, scope)
	}

	// configure environment vars for client
	err := flags.EnvConfig(cmd)
	if err != nil {
		log.Fatal(err)
	}

	var clientOpts = container.ClientOptions{}

	noPull, _ := f.GetBool("no-pull")
	clientOpts.PullImages = !noPull
	clientOpts.IncludeStopped, _ = f.GetBool("include-stopped")
	clientOpts.IncludeRestarting, _ = f.GetBool("include-restarting")
	clientOpts.ReviveStopped, _ = f.GetBool("revive-stopped")
	clientOpts.RemoveVolumes, _ = f.GetBool("remove-volumes")
	warnOnHeadPullFailed, _ := f.GetString("warn-on-head-failure")
	clientOpts.WarnOnHeadFailed = container.WarningStrategy(warnOnHeadPullFailed)

	if up.MonitorOnly && noPull {
		log.Warn("Using `WATCHTOWER_NO_PULL` and `WATCHTOWER_MONITOR_ONLY` simultaneously might lead to no action being taken at all. If this is intentional, you may safely ignore this message.")
	}

	client = container.NewClient(clientOpts)

	notifier = notifications.NewNotifier(cmd)
	notifier.AddLogHook()
}

// Run is the main execution flow of the command
func Run(c *cobra.Command, names []string) {
	filter, filterDesc := filters.BuildFilter(names, disableContainers, enableLabel, scope)
	up.Filter = filter
	runOnce, _ := c.PersistentFlags().GetBool("run-once")
	enableUpdateAPI, _ := c.PersistentFlags().GetBool("http-api-updates")
	enableMetricsAPI, _ := c.PersistentFlags().GetBool("http-api-metrics")
	unblockHTTPAPI, _ := c.PersistentFlags().GetBool("http-api-periodic-polls")
	apiToken, _ := c.PersistentFlags().GetString("http-api-token")
	healthCheck, _ := c.PersistentFlags().GetBool("health-check")

	enableScheduler := !enableUpdateAPI || unblockHTTPAPI

	if healthCheck {
		// health check should not have pid 1
		if os.Getpid() == 1 {
			time.Sleep(1 * time.Second)
			log.Fatal("The health check flag should never be passed to the main watchtower container process")
		}
		os.Exit(0)
	}

	if up.RollingRestart && up.MonitorOnly {
		log.Fatal("Rolling restarts is not compatible with the global monitor only flag")
	}

	awaitDockerClient()

	if err := actions.CheckForSanity(client, up.Filter, up.RollingRestart); err != nil {
		logNotifyExit(err)
	}

	if runOnce {
		writeStartupMessage(c, time.Time{}, filterDesc)
		runUpdatesWithNotifications(up)
		notifier.Close()
		os.Exit(0)
		return
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, up.Cleanup, scope); err != nil {
		logNotifyExit(err)
	}

	// The lock is shared between the scheduler and the HTTP API. It only allows one updates to run at a time.
	updateLock := sync.Mutex{}

	httpAPI := api.New(apiToken)

	if enableUpdateAPI {
		httpAPI.EnableUpdates(func(paramsFunc updates.ModifyParamsFunc) t.Report {
			apiUpdateParams := up
			paramsFunc(&apiUpdateParams)
			if up.MonitorOnly && !apiUpdateParams.MonitorOnly {
				apiUpdateParams.MonitorOnly = true
				localLog.Warn("Ignoring request to disable monitor only through API")
			}
			report := runUpdatesWithNotifications(apiUpdateParams)
			metrics.RegisterScan(metrics.NewMetric(report))
			return report
		}, &updateLock)
	}

	if enableMetricsAPI {
		httpAPI.EnableMetrics()
	}

	if err := httpAPI.Start(); err != nil {
		log.Error("failed to start API", err)
	}

	var firstScan time.Time
	var scheduler *cron.Cron
	if enableScheduler {
		var err error
		scheduler, err = runUpgradesOnSchedule(up, &updateLock)
		if err != nil {
			log.Errorf("Failed to start scheduler: %v", err)
		} else {
			firstScan = scheduler.Entries()[0].Schedule.Next(time.Now())
		}
	}

	writeStartupMessage(c, firstScan, filterDesc)

	// Graceful shut-down on SIGINT/SIGTERM
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	recievedSignal := <-interrupt
	localLog.WithField("signal", recievedSignal).Infof("Got shutdown signal. Gracefully shutting down...")
	if scheduler != nil {
		scheduler.Stop()
	}

	updateLock.Lock()
	go func() {
		time.Sleep(time.Second * 3)
		updateLock.Unlock()
	}()

	waitFor(httpAPI.Stop(), "Waiting for HTTP API requests to complete...")
	waitFor(&updateLock, "Waiting for running updates to be finished...")

	localLog.Info("Shutdown completed")
}

func waitFor(waitLock *sync.Mutex, delayMessage string) {
	if !waitLock.TryLock() {
		log.Info(delayMessage)
		waitLock.Lock()
	}
}

func logNotifyExit(err error) {
	log.Error(err)
	notifier.Close()
	os.Exit(1)
}

func awaitDockerClient() {
	log.Debug("Sleeping for a second to ensure the docker api client has been properly initialized.")
	time.Sleep(1 * time.Second)
}

func writeStartupMessage(c *cobra.Command, sched time.Time, filtering string) {
	noStartupMessage, _ := c.PersistentFlags().GetBool("no-startup-message")
	enableUpdateAPI, _ := c.PersistentFlags().GetBool("http-api-updates")

	var startupLog *log.Entry
	if noStartupMessage {
		startupLog = notifications.LocalLog
	} else {
		startupLog = log.NewEntry(log.StandardLogger())
		// Batch up startup messages to send them as a single notification
		notifier.StartNotification()
	}

	startupLog.Info("Watchtower ", meta.Version)

	notifierNames := notifier.GetNames()
	if len(notifierNames) > 0 {
		startupLog.Info("Using notifications: " + strings.Join(notifierNames, ", "))
	} else {
		startupLog.Info("Using no notifications")
	}

	startupLog.Info(filtering)

	if !sched.IsZero() {
		until := util.FormatDuration(time.Until(sched))
		startupLog.Info("Scheduling first run: " + sched.Format("2006-01-02 15:04:05 -0700 MST"))
		startupLog.Info("Note that the first check will be performed in " + until)
	} else if runOnce, _ := c.PersistentFlags().GetBool("run-once"); runOnce {
		startupLog.Info("Running a one time updates.")
	} else {
		startupLog.Info("Periodic runs are not enabled.")
	}

	if enableUpdateAPI {
		// TODO: make listen port configurable
		startupLog.Info("The HTTP API is enabled at :8080.")
	}

	if !noStartupMessage {
		// Send the queued up startup messages, not including the trace warning below (to make sure it's noticed)
		notifier.SendNotification(nil)
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		startupLog.Warn("Trace level enabled: log will include sensitive information as credentials and tokens")
	}
}

func runUpgradesOnSchedule(updateParams t.UpdateParams, updateLock *sync.Mutex) (*cron.Cron, error) {
	scheduler := cron.New()
	err := scheduler.AddFunc(
		scheduleSpec,
		func() {
			if updateLock.TryLock() {
				defer updateLock.Unlock()
				result := runUpdatesWithNotifications(updateParams)
				metrics.RegisterScan(metrics.NewMetric(result))
			} else {
				// Update was skipped
				metrics.RegisterScan(nil)
				log.Debug("Skipped another updates already running.")
			}

			nextRuns := scheduler.Entries()
			if len(nextRuns) > 0 {
				log.Debug("Scheduled next run: " + nextRuns[0].Next.String())
			}
		})

	if err != nil {
		return nil, err
	}

	scheduler.Start()

	return scheduler, nil
}

func runUpdatesWithNotifications(updateParams t.UpdateParams) t.Report {
	notifier.StartNotification()

	result, err := actions.Update(client, updateParams)
	if err != nil {
		log.Error(err)
	}
	notifier.SendNotification(result)

	localLog.WithFields(log.Fields{
		"Scanned": len(result.Scanned()),
		"Updated": len(result.Updated()),
		"Failed":  len(result.Failed()),
	}).Info("Session done")

	return result
}
