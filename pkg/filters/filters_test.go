package filters

import (
	"testing"

	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/stretchr/testify/assert"
)

func TestWatchtowerContainersFilter(t *testing.T) {
	container := new(mocks.FilterableContainer)

	container.On("IsWatchtower").Return(true)

	assert.True(t, WatchtowerContainersFilter(container))

	container.AssertExpectations(t)
}

func TestNoFilter(t *testing.T) {
	container := new(mocks.FilterableContainer)

	assert.True(t, NoFilter(container))

	container.AssertExpectations(t)
}

func TestFilterByNames(t *testing.T) {
	var names []string

	filter := FilterByNames(names, nil)
	assert.Nil(t, filter)

	names = append(names, "test")

	filter = FilterByNames(names, NoFilter)
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
	filter := FilterByEnableLabel(NoFilter)
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

func TestFilterByScope(t *testing.T) {
	var scope string
	scope = "testscope"

	filter := FilterByScope(scope, NoFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Scope").Return("testscope", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Scope").Return("nottestscope", true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Scope").Return("", false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByDisabledLabel(t *testing.T) {
	filter := FilterByDisabledLabel(NoFilter)
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

func TestFilterByImage(t *testing.T) {
	filterSingle := FilterByImage([]string{"registry"}, NoFilter)
	filterMultiple := FilterByImage([]string{"registry", "bla"}, NoFilter)
	assert.NotNil(t, filterSingle)
	assert.NotNil(t, filterMultiple)

	filterNil := FilterByImage(nil, NoFilter)
	assert.Same(t, filterNil, NoFilter)

	container := new(mocks.FilterableContainer)
	container.On("Image").Return("registry:2")
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Image").Return("registry:latest")
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Image").Return("abcdef1234")
	assert.False(t, filterSingle(container))
	assert.False(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Image").Return("bla:latest")
	assert.False(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

}

func TestBuildFilter(t *testing.T) {
	var names []string
	names = append(names, "test")

	filter, desc := BuildFilter(names, false, "")
	assert.Contains(t, desc, "test")

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

	filter, desc := BuildFilter(names, true, "")
	assert.Contains(t, desc, "using enable label")

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
