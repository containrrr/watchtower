package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// createDefaultMockContainer wraps mockActions.CreateMockContainerWithConfig with
// common defaults for ephemeral self-update tests. It sets the container to running,
// not restarting, on watchtower:latest, with a current timestamp and the provided labels.
//
// Parameters:
//   - id: Container ID for the mock.
//   - labels: Label map to apply to the container config.
//
// Returns:
//   - types.Container: Configured mock container.
func createDefaultMockContainer(id string, labels map[string]string) types.Container {
	return mockActions.CreateMockContainerWithConfig(
		id,
		"watchtower",
		"watchtower:latest",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{Image: "watchtower:latest", Labels: labels},
	)
}

// expectOrchestrator expects one orchestrator to be created for source with
// the given container chain and cleanup setting, and answers it with err.
//
// The mock has no other expectations, so a stop, start, or rename of the
// source fails the test.
//
// Parameters:
//   - t: Test the mock belongs to.
//   - source: Watchtower container being replaced.
//   - chain: Container chain the orchestrator is expected to receive.
//   - cleanup: Cleanup setting the orchestrator is expected to receive.
//   - err: Error the creation returns.
//
// Returns:
//   - *mockContainer.MockClient: Mocked container client.
func expectOrchestrator(t *testing.T, source types.Container, chain string, cleanup bool, err error) *mockContainer.MockClient {
	t.Helper()

	client := mockContainer.NewMockClient(t)
	client.EXPECT().
		CreateEphemeralOrchestrator(mock.Anything, source, "watchtower:latest", chain, cleanup).
		Return(types.ContainerID("orchestrator-id"), err).Once()

	return client
}

// Test_ephemeralSelfUpdate verifies that an ephemeral self-update only creates
// the orchestrator, passing it the new image, the container chain extended
// with the source container, and the cleanup setting, and that it reports the
// source as handed off.
func Test_ephemeralSelfUpdate(t *testing.T) {
	t.Parallel()

	watchtowerLabels := map[string]string{"com.centurylinklabs.watchtower": "true"}

	t.Run("the orchestrator is created successfully", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty container ID and true so the dying process skips cleanup", func(t *testing.T) {
			t.Parallel()

			source := createDefaultMockContainer("source-123", watchtowerLabels)
			client := expectOrchestrator(t, source, "source-123", false, nil)

			newID, renamed, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{})
			require.NoError(t, err)
			assert.Empty(t, newID)
			assert.True(t, renamed)
		})
	})

	t.Run("the source container has no existing chain", func(t *testing.T) {
		t.Parallel()

		t.Run("should use the source container ID as the new chain", func(t *testing.T) {
			t.Parallel()

			source := createDefaultMockContainer("source-456", watchtowerLabels)
			client := expectOrchestrator(t, source, "source-456", false, nil)

			newID, renamed, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{})
			require.NoError(t, err)
			assert.Empty(t, newID)
			assert.True(t, renamed)
		})
	})

	t.Run("the source container has an existing chain", func(t *testing.T) {
		t.Parallel()

		t.Run("should append the source ID to the existing chain", func(t *testing.T) {
			t.Parallel()

			const existingChain = "old-id-1,old-id-2"

			source := createDefaultMockContainer("source-789", map[string]string{
				"com.centurylinklabs.watchtower":                 "true",
				"com.centurylinklabs.watchtower.container-chain": existingChain,
			})

			chain, present := source.GetContainerChain()
			require.True(t, present)
			require.Equal(t, existingChain, chain)

			client := expectOrchestrator(t, source, "old-id-1,old-id-2,source-789", false, nil)

			newID, renamed, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{})
			require.NoError(t, err)
			assert.Empty(t, newID)
			assert.True(t, renamed)
		})
	})

	t.Run("CreateEphemeralOrchestrator fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error containing the orchestrator failure message", func(t *testing.T) {
			t.Parallel()

			source := createDefaultMockContainer("source-err", watchtowerLabels)
			client := expectOrchestrator(t, source, "source-err", false, errors.New("create failed"))

			_, _, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{})
			require.ErrorContains(t, err, "ephemeral orchestrator failed")
		})
	})

	t.Run("cleanup is enabled on the update params", func(t *testing.T) {
		t.Parallel()

		t.Run("should pass cleanup=true to the orchestrator", func(t *testing.T) {
			t.Parallel()

			source := createDefaultMockContainer("source-cleanup", watchtowerLabels)
			client := expectOrchestrator(t, source, "source-cleanup", true, nil)

			_, renamed, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{Cleanup: true})
			require.NoError(t, err)
			assert.True(t, renamed)
		})
	})

	// The self-update only starts the orchestrator. Replacement is asynchronous.
	t.Run("the orchestrator launches successfully", func(t *testing.T) {
		t.Parallel()

		t.Run("returns without performing stop or start on the source client path", func(t *testing.T) {
			t.Parallel()

			// expectOrchestrator allows no other calls, so a stop, start, or
			// rename fails the test.
			source := createDefaultMockContainer("source-async", watchtowerLabels)
			client := expectOrchestrator(t, source, "source-async", false, nil)

			newID, renamed, err := ephemeralSelfUpdate(newTestLogger(t), t.Context(), client, source, types.UpdateParams{})
			require.NoError(t, err)
			assert.Empty(t, newID)
			assert.True(t, renamed)
		})
	})
}

// TestEphemeralSelfUpdate_StoppingContainerLog verifies that the "Stopping
// container" entry, which notifications include, is logged only once the
// orchestrator that stops the container has started.
func TestEphemeralSelfUpdate_StoppingContainerLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// createErr fails the orchestrator creation when set.
		createErr error
		// wantLevels are the levels of the logged "Stopping container" records.
		wantLevels []string
	}{
		{name: "orchestrator started", wantLevels: []string{"info"}},
		{name: "orchestrator failed", createErr: errors.New("create failed"), wantLevels: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := createDefaultMockContainer("source-ephemeral", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			client := mockContainer.NewMockClient(t)
			client.EXPECT().
				CreateEphemeralOrchestrator(mock.Anything, source, mock.Anything, mock.Anything, mock.Anything).
				Return(types.ContainerID("orchestrator"), tt.createErr).Once()

			var out bytes.Buffer

			log := zerolog.New(&out).Level(zerolog.DebugLevel)

			_, _, err := ephemeralSelfUpdate(&log, t.Context(), client, source, types.UpdateParams{})
			if tt.createErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			var levels []string

			for line := range strings.Lines(out.String()) {
				var record map[string]any

				require.NoError(t, json.Unmarshal([]byte(line), &record))

				if record["message"] == "Stopping container" {
					assert.Equal(t, "watchtower", record["container"])
					assert.Equal(t, source.ID().ShortID(), record["id"])
					assert.Equal(t, source.ImageID().ShortID(), record["old_image_id"])

					level, ok := record["level"].(string)
					require.True(t, ok)

					levels = append(levels, level)
				}
			}

			assert.Equal(t, tt.wantLevels, levels)
		})
	}
}

// TestEphemeralSelfUpdate_OutlivesCanceledUpdate verifies that the orchestrator
// is created with a context the update's cancellation does not reach, so a
// canceled update still hands Watchtower off, and that the context keeps the
// update context's values.
func TestEphemeralSelfUpdate_OutlivesCanceledUpdate(t *testing.T) {
	t.Parallel()

	// parentKey is a context key only this test uses.
	type parentKey struct{}

	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), parentKey{}, "parent-value"))
	cancel()

	source := createDefaultMockContainer("source-canceled", map[string]string{
		"com.centurylinklabs.watchtower": "true",
	})

	client := mockContainer.NewMockClient(t)
	client.EXPECT().
		CreateEphemeralOrchestrator(
			mock.MatchedBy(func(created context.Context) bool {
				_, hasDeadline := created.Deadline()

				return created.Err() == nil && hasDeadline && created.Value(parentKey{}) == "parent-value"
			}),
			source, "watchtower:latest", "source-canceled", false,
		).
		Return(types.ContainerID("orchestrator-id"), nil).Once()

	_, renamed, err := ephemeralSelfUpdate(newTestLogger(t), ctx, client, source, types.UpdateParams{})
	require.NoError(t, err)
	assert.True(t, renamed)
}
