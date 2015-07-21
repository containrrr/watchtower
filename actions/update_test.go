package actions

import (
	"regexp"
	"testing"

	"github.com/CenturyLinkLabs/watchtower/container"
	"github.com/stretchr/testify/assert"
)

func TestCheckDependencies(t *testing.T) {
	cs := []container.Container{
		container.NewTestContainer("1", []string{}),
		container.NewTestContainer("2", []string{"1:"}),
		container.NewTestContainer("3", []string{"2:"}),
		container.NewTestContainer("4", []string{"3:"}),
		container.NewTestContainer("5", []string{"4:"}),
		container.NewTestContainer("6", []string{"5:"}),
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
