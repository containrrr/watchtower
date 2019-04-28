package main // import "github.com/containrrr/watchtower"

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"strconv"

	"github.com/containrrr/watchtower/actions"
	cliApp "github.com/containrrr/watchtower/app"
	"github.com/containrrr/watchtower/container"
	"github.com/containrrr/watchtower/notifications"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// DockerAPIMinVersion is the version of the docker API, which is minimally required by
// watchtower. Currently we require at least API 1.24 and therefore Docker 1.12 or later.
const DockerAPIMinVersion string = "1.24"

var version = "master"
var commit = "unknown"
var date = "unknown"

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

func init() {
	log.SetLevel(log.InfoLevel)
}

func main() {
	app := cli.NewApp()
	InitApp(app)
	cliApp.SetupCliFlags(app)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// InitApp initializes urfave app metadata and sets up entrypoints
func InitApp(app *cli.App) {
	app.Name = "watchtower"
	app.Version = version + " - " + commit + " - " + date
	app.Usage = "Automatically update running Docker containers"
	app.Before = before
	app.Action = start
}

func before(c *cli.Context) error {
	if c.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	pollingSet := c.IsSet("interval")
	cronSet := c.IsSet("schedule")
	cronLen := len(c.String("schedule"))

	if pollingSet && cronSet && cronLen > 0 {
		log.Fatal("Only schedule or interval can be defined, not both.")
	} else if cronSet && cronLen > 0 {
		scheduleSpec = c.String("schedule")
	} else {
		scheduleSpec = "@every " + strconv.Itoa(c.Int("interval")) + "s"
	}

	readFlags(c)

	if timeout < 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}
	enableLabel = c.GlobalBool("label-enable")

	// configure environment vars for client
	err := envConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(
		!c.GlobalBool("no-pull"),
		c.GlobalBool("include-stopped"),
	)
	notifier = notifications.NewNotifier(c)

	return nil
}

func start(c *cli.Context) error {
	names := c.Args()
	filter := container.BuildFilter(names, enableLabel)

	if c.GlobalBool("run-once") {
		log.Info("Running a one time update.")
		runUpdatesWithNotifications(filter)
		os.Exit(1)
		return nil
	}

	if err := actions.CheckForMultipleWatchtowerInstances(client, cleanup); err != nil {
		log.Fatal(err)
	}

	runUpgradesOnSchedule(filter)
	os.Exit(1)
	return nil
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

func setEnvOptStr(env string, opt string) error {
	if opt == "" || opt == os.Getenv(env) {
		return nil
	}
	err := os.Setenv(env, opt)
	if err != nil {
		return err
	}
	return nil
}

func setEnvOptBool(env string, opt bool) error {
	if opt == true {
		return setEnvOptStr(env, "1")
	}
	return nil
}

// envConfig translates the command-line options into environment variables
// that will initialize the api client
func envConfig(c *cli.Context) error {
	var err error

	err = setEnvOptStr("DOCKER_HOST", c.GlobalString("host"))
	err = setEnvOptBool("DOCKER_TLS_VERIFY", c.GlobalBool("tlsverify"))
	err = setEnvOptStr("DOCKER_API_VERSION", DockerAPIMinVersion)

	return err
}

func readFlags(c *cli.Context) {
	cleanup = c.GlobalBool("cleanup")
	noRestart = c.GlobalBool("no-restart")
	monitorOnly = c.GlobalBool("monitor-only")
	timeout = c.GlobalDuration("stop-timeout")
}
