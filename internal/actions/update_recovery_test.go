package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// recoveryContainer returns a Watchtower container for the orphan recovery
// tests.
//
// Parameters:
//   - id: Container ID.
//   - name: Container name.
//   - running: Whether the container is running. A container that is not
//     running is in the created state.
//   - scope: Scope label value. Empty leaves the container without a scope.
//
// Returns:
//   - types.Container: Configured mock container.
func recoveryContainer(id, name string, running bool, scope string) types.Container {
	labels := map[string]string{"com.centurylinklabs.watchtower": "true"}
	if scope != "" {
		labels["com.centurylinklabs.watchtower.scope"] = scope
	}

	ctr := mockActions.CreateMockContainerWithConfig(
		id,
		name,
		"watchtower:latest",
		running,
		false,
		time.Now(),
		&dockerContainer.Config{Image: "watchtower:latest", Labels: labels},
	)

	if !running {
		ctr.ContainerInfo().State.Status = dockerContainer.StateCreated
	}

	return ctr
}

// expectListed expects one listing of all containers and answers it with
// containers.
//
// Parameters:
//   - client: Mocked container client.
//   - containers: Containers the listing returns.
func expectListed(client *mockContainer.MockClient, containers ...types.Container) {
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(containers, nil).Once()
}

// TestTryRecoverOrphanedContainer verifies that a Watchtower replacement left
// in the created state is started and returned, and that recovery gives up
// without error when listing fails, the start fails, or no orphan in the
// current container's scope exists.
func TestTryRecoverOrphanedContainer(t *testing.T) {
	t.Parallel()

	t.Run("listing containers fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return false without error", func(t *testing.T) {
			t.Parallel()

			client := mockContainer.NewMockClient(t)
			client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Once()

			current := mockActions.CreateMockContainer("current-id", "watchtower", "watchtower:latest", time.Now())

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})

	t.Run("no orphaned container exists", func(t *testing.T) {
		t.Parallel()

		t.Run("should return false", func(t *testing.T) {
			t.Parallel()

			// The mock has no start expectation, so starting a container fails
			// the test.
			oldContainer := recoveryContainer("old-id", "watchtower-old-abc123", true, "")
			current := mockActions.CreateMockContainer("current-id", "watchtower", "watchtower:latest", time.Now())

			client := mockContainer.NewMockClient(t)
			expectListed(client, oldContainer, current)

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})

	t.Run("an orphaned created-state Watchtower container exists", func(t *testing.T) {
		t.Parallel()

		t.Run("should start it and return the recovered container", func(t *testing.T) {
			t.Parallel()

			orphaned := recoveryContainer("orphaned-id", "watchtower-xyz", false, "")
			current := recoveryContainer("current-id", "watchtower-old-abc123", true, "")

			client := mockContainer.NewMockClient(t)
			expectListed(client, orphaned, current)
			client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID("orphaned-id")).Return(nil).Once()

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.True(t, ok)
			require.NotNil(t, recovered)
			assert.Equal(t, types.ContainerID("orphaned-id"), recovered.ID())
		})
	})

	t.Run("start fails for the orphaned container", func(t *testing.T) {
		t.Parallel()

		t.Run("should continue searching and return false", func(t *testing.T) {
			t.Parallel()

			orphaned := recoveryContainer("orphaned-id", "watchtower-xyz", false, "")
			current := recoveryContainer("current-id", "watchtower-old-abc123", true, "")

			client := mockContainer.NewMockClient(t)
			expectListed(client, orphaned, current)
			client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID("orphaned-id")).
				Return(context.DeadlineExceeded).Once()

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})

	t.Run("the current container is nil", func(t *testing.T) {
		t.Parallel()

		t.Run("should not panic and return false", func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so any client call fails the test.
			client := mockContainer.NewMockClient(t)

			var (
				recovered types.Container
				ok        bool
			)

			require.NotPanics(t, func() {
				recovered, ok = TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, nil)
			})
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})

	t.Run("the orphaned container is in a different scope", func(t *testing.T) {
		t.Parallel()

		t.Run("should not start it", func(t *testing.T) {
			t.Parallel()

			orphaned := recoveryContainer("orphaned-id", "watchtower-xyz", false, "scope-b")
			current := recoveryContainer("current-id", "watchtower-old-abc123", true, "scope-a")

			client := mockContainer.NewMockClient(t)
			expectListed(client, orphaned, current)

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})

	t.Run("the current container has no scope and the orphaned container has an explicit scope", func(t *testing.T) {
		t.Parallel()

		t.Run("should not start it", func(t *testing.T) {
			t.Parallel()

			orphaned := recoveryContainer("orphaned-id", "watchtower-xyz", false, "scope-b")
			current := recoveryContainer("current-id", "watchtower-old-abc123", true, "")

			client := mockContainer.NewMockClient(t)
			expectListed(client, orphaned, current)

			recovered, ok := TryRecoverOrphanedContainer(newTestLogger(t), t.Context(), client, current)
			assert.False(t, ok)
			assert.Nil(t, recovered)
		})
	})
}
