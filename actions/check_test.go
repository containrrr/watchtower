package actions

import (
	"errors"
	"testing"
	"time"

	"github.com/CenturyLinkLabs/watchtower/container"
	"github.com/CenturyLinkLabs/watchtower/container/mockclient"
	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCheckPrereqs_Success(t *testing.T) {
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
	}
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:    "c1",
			Config:  cc,
			Created: "2015-07-01T12:00:01.000000000Z",
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:    "c2",
			Config:  cc,
			Created: "2015-07-01T12:00:00.000000000Z",
		},
		nil,
	)
	cs := []container.Container{c1, c2}

	client := &mockclient.MockClient{}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("StopContainer", c2, time.Duration(60)).Return(nil)

	err := CheckPrereqs(client, false)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestCheckPrereqs_WithCleanup(t *testing.T) {
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
	}
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:    "c1",
			Config:  cc,
			Created: "2015-07-01T12:00:01.000000000Z",
		},
		nil,
	)
	c2 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:    "c2",
			Config:  cc,
			Created: "2015-07-01T12:00:00.000000000Z",
		},
		nil,
	)
	cs := []container.Container{c1, c2}

	client := &mockclient.MockClient{}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)
	client.On("StopContainer", c2, time.Duration(60)).Return(nil)
	client.On("RemoveImage", c2).Return(nil)

	err := CheckPrereqs(client, true)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestCheckPrereqs_OnlyOneContainer(t *testing.T) {
	cc := &dockerclient.ContainerConfig{
		Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
	}
	c1 := *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name:    "c1",
			Config:  cc,
			Created: "2015-07-01T12:00:01.000000000Z",
		},
		nil,
	)
	cs := []container.Container{c1}

	client := &mockclient.MockClient{}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, nil)

	err := CheckPrereqs(client, false)

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestCheckPrereqs_ListError(t *testing.T) {
	cs := []container.Container{}

	client := &mockclient.MockClient{}
	client.On("ListContainers", mock.AnythingOfType("container.Filter")).Return(cs, errors.New("oops"))

	err := CheckPrereqs(client, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	client.AssertExpectations(t)
}
