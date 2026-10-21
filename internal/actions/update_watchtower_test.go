package actions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestWatchtowerContainerHandling verifies how an update handles Watchtower's
// own container: the rename and handoff, cleanup, the container chain label,
// scope inheritance, and the SkipSelfUpdate setting.
func TestWatchtowerContainerHandling(t *testing.T) {
	t.Parallel()

	t.Run("updating a Watchtower container", func(t *testing.T) {
		t.Parallel()

		t.Run("should rename and start a new container without cleanup", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos, "No cleanup for renamed Watchtower container")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			assert.Equal(t, int32(1), client.TestData.RenameContainerCount.Load(), "RenameContainer should be called once")
			assert.Equal(t, int32(1), client.TestData.SetRestartPolicyCount.Load(), "SetRestartPolicy should be called once for old Watchtower")
			assert.Equal(t, dockerContainer.RestartPolicyDisabled, client.TestData.LastRestartPolicy.Name)
			assert.Equal(t, int32(0), client.TestData.StopContainerCount.Load(), "StopContainer should not be called for old Watchtower (handled by cleanup logic)")
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called once for Watchtower")
		})

		t.Run("should not collect Watchtower image cleanup during ephemeral self-update", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:             true,
				EphemeralSelfUpdate: true,
				Filter:              filters.WatchtowerContainersFilter,
				PullFailureDelay:    10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos, "Dying process must not collect the Watchtower image for cleanup")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load(), "RenameContainer should not be called for ephemeral self-update")
			assert.True(t, client.TestData.LastCleanup)
		})

		t.Run("should skip rename with no-restart for Watchtower", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":              "true",
									"com.centurylinklabs.watchtower.monitor-only": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			config := types.UpdateParams{
				Cleanup:   true,
				NoRestart: true,
				Filter:    filters.WatchtowerContainersFilter,
			}
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Container should be scanned but not updated")
			assert.Empty(t, report.Updated(), "No containers should be updated with no-restart")
			assert.Empty(t, cleanupImageInfos, "No images should be collected for cleanup")
			assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load(), "RenameContainer should not be called with no-restart")
		})

		t.Run("should stop and recreate non-Watchtower containers with no-restart", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"nginx",
							"/nginx",
							"nginx:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{},
							},
						),
					},
					Staleness: map[string]bool{
						"nginx": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:   true,
				NoRestart: true,
				Filter:    filters.NoFilter,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1, "Non-Watchtower container should be updated with no-restart")
			require.Len(t, cleanupImageInfos, 1, "Cleanup info should be collected for updated container")
			assert.Equal(t, int32(1), client.TestData.StopContainerCount.Load(), "Container should be stopped")
			assert.Equal(t, int32(1), client.TestData.CreateContainerCount.Load(), "Container should be created with no-restart")
			assert.Equal(t, int32(0), client.TestData.StartContainerCount.Load(), "Container should not be started with no-restart")
		})

		t.Run("should not start a stopped container after update without revive-stopped", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"stopped-nginx",
							"/stopped-nginx",
							"nginx:latest",
							false,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{},
							},
						),
					},
					Staleness: map[string]bool{
						"stopped-nginx": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup: true,
				Filter:  filters.NoFilter,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1, "Stopped container should be updated")
			require.Len(t, cleanupImageInfos, 1, "Cleanup info should be collected for updated container")
			assert.Equal(t, int32(1), client.TestData.StopContainerCount.Load(), "Container should be stopped")
			assert.Equal(t, int32(1), client.TestData.CreateContainerCount.Load(), "Container should be created")
			assert.Equal(t, int32(0), client.TestData.StartContainerCount.Load(), "Stopped container should not be started without revive-stopped")
		})

		t.Run("should start a stopped container after update with revive-stopped", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"stopped-nginx",
							"/stopped-nginx",
							"nginx:latest",
							false,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{},
							},
						),
					},
					Staleness: map[string]bool{
						"stopped-nginx": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:       true,
				Filter:        filters.NoFilter,
				ReviveStopped: true,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1, "Stopped container should be updated with revive-stopped")
			require.Len(t, cleanupImageInfos, 1, "Cleanup info should be collected for updated container")
			assert.Equal(t, int32(1), client.TestData.StopContainerCount.Load(), "Container should be stopped")
			assert.Equal(t, int32(1), client.TestData.CreateContainerCount.Load(), "Container should be created")
			assert.Equal(t, int32(1), client.TestData.StartContainerCount.Load(), "Stopped container should be started with revive-stopped")
		})

		t.Run("should not rename Watchtower container in run-once mode", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			config := types.UpdateParams{
				Cleanup: true,
				RunOnce: true,
				Filter:  filters.WatchtowerContainersFilter,
			}
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Container should be scanned")
			assert.Empty(t, report.Updated(), "Watchtower should not be updated in run-once mode")
			assert.Empty(t, cleanupImageInfos, "No images should be collected for cleanup")
			assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load(), "RenameContainer should not be called in run-once mode")
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called once to pull the image")
		})

		t.Run("should not clean up unscoped instances when scope is specified", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower-scoped",
							"/watchtower-scoped",
							"watchtower:latest",
							true,
							false,
							time.Now().Add(-time.Hour),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "prod",
								},
							},
						),
						mockActions.CreateMockContainerWithConfig(
							"watchtower-unscoped",
							"/watchtower-unscoped",
							"watchtower:old",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
				},
				false,
				false,
			)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				client,
				true, // cleanup=true
				"prod",
				&cleanupImageInfos,
				nil, // no current container in this test context
			)
			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Equal(t, int32(0), client.TestData.StopContainerCount.Load(), "StopContainer should not be called for unscoped container")
			assert.Empty(t, cleanupImageInfos, "No cleanup should occur for unscoped container")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called for unscoped container")
		})

		t.Run("should not perform cleanup when currentContainer is nil", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"old",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now().Add(-time.Hour),
							&dockerContainer.Config{
								Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
							},
						),
						mockActions.CreateMockContainerWithConfig(
							"new",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
							},
						),
					},
				},
				false,
				false,
			)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newTestLogger(t),
				t.Context(),
				client,
				true,
				"",
				&cleanupImageInfos,
				nil,
			)
			require.NoError(t, err)
			assert.Equal(t, 0, cleanupOccurred)
			assert.Empty(t, cleanupImageInfos, "No cleanup when no current container")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load())
		})

		t.Run("should accumulate container IDs in container-chain label", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":                 "true",
									"com.centurylinklabs.watchtower.container-chain": "previous-id",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos, "No cleanup for renamed Watchtower container")

			// Check that the container-chain label was updated with the current ID appended
			updatedContainer := client.TestData.Containers[0]
			containerInfo := updatedContainer.ContainerInfo()
			assert.Contains(t, containerInfo.Config.Labels, "com.centurylinklabs.watchtower.container-chain")
			chain := containerInfo.Config.Labels["com.centurylinklabs.watchtower.container-chain"]
			assert.True(t, strings.HasPrefix(chain, "previous-id,"))
			assert.True(t, strings.HasSuffix(chain, string(updatedContainer.ID())))
		})

		t.Run("should inherit scope label during self-update container creation", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "prod",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos)

			// Check that the new container inherits the scope label
			updatedContainer := client.TestData.Containers[0]
			containerInfo := updatedContainer.ContainerInfo()
			assert.Contains(t, containerInfo.Config.Labels, "com.centurylinklabs.watchtower.scope")
			assert.Equal(t, "prod", containerInfo.Config.Labels["com.centurylinklabs.watchtower.scope"])
		})

		t.Run("should preserve scope during container rename operations in self-updates", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "test-scope",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos)
			assert.Equal(t, int32(1), client.TestData.RenameContainerCount.Load())

			// Check that scope label is preserved after rename operation
			updatedContainer := client.TestData.Containers[0]
			containerInfo := updatedContainer.ContainerInfo()
			assert.Contains(t, containerInfo.Config.Labels, "com.centurylinklabs.watchtower.scope")
			assert.Equal(t, "test-scope", containerInfo.Config.Labels["com.centurylinklabs.watchtower.scope"])
		})

		t.Run("should update Watchtower when the scope filter matches its scope label", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "container-scope",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			// Use a scoped filter that matches the container scope
			scopedFilter := filters.FilterByScope(newTestLogger(t),
				"container-scope",
				filters.WatchtowerContainersFilter,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           scopedFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should skip self-update when scope filter conflicts with container label scope", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "container-scope",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			// Use a scoped filter that does not match the container scope
			scopedFilter := filters.FilterByScope(newTestLogger(t),
				"different-scope",
				filters.WatchtowerContainersFilter,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           scopedFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			// Container should not be found by the filter, so no update
			assert.Empty(t, report.Updated())
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should keep the label scope on the new container when the filter selects that scope", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower":       "true",
									"com.centurylinklabs.watchtower.scope": "label-scope",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			// The filter selects the scope in the container's label.
			scopedFilter := filters.FilterByScope(newTestLogger(t), "label-scope", filters.WatchtowerContainersFilter)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				Filter:           scopedFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.Empty(t, cleanupImageInfos)

			// The new container inherits the label scope
			updatedContainer := client.TestData.Containers[0]
			containerInfo := updatedContainer.ContainerInfo()
			assert.Equal(t, "label-scope", containerInfo.Config.Labels["com.centurylinklabs.watchtower.scope"])
		})

		t.Run("container chain and scope interactions", func(t *testing.T) {
			t.Parallel()

			t.Run("should not accumulate chain when scope mismatches with previous container", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"watchtower",
								"/watchtower",
								"watchtower:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower":                 "true",
										"com.centurylinklabs.watchtower.scope":           "scope-a",
										"com.centurylinklabs.watchtower.container-chain": "previous-id",
									},
								},
							),
						},
						Staleness: map[string]bool{
							"watchtower": true,
						},
					},
					false,
					false,
				)
				// Use scope filter that doesn't match the container's scope
				scopedFilter := filters.FilterByScope(newTestLogger(t),
					"scope-b",
					filters.WatchtowerContainersFilter,
				)
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:          true,
					Filter:           scopedFilter,
					PullFailureDelay: 10 * time.Millisecond,
				}, ratelimit.New())

				require.NoError(t, err)
				// Container should not be updated due to scope mismatch
				assert.Empty(t, report.Updated())
				assert.Empty(t, cleanupImageInfos)
			})

			t.Run("should extend the chain when the previous container in it has another scope", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							// Container in scope-a with chain reference to scope-b
							mockActions.CreateMockContainerWithConfig(
								"watchtower-a",
								"/watchtower-a",
								"watchtower:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower":                 "true",
										"com.centurylinklabs.watchtower.scope":           "scope-a",
										"com.centurylinklabs.watchtower.container-chain": "previous-id-scope-b",
									},
								},
							),
							// Previous container in scope-b
							mockActions.CreateMockContainerWithConfig(
								"previous-id-scope-b",
								"/previous-watchtower",
								"watchtower:old",
								false,
								false,
								time.Now().Add(-time.Hour),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower":       "true",
										"com.centurylinklabs.watchtower.scope": "scope-b",
									},
								},
							),
						},
						Staleness: map[string]bool{
							"watchtower-a": true,
						},
					},
					false,
					false,
				)
				// Filter for scope-a only
				scopedFilter := filters.FilterByScope(newTestLogger(t),
					"scope-a",
					filters.WatchtowerContainersFilter,
				)
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:          true,
					Filter:           scopedFilter,
					PullFailureDelay: 10 * time.Millisecond,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.Empty(t, cleanupImageInfos)

				// The chain accumulates regardless of the scope of its earlier IDs.
				updatedContainer := client.TestData.Containers[0]
				containerInfo := updatedContainer.ContainerInfo()
				chainLabel := containerInfo.Config.Labels["com.centurylinklabs.watchtower.container-chain"]
				assert.True(t, strings.HasPrefix(chainLabel, "previous-id-scope-b,"))
			})

			t.Run("should extend a chain spanning several scopes when updating in the selected scope", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							// New container in scope-c
							mockActions.CreateMockContainerWithConfig(
								"watchtower-c",
								"/watchtower-c",
								"watchtower:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower":                 "true",
										"com.centurylinklabs.watchtower.scope":           "scope-c",
										"com.centurylinklabs.watchtower.container-chain": "id-scope-a,id-scope-b",
									},
								},
							),
						},
						Staleness: map[string]bool{
							"watchtower-c": true,
						},
					},
					false,
					false,
				)
				// Filter for scope-c only
				scopedFilter := filters.FilterByScope(newTestLogger(t),
					"scope-c",
					filters.WatchtowerContainersFilter,
				)
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:          true,
					Filter:           scopedFilter,
					PullFailureDelay: 10 * time.Millisecond,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.Empty(t, cleanupImageInfos)

				// Verify chain extends with new ID
				updatedContainer := client.TestData.Containers[0]
				containerInfo := updatedContainer.ContainerInfo()
				chainLabel := containerInfo.Config.Labels["com.centurylinklabs.watchtower.container-chain"]
				assert.True(t, strings.HasPrefix(chainLabel, "id-scope-a,id-scope-b,"))
				assert.True(t, strings.HasSuffix(chainLabel, string(updatedContainer.ID())))
			})

			t.Run("should extend a chain that holds an ID from another scope", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							// Container attempting to chain across scopes
							mockActions.CreateMockContainerWithConfig(
								"watchtower-invalid",
								"/watchtower-invalid",
								"watchtower:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower":                 "true",
										"com.centurylinklabs.watchtower.scope":           "scope-x",
										"com.centurylinklabs.watchtower.container-chain": "id-scope-y", // Different scope
									},
								},
							),
						},
						Staleness: map[string]bool{
							"watchtower-invalid": true,
						},
					},
					false,
					false,
				)
				// Filter for scope-x
				scopedFilter := filters.FilterByScope(newTestLogger(t), "scope-x", filters.WatchtowerContainersFilter)
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:          true,
					Filter:           scopedFilter,
					PullFailureDelay: 10 * time.Millisecond,
				}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.Empty(t, cleanupImageInfos)

				// The chain accumulates regardless of the scope of its earlier IDs.
				updatedContainer := client.TestData.Containers[0]
				containerInfo := updatedContainer.ContainerInfo()
				chainLabel := containerInfo.Config.Labels["com.centurylinklabs.watchtower.container-chain"]
				assert.True(t, strings.HasPrefix(chainLabel, "id-scope-y,"))
				assert.True(t, strings.HasSuffix(chainLabel, string(updatedContainer.ID())))
			})
		})
	})

	t.Run("SkipSelfUpdate functionality", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip staleness check for Watchtower when SkipSelfUpdate=true", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
						mockActions.CreateMockContainerWithConfig(
							"regular",
							"/regular",
							"nginx:latest",
							false,
							false,
							time.Now(),
							&dockerContainer.Config{
								Image:  "nginx:latest",
								Labels: make(map[string]string),
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
						"regular":    true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				SkipSelfUpdate:   true,
				Filter:           filters.NoFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 2)
			require.Len(t, report.Updated(), 1, "Only regular container should be updated")
			require.Len(t, cleanupImageInfos, 1, "Only regular container cleanup")
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called only for regular container")
		})

		t.Run("should not update Watchtower containers when SkipSelfUpdate=true", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				SkipSelfUpdate:   true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1)
			assert.Empty(t, report.Updated(), "Watchtower should not be updated")
			assert.Empty(t, cleanupImageInfos, "No cleanup for skipped Watchtower")
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called for Watchtower")
		})

		t.Run("should skip the Watchtower staleness check and update when SkipSelfUpdate=true and the pull would fail", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			// Simulate pull failure by setting an error on IsContainerStale
			client.TestData.IsContainerStaleError = errors.New("failed to pull image")

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				SkipSelfUpdate:   true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			// Watchtower should not be updated when SkipSelfUpdate=true
			assert.Empty(t, report.Updated(), "Watchtower should not be updated when SkipSelfUpdate=true")
			assert.Empty(t, cleanupImageInfos, "No cleanup should occur for skipped Watchtower")
			// IsContainerStale should NOT be called because staleness check is skipped
			// when SkipSelfUpdate=true for Watchtower containers
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called for Watchtower when SkipSelfUpdate=true")
		})

		t.Run("should still process regular containers when SkipSelfUpdate=true", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
						mockActions.CreateMockContainerWithConfig(
							"regular",
							"/regular",
							"nginx:latest",
							false,
							false,
							time.Now(),
							&dockerContainer.Config{
								Image:  "nginx:latest",
								Labels: make(map[string]string),
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
						"regular":    true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				SkipSelfUpdate:   true,
				Filter:           filters.NoFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			// Only regular container should be updated
			require.Len(t, report.Updated(), 1, "Only regular container should be updated")
			assert.Equal(t, "regular", report.Updated()[0].Name())
			require.Len(t, cleanupImageInfos, 1, "Regular container cleanup")
			// IsContainerStale should be called only for regular container
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called only for regular container")
		})

		t.Run("should check Watchtower staleness and skip the update when the pull fails with SkipSelfUpdate=false", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower",
							"/watchtower",
							"watchtower:latest",
							true,
							false,
							time.Now(),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower": "true",
								},
							},
						),
					},
					Staleness: map[string]bool{
						"watchtower": true,
					},
				},
				false,
				false,
			)
			// Simulate pull failure
			client.TestData.IsContainerStaleError = errors.New("failed to pull image")

			// Note: PullFailureDelay is tested via synctest in TestSafeguardDelay
			// This test verifies that the staleness check is called and fails
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:          true,
				SkipSelfUpdate:   false,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 10 * time.Millisecond,
			}, ratelimit.New())

			require.NoError(t, err)
			// Watchtower should not be updated on pull failure
			assert.Empty(t, report.Updated(), "Watchtower should not be updated on pull failure")
			assert.Empty(t, cleanupImageInfos, "No cleanup should occur on pull failure")
			// IsContainerStale should be called (unlike SkipSelfUpdate=true)
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called when SkipSelfUpdate=false")
		})
	})
}

// TestSafeguardDelay verifies that a failed Watchtower self-update pull delays
// the end of a run-once update, which is restarted at once by a restart
// policy, while a continuous update returns at once and leaves the next
// attempt to the schedule. The synctest clock advances by the time slept.
func TestSafeguardDelay(t *testing.T) {
	t.Parallel()

	const delay = 5 * time.Minute

	tests := []struct {
		name    string
		runOnce bool
		want    time.Duration
	}{
		{name: "continuous update returns at once", runOnce: false, want: 0},
		{name: "run-once update waits", runOnce: true, want: delay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"watchtower",
								"/watchtower",
								"watchtower:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower": "true",
									},
								},
							),
						},
						Staleness: map[string]bool{
							"watchtower": true,
						},
					},
					false,
					false,
				)

				// The Watchtower image pull fails.
				client.TestData.IsContainerStaleError = errors.New("failed to pull image")

				start := time.Now()

				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
					Cleanup:          true,
					Filter:           filters.WatchtowerContainersFilter,
					PullFailureDelay: delay,
					RunOnce:          tt.runOnce,
				}, ratelimit.New())

				require.NoError(t, err)

				assert.Equal(t, tt.want, time.Since(start))
				assert.Empty(t, report.Updated(), "Watchtower is not updated on pull failure")
				assert.Empty(t, cleanupImageInfos, "no image is cleaned up on pull failure")
			})
		})
	}
}

// TestPullFailureDelayContextCancellation verifies that canceling the context
// ends the delay after a failed Watchtower pull, and that nothing is updated or
// collected for cleanup.
func TestPullFailureDelayContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		client := mockActions.CreateMockClientWithContext(
			ctx,
			&mockActions.TestData{
				Containers: []types.Container{
					mockActions.CreateMockContainerWithConfig(
						"watchtower",
						"/watchtower",
						"watchtower:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels: map[string]string{
								"com.centurylinklabs.watchtower": "true",
							},
						},
					),
				},
				Staleness: map[string]bool{
					"watchtower": true, // Simulate stale Watchtower
				},
			},
			false,
			false,
		)

		// Mock IsContainerStale to return an error (simulating pull failure)
		client.TestData.IsContainerStaleError = errors.New("failed to pull image")

		done := make(chan struct{})

		var (
			report            types.Report
			cleanupImageInfos []types.RemovedImageInfo
			err               error
		)

		go func() {
			defer close(done)

			report, cleanupImageInfos, err = update(newTestLogger(t), ctx, client, types.UpdateParams{
				Cleanup:          true,
				Filter:           filters.WatchtowerContainersFilter,
				PullFailureDelay: 100 * time.Millisecond,

				RunOnce: true,
			}, ratelimit.New())
		}()

		// Let the goroutine reach its first blocking point (PullFailureDelay timer)
		// before canceling the context.
		synctest.Wait()
		cancel()

		// Wait for the update to complete
		<-done

		synctest.Wait()

		// The update should complete quickly due to context cancellation
		// We expect either context canceled error or successful completion with no updates
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}

		// report may be nil if context was canceled before update started
		if report != nil {
			if len(report.Updated()) != 0 {
				t.Fatal("Watchtower should not be updated on pull failure")
			}

			if len(cleanupImageInfos) != 0 {
				t.Fatal("No cleanup should occur on pull failure")
			}
		}
	})
}
