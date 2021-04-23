package mocks

import (
	"errors"
	"github.com/containrrr/watchtower/pkg/container"
	"time"

	t "github.com/containrrr/watchtower/pkg/types"
	cli "github.com/docker/docker/client"
)

// MockClient is a mock that passes as a watchtower Client
type MockClient struct {
	TestData      *TestData
	api           cli.CommonAPIClient
	pullImages    bool
	removeVolumes bool
}

// TestData is the data used to perform the test
type TestData struct {
	TriedToRemoveImageCount int
	NameOfContainerToKeep   string
	Containers              []container.Container
}

// TriedToRemoveImage is a test helper function to check whether RemoveImageByID has been called
func (testdata *TestData) TriedToRemoveImage() bool {
	return testdata.TriedToRemoveImageCount > 0
}

// CreateMockClient creates a mock watchtower Client for usage in tests
func CreateMockClient(data *TestData, api cli.CommonAPIClient, pullImages bool, removeVolumes bool) MockClient {
	return MockClient{
		data,
		api,
		pullImages,
		removeVolumes,
	}
}

// ListContainers is a mock method returning the provided container testdata
func (client MockClient) ListContainers(f t.Filter) ([]container.Container, error) {
	return client.TestData.Containers, nil
}

// StopContainer is a mock method
func (client MockClient) StopContainer(c container.Container, d time.Duration) error {
	if c.Name() == client.TestData.NameOfContainerToKeep {
		return errors.New("tried to stop the instance we want to keep")
	}
	return nil
}

// StartContainer is a mock method
func (client MockClient) StartContainer(c container.Container) (string, error) {
	return "", nil
}

// RenameContainer is a mock method
func (client MockClient) RenameContainer(c container.Container, s string) error {
	return nil
}

// RemoveImageByID increments the TriedToRemoveImageCount on being called
func (client MockClient) RemoveImageByID(id string) error {
	client.TestData.TriedToRemoveImageCount++
	return nil
}

// GetContainer is a mock method
func (client MockClient) GetContainer(containerID string) (container.Container, error) {
	return container.Container{}, nil
}

// ExecuteCommand is a mock method
func (client MockClient) ExecuteCommand(containerID string, command string, timeout int) error {
	return nil
}

// IsContainerStale is always true for the mock client
func (client MockClient) IsContainerStale(c container.Container) (bool, error) {
	return true, nil
}

// WarnOnHeadPullFailed is always true for the mock client
func (client MockClient) WarnOnHeadPullFailed(c container.Container) bool {
	return true
}
