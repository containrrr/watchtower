package cmd

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/pkg/api"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/notifications"
	t "github.com/containrrr/watchtower/pkg/types"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (
	client   container.Client
	notifier *notifications.Notifier
	c        flags.WatchConfig
)

var rootCmd = &cobra.Command{
	Use:   "watchtower",
	Short: "Automatically updates running Docker containers",
	Long: `
Watchtower automatically updates running Docker containers whenever a new image is released.
More information available at https://github.com/containrrr/watchtower/.
`,
	Run:    Run,
	PreRun: PreRun,
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

	// Update schedule if interval helper is set
	if viper.IsSet("interval") {
		if viper.IsSet("schedule") {
			log.Fatal("only schedule or interval can be defined, not both")
		}
		interval := viper.GetInt("interval")
		viper.Set("schedule", fmt.Sprintf("@every %ds", interval))
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

	notifier = notifications.NewNotifier(cmd)
}

// Run is the main execution flow of the command
func Run(_ *cobra.Command, names []string) {
	filter := filters.BuildFilter(names, c.EnableLabel, c.Scope)

	if c.RunOnce {
		if !c.NoStartupMessage {
			log.Info("Running a one time update.")
		}
		runUpdatesWithNotifications(filter)
		notifier.Close()
		os.Exit(0)
		return
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, c.Cleanup, c.Scope); err != nil {
		log.Fatal(err)
	}

	if c.HTTPAPI {
		if err := api.SetupHTTPUpdates(c.HTTPAPIToken, func() { runUpdatesWithNotifications(filter) }); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		api.WaitForHTTPUpdates()
	}

	if err := runUpgradesOnSchedule(filter); err != nil {
		log.Error(err)
	}

	os.Exit(1)
}

func runUpgradesOnSchedule(filter t.Filter) error {
	tryLockSem := make(chan bool, 1)
	tryLockSem <- true

	runner := cron.New()
	err := runner.AddFunc(
		viper.GetString("schedule"),
		func() {
			select {
			case v := <-tryLockSem:
				defer func() { tryLockSem <- v }()
				runUpdatesWithNotifications(filter)
			default:
				log.Debug("Skipped another update already running.")
			}

			nextRuns := runner.Entries()
			if len(nextRuns) > 0 {
				log.Debug("Scheduled next run: " + nextRuns[0].Next.String())
			}
		})

	if err != nil {
		return err
	}

	if !viper.GetBool("no-startup-message") {
		log.Info("Starting Watchtower and scheduling first run: " + runner.Entries()[0].Schedule.Next(time.Now()).String())
	}

	runner.Start()

	// Graceful shut-down on SIGINT/SIGTERM
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	<-interrupt
	runner.Stop()
	log.Info("Waiting for running update to be finished...")
	<-tryLockSem
	return nil
}

func runUpdatesWithNotifications(filter t.Filter) {
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
	err := actions.Update(client, updateParams)
	if err != nil {
		log.Println(err)
	}
	notifier.SendNotification()
}
