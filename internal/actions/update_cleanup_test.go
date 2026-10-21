package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_Cleanup verifies that an update with cleanup enabled
// collects one image entry per updated container for the caller to remove,
// without removing any image itself.
func TestUpdateAction_Cleanup(t *testing.T) {
	t.Parallel()

	t.Run("watchtower has been instructed to clean up", func(t *testing.T) {
		t.Parallel()

		t.Run("there are multiple containers using the same image", func(t *testing.T) {
			t.Parallel()

			t.Run("should collect one entry per container for deferred cleanup", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(getCommonTestData(), false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
					"test-container-03": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 3)
				// Each container using the same image gets its own entry so that
				// split-by-container notifications report correctly. Image-level
				// deduplication happens later in performImageCleanup.
				require.Len(t, cleanupImageInfos, 3)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image:latest"))
				assert.Contains(t, removedContainerNames(cleanupImageInfos), "test-container-01")
				assert.Contains(t, removedContainerNames(cleanupImageInfos), "test-container-02")
				assert.Contains(t, removedContainerNames(cleanupImageInfos), "test-container-03")
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("there are multiple containers using different images", func(t *testing.T) {
			t.Parallel()

			t.Run("should collect one entry per container for each image", func(t *testing.T) {
				t.Parallel()

				testData := getCommonTestData()
				testData.Containers = append(
					testData.Containers,
					mockActions.CreateMockContainer(
						"unique-test-container",
						"unique-test-container",
						"unique-fake-image:latest",
						time.Now(),
					),
				)
				client := mockActions.CreateMockClient(testData, false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01":     true,
					"test-container-02":     true,
					"test-container-03":     true,
					"unique-test-container": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 4)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image:latest"))
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("unique-fake-image:latest"))
				// 3 containers for fake-image:latest + 1 container for unique-fake-image:latest
				require.Len(t, cleanupImageInfos, 4)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("there are linked containers being updated", func(t *testing.T) {
			t.Parallel()

			t.Run("should collect only the stale container's image ID", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(getLinkedTestData(true), false, false)
				client.TestData.Staleness["test-container-01"] = true
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image1:latest"))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("performing a rolling restart update", func(t *testing.T) {
			t.Parallel()

			t.Run("should collect one entry per container for deferred cleanup", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(getCommonTestData(), false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
					"test-container-03": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true, RollingRestart: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 3)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image:latest"))
				// Each container using the same image gets its own entry for split notifications.
				require.Len(t, cleanupImageInfos, 3)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
				assert.Equal(t, int32(3), client.TestData.WaitForContainerHealthyCount.Load(), "WaitForContainerHealthy should be called for each updated container")
			})
		})

		t.Run("updating a linked container with missing image info", func(t *testing.T) {
			t.Parallel()

			t.Run("should update the stale container and collect its image entry", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(getLinkedTestData(false), false, false)
				client.TestData.Staleness["test-container-01"] = true
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				require.Len(t, report.Fresh(), 1)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image1:latest"))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})
	})
}
