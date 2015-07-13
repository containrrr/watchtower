package updater

import (
	"log"

	"github.com/samalba/dockerclient"
)

var (
	client dockerclient.Client
)

func init() {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Fatalf("Error instantiating Docker client: %s\n", err)
	}

	client = docker
}

func Run() error {
	containers, _ := client.ListContainers(false, false, "")

	for _, container := range containers {

		oldContainerInfo, _ := client.InspectContainer(container.Id)
		name := oldContainerInfo.Name
		oldImageId := oldContainerInfo.Image
		log.Printf("Running: %s (%s)\n", container.Image, oldImageId)

		oldImageInfo, _ := client.InspectImage(oldImageId)

		// First check to see if a newer image has already been built
		newImageInfo, _ := client.InspectImage(container.Image)

		if newImageInfo.Id == oldImageInfo.Id {
			_ = client.PullImage(container.Image, nil)
			newImageInfo, _ = client.InspectImage(container.Image)
		}

		newImageId := newImageInfo.Id
		log.Printf("Latest:  %s (%s)\n", container.Image, newImageId)

		if newImageId != oldImageId {
			log.Printf("Restarting %s with new image\n", name)
			if err := stopContainer(oldContainerInfo); err != nil {
			}

			config := GenerateContainerConfig(oldContainerInfo, oldImageInfo.Config)

			hostConfig := oldContainerInfo.HostConfig
			_ = startContainer(name, config, hostConfig)
		}
	}

	return nil
}

func stopContainer(container *dockerclient.ContainerInfo) error {
	signal := "SIGTERM"

	if sig, ok := container.Config.Labels["com.centurylinklabs.watchtower.stop-signal"]; ok {
		signal = sig
	}

	if err := client.KillContainer(container.Id, signal); err != nil {
		return err
	}

	return client.RemoveContainer(container.Id, true, false)
}

func startContainer(name string, config *dockerclient.ContainerConfig, hostConfig *dockerclient.HostConfig) error {
	newContainerId, err := client.CreateContainer(config, name)
	if err != nil {
		return err
	}

	return client.StartContainer(newContainerId, hostConfig)
}
