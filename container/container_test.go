package container

import (
	"testing"

	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{Name: "foo"},
	}

	name := c.Name()

	assert.Equal(t, "foo", name)
}

func TestLinks(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			HostConfig: &dockerclient.HostConfig{
				Links: []string{"foo:foo", "bar:bar"},
			},
		},
	}

	links := c.Links()

	assert.Equal(t, []string{"foo", "bar"}, links)
}

func TestIsWatchtower_True(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			},
		},
	}

	assert.True(t, c.IsWatchtower())
}

func TestIsWatchtower_WrongLabelValue(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.centurylinklabs.watchtower": "false"},
			},
		},
	}

	assert.False(t, c.IsWatchtower())
}

func TestIsWatchtower_NoLabel(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{},
			},
		},
	}

	assert.False(t, c.IsWatchtower())
}
