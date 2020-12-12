package cmd

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/internal/meta"
	"github.com/containrrr/watchtower/pkg/api"
	apiMetrics "github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/api/update"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/containrrr/watchtower/pkg/notifications"
	t "github.com/containrrr/watchtower/pkg/types"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	client   container.Client
	notifier *notifications.Notifier
	c        flags.WatchConfig
)

var rootCmd = NewRootCommand()

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
	}
}

func init() {
	flags.RegisterDockerFlags(rootCmd)
	flags.RegisterSystemFlags(rootCmd)
	flags.RegisterNotificationFlags(rootCmd)
	flags.SetEnvBindings()
	flags.BindViperFlags(rootCmd)
}

// Execute the root func and exit in case of errors
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// PreRun is a lifecycle hook that runs before the command is executed.
func PreRun(cmd *cobra.Command, _ []string) {

	// First apply all the settings that affect the output
	if viper.GetBool("no-color") {
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
		})
	} else {
		// enable logrus built-in support for https://bixense.com/clicolors/
		log.SetFormatter(&log.TextFormatter{
			EnvironmentOverrideColors: true,
		})
	}

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	if viper.GetBool("trace") {
		log.SetLevel(log.TraceLevel)
	}

	interval := viper.GetInt("interval")

	// If empty, set schedule using interval helper value
	if viper.GetString("schedule") == "" {
		viper.Set("schedule", fmt.Sprintf("@every %ds", interval))
	} else if interval != flags.DefaultInterval {
		log.Fatal("only schedule or interval can be defined, not both")
	}

	// Then load the rest of the settings
	err := viper.Unmarshal(&c)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	flags.GetSecretsFromFiles()

	if c.Timeout <= 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}

	log.Debugf("Using scope %v", c.Scope)

	if err = flags.EnvConfig(); err != nil {
		log.Fatalf("failed to setup environment variables: %v", err)
	}

	if c.MonitorOnly && c.NoPull {
		log.Warn("Using `WATCHTOWER_NO_PULL` and `WATCHTOWER_MONITOR_ONLY` simultaneously might lead to no action being taken at all. If this is intentional, you may safely ignore this message.")
	}

	client = container.NewClient(&c)

	notifier = notifications.NewNotifier()
}

// Run is the main execution flow of the command
func Run(_ *cobra.Command, names []string) {
	filter, filterDesc := filters.BuildFilter(names, c.EnableLabel, c.Scope)

	if c.RollingRestart && c.MonitorOnly {
		log.Fatal("Rolling restarts is not compatible with the global monitor only flag")
	}

	awaitDockerClient()

	if err := actions.CheckForSanity(client, filter, c.RollingRestart); err != nil {
		logNotifyExit(err)
	}

	if c.RunOnce {
		writeStartupMessage(time.Time{}, filterDesc)
		runUpdatesWithNotifications(filter)
		notifier.Close()
		os.Exit(0)
		return
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, c.Cleanup, c.Scope); err != nil {
		logNotifyExit(err)
	}

	// The lock is shared between the scheduler and the HTTP API. It only allows one update to run at a time.
	updateLock := make(chan bool, 1)
	updateLock <- true

	httpAPI := api.New(c.HTTPAPIToken)

	if c.EnableUpdateAPI {
		updateHandler := update.New(func() { runUpdatesWithNotifications(filter) }, updateLock)
		httpAPI.RegisterFunc(updateHandler.Path, updateHandler.Handle)
	}

	if c.EnableMetricsAPI {
		metricsHandler := apiMetrics.New()
		httpAPI.RegisterHandler(metricsHandler.Path, metricsHandler.Handle)
	}

	if err := httpAPI.Start(c.EnableUpdateAPI && !c.UpdateAPIWithScheduler); err != nil {
		log.Error("failed to start API", err)
	}

	if err := runUpgradesOnSchedule(filter, filterDesc, updateLock); err != nil {
		log.Error(err)
	}

	os.Exit(1)
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

func formatDuration(d time.Duration) string {
	sb := strings.Builder{}

	hours := int64(d.Hours())
	minutes := int64(math.Mod(d.Minutes(), 60))
	seconds := int64(math.Mod(d.Seconds(), 60))

	if hours == 1 {
		sb.WriteString("1 hour")
	} else if hours != 0 {
		sb.WriteString(strconv.FormatInt(hours, 10))
		sb.WriteString(" hours")
	}

	if hours != 0 && (seconds != 0 || minutes != 0) {
		sb.WriteString(", ")
	}

	if minutes == 1 {
		sb.WriteString("1 minute")
	} else if minutes != 0 {
		sb.WriteString(strconv.FormatInt(minutes, 10))
		sb.WriteString(" minutes")
	}

	if minutes != 0 && (seconds != 0) {
		sb.WriteString(", ")
	}

	if seconds == 1 {
		sb.WriteString("1 second")
	} else if seconds != 0 || (hours == 0 && minutes == 0) {
		sb.WriteString(strconv.FormatInt(seconds, 10))
		sb.WriteString(" seconds")
	}

	return sb.String()
}

func writeStartupMessage(sched time.Time, filtering string) {
	if !c.NoStartupMessage {
		schedMessage := "Running a one time update."
		if !sched.IsZero() {
			until := formatDuration(time.Until(sched))
			schedMessage = "Scheduling first run: " + sched.Format("2006-01-02 15:04:05 -0700 MST") +
				"\nNote that the first check will be performed in " + until
		}

		notifs := "Using no notifications"
		notifList := notifier.String()
		if len(notifList) > 0 {
			notifs = "Using notifications: " + notifList
		}

		log.Info("Watchtower ", meta.Version, "\n", notifs, "\n", filtering, "\n", schedMessage)
		if log.IsLevelEnabled(log.TraceLevel) {
			log.Warn("trace level enabled: log will include sensitive information as credentials and tokens")
		}
	}
}

func runUpgradesOnSchedule(filter t.Filter, filtering string, lock chan bool) error {
	if lock == nil {
		lock = make(chan bool, 1)
		lock <- true
	}

	scheduler := cron.New()
	err := scheduler.AddFunc(
		c.Schedule,
		func() {
			select {
			case v := <-lock:
				defer func() { lock <- v }()
				metric := runUpdatesWithNotifications(filter)
				metrics.RegisterScan(metric)
			default:
				// Update was skipped
				metrics.RegisterScan(nil)
				log.Debug("Skipped another update already running.")
			}

			nextRuns := scheduler.Entries()
			if len(nextRuns) > 0 {
				log.Debug("Scheduled next run: " + nextRuns[0].Next.String())
			}
		})

	if err != nil {
		return err
	}

	writeStartupMessage(scheduler.Entries()[0].Schedule.Next(time.Now()), filtering)

	scheduler.Start()

	// Graceful shut-down on SIGINT/SIGTERM
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	<-interrupt
	scheduler.Stop()
	log.Info("Waiting for running update to be finished...")
	<-lock
	return nil
}

func runUpdatesWithNotifications(filter t.Filter) *metrics.Metric {
	notifier.StartNotification()
	updateParams := t.UpdateParams{
		Filter:         filter,
		Cleanup:        c.Cleanup,
		NoRestart:      c.NoRestart,
		Timeout:        c.Timeout,
		MonitorOnly:    c.MonitorOnly,
		LifecycleHooks: c.LifecycleHooks,
		RollingRestart: c.RollingRestart,
	}
	metricResults, err := actions.Update(client, updateParams)
	if err != nil {
		log.Error(err)
	}
	notifier.SendNotification()
	log.Debugf("Session done: %v scanned, %v updated, %v failed",
		metricResults.Scanned, metricResults.Updated, metricResults.Failed)
	return metricResults
}
