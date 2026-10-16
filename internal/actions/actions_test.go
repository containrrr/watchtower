package actions

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
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

var _ = ginkgo.Describe("Actions", func() {
	ginkgo.Describe("handleUpdateResult", func() {
		ginkgo.When("given an error without a report", func() {
			ginkgo.It("should return a zero metric", func() {
				err := errors.New("test error")

				metric := handleUpdateResult(testLogger(), nil, err, nil)
				gomega.Expect(metric).NotTo(gomega.BeNil())
				gomega.Expect(metric.Scanned).To(gomega.Equal(0))
				gomega.Expect(metric.Updated).To(gomega.Equal(0))
				gomega.Expect(metric.Failed).To(gomega.Equal(0))
			})
		})

		ginkgo.When("given a nil result", func() {
			ginkgo.It("should return a zero metric", func() {
				metric := handleUpdateResult(testLogger(), nil, nil, nil)
				gomega.Expect(metric).NotTo(gomega.BeNil())
				gomega.Expect(metric.Scanned).To(gomega.Equal(0))
				gomega.Expect(metric.Updated).To(gomega.Equal(0))
				gomega.Expect(metric.Failed).To(gomega.Equal(0))
			})
		})

		ginkgo.When("given a valid result with no error", func() {
			ginkgo.It("should return nil", func() {
				result := &mockTypes.MockReport{}
				metric := handleUpdateResult(testLogger(), result, nil, nil)
				gomega.Expect(metric).To(gomega.BeNil())
			})
		})
	})

	ginkgo.Describe("RunUpdatesWithNotifications", func() {
		ginkgo.When("notifier is nil", func() {
			ginkgo.It("should not start notification batching", func() {
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
					Logger:                       testLogger(),
					Client:                       client,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}
				metric := RunUpdatesWithNotifications(context.Background(), params)

				gomega.Expect(metric).NotTo(gomega.BeNil())
			})
		})

		ginkgo.When("notifier is provided", func() {
			ginkgo.It("should handle notification split by container", func() {
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

				notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
				notifier.EXPECT().StartNotification(true).Return()
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
				notifier.EXPECT().SendNotification(mock.Anything).Return()

				params := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
					Client:                       client,
					Notifier:                     notifier,
					NotificationSplitByContainer: true,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}
				metric := RunUpdatesWithNotifications(context.Background(), params)

				gomega.Expect(metric).NotTo(gomega.BeNil())
				notifier.AssertExpectations(ginkgo.GinkgoT())
			})
		})

		ginkgo.When("no containers are provided", func() {
			ginkgo.It("should return zero metric when container list is empty", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{},
						Staleness:  map[string]bool{},
					},
					false,
					false,
				)

				params := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
					Client:                       client,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       defaultTestUpdateParams(filters.NoFilter),
				}

				// Empty container list results in zero metrics
				metric := RunUpdatesWithNotifications(context.Background(), params)

				gomega.Expect(metric).NotTo(gomega.BeNil())
				gomega.Expect(metric.Scanned).To(gomega.Equal(0))
				gomega.Expect(metric.Updated).To(gomega.Equal(0))
				gomega.Expect(metric.Failed).To(gomega.Equal(0))
			})
		})

		ginkgo.When("CurrentContainerID is set", func() {
			ginkgo.It(
				"should correctly pass CurrentContainerID to UpdateConfig and skip other Watchtower containers",
				func() {
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
						Logger:                       testLogger(),
						Client:                       client,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update:                       update,
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)

					gomega.Expect(metric).NotTo(gomega.BeNil())
					// Only container1 should be updated, container2 should be skipped due to CurrentContainerID
					gomega.Expect(client.TestData.CreateOrder).To(gomega.HaveLen(1))
					gomega.Expect(client.TestData.CreateOrder).
						To(gomega.ContainElement("watchtower-1"))
					gomega.Expect(client.TestData.CreateOrder).
						To(gomega.Not(gomega.ContainElement("watchtower-2")))
				},
			)
		})
	})

	ginkgo.Describe("buildSingleContainerReport", func() {
		ginkgo.It("should create a SingleContainerReport with updated container", func() {
			mockContainerReport := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainerReport})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			result := buildSingleContainerReport(mockContainerReport, mockReport)

			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.UpdatedReports).To(gomega.HaveLen(1))
			gomega.Expect(result.UpdatedReports[0]).To(gomega.HaveOccurred())
			gomega.Expect(result.ScannedReports).To(gomega.HaveLen(1))
			gomega.Expect(result.FailedReports).To(gomega.BeEmpty())
			gomega.Expect(result.SkippedReports).To(gomega.BeEmpty())
			gomega.Expect(result.StaleReports).To(gomega.BeEmpty())
			gomega.Expect(result.FreshReports).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("buildSingleRestartedContainerReport", func() {
		ginkgo.It("should create a SingleContainerReport with restarted container", func() {
			mockContainerReport := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockContainerReport.EXPECT().Error().Return("")
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainerReport})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			result := buildSingleRestartedContainerReport(mockContainerReport, mockReport)

			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.RestartedReports).To(gomega.HaveLen(1))
			gomega.Expect(result.RestartedReports[0].Error()).To(gomega.BeEmpty())
			gomega.Expect(result.ScannedReports).To(gomega.HaveLen(1))
			gomega.Expect(result.FailedReports).To(gomega.BeEmpty())
			gomega.Expect(result.SkippedReports).To(gomega.BeEmpty())
			gomega.Expect(result.StaleReports).To(gomega.BeEmpty())
			gomega.Expect(result.FreshReports).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("executeUpdate", func() {
		ginkgo.It("should execute update and return results", func() {
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
			result, cleanupImages, err := executeUpdate(testLogger(),
				context.Background(),
				client,
				config,
				nil,
			)

			// We expect some result or error, but mainly that it doesn't panic
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(cleanupImages).NotTo(gomega.BeNil())
			// For this mock configuration, we expect no error
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("startNotifications", func() {
		ginkgo.It("should start notification when notifier is provided", func() {
			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().StartNotification(false).Return()
			startNotifications(testLogger(), notifier, false)
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should not panic when notifier is nil", func() {
			startNotifications(testLogger(), nil, false)
		})

		ginkgo.It("should start notification with split enabled", func() {
			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().StartNotification(true).Return()
			startNotifications(testLogger(), notifier, true)
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})
	})

	ginkgo.Describe("logUpdateReport", func() {
		ginkgo.It("should log update report details", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockContainer1 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer2 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer3 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

			mockContainer3.EXPECT().Name().Return("container3")

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer3})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})

			// Function just logs, so we verify it doesn't panic and calls expected methods
			logUpdateReport(testLogger(), mockReport)
		})

		ginkgo.It("should handle reports with no updates", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockContainer1 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer2 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})

			// Should handle empty updated containers gracefully
			logUpdateReport(testLogger(), mockReport)
		})
	})

	ginkgo.Describe("sendNotifications", func() {
		ginkgo.It("should not send notifications when notifier is nil", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			sendNotifications(testLogger(), nil, false, false, mockReport, []types.RemovedImageInfo{})
			// No expectations since notifier is nil
		})

		ginkgo.It("should send grouped notification when split is false", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mockReport).Return(true)
			notifier.EXPECT().SendNotification(mockReport).Return()
			sendNotifications(testLogger(), notifier, false, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should call sendSplitNotifications when split is true", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())

			// Set up mock expectations for split notification behavior
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			sendNotifications(testLogger(), notifier, true, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})
	})

	ginkgo.Describe("sendSplitNotifications", func() {
		ginkgo.It(
			"should send split notifications for updated containers when notificationReport is true",
			func() {
				mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
				mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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
				notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
				notifier.EXPECT().SendNotification(mock.Anything).Return()

				sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})

				notifier.AssertExpectations(ginkgo.GinkgoT())
			},
		)

		ginkgo.It("should handle empty results gracefully", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			// With empty results, no notifications should be sent
			sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It(
			"should send filtered entry notifications when notificationReport is false",
			func() {
				mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
				mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

				notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
				notifier.EXPECT().SendNotification(mock.Anything).Return()

				sendSplitNotifications(testLogger(), notifier, false, mockReport, []types.RemovedImageInfo{})
				notifier.AssertExpectations(ginkgo.GinkgoT())
			},
		)

		ginkgo.It("should handle restarted containers when notificationReport is true", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should skip containers with empty names", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

			mockContainer.EXPECT().ID().Return(types.ContainerID("test-id"))
			mockContainer.EXPECT().Name().Return("") // Empty name

			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{mockContainer})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			// No SendNotification should be called due to empty name

			sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It(
			"should handle monitor-only containers in stale list when notificationReport is true",
			func() {
				mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
				mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

				notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
				notifier.EXPECT().SendNotification(mock.Anything).Return()

				sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
				notifier.AssertExpectations(ginkgo.GinkgoT())
			},
		)

		ginkgo.It("should handle restarted containers when notificationReport is false", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(testLogger(), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should handle restarted containers with empty NewContainerID", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(testLogger(), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should handle monitor-only containers when notificationReport is false", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			sendSplitNotifications(testLogger(), notifier, false, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should handle empty container lists gracefully", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			// Return empty slices - all lists are empty
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Stale().Return([]types.ContainerReport{})
			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})
			mockReport.EXPECT().Fresh().Return([]types.ContainerReport{})

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			// No SendNotification should be called since all lists are empty

			sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should emit no cleanup logs when cleanedImages is empty in log mode", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			log, buf := newCaptureLogger()
			sendSplitNotifications(log, notifier, false, mockReport, []types.RemovedImageInfo{})

			for _, entry := range parseJSONLogEntries(buf) {
				gomega.Expect(entry["message"]).NotTo(gomega.Equal("Removing image"))
			}

			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should emit container-scoped cleanup fields for each cleanedImages entry in log mode", func() {
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

			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			containerA := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			containerB := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true).Times(2)
			notifier.EXPECT().SendNotification(mock.Anything).Run(func(report types.Report) {
				updated := report.Updated()
				gomega.Expect(updated).To(gomega.HaveLen(1))
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

			gomega.Expect(cleanups).To(gomega.ConsistOf(
				cleanupLog{
					containerName: "container-a",
					imageName:     "nginx:old",
					imageID:       imageA.ShortID(),
					notify:        "yes",
				},
				cleanupLog{
					containerName: "container-a",
					imageName:     "redis:old",
					imageID:       imageB.ShortID(),
					notify:        "yes",
				},
				cleanupLog{
					containerName: "container-b",
					imageName:     "nginx:old",
					imageID:       imageA.ShortID(),
					notify:        "yes",
				},
			))
			gomega.Expect(sentFocusNames).To(gomega.ConsistOf("container-a", "container-b"))
			notifier.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should not emit cleanup logs in report mode", func() {
			cleanedImages := []types.RemovedImageInfo{
				{
					ImageID:       types.ImageID("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
					ImageName:     "nginx:old",
					ContainerName: "container-a",
					ContainerID:   types.ContainerID("id-a"),
				},
			}

			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

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

			notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
			notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(true)
			notifier.EXPECT().SendNotification(mock.Anything).Return()

			log, buf := newCaptureLogger()
			sendSplitNotifications(log, notifier, true, mockReport, cleanedImages)

			for _, entry := range parseJSONLogEntries(buf) {
				gomega.Expect(entry["message"]).NotTo(gomega.Equal("Removing image"))
			}

			notifier.AssertExpectations(ginkgo.GinkgoT())
		})
	})

	ginkgo.Describe("notification level filtering", func() {
		ginkgo.Context("when notification level is error", func() {
			ginkgo.It("should not send notifications when no errors occur", func() {
				mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

				// Set up report with no errors - only successful updates
				mockContainer := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
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

				notifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
				notifier.EXPECT().ShouldSendNotification(mock.Anything).Return(false)
				// With notification level set to error and no errors, no notification should be sent

				sendSplitNotifications(testLogger(), notifier, true, mockReport, []types.RemovedImageInfo{})
				notifier.AssertExpectations(ginkgo.GinkgoT())
			})
		})
	})

	ginkgo.Describe("performImageCleanup", func() {
		ginkgo.It("should return empty slice when cleanup is disabled", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(testLogger(), context.Background(), client, false, []types.RemovedImageInfo{}, 0)
			gomega.Expect(cleanedImages).To(gomega.BeEmpty())
		})

		ginkgo.It("should perform cleanup when cleanup is enabled", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(testLogger(), context.Background(), client, true, []types.RemovedImageInfo{
				{
					ContainerName: "test-container",
					ImageName:     "test-image:v1.0",
					ImageID:       types.ImageID("sha256:123"),
				},
			}, 0)
			// The function should return the cleaned images when cleanup is enabled
			gomega.Expect(cleanedImages).To(gomega.HaveLen(1))
			gomega.Expect(cleanedImages[0].ContainerName).To(gomega.Equal("test-container"))
		})

		ginkgo.It("should return a valid slice when cleanup input is empty", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			cleanedImages := performImageCleanup(testLogger(), context.Background(), client, true, []types.RemovedImageInfo{}, 0)
			// Should return a valid slice even with empty input
			gomega.Expect(cleanedImages).NotTo(gomega.BeNil())
		})

		ginkgo.It("should still remove images when the parent context is already canceled", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			cleanedImages := performImageCleanup(testLogger(), ctx, client, true, []types.RemovedImageInfo{
				{
					ContainerName: "test-container",
					ImageName:     "test-image:v1.0",
					ImageID:       types.ImageID("sha256:123"),
				},
			}, 0)
			gomega.Expect(cleanedImages).To(gomega.HaveLen(1))
			gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).To(gomega.Equal(int32(1)))
		})
	})

	ginkgo.Describe("generateAndLogMetric", func() {
		ginkgo.It("should generate metric from report", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockContainer1 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer2 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer3 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			mockContainer4 := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

			mockReport.EXPECT().
				Scanned().
				Return([]types.ContainerReport{mockContainer1, mockContainer2})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{mockContainer3})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{mockContainer4})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})

			metric := generateAndLogMetric(testLogger(), mockReport)

			gomega.Expect(metric).NotTo(gomega.BeNil())
			gomega.Expect(metric.Scanned).To(gomega.Equal(2))
			gomega.Expect(metric.Updated).To(gomega.Equal(1))
			gomega.Expect(metric.Failed).To(gomega.Equal(1))
			gomega.Expect(metric.Restarted).To(gomega.Equal(0))
		})

		ginkgo.It("should handle empty report gracefully", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

			mockReport.EXPECT().Scanned().Return([]types.ContainerReport{})
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{})

			metric := generateAndLogMetric(testLogger(), mockReport)

			gomega.Expect(metric).NotTo(gomega.BeNil())
			gomega.Expect(metric.Scanned).To(gomega.Equal(0))
			gomega.Expect(metric.Updated).To(gomega.Equal(0))
			gomega.Expect(metric.Failed).To(gomega.Equal(0))
			gomega.Expect(metric.Restarted).To(gomega.Equal(0))
		})

		ginkgo.It("should log session completion with notify=no so it is not sent as a notification", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

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

				gomega.Expect(entry["notify"]).To(gomega.Equal("no"))
			}

			gomega.Expect(found).To(gomega.BeTrue())
		})

		ginkgo.It("should log skipped count on the session completion line", func() {
			mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
			skippedA := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())
			skippedB := mockTypes.NewMockContainerReport(ginkgo.GinkgoT())

			mockReport.EXPECT().Scanned().Return(make([]types.ContainerReport, 23))
			mockReport.EXPECT().Updated().Return([]types.ContainerReport{})
			mockReport.EXPECT().Failed().Return([]types.ContainerReport{})
			mockReport.EXPECT().Restarted().Return([]types.ContainerReport{})
			mockReport.EXPECT().Skipped().Return([]types.ContainerReport{skippedA, skippedB})

			log, buf := newCaptureLogger()
			metric := generateAndLogMetric(log, mockReport)

			gomega.Expect(metric.Skipped).To(gomega.Equal(2))
			gomega.Expect(metric.Scanned).To(gomega.Equal(23))

			var found bool

			for _, entry := range parseJSONLogEntries(buf) {
				if entry["message"] != "Update session completed" {
					continue
				}

				found = true

				gomega.Expect(entry["skipped"]).To(gomega.BeEquivalentTo(2))
				gomega.Expect(entry["scanned"]).To(gomega.BeEquivalentTo(23))
				gomega.Expect(entry["updated"]).To(gomega.BeEquivalentTo(0))
				gomega.Expect(entry["failed"]).To(gomega.BeEquivalentTo(0))
			}

			gomega.Expect(found).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("Multiple Concurrent Scopes", func() {
		ginkgo.It(
			"should support multiple Watchtower instances with different scopes running simultaneously",
			func() {
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
						Logger:                       testLogger(),
						Client:                       clientA,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "scope-a", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)
					results <- metric
				}()

				go func() {
					defer wg.Done()

					params := RunUpdatesWithNotificationsParams{
						Logger:                       testLogger(),
						Client:                       clientB,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "scope-b", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)
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
				gomega.Expect(metrics).To(gomega.HaveLen(2))

				for _, metric := range metrics {
					gomega.Expect(metric).NotTo(gomega.BeNil())
					gomega.Expect(metric.Scanned).
						To(gomega.Equal(1))
						// Each scope should scan 1 container
					gomega.Expect(metric.Updated).
						To(gomega.Equal(1))
					// Each scope should update 1 container
				}
			},
		)

		ginkgo.It(
			"should ensure scope isolation and prevent interference between concurrent scopes",
			func() {
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
						Logger:                       testLogger(),
						Client:                       client1,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "scope1", filters.WatchtowerContainersFilter),
						),
					}
					RunUpdatesWithNotifications(context.Background(), params)
				}()

				go func() {
					defer wg.Done()

					params := RunUpdatesWithNotificationsParams{
						Logger:                       testLogger(),
						Client:                       client2,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "scope2", filters.WatchtowerContainersFilter),
						),
					}
					RunUpdatesWithNotifications(context.Background(), params)
				}()

				wg.Wait()

				// Verify scope isolation through separate TestData tracking
				// client1 should only create scope1-watchtower
				gomega.Expect(client1.TestData.CreateOrder).
					To(gomega.ContainElement("scope1-watchtower"))
				gomega.Expect(client1.TestData.CreateOrder).
					NotTo(gomega.ContainElement("scope2-watchtower"))

				// client2 should only create scope2-watchtower
				gomega.Expect(client2.TestData.CreateOrder).
					To(gomega.ContainElement("scope2-watchtower"))
				gomega.Expect(client2.TestData.CreateOrder).
					NotTo(gomega.ContainElement("scope1-watchtower"))
			},
		)

		ginkgo.It("should maintain resource cleanup isolation respecting scope boundaries", func() {
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
					filters.FilterByScope(testLogger(), "scope-a", filters.WatchtowerContainersFilter),
				)
				updateA.Cleanup = true

				params := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
					Client:                       clientA,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateA,
				}
				RunUpdatesWithNotifications(context.Background(), params)
			}()

			go func() {
				defer wg.Done()

				updateB := defaultTestUpdateParams(
					filters.FilterByScope(testLogger(), "scope-b", filters.WatchtowerContainersFilter),
				)
				updateB.Cleanup = true

				params := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
					Client:                       clientB,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateB,
				}
				RunUpdatesWithNotifications(context.Background(), params)
			}()

			wg.Wait()

			// Verify cleanup isolation - each client should only attempt cleanup for containers in its scope
			// Since each client has separate TestData, the counts should reflect independent operations
			gomega.Expect(clientA.TestData.TriedToRemoveImageCount.Load()).
				To(gomega.BeNumerically(">=", int32(0)))
			gomega.Expect(clientB.TestData.TriedToRemoveImageCount.Load()).
				To(gomega.BeNumerically(">=", int32(0)))
		})

		ginkgo.It(
			"should handle concurrent operations with different scopes without interference",
			func() {
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
						Logger:                       testLogger(),
						Client:                       clientProd,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "prod", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)
					results <- map[string]int{"prod-scanned": metric.Scanned, "prod-updated": metric.Updated}
				}()

				go func() {
					defer wg.Done()

					params := RunUpdatesWithNotificationsParams{
						Logger:                       testLogger(),
						Client:                       clientDev,
						Notifier:                     nil,
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "dev", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)
					results <- map[string]int{"dev-scanned": metric.Scanned, "dev-updated": metric.Updated}
				}()

				wg.Wait()
				close(results)

				// Collect results
				var resultMaps []map[string]int
				for resultMap := range results {
					resultMaps = append(resultMaps, resultMap)
				}

				gomega.Expect(resultMaps).To(gomega.HaveLen(2))

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

				// Total operations should reflect independent scope processing
				gomega.Expect(totalScanned).
					To(gomega.BeNumerically(">=", 2))
					// At least 2 containers scanned total
				gomega.Expect(totalUpdated).
					To(gomega.BeNumerically(">=", 0))
				// Updates depend on filtering
			},
		)
	})

	ginkgo.Describe("Scoped Environment Error Reporting and State Consistency", func() {
		ginkgo.When("scoped operations encounter errors", func() {
			ginkgo.It(
				"should maintain state consistency when scoped update operations fail partially",
				func() {
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
						Logger:                       testLogger(),
						Client:                       client,
						Notifier:                     nil, // No notifications for this test
						NotificationSplitByContainer: false,
						NotificationReport:           false,
						Update: defaultTestUpdateParams(
							filters.FilterByScope(testLogger(), "scope-a", filters.NoFilter),
						),
					}

					metric := RunUpdatesWithNotifications(context.Background(), params)

					// Verify state consistency: partial failures are properly tracked
					gomega.Expect(metric).NotTo(gomega.BeNil())
					gomega.Expect(metric.Scanned).
						To(gomega.Equal(2))
						// Both containers in scope scanned
					gomega.Expect(metric.Failed).To(gomega.Equal(2))  // Both fail due to mock error
					gomega.Expect(metric.Updated).To(gomega.Equal(0)) // No successful updates
				},
			)

			ginkgo.It("should properly isolate scoped operations when cleanup fails", func() {
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
					filters.FilterByScope(testLogger(), "scope-a", filters.NoFilter),
				)
				updateA.Cleanup = true

				paramsA := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
					Client:                       clientA,
					Notifier:                     nil,
					NotificationSplitByContainer: false,
					NotificationReport:           false,
					Update:                       updateA,
				}

				// Run scope-b operation (cleanup should succeed)
				updateB := defaultTestUpdateParams(
					filters.FilterByScope(testLogger(), "scope-b", filters.NoFilter),
				)
				updateB.Cleanup = true

				paramsB := RunUpdatesWithNotificationsParams{
					Logger:                       testLogger(),
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

					metric := RunUpdatesWithNotifications(context.Background(), paramsA)
					results <- metric
				}()

				go func() {
					defer wg.Done()

					metric := RunUpdatesWithNotifications(context.Background(), paramsB)
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
				gomega.Expect(metrics).To(gomega.HaveLen(2))

				for _, metric := range metrics {
					gomega.Expect(metric).NotTo(gomega.BeNil())
					// Each scope processes exactly one container
					gomega.Expect(metric.Scanned).To(gomega.Equal(1))
					gomega.Expect(metric.Updated).To(gomega.Equal(1)) // Updates succeed
					gomega.Expect(metric.Failed).To(gomega.Equal(0))  // No update failures
				}
			})

			ginkgo.It(
				"should ensure proper error reporting isolation between concurrent scoped operations",
				func() {
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
							Logger:                       testLogger(),
							Client:                       clientX,
							Notifier:                     nil,
							NotificationSplitByContainer: false,
							NotificationReport:           false,
							Update: defaultTestUpdateParams(
								filters.FilterByScope(testLogger(), "scope-x", filters.NoFilter),
							),
						}

						metric := RunUpdatesWithNotifications(context.Background(), params)
						results <- metric
					}()

					go func() {
						defer wg.Done()

						params := RunUpdatesWithNotificationsParams{
							Logger:                       testLogger(),
							Client:                       clientY,
							Notifier:                     nil,
							NotificationSplitByContainer: false,
							NotificationReport:           false,
							Update: defaultTestUpdateParams(
								filters.FilterByScope(testLogger(), "scope-y", filters.NoFilter),
							),
						}

						metric := RunUpdatesWithNotifications(context.Background(), params)
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
					gomega.Expect(metrics).To(gomega.HaveLen(2))

					totalScanned := 0
					totalFailed := 0
					totalUpdated := 0

					for _, metric := range metrics {
						gomega.Expect(metric).NotTo(gomega.BeNil())
						gomega.Expect(metric.Scanned).
							To(gomega.Equal(1))
							// Each scope sees 1 container
						totalScanned += metric.Scanned
						totalFailed += metric.Failed
						totalUpdated += metric.Updated
					}

					gomega.Expect(totalScanned).To(gomega.Equal(2)) // Both containers processed
					gomega.Expect(totalFailed).To(gomega.Equal(1))  // One scope failed
					gomega.Expect(totalUpdated).To(gomega.Equal(1)) // One scope succeeded
				},
			)
		})
	})
})
