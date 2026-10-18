package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
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

// newNopLogger returns a logger that discards every event, for Ginkgo specs,
// benchmarks, and fuzz targets.
//
// Returns:
//   - *zerolog.Logger: Logger that discards all events.
func newNopLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

const (
	currentWatchtowerID = "current-watchtower-id"
	otherWatchtowerID   = "other-watchtower-id"
)

var _ = ginkgo.Describe("restartStaleContainer", func() {
	ginkgo.It("should not rename Watchtower container in run-once mode", func() {
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
		params := types.UpdateParams{
			RunOnce: true,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newNopLogger(), context.Background(), testContainer, client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(renamed).To(gomega.BeFalse())
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(newID).NotTo(gomega.BeEmpty())
	})

	ginkgo.It("should rename Watchtower container when not in run-once mode", func() {
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
		params := types.UpdateParams{
			RunOnce: false,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newNopLogger(), context.Background(), testContainer, client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(renamed).To(gomega.BeTrue())
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(1)))
		gomega.Expect(newID).NotTo(gomega.BeEmpty())
	})

	ginkgo.It("should skip rename if source container is already named with the target old name", func() {
		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{
					mockActions.CreateMockContainerWithConfig(
						"abc123def456",
						"/watchtower-old-abc123def456",
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
		params := types.UpdateParams{
			RunOnce: false,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newNopLogger(), context.Background(), testContainer, client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(renamed).To(gomega.BeTrue())
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(newID).NotTo(gomega.BeEmpty())
	})

	ginkgo.It("should treat ephemeral self-update as a completed handoff", func() {
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
		params := types.UpdateParams{
			EphemeralSelfUpdate: true,
			Cleanup:             true,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newNopLogger(), context.Background(), testContainer, client, params)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(renamed).To(gomega.BeTrue())
		gomega.Expect(newID).To(gomega.BeEmpty())
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(client.TestData.LastCleanup).To(gomega.BeTrue())
	})
})

var _ = ginkgo.Describe("handleUpdateResult", func() {
	ginkgo.It("should return a failed zero metric when error is not nil and there is no report", func() {
		err := errors.New("test error")
		result := handleUpdateResult(newNopLogger(), nil, err, nil)
		gomega.Expect(result).To(gomega.Equal(&metrics.Metric{ScanFailed: true}))
	})

	ginkgo.It("should return the partial report's counts, marked failed, when error is not nil", func() {
		mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
		mockReport.EXPECT().Scanned().Return(make([]types.ContainerReport, 3))
		mockReport.EXPECT().Updated().Return(make([]types.ContainerReport, 1))
		mockReport.EXPECT().Failed().Return(make([]types.ContainerReport, 2))
		mockReport.EXPECT().Restarted().Return(nil)
		mockReport.EXPECT().Skipped().Return(nil)

		err := errors.New("rolling restart canceled")
		result := handleUpdateResult(newNopLogger(), mockReport, err, nil)
		gomega.Expect(result).To(gomega.Equal(&metrics.Metric{Scanned: 3, Updated: 1, Failed: 2, ScanFailed: true}))
	})

	ginkgo.It("should return a failed zero metric when result is nil", func() {
		var err error

		result := handleUpdateResult(newNopLogger(), nil, err, nil)
		gomega.Expect(result).To(gomega.Equal(&metrics.Metric{ScanFailed: true}))
	})

	ginkgo.It("should return nil when result is not nil and error is nil", func() {
		mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

		var err error

		result := handleUpdateResult(newNopLogger(), mockReport, err, nil)
		gomega.Expect(result).To(gomega.BeNil())
	})

	ginkgo.It("should send an empty report when error occurs without a report", func() {
		// Create a mock notifier that tracks if SendNotification was called
		mockNotifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
		mockNotifier.EXPECT().SendNotification(emptyReport{}).Times(1)

		// Call handleUpdateResult with an error and the mock notifier
		err := errors.New("dependency resolution error")
		result := handleUpdateResult(newNopLogger(), nil, err, mockNotifier)

		// Verify we got the expected metric
		gomega.Expect(result).To(gomega.Equal(&metrics.Metric{ScanFailed: true}))
	})

	ginkgo.It("should send the partial report when error occurs after some work", func() {
		mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())
		mockReport.EXPECT().Scanned().Return(nil)
		mockReport.EXPECT().Updated().Return(nil)
		mockReport.EXPECT().Failed().Return(nil)
		mockReport.EXPECT().Restarted().Return(nil)
		mockReport.EXPECT().Skipped().Return(nil)

		mockNotifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())
		mockNotifier.EXPECT().SendNotification(mockReport).Times(1)

		err := errors.New("rolling restart canceled")
		handleUpdateResult(newNopLogger(), mockReport, err, mockNotifier)
	})

	ginkgo.It("should not send notification when error occurs and notifier is nil", func() {
		// Call handleUpdateResult with an error and nil notifier
		err := errors.New("dependency resolution error")
		result := handleUpdateResult(newNopLogger(), nil, err, nil)

		// Verify we got the expected metric
		gomega.Expect(result).To(gomega.Equal(&metrics.Metric{ScanFailed: true}))
	})

	ginkgo.It("should not send notification when there is no error", func() {
		// Create a mock notifier with no expectations (will fail if any method is called)
		mockNotifier := mockTypes.NewMockNotifier(ginkgo.GinkgoT())

		// Call handleUpdateResult without an error
		mockReport := mockTypes.NewMockReport(ginkgo.GinkgoT())

		var err error

		result := handleUpdateResult(newNopLogger(), mockReport, err, mockNotifier)

		// Verify we got the expected result
		gomega.Expect(result).To(gomega.BeNil())
	})
})

var _ = ginkgo.Describe("executeUpdate", func() {
	ginkgo.It("should execute update successfully", func() {
		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{
					mockActions.CreateMockContainerWithConfig(
						"test-container",
						"/test-container",
						"test:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{},
					),
				},
				Staleness: map[string]bool{
					"test-container": false,
				},
			},
			false,
			false,
		)
		config := types.UpdateParams{
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newNopLogger(),
			context.Background(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report).NotTo(gomega.BeNil())
		gomega.Expect(cleanupInfos).NotTo(gomega.BeNil())
	})

	ginkgo.It("should not return error when no containers to update", func() {
		client := mockActions.CreateMockClient(
			&mockActions.TestData{},
			false,
			false,
		)
		config := types.UpdateParams{
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newNopLogger(),
			context.Background(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report).NotTo(gomega.BeNil())
		gomega.Expect(cleanupInfos).NotTo(gomega.BeNil())
	})

	ginkgo.It("should execute update logic for stale containers", func() {
		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{
					mockActions.CreateMockContainerWithConfig(
						"test-container",
						"/test-container",
						"test:latest",
						true,
						false,
						time.Now(),
						&dockerContainer.Config{},
					),
				},
				Staleness: map[string]bool{
					"test-container": true,
				},
			},
			false,
			false,
		)
		config := types.UpdateParams{
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newNopLogger(),
			context.Background(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report).NotTo(gomega.BeNil())
		gomega.Expect(cleanupInfos).NotTo(gomega.BeNil())
		gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(1)))
	})

	ginkgo.It("should propagate RunOnce mode and skip Watchtower self-update", func() {
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
			Filter:  filters.NoFilter,
			RunOnce: true,
		}
		report, cleanupInfos, err := executeUpdate(newNopLogger(),
			context.Background(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report).NotTo(gomega.BeNil())
		gomega.Expect(cleanupInfos).NotTo(gomega.BeNil())
		gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
		gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))
	})

	ginkgo.It("should call SetRestartPolicy for Watchtower restart policy changes", func() {
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
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newNopLogger(),
			context.Background(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(report).NotTo(gomega.BeNil())
		gomega.Expect(cleanupInfos).NotTo(gomega.BeNil())
		gomega.Expect(client.TestData.SetRestartPolicyCount.Load()).To(gomega.Equal(int32(1)))
	})
})

var _ = ginkgo.Describe("shouldUpdateContainer", func() {
	ginkgo.It("should allow self-update of current Watchtower container", func() {
		currentID := currentWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			currentID,
			"watchtower-current",
			"watchtower:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			},
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should skip other Watchtower containers from self-updates", func() {
		currentID := currentWatchtowerID
		otherID := otherWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			otherID,
			"watchtower-other",
			"watchtower:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			},
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should not affect non-Watchtower containers", func() {
		currentID := currentWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			"non-watchtower-id",
			"nginx",
			"nginx:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{},
			},
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should allow self-update of scoped Watchtower container", func() {
		currentID := currentWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			currentID,
			"watchtower-current",
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
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It(
		"should skip other scoped Watchtower containers with same scope from self-updates",
		func() {
			currentID := currentWatchtowerID
			otherID := otherWatchtowerID
			container := mockActions.CreateMockContainerWithConfig(
				otherID,
				"watchtower-other",
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
			)
			params := types.UpdateParams{
				CurrentContainerID: types.ContainerID(currentID),
			}
			result := shouldUpdateContainer(container, true, params)
			gomega.Expect(result).To(gomega.BeFalse())
		},
	)

	ginkgo.It(
		"should skip other scoped Watchtower containers with different scopes from self-updates",
		func() {
			currentID := currentWatchtowerID
			otherID := otherWatchtowerID
			container := mockActions.CreateMockContainerWithConfig(
				otherID,
				"watchtower-other",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "dev",
					},
				},
			)
			params := types.UpdateParams{
				CurrentContainerID: types.ContainerID(currentID),
			}
			result := shouldUpdateContainer(container, true, params)
			gomega.Expect(result).To(gomega.BeFalse())
		},
	)

	ginkgo.It("should skip unscoped Watchtower containers from scoped self-updates", func() {
		currentID := currentWatchtowerID
		otherID := otherWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			otherID,
			"watchtower-other",
			"watchtower:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			},
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should skip scoped Watchtower containers from unscoped self-updates", func() {
		currentID := currentWatchtowerID
		otherID := otherWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			otherID,
			"watchtower-other",
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
		)
		params := types.UpdateParams{
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should allow update of non-Watchtower containers when NoRestart is true", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"non-watchtower-id",
			"nginx",
			"nginx:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{},
			},
		)
		params := types.UpdateParams{
			NoRestart: true,
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should allow update of Watchtower containers when NoRestart is true", func() {
		currentID := currentWatchtowerID
		container := mockActions.CreateMockContainerWithConfig(
			currentID,
			"watchtower-current",
			"watchtower:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			},
		)
		params := types.UpdateParams{
			NoRestart:          true,
			CurrentContainerID: types.ContainerID(currentID),
		}
		result := shouldUpdateContainer(container, true, params)
		gomega.Expect(result).To(gomega.BeTrue())
	})
})

var _ = ginkgo.Describe("hasSelfDependency", func() {
	ginkgo.It("should return false when no depends-on label is present", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels:       map[string]string{},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when depends-on label is empty", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when depends-on contains other containers", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "other-container",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return true when depends-on contains self", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "test-container",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It(
		"should return true when depends-on contains self among multiple dependencies",
		func() {
			container := mockActions.CreateMockContainerWithConfig(
				"test-container",
				"/test-container",
				"test:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "other-container,test-container,another-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			result := hasSelfDependency(container)
			gomega.Expect(result).To(gomega.BeTrue())
		},
	)

	ginkgo.It("should handle spaces and trimming correctly", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": " other-container , test-container , another-container ",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should handle leading slashes in container names", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "/test-container",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should return false when Config is nil", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			nil,
		) // Config is nil
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when Labels is nil", func() {
		container := mockActions.CreateMockContainerWithConfig(
			"test-container",
			"/test-container",
			"test:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels:       nil, // Labels is nil
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		result := hasSelfDependency(container)
		gomega.Expect(result).To(gomega.BeFalse())
	})
})

var _ = ginkgo.Describe("emptyReport", func() {
	ginkgo.It("Scanned() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Scanned()).To(gomega.BeNil())
	})

	ginkgo.It("Updated() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Updated()).To(gomega.BeNil())
	})

	ginkgo.It("Failed() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Failed()).To(gomega.BeNil())
	})

	ginkgo.It("Skipped() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Skipped()).To(gomega.BeNil())
	})

	ginkgo.It("Stale() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Stale()).To(gomega.BeNil())
	})

	ginkgo.It("Fresh() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Fresh()).To(gomega.BeNil())
	})

	ginkgo.It("Restarted() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.Restarted()).To(gomega.BeNil())
	})

	ginkgo.It("All() should return nil", func() {
		report := emptyReport{}
		gomega.Expect(report.All()).To(gomega.BeNil())
	})
})

// parseJSONLogEntries parses zerolog JSON lines from a buffer into field maps.
func parseJSONLogEntries(buf *bytes.Buffer) []map[string]any {
	var entries []map[string]any

	for line := range bytes.SplitSeq(buf.Bytes(), []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var m map[string]any

		err := json.Unmarshal(line, &m)
		if err != nil {
			continue
		}

		entries = append(entries, m)
	}

	return entries
}

// newCaptureLogger returns a JSON zerolog logger writing to a buffer for assertions.
func newCaptureLogger() (*zerolog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := zerolog.New(buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()

	return &l, buf
}

// stopContainersTestCase represents a test case for stopContainersInReversedOrder cancellation.
type stopContainersTestCase struct {
	name                string
	numContainers       int
	cancelAtIndex       int    // Index at which to cancel (from end, -1 means no cancellation)
	expectedStopped     int    // Number of containers that should be stopped
	expectedSkipped     int    // Number of containers that should be skipped
	expectedLogMessages int    // Expected number of log messages for skipped containers
	description         string // Human-readable description
}

// DetachedContext specs cover the detached context that restartStaleContainer creates
// from the Timeout setting. A positive Timeout sets the deadline, and a zero or
// negative Timeout falls back to a default deadline that prevents indefinite blocking.
var _ = ginkgo.Describe("DetachedContext", func() {
	// TestDetachedContextDeadlineCase represents a test case for detached context deadline behavior.
	type TestDetachedContextDeadlineCase struct {
		name            string
		timeout         time.Duration
		expectDeadline  bool
		expectedTimeout time.Duration
		description     string
	}

	ginkgo.Describe("restartStaleContainer detached context deadline", func() {
		testCases := []TestDetachedContextDeadlineCase{
			{
				name:            "positive timeout above minimum uses configured timeout",
				timeout:         10 * time.Minute,
				expectDeadline:  true,
				expectedTimeout: 10 * time.Minute,
				description:     "When Timeout exceeds defaultCreateStartTimeout, the detached context uses the configured timeout",
			},
			{
				name:            "positive timeout below minimum creates context with minimum deadline",
				timeout:         30 * time.Second,
				expectDeadline:  true,
				expectedTimeout: defaultCreateStartTimeout,
				description:     "When Timeout is below defaultCreateStartTimeout, the detached context uses the minimum",
			},
			{
				name:            "zero timeout creates context with minimum deadline",
				timeout:         0,
				expectDeadline:  true,
				expectedTimeout: defaultCreateStartTimeout,
				description:     "When Timeout is zero, the detached context uses the minimum create/start deadline",
			},
			{
				name:            "negative timeout creates context with minimum deadline",
				timeout:         -1 * time.Second,
				expectDeadline:  true,
				expectedTimeout: defaultCreateStartTimeout,
				description:     "When Timeout is negative, the detached context uses the minimum create/start deadline",
			},
		}

		for _, tc := range testCases {
			ginkgo.It(tc.name, func() {
				// Create a mock client with a Watchtower container that will trigger
				// the restart policy update path where the detached context is used.
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

				// Configure params with the test timeout value.
				// RunOnce is false to enable the rename path which uses the detached context.
				params := types.UpdateParams{
					Timeout: tc.timeout,
					RunOnce: false,
				}

				testContainer := client.TestData.Containers[0]

				// Call restartStaleContainer which creates and uses the detached context.
				newID, renamed, err := restartStaleContainer(newNopLogger(),
					context.Background(),
					testContainer,
					client,
					params,
				)

				// Verify the operation succeeded.
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(renamed).To(gomega.BeTrue())
				gomega.Expect(newID).NotTo(gomega.BeEmpty())

				// Verify SetRestartPolicy was called (this uses the detached context).
				// The detached context is used for updating the restart policy of the
				// renamed Watchtower container.
				gomega.Expect(client.TestData.SetRestartPolicyCount.Load()).To(gomega.Equal(int32(1)))

				// Verify CreateContainer and StartContainerByID also use the detached context.
				// These operations run after the initial rename, so they should share the same
				// deadline when expectDeadline is true.
				createCtx := client.TestData.CreateContainerCtx
				startCtx := client.TestData.StartContainerByIDCtx
				noRestartCtx := client.TestData.SetRestartPolicyCtx

				gomega.Expect(createCtx).NotTo(gomega.BeNil(), "CreateContainer should receive a context")
				gomega.Expect(startCtx).NotTo(gomega.BeNil(), "StartContainerByID should receive a context")
				gomega.Expect(createCtx).To(gomega.Equal(startCtx), "CreateContainer and StartContainerByID should share the same detached context")
				gomega.Expect(createCtx).To(gomega.Equal(noRestartCtx), "CreateContainer should use the same detached context as SetRestartPolicy")

				if tc.expectDeadline {
					_, createHasDeadline := createCtx.Deadline()
					_, startHasDeadline := startCtx.Deadline()

					gomega.Expect(createHasDeadline).To(gomega.BeTrue())
					gomega.Expect(startHasDeadline).To(gomega.BeTrue())

					expectedDeadline := time.Now().Add(tc.expectedTimeout)
					createDeadline, _ := createCtx.Deadline()
					startDeadline, _ := startCtx.Deadline()

					gomega.Expect(createDeadline).To(gomega.BeTemporally("~", expectedDeadline, time.Second))
					gomega.Expect(startDeadline).To(gomega.BeTemporally("~", expectedDeadline, time.Second))
				} else {
					_, createHasDeadline := createCtx.Deadline()
					_, startHasDeadline := startCtx.Deadline()

					gomega.Expect(createHasDeadline).To(gomega.BeFalse())
					gomega.Expect(startHasDeadline).To(gomega.BeFalse())
				}

				// Verify CreateContainer and StartContainerByID also use the detached context.
				// These operations run after CreateContainer succeeds, so the same deadline
				// should apply when expectDeadline is true.
				gomega.Expect(client.TestData.CreateContainerCount.Load()).To(gomega.Equal(int32(1)))
				gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(1)))

				if tc.expectDeadline {
					createDeadline, createHasDeadline := client.TestData.CreateContainerCtx.Deadline()
					gomega.Expect(createHasDeadline).To(gomega.BeTrue())

					startDeadline, startHasDeadline := client.TestData.StartContainerByIDCtx.Deadline()
					gomega.Expect(startHasDeadline).To(gomega.BeTrue())

					expectedDeadline := time.Now().Add(tc.expectedTimeout)
					gomega.Expect(createDeadline).To(gomega.BeTemporally("~", expectedDeadline, time.Second))
					gomega.Expect(startDeadline).To(gomega.BeTemporally("~", expectedDeadline, time.Second))
				}
			})
		}
	})

	ginkgo.Describe("Watchtower self-update create failure recovery", func() {
		ginkgo.It("renames the source back to the original name when create fails", func() {
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower-source-id",
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
					CreateContainerError: errors.New("simulated create failure"),
				},
				false,
				false,
			)

			params := types.UpdateParams{
				Timeout: 0,
				RunOnce: false,
			}

			testContainer := client.TestData.Containers[0]

			// Use a cancellable parent context to verify rename-back survives cancellation.
			parentCtx, parentCancel := context.WithCancel(context.Background())
			defer parentCancel()

			// Add simulated latency to CreateContainer so the initial rename completes
			// before we cancel the parent context.
			client.TestData.SimulatedLatency = 5 * time.Millisecond

			var (
				renamed bool
				err     error
			)

			// Run restartStaleContainer in a goroutine so we can cancel the parent
			// after the initial handoff rename but before/during create failure.
			var wg sync.WaitGroup
			wg.Go(func() {
				_, renamed, err = restartStaleContainer(newNopLogger(),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for CreateContainer to be called, indicating the initial rename
			// has completed. Then cancel the parent context.
			gomega.Eventually(func() int32 {
				return client.TestData.CreateContainerCount.Load()
			}).Should(gomega.BeNumerically(">", 0))

			parentCancel()
			wg.Wait()

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to create container"))
			gomega.Expect(renamed).To(gomega.BeFalse(), "name should be restored after create failure")
			gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(2)))
			gomega.Expect(client.TestData.RenameTargets).To(gomega.HaveLen(2))
			gomega.Expect(client.TestData.RenameTargets[1]).To(gomega.Equal("watchtower"))
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))

			// Assert that CreateContainer and RenameContainer received distinct contexts.
			// The first rename (initial handoff) uses the caller context, the second
			// rename (recovery after create failure) uses a fresh bounded context.
			createCtx := client.TestData.CreateContainerCtx
			renameBackCtx := client.TestData.RenameContainerCtx

			gomega.Expect(createCtx).NotTo(gomega.BeNil(), "CreateContainer should receive a context")
			gomega.Expect(renameBackCtx).NotTo(gomega.BeNil(), "RenameContainer should receive a context for rename-back")
			gomega.Expect(createCtx).NotTo(gomega.Equal(renameBackCtx), "CreateContainer and rename-back should use distinct contexts")

			// Assert that the rename-back context has an active deadline (is a bounded timeout context).
			_, hasDeadline := renameBackCtx.Deadline()
			gomega.Expect(hasDeadline).To(gomega.BeTrue(), "rename-back context should have a deadline for bounded recovery")
		})
	})

	ginkgo.Describe("Watchtower self-update start failure cleanup", func() {
		ginkgo.It("removes the newly created container, not the renamed source", func() {
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"watchtower-source-id",
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
					StartContainerByIDError: errors.New("simulated start failure"),
				},
				false,
				false,
			)

			params := types.UpdateParams{
				Timeout: 0,
				RunOnce: false,
			}

			testContainer := client.TestData.Containers[0]

			// Use a cancellable parent context to verify cleanup survives cancellation.
			parentCtx, parentCancel := context.WithCancel(context.Background())
			defer parentCancel()

			// Add simulated latency so the initial rename and create complete
			// before we cancel the parent context.
			client.TestData.SimulatedLatency = 5 * time.Millisecond

			var (
				renamed bool
				err     error
			)

			// Run restartStaleContainer in a goroutine so we can cancel the parent
			// after the initial handoff and create but before/during start failure.
			var wg sync.WaitGroup
			wg.Go(func() {
				_, renamed, err = restartStaleContainer(newNopLogger(),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for StartContainer to be called, indicating the initial rename
			// and create have completed. Then cancel the parent context.
			gomega.Eventually(func() int32 {
				return client.TestData.StartContainerCount.Load()
			}).Should(gomega.BeNumerically(">", 0))

			parentCancel()
			wg.Wait()

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to start container"))
			gomega.Expect(renamed).To(gomega.BeTrue())
			gomega.Expect(client.TestData.CreateContainerCount.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(client.TestData.LastCreatedContainerID).ToNot(gomega.BeEmpty())
			gomega.Expect(client.TestData.StopAndRemoveContainerCount.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(client.TestData.RemoveContainerCount.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(client.TestData.LastStopAndRemoveID).To(gomega.Equal(client.TestData.LastCreatedContainerID))
			gomega.Expect(client.TestData.LastStopAndRemoveID).ToNot(gomega.Equal(testContainer.ID()))

			// Assert that GetContainer and StopAndRemoveContainer receive a fresh
			// context distinct from the create/start detached context.
			getCtx := client.TestData.GetContainerCtx
			cleanupCtx := client.TestData.StopAndRemoveContainerCtx
			createCtx := client.TestData.CreateContainerCtx

			gomega.Expect(getCtx).NotTo(gomega.BeNil(), "GetContainer should receive a context")
			gomega.Expect(cleanupCtx).NotTo(gomega.BeNil(), "StopAndRemoveContainer should receive a context")
			gomega.Expect(getCtx).To(gomega.Equal(cleanupCtx), "GetContainer and StopAndRemoveContainer should share the same cleanup context")
			gomega.Expect(getCtx).NotTo(gomega.Equal(createCtx), "cleanup context should be distinct from the create/start detached context")

			// Assert that the cleanup context has an active deadline (is a bounded timeout context).
			_, hasDeadline := getCtx.Deadline()
			gomega.Expect(hasDeadline).To(gomega.BeTrue(), "cleanup context should have a deadline for bounded recovery")
		})
	})

	ginkgo.Describe("restartStaleContainer detached context survival", func() {
		ginkgo.It("cleanup operations complete when parent context is canceled during execution", func() {
			// Create a parent context that we will cancel while restartStaleContainer is running.
			parentCtx, parentCancel := context.WithCancel(context.Background())

			// Create a mock client with a Watchtower container.
			// Configure StartContainerByIDError to trigger the cleanup path.
			// Add simulated latency to allow time for operations to complete.
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
					StartContainerByIDError: errors.New("simulated start failure"),
					SimulatedLatency:        5 * time.Millisecond, // Allow time for operations
				},
				false,
				false,
			)

			params := types.UpdateParams{
				Timeout: 0, // Fallback deadline applied to detached context
				RunOnce: false,
			}

			testContainer := client.TestData.Containers[0]

			// Run restartStaleContainer in a goroutine so we can cancel the parent context
			// while it's still executing.
			var (
				err     error
				renamed bool
				wg      sync.WaitGroup
			)

			wg.Go(func() {
				// Call restartStaleContainer with the parent context.
				// The test flow is:
				// 1. RenameContainer succeeds (uses parent context)
				// 2. StartContainerByID fails due to StartContainerByIDError
				// 3. Cleanup runs using the detached context (should survive parent cancellation)
				_, renamed, err = restartStaleContainer(newNopLogger(),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for StartContainer to be called (which means RenameContainer has completed)
			// before canceling the parent context. This ensures we cancel at the right moment -
			// after rename succeeds but during/after start fails.
			gomega.Eventually(func() int32 {
				return client.TestData.StartContainerCount.Load()
			}).Should(gomega.BeNumerically(">", 0))

			// Cancel the parent context after StartContainer has been called.
			// The detached context should allow cleanup to proceed even though
			// the parent context is canceled.
			parentCancel()

			// Wait for the goroutine to complete.
			wg.Wait()

			// The operation should fail due to StartContainerByID error, but the
			// cleanup (StopAndRemoveContainer) should have been attempted
			// using the detached context, which survives parent cancellation.
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to start container"))
			gomega.Expect(renamed).To(gomega.BeTrue())

			// Verify that RemoveContainer was called during cleanup.
			// This demonstrates that the detached context allowed the cleanup
			// operation to proceed even though the parent context was canceled.
			gomega.Expect(client.TestData.RemoveContainerCount.Load()).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("cleanup operations complete when parent context is already canceled", func() {
			// Create a parent context that is already canceled.
			parentCtx, parentCancel := context.WithCancel(context.Background())
			parentCancel() // Cancel immediately before calling restartStaleContainer

			// Create a mock client with a Watchtower container.
			// Configure StartContainerError to trigger the cleanup path.
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
					StartContainerError: errors.New("simulated start failure"),
				},
				false,
				false,
			)

			params := types.UpdateParams{
				Timeout: 0, // Fallback deadline applied to detached context
				RunOnce: false,
			}

			testContainer := client.TestData.Containers[0]

			// Call restartStaleContainer with an already-canceled parent context.
			// The RenameContainer operation should fail because the parent context is canceled.
			_, renamed, err := restartStaleContainer(newNopLogger(),
				parentCtx,
				testContainer,
				client,
				params,
			)

			// The operation should fail at RenameContainer due to parent context cancellation.
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to rename Watchtower container"))
			gomega.Expect(renamed).To(gomega.BeFalse())

			// RenameContainer should have been attempted but failed due to context cancellation.
			gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("restart policy update uses detached context after successful start", func() {
			// This test verifies that SetRestartPolicy uses the detached context,
			// not the parent context. Since StartContainer uses the parent context,
			// we cannot cancel it before calling restartStaleContainer. Instead, we
			// verify that SetRestartPolicy is called after a successful start,
			// demonstrating the detached context is properly created and used.

			// Create a mock client with a Watchtower container that succeeds.
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

			// Use a timeout of 0 to create a detached context with the fallback deadline.
			params := types.UpdateParams{
				Timeout: 0,
				RunOnce: false,
			}

			testContainer := client.TestData.Containers[0]

			// Call restartStaleContainer with a background context.
			newID, renamed, err := restartStaleContainer(newNopLogger(),
				context.Background(),
				testContainer,
				client,
				params,
			)

			// The operation should succeed completely.
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(renamed).To(gomega.BeTrue())
			gomega.Expect(newID).NotTo(gomega.BeEmpty())

			// Verify that both StartContainer and SetRestartPolicy were called.
			// SetRestartPolicy uses the detached context for the restart policy update.
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(client.TestData.SetRestartPolicyCount.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(client.TestData.SetRestartPolicyCtx).NotTo(gomega.Equal(context.Background()))
		})
	})
})

// Tests for stopContainersInReversedOrder cancellation handling.
// These tests verify that when context cancellation occurs during container stopping:
// 1. All remaining containers are logged with appropriate fields
// 2. All remaining containers are added to the failed map with wrapped errors
// 3. Edge cases (cancellation at start, middle, end) are handled correctly.
//
// Important: When context is canceled at index i, the function adds the current container
// at index i to the failed map, then adds containers from i-1 down to 0.
// All containers from i down to 0 are properly logged and tracked as failed.
var _ = ginkgo.Describe("stopContainersInReversedOrder", func() {
	ginkgo.When("context is canceled during iteration", func() {
		// Table-driven tests for various cancellation scenarios.
		// Note: When context is already canceled at the start of iteration (i = len-1),
		// all containers from i down to 0 are added to failed.
		testCases := []stopContainersTestCase{
			{
				name:                "cancellation_at_start_all_skipped",
				numContainers:       3,
				cancelAtIndex:       0, // Context already canceled - at i=2, containers 2,1,0 are skipped
				expectedStopped:     0,
				expectedSkipped:     3, // containers 2, 1, and 0 are added to failed
				expectedLogMessages: 3,
				description:         "When context is canceled at the start, all containers should be skipped",
			},
			{
				name:                "cancellation_in_middle_partial_skip",
				numContainers:       5,
				cancelAtIndex:       0, // Context already canceled - at i=4, containers 4,3,2,1,0 are skipped
				expectedStopped:     0,
				expectedSkipped:     5, // containers 4,3,2,1,0 are added to failed
				expectedLogMessages: 5,
				description:         "When context is canceled mid-iteration, all containers should be skipped",
			},
			{
				name:                "cancellation_at_end_no_skip",
				numContainers:       3,
				cancelAtIndex:       -1, // No cancellation
				expectedStopped:     3,
				expectedSkipped:     0,
				expectedLogMessages: 0,
				description:         "When no cancellation occurs, all containers should be stopped",
			},
			{
				name:                "single_container_canceled",
				numContainers:       1,
				cancelAtIndex:       0, // Context already canceled - at i=0, container 0 is skipped
				expectedStopped:     0,
				expectedSkipped:     1, // container 0 is added to failed
				expectedLogMessages: 1,
				description:         "Single container scenario with cancellation",
			},
			{
				name:                "single_container_not_canceled",
				numContainers:       1,
				cancelAtIndex:       -1, // No cancellation
				expectedStopped:     1,
				expectedSkipped:     0,
				expectedLogMessages: 0,
				description:         "Single container scenario without cancellation",
			},
		}

		for _, tc := range testCases {
			ginkgo.It(tc.name, func() {
				ginkgo.By(tc.description)

				// Create mock containers with ToRestart set to true.
				containers := make([]types.Container, tc.numContainers)
				for i := range tc.numContainers {
					containerID := fmt.Sprintf("container-%d", i)
					containerName := fmt.Sprintf("/container-%d", i)
					imageName := fmt.Sprintf("image-%d:latest", i)

					c := mockActions.CreateMockContainerWithConfig(
						containerID,
						containerName,
						imageName,
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)
					// Mark container for restart so it will be processed.
					c.SetStale(true)
					containers[i] = c
				}

				// Create mock client.
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: containers,
						Staleness:  make(map[string]bool),
					},
					false,
					false,
				)

				// Mark all containers as stale.
				for i := range tc.numContainers {
					client.TestData.Staleness[fmt.Sprintf("container-%d", i)] = true
				}

				// Set up log capture to verify log messages.
				log, logBuf := newCaptureLogger()

				// Create context - either canceled or not based on test case.
				ctx := context.Background()
				if tc.cancelAtIndex >= 0 {
					// Create an already-canceled context to simulate cancellation.
					canceledCtx, cancel := context.WithCancel(context.Background())
					cancel() // Cancel immediately

					ctx = canceledCtx
				}

				// Call stopContainersInReversedOrder.
				failed, stopped := stopContainersInReversedOrder(log,
					ctx,
					containers,
					client,
					types.UpdateParams{},
				)

				// Verify the number of stopped containers.
				gomega.Expect(stopped).
					To(gomega.HaveLen(tc.expectedStopped), "Expected %d stopped containers", tc.expectedStopped)

				// Verify the number of failed containers.
				gomega.Expect(failed).
					To(gomega.HaveLen(tc.expectedSkipped), "Expected %d failed containers", tc.expectedSkipped)

				// Verify log messages for skipped containers.
				skippedLogCount := 0

				for _, entry := range parseJSONLogEntries(logBuf) {
					if entry["message"] == "Skipped container stop due to context cancellation" {
						skippedLogCount++

						// Verify log fields contain expected keys.
						gomega.Expect(entry).To(gomega.HaveKey("container"))
						gomega.Expect(entry).To(gomega.HaveKey("image"))
						gomega.Expect(entry).To(gomega.HaveKey("container_id"))
					}
				}

				gomega.Expect(skippedLogCount).
					To(gomega.Equal(tc.expectedLogMessages), "Expected %d log messages for skipped containers", tc.expectedLogMessages)
			})
		}
	})

	ginkgo.When("context is canceled mid-iteration", func() {
		ginkgo.It("should add remaining containers to failed map with wrapped error", func() {
			// Create 4 containers.
			// When context is already canceled at the start:
			// - At i=3, ctx.Err() != nil, so container 3 is added to failed
			// - Then containers 2,1,0 are also added to failed
			// - All 4 containers are properly tracked as failed
			containers := make([]types.Container, 4)

			for i := range 4 {
				containerID := fmt.Sprintf("container-%d", i)
				containerName := fmt.Sprintf("/container-%d", i)
				imageName := fmt.Sprintf("image-%d:latest", i)

				c := mockActions.CreateMockContainerWithConfig(
					containerID,
					containerName,
					imageName,
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
						"container-3": true,
					},
				},
				false,
				false,
			)

			// Create a canceled context.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			// Call stopContainersInReversedOrder.
			failed, stopped := stopContainersInReversedOrder(newNopLogger(),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
			)

			// All 4 containers should be in failed map (containers 0, 1, 2, 3).
			gomega.Expect(failed).To(gomega.HaveLen(4))
			gomega.Expect(stopped).To(gomega.BeEmpty())

			// Verify all containers 0, 1, 2, 3 are in failed map with wrapped error.
			for i := range 4 {
				containerID := types.ContainerID(fmt.Sprintf("container-%d", i))
				err, exists := failed[containerID]
				gomega.Expect(exists).To(gomega.BeTrue(), "Container %d should be in failed map", i)

				// Verify error message contains "stop skipped".
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("stop skipped"))

				// Verify error wraps context.Canceled.
				gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue(),
					"Error should wrap context.Canceled")
			}
		})

		ginkgo.It("should log each skipped container with correct fields", func() {
			// Create containers.
			// When context is already canceled at the start:
			// - At i=2, ctx.Err() != nil, so container 2 is logged and added to failed
			// - Then containers 1,0 are also logged and added to failed
			// - All 3 containers are properly logged
			containers := make([]types.Container, 3)
			expectedNames := []string{"container-0", "container-1", "container-2"} // All 3 are logged

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
				},
				false,
				false,
			)

			// Set up log capture.
			log, logBuf := newCaptureLogger()

			// Create a canceled context.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			// Call stopContainersInReversedOrder.
			_, _ = stopContainersInReversedOrder(log,
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
			)

			// Verify log entries contain expected container details.
			loggedNames := make(map[string]bool)

			for _, entry := range parseJSONLogEntries(logBuf) {
				if entry["message"] == "Skipped container stop due to context cancellation" {
					containerName, ok := entry["container"]
					if ok {
						loggedNames[containerName.(string)] = true
					}

					// Verify all expected fields are present.
					gomega.Expect(entry).To(gomega.HaveKey("container"))
					gomega.Expect(entry).To(gomega.HaveKey("image"))
					gomega.Expect(entry).To(gomega.HaveKey("container_id"))
				}
			}

			// Verify all containers were logged.
			for _, name := range expectedNames {
				gomega.Expect(loggedNames).To(gomega.HaveKey(name),
					"Container %s should have been logged", name)
			}

			// Verify we got the expected number of log messages.
			gomega.Expect(loggedNames).To(gomega.HaveLen(3))
		})
	})

	ginkgo.When("context is not canceled", func() {
		ginkgo.It("should process all containers without adding to failed map", func() {
			// Create containers.
			containers := make([]types.Container, 3)

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
				},
				false,
				false,
			)

			// Set up log capture.
			log, logBuf := newCaptureLogger()

			// Call with valid context.
			failed, stopped := stopContainersInReversedOrder(log,
				context.Background(),
				containers,
				client,
				types.UpdateParams{},
			)

			// All containers should be stopped, none failed.
			gomega.Expect(stopped).To(gomega.HaveLen(3))
			gomega.Expect(failed).To(gomega.BeEmpty())

			// Verify no "Skipped container stop" log messages.
			for _, entry := range parseJSONLogEntries(logBuf) {
				gomega.Expect(entry["message"]).
					NotTo(gomega.Equal("Skipped container stop due to context cancellation"))
			}
		})
	})

	ginkgo.When("containers are processed in reverse order", func() {
		ginkgo.It("should stop containers from last to first", func() {
			// Create containers.
			containers := make([]types.Container, 3)

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
					StopOrder: []string{},
				},
				false,
				false,
			)

			// Call with valid context.
			_, _ = stopContainersInReversedOrder(newNopLogger(),
				context.Background(),
				containers,
				client,
				types.UpdateParams{},
			)

			// Verify stop order is reverse (container-2, container-1, container-0).
			gomega.Expect(client.TestData.StopOrder).To(gomega.HaveLen(3))
			gomega.Expect(client.TestData.StopOrder[0]).To(gomega.Equal("container-2"))
			gomega.Expect(client.TestData.StopOrder[1]).To(gomega.Equal("container-1"))
			gomega.Expect(client.TestData.StopOrder[2]).To(gomega.Equal("container-0"))
		})
	})
})

var _ = ginkgo.Describe("restartContainersInSortedOrder cancel recovery", func() {
	ginkgo.It("still recreates containers that were already stopped when parent ctx is canceled", func() {
		containers := make([]types.Container, 3)

		for i := range 3 {
			c := mockActions.CreateMockContainerWithConfig(
				fmt.Sprintf("container-%d", i),
				fmt.Sprintf("/container-%d", i),
				fmt.Sprintf("image-%d:latest", i),
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			c.SetStale(true)
			containers[i] = c
		}

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: containers,
				Staleness: map[string]bool{
					"container-0": true,
					"container-1": true,
					"container-2": true,
				},
			},
			false,
			false,
		)

		// Pretend the stop phase already removed all three.
		stoppedImages := []types.RemovedImageInfo{
			{ContainerID: containers[0].ID(), ImageID: containers[0].ImageID(), ImageName: containers[0].ImageName(), ContainerName: containers[0].Name()},
			{ContainerID: containers[1].ID(), ImageID: containers[1].ImageID(), ImageName: containers[1].ImageName(), ContainerName: containers[1].Name()},
			{ContainerID: containers[2].ID(), ImageID: containers[2].ImageID(), ImageName: containers[2].ImageName(), ContainerName: containers[2].Name()},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		progress := session.Progress{}

		var cleanup []types.RemovedImageInfo

		failed := restartContainersInSortedOrder(newNopLogger(),
			ctx,
			containers,
			client,
			types.UpdateParams{Timeout: 0},
			stoppedImages,
			&cleanup,
			&progress,
			nil,
		)

		gomega.Expect(failed).To(gomega.BeEmpty(), "stopped containers must still be recreated after cancel")
		gomega.Expect(client.TestData.CreateContainerCount.Load()).To(gomega.Equal(int32(3)))
		gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(3)))
	})
})

// rollingRestartTestCase represents a test case for performRollingRestart cancellation.
type rollingRestartTestCase struct {
	name                string
	numContainers       int
	cancelAtIndex       int    // Index at which to cancel (-1 means no cancellation)
	expectedProcessed   int    // Number of containers that should be processed
	expectedSkipped     int    // Number of containers that should be skipped
	expectedLogMessages int    // Expected number of log messages for skipped containers
	description         string // Human-readable description
}

// Tests for performRollingRestart cancellation handling.
// These tests verify that when context cancellation occurs during rolling restart:
// 1. All remaining containers are logged with appropriate fields
// 2. All remaining containers are added to the failed map with wrapped errors
// 3. Error messages contain "restart skipped"
// 4. The returned error wraps context.Canceled
// 5. Edge cases (cancellation at start, middle, end) are handled correctly.
//
// Important: When context is canceled at index i, the function adds the current container
// at index i to the failed map, then adds containers from i+1 to the end.
// All containers from i to the end are properly logged and tracked as failed.
var _ = ginkgo.Describe("performRollingRestart", func() {
	ginkgo.When("context is canceled during iteration", func() {
		// Table-driven tests for various cancellation scenarios.
		// Note: When context is already canceled at the start (i = 0),
		// all containers from 0 to len-1 are added to failed.
		testCases := []rollingRestartTestCase{
			{
				name:                "cancellation_at_start_all_skipped",
				numContainers:       3,
				cancelAtIndex:       0, // Context already canceled - at i=0, containers 0,1,2 are skipped
				expectedProcessed:   0,
				expectedSkipped:     3, // containers 0, 1, and 2 are added to failed
				expectedLogMessages: 3,
				description:         "When context is canceled at the start, all containers should be skipped",
			},
			{
				name:                "cancellation_in_middle_partial_skip",
				numContainers:       5,
				cancelAtIndex:       0, // Context already canceled - at i=0, containers 0,1,2,3,4 are skipped
				expectedProcessed:   0,
				expectedSkipped:     5, // containers 0,1,2,3,4 are added to failed
				expectedLogMessages: 5,
				description:         "When context is canceled mid-iteration, all containers should be skipped",
			},
			{
				name:                "cancellation_at_end_no_skip",
				numContainers:       3,
				cancelAtIndex:       -1, // No cancellation
				expectedProcessed:   3,
				expectedSkipped:     0,
				expectedLogMessages: 0,
				description:         "When no cancellation occurs, all containers should be processed",
			},
			{
				name:                "single_container_canceled",
				numContainers:       1,
				cancelAtIndex:       0, // Context already canceled - at i=0, container 0 is skipped
				expectedProcessed:   0,
				expectedSkipped:     1, // container 0 is added to failed
				expectedLogMessages: 1,
				description:         "Single container scenario with cancellation",
			},
			{
				name:                "single_container_not_canceled",
				numContainers:       1,
				cancelAtIndex:       -1, // No cancellation
				expectedProcessed:   1,
				expectedSkipped:     0,
				expectedLogMessages: 0,
				description:         "Single container scenario without cancellation",
			},
		}

		for _, tc := range testCases {
			ginkgo.It(tc.name, func() {
				ginkgo.By(tc.description)

				// Create mock containers with ToRestart set to true.
				containers := make([]types.Container, tc.numContainers)
				for i := range tc.numContainers {
					containerID := fmt.Sprintf("container-%d", i)
					containerName := fmt.Sprintf("/container-%d", i)
					imageName := fmt.Sprintf("image-%d:latest", i)

					c := mockActions.CreateMockContainerWithConfig(
						containerID,
						containerName,
						imageName,
						true,
						false,
						time.Now(),
						&dockerContainer.Config{
							Labels:       map[string]string{},
							ExposedPorts: dockerNetwork.PortSet{},
						},
					)
					// Mark container for restart so it will be processed.
					c.SetStale(true)
					containers[i] = c
				}

				// Create mock client.
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: containers,
						Staleness:  make(map[string]bool),
					},
					false,
					false,
				)

				// Mark all containers as stale.
				for i := range tc.numContainers {
					client.TestData.Staleness[fmt.Sprintf("container-%d", i)] = true
				}

				// Set up log capture to verify log messages.
				log, logBuf := newCaptureLogger()

				// Create context - either canceled or not based on test case.
				ctx := context.Background()
				if tc.cancelAtIndex >= 0 {
					// Create an already-canceled context to simulate cancellation.
					canceledCtx, cancel := context.WithCancel(context.Background())
					cancel() // Cancel immediately

					ctx = canceledCtx
				}

				// Call performRollingRestart.
				var cleanupImageInfos []types.RemovedImageInfo

				failed, err := performRollingRestart(log,
					ctx,
					containers,
					client,
					types.UpdateParams{},
					&cleanupImageInfos,
					nil, // progress is not needed for this test
					nil,
				)

				// Verify the number of failed containers.
				gomega.Expect(failed).
					To(gomega.HaveLen(tc.expectedSkipped), "Expected %d failed containers", tc.expectedSkipped)

				// Verify error is returned when context is canceled.
				if tc.cancelAtIndex >= 0 {
					gomega.Expect(err).To(gomega.HaveOccurred(), "Expected an error when context is canceled")
					gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue(),
						"Error should wrap context.Canceled")
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("rolling restart canceled"),
						"Error message should contain 'rolling restart canceled'")
				} else {
					gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Expected no error when context is not canceled")
				}

				// Verify log messages for skipped containers.
				skippedLogCount := 0

				for _, entry := range parseJSONLogEntries(logBuf) {
					if entry["message"] == "Skipped container restart due to context cancellation" {
						skippedLogCount++

						// Verify log fields contain expected keys.
						gomega.Expect(entry).To(gomega.HaveKey("container"))
						gomega.Expect(entry).To(gomega.HaveKey("image"))
						gomega.Expect(entry).To(gomega.HaveKey("container_id"))
					}
				}

				gomega.Expect(skippedLogCount).
					To(gomega.Equal(tc.expectedLogMessages), "Expected %d log messages for skipped containers", tc.expectedLogMessages)
			})
		}
	})

	ginkgo.When("context is canceled mid-iteration", func() {
		ginkgo.It("should add remaining containers to failed map with wrapped error", func() {
			// Create 4 containers.
			// When context is already canceled at the start:
			// - At i=0, ctx.Done() is triggered, so container 0 is added to failed
			// - Then containers 1,2,3 are also added to failed
			// - All 4 containers are properly tracked as failed
			containers := make([]types.Container, 4)

			for i := range 4 {
				containerID := fmt.Sprintf("container-%d", i)
				containerName := fmt.Sprintf("/container-%d", i)
				imageName := fmt.Sprintf("image-%d:latest", i)

				c := mockActions.CreateMockContainerWithConfig(
					containerID,
					containerName,
					imageName,
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
						"container-3": true,
					},
				},
				false,
				false,
			)

			// Create a canceled context.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newNopLogger(),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// All 4 containers should be in failed map (containers 0, 1, 2, 3).
			gomega.Expect(failed).To(gomega.HaveLen(4))

			// Verify error is returned.
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue(),
				"Error should wrap context.Canceled")
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("rolling restart canceled"))

			// Verify all containers 0, 1, 2, 3 are in failed map with wrapped error.
			for i := range 4 {
				containerID := types.ContainerID(fmt.Sprintf("container-%d", i))
				containerErr, exists := failed[containerID]
				gomega.Expect(exists).To(gomega.BeTrue(), "Container %d should be in failed map", i)

				// Verify error message contains "restart skipped".
				gomega.Expect(containerErr.Error()).To(gomega.ContainSubstring("restart skipped"))

				// Verify error wraps context.Canceled.
				gomega.Expect(errors.Is(containerErr, context.Canceled)).To(gomega.BeTrue(),
					"Error should wrap context.Canceled")
			}
		})

		ginkgo.It("should log each skipped container with correct fields", func() {
			// Create containers.
			// When context is already canceled at the start:
			// - At i=0, ctx.Done() is triggered, so container 0 is logged and added to failed
			// - Then containers 1,2 are also logged and added to failed
			// - All 3 containers are properly logged
			containers := make([]types.Container, 3)
			expectedNames := []string{"container-0", "container-1", "container-2"} // All 3 are logged

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
				},
				false,
				false,
			)

			// Set up log capture.
			log, logBuf := newCaptureLogger()

			// Create a canceled context.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			_, _ = performRollingRestart(log,
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify log entries contain expected container details.
			loggedNames := make(map[string]bool)

			for _, entry := range parseJSONLogEntries(logBuf) {
				if entry["message"] == "Skipped container restart due to context cancellation" {
					containerName, ok := entry["container"]
					if ok {
						loggedNames[containerName.(string)] = true
					}

					// Verify all expected fields are present.
					gomega.Expect(entry).To(gomega.HaveKey("container"))
					gomega.Expect(entry).To(gomega.HaveKey("image"))
					gomega.Expect(entry).To(gomega.HaveKey("container_id"))
				}
			}

			// Verify all containers were logged.
			for _, name := range expectedNames {
				gomega.Expect(loggedNames).To(gomega.HaveKey(name),
					"Container %s should have been logged", name)
			}

			// Verify we got the expected number of log messages.
			gomega.Expect(loggedNames).To(gomega.HaveLen(3))
		})
	})

	ginkgo.When("context is not canceled", func() {
		ginkgo.It("should process all containers without adding to failed map", func() {
			// Create containers.
			containers := make([]types.Container, 3)

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
				},
				false,
				false,
			)

			// Set up log capture.
			log, logBuf := newCaptureLogger()

			// Call with valid context.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(log,
				context.Background(),
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// All containers should be processed, none failed due to cancellation.
			gomega.Expect(failed).To(gomega.BeEmpty())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify no "Skipped container restart" log messages.
			for _, entry := range parseJSONLogEntries(logBuf) {
				gomega.Expect(entry["message"]).
					NotTo(gomega.Equal("Skipped container restart due to context cancellation"))
			}
		})
	})

	ginkgo.When("containers are processed in forward order", func() {
		ginkgo.It("should process containers from first to last", func() {
			// Create containers.
			containers := make([]types.Container, 3)

			for i := range 3 {
				c := mockActions.CreateMockContainerWithConfig(
					fmt.Sprintf("container-%d", i),
					fmt.Sprintf("/container-%d", i),
					fmt.Sprintf("image-%d:latest", i),
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)
				c.SetStale(true)
				containers[i] = c
			}

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-0": true,
						"container-1": true,
						"container-2": true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			// Call with valid context.
			var cleanupImageInfos []types.RemovedImageInfo

			_, _ = performRollingRestart(newNopLogger(),
				context.Background(),
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify create order is forward (container-0, container-1, container-2).
			gomega.Expect(client.TestData.CreateOrder).To(gomega.HaveLen(3))
			gomega.Expect(client.TestData.CreateOrder[0]).To(gomega.Equal("container-0"))
			gomega.Expect(client.TestData.CreateOrder[1]).To(gomega.Equal("container-1"))
			gomega.Expect(client.TestData.CreateOrder[2]).To(gomega.Equal("container-2"))
		})
	})

	ginkgo.When("verifying error wrapping", func() {
		ginkgo.It("should return error that wraps context.Canceled", func() {
			// Create a single container.
			containers := []types.Container{
				mockActions.CreateMockContainerWithConfig(
					"container-0",
					"/container-0",
					"image-0:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				),
			}
			containers[0].SetStale(true)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness:  map[string]bool{"container-0": true},
				},
				false,
				false,
			)

			// Create a canceled context.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newNopLogger(),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify error is returned.
			gomega.Expect(err).To(gomega.HaveOccurred())

			// Verify error wraps context.Canceled using errors.Is.
			gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue(),
				"Error should wrap context.Canceled")

			// Verify error message format.
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("rolling restart canceled"))

			// Verify failed container error also wraps context.Canceled.
			gomega.Expect(failed).To(gomega.HaveLen(1))
			containerErr := failed[types.ContainerID("container-0")]
			gomega.Expect(containerErr).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(containerErr, context.Canceled)).To(gomega.BeTrue(),
				"Container error should wrap context.Canceled")
			gomega.Expect(containerErr.Error()).To(gomega.ContainSubstring("restart skipped"))
		})

		ginkgo.It("should return error that wraps context.DeadlineExceeded when deadline exceeded", func() {
			// Create a single container.
			containers := []types.Container{
				mockActions.CreateMockContainerWithConfig(
					"container-0",
					"/container-0",
					"image-0:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				),
			}
			containers[0].SetStale(true)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness:  map[string]bool{"container-0": true},
				},
				false,
				false,
			)

			// Create a context with an already-expired deadline.
			deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
			defer cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newNopLogger(),
				deadlineCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify error is returned.
			gomega.Expect(err).To(gomega.HaveOccurred())

			// Verify error wraps context.DeadlineExceeded using errors.Is.
			gomega.Expect(errors.Is(err, context.DeadlineExceeded)).To(gomega.BeTrue(),
				"Error should wrap context.DeadlineExceeded")

			// Verify error message format.
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("rolling restart canceled"))

			// Verify failed container error also wraps context.DeadlineExceeded.
			gomega.Expect(failed).To(gomega.HaveLen(1))
			containerErr := failed[types.ContainerID("container-0")]
			gomega.Expect(containerErr).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(containerErr, context.DeadlineExceeded)).To(gomega.BeTrue(),
				"Container error should wrap context.DeadlineExceeded")
			gomega.Expect(containerErr.Error()).To(gomega.ContainSubstring("restart skipped"))
		})
	})
})

var _ = ginkgo.Describe("deduplicateByImageID", func() {
	ginkgo.It("should return empty slice for empty input", func() {
		result := deduplicateByImageID([]types.RemovedImageInfo{})
		gomega.Expect(result).To(gomega.BeEmpty())
	})

	ginkgo.It("should return all entries when all ImageIDs are unique", func() {
		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:bbb"), ContainerName: "container-b"},
		}

		result := deduplicateByImageID(images)
		gomega.Expect(result).To(gomega.HaveLen(2))
		gomega.Expect(result[0].ImageID).To(gomega.Equal(types.ImageID("sha256:aaa")))
		gomega.Expect(result[1].ImageID).To(gomega.Equal(types.ImageID("sha256:bbb")))
	})

	ginkgo.It("should keep only first occurrence when ImageIDs are duplicated", func() {
		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-b"},
		}

		result := deduplicateByImageID(images)
		gomega.Expect(result).To(gomega.HaveLen(1))
		// Should keep the first occurrence with container-a
		gomega.Expect(result[0].ContainerName).To(gomega.Equal("container-a"))
	})

	ginkgo.It("should handle mixed unique and duplicate ImageIDs", func() {
		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:bbb"), ContainerName: "container-b"},
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-c"},
			{ImageID: types.ImageID("sha256:ccc"), ContainerName: "container-d"},
		}

		result := deduplicateByImageID(images)
		gomega.Expect(result).To(gomega.HaveLen(3))
		gomega.Expect(result[0].ImageID).To(gomega.Equal(types.ImageID("sha256:aaa")))
		gomega.Expect(result[0].ContainerName).To(gomega.Equal("container-a"))
		gomega.Expect(result[1].ImageID).To(gomega.Equal(types.ImageID("sha256:bbb")))
		gomega.Expect(result[2].ImageID).To(gomega.Equal(types.ImageID("sha256:ccc")))
	})
})

var _ = ginkgo.Describe("isPinned", func() {
	// Valid 64-hex digest used by Docker/OCI references.
	const fullDigest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	ginkgo.DescribeTable("digest pin detection via shared helper",
		func(image string, wantPinned bool) {
			cont := mockActions.CreateMockContainer(
				"id1",
				"/name",
				image,
				time.Now(),
			)
			progress := session.Progress{}
			params := types.UpdateParams{}

			pinned, err := isPinned(newNopLogger(), cont, &progress, params, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pinned).To(gomega.Equal(wantPinned))

			if wantPinned {
				gomega.Expect(progress).To(gomega.HaveLen(1), "pinned containers are marked scanned")
			} else {
				gomega.Expect(progress).To(gomega.BeEmpty(), "unpinned containers are not scanned here")
			}
		},
		ginkgo.Entry("tagged image", "nginx:latest", false),
		ginkgo.Entry("repo@digest", "nginx@"+fullDigest, true),
		ginkgo.Entry("tag and digest", "nginx:1.27@"+fullDigest, true),
		ginkgo.Entry("registry/repo@digest", "registry.example.com/org/app@"+fullDigest, true),
		ginkgo.Entry("bare content digest", fullDigest, true),
	)

	ginkgo.It("detects pin before parse fallback can replace the image name", func() {
		// ImageName is a valid digest pin. Even if parse were flaky, pin must win.
		cont := mockActions.CreateMockContainer(
			"pinned-id",
			"/pinned-name",
			"library/nginx@"+fullDigest,
			time.Now(),
		)
		progress := session.Progress{}

		pinned, err := isPinned(newNopLogger(), cont, &progress, types.UpdateParams{}, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pinned).To(gomega.BeTrue())
		gomega.Expect(progress).To(gomega.HaveLen(1))
	})

	ginkgo.It("returns error for unresolvable invalid image names", func() {
		cont := mockActions.CreateMockContainer(
			"InvalidContainer",
			"/InvalidContainer",
			":latest",
			time.Now(),
		)
		progress := session.Progress{}

		pinned, err := isPinned(newNopLogger(), cont, &progress, types.UpdateParams{}, nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(pinned).To(gomega.BeFalse())
		gomega.Expect(progress).To(gomega.BeEmpty())
	})

	ginkgo.It("does not panic when ContainerInfo Config is nil", func() {
		cont := mockTypes.NewMockContainer(ginkgo.GinkgoT())
		cont.On("Name").Return("/nil-config")
		cont.On("ImageName").Return("nginx:latest")
		cont.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: nil,
		})

		progress := session.Progress{}

		pinned, err := isPinned(newNopLogger(), cont, &progress, types.UpdateParams{}, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pinned).To(gomega.BeFalse())
	})
})
