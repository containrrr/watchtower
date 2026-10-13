package actions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// createDefaultMockContainer wraps mockActions.CreateMockContainerWithConfig with
// common defaults for ephemeral self-update tests. It sets the container to running,
// not restarting, with a current timestamp and the provided labels.
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
		&dockerContainer.Config{Labels: labels},
	)
}

// createDefaultMockClient wraps mockActions.CreateMockClient with common defaults
// for ephemeral self-update tests. It uses an empty TestData, no image pulling,
// and no volume removal.
//
// Parameters:
//   - td: Test data structure for capturing orchestrator parameters.
//
// Returns:
//   - mockActions.MockClient: Configured mock client.
func createDefaultMockClient(td *mockActions.TestData) mockActions.MockClient {
	return mockActions.CreateMockClient(td, false, false)
}

var _ = ginkgo.Describe("EphemeralSelfUpdate", func() {
	ginkgo.When("the orchestrator is created successfully", func() {
		ginkgo.It("should return empty container ID and true so the dying process skips cleanup", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sourceContainer := createDefaultMockContainer("source-123", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			client := createDefaultMockClient(&mockActions.TestData{})

			newID, renamed, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(renamed).To(gomega.BeTrue())
		})
	})

	ginkgo.When("the source container has no existing chain", func() {
		ginkgo.It("should use the source container ID as the new chain", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sourceContainer := createDefaultMockContainer("source-456", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			client := createDefaultMockClient(&mockActions.TestData{})

			newID, renamed, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(renamed).To(gomega.BeTrue())

			// Verify the orchestrator's chain was set to the source container ID.
			gomega.Expect(client.TestData.LastContainerChain).To(
				gomega.Equal("source-456"),
				"orchestrator chain should be the source container ID when no existing chain is present",
			)
		})
	})

	ginkgo.When("the source container has an existing chain", func() {
		ginkgo.It("should append the source ID to the existing chain", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			existingChain := "old-id-1,old-id-2"

			sourceContainer := createDefaultMockContainer("source-789", map[string]string{
				"com.centurylinklabs.watchtower":                 "true",
				"com.centurylinklabs.watchtower.container-chain": existingChain,
			})

			// Verify the mock container has the expected chain label.
			chain, present := sourceContainer.GetContainerChain()
			gomega.Expect(present).To(gomega.BeTrue())
			gomega.Expect(chain).To(gomega.Equal(existingChain))

			client := createDefaultMockClient(&mockActions.TestData{})

			newID, renamed, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(renamed).To(gomega.BeTrue())

			// Verify the orchestrator's chain has the source ID appended to the existing chain.
			gomega.Expect(client.TestData.LastContainerChain).To(
				gomega.Equal("old-id-1,old-id-2,source-789"),
				"orchestrator chain should have source container ID appended to existing chain",
			)
		})
	})

	ginkgo.When("CreateEphemeralOrchestrator fails", func() {
		ginkgo.It("should return an error containing the orchestrator failure message", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sourceContainer := createDefaultMockContainer("source-err", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			// Use an already-cancelled client context to force CreateEphemeralOrchestrator to fail.
			cancelledCtx, cancelCancelled := context.WithCancel(context.Background())
			cancelCancelled()

			// Note: CreateMockClientWithContext is used here instead of createDefaultMockClient
			// because this test requires a pre-cancelled context to trigger the failure path.
			client := mockActions.CreateMockClientWithContext(
				cancelledCtx,
				&mockActions.TestData{},
				false,
				false,
			)

			_, _, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{},
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("ephemeral orchestrator failed"))
		})
	})

	ginkgo.When("cleanup is enabled on the update params", func() {
		ginkgo.It("should pass cleanup=true to the orchestrator", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sourceContainer := createDefaultMockContainer("source-cleanup", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			client := createDefaultMockClient(&mockActions.TestData{})

			_, renamed, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{Cleanup: true},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(renamed).To(gomega.BeTrue())
			gomega.Expect(client.TestData.LastCleanup).To(gomega.BeTrue())
		})
	})

	// EphemeralSelfUpdate only starts the orchestrator. Replacement is asynchronous.
	ginkgo.When("the orchestrator launches successfully", func() {
		ginkgo.It("returns without performing stop or start on the source client path", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			sourceContainer := createDefaultMockContainer("source-async", map[string]string{
				"com.centurylinklabs.watchtower": "true",
			})

			client := createDefaultMockClient(&mockActions.TestData{})

			newID, renamed, err := actions.EphemeralSelfUpdate(testLogger(),
				ctx,
				client,
				sourceContainer,
				types.UpdateParams{},
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(renamed).To(gomega.BeTrue())
			gomega.Expect(client.TestData.StopContainerCount.Load()).To(gomega.Equal(int32(0)))
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))
			gomega.Expect(client.TestData.RenameContainerCount.Load()).To(gomega.Equal(int32(0)))
		})
	})
})

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

			_, _, err := actions.EphemeralSelfUpdate(&log, t.Context(), client, source, types.UpdateParams{})
			if tt.createErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			var levels []string

			for line := range strings.Lines(out.String()) {
				var record map[string]any

				require.NoError(t, json.Unmarshal([]byte(line), &record))

				if record["message"] == actions.StoppingContainerMessage {
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
