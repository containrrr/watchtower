package mocks

import (
	"errors"
	"github.com/containrrr/watchtower/pkg/container"
	"time"

	t "github.com/containrrr/watchtower/pkg/types"
	cli "github.com/docker/docker/client"
)

type MockClient struct {
	TestData      *TestData
	api           cli.CommonAPIClient
	pullImages    bool
	removeVolumes bool
}

type TestData struct {
	TriedToRemoveImageCount int
	NameOfContainerToKeep string
	Containers            []container.Container
}

func (testdata *TestData) TriedToRemoveImage() bool {
	return testdata.TriedToRemoveImageCount > 0
}

func CreateMockClient(data *TestData, api cli.CommonAPIClient, pullImages bool, removeVolumes bool) MockClient {
	return MockClient {
		data,
		api,
		pullImages,
		removeVolumes,
	}
}

func (client MockClient) ListContainers(f t.Filter) ([]container.Container, error) {
	return client.TestData.Containers, nil
}

func (client MockClient) StopContainer(c container.Container, d time.Duration) error {
	if c.Name() == client.TestData.NameOfContainerToKeep {
		return errors.New("tried to stop the instance we want to keep")
	}
	return nil
}
func (client MockClient) StartContainer(c container.Container) (string, error) {
	return "", nil
}

func (client MockClient) RenameContainer(c container.Container, s string) error {
	return nil
}

func (client MockClient) RemoveImageByID(id string) error {
	client.TestData.TriedToRemoveImageCount++
	return nil
}

func (client MockClient) GetContainer(containerID string) (container.Container, error) {
	return container.Container{}, nil
}

func (client MockClient) ExecuteCommand(containerID string, command string) error {
	return nil
}

func (client MockClient) IsContainerStale(c container.Container) (bool, error) {
	return true, nil
}

