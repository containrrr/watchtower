package main // import "github.com/CenturyLinkLabs/watchtower"

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/CenturyLinkLabs/watchtower/actions"
	"github.com/CenturyLinkLabs/watchtower/container"
	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	wg           sync.WaitGroup
	client       container.Client
	pollInterval time.Duration
	cleanup      bool
)

func init() {
	log.SetLevel(log.InfoLevel)
}

func main() {
	app := cli.NewApp()
	app.Name = "watchtower"
	app.Usage = "Automatically update running Docker containers"
	app.Before = before
	app.Action = start
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.IntFlag{
			Name:  "interval, i",
			Usage: "poll interval (in seconds)",
			Value: 300,
		},
		cli.BoolFlag{
			Name:  "no-pull",
			Usage: "do not pull new images",
		},
		cli.BoolFlag{
			Name:  "cleanup",
			Usage: "remove old images after updating",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
		},
		cli.StringFlag{
			Name:   "apiversion",
			Usage:  "the version of the docker api",
			EnvVar: "DOCKER_API_VERSION",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	if c.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	pollInterval = time.Duration(c.Int("interval")) * time.Second
	cleanup = c.GlobalBool("cleanup")

	// configure environment vars for client
	err := envConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(!c.GlobalBool("no-pull"))

	handleSignals()
	return nil
}

func start(c *cli.Context) {
	names := c.Args()

	if err := actions.CheckPrereqs(client, cleanup); err != nil {
		log.Fatal(err)
	}

	for {
		wg.Add(1)
		if err := actions.Update(client, names, cleanup); err != nil {
			fmt.Println(err)
		}
		wg.Done()

		time.Sleep(pollInterval)
	}
}

func handleSignals() {
	// Graceful shut-down on SIGINT/SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		<-c
		wg.Wait()
		os.Exit(1)
	}()
}

func setEnvOptStr(env string, opt string) error {
	if opt != "" && opt != os.Getenv(env) {
		err := os.Setenv(env, opt)
		if err != nil {
			return err
		}
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
	err = setEnvOptStr("DOCKER_API_VERSION", c.GlobalString("apiversion"))

	return err
}
