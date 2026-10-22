package filters

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
)

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

func TestExcludeOldWatchtowerFilter(t *testing.T) {
	t.Parallel()

	// Test non-Watchtower container (should pass)
	nonWatchtower := new(mockContainer.FilterableContainer)
	nonWatchtower.On("IsWatchtower").Return(false)
	nonWatchtower.On("Name").Return("/regular-app").Maybe()
	assert.True(t, excludeOldWatchtowerFilter(nonWatchtower))
	nonWatchtower.AssertExpectations(t)

	// Test regular Watchtower container (should pass)
	watchtower := new(mockContainer.FilterableContainer)
	watchtower.On("IsWatchtower").Return(true)
	watchtower.On("Name").Return("/watchtower")
	assert.True(t, excludeOldWatchtowerFilter(watchtower))
	watchtower.AssertExpectations(t)

	// Test old Watchtower container (should be excluded)
	oldContainer := new(mockContainer.FilterableContainer)
	oldContainer.On("IsWatchtower").Return(true)
	oldContainer.On("Name").Return("/watchtower-old-abc123")
	assert.False(t, excludeOldWatchtowerFilter(oldContainer))
	oldContainer.AssertExpectations(t)

	// Test old Watchtower container without leading slash (should be excluded)
	oldContainerNoSlash := new(mockContainer.FilterableContainer)
	oldContainerNoSlash.On("IsWatchtower").Return(true)
	oldContainerNoSlash.On("Name").Return("watchtower-old-def456")
	assert.False(t, excludeOldWatchtowerFilter(oldContainerNoSlash))
	oldContainerNoSlash.AssertExpectations(t)

	// Test container with similar but different prefix (should pass)
	similarPrefix := new(mockContainer.FilterableContainer)
	similarPrefix.On("IsWatchtower").Return(true)
	similarPrefix.On("Name").Return("/watchtower-oldinstance")
	assert.True(t, excludeOldWatchtowerFilter(similarPrefix))
	similarPrefix.AssertExpectations(t)
}

func TestIsOldWatchtower(t *testing.T) {
	t.Parallel()

	nonWatchtower := new(mockContainer.FilterableContainer)
	nonWatchtower.On("IsWatchtower").Return(false)
	nonWatchtower.On("Name").Return("/regular-app").Maybe()
	assert.False(t, IsOldWatchtower(nonWatchtower))
	nonWatchtower.AssertExpectations(t)

	watchtower := new(mockContainer.FilterableContainer)
	watchtower.On("IsWatchtower").Return(true)
	watchtower.On("Name").Return("/watchtower")
	assert.False(t, IsOldWatchtower(watchtower))
	watchtower.AssertExpectations(t)

	oldContainer := new(mockContainer.FilterableContainer)
	oldContainer.On("IsWatchtower").Return(true)
	oldContainer.On("Name").Return("/watchtower-old-abc123")
	assert.True(t, IsOldWatchtower(oldContainer))
	oldContainer.AssertExpectations(t)

	oldContainerNoSlash := new(mockContainer.FilterableContainer)
	oldContainerNoSlash.On("IsWatchtower").Return(true)
	oldContainerNoSlash.On("Name").Return("watchtower-old-def456")
	assert.True(t, IsOldWatchtower(oldContainerNoSlash))
	oldContainerNoSlash.AssertExpectations(t)

	similarPrefix := new(mockContainer.FilterableContainer)
	similarPrefix.On("IsWatchtower").Return(true)
	similarPrefix.On("Name").Return("/watchtower-oldinstance")
	assert.False(t, IsOldWatchtower(similarPrefix))
	similarPrefix.AssertExpectations(t)
}

func TestWatchtowerContainersFilter(t *testing.T) {
	t.Parallel()

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("test").Maybe()
	container.On("IsWatchtower").Return(true)
	assert.True(t, WatchtowerContainersFilter(container))
	container.AssertExpectations(t)
}

func TestUnscopedWatchtowerContainersFilter(t *testing.T) {
	t.Parallel()

	// Test unscoped Watchtower container (should pass)
	unscoped := new(mockContainer.FilterableContainer)
	unscoped.On("IsWatchtower").Return(true)
	unscoped.On("Scope").Return("", false) // No scope set
	unscoped.On("Name").Return("/unscoped-watchtower").Maybe()
	assert.True(t, UnscopedWatchtowerContainersFilter(unscoped))
	unscoped.AssertExpectations(t)

	// Test explicitly scoped Watchtower container (should fail)
	scoped := new(mockContainer.FilterableContainer)
	scoped.On("IsWatchtower").Return(true)
	scoped.On("Scope").Return("prod", true) // Has scope
	scoped.On("Name").Return("/scoped-watchtower").Maybe()
	assert.False(t, UnscopedWatchtowerContainersFilter(scoped))
	scoped.AssertExpectations(t)

	// Test none-scoped Watchtower container (should pass)
	noneScoped := new(mockContainer.FilterableContainer)
	noneScoped.On("IsWatchtower").Return(true)
	noneScoped.On("Scope").Return("none", true) // Explicitly none scope
	noneScoped.On("Name").Return("/none-scoped-watchtower").Maybe()
	assert.True(t, UnscopedWatchtowerContainersFilter(noneScoped))
	noneScoped.AssertExpectations(t)

	// Test non-Watchtower container (should fail)
	nonWatchtower := new(mockContainer.FilterableContainer)
	nonWatchtower.On("IsWatchtower").Return(false)
	nonWatchtower.On("Name").Return("/regular-app").Maybe()
	assert.False(t, UnscopedWatchtowerContainersFilter(nonWatchtower))
	nonWatchtower.AssertExpectations(t)
}

func TestNoFilter(t *testing.T) {
	t.Parallel()

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("test").Maybe()
	assert.True(t, NoFilter(container))
	container.AssertExpectations(t)
}

func TestFilterByNames(t *testing.T) {
	t.Parallel()

	names := make([]string, 0, 1)

	filter := FilterByNames(newTestLogger(t), names, nil)
	assert.Nil(t, filter)

	names = append(names, "test")
	filter = FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("NoTest")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByNamesLeadingSlashScenarios(t *testing.T) {
	t.Parallel()

	// Test container with normalized name matching filter without slash
	names := []string{"test"}
	filter := FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Test container with normalized name matching filter with slash
	names = []string{"/test"}
	filter = FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Test multiple containers with normalized filter inputs
	names = []string{"container1", "container2", "container3"}
	filter = FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	// Should match container1
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container1")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Should match container2
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container2")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Should match container3
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container3")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Test multiple containers with leading slash in filter inputs
	names = []string{"/container1", "/container2", "/container3"}
	filter = FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	// Should match /container1
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container1")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Should match /container2
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container2")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Should match /container3
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container3")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Should not match non-matching container
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container4")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByNamesRegex(t *testing.T) {
	t.Parallel()

	names := []string{`ba(b|ll)oon`}

	filter := FilterByNames(newTestLogger(t), names, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("balloon")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("spoon")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("baboonious")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByEnabledLabelsPresenceCheck(t *testing.T) {
	t.Parallel()

	filter := filterByEnabledLabels(newTestLogger(t), map[string]string{enableLabelKey: ""}, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("true", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("false", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("", false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByScope(t *testing.T) {
	t.Parallel()

	scope := "testscope"

	filter := FilterByScope(newTestLogger(t), scope, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Scope").Return("testscope", true)
	container.On("Name").Return("/test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Scope").Return("nottestscope", true)
	container.On("Name").Return("/test")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Scope").Return("", false)
	container.On("Name").Return("/test")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByNoneScope(t *testing.T) {
	t.Parallel()

	scope := "none"

	filter := FilterByScope(newTestLogger(t), scope, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Scope").Return("anyscope", true)
	container.On("Name").Return("/test")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Scope").Return("", false)
	container.On("Name").Return("/test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Scope").Return("", true)
	container.On("Name").Return("/test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Scope").Return("none", true)
	container.On("Name").Return("/test")
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByNoneScope_TransitionScenarios(t *testing.T) {
	t.Parallel()

	filter := FilterByScope(newTestLogger(t), "none", NoFilter)
	assert.NotNil(t, filter)

	tests := []struct {
		name           string
		containerScope string
		scopeExists    bool
		expected       bool
		description    string
	}{
		{
			name:           "explicit_none_scope",
			containerScope: "none",
			scopeExists:    true,
			expected:       true,
			description:    "container with explicit 'none' scope should match",
		},
		{
			name:           "implicit_unscoped",
			containerScope: "",
			scopeExists:    false,
			expected:       true,
			description:    "container with no scope label should default to 'none' and match",
		},
		{
			name:           "explicit_empty_scope",
			containerScope: "",
			scopeExists:    true,
			expected:       true,
			description:    "container with explicitly empty scope label should default to 'none' and match",
		},
		{
			name:           "transition_from_scoped",
			containerScope: "production",
			scopeExists:    true,
			expected:       false,
			description:    "container transitioning from scoped to none should not match when scope is set",
		},
		{
			name:           "scoped_container",
			containerScope: "dev",
			scopeExists:    true,
			expected:       false,
			description:    "regular scoped container should not match none filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			container := new(mockContainer.FilterableContainer)
			container.On("Scope").Return(tt.containerScope, tt.scopeExists)
			container.On("Name").Return("/test-container")

			result := filter(container)
			assert.Equal(t, tt.expected, result, tt.description)
			container.AssertExpectations(t)
		})
	}
}

func TestFilterByNoneScope_MixedHandling(t *testing.T) {
	t.Parallel()

	filter := FilterByScope(newTestLogger(t), "none", NoFilter)
	assert.NotNil(t, filter)

	// Test mixed batch of containers with different scope configurations
	containers := []struct {
		name           string
		containerScope string
		scopeExists    bool
		expected       bool
	}{
		{"explicit-none", "none", true, true},
		{"implicit-unscoped", "", false, true},
		{"explicit-empty", "", true, true},
		{"scoped-prod", "prod", true, false},
		{"scoped-dev", "dev", true, false},
		{"scoped-empty-transition", "", true, true}, // empty scope still defaults to none
	}

	results := make([]bool, len(containers))
	for i, tc := range containers {
		container := new(mockContainer.FilterableContainer)
		container.On("Scope").Return(tc.containerScope, tc.scopeExists)
		container.On("Name").Return("/" + tc.name)
		results[i] = filter(container)
		container.AssertExpectations(t)
	}

	// Verify results
	expectedResults := make([]bool, len(containers))
	for i, tc := range containers {
		expectedResults[i] = tc.expected
	}

	assert.Equal(
		t,
		expectedResults,
		results,
		"mixed handling of explicitly 'none'-scoped and truly unscoped containers",
	)
}

func TestBuildFilterNoneScope(t *testing.T) {
	t.Parallel()

	filter, desc, err := BuildFilter(newTestLogger(t), nil, nil, nil, nil, nil, nil, false, "none")
	require.NoError(t, err)

	assert.Contains(t, desc, "without a scope")

	scoped := new(mockContainer.FilterableContainer)
	scoped.On("IsWatchtower").Return(false).Maybe()
	scoped.On("Enabled").Return(false, false).Maybe()
	scoped.On("Scope").Return("anyscope", true)
	scoped.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	scoped.On("Name").Return("/scoped")

	unscoped := new(mockContainer.FilterableContainer)
	unscoped.On("IsWatchtower").Return(false).Maybe()
	unscoped.On("Enabled").Return(false, false).Maybe()
	unscoped.On("Scope").Return("", false)
	unscoped.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	unscoped.On("Name").Return("/unscoped")

	assert.False(t, filter(scoped))
	assert.True(t, filter(unscoped))

	scoped.AssertExpectations(t)
	unscoped.AssertExpectations(t)

	oldNamed := new(mockContainer.FilterableContainer)
	oldNamed.On("IsWatchtower").Return(true)
	oldNamed.On("Name").Return("/watchtower-old-abc123")

	assert.False(t, filter(oldNamed))
	oldNamed.AssertExpectations(t)
}

func TestFilterByDisabledLabelsExactMatch(t *testing.T) {
	t.Parallel()

	filter := filterByDisabledLabels(newTestLogger(t), map[string]string{enableLabelKey: "false"}, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("true", true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("false", true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/test")
	container.On("GetLabel", enableLabelKey).Return("", false)
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByDisableNamesLeadingSlashScenarios(t *testing.T) {
	t.Parallel()

	// Test container with normalized name excluded by filter without slash
	disableNames := []string{"excluded"}
	filter := FilterByDisableNames(newTestLogger(t), disableNames, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("excluded")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Test container with normalized name excluded by filter with slash
	disableNames = []string{"/excluded"}
	filter = FilterByDisableNames(newTestLogger(t), disableNames, NoFilter)
	assert.NotNil(t, filter)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("excluded")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Test multiple containers with normalized filter inputs
	disableNames = []string{"container1", "container2", "container3"}
	filter = FilterByDisableNames(newTestLogger(t), disableNames, NoFilter)
	assert.NotNil(t, filter)

	// Should exclude container1
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container1")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Should exclude container2
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container2")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Should exclude container3
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container3")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Test multiple containers with leading slash in filter inputs
	disableNames = []string{"/container1", "/container2", "/container3"}
	filter = FilterByDisableNames(newTestLogger(t), disableNames, NoFilter)
	assert.NotNil(t, filter)

	// Should exclude /container1
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container1")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Should exclude /container2
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container2")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Should exclude /container3
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container3")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Should allow non-excluded container
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("container4")
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByImage(t *testing.T) {
	t.Parallel()

	filterEmpty := FilterByImage(newTestLogger(t), nil, NoFilter)
	filterSingle := FilterByImage(newTestLogger(t), []string{"registry"}, NoFilter)
	filterMultiple := FilterByImage(newTestLogger(t), []string{"registry", "bla"}, NoFilter)

	assert.NotNil(t, filterSingle)
	assert.NotNil(t, filterMultiple)

	container := new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("registry:2")
	container.On("Name").Return("/test")
	assert.True(t, filterEmpty(container))
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("registry:latest")
	container.On("Name").Return("/test")
	assert.True(t, filterEmpty(container))
	assert.True(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("abcdef1234")
	container.On("Name").Return("/test")
	assert.True(t, filterEmpty(container))
	assert.False(t, filterSingle(container))
	assert.False(t, filterMultiple(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("bla:latest")
	container.On("Name").Return("/test")
	assert.True(t, filterEmpty(container))
	assert.False(t, filterSingle(container))
	assert.True(t, filterMultiple(container))
	container.AssertExpectations(t)

	filterEmptyTagged := FilterByImage(newTestLogger(t), nil, NoFilter)
	filterSingleTagged := FilterByImage(newTestLogger(t), []string{"registry:develop"}, NoFilter)
	filterMultipleTagged := FilterByImage(newTestLogger(t), []string{"registry:develop", "registry:latest"}, NoFilter)

	assert.NotNil(t, filterSingleTagged)
	assert.NotNil(t, filterMultipleTagged)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("bla:latest")
	container.On("Name").Return("/test")
	assert.True(t, filterEmptyTagged(container))
	assert.False(t, filterSingleTagged(container))
	assert.False(t, filterMultipleTagged(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("registry:latest")
	container.On("Name").Return("/test")
	assert.True(t, filterEmptyTagged(container))
	assert.False(t, filterSingleTagged(container))
	assert.True(t, filterMultipleTagged(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("registry:develop")
	container.On("Name").Return("/test")
	assert.True(t, filterEmptyTagged(container))
	assert.True(t, filterSingleTagged(container))
	assert.True(t, filterMultipleTagged(container))
	container.AssertExpectations(t)
}

func TestFilterByImageMalformed(t *testing.T) {
	t.Parallel()

	filter := FilterByImage(newTestLogger(t), []string{"valid:image", "invalid::tag", "image:", ":tag", ""}, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("valid:image")
	container.On("Name").Return("/test")
	assert.True(t, filter(container)) // Valid match
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("valid:other")
	container.On("Name").Return("/test")
	assert.False(t, filter(container)) // Tag mismatch
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("invalid:tag")
	container.On("Name").Return("/test")
	assert.False(t, filter(container)) // Malformed input ignored
	container.AssertExpectations(t)
}

func TestMatchImageAndTagHostPortRegistry(t *testing.T) {
	t.Parallel()

	// host:port must not be treated as image:tag.
	// Different tags must not match.
	assert.False(t, matchImageAndTag("localhost:5000/nginx:alpine", "localhost:5000/nginx:latest"))
	assert.True(t, matchImageAndTag("localhost:5000/nginx:alpine", "localhost:5000/nginx:alpine"))
	assert.True(t, matchImageAndTag("localhost:5000/nginx:alpine", "localhost:5000/nginx"))
	assert.False(t, matchImageAndTag("my.reg:443/a/b:1", "my.reg:443/a/b:2"))
	assert.True(t, matchImageAndTag("my.reg:443/a/b:1", "my.reg:443/a/b:1"))

	// Standard image:tag matching still works.
	assert.False(t, matchImageAndTag("nginx:latest", "nginx:develop"))
	assert.True(t, matchImageAndTag("nginx:latest", "nginx:latest"))
	assert.True(t, matchImageAndTag("nginx:latest", "nginx"))
}

func TestFilterByImageHostPortRegistry(t *testing.T) {
	t.Parallel()

	filterTagged := FilterByImage(newTestLogger(t), []string{"localhost:5000/app:v2"}, NoFilter)

	container := new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("localhost:5000/app:v1")
	container.On("Name").Return("/app")
	assert.False(t, filterTagged(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("ImageName").Return("localhost:5000/app:v2")
	container.On("Name").Return("/app")
	assert.True(t, filterTagged(container))
	container.AssertExpectations(t)
}

func TestBuildFilter(t *testing.T) {
	t.Parallel()

	names := []string{"test", "valid"}

	filter, desc, err := BuildFilter(newTestLogger(t), names, []string{}, nil, nil, nil, nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, desc, "test")
	assert.Contains(t, desc, "or")
	assert.Contains(t, desc, "valid")

	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("Invalid").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("test").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("Invalid").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("test").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterEnableLabel(t *testing.T) {
	// t.Parallel()
	names := make([]string, 0, 1)
	names = append(names, "test")

	filter, desc, err := BuildFilter(newTestLogger(t), names, []string{}, nil, nil, nil, nil, true, "")
	require.NoError(t, err)
	assert.Contains(t, desc, "with label")
	assert.Contains(t, desc, `com.centurylinklabs.watchtower.enable`)

	// Container without enable label: excluded because enableLabel requires it.
	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("Name").Return("test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Container with enable=false: excluded by boolean parsing.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Enabled").Return(false, true).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("Name").Return("test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Container with enable=true: included.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Once()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Once()
	container.On("Name").Return("test").Maybe()
	result := filter(container)
	assert.True(t, result)
	container.AssertExpectations(t)
}

// TestBuildFilterEnableLabelPreservesUserCriteria verifies that user-provided
// enabled label criteria for the enable-label key are not silently overwritten
// by the boolean enableLabel flag. Both filters apply independently.
func TestBuildFilterEnableLabelPreservesUserCriteria(t *testing.T) {
	t.Parallel()

	// User requests only containers with enable=false, while enableLabel=true
	// requires enable=true. These conflict, so a container with enable=true
	// must be excluded by the user's exact-match criterion.
	filter, _, err := BuildFilter(newTestLogger(t),
		[]string{"test"},
		[]string{},
		nil,
		nil,
		[]string{enableLabelKey + "=false"},
		nil,
		true,
		"",
	)
	require.NoError(t, err)

	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("GetLabel", enableLabelKey).Return("true", true).Once()
	container.On("Scope").Return("", false).Maybe()
	container.On("Name").Return("test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterDisableContainer(t *testing.T) {
	t.Parallel()

	filter, desc, err := BuildFilter(newTestLogger(t), []string{}, []string{"excluded", "notfound"}, nil, nil, nil, nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, desc, "not named")
	assert.Contains(t, desc, "excluded")
	assert.Contains(t, desc, "or")
	assert.Contains(t, desc, "notfound")

	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("Another").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("AnotherOne").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("test").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("excluded").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("excludedAsSubstring").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("notfound").Maybe()
	container.On("Enabled").Return(true, true).Maybe()
	container.On("Scope").Return("", false).Maybe() // No scope set, defaults to "none"
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Enabled").Return(false, true).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("Name").Return("/test").Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByImageNames(t *testing.T) {
	t.Parallel()

	patterns := make([]string, 0, 1)

	filter := filterByMonitoredImageNamePatterns(newTestLogger(t), patterns, nil)
	assert.Nil(t, filter)

	patterns = append(patterns, "nginx:latest")
	filter = filterByMonitoredImageNamePatterns(newTestLogger(t), patterns, NoFilter)
	assert.NotNil(t, filter)

	// Image matches -> kept.
	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("nginx:latest")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Image does not match -> excluded.
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("cache").Maybe()
	container.On("ImageName").Return("redis:latest")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByImageNamesRegex(t *testing.T) {
	t.Parallel()

	filter := filterByMonitoredImageNamePatterns(newTestLogger(t), []string{"nginx:.*"}, NoFilter)
	assert.NotNil(t, filter)

	// Anchored regex matches any nginx tag.
	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("nginx:1.25")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Anchored regex does not match a different image name.
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("nginxx:1.25")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByImageNamesRegistryPath(t *testing.T) {
	t.Parallel()

	filter := filterByMonitoredImageNamePatterns(newTestLogger(t), []string{"docker.io/library/nginx:.*"}, NoFilter)
	assert.NotNil(t, filter)

	// Registry path with slashes matches.
	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("docker.io/library/nginx:1.25")
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Different registry does not match.
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("ghcr.io/org/nginx:1.25")
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterBySkippedImageNames(t *testing.T) {
	t.Parallel()

	patterns := make([]string, 0, 1)

	filter := filterBySkippedImageNamePatterns(newTestLogger(t), patterns, nil)
	assert.Nil(t, filter)

	patterns = append(patterns, "nginx:latest")
	filter = filterBySkippedImageNamePatterns(newTestLogger(t), patterns, NoFilter)
	assert.NotNil(t, filter)

	// Skipped image.
	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("nginx:latest")
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Non-skipped image passes through baseFilter.
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("cache").Maybe()
	container.On("ImageName").Return("redis:latest")
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterImageNames(t *testing.T) {
	t.Parallel()

	filter, desc, err := BuildFilter(newTestLogger(t),
		nil, nil,
		[]string{"nginx:.*", "redis:.*"},
		[]string{"redis:latest"},
		nil, nil,
		false, "",
	)
	require.NoError(t, err)
	assert.Contains(t, desc, "which image matches")
	assert.Contains(t, desc, "nginx:.*")
	assert.Contains(t, desc, "whose image is not one of")
	assert.Contains(t, desc, "redis:latest")

	// Image matches a pattern and is not skipped -> kept.
	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/web").Maybe()
	container.On("ImageName").Return("nginx:1.25").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Image matches no pattern -> excluded.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/api").Maybe()
	container.On("ImageName").Return("api:latest").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Image matches a pattern but is explicitly skipped -> excluded.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/cache").Maybe()
	container.On("ImageName").Return("redis:latest").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Image matches pattern and skip pattern does not match -> kept.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/cache2").Maybe()
	container.On("ImageName").Return("redis:7").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterImageNamesWithContainerNames(t *testing.T) {
	t.Parallel()

	// When both container name and image name patterns are set,
	// a container must match BOTH to be included.
	filter, desc, err := BuildFilter(newTestLogger(t),
		[]string{"web"},
		nil,
		[]string{"nginx:.*"},
		nil,
		nil, nil,
		false, "",
	)
	require.NoError(t, err)
	assert.Contains(t, desc, "which name matches")
	assert.Contains(t, desc, "which image matches")

	// Matches both container name and image name -> kept.
	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("nginx:1.25").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	// Matches container name but not image name -> excluded.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("web").Maybe()
	container.On("ImageName").Return("redis:latest").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	// Matches image name but not container name -> excluded.
	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("api").Maybe()
	container.On("ImageName").Return("nginx:1.25").Maybe()
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestFilterByEnabledLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"Service": "Pelican", "Env": "prod"}

	filter := filterByEnabledLabels(newTestLogger(t), labels, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("/pelican-1")
	container.On("GetLabel", "Service").Return("Pelican", true).Once()
	container.On("GetLabel", "Env").Return("other", true).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/prod-app")
	container.On("GetLabel", "Env").Return("prod", true).Once()
	container.On("GetLabel", "Service").Return("other", true).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/other")
	container.On("GetLabel", "Service").Return("wrong", true).Once()
	container.On("GetLabel", "Env").Return("wrong", true).Once()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/no-label")
	container.On("GetLabel", "Service").Return("", false).Once()
	container.On("GetLabel", "Env").Return("", false).Once()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	emptyFilter := filterByEnabledLabels(newTestLogger(t), nil, NoFilter)
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/anything").Maybe()
	assert.True(t, emptyFilter(container))
	container.AssertExpectations(t)
}

func TestFilterByDisabledLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"Service": "Pelican", "Env": "dev"}

	filter := filterByDisabledLabels(newTestLogger(t), labels, NoFilter)
	assert.NotNil(t, filter)

	container := new(mockContainer.FilterableContainer)
	container.On("Name").Return("/pelican-1")
	container.On("GetLabel", "Service").Return("Pelican", true).Once()
	container.On("GetLabel", "Env").Return("other", true).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/dev-app")
	container.On("GetLabel", "Env").Return("dev", true).Once()
	container.On("GetLabel", "Service").Return("other", true).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/prod-app")
	container.On("GetLabel", "Service").Return("prod", true).Once()
	container.On("GetLabel", "Env").Return("prod", true).Once()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/no-label")
	container.On("GetLabel", "Service").Return("", false).Once()
	container.On("GetLabel", "Env").Return("", false).Once()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	emptyFilter := filterByDisabledLabels(newTestLogger(t), nil, NoFilter)
	container = new(mockContainer.FilterableContainer)
	container.On("Name").Return("/anything").Maybe()
	assert.True(t, emptyFilter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterEnabledLabels(t *testing.T) {
	t.Parallel()

	filter, desc, err := BuildFilter(newTestLogger(t), nil, nil, nil, nil, []string{"Service=Pelican"}, nil, false, "none")
	require.NoError(t, err)
	assert.Contains(t, desc, "with label")
	assert.Contains(t, desc, `Service="Pelican"`)

	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/pelican-1").Maybe()
	container.On("GetLabel", "Service").Return("Pelican", true)
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/other").Maybe()
	container.On("GetLabel", "Service").Return("Other", true)
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildFilterDisabledLabels(t *testing.T) {
	t.Parallel()

	filter, desc, err := BuildFilter(newTestLogger(t), nil, nil, nil, nil, nil, []string{"Service=Pelican"}, false, "none")
	require.NoError(t, err)
	assert.Contains(t, desc, "without label")
	assert.Contains(t, desc, `Service="Pelican"`)

	container := new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/pelican-1").Maybe()
	container.On("GetLabel", "Service").Return("Pelican", true)
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mockContainer.FilterableContainer)
	container.On("IsWatchtower").Return(false).Maybe()
	container.On("Name").Return("/other").Maybe()
	container.On("GetLabel", "Service").Return("Other", true)
	container.On("Enabled").Return(false, false).Maybe()
	container.On("Scope").Return("", false).Maybe()
	container.On("GetLabel", enableLabelKey).Return("", false).Maybe()
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestParseLabelPairs_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:     "simple pair",
			input:    []string{"key=value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "empty value",
			input:    []string{"key="},
			expected: map[string]string{"key": ""},
		},
		{
			name:     "multiple pairs",
			input:    []string{"a=1", "b=2"},
			expected: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:     "value with equals",
			input:    []string{"key=val=ue"},
			expected: map[string]string{"key": "val=ue"},
		},
		{
			name:     "whitespace trimmed from key and value",
			input:    []string{"  key  =  value  "},
			expected: map[string]string{"key": "value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseLabelPairs(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseLabelPairs_TooLong(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("a", maxLabelPairBytes-len("key")+1)
	input := []string{"key=" + longValue}

	result, err := parseLabelPairs(input)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errLabelPairTooLong)
}

func TestParseLabelPairs_TooMany(t *testing.T) {
	t.Parallel()

	input := make([]string, maxLabelPairs+1)
	for i := range input {
		input[i] = fmt.Sprintf("key%d=val%d", i, i)
	}

	result, err := parseLabelPairs(input)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errTooManyLabelPairs)
}

func TestParseLabelPairs_Malformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     []string
		errPrefix error
	}{
		{
			name:      "missing equals",
			input:     []string{"keyvalue"},
			errPrefix: errLabelPairMissingEquals,
		},
		{
			name:      "empty key",
			input:     []string{"=value"},
			errPrefix: errLabelPairEmptyKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseLabelPairs(tc.input)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, tc.errPrefix)
		})
	}
}

func TestCompileNamePatterns_ClassifiesLiteralsAndRegex(t *testing.T) {
	t.Parallel()

	compiled := compileNamePatterns([]string{"web", "nginx:.*", "["}, false)

	require.Equal(t, []string{"web", "["}, compiled.exact)
	require.Len(t, compiled.regexes, 1)
	assert.True(t, compiled.match("web"))
	assert.True(t, compiled.match("nginx:1.25"))
	assert.True(t, compiled.match("["))
	assert.False(t, compiled.match("db"))
}

func TestFilterByDisableNames_InvalidRegexIsExactMatch(t *testing.T) {
	t.Parallel()

	filter := FilterByDisableNames(newTestLogger(t), []string{"["}, NoFilter)

	excluded := new(mockContainer.FilterableContainer)
	excluded.On("Name").Return("[")
	assert.False(t, filter(excluded))
	excluded.AssertExpectations(t)

	kept := new(mockContainer.FilterableContainer)
	kept.On("Name").Return("app")
	assert.True(t, filter(kept))
	kept.AssertExpectations(t)
}

func TestFilterByNames_InvalidRegexIsExactMatch(t *testing.T) {
	t.Parallel()

	filter := FilterByNames(newTestLogger(t), []string{"["}, NoFilter)

	matched := new(mockContainer.FilterableContainer)
	matched.On("Name").Return("[")
	assert.True(t, filter(matched))
	matched.AssertExpectations(t)

	other := new(mockContainer.FilterableContainer)
	other.On("Name").Return("app")
	assert.False(t, filter(other))
	other.AssertExpectations(t)
}

func TestFilters_DebugLogging(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer

	debug := zerolog.New(&logBuf).Level(zerolog.DebugLevel)

	nameFilter := FilterByNames(&debug, []string{"web"}, NoFilter)
	named := new(mockContainer.FilterableContainer)
	named.On("Name").Return("web")
	assert.True(t, nameFilter(named))
	named.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Matched container by name/pattern")

	logBuf.Reset()

	missed := new(mockContainer.FilterableContainer)
	missed.On("Name").Return("db")
	assert.False(t, nameFilter(missed))
	missed.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container name did not match any filter")

	logBuf.Reset()

	disableFilter := FilterByDisableNames(&debug, []string{"skip"}, NoFilter)
	skipped := new(mockContainer.FilterableContainer)
	skipped.On("Name").Return("skip")
	assert.False(t, disableFilter(skipped))
	skipped.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container excluded by disable name/pattern")

	logBuf.Reset()

	kept := new(mockContainer.FilterableContainer)
	kept.On("Name").Return("web")
	assert.True(t, disableFilter(kept))
	kept.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container not excluded by disable names")

	logBuf.Reset()

	imageFilter := filterByMonitoredImageNamePatterns(&debug, []string{"nginx:.*"}, NoFilter)
	matchedImage := new(mockContainer.FilterableContainer)
	matchedImage.On("Name").Return("web")
	matchedImage.On("ImageName").Return("nginx:1.25")
	assert.True(t, imageFilter(matchedImage))
	matchedImage.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container image matched pattern")

	logBuf.Reset()

	otherImage := new(mockContainer.FilterableContainer)
	otherImage.On("Name").Return("cache")
	otherImage.On("ImageName").Return("redis:latest")
	assert.False(t, imageFilter(otherImage))
	otherImage.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container image did not match any pattern")

	logBuf.Reset()

	skipImageFilter := filterBySkippedImageNamePatterns(&debug, []string{"redis:.*"}, NoFilter)
	skippedImage := new(mockContainer.FilterableContainer)
	skippedImage.On("Name").Return("cache")
	skippedImage.On("ImageName").Return("redis:latest")
	assert.False(t, skipImageFilter(skippedImage))
	skippedImage.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container image matched skip pattern")

	logBuf.Reset()

	keptImage := new(mockContainer.FilterableContainer)
	keptImage.On("Name").Return("web")
	keptImage.On("ImageName").Return("nginx:latest")
	assert.True(t, skipImageFilter(keptImage))
	keptImage.AssertExpectations(t)
	assert.Contains(t, logBuf.String(), "Container not skipped by image name patterns")
}
