package actions

import (
	"regexp"
	"testing"

	"github.com/CenturyLinkLabs/watchtower/container"
	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestContainerFilter_StraightMatch(t *testing.T) {
	c := newTestContainer("foo", []string{})
	f := containerFilter([]string{"foo"})
	assert.True(t, f(c))
}

func TestContainerFilter_SlashMatch(t *testing.T) {
	c := newTestContainer("/foo", []string{})
	f := containerFilter([]string{"foo"})
	assert.True(t, f(c))
}

func TestContainerFilter_NoMatch(t *testing.T) {
	c := newTestContainer("/bar", []string{})
	f := containerFilter([]string{"foo"})
	assert.False(t, f(c))
}

func TestContainerFilter_NoFilters(t *testing.T) {
	c := newTestContainer("/bar", []string{})
	f := containerFilter([]string{})
	assert.True(t, f(c))
}

func TestCheckDependencies(t *testing.T) {
	cs := []container.Container{
		newTestContainer("1", []string{}),
		newTestContainer("2", []string{"1:"}),
		newTestContainer("3", []string{"2:"}),
		newTestContainer("4", []string{"3:"}),
		newTestContainer("5", []string{"4:"}),
		newTestContainer("6", []string{"5:"}),
	}
	cs[3].Stale = true

	checkDependencies(cs)

	assert.False(t, cs[0].Stale)
	assert.False(t, cs[1].Stale)
	assert.False(t, cs[2].Stale)
	assert.True(t, cs[3].Stale)
	assert.True(t, cs[4].Stale)
	assert.True(t, cs[5].Stale)
}

func TestRandName(t *testing.T) {
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

	name := randName()

	assert.True(t, validPattern.MatchString(name))
}

func newTestContainer(name string, links []string) container.Container {
	return *container.NewContainer(
		&dockerclient.ContainerInfo{
			Name: name,
			HostConfig: &dockerclient.HostConfig{
				Links: links,
			},
		},
		nil,
	)
}
