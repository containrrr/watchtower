package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/v2tec/watchtower/container/mocks"
)

func TestWatchtowerContainersFilter(t *testing.T) {
	container := new(mocks.FilterableContainer)

	container.On("IsWatchtower").Return(true)

	assert.True(t, WatchtowerContainersFilter(container))

	container.AssertExpectations(t)
}

func TestNoFilter(t *testing.T) {
	container := new(mocks.FilterableContainer)

	assert.True(t, noFilter(container))

	container.AssertExpectations(t)
}

func TestFilterByNames(t *testing.T) {
	var names []string

	filter := filterByNames(names, nil)
	assert.Nil(t, filter)

	names = append(names, "test")

	filter = filterByNames(names, noFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Name").Return("test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("NoTest")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByEnableLabel(t *testing.T) {
	filter := filterByEnableLabel(noFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Enabled").Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByDisabledLabel(t *testing.T) {
	filter := filterByDisabledLabel(noFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Enabled").Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilter(t *testing.T) {
	var names []string
	names = append(names, "test")

	filter := BuildFilter(names, false)

	container := new(mocks.FilterableContainer)
	container.On("Name").Return("Invalid")
	container.On("Enabled").Return(false, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("test")
	container.On("Enabled").Return(false, false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("Invalid")
	container.On("Enabled").Return(true, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("test")
	container.On("Enabled").Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterEnableLabel(t *testing.T) {
	var names []string
	names = append(names, "test")

	filter := BuildFilter(names, true)

	container := new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("Invalid")
	container.On("Enabled").Twice().Return(true, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("test")
	container.On("Enabled").Twice().Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}
