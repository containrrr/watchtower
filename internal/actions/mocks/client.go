package mocks

import (
	"errors"
	"fmt"
	"time"

	"github.com/containrrr/watchtower/pkg/container"

	t "github.com/containrrr/watchtower/pkg/types"
)

// MockClient is a mock that passes as a watchtower Client
type MockClient struct {
	TestData      *TestData
	pullImages    bool
	removeVolumes bool
}

// TestData is the data used to perform the test
type TestData struct {
	TriedToRemoveImageCount int
	NameOfContainerToKeep   string
	Containers              []container.Container
	Staleness               map[string]bool
}

// TriedToRemoveImage is a test helper function to check whether RemoveImageByID has been called
func (testdata *TestData) TriedToRemoveImage() bool {
	return testdata.TriedToRemoveImageCount > 0
}

// CreateMockClient creates a mock watchtower Client for usage in tests
func CreateMockClient(data *TestData, pullImages bool, removeVolumes bool) MockClient {
	return MockClient{
		data,
		pullImages,
		removeVolumes,
	}
}

// ListContainers is a mock method returning the provided container testdata
func (client MockClient) ListContainers(_ t.Filter) ([]container.Container, error) {
	return client.TestData.Containers, nil
}

// StopContainer is a mock method
func (client MockClient) StopContainer(c container.Container, _ time.Duration) error {
	if c.Name() == client.TestData.NameOfContainerToKeep {
		return errors.New("tried to stop the instance we want to keep")
	}
	return nil
}

// StartContainer is a mock method
func (client MockClient) StartContainer(_ container.Container) (t.ContainerID, error) {
	return "", nil
}

// RenameContainer is a mock method
func (client MockClient) RenameContainer(_ container.Container, _ string) error {
	return nil
}

// RemoveImageByID increments the TriedToRemoveImageCount on being called
func (client MockClient) RemoveImageByID(_ t.ImageID) error {
	client.TestData.TriedToRemoveImageCount++
	return nil
}

// GetContainer is a mock method
func (client MockClient) GetContainer(_ t.ContainerID) (container.Container, error) {
	return client.TestData.Containers[0], nil
}

// ExecuteCommand is a mock method
func (client MockClient) ExecuteCommand(_ t.ContainerID, command string, _ int) (SkipUpdate bool, err error) {
	switch command {
	case "/PreUpdateReturn0.sh":
		return false, nil
	case "/PreUpdateReturn1.sh":
		return false, fmt.Errorf("command exited with code 1")
	case "/PreUpdateReturn75.sh":
		return true, nil
	default:
		return false, nil
	}
}

// IsContainerStale is true if not explicitly stated in TestData for the mock client
func (client MockClient) IsContainerStale(cont container.Container) (bool, t.ImageID, error) {
	stale, found := client.TestData.Staleness[cont.Name()]
	if !found {
		stale = true
	}
	return stale, "", nil
}

// WarnOnHeadPullFailed is always true for the mock client
func (client MockClient) WarnOnHeadPullFailed(_ container.Container) bool {
	return true
}
