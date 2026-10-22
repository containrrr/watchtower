package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_MonitorOnly verifies that monitor-only containers, set by
// label or globally, are not updated and collect no images, and how the
// label-precedence setting resolves the two.
func TestUpdateAction_MonitorOnly(t *testing.T) {
	t.Parallel()

	t.Run("watchtower has been instructed to monitor only", func(t *testing.T) {
		t.Parallel()

		t.Run("certain containers are set to monitor only", func(t *testing.T) {
			t.Parallel()

			t.Run("should not update those containers and collect no image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						NameOfContainerToKeep: "test-container-02",
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image1:latest",
								time.Now(),
							),
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"/test-container-02",
								"fake-image2:latest",
								false,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.monitor-only": "true",
									},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				require.NoError(t, err)
				require.Len(t, report.Updated(), 1)
				assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image1:latest"))
				require.Len(t, cleanupImageInfos, 1)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})
		})

		t.Run("monitor only is set globally", func(t *testing.T) {
			t.Parallel()

			t.Run("should not update any containers and collect no image IDs", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image:latest",
								time.Now(),
							),
							mockActions.CreateMockContainer(
								"test-container-02",
								"test-container-02",
								"fake-image:latest",
								time.Now(),
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true, MonitorOnly: true}, ratelimit.New())

				require.NoError(t, err)
				assert.Empty(t, report.Updated())
				assert.Empty(t, cleanupImageInfos)
				assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
			})

			t.Run("watchtower has been instructed to have label take precedence", func(t *testing.T) {
				t.Parallel()

				t.Run("it should update containers when monitor only is set to false", func(t *testing.T) {
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
											"com.centurylinklabs.watchtower.monitor-only": "false",
										},
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
						Cleanup:         true,
						MonitorOnly:     true,
						LabelPrecedence: true,
					}, ratelimit.New())

					require.NoError(t, err)
					require.Len(t, report.Updated(), 1)
					assert.Contains(t, removedImageIDs(cleanupImageInfos), types.ImageID("fake-image2:latest"))
					require.Len(t, cleanupImageInfos, 1)
					assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
				})

				t.Run("it should not update containers when monitor only is set to true", func(t *testing.T) {
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
											"com.centurylinklabs.watchtower.monitor-only": "true",
										},
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
						Cleanup:         true,
						MonitorOnly:     true,
						LabelPrecedence: true,
					}, ratelimit.New())

					require.NoError(t, err)
					assert.Empty(t, report.Updated())
					assert.Empty(t, cleanupImageInfos)
					assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
				})

				t.Run("it should not update containers when monitor only is not set", func(t *testing.T) {
					t.Parallel()

					client := mockActions.CreateMockClient(
						&mockActions.TestData{
							Containers: []types.Container{
								mockActions.CreateMockContainer(
									"test-container-01",
									"test-container-01",
									"fake-image:latest",
									time.Now(),
								),
							},
						},
						false,
						false,
					)
					client.TestData.Staleness = map[string]bool{
						"test-container-01": true,
					}
					report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
						Cleanup:         true,
						MonitorOnly:     true,
						LabelPrecedence: true,
					}, ratelimit.New())

					require.NoError(t, err)
					assert.Empty(t, report.Updated())
					assert.Empty(t, cleanupImageInfos)
					assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called during Update")
				})
			})
		})
	})
}
