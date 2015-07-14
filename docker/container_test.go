package docker

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
