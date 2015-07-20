package docker

import (
	"sort"
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

func TestByCreated(t *testing.T) {
	c1 := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Created: "2015-07-01T12:00:01.000000000Z",
		},
	}
	c2 := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Created: "2015-07-01T12:00:02.000000000Z",
		},
	}
	c3 := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Created: "2015-07-01T12:00:02.000000001Z",
		},
	}
	cs := []Container{c3, c2, c1}

	sort.Sort(ByCreated(cs))

	assert.Equal(t, []Container{c1, c2, c3}, cs)
}
