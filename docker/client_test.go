package docker

import (
	"errors"
	"testing"

	"github.com/samalba/dockerclient"
	"github.com/samalba/dockerclient/mockclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListContainers_Success(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123"}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo"}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, nil)

	client := DockerClient{api: api}
	cs, err := client.ListContainers()

	assert.NoError(t, err)
	assert.Len(t, cs, 1)
	assert.Equal(t, ci, cs[0].containerInfo)
	assert.Equal(t, ii, cs[0].imageInfo)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{}, errors.New("oops"))

	client := DockerClient{api: api}
	_, err := client.ListContainers()

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo"}}, nil)
	api.On("InspectContainer", "foo").Return(&dockerclient.ContainerInfo{}, errors.New("uh-oh"))

	client := DockerClient{api: api}
	_, err := client.ListContainers()

	assert.Error(t, err)
	assert.EqualError(t, err, "uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123"}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo"}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, errors.New("whoops"))

	client := DockerClient{api: api}
	_, err := client.ListContainers()

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestRefreshImage_NotStaleSuccess(t *testing.T) {
	c := &Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Config: &dockerclient.ContainerConfig{Image: "bar"},
		},
		imageInfo: &dockerclient.ImageInfo{Id: "abc123"},
	}
	newImageInfo := &dockerclient.ImageInfo{Id: "abc123"}

	api := mockclient.NewMockClient()
	api.On("PullImage", "bar:latest", mock.Anything).Return(nil)
	api.On("InspectImage", "bar:latest").Return(newImageInfo, nil)

	client := DockerClient{api: api}
	err := client.RefreshImage(c)

	assert.NoError(t, err)
	assert.False(t, c.Stale)
	api.AssertExpectations(t)
}

func TestRefreshImage_StaleSuccess(t *testing.T) {
	c := &Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Config: &dockerclient.ContainerConfig{Image: "bar:1.0"},
		},
		imageInfo: &dockerclient.ImageInfo{Id: "abc123"},
	}
	newImageInfo := &dockerclient.ImageInfo{Id: "xyz789"}

	api := mockclient.NewMockClient()
	api.On("PullImage", "bar:1.0", mock.Anything).Return(nil)
	api.On("InspectImage", "bar:1.0").Return(newImageInfo, nil)

	client := DockerClient{api: api}
	err := client.RefreshImage(c)

	assert.NoError(t, err)
	assert.True(t, c.Stale)
	api.AssertExpectations(t)
}

func TestRefreshImage_PullImageError(t *testing.T) {
	c := &Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Config: &dockerclient.ContainerConfig{Image: "bar:latest"},
		},
		imageInfo: &dockerclient.ImageInfo{Id: "abc123"},
	}

	api := mockclient.NewMockClient()
	api.On("PullImage", "bar:latest", mock.Anything).Return(errors.New("oops"))

	client := DockerClient{api: api}
	err := client.RefreshImage(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestRefreshImage_InspectImageError(t *testing.T) {
	c := &Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Config: &dockerclient.ContainerConfig{Image: "bar:latest"},
		},
		imageInfo: &dockerclient.ImageInfo{Id: "abc123"},
	}
	newImageInfo := &dockerclient.ImageInfo{}

	api := mockclient.NewMockClient()
	api.On("PullImage", "bar:latest", mock.Anything).Return(nil)
	api.On("InspectImage", "bar:latest").Return(newImageInfo, errors.New("uh-oh"))

	client := DockerClient{api: api}
	err := client.RefreshImage(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "uh-oh")
	api.AssertExpectations(t)
}

func TestStop_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil)
	api.On("RemoveContainer", "abc123", true, false).Return(nil)

	client := DockerClient{api: api}
	err := client.Stop(c)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStop_CustomSignalSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name: "foo",
			Id:   "abc123",
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.centurylinklabs.watchtower.stop-signal": "SIGUSR1"}},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGUSR1").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil)
	api.On("RemoveContainer", "abc123", true, false).Return(nil)

	client := DockerClient{api: api}
	err := client.Stop(c)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStop_KillContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(errors.New("oops"))

	client := DockerClient{api: api}
	err := client.Stop(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStop_RemoveContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("dangit"))
	api.On("RemoveContainer", "abc123", true, false).Return(errors.New("whoops"))

	client := DockerClient{api: api}
	err := client.Stop(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStart_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.AnythingOfType("*dockerclient.ContainerConfig"), "foo").Return("def789", nil)
	api.On("StartContainer", "def789", mock.AnythingOfType("*dockerclient.HostConfig")).Return(nil)

	client := DockerClient{api: api}
	err := client.Start(c)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStart_CreateContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo").Return("", errors.New("oops"))

	client := DockerClient{api: api}
	err := client.Start(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStart_StartContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo").Return("def789", nil)
	api.On("StartContainer", "def789", mock.Anything).Return(errors.New("whoops"))

	client := DockerClient{api: api}
	err := client.Start(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}
