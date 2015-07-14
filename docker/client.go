package docker

import (
	"log"

	"github.com/samalba/dockerclient"
)

var (
	pullImages bool
)

func init() {
	pullImages = true
}

type Client interface {
	ListContainers() ([]Container, error)
	RefreshImage(container *Container) error
	Stop(container Container) error
	Start(container Container) error
}

func NewClient() Client {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)

	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s\n", err)
	}

	return DockerClient{api: docker}
}

type DockerClient struct {
	api dockerclient.Client
}

func (client DockerClient) ListContainers() ([]Container, error) {
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

		cs = append(cs, Container{containerInfo: containerInfo, imageInfo: imageInfo})
	}

	return cs, nil
}

func (client DockerClient) RefreshImage(c *Container) error {
	containerInfo := c.containerInfo
	oldImageInfo := c.imageInfo
	imageName := containerInfo.Config.Image

	if pullImages {
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

func (client DockerClient) Stop(c Container) error {
	signal := "SIGTERM"

	if sig, ok := c.containerInfo.Config.Labels["com.centurylinklabs.watchtower.stop-signal"]; ok {
		signal = sig
	}

	log.Printf("Stopping: %s\n", c.Name())

	if err := client.api.KillContainer(c.containerInfo.Id, signal); err != nil {
		return err
	}

	return client.api.RemoveContainer(c.containerInfo.Id, true, false)
}

func (client DockerClient) Start(c Container) error {
	config := c.runtimeConfig()
	hostConfig := c.hostConfig()

	log.Printf("Starting: %s\n", c.Name())

	newContainerId, err := client.api.CreateContainer(config, c.Name())
	if err != nil {
		return err
	}

	return client.api.StartContainer(newContainerId, hostConfig)
}
