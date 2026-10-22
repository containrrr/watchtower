package actions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// defaultTestUpdateParams returns a complete UpdateParams snapshot for tests.
//
// Parameters:
//   - filter: Container filter for the update session.
//
// Returns:
//   - types.UpdateParams: Policy values matching the previous test defaults.
func defaultTestUpdateParams(filter types.Filter) types.UpdateParams {
	return types.UpdateParams{
		Filter:       filter,
		Timeout:      time.Minute,
		LifecycleUID: 1000,
		LifecycleGID: 1001,
	}
}

// TestRunUpdates verifies an update run: the metric it records, the reports
// and notifications it sends, its image cleanup and logs, and its behavior
// with several scopes and when errors occur.
func TestRunUpdates(t *testing.T) {
	t.Parallel()

	t.Run("handleUpdateResult", func(t *testing.T) {
		t.Parallel()

		t.Run("given an error without a report", func(t *testing.T) {
			t.Parallel()

			t.Run("should return a zero metric", func(t *testing.T) {
				t.Parallel()

				err := errors.New("test error")

				metric := handleUpdateResult(newTestLogger(t), nil, err, nil)
				require.NotNil(t, metric)
				assert.Equal(t, 0, metric.Scanned)
				assert.Equal(t, 0, metric.Updated)
				assert.Equal(t, 0, metric.Failed)
			})
		})

		t.Run("given a nil result", func(t *testing.T) {
			t.Parallel()

			t.Run("should return a zero metric", func(t *testing.T) {
				t.Parallel()

				metric := handleUpdateResult(newTestLogger(t), nil, nil, nil)
				require.NotNil(t, metric)
				assert.Equal(t, 0, metric.Scanned)
				assert.Equal(t, 0, metric.Updated)
				assert.Equal(t, 0, metric.Failed)
			})
		})

		t.Run("given a valid result with no error", func(t *testing.T) {
			t.Parallel()

			t.Run("should return nil", func(t *testing.T) {
				t.Parallel()

				result := &mockTypes.MockReport{}
				metric := handleUpdateResult(newTestLogger(t), result, nil, nil)
				assert.Nil(t, metric)
			})
		})
	})

	t.Run("RunUpdatesWithNotifications", func(t *testing.T) {
		t.Parallel()

		t.Run("notifier is nil", func(t *testing.T) {
			t.Parallel()

			t.Run("should not start notification batching", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"test-container",
								"test-container",
								"image:latest",
								time.Now(),
							),
						},
						Staleness: map[string]bool{
							"test-container": false, // Container is not stale
						},
					},
					false,
					false,
				)

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}
				metric := RunUpdatesWithNotifications(t.Context(), params)

				require.NotNil(t, metric)
			})
		})

		t.Run("notifier is provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should handle notification split by container", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container",
								"test-container",
								"image:latest",
								true,
								false,
								time.Now().Add(-time.Hour),
								&dockerContainer.Config{},
							),
						},
						Staleness: map[string]bool{"test-container": true},
					},
					false,
					false,
				)

				notifier := mockTypes.NewMockNotifier(t)
				notifier.EXPECT().StartNotification(true).Return()
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
				notifier.EXPECT().SendNotification(mock.Anything).Return()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client,
					Notifier:                     notifier,
					NotificationSplitByContainer: true,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}
				metric := RunUpdatesWithNotifications(t.Context(), params)

				require.NotNil(t, metric)
				notifier.AssertExpectations(t)
			})
		})

		t.Run("no containers are provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should return zero metric when container list is empty", func(t *testing.T) {
				t.Parallel()

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{},
						Staleness:  map[string]bool{},
					},
					false,
					false,
				)

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}

				// Empty container list results in zero metrics
				metric := RunUpdatesWithNotifications(t.Context(), params)

				require.NotNil(t, metric)
				assert.Equal(t, 0, metric.Scanned)
				assert.Equal(t, 0, metric.Updated)
				assert.Equal(t, 0, metric.Failed)
			})
		})

		t.Run("CurrentContainerID is set", func(t *testing.T) {
			t.Parallel()

			t.Run("should correctly pass CurrentContainerID to UpdateConfig and skip other Watchtower containers", func(t *testing.T) {
				t.Parallel()

				// Create two Watchtower containers with the watchtower label, but only container1 should be updated due to CurrentContainerID
				watchtowerConfig := &dockerContainer.Config{
					Image:  "watchtower:latest",
					Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
				}

				container1 := mockActions.CreateMockContainerWithConfig(
					"container1-id",
					"watchtower-1",
					"watchtower:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					watchtowerConfig,
				)

				container2 := mockActions.CreateMockContainerWithConfig(
					"container2-id",
					"watchtower-2",
					"watchtower:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					watchtowerConfig,
				)

				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{container1, container2},
						Staleness: map[string]bool{
							"watchtower-1": true,
							"watchtower-2": true,
						},
					},
					false,
					false,
				)

				update := defaultTestUpdateParams(filters.NoFilter)
				update.CurrentContainerID = types.ContainerID("container1-id")

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       update,
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)

				require.NotNil(t, metric)
				// Only container1 should be updated, container2 should be skipped due to CurrentContainerID
				require.Len(t, client.TestData.CreateOrder, 1)
				assert.Contains(t, client.TestData.CreateOrder, "watchtower-1")
				assert.NotContains(t, client.TestData.CreateOrder, "watchtower-2")
			})
		})
	})

	t.Run("buildSingleContainerReport", func(t *testing.T) {
		t.Parallel()

		t.Run("should create a SingleContainerReport with updated container", func(t *testing.T) {
			t.Parallel()

			mockContainerReport := mockTypes.NewMockContainerReport(t)
			mockReport := mockTypes.NewMockReport(t)

			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainerReport})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			result := buildSingleContainerReport(mockContainerReport, mockReport)

			require.NotNil(t, result)
			require.Len(t, result.UpdatedReports, 1)
			assert.Same(t, mockContainerReport, result.UpdatedReports[0])
			require.Len(t, result.ScannedReports, 1)
			assert.Empty(t, result.FailedReports)
			assert.Empty(t, result.SkippedReports)
			assert.Empty(t, result.StaleReports)
			assert.Empty(t, result.FreshReports)
		})
	})

	t.Run("buildSingleRestartedContainerReport", func(t *testing.T) {
		t.Parallel()

		t.Run("should create a SingleContainerReport with restarted container", func(t *testing.T) {
			t.Parallel()

			mockContainerReport := mockTypes.NewMockContainerReport(t)
			mockReport := mockTypes.NewMockReport(t)

			mockContainerReport.EXPECT().Error().Return("")
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainerReport})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			result := buildSingleRestartedContainerReport(mockContainerReport, mockReport)

			require.NotNil(t, result)
			require.Len(t, result.RestartedReports, 1)
			assert.Empty(t, result.RestartedReports[0].Error())
			require.Len(t, result.ScannedReports, 1)
			assert.Empty(t, result.FailedReports)
			assert.Empty(t, result.SkippedReports)
			assert.Empty(t, result.StaleReports)
			assert.Empty(t, result.FreshReports)
		})
	})

	t.Run("executeUpdate", func(t *testing.T) {
		t.Parallel()

		t.Run("should execute update and return results", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"test-container",
							"test-container",
							"image:latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{"test-container": true},
				},
				false,
				false,
			)

			config := types.UpdateParams{
				Filter:           filters.NoFilter,
				Cleanup:          false,
				NoRestart:        false,
				MonitorOnly:      false,
				LifecycleHooks:   false,
				RollingRestart:   false,
				LabelPrecedence:  false,
				NoPull:           false,
				Timeout:          time.Minute,
				LifecycleUID:     1000,
				LifecycleGID:     1001,
				PullFailureDelay: time.Duration(0),
				RunOnce:          true,
			}

			// Test that function can be called without panicking
			result, cleanupImages, err := executeUpdate(newTestLogger(t),
				t.Context(),
				client,
				config,
				ratelimit.New(),
				nil,
			)

			// We expect some result or error, but mainly that it doesn't panic
			require.NotNil(t, result)
			require.NotNil(t, cleanupImages)
			// For this mock configuration, we expect no error
			require.NoError(t, err)
		})
	})

	t.Run("startNotifications", func(t *testing.T) {
		t.Parallel()

		t.Run("should start notification when notifier is provided", func(t *testing.T) {
			t.Parallel()

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().StartNotification(false).Return()
			startNotifications(newTestLogger(t), notifier, false)
			notifier.AssertExpectations(t)
		})

		t.Run("should not panic when notifier is nil", func(t *testing.T) {
			t.Parallel()

			startNotifications(newTestLogger(t), nil, false)
		})

		t.Run("should start notification with split enabled", func(t *testing.T) {
			t.Parallel()

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().StartNotification(true).Return()
			startNotifications(newTestLogger(t), notifier, true)
			notifier.AssertExpectations(t)
		})
	})

	t.Run("logUpdateReport", func(t *testing.T) {
		t.Parallel()

		t.Run("should log update report details", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			mockContainer1 := mockTypes.NewMockContainerReport(t)
			mockContainer2 := mockTypes.NewMockContainerReport(t)
			mockContainer3 := mockTypes.NewMockContainerReport(t)

			mockContainer3.EXPECT().Name().Return("container3")

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer3})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})

			// Function just logs, so we verify it doesn't panic and calls expected methods
			logUpdateReport(newTestLogger(t), mockReport)
		})

		t.Run("should handle reports with no updates", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			mockContainer1 := mockTypes.NewMockContainerReport(t)
			mockContainer2 := mockTypes.NewMockContainerReport(t)

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})

			// Should handle empty updated containers gracefully
			logUpdateReport(newTestLogger(t), mockReport)
		})
	})

	t.Run("sendNotifications", func(t *testing.T) {
		t.Parallel()

		t.Run("should not send notifications when notifier is nil", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			sendNotifications(newTestLogger(t), nil, false, false, mockReport, []types.RemovedImageInfo{})
			// No expectations since notifier is nil
		})

		t.Run("should send grouped notification when split is false", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mockReport).Return(true)
			notifier.EXPECT().SendNotification(mockReport).Return()
			sendNotifications(newTestLogger(t), notifier, false, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should call sendSplitNotifications when split is true", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			notifier := mockTypes.NewMockNotifier(t)

			// Set up mock expectations for split notification behavior
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			sendNotifications(newTestLogger(t), notifier, true, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})
	})

	t.Run("sendSplitNotifications", func(t *testing.T) {
		t.Parallel()

		t.Run("should send split notifications for updated containers when notificationReport is true", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("test-id"))
			mockContainer.EXPECT().Name().Return("test-container")
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			// Mock the SendNotification call
			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})

			notifier.AssertExpectations(t)
		})

		t.Run("should handle empty results gracefully", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			notifier := mockTypes.NewMockNotifier(t)

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			// With empty results, no notifications should be sent
			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should send filtered entry notifications when notificationReport is false", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("test-id"))
			mockContainer.EXPECT().Name().Return("test-container")
			mockContainer.EXPECT().ImageName().Return("test-image:latest")
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle restarted containers when notificationReport is true", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("restart-id"))
			mockContainer.EXPECT().Name().Return("restart-container")
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should skip containers with empty names", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("test-id"))
			mockContainer.EXPECT().Name().Return("") // Empty name

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			// No SendNotification should be called due to empty name

			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle monitor-only containers in stale list when notificationReport is true", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("stale-id"))
			mockContainer.EXPECT().Name().Return("stale-container")
			mockContainer.EXPECT().IsMonitorOnly().Return(true)
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle restarted containers when notificationReport is false", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("restart-id"))
			mockContainer.EXPECT().Name().Return("restart-container")
			mockContainer.EXPECT().ImageName().Return("restart-image:latest")
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle restarted containers with empty NewContainerID", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("restart-id"))
			mockContainer.EXPECT().Name().Return("restart-container")
			mockContainer.EXPECT().ImageName().Return("restart-image:latest")
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle monitor-only containers when notificationReport is false", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("monitor-id"))
			mockContainer.EXPECT().Name().Return("monitor-container")
			mockContainer.EXPECT().ImageName().Return("monitor-image:latest")
			mockContainer.EXPECT().IsMonitorOnly().Return(true)
			mockContainer.EXPECT().Error().Return("")

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(newTestLogger(t), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should handle empty container lists gracefully", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			// Return empty slices - all lists are empty
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(t)
			// No SendNotification should be called since all lists are empty

			sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(t)
		})

		t.Run("should emit no cleanup logs when cleanedImages is empty in log mode", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("id-a")).Maybe()
			mockContainer.EXPECT().Name().Return("container-a").Maybe()
			mockContainer.EXPECT().ImageName().Return("nginx:latest").Maybe()
			mockContainer.EXPECT().Error().Return("").Maybe()

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer}).Maybe()
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{}).Maybe()

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			log, buf := newCaptureLogger()
			sendSplitNotifications(log, notifier, false, mockReport, []types.RemovedImageInfo{})

			for _, entry := range parseJSONLogEntries(buf) {
				assert.NotEqual(t, "Removing image", entry["message"])
			}

			notifier.AssertExpectations(t)
		})

		t.Run("should emit container-scoped cleanup fields for each cleanedImages entry in log mode", func(t *testing.T) {
			t.Parallel()

			imageA := types.ImageID("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
			imageB := types.ImageID("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
			cleanedImages := []types.RemovedImageInfo{
				{
					ImageID:       imageA,
					ImageName:     "nginx:old",
					ContainerName: "container-a",
					ContainerID:   types.ContainerID("id-a"),
				},
				{
					ImageID:       imageB,
					ImageName:     "redis:old",
					ContainerName: "container-a",
					ContainerID:   types.ContainerID("id-a"),
				},
				{
					ImageID:       imageA,
					ImageName:     "nginx:old",
					ContainerName: "container-b",
					ContainerID:   types.ContainerID("id-b"),
				},
			}

			mockReport := mockTypes.NewMockReport(t)
			containerA := mockTypes.NewMockContainerReport(t)
			containerB := mockTypes.NewMockContainerReport(t)

			containerA.EXPECT().ID().Return(types.ContainerID("id-a")).Maybe()
			containerA.EXPECT().Name().Return("container-a").Maybe()
			containerA.EXPECT().ImageName().Return("nginx:latest").Maybe()
			containerA.EXPECT().Error().Return("").Maybe()

			containerB.EXPECT().ID().Return(types.ContainerID("id-b")).Maybe()
			containerB.EXPECT().Name().Return("container-b").Maybe()
			containerB.EXPECT().ImageName().Return("nginx:latest").Maybe()
			containerB.EXPECT().Error().Return("").Maybe()

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{containerA, containerB})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{containerA, containerB}).Maybe()
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{}).Maybe()

			// Split path sends one SingleContainerReport-focused notification per updated container.
			var sentFocusNames []string

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true).Times(2)
			notifier.EXPECT().SendNotification(mock.Anything).Run(func(report types.Report) {
				updated := report.Updated()
				require.Len(t, updated, 1)
				sentFocusNames = append(sentFocusNames, updated[0].Name())
			}).Times(2)

			log, buf := newCaptureLogger()
			sendSplitNotifications(log, notifier, false, mockReport, cleanedImages)

			type cleanupLog struct {
				containerName string
				imageName     string
				imageID       string
				notify        string
			}

			asString := func(v any) string {
				s, _ := v.(string)

				return s
			}

			cleanups := make([]cleanupLog, 0)

			for _, entry := range parseJSONLogEntries(buf) {
				if entry["message"] != "Removing image" {
					continue
				}

				cleanups = append(cleanups, cleanupLog{
					containerName: asString(entry["container_name"]),
					imageName:     asString(entry["image_name"]),
					imageID:       asString(entry["image_id"]),
					notify:        asString(entry["notify"]),
				})
			}

			assert.ElementsMatch(t, []cleanupLog{
				{
					containerName: "container-a",
					imageName:     "nginx:old",
					imageID:       imageA.ShortID(),
					notify:        "yes",
				},
				{
					containerName: "container-a",
					imageName:     "redis:old",
					imageID:       imageB.ShortID(),
					notify:        "yes",
				},
				{
					containerName: "container-b",
					imageName:     "nginx:old",
					imageID:       imageA.ShortID(),
					notify:        "yes",
				},
			}, cleanups)
			assert.ElementsMatch(t, []string{"container-a", "container-b"}, sentFocusNames)
			notifier.AssertExpectations(t)
		})

		t.Run("should not emit cleanup logs in report mode", func(t *testing.T) {
			t.Parallel()

			cleanedImages := []types.RemovedImageInfo{
				{
					ImageID:       types.ImageID("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
					ImageName:     "nginx:old",
					ContainerName: "container-a",
					ContainerID:   types.ContainerID("id-a"),
				},
			}

			mockReport := mockTypes.NewMockReport(t)
			mockContainer := mockTypes.NewMockContainerReport(t)

			mockContainer.EXPECT().ID().Return(types.ContainerID("id-a")).Maybe()
			mockContainer.EXPECT().Name().Return("container-a").Maybe()
			mockContainer.EXPECT().ImageName().Return("nginx:latest").Maybe()
			mockContainer.EXPECT().Error().Return("").Maybe()

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer}).Maybe()
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{}).Maybe()
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{}).Maybe()

			notifier := mockTypes.NewMockNotifier(t)
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			log, buf := newCaptureLogger()
			sendSplitNotifications(log, notifier, true, mockReport, cleanedImages)

			for _, entry := range parseJSONLogEntries(buf) {
				assert.NotEqual(t, "Removing image", entry["message"])
			}

			notifier.AssertExpectations(t)
		})
	})

	t.Run("notification level filtering", func(t *testing.T) {
		t.Parallel()

		t.Run("when notification level is error", func(t *testing.T) {
			t.Parallel()

			t.Run("should not send notifications when no errors occur", func(t *testing.T) {
				t.Parallel()

				mockReport := mockTypes.NewMockReport(t)

				// Set up report with no errors - only successful updates
				mockContainer := mockTypes.NewMockContainerReport(t)
				mockContainer.EXPECT().ID().Return(types.ContainerID("test-id"))
				mockContainer.EXPECT().Name().Return("test-container")
				mockContainer.EXPECT().Error().Return("") // No error

				mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
				mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
				mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
				mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
				mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
				mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
				mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

				notifier := mockTypes.NewMockNotifier(t)
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(false)
				// With notification level set to error and no errors, no notification should be sent

				sendSplitNotifications(newTestLogger(t), notifier, true, mockReport, []types.RemovedImageInfo{})
				notifier.AssertExpectations(t)
			})
		})
	})

	t.Run("performImageCleanup", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty slice when cleanup is disabled", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(newTestLogger(t), t.Context(), client, false, []types.RemovedImageInfo{}, 0)
			assert.Empty(t, cleanedImages)
		})

		t.Run("should perform cleanup when cleanup is enabled", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(newTestLogger(t), t.Context(), client, true, []types.RemovedImageInfo{
				{
					ContainerName: "test-container",
					ImageName:     "test-image:v1.0",
					ImageID:       types.ImageID("sha256:123"),
				},
			}, 0)
			// The function should return the cleaned images when cleanup is enabled
			require.Len(t, cleanedImages, 1)
			assert.Equal(t, "test-container", cleanedImages[0].ContainerName)
		})

		t.Run("should return a valid slice when cleanup input is empty", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(newTestLogger(t), t.Context(), client, true, []types.RemovedImageInfo{}, 0)
			// Should return a valid slice even with empty input
			require.NotNil(t, cleanedImages)
		})

		t.Run("should still remove images when the parent context is already canceled", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			cleanedImages := performImageCleanup(newTestLogger(t), ctx, client, true, []types.RemovedImageInfo{
				{
					ContainerName: "test-container",
					ImageName:     "test-image:v1.0",
					ImageID:       types.ImageID("sha256:123"),
				},
			}, 0)
			require.Len(t, cleanedImages, 1)
			assert.Equal(t, int32(1), client.TestData.TriedToRemoveImageCount.Load())
		})
	})

	t.Run("generateAndLogMetric", func(t *testing.T) {
		t.Parallel()

		t.Run("should generate metric from report", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			mockContainer1 := mockTypes.NewMockContainerReport(t)
			mockContainer2 := mockTypes.NewMockContainerReport(t)
			mockContainer3 := mockTypes.NewMockContainerReport(t)
			mockContainer4 := mockTypes.NewMockContainerReport(t)

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer3})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{mockContainer4})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})

			metric := generateAndLogMetric(newTestLogger(t), mockReport)

			require.NotNil(t, metric)
			assert.Equal(t, 2, metric.Scanned)
			assert.Equal(t, 1, metric.Updated)
			assert.Equal(t, 1, metric.Failed)
			assert.Equal(t, 0, metric.Restarted)
		})

		t.Run("should handle empty report gracefully", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})

			metric := generateAndLogMetric(newTestLogger(t), mockReport)

			require.NotNil(t, metric)
			assert.Equal(t, 0, metric.Scanned)
			assert.Equal(t, 0, metric.Updated)
			assert.Equal(t, 0, metric.Failed)
			assert.Equal(t, 0, metric.Restarted)
		})

		t.Run("should log session completion with notify=no so it is not sent as a notification", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)

			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})

			log, buf := newCaptureLogger()
			_ = generateAndLogMetric(log, mockReport)

			var found bool

			for _, entry := range parseJSONLogEntries(buf) {
				if entry["message"] != "Update session completed" {
					continue
				}

				found = true

				assert.Equal(t, "no", entry["notify"])
			}

			assert.True(t, found)
		})

		t.Run("should log skipped count on the session completion line", func(t *testing.T) {
			t.Parallel()

			mockReport := mockTypes.NewMockReport(t)
			skippedA := mockTypes.NewMockContainerReport(t)
			skippedB := mockTypes.NewMockContainerReport(t)

			mockReport.EXPECT().Scanned().Return(make([]types.ContainerReport, 23))
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{skippedA, skippedB})

			log, buf := newCaptureLogger()
			metric := generateAndLogMetric(log, mockReport)

			assert.Equal(t, 2, metric.Skipped)
			assert.Equal(t, 23, metric.Scanned)

			var found bool

			for _, entry := range parseJSONLogEntries(buf) {
				if entry["message"] != "Update session completed" {
					continue
				}

				found = true

				assert.EqualValues(t, 2, entry["skipped"])
				assert.EqualValues(t, 23, entry["scanned"])
				assert.EqualValues(t, 0, entry["updated"])
				assert.EqualValues(t, 0, entry["failed"])
			}

			assert.True(t, found)
		})
	})

	t.Run("Multiple Concurrent Scopes", func(t *testing.T) {
		t.Parallel()

		t.Run("should support multiple Watchtower instances with different scopes running simultaneously", func(t *testing.T) {
			t.Parallel()

			// Create containers with different scopes
			scopeAContainer := mockActions.CreateMockContainerWithConfig(
				"scope-a-container",
				"scope-a-app",
				"app:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-a",
					},
				},
			)

			scopeBContainer := mockActions.CreateMockContainerWithConfig(
				"scope-b-container",
				"scope-b-app",
				"app:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-b",
					},
				},
			)

			// Create separate container lists for each client
			containersA := []types.Container{scopeAContainer}
			containersB := []types.Container{scopeBContainer}

			// Create separate test data for each client
			testDataA := &mockActions.TestData{
				Containers: containersA,
				Staleness: map[string]bool{
					"scope-a-app": true,
				},
			}

			testDataB := &mockActions.TestData{
				Containers: containersB,
				Staleness: map[string]bool{
					"scope-b-app": true,
				},
			}

			// Create mock clients for each scope
			clientA := mockActions.CreateMockClient(testDataA, false, false)
			clientB := mockActions.CreateMockClient(testDataB, false, false)

			// Results channel to collect metrics from concurrent operations
			results := make(chan *metrics.Metric, 2)

			var wg sync.WaitGroup

			// Launch concurrent updates for different scopes
			wg.Add(2)

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientA,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "scope-a", filters.NoFilter),
					),
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)
				results <- metric
			}()

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientB,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "scope-b", filters.NoFilter),
					),
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)
				results <- metric
			}()

			// Wait for all goroutines to complete
			wg.Wait()
			close(results)

			// Collect results
			var metrics []*metrics.Metric
			for metric := range results {
				metrics = append(metrics, metric)
			}

			// Verify we got results from both concurrent operations
			require.Len(t, metrics, 2)

			for _, metric := range metrics {
				require.NotNil(t, metric)
				assert.Equal(t, 1, metric.Scanned)
				// Each scope should scan 1 container
				assert.Equal(t, 1, metric.Updated)
				// Each scope should update 1 container
			}
		})

		t.Run("should ensure scope isolation and prevent interference between concurrent scopes", func(t *testing.T) {
			t.Parallel()

			// Create containers for different scopes
			scope1Container := mockActions.CreateMockContainerWithConfig(
				"scope1-watchtower",
				"scope1-watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope1",
					},
				},
			)

			scope2Container := mockActions.CreateMockContainerWithConfig(
				"scope2-watchtower",
				"scope2-watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope2",
					},
				},
			)

			allContainers := []types.Container{scope1Container, scope2Container}

			// Create separate test data for each client to track operations independently
			testData1 := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"scope1-watchtower": true,
					"scope2-watchtower": false, // scope2 container not stale for scope1 filter
				},
			}

			testData2 := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"scope1-watchtower": false, // scope1 container not stale for scope2 filter
					"scope2-watchtower": true,
				},
			}

			client1 := mockActions.CreateMockClient(testData1, false, false)
			client2 := mockActions.CreateMockClient(testData2, false, false)

			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client1,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "scope1", filters.WatchtowerContainersFilter),
					),
				}
				RunUpdatesWithNotifications(t.Context(), params)
			}()

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client2,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "scope2", filters.WatchtowerContainersFilter),
					),
				}
				RunUpdatesWithNotifications(t.Context(), params)
			}()

			wg.Wait()

			// Verify scope isolation through separate TestData tracking
			// client1 should only create scope1-watchtower
			assert.Contains(t, client1.TestData.CreateOrder, "scope1-watchtower")
			assert.NotContains(t, client1.TestData.CreateOrder, "scope2-watchtower")

			// client2 should only create scope2-watchtower
			assert.Contains(t, client2.TestData.CreateOrder, "scope2-watchtower")
			assert.NotContains(t, client2.TestData.CreateOrder, "scope1-watchtower")
		})

		t.Run("should maintain resource cleanup isolation respecting scope boundaries", func(t *testing.T) {
			t.Parallel()

			// Create Watchtower containers for different scopes
			scopeAWatchtower := mockActions.CreateMockContainerWithConfig(
				"watchtower-a",
				"watchtower-a",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-a",
					},
				},
			)

			scopeBWatchtower := mockActions.CreateMockContainerWithConfig(
				"watchtower-b",
				"watchtower-b",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-b",
					},
				},
			)

			allContainers := []types.Container{scopeAWatchtower, scopeBWatchtower}

			// Create separate test data for each client to track cleanup independently
			testDataA := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"watchtower-a": true,
					"watchtower-b": false, // scope-b container not processed by scope-a filter
				},
			}

			testDataB := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"watchtower-a": false, // scope-a container not processed by scope-b filter
					"watchtower-b": true,
				},
			}

			clientA := mockActions.CreateMockClient(testDataA, false, false)
			clientB := mockActions.CreateMockClient(testDataB, false, false)

			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()

				updateA := defaultTestUpdateParams(
					filters.FilterByScope(newTestLogger(t), "scope-a", filters.WatchtowerContainersFilter),
				)
				updateA.Cleanup = true

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientA,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateA,
				}
				RunUpdatesWithNotifications(t.Context(), params)
			}()

			go func() {
				defer wg.Done()

				updateB := defaultTestUpdateParams(
					filters.FilterByScope(newTestLogger(t), "scope-b", filters.WatchtowerContainersFilter),
				)
				updateB.Cleanup = true

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientB,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateB,
				}
				RunUpdatesWithNotifications(t.Context(), params)
			}()

			wg.Wait()

			// Verify cleanup isolation - each client updates only the Watchtower container in its scope
			assert.Contains(t, clientA.TestData.CreateOrder, "watchtower-a")
			assert.NotContains(t, clientA.TestData.CreateOrder, "watchtower-b")
			assert.Contains(t, clientB.TestData.CreateOrder, "watchtower-b")
			assert.NotContains(t, clientB.TestData.CreateOrder, "watchtower-a")

			// Neither client removes an image, because the new instance of a renamed
			// Watchtower container cleans up its old image.
			assert.Equal(t, int32(0), clientA.TestData.TriedToRemoveImageCount.Load())
			assert.Equal(t, int32(0), clientB.TestData.TriedToRemoveImageCount.Load())
		})

		t.Run("should handle concurrent operations with different scopes without interference", func(t *testing.T) {
			t.Parallel()

			// Create mixed containers - regular apps and watchtower instances
			appContainer := mockActions.CreateMockContainerWithConfig(
				"app-1",
				"my-app",
				"myapp:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "prod", // This app belongs to prod scope
					},
				},
			)

			watchtowerProd := mockActions.CreateMockContainerWithConfig(
				"wt-prod",
				"watchtower-prod",
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
			)

			watchtowerDev := mockActions.CreateMockContainerWithConfig(
				"wt-dev",
				"watchtower-dev",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "dev",
					},
				},
			)

			allContainers := []types.Container{appContainer, watchtowerProd, watchtowerDev}

			testDataProd := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"my-app":          true,
					"watchtower-prod": true,
					"watchtower-dev":  true,
				},
			}

			testDataDev := &mockActions.TestData{
				Containers: allContainers,
				Staleness: map[string]bool{
					"my-app":          true,
					"watchtower-prod": true,
					"watchtower-dev":  true,
				},
			}

			clientProd := mockActions.CreateMockClient(testDataProd, false, false)
			clientDev := mockActions.CreateMockClient(testDataDev, false, false)

			results := make(chan map[string]int, 2)

			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientProd,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "prod", filters.NoFilter),
					),
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)
				results <- map[string]int{"prod-scanned": metric.Scanned, "prod-updated": metric.Updated}
			}()

			go func() {
				defer wg.Done()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientDev,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "dev", filters.NoFilter),
					),
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)
				results <- map[string]int{"dev-scanned": metric.Scanned, "dev-updated": metric.Updated}
			}()

			wg.Wait()
			close(results)

			// Collect results
			var resultMaps []map[string]int
			for resultMap := range results {
				resultMaps = append(resultMaps, resultMap)
			}

			require.Len(t, resultMaps, 2)

			// Verify concurrent operations completed independently
			totalScanned := 0
			totalUpdated := 0

			for _, resultMap := range resultMaps {
				for key, value := range resultMap {
					if key == "prod-scanned" || key == "dev-scanned" {
						totalScanned += value
					}

					if key == "prod-updated" || key == "dev-updated" {
						totalUpdated += value
					}
				}
			}

			// prod scans my-app and watchtower-prod, and dev scans watchtower-dev
			assert.Equal(t, 3, totalScanned)
			// prod updates my-app and watchtower-prod, and dev updates watchtower-dev
			assert.Equal(t, 3, totalUpdated)
		})
	})

	t.Run("Scoped Environment Error Reporting and State Consistency", func(t *testing.T) {
		t.Parallel()

		t.Run("scoped operations encounter errors", func(t *testing.T) {
			t.Parallel()

			t.Run("should maintain state consistency when scoped update operations fail partially", func(t *testing.T) {
				t.Parallel()

				// Create containers in same scope with one failing update
				scopeAContainer1 := mockActions.CreateMockContainerWithConfig(
					"scope-a-1",
					"app-a-1",
					"app:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.scope": "scope-a",
						},
					},
				)

				scopeAContainer2 := mockActions.CreateMockContainerWithConfig(
					"scope-a-2",
					"app-a-2",
					"app:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.scope": "scope-a",
						},
					},
				)

				allContainers := []types.Container{scopeAContainer1, scopeAContainer2}

				// Set up mock to fail updates
				testData := &mockActions.TestData{
					Containers: allContainers,
					Staleness:  map[string]bool{"app-a-1": true, "app-a-2": true},
					StopContainerError: errors.New(
						"container stop failed",
					), // Fail during update process
					StopContainerFailCount: 2, // Fail both containers
				}

				client := mockActions.CreateMockClient(testData, false, false)

				params := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       client,
					Notifier:                     nil, // No notifications for this test
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update: defaultTestUpdateParams(
						filters.FilterByScope(newTestLogger(t), "scope-a", filters.NoFilter),
					),
				}

				metric := RunUpdatesWithNotifications(t.Context(), params)

				// Verify state consistency: partial failures are properly tracked
				require.NotNil(t, metric)
				assert.Equal(t, 2, metric.Scanned)
				// Both containers in scope scanned
				assert.Equal(t, 2, metric.Failed)  // Both fail due to mock error
				assert.Equal(t, 0, metric.Updated) // No successful updates
			})

			t.Run("should properly isolate scoped operations when cleanup fails", func(t *testing.T) {
				t.Parallel()

				// Test that cleanup failures in one scope don't affect other scopes
				scopeAContainer := mockActions.CreateMockContainerWithConfig(
					"cleanup-a",
					"cleanup-app-a",
					"app:v1",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower":       "true",
							"com.centurylinklabs.watchtower.scope": "scope-a",
						},
					},
				)

				scopeBContainer := mockActions.CreateMockContainerWithConfig(
					"cleanup-b",
					"cleanup-app-b",
					"app:v2",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower":       "true",
							"com.centurylinklabs.watchtower.scope": "scope-b",
						},
					},
				)

				allContainers := []types.Container{scopeAContainer, scopeBContainer}

				// Different test data for different scopes
				testDataA := &mockActions.TestData{
					Containers: allContainers,
					Staleness: map[string]bool{
						"cleanup-app-a": true,
						"cleanup-app-b": false,
					},
					RemoveImageError: errors.New("cleanup failed in scope-a"),
					FailedImageIDs: []types.ImageID{
						types.ImageID("app:v1"),
					}, // Fail cleanup for scope-a
				}

				testDataB := &mockActions.TestData{
					Containers: allContainers,
					Staleness:  map[string]bool{"cleanup-app-a": false, "cleanup-app-b": true},
					// scope-b cleanup succeeds
				}

				clientA := mockActions.CreateMockClient(testDataA, false, false)
				clientB := mockActions.CreateMockClient(testDataB, false, false)

				// Run scope-a operation (cleanup should fail)
				updateA := defaultTestUpdateParams(
					filters.FilterByScope(newTestLogger(t), "scope-a", filters.NoFilter),
				)
				updateA.Cleanup = true

				paramsA := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientA,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateA,
				}

				// Run scope-b operation (cleanup should succeed)
				updateB := defaultTestUpdateParams(
					filters.FilterByScope(newTestLogger(t), "scope-b", filters.NoFilter),
				)
				updateB.Cleanup = true

				paramsB := RunUpdatesWithNotificationsParams{
					Logger:                       newTestLogger(t),
					Client:                       clientB,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateB,
				}

				var wg sync.WaitGroup

				results := make(chan *metrics.Metric, 2)

				wg.Add(2)

				go func() {
					defer wg.Done()

					metric := RunUpdatesWithNotifications(t.Context(), paramsA)
					results <- metric
				}()

				go func() {
					defer wg.Done()

					metric := RunUpdatesWithNotifications(t.Context(), paramsB)
					results <- metric
				}()

				wg.Wait()
				close(results)

				// Collect results
				var metrics []*metrics.Metric
				for metric := range results {
					metrics = append(metrics, metric)
				}

				// Verify scope isolation in error handling
				require.Len(t, metrics, 2)

				for _, metric := range metrics {
					require.NotNil(t, metric)
					// Each scope processes exactly one container
					assert.Equal(t, 1, metric.Scanned)
					assert.Equal(t, 1, metric.Updated) // Updates succeed
					assert.Equal(t, 0, metric.Failed)  // No update failures
				}
			})

			t.Run("should ensure proper error reporting isolation between concurrent scoped operations", func(t *testing.T) {
				t.Parallel()

				// Test concurrent scoped operations with different error patterns
				containerX := mockActions.CreateMockContainerWithConfig(
					"concurrent-x",
					"app-x",
					"app:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.scope": "scope-x",
						},
					},
				)

				containerY := mockActions.CreateMockContainerWithConfig(
					"concurrent-y",
					"app-y",
					"app:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.scope": "scope-y",
						},
					},
				)

				allContainers := []types.Container{containerX, containerY}

				// scope-x has update errors, scope-y succeeds
				testDataX := &mockActions.TestData{
					Containers:             allContainers,
					Staleness:              map[string]bool{"app-x": true, "app-y": false},
					StopContainerError:     errors.New("scope-x update failure"),
					StopContainerFailCount: 1, // Fail once
				}

				testDataY := &mockActions.TestData{
					Containers: allContainers,
					Staleness:  map[string]bool{"app-x": false, "app-y": true},
					// scope-y succeeds
				}

				clientX := mockActions.CreateMockClient(testDataX, false, false)
				clientY := mockActions.CreateMockClient(testDataY, false, false)

				// Run concurrent operations
				var wg sync.WaitGroup

				results := make(chan *metrics.Metric, 2)

				wg.Add(2)

				go func() {
					defer wg.Done()

					params := RunUpdatesWithNotificationsParams{
						Logger:                       newTestLogger(t),
						Client:                       clientX,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(newTestLogger(t), "scope-x", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(t.Context(), params)
					results <- metric
				}()

				go func() {
					defer wg.Done()

					params := RunUpdatesWithNotificationsParams{
						Logger:                       newTestLogger(t),
						Client:                       clientY,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(newTestLogger(t), "scope-y", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(t.Context(), params)
					results <- metric
				}()

				wg.Wait()
				close(results)

				// Collect results
				var metrics []*metrics.Metric
				for metric := range results {
					metrics = append(metrics, metric)
				}

				// Verify proper isolation of error reporting
				require.Len(t, metrics, 2)

				totalScanned := 0
				totalFailed := 0
				totalUpdated := 0

				for _, metric := range metrics {
					require.NotNil(t, metric)
					assert.Equal(t, 1, metric.Scanned)
					// Each scope sees 1 container
					totalScanned += metric.Scanned
					totalFailed += metric.Failed
					totalUpdated += metric.Updated
				}

				assert.Equal(t, 2, totalScanned) // Both containers processed
				assert.Equal(t, 1, totalFailed)  // One scope failed
				assert.Equal(t, 1, totalUpdated) // One scope succeeded
			})
		})
	})
}
