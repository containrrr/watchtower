package cmd

import (
	"github.com/containrrr/watchtower/actions"
	"github.com/containrrr/watchtower/container"
	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/notifications"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// DockerAPIMinVersion is the minimum version of the docker api required to
// use watchtower
const DockerAPIMinVersion string = "1.24"

var (
	client       container.Client
	scheduleSpec string
	cleanup      bool
	noRestart    bool
	monitorOnly  bool
	enableLabel  bool
	notifier     *notifications.Notifier
	timeout      time.Duration
)

var rootCmd = &cobra.Command{
	Use:    "watchtower",
	Short:  "Automatically updates running Docker containers",
	Long:   `
Watchtower automatically updates running Docker containers whenever a new image is released.
More information available at https://github.com/containrrr/watchtower/.
`,
	Run:    Run,
	PreRun: PreRun,
}

func init() {
	flags.SetDefaults()
	flags.RegisterDockerFlags(rootCmd)
	flags.RegisterSystemFlags(rootCmd)
	flags.RegisterNotificationFlags(rootCmd)
}

// Execute the root func and exit in case of errors
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// PreRun is a lifecycle hook that runs before the command is executed.
func PreRun(cmd *cobra.Command, args []string) {
	f := cmd.PersistentFlags()

	if enabled, _ := f.GetBool("debug"); enabled == true {
		log.SetLevel(log.DebugLevel)
	}

	pollingSet := f.Changed("interval")
	schedule, _ := f.GetString("schedule")
	cronLen := len(schedule)

	if pollingSet && cronLen > 0 {
		log.Fatal("Only schedule or interval can be defined, not both.")
	} else if cronLen > 0 {
		scheduleSpec, _ = f.GetString("schedule")
	} else {
		interval, _ := f.GetInt("interval")
		scheduleSpec = "@every " + strconv.Itoa(interval) + "s"
	}

	cleanup, noRestart, monitorOnly, timeout = flags.ReadFlags(cmd)

	if timeout < 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}
	enableLabel, _ = f.GetBool("label-enable")

	// configure environment vars for client
	err := flags.EnvConfig(cmd, DockerAPIMinVersion)
	if err != nil {
		log.Fatal(err)
	}

	noPull, _ := f.GetBool("no-pull")
	includeStopped, _ := f.GetBool("include-stopped")
	client = container.NewClient(
		!noPull,
		includeStopped,
	)

	notifier = notifications.NewNotifier(cmd)
}

// Run is the main execution flow of the command
func Run(c *cobra.Command, names []string) {
	filter := container.BuildFilter(names, enableLabel)
	runOnce, _ := c.PersistentFlags().GetBool("run-once")

	if runOnce {
		log.Info("Running a one time update.")
		runUpdatesWithNotifications(filter)
		os.Exit(1)
		return
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, cleanup); err != nil {
		log.Fatal(err)
	}

	runUpgradesOnSchedule(filter)
	os.Exit(1)
}

func runUpgradesOnSchedule(filter container.Filter) error {
	tryLockSem := make(chan bool, 1)
	tryLockSem <- true

	cron := cron.New()
	err := cron.AddFunc(
		scheduleSpec,
		func() {
			select {
			case v := <-tryLockSem:
				defer func() { tryLockSem <- v }()
				runUpdatesWithNotifications(filter)
			default:
				log.Debug("Skipped another update already running.")
			}

			nextRuns := cron.Entries()
			if len(nextRuns) > 0 {
				log.Debug("Scheduled next run: " + nextRuns[0].Next.String())
			}
		})

	if err != nil {
		return err
	}

	log.Debug("Starting Watchtower and scheduling first run: " + cron.Entries()[0].Schedule.Next(time.Now()).String())
	cron.Start()

	// Graceful shut-down on SIGINT/SIGTERM
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	<-interrupt
	cron.Stop()
	log.Info("Waiting for running update to be finished...")
	<-tryLockSem
	return nil
}

func runUpdatesWithNotifications(filter container.Filter) {
	notifier.StartNotification()
	updateParams := actions.UpdateParams{
		Filter:      filter,
		Cleanup:     cleanup,
		NoRestart:   noRestart,
		Timeout:     timeout,
		MonitorOnly: monitorOnly,
	}
	err := actions.Update(client, updateParams)
	if err != nil {
		log.Println(err)
	}
	notifier.SendNotification()
}


