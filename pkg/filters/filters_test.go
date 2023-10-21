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

func TestFilterByNamesRegex(t *testing.T) {
	names := []string{`ba(b|ll)oon`}

	filter := FilterByNames(names, NoFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Name").Return("balloon")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("spoon")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("baboonious")
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
	scope := "testscope"

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

func TestFilterByNoneScope(t *testing.T) {
	scope := "none"

	filter := FilterByScope(scope, NoFilter)
	assert.NotNil(t, filter)

	container := new(mocks.FilterableContainer)
	container.On("Scope").Return("anyscope", true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Scope").Return("", false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Scope").Return("", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Scope").Return("none", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterNoneScope(t *testing.T) {
	filter, desc := BuildFilter(nil, nil, false, "none")

	assert.Contains(t, desc, "without a scope")

	scoped := new(mocks.FilterableContainer)
	scoped.On("Enabled").Return(false, false)
	scoped.On("Scope").Return("anyscope", true)

	unscoped := new(mocks.FilterableContainer)
	unscoped.On("Enabled").Return(false, false)
	unscoped.On("Scope").Return("", false)

	assert.False(t, filter(scoped))
	assert.True(t, filter(unscoped))

	scoped.AssertExpectations(t)
	unscoped.AssertExpectations(t)
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
	filterEmpty := FilterByImage(nil, NoFilter)
	filterSingle := FilterByImage([]string{"registry"}, NoFilter)
	filterMultiple := FilterByImage([]string{"registry", "bla"}, NoFilter)
	assert.NotNil(t, filterSingle)
	assert.NotNil(t, filterMultiple)

	container := new(mocks.FilterableContainer)
	container.On("ImageName").Return("registry:2")
	assert.True(t, filterEmpty(container))
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("ImageName").Return("registry:latest")
	assert.True(t, filterEmpty(container))
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("ImageName").Return("abcdef1234")
	assert.True(t, filterEmpty(container))
	assert.False(t, filterSingle(container))
	assert.False(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("ImageName").Return("bla:latest")
	assert.True(t, filterEmpty(container))
	assert.False(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

}

func TestBuildFilter(t *testing.T) {
	names := []string{"test", "valid"}

	filter, desc := BuildFilter(names, []string{}, false, "")
	assert.Contains(t, desc, "test")
	assert.Contains(t, desc, "or")
	assert.Contains(t, desc, "valid")

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

	filter, desc := BuildFilter(names, []string{}, true, "")
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

func TestBuildFilterDisableContainer(t *testing.T) {
	filter, desc := BuildFilter([]string{}, []string{"excluded", "notfound"}, false, "")
	assert.Contains(t, desc, "not named")
	assert.Contains(t, desc, "excluded")
	assert.Contains(t, desc, "or")
	assert.Contains(t, desc, "notfound")

	container := new(mocks.FilterableContainer)
	container.On("Name").Return("Another")
	container.On("Enabled").Return(false, false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("AnotherOne")
	container.On("Enabled").Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("test")
	container.On("Enabled").Return(false, false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("excluded")
	container.On("Enabled").Return(true, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("excludedAsSubstring")
	container.On("Enabled").Return(true, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Name").Return("notfound")
	container.On("Enabled").Return(true, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("Enabled").Return(false, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}
