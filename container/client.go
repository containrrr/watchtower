package container

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/samalba/dockerclient"
)

const (
	defaultStopSignal = "SIGTERM"
	signalLabel       = "com.centurylinklabs.watchtower.stop-signal"
)

var (
	pullImages bool
)

func init() {
	pullImages = true
}

type ContainerFilter func(Container) bool

type Client interface {
	ListContainers(ContainerFilter) ([]Container, error)
	RefreshImage(*Container) error
	Stop(Container, time.Duration) error
	Start(Container) error
	Rename(Container, string) error
}

func NewClient(dockerHost string) Client {
	docker, err := dockerclient.NewDockerClient(dockerHost, nil)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s\n", err)
	}

	return DockerClient{api: docker}
}

type DockerClient struct {
	api dockerclient.Client
}

func (client DockerClient) ListContainers(fn ContainerFilter) ([]Container, error) {
	cs := []Container{}

	runningContainers, err := client.api.ListContainers(false, false, "")
	if err != nil {
		return nil, err
	}

	for _, runningContainer := range runningContainers {
		containerInfo, err := client.api.InspectContainer(runningContainer.Id)
		if err != nil {
			return nil, err
		}

		imageInfo, err := client.api.InspectImage(containerInfo.Image)
		if err != nil {
			return nil, err
		}

		c := Container{containerInfo: containerInfo, imageInfo: imageInfo}
		if fn(c) {
			cs = append(cs, Container{containerInfo: containerInfo, imageInfo: imageInfo})
		}
	}

	return cs, nil
}

func (client DockerClient) RefreshImage(c *Container) error {
	containerInfo := c.containerInfo
	oldImageInfo := c.imageInfo
	imageName := containerInfo.Config.Image

	if pullImages {
		if !strings.Contains(imageName, ":") {
			imageName = fmt.Sprintf("%s:latest", imageName)
		}

		log.Printf("Pulling %s for %s\n", imageName, c.Name())
		if err := client.api.PullImage(imageName, nil); err != nil {
			return err
		}
	}

	newImageInfo, err := client.api.InspectImage(imageName)
	if err != nil {
		return err
	}

	if newImageInfo.Id != oldImageInfo.Id {
		log.Printf("Found new %s image (%s)\n", imageName, newImageInfo.Id)
		c.Stale = true
	}

	return nil
}

func (client DockerClient) Stop(c Container, timeout time.Duration) error {
	signal := defaultStopSignal

	if sig, ok := c.containerInfo.Config.Labels[signalLabel]; ok {
		signal = sig
	}

	log.Printf("Stopping: %s\n", c.Name())

	if err := client.api.KillContainer(c.containerInfo.Id, signal); err != nil {
		return err
	}

	// Wait for container to exit, but proceed anyway after the timeout elapses
	client.waitForStop(c, timeout)

	return client.api.RemoveContainer(c.containerInfo.Id, true, false)
}

func (client DockerClient) Start(c Container) error {
	config := c.runtimeConfig()
	hostConfig := c.hostConfig()
	name := c.Name()

	if name == "" {
		log.Printf("Starting new container from %s", c.containerInfo.Config.Image)
	} else {
		log.Printf("Starting %s\n", name)
	}

	newContainerId, err := client.api.CreateContainer(config, name)
	if err != nil {
		return err
	}

	return client.api.StartContainer(newContainerId, hostConfig)
}

func (client DockerClient) Rename(c Container, newName string) error {
	return client.api.RenameContainer(c.containerInfo.Id, newName)
}

func (client DockerClient) waitForStop(c Container, waitTime time.Duration) error {
	timeout := time.After(waitTime)

	for {
		select {
		case <-timeout:
			return nil
		default:
			if ci, err := client.api.InspectContainer(c.containerInfo.Id); err != nil {
				return err
			} else if !ci.State.Running {
				return nil
			}

			time.Sleep(1 * time.Second)
		}
	}
}
