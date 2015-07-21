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
	"github.com/codegangsta/cli"
)

var (
	wg sync.WaitGroup
)

func main() {
	app := cli.NewApp()
	app.Name = "watchtower"
	app.Usage = "Automatically update running Docker containers"
	app.Before = before
	app.Action = start
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Value:  "unix:///var/run/docker.sock",
			Usage:  "Docker daemon socket to connect to",
			EnvVar: "DOCKER_HOST",
		},
		cli.IntFlag{
			Name:  "interval, i",
			Value: 300,
			Usage: "poll interval (in seconds)",
		},
		cli.BoolFlag{
			Name:  "no-pull",
			Usage: "do not pull new images",
		},
	}

	handleSignals()
	app.Run(os.Args)
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

func before(c *cli.Context) error {
	client := newContainerClient(c)
	return actions.CheckPrereqs(client)
}

func start(c *cli.Context) {
	client := newContainerClient(c)
	secs := time.Duration(c.Int("interval")) * time.Second

	for {
		wg.Add(1)
		if err := actions.Update(client); err != nil {
			fmt.Println(err)
		}
		wg.Done()

		time.Sleep(secs)
	}
}

func newContainerClient(c *cli.Context) container.Client {
	dockerHost := c.GlobalString("host")
	noPull := c.GlobalBool("no-pull")
	return container.NewClient(dockerHost, !noPull)
}
