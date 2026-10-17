package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdate_PostChecksTargetCurrentContainers verifies that post-check hooks
// run in the containers that exist after the update. An updated container's
// hook runs in its replacement, since the container scanned before the update
// has been removed, and an unchanged container's hook runs in that container.
func TestUpdate_PostChecksTargetCurrentContainers(t *testing.T) {
	t.Parallel()

	const (
		postCheck = "/post-check.sh"
		oldID     = "01d0000000000000000000000000000000000000000000000000000000000000"
		newID     = "e000000000000000000000000000000000000000000000000000000000000000"
	)

	// appContainer returns a running container with a post-check hook.
	appContainer := func(id string) types.Container {
		return mockActions.CreateMockContainerWithConfig(
			id,
			"/app",
			"org/app:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Image: "org/app:latest",
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.lifecycle.post-check": postCheck,
				},
			},
		)
	}

	tests := []struct {
		name string
		// stale reports whether the scanned container has a newer image.
		stale bool
		// wantTarget is the ID of the container the post-check runs in.
		wantTarget types.ContainerID
	}{
		{name: "updated container", stale: true, wantTarget: newID},
		{name: "unchanged container", stale: false, wantTarget: oldID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scanned := appContainer(oldID)
			afterUpdate := scanned

			client := mockContainer.NewMockClient(t)
			client.EXPECT().IsContainerStale(mock.Anything, scanned, mock.Anything).
				Return(tt.stale, types.ImageID("sha256:new"), "", nil).Once()

			if tt.stale {
				afterUpdate = appContainer(newID)

				client.EXPECT().GetImageAnnotations(mock.Anything, mock.Anything).Return(oci.Annotations{})
				client.EXPECT().StopAndRemoveContainer(mock.Anything, scanned, mock.Anything).Return(nil).Once()
				client.EXPECT().CreateContainer(mock.Anything, scanned).Return(types.ContainerID(newID), nil).Once()
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(newID)).Return(nil).Once()
				client.EXPECT().GetContainer(mock.Anything, types.ContainerID(newID)).Return(afterUpdate, nil)
			}

			// The scan lists the container before the update, and the post-checks
			// list again to find the containers that exist afterwards.
			client.EXPECT().ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scanned}, nil).Once()
			client.EXPECT().ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{afterUpdate}, nil).Once()

			var targets []types.ContainerID

			client.EXPECT().
				ExecuteCommand(mock.Anything, mock.Anything, postCheck, mock.Anything, mock.Anything, mock.Anything).
				Run(func(_ context.Context, c types.Container, _ string, _, _, _ int) {
					targets = append(targets, c.ID())
				}).
				Return(false, nil)

			_, _, err := update(testLogger(), t.Context(), client, types.UpdateParams{LifecycleHooks: true}, ratelimit.New())
			require.NoError(t, err)

			assert.Equal(t, []types.ContainerID{tt.wantTarget}, targets)
		})
	}
}
