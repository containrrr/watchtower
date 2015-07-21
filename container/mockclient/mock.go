package mockclient

import (
	"time"

	"github.com/CenturyLinkLabs/watchtower/container"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) ListContainers(cf container.ContainerFilter) ([]container.Container, error) {
	args := m.Called(cf)
	return args.Get(0).([]container.Container), args.Error(1)
}

func (m *MockClient) RefreshImage(c *container.Container) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *MockClient) Stop(c container.Container, timeout time.Duration) error {
	args := m.Called(c, timeout)
	return args.Error(0)
}

func (m *MockClient) Start(c container.Container) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *MockClient) Rename(c container.Container, name string) error {
	args := m.Called(c, name)
	return args.Error(0)
}
