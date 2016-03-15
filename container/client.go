package container

import (
	"crypto/tls"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"
)

const (
	defaultStopSignal = "SIGTERM"
)

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(Container) bool

// A Client is the interface through which watchtower interacts with the
// Docker API.
type Client interface {
	ListContainers(Filter) ([]Container, error)
	StopContainer(Container, time.Duration) error
	StartContainer(Container) error
	RenameContainer(Container, string) error
	IsContainerStale(Container) (bool, error)
	RemoveImage(Container) error
}

// NewClient returns a new Client instance which can be used to interact with
// the Docker API.
func NewClient(dockerHost string, tlsConfig *tls.Config, pullImages bool) Client {
	docker, err := dockerclient.NewDockerClient(dockerHost, tlsConfig)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s", err)
	}

	return dockerClient{api: docker, pullImages: pullImages}
}

type dockerClient struct {
	api        dockerclient.Client
	pullImages bool
}

func (client dockerClient) ListContainers(fn Filter) ([]Container, error) {
	cs := []Container{}

	log.Debug("Retrieving running containers")

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

func (client dockerClient) StopContainer(c Container, timeout time.Duration) error {
	signal := c.StopSignal()
	if signal == "" {
		signal = defaultStopSignal
	}

	log.Infof("Stopping %s (%s) with %s", c.Name(), c.ID(), signal)

	if err := client.api.KillContainer(c.ID(), signal); err != nil {
		return err
	}

	// Wait for container to exit, but proceed anyway after the timeout elapses
	client.waitForStop(c, timeout)

	log.Debugf("Removing container %s", c.ID())

	if err := client.api.RemoveContainer(c.ID(), true, false); err != nil {
		return err
	}

	// Wait for container to be removed. In this case an error is a good thing
	if err := client.waitForStop(c, timeout); err == nil {
		return fmt.Errorf("Container %s (%s) could not be removed", c.Name(), c.ID())
	}

	return nil
}

func (client dockerClient) StartContainer(c Container) error {
	config := c.runtimeConfig()
	config.HostConfig = *c.hostConfig()
	name := c.Name()

	log.Infof("Starting %s", name)

	newContainerID, err := client.api.CreateContainer(config, name, nil)
	if err != nil {
		return err
	}

	log.Debugf("Starting container %s (%s)", name, newContainerID)

	return client.api.StartContainer(newContainerID, nil)
}

func (client dockerClient) RenameContainer(c Container, newName string) error {
	log.Debugf("Renaming container %s (%s) to %s", c.Name(), c.ID(), newName)
	return client.api.RenameContainer(c.ID(), newName)
}

func (client dockerClient) IsContainerStale(c Container) (bool, error) {
	oldImageInfo := c.imageInfo
	imageName := c.ImageName()

	if client.pullImages {
		log.Debugf("Pulling %s for %s", imageName, c.Name())
		if err := client.api.PullImage(imageName, nil); err != nil {
			return false, err
		}
	}

	newImageInfo, err := client.api.InspectImage(imageName)
	if err != nil {
		return false, err
	}

	if newImageInfo.Id != oldImageInfo.Id {
		log.Infof("Found new %s image (%s)", imageName, newImageInfo.Id)
		return true, nil
	}

	return false, nil
}

func (client dockerClient) RemoveImage(c Container) error {
	imageID := c.ImageID()
	log.Infof("Removing image %s", imageID)
	_, err := client.api.RemoveImage(imageID, false)
	return err
}

func (client dockerClient) waitForStop(c Container, waitTime time.Duration) error {
	timeout := time.After(waitTime)

	for {
		select {
		case <-timeout:
			return nil
		default:
			if ci, err := client.api.InspectContainer(c.ID()); err != nil {
				return err
			} else if !ci.State.Running {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}
}
