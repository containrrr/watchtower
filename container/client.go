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

type Filter func(Container) bool

type Client interface {
	ListContainers(Filter) ([]Container, error)
	StopContainer(Container, time.Duration) error
	StartContainer(Container) error
	RenameContainer(Container, string) error
	IsContainerStale(Container) (bool, error)
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

func (client DockerClient) ListContainers(fn Filter) ([]Container, error) {
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
			cs = append(cs, c)
		}
	}

	return cs, nil
}

func (client DockerClient) StopContainer(c Container, timeout time.Duration) error {
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

func (client DockerClient) StartContainer(c Container) error {
	config := c.runtimeConfig()
	hostConfig := c.hostConfig()
	name := c.Name()

	if name == "" {
		log.Printf("Starting new container from %s", c.containerInfo.Config.Image)
	} else {
		log.Printf("Starting %s\n", name)
	}

	newContainerID, err := client.api.CreateContainer(config, name)
	if err != nil {
		return err
	}

	return client.api.StartContainer(newContainerID, hostConfig)
}

func (client DockerClient) RenameContainer(c Container, newName string) error {
	return client.api.RenameContainer(c.containerInfo.Id, newName)
}

func (client DockerClient) IsContainerStale(c Container) (bool, error) {
	containerInfo := c.containerInfo
	oldImageInfo := c.imageInfo
	imageName := containerInfo.Config.Image

	if pullImages {
		if !strings.Contains(imageName, ":") {
			imageName = fmt.Sprintf("%s:latest", imageName)
		}

		log.Printf("Pulling %s for %s\n", imageName, c.Name())
		if err := client.api.PullImage(imageName, nil); err != nil {
			return false, err
		}
	}

	newImageInfo, err := client.api.InspectImage(imageName)
	if err != nil {
		return false, err
	}

	if newImageInfo.Id != oldImageInfo.Id {
		log.Printf("Found new %s image (%s)\n", imageName, newImageInfo.Id)
		return true, nil
	}

	return false, nil
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
