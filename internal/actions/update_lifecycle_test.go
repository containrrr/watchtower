package actions

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_LifecycleHooks verifies how pre-update hook exit codes,
// hook user and group IDs, and container state decide whether an update runs
// the hooks and proceeds.
func TestUpdateAction_LifecycleHooks(t *testing.T) {
	t.Parallel()

	t.Run("watchtower has been instructed to run lifecycle hooks", func(t *testing.T) {
		t.Parallel()

		t.Run("pre-update script returns 1", func(t *testing.T) {
			t.Parallel()

			t.Run("should not update those containers and collect no image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
				}, ratelimit.New())

				require.NoError(t, err)
				assert.Empty(t, report.Updated())
				assert.Empty(t, cleanupImageInfos)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("lifecycle UID and GID are specified", func(t *testing.T) {
			t.Parallel()

			t.Run("should pass UID and GID to lifecycle hook execution", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-uid-gid",
								"test-container-uid-gid",
								"fake-image:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update": "/PreUpdateReturn0.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-uid-gid": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
					LifecycleUID:   1000,
					LifecycleGID:   1001,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image:latest") }))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
				assert.Equal(t, 1000, client.TestData.LastExecUID, "the hook runs as the configured UID")
				assert.Equal(t, 1001, client.TestData.LastExecGID, "the hook runs as the configured GID")
			})
		})

		t.Run("preupdate script returns 75", func(t *testing.T) {
			t.Parallel()

			t.Run("should skip those containers and collect no image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn75.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
				}, ratelimit.New())

				require.NoError(t, err)
				assert.Empty(t, report.Updated())
				assert.Empty(t, report.Failed())
				require.Len(t, report.Skipped(), 1)
				assert.Empty(t, cleanupImageInfos)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("preupdate script returns 0", func(t *testing.T) {
			t.Parallel()

			t.Run("should update those containers and collect image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn0.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image2:latest") }))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("container is not running", func(t *testing.T) {
			t.Parallel()

			t.Run("should skip running preupdate and collect image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								false,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image2:latest") }))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("container is restarting", func(t *testing.T) {
			t.Parallel()

			t.Run("should skip running preupdate and collect image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								false,
								true,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:        true,
					LifecycleHooks: true,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image2:latest") }))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})
	})
}
