package updater

import (
	"testing"

	"github.com/CenturyLinkLabs/watchtower/docker"
	"github.com/stretchr/testify/assert"
)

func TestContainerSorter_Success(t *testing.T) {
	c1 := docker.NewTestContainer("1", []string{})
	c2 := docker.NewTestContainer("2", []string{"1:"})
	c3 := docker.NewTestContainer("3", []string{"2:"})
	c4 := docker.NewTestContainer("4", []string{"3:"})
	c5 := docker.NewTestContainer("5", []string{"4:"})
	c6 := docker.NewTestContainer("6", []string{"5:", "3:"})
	containers := []docker.Container{c6, c2, c4, c1, c3, c5}

	cs := ContainerSorter{}
	result, err := cs.Sort(containers)

	assert.NoError(t, err)
	assert.Equal(t, []docker.Container{c1, c2, c3, c4, c5, c6}, result)
}

func TestContainerSorter_Error(t *testing.T) {
	c1 := docker.NewTestContainer("1", []string{"3:"})
	c2 := docker.NewTestContainer("2", []string{"1:"})
	c3 := docker.NewTestContainer("3", []string{"2:"})
	containers := []docker.Container{c1, c2, c3}

	cs := ContainerSorter{}
	_, err := cs.Sort(containers)

	assert.Error(t, err)
	assert.EqualError(t, err, "Circular reference to 1")
}
