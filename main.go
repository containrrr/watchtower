package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/CenturyLinkLabs/watchtower/updater"
	"github.com/codegangsta/cli"
)

var (
	wg sync.WaitGroup
)

func main() {
	app := cli.NewApp()
	app.Name = "watchtower"
	app.Usage = "Automatically update running Docker containers"
	app.Action = start
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "interval, i",
			Value: 300,
			Usage: "poll interval (in seconds)",
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

func start(c *cli.Context) {
	secs := time.Duration(c.Int("interval")) * time.Second

	for {
		wg.Add(1)
		updater.Run()
		wg.Done()

		time.Sleep(secs)
	}
}
