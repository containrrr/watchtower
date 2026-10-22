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

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
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

const (
	currentWatchtowerID = "current-watchtower-id"
	otherWatchtowerID   = "other-watchtower-id"
)

// Test_restartStaleContainer verifies when restarting a stale Watchtower
// container renames it first, including run-once mode, an existing old name,
// and an ephemeral self-update handoff.
func Test_restartStaleContainer(t *testing.T) {
	t.Parallel()

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
		params := types.UpdateParams{
			RunOnce: true,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newTestLogger(t), t.Context(), testContainer, client, params)
		require.NoError(t, err)
		assert.False(t, renamed)
		assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load())
		assert.NotEmpty(t, newID)
	})

	t.Run("should rename Watchtower container when not in run-once mode", func(t *testing.T) {
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
		params := types.UpdateParams{
			RunOnce: false,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newTestLogger(t), t.Context(), testContainer, client, params)
		require.NoError(t, err)
		assert.True(t, renamed)
		assert.Equal(t, int32(1), client.TestData.RenameContainerCount.Load())
		assert.NotEmpty(t, newID)
	})

	t.Run("should skip rename if source container is already named with the target old name", func(t *testing.T) {
		t.Parallel()

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
		newID, renamed, err := restartStaleContainer(newTestLogger(t), t.Context(), testContainer, client, params)
		require.NoError(t, err)
		assert.True(t, renamed)
		assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load())
		assert.NotEmpty(t, newID)
	})

	t.Run("should treat ephemeral self-update as a completed handoff", func(t *testing.T) {
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
		params := types.UpdateParams{
			EphemeralSelfUpdate: true,
			Cleanup:             true,
		}
		testContainer := client.TestData.Containers[0]
		newID, renamed, err := restartStaleContainer(newTestLogger(t), t.Context(), testContainer, client, params)
		require.NoError(t, err)
		assert.True(t, renamed)
		assert.Empty(t, newID)
		assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load())
		assert.True(t, client.TestData.LastCleanup)
	})
}

// Test_handleUpdateResult verifies the metric and notification an update
// result produces, with and without an error or a partial report.
func Test_handleUpdateResult(t *testing.T) {
	t.Parallel()

	t.Run("should return a failed zero metric when error is not nil and there is no report", func(t *testing.T) {
		t.Parallel()

		err := errors.New("test error")
		result := handleUpdateResult(newTestLogger(t), nil, err, nil)
		assert.Equal(t, &metrics.Metric{ScanFailed: true}, result)
	})

	t.Run("should return the partial report's counts, marked failed, when error is not nil", func(t *testing.T) {
		t.Parallel()

		mockReport := mockTypes.NewMockReport(t)
		mockReport.EXPECT().Scanned().Return(make([]types.ContainerReport, 3))
		mockReport.EXPECT().Updated().Return(make([]types.ContainerReport, 1))
		mockReport.EXPECT().Failed().Return(make([]types.ContainerReport, 2))
		mockReport.EXPECT().Restarted().Return(nil)
		mockReport.EXPECT().Skipped().Return(nil)

		err := errors.New("rolling restart canceled")
		result := handleUpdateResult(newTestLogger(t), mockReport, err, nil)
		assert.Equal(t, &metrics.Metric{Scanned: 3, Updated: 1, Failed: 2, ScanFailed: true}, result)
	})

	t.Run("should return a failed zero metric when result is nil", func(t *testing.T) {
		t.Parallel()

		var err error

		result := handleUpdateResult(newTestLogger(t), nil, err, nil)
		assert.Equal(t, &metrics.Metric{ScanFailed: true}, result)
	})

	t.Run("should return nil when result is not nil and error is nil", func(t *testing.T) {
		t.Parallel()

		mockReport := mockTypes.NewMockReport(t)

		var err error

		result := handleUpdateResult(newTestLogger(t), mockReport, err, nil)
		assert.Nil(t, result)
	})

	t.Run("should send an empty report when error occurs without a report", func(t *testing.T) {
		t.Parallel()

		// Create a mock notifier that tracks if SendNotification was called
		mockNotifier := mockTypes.NewMockNotifier(t)
		mockNotifier.EXPECT().SendNotification(emptyReport{}).Times(1)

		// Call handleUpdateResult with an error and the mock notifier
		err := errors.New("dependency resolution error")
		result := handleUpdateResult(newTestLogger(t), nil, err, mockNotifier)

		// Verify we got the expected metric
		assert.Equal(t, &metrics.Metric{ScanFailed: true}, result)
	})

	t.Run("should send the partial report when error occurs after some work", func(t *testing.T) {
		t.Parallel()

		mockReport := mockTypes.NewMockReport(t)
		mockReport.EXPECT().Scanned().Return(nil)
		mockReport.EXPECT().Updated().Return(nil)
		mockReport.EXPECT().Failed().Return(nil)
		mockReport.EXPECT().Restarted().Return(nil)
		mockReport.EXPECT().Skipped().Return(nil)

		mockNotifier := mockTypes.NewMockNotifier(t)
		mockNotifier.EXPECT().SendNotification(mockReport).Times(1)

		err := errors.New("rolling restart canceled")
		handleUpdateResult(newTestLogger(t), mockReport, err, mockNotifier)
	})

	t.Run("should not send notification when error occurs and notifier is nil", func(t *testing.T) {
		t.Parallel()

		// Call handleUpdateResult with an error and nil notifier
		err := errors.New("dependency resolution error")
		result := handleUpdateResult(newTestLogger(t), nil, err, nil)

		// Verify we got the expected metric
		assert.Equal(t, &metrics.Metric{ScanFailed: true}, result)
	})

	t.Run("should not send notification when there is no error", func(t *testing.T) {
		t.Parallel()

		// Create a mock notifier with no expectations (will fail if any method is called)
		mockNotifier := mockTypes.NewMockNotifier(t)

		// Call handleUpdateResult without an error
		mockReport := mockTypes.NewMockReport(t)

		var err error

		result := handleUpdateResult(newTestLogger(t), mockReport, err, mockNotifier)

		// Verify we got the expected result
		assert.Nil(t, result)
	})
}

// Test_executeUpdate verifies that an update session runs the update for
// stale containers, honors run-once mode, and sets Watchtower's restart policy.
func Test_executeUpdate(t *testing.T) {
	t.Parallel()

	t.Run("should execute update successfully", func(t *testing.T) {
		t.Parallel()

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
		report, cleanupInfos, err := executeUpdate(newTestLogger(t),
			t.Context(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, cleanupInfos)
	})

	t.Run("should not return error when no containers to update", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(
			&mockActions.TestData{},
			false,
			false,
		)
		config := types.UpdateParams{
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newTestLogger(t),
			t.Context(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, cleanupInfos)
	})

	t.Run("should execute update logic for stale containers", func(t *testing.T) {
		t.Parallel()

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
		report, cleanupInfos, err := executeUpdate(newTestLogger(t),
			t.Context(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, cleanupInfos)
		assert.Equal(t, int32(1), client.TestData.StartContainerCount.Load())
	})

	t.Run("should propagate RunOnce mode and skip Watchtower self-update", func(t *testing.T) {
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
			Filter:  filters.NoFilter,
			RunOnce: true,
		}
		report, cleanupInfos, err := executeUpdate(newTestLogger(t),
			t.Context(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, cleanupInfos)
		assert.Equal(t, int32(0), client.TestData.RenameContainerCount.Load())
		assert.Equal(t, int32(0), client.TestData.StartContainerCount.Load())
	})

	t.Run("should call SetRestartPolicy for Watchtower restart policy changes", func(t *testing.T) {
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
			Filter: filters.NoFilter,
		}
		report, cleanupInfos, err := executeUpdate(newTestLogger(t),
			t.Context(),
			client,
			config,
			ratelimit.New(),
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, report)
		require.NotNil(t, cleanupInfos)
		assert.Equal(t, int32(1), client.TestData.SetRestartPolicyCount.Load())
	})
}

// Test_shouldUpdateContainer verifies that Watchtower updates only itself among
// Watchtower containers, within its scope, and leaves other containers alone.
func Test_shouldUpdateContainer(t *testing.T) {
	t.Parallel()

	t.Run("should allow self-update of current Watchtower container", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should skip other Watchtower containers from self-updates", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should not affect non-Watchtower containers", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should allow self-update of scoped Watchtower container", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should skip other scoped Watchtower containers with same scope from self-updates", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should skip other scoped Watchtower containers with different scopes from self-updates", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should skip unscoped Watchtower containers from scoped self-updates", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should skip scoped Watchtower containers from unscoped self-updates", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should allow update of non-Watchtower containers when NoRestart is true", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should allow update of Watchtower containers when NoRestart is true", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})
}

// Test_hasSelfDependency verifies that a container whose depends-on label
// names itself is detected.
func Test_hasSelfDependency(t *testing.T) {
	t.Parallel()

	t.Run("should return false when no depends-on label is present", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should return false when depends-on label is empty", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should return false when depends-on contains other containers", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should return true when depends-on contains self", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should return true when depends-on contains self among multiple dependencies", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should handle spaces and trimming correctly", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should handle leading slashes in container names", func(t *testing.T) {
		t.Parallel()

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
		assert.True(t, result)
	})

	t.Run("should return false when Config is nil", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})

	t.Run("should return false when Labels is nil", func(t *testing.T) {
		t.Parallel()

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
		assert.False(t, result)
	})
}

// Test_emptyReport verifies that every list of the empty report is nil.
func Test_emptyReport(t *testing.T) {
	t.Parallel()

	t.Run("Scanned() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Scanned())
	})

	t.Run("Updated() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Updated())
	})

	t.Run("Failed() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Failed())
	})

	t.Run("Skipped() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Skipped())
	})

	t.Run("Stale() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Stale())
	})

	t.Run("Fresh() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Fresh())
	})

	t.Run("Restarted() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.Restarted())
	})

	t.Run("All() should return nil", func(t *testing.T) {
		t.Parallel()

		report := emptyReport{}
		assert.Nil(t, report.All())
	})
}

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
func TestDetachedContext(t *testing.T) {
	t.Parallel()

	// TestDetachedContextDeadlineCase represents a test case for detached context deadline behavior.
	type TestDetachedContextDeadlineCase struct {
		name            string
		timeout         time.Duration
		expectDeadline  bool
		expectedTimeout time.Duration
		description     string
	}

	t.Run("restartStaleContainer detached context deadline", func(t *testing.T) {
		t.Parallel()

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
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

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
				newID, renamed, err := restartStaleContainer(newTestLogger(t),
					t.Context(),
					testContainer,
					client,
					params,
				)

				// Verify the operation succeeded.
				require.NoError(t, err)
				assert.True(t, renamed)
				assert.NotEmpty(t, newID)

				// Verify SetRestartPolicy was called (this uses the detached context).
				// The detached context is used for updating the restart policy of the
				// renamed Watchtower container.
				assert.Equal(t, int32(1), client.TestData.SetRestartPolicyCount.Load())

				// Verify CreateContainer and StartContainerByID also use the detached context.
				// These operations run after the initial rename, so they should share the same
				// deadline when expectDeadline is true.
				createCtx := client.TestData.CreateContainerCtx
				startCtx := client.TestData.StartContainerByIDCtx
				noRestartCtx := client.TestData.SetRestartPolicyCtx

				require.NotNil(t, createCtx, "CreateContainer should receive a context")
				require.NotNil(t, startCtx, "StartContainerByID should receive a context")
				assert.Same(t, startCtx, createCtx, "CreateContainer and StartContainerByID should share the same detached context")
				assert.Same(t, noRestartCtx, createCtx, "CreateContainer should use the same detached context as SetRestartPolicy")

				if tc.expectDeadline {
					_, createHasDeadline := createCtx.Deadline()
					_, startHasDeadline := startCtx.Deadline()

					assert.True(t, createHasDeadline)
					assert.True(t, startHasDeadline)

					expectedDeadline := time.Now().Add(tc.expectedTimeout)
					createDeadline, _ := createCtx.Deadline()
					startDeadline, _ := startCtx.Deadline()

					assert.WithinDuration(t, expectedDeadline, createDeadline, time.Second)
					assert.WithinDuration(t, expectedDeadline, startDeadline, time.Second)
				} else {
					_, createHasDeadline := createCtx.Deadline()
					_, startHasDeadline := startCtx.Deadline()

					assert.False(t, createHasDeadline)
					assert.False(t, startHasDeadline)
				}

				// Verify CreateContainer and StartContainerByID also use the detached context.
				// These operations run after CreateContainer succeeds, so the same deadline
				// should apply when expectDeadline is true.
				assert.Equal(t, int32(1), client.TestData.CreateContainerCount.Load())
				assert.Equal(t, int32(1), client.TestData.StartContainerCount.Load())

				if tc.expectDeadline {
					createDeadline, createHasDeadline := client.TestData.CreateContainerCtx.Deadline()
					assert.True(t, createHasDeadline)

					startDeadline, startHasDeadline := client.TestData.StartContainerByIDCtx.Deadline()
					assert.True(t, startHasDeadline)

					expectedDeadline := time.Now().Add(tc.expectedTimeout)
					assert.WithinDuration(t, expectedDeadline, createDeadline, time.Second)
					assert.WithinDuration(t, expectedDeadline, startDeadline, time.Second)
				}
			})
		}
	})

	t.Run("Watchtower self-update create failure recovery", func(t *testing.T) {
		t.Parallel()

		t.Run("renames the source back to the original name when create fails", func(t *testing.T) {
			t.Parallel()

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
			parentCtx, parentCancel := context.WithCancel(t.Context())
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
				_, renamed, err = restartStaleContainer(newTestLogger(t),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for CreateContainer to be called, indicating the initial rename
			// has completed. Then cancel the parent context.
			require.Eventually(t, func() bool {
				return client.TestData.CreateContainerCount.Load() > 0
			}, 5*time.Second, 10*time.Millisecond)

			parentCancel()
			wg.Wait()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create container")
			assert.False(t, renamed, "name should be restored after create failure")
			assert.Equal(t, int32(2), client.TestData.RenameContainerCount.Load())
			require.Len(t, client.TestData.RenameTargets, 2)
			assert.Equal(t, "watchtower", client.TestData.RenameTargets[1])
			assert.Equal(t, int32(0), client.TestData.StartContainerCount.Load())

			// Assert that CreateContainer and RenameContainer received distinct contexts.
			// The first rename (initial handoff) uses the caller context, the second
			// rename (recovery after create failure) uses a fresh bounded context.
			createCtx := client.TestData.CreateContainerCtx
			renameBackCtx := client.TestData.RenameContainerCtx

			require.NotNil(t, createCtx, "CreateContainer should receive a context")
			require.NotNil(t, renameBackCtx, "RenameContainer should receive a context for rename-back")
			assert.NotSame(t, renameBackCtx, createCtx, "CreateContainer and rename-back should use distinct contexts")

			// Assert that the rename-back context has an active deadline (is a bounded timeout context).
			_, hasDeadline := renameBackCtx.Deadline()
			assert.True(t, hasDeadline, "rename-back context should have a deadline for bounded recovery")
		})
	})

	t.Run("Watchtower self-update start failure cleanup", func(t *testing.T) {
		t.Parallel()

		t.Run("removes the newly created container, not the renamed source", func(t *testing.T) {
			t.Parallel()

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
			parentCtx, parentCancel := context.WithCancel(t.Context())
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
				_, renamed, err = restartStaleContainer(newTestLogger(t),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for StartContainer to be called, indicating the initial rename
			// and create have completed. Then cancel the parent context.
			require.Eventually(t, func() bool {
				return client.TestData.StartContainerCount.Load() > 0
			}, 5*time.Second, 10*time.Millisecond)

			parentCancel()
			wg.Wait()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to start container")
			assert.True(t, renamed)
			assert.Equal(t, int32(1), client.TestData.CreateContainerCount.Load())
			assert.NotEmpty(t, client.TestData.LastCreatedContainerID)
			assert.Equal(t, int32(1), client.TestData.StopAndRemoveContainerCount.Load())
			assert.Equal(t, int32(1), client.TestData.RemoveContainerCount.Load())
			assert.Equal(t, client.TestData.LastCreatedContainerID, client.TestData.LastStopAndRemoveID)
			assert.NotEqual(t, testContainer.ID(), client.TestData.LastStopAndRemoveID)

			// Assert that GetContainer and StopAndRemoveContainer receive a fresh
			// context distinct from the create/start detached context.
			getCtx := client.TestData.GetContainerCtx
			cleanupCtx := client.TestData.StopAndRemoveContainerCtx
			createCtx := client.TestData.CreateContainerCtx

			require.NotNil(t, getCtx, "GetContainer should receive a context")
			require.NotNil(t, cleanupCtx, "StopAndRemoveContainer should receive a context")
			assert.Same(t, cleanupCtx, getCtx, "GetContainer and StopAndRemoveContainer should share the same cleanup context")
			assert.NotSame(t, createCtx, getCtx, "cleanup context should be distinct from the create/start detached context")

			// Assert that the cleanup context has an active deadline (is a bounded timeout context).
			_, hasDeadline := getCtx.Deadline()
			assert.True(t, hasDeadline, "cleanup context should have a deadline for bounded recovery")
		})
	})

	t.Run("restartStaleContainer detached context survival", func(t *testing.T) {
		t.Parallel()

		t.Run("cleanup operations complete when parent context is canceled during execution", func(t *testing.T) {
			t.Parallel()

			// Create a parent context that we will cancel while restartStaleContainer is running.
			parentCtx, parentCancel := context.WithCancel(t.Context())

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
				_, renamed, err = restartStaleContainer(newTestLogger(t),
					parentCtx,
					testContainer,
					client,
					params,
				)
			})

			// Wait for StartContainer to be called (which means RenameContainer has completed)
			// before canceling the parent context. This ensures we cancel at the right moment -
			// after rename succeeds but during/after start fails.
			require.Eventually(t, func() bool {
				return client.TestData.StartContainerCount.Load() > 0
			}, 5*time.Second, 10*time.Millisecond)

			// Cancel the parent context after StartContainer has been called.
			// The detached context should allow cleanup to proceed even though
			// the parent context is canceled.
			parentCancel()

			// Wait for the goroutine to complete.
			wg.Wait()

			// The operation should fail due to StartContainerByID error, but the
			// cleanup (StopAndRemoveContainer) should have been attempted
			// using the detached context, which survives parent cancellation.
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to start container")
			assert.True(t, renamed)

			// Verify that RemoveContainer was called during cleanup.
			// This demonstrates that the detached context allowed the cleanup
			// operation to proceed even though the parent context was canceled.
			assert.Equal(t, int32(1), client.TestData.RemoveContainerCount.Load())
		})

		t.Run("cleanup operations complete when parent context is already canceled", func(t *testing.T) {
			t.Parallel()

			// Create a parent context that is already canceled.
			parentCtx, parentCancel := context.WithCancel(t.Context())
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
			_, renamed, err := restartStaleContainer(newTestLogger(t),
				parentCtx,
				testContainer,
				client,
				params,
			)

			// The operation should fail at RenameContainer due to parent context cancellation.
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to rename Watchtower container")
			assert.False(t, renamed)

			// RenameContainer should have been attempted but failed due to context cancellation.
			assert.Equal(t, int32(1), client.TestData.RenameContainerCount.Load())
		})

		t.Run("restart policy update uses detached context after successful start", func(t *testing.T) {
			t.Parallel()

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
			newID, renamed, err := restartStaleContainer(newTestLogger(t),
				t.Context(),
				testContainer,
				client,
				params,
			)

			// The operation should succeed completely.
			require.NoError(t, err)
			assert.True(t, renamed)
			assert.NotEmpty(t, newID)

			// Verify that both StartContainer and SetRestartPolicy were called.
			// SetRestartPolicy uses the detached context for the restart policy update.
			assert.Equal(t, int32(1), client.TestData.StartContainerCount.Load())
			assert.Equal(t, int32(1), client.TestData.SetRestartPolicyCount.Load())

			restartCtx := client.TestData.SetRestartPolicyCtx
			require.NotNil(t, restartCtx, "SetRestartPolicy should receive a context")
			assert.Same(t, client.TestData.CreateContainerCtx, restartCtx, "SetRestartPolicy should use the shared detached context")

			_, hasDeadline := restartCtx.Deadline()
			assert.True(t, hasDeadline, "restart policy context should have a deadline")
		})
	})
}

// Tests for stopContainersInReversedOrder cancellation handling.
// These tests verify that when context cancellation occurs during container stopping:
// 1. All remaining containers are logged with appropriate fields
// 2. All remaining containers are added to the failed map with wrapped errors
// 3. Edge cases (cancellation at start, middle, end) are handled correctly.
//
// Important: When context is canceled at index i, the function adds the current container
// at index i to the failed map, then adds containers from i-1 down to 0.
// All containers from i down to 0 are properly logged and tracked as failed.
func Test_stopContainersInReversedOrder(t *testing.T) {
	t.Parallel()

	t.Run("context is canceled during iteration", func(t *testing.T) {
		t.Parallel()

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
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				t.Log(tc.description)

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
				ctx := t.Context()
				if tc.cancelAtIndex >= 0 {
					// Create an already-canceled context to simulate cancellation.
					canceledCtx, cancel := context.WithCancel(t.Context())
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
				require.Len(t, stopped, tc.expectedStopped, "Expected %d stopped containers", tc.expectedStopped)

				// Verify the number of failed containers.
				require.Len(t, failed, tc.expectedSkipped, "Expected %d failed containers", tc.expectedSkipped)

				// Verify log messages for skipped containers.
				skippedLogCount := 0

				for _, entry := range parseJSONLogEntries(logBuf) {
					if entry["message"] == "Skipped container stop due to context cancellation" {
						skippedLogCount++

						// Verify log fields contain expected keys.
						assert.Contains(t, entry, "container")
						assert.Contains(t, entry, "image")
						assert.Contains(t, entry, "container_id")
					}
				}

				assert.Equal(t, tc.expectedLogMessages, skippedLogCount, "Expected %d log messages for skipped containers", tc.expectedLogMessages)
			})
		}
	})

	t.Run("context is canceled mid-iteration", func(t *testing.T) {
		t.Parallel()

		t.Run("should add remaining containers to failed map with wrapped error", func(t *testing.T) {
			t.Parallel()

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
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()

			// Call stopContainersInReversedOrder.
			failed, stopped := stopContainersInReversedOrder(newTestLogger(t),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
			)

			// All 4 containers should be in failed map (containers 0, 1, 2, 3).
			require.Len(t, failed, 4)
			assert.Empty(t, stopped)

			// Verify all containers 0, 1, 2, 3 are in failed map with wrapped error.
			for i := range 4 {
				containerID := types.ContainerID(fmt.Sprintf("container-%d", i))
				err, exists := failed[containerID]
				require.True(t, exists, "Container %d should be in failed map", i)

				// Verify error message contains "stop skipped".
				assert.Contains(t, err.Error(), "stop skipped")

				// Verify error wraps context.Canceled.
				require.ErrorIs(t, err, context.Canceled, "Error should wrap context.Canceled")
			}
		})

		t.Run("should log each skipped container with correct fields", func(t *testing.T) {
			t.Parallel()

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
			canceledCtx, cancel := context.WithCancel(t.Context())
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
					assert.Contains(t, entry, "container")
					assert.Contains(t, entry, "image")
					assert.Contains(t, entry, "container_id")
				}
			}

			// Verify all containers were logged.
			for _, name := range expectedNames {
				assert.Contains(t, loggedNames, name, "Container %s should have been logged", name)
			}

			// Verify we got the expected number of log messages.
			require.Len(t, loggedNames, 3)
		})
	})

	t.Run("context is not canceled", func(t *testing.T) {
		t.Parallel()

		t.Run("should process all containers without adding to failed map", func(t *testing.T) {
			t.Parallel()

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
				t.Context(),
				containers,
				client,
				types.UpdateParams{},
			)

			// All containers should be stopped, none failed.
			require.Len(t, stopped, 3)
			assert.Empty(t, failed)

			// Verify no "Skipped container stop" log messages.
			for _, entry := range parseJSONLogEntries(logBuf) {
				assert.NotEqual(t, "Skipped container stop due to context cancellation", entry["message"])
			}
		})
	})

	t.Run("containers are processed in reverse order", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop containers from last to first", func(t *testing.T) {
			t.Parallel()

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
			_, _ = stopContainersInReversedOrder(newTestLogger(t),
				t.Context(),
				containers,
				client,
				types.UpdateParams{},
			)

			// Verify stop order is reverse (container-2, container-1, container-0).
			require.Len(t, client.TestData.StopOrder, 3)
			assert.Equal(t, "container-2", client.TestData.StopOrder[0])
			assert.Equal(t, "container-1", client.TestData.StopOrder[1])
			assert.Equal(t, "container-0", client.TestData.StopOrder[2])
		})
	})
}

// TestRestartContainersInSortedOrderCancelRecovery verifies that containers
// already stopped are still recreated when the update is canceled.
func TestRestartContainersInSortedOrderCancelRecovery(t *testing.T) {
	t.Parallel()

	t.Run("still recreates containers that were already stopped when parent ctx is canceled", func(t *testing.T) {
		t.Parallel()

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

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		progress := session.Progress{}

		var cleanup []types.RemovedImageInfo

		failed := restartContainersInSortedOrder(newTestLogger(t),
			ctx,
			containers,
			client,
			types.UpdateParams{Timeout: 0},
			stoppedImages,
			&cleanup,
			&progress,
			nil,
		)

		assert.Empty(t, failed, "stopped containers must still be recreated after cancel")
		assert.Equal(t, int32(3), client.TestData.CreateContainerCount.Load())
		assert.Equal(t, int32(3), client.TestData.StartContainerCount.Load())
	})
}

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
func Test_performRollingRestart(t *testing.T) {
	t.Parallel()

	t.Run("context is canceled during iteration", func(t *testing.T) {
		t.Parallel()

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
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				t.Log(tc.description)

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
				ctx := t.Context()
				if tc.cancelAtIndex >= 0 {
					// Create an already-canceled context to simulate cancellation.
					canceledCtx, cancel := context.WithCancel(t.Context())
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
				require.Len(t, failed, tc.expectedSkipped, "Expected %d failed containers", tc.expectedSkipped)

				// Verify error is returned when context is canceled.
				if tc.cancelAtIndex >= 0 {
					require.ErrorIs(t, err, context.Canceled, "Error should wrap context.Canceled")
					assert.Contains(t, err.Error(), "rolling restart canceled", "Error message should contain 'rolling restart canceled'")
				} else {
					require.NoError(t, err, "Expected no error when context is not canceled")
				}

				// Verify log messages for skipped containers.
				skippedLogCount := 0

				for _, entry := range parseJSONLogEntries(logBuf) {
					if entry["message"] == "Skipped container restart due to context cancellation" {
						skippedLogCount++

						// Verify log fields contain expected keys.
						assert.Contains(t, entry, "container")
						assert.Contains(t, entry, "image")
						assert.Contains(t, entry, "container_id")
					}
				}

				assert.Equal(t, tc.expectedLogMessages, skippedLogCount, "Expected %d log messages for skipped containers", tc.expectedLogMessages)
			})
		}
	})

	t.Run("context is canceled mid-iteration", func(t *testing.T) {
		t.Parallel()

		t.Run("should add remaining containers to failed map with wrapped error", func(t *testing.T) {
			t.Parallel()

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
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newTestLogger(t),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// All 4 containers should be in failed map (containers 0, 1, 2, 3).
			require.Len(t, failed, 4)

			// Verify error is returned.
			require.ErrorIs(t, err, context.Canceled, "Error should wrap context.Canceled")
			assert.Contains(t, err.Error(), "rolling restart canceled")

			// Verify all containers 0, 1, 2, 3 are in failed map with wrapped error.
			for i := range 4 {
				containerID := types.ContainerID(fmt.Sprintf("container-%d", i))
				containerErr, exists := failed[containerID]
				require.True(t, exists, "Container %d should be in failed map", i)

				// Verify error message contains "restart skipped".
				assert.Contains(t, containerErr.Error(), "restart skipped")

				// Verify error wraps context.Canceled.
				require.ErrorIs(t, containerErr, context.Canceled, "Error should wrap context.Canceled")
			}
		})

		t.Run("should log each skipped container with correct fields", func(t *testing.T) {
			t.Parallel()

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
			canceledCtx, cancel := context.WithCancel(t.Context())
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
					assert.Contains(t, entry, "container")
					assert.Contains(t, entry, "image")
					assert.Contains(t, entry, "container_id")
				}
			}

			// Verify all containers were logged.
			for _, name := range expectedNames {
				assert.Contains(t, loggedNames, name, "Container %s should have been logged", name)
			}

			// Verify we got the expected number of log messages.
			require.Len(t, loggedNames, 3)
		})
	})

	t.Run("context is not canceled", func(t *testing.T) {
		t.Parallel()

		t.Run("should process all containers without adding to failed map", func(t *testing.T) {
			t.Parallel()

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
				t.Context(),
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// All containers should be processed, none failed due to cancellation.
			assert.Empty(t, failed)
			require.NoError(t, err)

			// Verify no "Skipped container restart" log messages.
			for _, entry := range parseJSONLogEntries(logBuf) {
				assert.NotEqual(t, "Skipped container restart due to context cancellation", entry["message"])
			}
		})
	})

	t.Run("containers are processed in forward order", func(t *testing.T) {
		t.Parallel()

		t.Run("should process containers from first to last", func(t *testing.T) {
			t.Parallel()

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

			_, _ = performRollingRestart(newTestLogger(t),
				t.Context(),
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify create order is forward (container-0, container-1, container-2).
			require.Len(t, client.TestData.CreateOrder, 3)
			assert.Equal(t, "container-0", client.TestData.CreateOrder[0])
			assert.Equal(t, "container-1", client.TestData.CreateOrder[1])
			assert.Equal(t, "container-2", client.TestData.CreateOrder[2])
		})
	})

	t.Run("verifying error wrapping", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error that wraps context.Canceled", func(t *testing.T) {
			t.Parallel()

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
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newTestLogger(t),
				canceledCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify error is returned.
			require.Error(t, err)

			// Verify error wraps context.Canceled using errors.Is.
			require.ErrorIs(t, err, context.Canceled, "Error should wrap context.Canceled")

			// Verify error message format.
			assert.Contains(t, err.Error(), "rolling restart canceled")

			// Verify failed container error also wraps context.Canceled.
			require.Len(t, failed, 1)
			containerErr := failed[types.ContainerID("container-0")]
			require.ErrorIs(t, containerErr, context.Canceled, "Container error should wrap context.Canceled")
			assert.Contains(t, containerErr.Error(), "restart skipped")
		})

		t.Run("should return error that wraps context.DeadlineExceeded when deadline exceeded", func(t *testing.T) {
			t.Parallel()

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
			deadlineCtx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-1*time.Hour))
			defer cancel()

			// Call performRollingRestart.
			var cleanupImageInfos []types.RemovedImageInfo

			failed, err := performRollingRestart(newTestLogger(t),
				deadlineCtx,
				containers,
				client,
				types.UpdateParams{},
				&cleanupImageInfos,
				nil,
				nil,
			)

			// Verify error is returned.
			require.Error(t, err)

			// Verify error wraps context.DeadlineExceeded using errors.Is.
			require.ErrorIs(t, err, context.DeadlineExceeded, "Error should wrap context.DeadlineExceeded")

			// Verify error message format.
			assert.Contains(t, err.Error(), "rolling restart canceled")

			// Verify failed container error also wraps context.DeadlineExceeded.
			require.Len(t, failed, 1)
			containerErr := failed[types.ContainerID("container-0")]
			require.ErrorIs(t, containerErr, context.DeadlineExceeded, "Container error should wrap context.DeadlineExceeded")
			assert.Contains(t, containerErr.Error(), "restart skipped")
		})
	})
}

// Test_deduplicateByImageID verifies that cleanup entries keep only the first
// entry for each image ID, in order.
func Test_deduplicateByImageID(t *testing.T) {
	t.Parallel()

	t.Run("should return empty slice for empty input", func(t *testing.T) {
		t.Parallel()

		result := deduplicateByImageID([]types.RemovedImageInfo{})
		assert.Empty(t, result)
	})

	t.Run("should return all entries when all ImageIDs are unique", func(t *testing.T) {
		t.Parallel()

		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:bbb"), ContainerName: "container-b"},
		}

		result := deduplicateByImageID(images)
		require.Len(t, result, 2)
		assert.Equal(t, types.ImageID("sha256:aaa"), result[0].ImageID)
		assert.Equal(t, types.ImageID("sha256:bbb"), result[1].ImageID)
	})

	t.Run("should keep only first occurrence when ImageIDs are duplicated", func(t *testing.T) {
		t.Parallel()

		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-b"},
		}

		result := deduplicateByImageID(images)
		require.Len(t, result, 1)
		// Should keep the first occurrence with container-a
		assert.Equal(t, "container-a", result[0].ContainerName)
	})

	t.Run("should handle mixed unique and duplicate ImageIDs", func(t *testing.T) {
		t.Parallel()

		images := []types.RemovedImageInfo{
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-a"},
			{ImageID: types.ImageID("sha256:bbb"), ContainerName: "container-b"},
			{ImageID: types.ImageID("sha256:aaa"), ContainerName: "container-c"},
			{ImageID: types.ImageID("sha256:ccc"), ContainerName: "container-d"},
		}

		result := deduplicateByImageID(images)
		require.Len(t, result, 3)
		assert.Equal(t, types.ImageID("sha256:aaa"), result[0].ImageID)
		assert.Equal(t, "container-a", result[0].ContainerName)
		assert.Equal(t, types.ImageID("sha256:bbb"), result[1].ImageID)
		assert.Equal(t, types.ImageID("sha256:ccc"), result[2].ImageID)
	})
}

// Test_isPinned verifies which image references count as pinned by digest.
func Test_isPinned(t *testing.T) {
	t.Parallel()

	// Valid 64-hex digest used by Docker/OCI references.
	const fullDigest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	t.Run("digest pin detection via shared helper", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			image      string
			wantPinned bool
		}{
			{name: "tagged image", image: "nginx:latest", wantPinned: false},
			{name: "repo@digest", image: "nginx@" + fullDigest, wantPinned: true},
			{name: "tag and digest", image: "nginx:1.27@" + fullDigest, wantPinned: true},
			{name: "registry/repo@digest", image: "registry.example.com/org/app@" + fullDigest, wantPinned: true},
			{name: "bare content digest", image: fullDigest, wantPinned: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				image, wantPinned := tt.image, tt.wantPinned

				cont := mockActions.CreateMockContainer(
					"id1",
					"/name",
					image,
					time.Now(),
				)
				progress := session.Progress{}
				params := types.UpdateParams{}

				pinned, err := isPinned(newTestLogger(t), cont, &progress, params, nil)
				require.NoError(t, err)
				assert.Equal(t, wantPinned, pinned)

				if wantPinned {
					require.Len(t, progress, 1, "pinned containers are marked scanned")
				} else {
					assert.Empty(t, progress, "unpinned containers are not scanned here")
				}
			})
		}
	})

	t.Run("detects pin before parse fallback can replace the image name", func(t *testing.T) {
		t.Parallel()

		// ImageName is a valid digest pin. Even if parse were flaky, pin must win.
		cont := mockActions.CreateMockContainer(
			"pinned-id",
			"/pinned-name",
			"library/nginx@"+fullDigest,
			time.Now(),
		)
		progress := session.Progress{}

		pinned, err := isPinned(newTestLogger(t), cont, &progress, types.UpdateParams{}, nil)
		require.NoError(t, err)
		assert.True(t, pinned)
		require.Len(t, progress, 1)
	})

	t.Run("returns error for unresolvable invalid image names", func(t *testing.T) {
		t.Parallel()

		cont := mockActions.CreateMockContainer(
			"InvalidContainer",
			"/InvalidContainer",
			":latest",
			time.Now(),
		)
		progress := session.Progress{}

		pinned, err := isPinned(newTestLogger(t), cont, &progress, types.UpdateParams{}, nil)
		require.Error(t, err)
		assert.False(t, pinned)
		assert.Empty(t, progress)
	})

	t.Run("does not panic when ContainerInfo Config is nil", func(t *testing.T) {
		t.Parallel()

		cont := mockTypes.NewMockContainer(t)
		cont.On("Name").Return("/nil-config")
		cont.On("ImageName").Return("nginx:latest")
		cont.On("ContainerInfo").Return(&dockerContainer.InspectResponse{
			Config: nil,
		})

		progress := session.Progress{}

		pinned, err := isPinned(newTestLogger(t), cont, &progress, types.UpdateParams{}, nil)
		require.NoError(t, err)
		assert.False(t, pinned)
	})
}
