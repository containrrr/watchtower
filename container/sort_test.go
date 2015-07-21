package container

import (
	"sort"
	"testing"

	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
)

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

func TestSortByDependencies_Success(t *testing.T) {
	c1 := newTestContainer("1", []string{})
	c2 := newTestContainer("2", []string{"1:"})
	c3 := newTestContainer("3", []string{"2:"})
	c4 := newTestContainer("4", []string{"3:"})
	c5 := newTestContainer("5", []string{"4:"})
	c6 := newTestContainer("6", []string{"5:", "3:"})
	containers := []Container{c6, c2, c4, c1, c3, c5}

	result, err := SortByDependencies(containers)

	assert.NoError(t, err)
	assert.Equal(t, []Container{c1, c2, c3, c4, c5, c6}, result)
}

func TestSortByDependencies_Error(t *testing.T) {
	c1 := newTestContainer("1", []string{"3:"})
	c2 := newTestContainer("2", []string{"1:"})
	c3 := newTestContainer("3", []string{"2:"})
	containers := []Container{c1, c2, c3}

	_, err := SortByDependencies(containers)

	assert.Error(t, err)
	assert.EqualError(t, err, "Circular reference to 1")
}

func newTestContainer(name string, links []string) Container {
	return *NewContainer(
		&dockerclient.ContainerInfo{
			Name: name,
			HostConfig: &dockerclient.HostConfig{
				Links: links,
			},
		},
		nil,
	)
}
