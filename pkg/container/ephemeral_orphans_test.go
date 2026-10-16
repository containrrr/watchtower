package container

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestRemoveOrphanedOrchestrators_SkipsInFlight verifies that startup cleanup
// removes only orchestrators that have run longer than any self-update can
// take. A younger orchestrator may still be handing off to a new Watchtower,
// whether this instance's or another scope's, so it is kept, as is one whose
// start time cannot be read.
func TestRemoveOrphanedOrchestrators_SkipsInFlight(t *testing.T) {
	t.Parallel()

	// startedAt formats a start time the way Docker reports it.
	startedAt := func(age time.Duration) string {
		return time.Now().Add(-age).UTC().Format(time.RFC3339Nano)
	}

	tests := []struct {
		name string
		// state is the orchestrator's Docker state.
		state dockerContainer.State
		// created is the orchestrator's creation time, used without a start time.
		created string
		// wantRemoved reports whether the orchestrator is removed.
		wantRemoved bool
	}{
		{
			name:  "just started",
			state: dockerContainer.State{Running: true, StartedAt: startedAt(time.Second)},
		},
		{
			name:  "within the self-update window",
			state: dockerContainer.State{Running: true, StartedAt: startedAt(4 * time.Minute)},
		},
		{
			name:        "past the self-update window",
			state:       dockerContainer.State{Running: true, StartedAt: startedAt(11 * time.Minute)},
			wantRemoved: true,
		},
		{
			name:  "start time in the future",
			state: dockerContainer.State{Running: true, StartedAt: startedAt(-time.Minute)},
		},
		{
			name:  "unreadable start time",
			state: dockerContainer.State{Running: true, StartedAt: "not-a-time"},
		},
		{
			name:        "no start time and old creation time",
			state:       dockerContainer.State{Running: true, StartedAt: "0001-01-01T00:00:00Z"},
			created:     startedAt(time.Hour),
			wantRemoved: true,
		},
		{
			name:    "no start time and recent creation time",
			state:   dockerContainer.State{Running: true},
			created: startedAt(time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			orchestrator := MockContainer(
				WithID("orch123"),
				WithName("watchtower-orchestrator"),
				WithLabels(map[string]string{orchestratorLabel: "true"}),
				WithContainerState(tt.state),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					c.Created = tt.created
				},
			)
			// A container without the orchestrator label is never removed.
			app := MockContainer(WithID("app123"), WithName("app"))

			client := mockContainer.NewMockClient(t)
			client.EXPECT().ListContainers(mock.Anything).
				Return([]types.Container{orchestrator, app}, nil).Once()

			wantCount := 0

			if tt.wantRemoved {
				wantCount = 1

				client.EXPECT().StopAndRemoveContainer(mock.Anything, orchestrator, time.Duration(0)).
					Return(nil).Once()
			}

			count, err := RemoveOrphanedOrchestrators(testLog(), t.Context(), client)
			require.NoError(t, err)
			assert.Equal(t, wantCount, count)
		})
	}
}

// TestRemoveOrphanedOrchestrators_AlreadyGone verifies that an orphaned
// orchestrator which no longer exists when cleanup reaches it, for example one
// Docker removed on exit, is not reported as a failed removal.
func TestRemoveOrphanedOrchestrators_AlreadyGone(t *testing.T) {
	t.Parallel()

	orchestrator := MockContainer(
		WithID("orch123"),
		WithName("watchtower-orchestrator"),
		WithLabels(map[string]string{orchestratorLabel: "true"}),
		WithContainerState(dockerContainer.State{
			Running:   true,
			StartedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
		}),
	)

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{orchestrator}, nil).Once()
	client.EXPECT().StopAndRemoveContainer(mock.Anything, orchestrator, time.Duration(0)).
		Return(fmt.Errorf("no such container: %w", cerrdefs.ErrNotFound)).Once()

	var out bytes.Buffer

	log := zerolog.New(&out).Level(zerolog.WarnLevel)

	count, err := RemoveOrphanedOrchestrators(&log, t.Context(), client)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, out.String(), "a missing orchestrator is not a warning")
}
