package actions

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestRemoveExcessWatchtowerInstances verifies which other Watchtower instances
// are stopped and removed, with and without a scope, which of their images are
// collected for cleanup, how chained containers are handled, and how stop and
// removal failures are reported.
func TestRemoveExcessWatchtowerInstances(t *testing.T) {
	t.Parallel()

	t.Run("no scope is specified", func(t *testing.T) {
		t.Parallel()

		t.Run("should return nil when only one instance exists", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			mockContainer := createMockContainer(
				"watchtower",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{mockContainer}, nil)

			var cleanupImageInfo []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"",
				&cleanupImageInfo,
				mockContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Empty(t, cleanupImageInfo)
		})

		t.Run("should stop excess instances and collect image IDs when cleanup enabled", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"watchtower:old",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			require.Len(t, cleanupImageIDs, 1)
			assert.Equal(t, types.ImageID("watchtower:old"), cleanupImageIDs[0].ImageID)
		})
	})

	t.Run("scope is specified", func(t *testing.T) {
		t.Parallel()

		t.Run("should only clean up instances in the same scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			scopedOldContainer := createMockContainer(
				"watchtower-scoped",
				"watchtower-scoped",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopedOldContainer}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"prod",
				&cleanupImageIDs,
				scopedOldContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})

		t.Run("should clean up multiple instances within the same scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"watchtower-prod-old",
				"watchtower-prod-old",
				"watchtower:1.11.0",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)
			newContainer := createMockContainer(
				"watchtower-prod-new",
				"watchtower-prod-new",
				"watchtower:1.12.0",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"prod",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.True(t, slices.ContainsFunc(cleanupImageIDs, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("watchtower:1.11.0") }))
			require.Len(t, cleanupImageIDs, 1)
		})

		t.Run("should return false when only one instance exists in scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			scopedContainer := createMockContainer(
				"watchtower-prod",
				"watchtower-prod",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopedContainer}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"prod",
				&cleanupImageIDs,
				scopedContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})
	})

	t.Run("cleanup is disabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop excess instances but not collect image IDs", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"watchtower:old",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})
	})

	t.Run("error scenarios", func(t *testing.T) {
		t.Parallel()

		t.Run("should not perform cleanup when currentContainer is nil", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})

		t.Run("should return error when stopping container fails", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				oldContainer := createMockContainer(
					"watchtower-old",
					"watchtower-old",
					"watchtower:old",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{oldContainer, newContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
					Return(errors.New("stop container failed")).
					Times(maxRemovalAttempts)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					false,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "1 of 1 instances failed to stop")
				assert.Equal(t, 0, cleanupOccurred)
				assert.Empty(t, cleanupImageIDs)
			})
		})

		t.Run("should fail completely when some containers fail to stop after retries", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				old1Container := createMockContainer(
					"watchtower-old1",
					"watchtower-old1",
					"watchtower:old1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				old2Container := createMockContainer(
					"watchtower-old2",
					"watchtower-old2",
					"watchtower:old2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{old1Container, old2Container, newContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
					Return(errors.New("stop container failed")).
					Times(maxRemovalAttempts)
				mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				require.Error(t, err)
				// Partial success: one container was removed before failure
				assert.Equal(t, 1, cleanupOccurred)
				assert.Empty(t, cleanupImageIDs)
			})
		})

		t.Run("should retry 'already in progress' errors and fail if they persist", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				old1Container := createMockContainer(
					"watchtower-old1",
					"watchtower-old1",
					"watchtower:old1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				old2Container := createMockContainer(
					"watchtower-old2",
					"watchtower-old2",
					"watchtower:old2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{old1Container, old2Container, newContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
					Return(errors.New("removal of container watchtower-old1 is already in progress")).
					Times(maxRemovalAttempts)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).
					Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				require.Error(t, err)
				assert.Equal(t, 1, cleanupOccurred)
				assert.Empty(t, cleanupImageIDs)
			})
		})

		t.Run("should treat 'no such container' errors as non-errors and continue cleanup", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			old1Container := createMockContainer(
				"watchtower-old1",
				"watchtower-old1",
				"watchtower:old1",
				true,
				false,
				time.Now().Add(-2*time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			old2Container := createMockContainer(
				"watchtower-old2",
				"watchtower-old2",
				"watchtower:old2",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{old1Container, old2Container, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
				Return(cerrdefs.ErrNotFound)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).
				Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 2, cleanupOccurred)
			assert.True(t, slices.ContainsFunc(cleanupImageIDs, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("watchtower:old2") }))
			require.Len(t, cleanupImageIDs, 1)
		})
	})

	t.Run("image ID handling", func(t *testing.T) {
		t.Parallel()

		t.Run("should not collect image ID when excess container shares image with newest", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})

		t.Run("should skip empty image IDs", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageIDs)
		})
	})

	t.Run("container chain cleanup", func(t *testing.T) {
		t.Parallel()

		t.Run("should cleanup old containers in the chain", func(t *testing.T) {
			t.Parallel()

			oldID := types.ContainerID("old123")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			newID := types.ContainerID("new456")

			newContainer := createMockContainer(
				string(newID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"",
				&cleanupImageInfos,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should cleanup old container via name belt even when current detection points to the old one", func(t *testing.T) {
			t.Parallel()

			oldID := types.ContainerID("old123")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old-old123",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			newID := types.ContainerID("new456")

			newContainer := createMockContainer(
				string(newID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			// Belt case: current detection selected the old predecessor.
			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"",
				&cleanupImageInfos,
				oldContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should select same-scope successor when resolving old current container", func(t *testing.T) {
			t.Parallel()

			// Old current in scope-a with two non-old successors in different scopes.
			// Must pick the scope-a successor, not the scope-b one.
			oldID := types.ContainerID("old-scoped-a")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old-scoped-a",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-2*time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-a",
				},
			)

			scopeANewID := types.ContainerID("new-scoped-a")

			scopeANewContainer := createMockContainer(
				string(scopeANewID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "scope-a",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			scopeBNewID := types.ContainerID("new-scoped-b")

			scopeBNewContainer := createMockContainer(
				string(scopeBNewID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-b",
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			// scope-b container appears first in the list to test that it is skipped
			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopeBNewContainer, oldContainer, scopeANewContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"scope-a",
				&cleanupImageInfos,
				oldContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should respect scope boundaries when cleaning up container chains", func(t *testing.T) {
			t.Parallel()

			oldID := types.ContainerID("old-scoped")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			newID := types.ContainerID("new-scoped")

			newContainer := createMockContainer(
				string(newID),
				"watchtower-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(t.Context(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)
			// Should not clean up different scope container

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"prod", // scope specified
				&cleanupImageInfos,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred) // Only oldContainer cleaned
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should clean up chained parent containers from a different scope", func(t *testing.T) {
			t.Parallel()

			// The chain names a parent in a different scope. Chained parents are
			// removed even though they differ from the scope filter.
			oldDifferentScopeID := types.ContainerID("old-different-scope")

			oldDifferentScopeContainer := createMockContainer(
				string(oldDifferentScopeID),
				"watchtower-old-different",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "different-scope",
				},
			)

			newID := types.ContainerID("new-scoped")

			newContainer := createMockContainer(
				string(newID),
				"watchtower-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(oldDifferentScopeID),
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldDifferentScopeContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldDifferentScopeContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"prod", // scope specified
				&cleanupImageInfos,
				newContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should validate chain cleanup isolation with multiple scopes", func(t *testing.T) {
			t.Parallel()

			// Multiple containers in different scopes with chains
			prodOldID := types.ContainerID("prod-old")

			prodOldContainer := createMockContainer(
				string(prodOldID),
				"watchtower-prod-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			prodNewContainer := createMockContainer(
				"prod-new",
				"watchtower-prod-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(prodOldID),
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{prodOldContainer, prodNewContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, prodOldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			// Test cleanup without scope filter - should clean chained container only
			// Note: unscoped filter excludes containers with scopes, so only chained container is cleaned
			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"", // no scope filter
				&cleanupImageInfos,
				prodNewContainer, // Use prod new as current for prod scope
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			// Only chained container cleaned (scoped containers excluded from unscoped filter)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should clean cross-scope chained containers as parent containers must be removed", func(t *testing.T) {
			t.Parallel()

			// Container with chain referencing different scope - chained containers are parent containers that must be removed
			crossScopeChainContainer := createMockContainer(
				"cross-chain",
				"watchtower-cross",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "scope-a",
					"com.centurylinklabs.watchtower.container-chain": "id-from-scope-b",
				},
			)

			// Referenced container from different scope - should be cleaned as chained parent
			referencedContainer := createMockContainer(
				"id-from-scope-b",
				"watchtower-referenced",
				"watchtower:old",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-b",
				},
			)

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{referencedContainer, crossScopeChainContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, referencedContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			// Cleanup should clean the referenced container as it's a chained parent container
			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"scope-a", // Only clean scope-a
				&cleanupImageInfos,
				crossScopeChainContainer,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, cleanupOccurred)
			// Referenced container cleaned as chained parent
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, types.ImageID("watchtower:old"), cleanupImageInfos[0].ImageID)
		})
	})

	t.Run("error scenarios in scoped operations", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle partial failure during scoped cleanup with image removal errors", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				// Create containers in the same scope
				container1 := createMockContainer(
					"scoped-1",
					"watchtower-scoped-1",
					"watchtower:v1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)
				container2 := createMockContainer(
					"scoped-2",
					"watchtower-scoped-2",
					"watchtower:v2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)
				currentContainer := createMockContainer(
					"current",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{container1, container2, currentContainer}, nil)
				// Mock partial failures: container1 stops successfully,
				// container2 fails to stop entirely (should prevent all image cleanup)
				mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, container1, 10*time.Minute).Return(nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, container2, 10*time.Minute).
					Return(errors.New("container stop failed"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					true,
					"test-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "1 of 2 instances failed to stop")
				assert.Equal(t, 1, cleanupOccurred)
				// Partial success: one container removed before failure
				assert.Empty(t, cleanupImageInfos)
				// Image info cleared on failure
			})
		})

		t.Run("should maintain state consistency when scoped cleanup encounters mixed errors", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				// Create containers with different error scenarios in same scope
				notFoundContainer := createMockContainer(
					"not-found",
					"watchtower-not-found",
					"watchtower:missing",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)
				stopErrorContainer := createMockContainer(
					"stop-error",
					"watchtower-stop-error",
					"watchtower:error",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)
				currentContainer := createMockContainer(
					"current-scoped",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{notFoundContainer, stopErrorContainer, currentContainer}, nil)
				// Mock different error types
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, notFoundContainer, 10*time.Minute).
					Return(cerrdefs.ErrNotFound)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, stopErrorContainer, 10*time.Minute).
					Return(errors.New("stop failed"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					false, // No image cleanup to focus on container removal
					"error-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "1 of 2 instances failed to stop")
				assert.Equal(t, 1, cleanupOccurred)
				// Partial success: one container removed before failure
				assert.Empty(t, cleanupImageInfos)
			})
		})

		t.Run("should handle scoped cleanup when image removal is interrupted by container failure", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				successContainer := createMockContainer(
					"success",
					"watchtower-success",
					"watchtower:v1",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)
				failureContainer := createMockContainer(
					"failure",
					"watchtower-failure",
					"watchtower:v2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)
				currentContainer := createMockContainer(
					"current-interrupt",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{successContainer, failureContainer, currentContainer}, nil)
				// Success container stops successfully
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, successContainer, 10*time.Minute).
					Return(nil)
				// Failure container causes interruption (should prevent all image cleanup)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, failureContainer, 10*time.Minute).
					Return(errors.New("interruption error"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					true,
					"interrupt-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				require.Error(t, err)
				assert.Equal(t, 1, cleanupOccurred) // Partial success before failure
				assert.Empty(t, cleanupImageInfos)
				// Image info cleared on any failure
			})
		})

		t.Run("should ensure scope isolation prevents error propagation across scopes", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				// Scope A container that should fail
				scopeAContainer := createMockContainer(
					"scope-a-fail",
					"watchtower-scope-a",
					"watchtower:fail",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-a",
					},
				)

				currentContainer := createMockContainer(
					"current-a",
					"watchtower-current-a",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-a",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{scopeAContainer, currentContainer}, nil)
				// Only scope-a operations should occur
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, scopeAContainer, 10*time.Minute).
					Return(errors.New("scope-a failure"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
					t.Context(),
					mockClient,
					false,
					"scope-a", // Only clean scope-a
					&cleanupImageInfos,
					currentContainer,
				)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "1 of 1 instances failed to stop")
				assert.Equal(t, 0, cleanupOccurred)
				assert.Empty(t, cleanupImageInfos)
			})
		})
	})
}

// TestCleanupImages verifies that each collected image is removed, and that a
// missing image, a conflict, or a canceled context does not count as a
// failure.
func TestCleanupImages(t *testing.T) {
	t.Parallel()

	t.Run("should do nothing when no images are provided", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, nil)
		require.NoError(t, err)
		assert.Empty(t, cleaned)
	})

	t.Run("should attempt to remove each image ID", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
			{ImageID: ""}, // empty ID should be skipped
		}

		mockClient.EXPECT().RemoveImageByID(t.Context(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image2"), "").Return(nil)

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, cleanedImages)
		require.NoError(t, err)
		require.Len(t, cleaned, 2)
		assert.Equal(t, types.ImageID("image1"), cleaned[0].ImageID)
		assert.Equal(t, types.ImageID("image2"), cleaned[1].ImageID)
	})

	t.Run("should return error when image removal fails", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(t.Context(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(t.Context(), types.ImageID("image2"), "").
			Return(errors.New("image removal failed"))

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, cleanedImages)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "errors occurred during image cleanup")
		require.Len(t, cleaned, 1)
		assert.Equal(t, types.ImageID("image1"), cleaned[0].ImageID)
	})

	t.Run("should treat 'not found' errors as non-errors and not add to cleaned", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(t.Context(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(t.Context(), types.ImageID("image2"), "").
			Return(cerrdefs.ErrNotFound)

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, cleanedImages)
		require.NoError(t, err)
		require.Len(t, cleaned, 1)
		assert.Equal(t, types.ImageID("image1"), cleaned[0].ImageID)
	})

	t.Run("should treat context cancellation as a non-error and not add to cleaned", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
			{ImageID: "image2", ImageName: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(t.Context(), types.ImageID("image2"), "image2").
			Return(context.Canceled)

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, cleanedImages)
		require.NoError(t, err)
		require.Len(t, cleaned, 1)
		assert.Equal(t, types.ImageID("image1"), cleaned[0].ImageID)
	})

	t.Run("should treat a canceled context cause that is not context.Canceled as a non-error", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
		}

		canceledCtx, cancel := context.WithCancelCause(t.Context())
		cancel(errors.New("terminated signal received"))

		mockClient.EXPECT().
			RemoveImageByID(canceledCtx, types.ImageID("image1"), "image1").
			Return(errors.New("cannot verify image usage: terminated signal received"))

		cleaned, err := removeImages(newTestLogger(t), canceledCtx, mockClient, cleanedImages)
		require.NoError(t, err)
		assert.Empty(t, cleaned)
	})

	t.Run("should treat 'conflict' errors as non-errors and not add to cleaned", func(t *testing.T) {
		t.Parallel()

		mockClient := mockContainer.NewMockClient(t)

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
			{ImageID: "image2", ImageName: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(t.Context(), types.ImageID("image2"), "image2").
			Return(cerrdefs.ErrConflict)

		cleaned, err := removeImages(newTestLogger(t), t.Context(), mockClient, cleanedImages)
		require.NoError(t, err)
		require.Len(t, cleaned, 1)
		assert.Equal(t, types.ImageID("image1"), cleaned[0].ImageID)
	})
}

// Test_removeExcessContainers verifies that excess instances are removed and
// their images collected only when asked, and that removal is retried until it
// succeeds, the attempts run out, or the context is canceled.
func Test_removeExcessContainers(t *testing.T) {
	t.Parallel()

	t.Run("removeImageInfos is nil", func(t *testing.T) {
		t.Parallel()

		t.Run("should remove the images of removed instances right away", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().StopAndRemoveContainer(t.Context(), excessContainer, 10*time.Minute).Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(t.Context(), types.ImageID("image1"), "image1:latest").
				Return(nil)

			removed, err := removeExcessContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				[]types.Container{excessContainer},
				true,
				currentContainer,
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})

		t.Run("should log removeImages errors and still report the removed instances", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer1 := createMockContainer(
				"excess1",
				"excess1",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			excessContainer2 := createMockContainer(
				"excess2",
				"excess2",
				"image2",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image3",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer1, 10*time.Minute).Return(nil)
			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer2, 10*time.Minute).Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1:latest").
				Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(mock.Anything, types.ImageID("image2"), "image2:latest").
				Return(errors.New("remove failed"))

			removed, err := removeExcessContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				[]types.Container{excessContainer1, excessContainer2},
				true,
				currentContainer,
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 2, removed)
		})
	})

	t.Run("removeImageInfos is not nil", func(t *testing.T) {
		t.Parallel()

		t.Run("should append to provided slice without removing the images", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			var removeInfos []types.RemovedImageInfo

			mockClient.EXPECT().StopAndRemoveContainer(t.Context(), excessContainer, 10*time.Minute).Return(nil)

			removed, err := removeExcessContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				[]types.Container{excessContainer},
				true,
				currentContainer,
				&removeInfos,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
			require.Len(t, removeInfos, 1)
			assert.Equal(t, types.ImageID("image1"), removeInfos[0].ImageID)
		})
	})

	t.Run("cleanupImages is false", func(t *testing.T) {
		t.Parallel()

		t.Run("should not call removeImages", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			var removeInfos []types.RemovedImageInfo

			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).Return(nil)
			// No RemoveImageByID expected

			removed, err := removeExcessContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				&removeInfos,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
			assert.Empty(t, removeInfos)
		})
	})

	t.Run("context cancellation during retry delay", func(t *testing.T) {
		t.Parallel()

		t.Run("should return immediately when context is canceled during retry delay", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// Create a canceled context
			ctx, cancel := context.WithCancel(t.Context())
			// Cancel immediately to simulate cancellation during retry delay
			cancel()

			// First call fails, triggering retry with context cancellation
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(1)

			removed, err := removeExcessContainers(newTestLogger(t),
				ctx,
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled during retry delay")
			assert.Equal(t, 0, removed)
		})

		t.Run("should return immediately when context times out during retry delay", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// An already-expired deadline is done before the retry loop's
			// select statement runs.
			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			defer cancel()

			// First call fails, triggering retry with expired context
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(1)

			removed, err := removeExcessContainers(newTestLogger(t),
				ctx,
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			require.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Contains(t, err.Error(), "during retry delay")
			assert.Equal(t, 0, removed)
		})

		t.Run("should proceed with retry after delay when context is not canceled", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				excessContainer := createMockContainer(
					"excess",
					"excess",
					"image1",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{},
				)
				currentContainer := createMockContainer(
					"current",
					"current",
					"image2",
					true,
					false,
					time.Now(),
					map[string]string{},
				)

				// First call fails, second call succeeds after retry delay
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
					Return(errors.New("container stop failed")).
					Times(1)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
					Return(nil).
					Times(1)

				removed, err := removeExcessContainers(newTestLogger(t),
					t.Context(),
					mockClient,
					[]types.Container{excessContainer},
					false,
					currentContainer,
					nil,
				)

				require.NoError(t, err)
				assert.Equal(t, 1, removed)
			})
		})

		t.Run("should retry multiple times before succeeding when context is not canceled", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				excessContainer := createMockContainer(
					"excess",
					"excess",
					"image1",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{},
				)
				currentContainer := createMockContainer(
					"current",
					"current",
					"image2",
					true,
					false,
					time.Now(),
					map[string]string{},
				)

				// First two attempts fail, third attempt succeeds
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
					Return(errors.New("container stop failed")).
					Times(2)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
					Return(nil).
					Times(1)

				removed, err := removeExcessContainers(newTestLogger(t),
					t.Context(),
					mockClient,
					[]types.Container{excessContainer},
					false,
					currentContainer,
					nil,
				)

				require.NoError(t, err)
				assert.Equal(t, 1, removed)
			})
		})

		t.Run("should fail after max attempts when context is never canceled", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				excessContainer := createMockContainer(
					"excess",
					"excess",
					"image1",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{},
				)
				currentContainer := createMockContainer(
					"current",
					"current",
					"image2",
					true,
					false,
					time.Now(),
					map[string]string{},
				)

				// All attempts fail
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
					Return(errors.New("container stop failed")).
					Times(maxRemovalAttempts)

				removed, err := removeExcessContainers(newTestLogger(t),
					t.Context(),
					mockClient,
					[]types.Container{excessContainer},
					false,
					currentContainer,
					nil,
				)

				require.Error(t, err)
				assert.Contains(t, err.Error(), "1 of 1 instances failed to stop")
				assert.Equal(t, 0, removed)
			})
		})
	})
}

// Test_containerNames verifies that the names of a container list are returned
// in order.
func Test_containerNames(t *testing.T) {
	t.Parallel()

	t.Run("should return empty slice for empty container list", func(t *testing.T) {
		t.Parallel()

		containers := []types.Container{}
		result := containerNames(containers)
		assert.Empty(t, result)
	})

	t.Run("should return container names", func(t *testing.T) {
		t.Parallel()

		container1 := createMockContainer("id1", "name1", "image1", true, false, time.Now(), nil)
		container2 := createMockContainer("id2", "name2", "image2", true, false, time.Now(), nil)

		containers := []types.Container{container1, container2}
		result := containerNames(containers)
		assert.Equal(t, []string{"name1", "name2"}, result)
	})
}

// createMockContainer is a helper function to create a mock container for testing.
//
//nolint:unparam // running parameter is intentionally fixed for test purposes
func createMockContainer(
	id, name, image string,
	running, restarting bool,
	created time.Time,
	labels map[string]string,
) types.Container {
	if labels == nil {
		labels = make(map[string]string)
	}

	content := dockerContainer.InspectResponse{
		ID:    id,
		Image: image,
		Name:  name,
		State: &dockerContainer.State{
			Running:    running,
			Restarting: restarting,
		},
		Created: created.Format(time.RFC3339Nano),
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{
			Image:        image,
			Labels:       labels,
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	imageInfo := &dockerImage.InspectResponse{
		ID: image,
		RepoDigests: []string{
			image + "@sha256:" + strings.ReplaceAll(image, ":", "_"),
		},
	}

	return container.NewContainer(nil, &content, imageInfo)
}

// createMockCreatedContainer creates a Watchtower container in the "created" state,
// simulating a container that was created but never started during a failed self-update.
func createMockCreatedContainer(
	id, name, image string,
	created time.Time,
	labels map[string]string,
) types.Container {
	c := createMockContainer(id, name, image, false, false, created, labels)

	info := c.ContainerInfo()
	if info != nil && info.State != nil {
		info.State.Status = dockerContainer.StateCreated
	}

	return c
}

// Test_cleanupOldWatchtowerContainers verifies that old Watchtower containers
// in the current scope are found and removed, with their images when cleanup
// is enabled, that orphaned created-state containers are handled, and that
// listing failures are reported.
func Test_cleanupOldWatchtowerContainers(t *testing.T) {
	t.Parallel()

	t.Run("no old containers exist", func(t *testing.T) {
		t.Parallel()

		t.Run("should return nil when no containers exist", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should return nil when only non-Watchtower containers exist", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			nonWT := createMockContainer(
				"other-id",
				"other-app",
				"other:latest",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{nonWT}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should return nil when only current non-old Watchtower exists", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			current := createMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{current}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})
	})

	t.Run("scope normalization applies", func(t *testing.T) {
		t.Parallel()

		t.Run("should match unscoped old containers when scope is empty", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})
	})

	t.Run("scope filtering applies", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip old containers in different scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldScopedContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "other-scope",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldScopedContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"my-scope",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should skip old containers with explicit scope when current is unscoped", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldScopedContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "some-scope",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldScopedContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})
	})

	t.Run("old containers are found and removed", func(t *testing.T) {
		t.Parallel()

		t.Run("should remove old container in same scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})

		t.Run("should remove old container with matching explicit scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"prod",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})

		t.Run("should skip old container with same ID as current container", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should remove multiple old containers in same scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			old1 := createMockContainer(
				"old-1",
				"watchtower-old-aaa",
				"watchtower:old1",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)
			old2 := createMockContainer(
				"old-2",
				"watchtower-old-bbb",
				"watchtower:old2",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{old1, old2}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old1, 10*time.Minute).
				Return(nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old2, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"prod",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 2, removed)
		})
	})

	t.Run("cleanup images is enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should remove old container and collect its image", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			currentContainer := createMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{currentContainer, oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			var imageInfos []types.RemovedImageInfo

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				true,
				"none",
				"current-id",
				&imageInfos,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
			require.Len(t, imageInfos, 1)
		})

		t.Run("should handle removal failure and return partial count", func(t *testing.T) {
			t.Parallel()

			// The removal retries wait on the synctest clock.
			synctest.Test(t, func(t *testing.T) {
				mockClient := mockContainer.NewMockClient(t)

				oldContainer := createMockContainer(
					"old-id",
					"watchtower-old-abc123",
					"watchtower:old",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{oldContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
					Return(errors.New("stop container failed")).
					Times(maxRemovalAttempts)

				removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
					t.Context(),
					mockClient,
					false,
					"none",
					"current-id",
					nil,
				)

				require.Error(t, err)
				assert.Equal(t, 0, removed)
			})
		})
	})

	t.Run("list containers fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return(nil, errors.New("docker daemon unreachable"))

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to list containers")
			assert.Equal(t, 0, removed)
		})
	})

	t.Run("current container not found in list", func(t *testing.T) {
		t.Parallel()

		t.Run("should still remove old containers with nil current", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"nonexistent-current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})
	})

	t.Run("orphaned created-state containers exist", func(t *testing.T) {
		t.Parallel()

		t.Run("should remove Watchtower containers stuck in created state", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			orphaned := createMockCreatedContainer(
				"orphaned-id",
				"watchtower-abc123",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{orphaned}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, orphaned, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 1, removed)
		})

		t.Run("should skip created-state containers that are not Watchtower", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			nonWT := createMockCreatedContainer(
				"non-wt-id",
				"other-app",
				"other:latest",
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{nonWT}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should skip the current container even if it is in created state", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			current := createMockCreatedContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{current}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})

		t.Run("should remove both old and orphaned created-state containers together", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			orphaned := createMockCreatedContainer(
				"orphaned-id",
				"watchtower-xyz789",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, orphaned}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, orphaned, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 2, removed)
		})

		t.Run("should skip created-state containers from a different scope", func(t *testing.T) {
			t.Parallel()

			mockClient := mockContainer.NewMockClient(t)

			differentScopeOrphaned := createMockCreatedContainer(
				"orphaned-scoped-id",
				"watchtower-scoped",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{differentScopeOrphaned}, nil)

			removed, err := cleanupOldWatchtowerContainers(newTestLogger(t),
				t.Context(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, 0, removed)
		})
	})
}
