package main // import "github.com/CenturyLinkLabs/watchtower"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CenturyLinkLabs/watchtower/actions"
	"github.com/CenturyLinkLabs/watchtower/container"
	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
)

var (
	wg             sync.WaitGroup
	client         container.Client
	registryClient container.Client
	pollInterval   time.Duration
	cleanup        bool
)

func init() {
	log.SetLevel(log.InfoLevel)
}

func main() {
	rootCertPath := "/etc/ssl/docker"

	if os.Getenv("DOCKER_CERT_PATH") != "" {
		rootCertPath = os.Getenv("DOCKER_CERT_PATH")
	}

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
		cli.StringFlag{
			Name:   "registry",
			Usage:  "docker registry from which to pull images",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_REGISTRY",
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
			Name:  "tls",
			Usage: "use TLS; implied by --tlsverify",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:  "tlscacert",
			Usage: "trust certs signed only by this CA",
			Value: fmt.Sprintf("%s/ca.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlscert",
			Usage: "client certificate for TLS authentication",
			Value: fmt.Sprintf("%s/cert.pem", rootCertPath),
		},
		cli.StringFlag{
			Name:  "tlskey",
			Usage: "client key for TLS authentication",
			Value: fmt.Sprintf("%s/key.pem", rootCertPath),
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
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

	// Set-up container client
	_, err := tlsConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(c.GlobalString("host"), !c.GlobalBool("no-pull"))
	//login(c)

	handleSignals()
	return nil
}

func login(c *cli.Context) {
	registry := c.GlobalString("registry")
	if registry != "" && registry != c.GlobalString("host") {
		log.Debug("Logging into registry")
		clint, err := dockerclient.NewEnvClient()
		if err != nil {
			panic(err)
		}
		regAuth := types.AuthConfig{
			Username:      "testuser",
			Password:      "testpassword",
			ServerAddress: registry,
		}
		resp, err := clint.RegistryLogin(context.Background(), regAuth)
		if err != nil {
			panic(err)
		}
		log.Debugf("Authenticated client with registry %s, %s", registry, resp)
	}
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

// tlsConfig translates the command-line options into a tls.Config struct
func tlsConfig(c *cli.Context) (*tls.Config, error) {
	var tlsConfig *tls.Config
	var err error
	caCertFlag := c.GlobalString("tlscacert")
	certFlag := c.GlobalString("tlscert")
	keyFlag := c.GlobalString("tlskey")

	if c.GlobalBool("tls") || c.GlobalBool("tlsverify") {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !c.GlobalBool("tlsverify"),
		}

		// Load CA cert
		if caCertFlag != "" {
			var caCert []byte

			if strings.HasPrefix(caCertFlag, "/") {
				caCert, err = ioutil.ReadFile(caCertFlag)
				if err != nil {
					return nil, err
				}
			} else {
				caCert = []byte(caCertFlag)
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			tlsConfig.RootCAs = caCertPool
		}

		// Load client certificate
		if certFlag != "" && keyFlag != "" {
			var cert tls.Certificate

			if strings.HasPrefix(certFlag, "/") && strings.HasPrefix(keyFlag, "/") {
				cert, err = tls.LoadX509KeyPair(certFlag, keyFlag)
				if err != nil {
					return nil, err
				}
			} else {
				cert, err = tls.X509KeyPair([]byte(certFlag), []byte(keyFlag))
				if err != nil {
					return nil, err
				}
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return tlsConfig, nil
}
