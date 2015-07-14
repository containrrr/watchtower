package updater

import (
	"testing"

	"github.com/CenturyLinkLabs/watchtower/docker"
	"github.com/stretchr/testify/assert"
)

func TestCheckDependencies(t *testing.T) {
	cs := []docker.Container{
		docker.NewTestContainer("1", []string{}),
		docker.NewTestContainer("2", []string{"1:"}),
		docker.NewTestContainer("3", []string{"2:"}),
		docker.NewTestContainer("4", []string{"3:"}),
		docker.NewTestContainer("5", []string{"4:"}),
		docker.NewTestContainer("6", []string{"5:"}),
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
