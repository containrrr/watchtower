package actions

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockContainerTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// TestValidateRollingRestartDependencies verifies that rolling restarts are
// rejected for containers with links and that listing failures are reported.
func TestValidateRollingRestartDependencies(t *testing.T) {
	t.Parallel()

	// filter accepts every container.
	filter := types.Filter(func(types.FilterableContainer) bool { return true })

	t.Run("ListContainers fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return wrapped error", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)
			expectedErr := errors.New("list containers failed")
			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(nil, expectedErr)

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.Error(t, err)
			require.ErrorContains(t, err, "list containers failed")
		})
	})

	t.Run("no containers are found", func(t *testing.T) {
		t.Parallel()

		t.Run("should return nil", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)
			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil)

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
		})
	})

	t.Run("containers have no links", func(t *testing.T) {
		t.Parallel()

		t.Run("should return nil", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)
			mockContainer1 := mockContainerTypes.NewMockContainer(t)
			mockContainer2 := mockContainerTypes.NewMockContainer(t)

			mockContainer1.EXPECT().Links(true).Return([]string{})
			mockContainer2.EXPECT().Links(true).Return([]string{})

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				mockContainer1,
				mockContainer2,
			}, nil)

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
		})
	})

	t.Run("a container has links", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error with container name and links", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)
			mockContainer1 := mockContainerTypes.NewMockContainer(t)
			mockContainer2 := mockContainerTypes.NewMockContainer(t)

			mockContainer1.EXPECT().Links(true).Return([]string{})
			mockContainer2.EXPECT().Links(true).Return([]string{"db", "redis"})
			mockContainer2.EXPECT().Name().Return("web-container")

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{
				mockContainer1,
				mockContainer2,
			}, nil)

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.Error(t, err)
			assert.Contains(t, err.Error(), `"web-container" depends on [db redis]`)
		})
	})
}

// listFiltered returns a ListContainers implementation that keeps the
// containers every given filter accepts, as the client does, and records them
// in selected.
//
// Parameters:
//   - selected: Receives the containers the filters accept.
//   - containers: Containers the fake daemon holds.
//
// Returns:
//   - func(context.Context, ...types.Filter) ([]types.Container, error): The ListContainers implementation.
func listFiltered(
	selected *[]types.Container,
	containers ...types.Container,
) func(context.Context, ...types.Filter) ([]types.Container, error) {
	return func(_ context.Context, filters ...types.Filter) ([]types.Container, error) {
		for _, c := range containers {
			if !slices.ContainsFunc(filters, func(f types.Filter) bool { return !f(c) }) {
				*selected = append(*selected, c)
			}
		}

		return *selected, nil
	}
}

// TestScopeValidationErrorCases verifies that rolling restart validation
// checks only the containers a scope filter selects when scope labels are
// empty, malformed, or conflicting.
func TestScopeValidationErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("scope labels are malformed", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle empty scope labels gracefully", func(t *testing.T) {
			t.Parallel()

			// Test that empty scope labels are treated as unscoped
			mockClient := mockContainer.NewMockClient(t)

			// Container with no scope label
			container := mockContainerTypes.NewMockContainer(t)
			container.EXPECT().Scope().Return("", false)
			container.EXPECT().Links(true).Return([]string{}) // No links for validation

			// Scoped container the filter excludes
			scoped := mockContainerTypes.NewMockContainer(t)
			scoped.EXPECT().Scope().Return("prod", true)

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container, scoped))

			// Create unscoped filter (empty scope should default to unscoped)
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()
				if !hasScope || scope == "" {
					return true // Unscoped containers pass
				}

				return false
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container}, selected)
		})

		t.Run("should handle containers with invalid scope label formats", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			// Container with malformed scope label (e.g., contains special chars)
			container1 := mockContainerTypes.NewMockContainer(t)
			container2 := mockContainerTypes.NewMockContainer(t)

			container1.EXPECT().Scope().Return("valid-scope", true)
			container2.EXPECT().Scope().Return("invalid$scope@123", true) // Malformed scope
			container2.EXPECT().Links(true).Return([]string{})

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container1, container2))

			// Filter that matches malformed scope
			filter := func(c types.FilterableContainer) bool {
				scope, _ := c.Scope()

				return scope == "invalid$scope@123"
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container2}, selected)
		})

		t.Run("should handle scope labels with whitespace and normalization", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			// Container with scope label containing leading and trailing spaces
			container := mockContainerTypes.NewMockContainer(t)
			container.EXPECT().Scope().Return("  spaced scope  ", true)
			container.EXPECT().Links(true).Return([]string{})

			// Container whose scope matches only after trimming
			trimmed := mockContainerTypes.NewMockContainer(t)
			trimmed.EXPECT().Scope().Return("spaced scope", true)

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container, trimmed))

			// Filter that expects the scope as written
			filter := func(c types.FilterableContainer) bool {
				scope, _ := c.Scope()

				return scope == "  spaced scope  " // Exact match including spaces
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container}, selected)
		})
	})

	t.Run("scope validation encounters errors", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle scope label retrieval errors gracefully", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			// Container without a scope label
			container := mockContainerTypes.NewMockContainer(t)
			container.EXPECT().Scope().Return("", false)      // Simulate missing scope label
			container.EXPECT().Links(true).Return([]string{}) // No validation issues

			// Scoped container the filter excludes
			scoped := mockContainerTypes.NewMockContainer(t)
			scoped.EXPECT().Scope().Return("prod", true)

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container, scoped))

			// Filter that handles missing scopes
			filter := func(c types.FilterableContainer) bool {
				_, hasScope := c.Scope()

				return !hasScope // Include containers without scope labels
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container}, selected)
		})

		t.Run("should validate scope consistency across operations", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			// Multiple containers that should have consistent scope behavior
			container1 := mockContainerTypes.NewMockContainer(t)
			container2 := mockContainerTypes.NewMockContainer(t)

			container1.EXPECT().Scope().Return("consistent-scope", true)
			container1.EXPECT().Links(true).Return([]string{})
			container2.EXPECT().Scope().Return("consistent-scope", true)
			container2.EXPECT().Links(true).Return([]string{})

			// Container in another scope the filter excludes
			other := mockContainerTypes.NewMockContainer(t)
			other.EXPECT().Scope().Return("other-scope", true)

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container1, other, container2))

			// Filter requiring scope consistency
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()

				return hasScope && scope == "consistent-scope"
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container1, container2}, selected)
		})

		t.Run("should handle scope label conflicts during validation", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			// Container with conflicting scope information
			container := mockContainerTypes.NewMockContainer(t)
			// Simulate potential inconsistency - scope exists but is empty
			container.EXPECT().Scope().Return("", true) // Has scope label but it's empty
			container.EXPECT().Links(true).Return([]string{})

			// Container without a scope label the filter excludes
			unscoped := mockContainerTypes.NewMockContainer(t)
			unscoped.EXPECT().Scope().Return("", false)

			var selected []types.Container

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				RunAndReturn(listFiltered(&selected, container, unscoped))

			// Filter that detects empty scope labels
			filter := func(c types.FilterableContainer) bool {
				scope, hasScope := c.Scope()

				return hasScope && scope == "" // Allow empty scoped containers
			}

			err := ValidateRollingRestartDependencies(newTestLogger(t), t.Context(), mockClient, filter, true)

			require.NoError(t, err)
			assert.Equal(t, []types.Container{container}, selected)
		})
	})
}
