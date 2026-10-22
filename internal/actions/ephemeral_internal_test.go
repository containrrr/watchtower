package actions

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// handoffNewID is the ID of the replacement container the mocked client starts.
const handoffNewID types.ContainerID = "wt-new-id"

// createEphemeralHandoffContainer builds a running Watchtower mock used by
// orchestrateSelfUpdate handoff tests.
//
// Parameters:
//   - id: Container ID.
//   - portBindings: Optional host port bindings. Nil yields empty bindings.
//
// Returns:
//   - types.Container: Configured mock container.
func createEphemeralHandoffContainer(
	id string,
	portBindings dockerNetwork.PortMap,
) types.Container {
	if portBindings == nil {
		portBindings = dockerNetwork.PortMap{}
	}

	const (
		name  = "/watchtower"
		image = "watchtower:v1"
	)

	ctr := mockActions.CreateMockContainerWithConfig(
		id,
		name,
		image,
		true,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image: image,
			Labels: map[string]string{
				"com.centurylinklabs.watchtower": "true",
			},
		},
	)

	info := ctr.ContainerInfo()
	if info != nil && info.HostConfig != nil {
		info.HostConfig.PortBindings = portBindings
	}

	return ctr
}

// handoffCalls are the expected client calls of a self-update handoff, so a
// test can order them.
type handoffCalls struct {
	stop   *mock.Call
	rename *mock.Call
	start  *mock.Call
}

// expectHandoffStart expects the old container to be inspected, stopped,
// renamed out of the way, and then used to create and start the replacement,
// which startErr fails when set.
//
// Parameters:
//   - client: Mocked container client.
//   - old: The Watchtower container being replaced.
//   - startErr: Error the replacement start returns.
//
// Returns:
//   - handoffCalls: The stop, rename, and start expectations.
func expectHandoffStart(client *mockContainer.MockClient, old types.Container, startErr error) handoffCalls {
	client.EXPECT().GetContainer(mock.Anything, old.ID()).Return(old, nil).Once()

	newID := handoffNewID
	if startErr != nil {
		newID = ""
	}

	return handoffCalls{
		stop: client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil).Once(),
		rename: client.EXPECT().RenameContainer(mock.Anything, old, types.WatchtowerOldPrefix+old.ID().ShortID()).
			Return(nil).Once(),
		start: client.EXPECT().StartContainer(mock.Anything, old).Return(newID, startErr).Once(),
	}
}

// expectHandoffFinish expects the replacement to be verified running and the
// old container to be removed.
//
// Parameters:
//   - client: Mocked container client.
//   - old: The Watchtower container being replaced.
//
// Returns:
//   - *mock.Call: The removal expectation.
func expectHandoffFinish(client *mockContainer.MockClient, old types.Container) *mock.Call {
	client.EXPECT().GetContainer(mock.Anything, handoffNewID).
		Return(createEphemeralHandoffContainer(string(handoffNewID), nil), nil).Once()

	return client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil).Once()
}

// expectHandoff expects a complete, successful handoff.
//
// Parameters:
//   - client: Mocked container client.
//   - old: The Watchtower container being replaced.
func expectHandoff(client *mockContainer.MockClient, old types.Container) {
	expectHandoffStart(client, old, nil)
	expectHandoffFinish(client, old)
}

// Test_orchestrateSelfUpdate verifies the order of the self-update handoff,
// that a failed stop or create leaves the old container in charge, that the
// replacement is created from the new image with the container chain, and
// that old image cleanup tolerates an image in use or already gone.
func Test_orchestrateSelfUpdate(t *testing.T) {
	t.Parallel()

	// Stop frees host ports. Rename keeps the stopped predecessor for recovery.
	// Remove runs only after the replacement is verified running.
	t.Run("stops, renames, creates, then removes the old container on success", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer("wt-old-id-ok", nil)
		client := mockContainer.NewMockClient(t)
		calls := expectHandoffStart(client, old, nil)
		remove := expectHandoffFinish(client, old)
		mock.InOrder(calls.stop, calls.rename, calls.start, remove)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-id-ok", "watchtower:v2", "watchtower", "", false)
		require.NoError(t, err)
	})

	t.Run("stops the old container before create when host ports are published", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer(
			"wt-old-ports",
			dockerNetwork.PortMap{
				dockerNetwork.MustParsePort("8080/tcp"): {
					{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "8080"},
				},
			},
		)
		require.True(t, old.HasExposedPorts())

		client := mockContainer.NewMockClient(t)
		calls := expectHandoffStart(client, old, nil)
		expectHandoffFinish(client, old)
		mock.InOrder(calls.stop, calls.start)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-ports", "watchtower:v2", "watchtower", "", false)
		require.NoError(t, err)
	})

	t.Run("does not create a new container when stopping the old container fails", func(t *testing.T) {
		t.Parallel()

		// The old container is still running after the failed stop, so the
		// handoff ends before any rename or create.
		old := createEphemeralHandoffContainer("wt-old-stop-fail", nil)
		client := mockContainer.NewMockClient(t)
		client.EXPECT().GetContainer(mock.Anything, old.ID()).Return(old, nil).Twice()
		client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).
			Return(errors.New("simulated stop failure")).Once()

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-stop-fail", "watchtower:v2", "watchtower", "", false)
		require.ErrorContains(t, err, "failed to stop old container")
	})

	t.Run("restores and restarts the old container when create fails after rename", func(t *testing.T) {
		t.Parallel()

		// The predecessor gets its name back and is restarted, and is never
		// removed.
		old := createEphemeralHandoffContainer("wt-old-create-fail", nil)
		client := mockContainer.NewMockClient(t)
		calls := expectHandoffStart(client, old, errors.New("simulated create failure"))
		renameBack := client.EXPECT().RenameContainer(mock.Anything, old, "watchtower").Return(nil).Once()
		restart := client.EXPECT().StartContainerByID(mock.Anything, old.ID()).Return(nil).Once()
		mock.InOrder(calls.stop, calls.rename, calls.start, renameBack, restart)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-create-fail", "watchtower:v2", "watchtower", "", false)
		require.ErrorContains(t, err, "failed to create new container")
	})

	t.Run("pins Config.Image to the new image reference before create", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer("wt-old-pin", nil)
		client := mockContainer.NewMockClient(t)
		calls := expectHandoffStart(client, old, nil)
		calls.start.Run(func(args mock.Arguments) {
			created, ok := args.Get(1).(types.Container)
			require.True(t, ok)
			assert.Equal(t, "watchtower:v2", created.ImageName())
		})
		expectHandoffFinish(client, old)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-pin", "watchtower:v2", "watchtower", "", false)
		require.NoError(t, err)
	})

	t.Run("propagates the container chain label onto the source config used for create", func(t *testing.T) {
		t.Parallel()

		const chain = "prev-id,wt-old-chain"

		old := createEphemeralHandoffContainer("wt-old-chain", nil)
		client := mockContainer.NewMockClient(t)
		calls := expectHandoffStart(client, old, nil)
		calls.start.Run(func(args mock.Arguments) {
			created, ok := args.Get(1).(types.Container)
			require.True(t, ok)

			gotChain, present := created.GetContainerChain()
			assert.True(t, present)
			assert.Equal(t, chain, gotChain)
		})
		expectHandoffFinish(client, old)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-chain", "watchtower:v2", "watchtower", chain, false)
		require.NoError(t, err)
	})

	t.Run("removes the old image after a successful handoff when cleanup is enabled", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer("wt-old-cleanup", nil)
		client := mockContainer.NewMockClient(t)
		expectHandoff(client, old)
		client.EXPECT().RemoveImageByID(mock.Anything, old.ImageID(), "watchtower:v1").Return(nil).Once()

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-cleanup", "watchtower:v2", "watchtower", "", true)
		require.NoError(t, err)
	})

	t.Run("does not remove the old image when cleanup is disabled", func(t *testing.T) {
		t.Parallel()

		// The mock has no image removal expectation, so a removal fails the test.
		old := createEphemeralHandoffContainer("wt-old-no-cleanup", nil)
		client := mockContainer.NewMockClient(t)
		expectHandoff(client, old)

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-no-cleanup", "watchtower:v2", "watchtower", "", false)
		require.NoError(t, err)
	})

	t.Run("treats an in-use old image as a non-fatal skip", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer("wt-old-in-use", nil)
		client := mockContainer.NewMockClient(t)
		expectHandoff(client, old)
		client.EXPECT().RemoveImageByID(mock.Anything, old.ImageID(), "watchtower:v1").
			Return(container.ErrImageInUse).Once()

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-in-use", "watchtower:v2", "watchtower", "", true)
		require.NoError(t, err)
	})

	t.Run("treats a missing old image as a non-fatal skip", func(t *testing.T) {
		t.Parallel()

		old := createEphemeralHandoffContainer("wt-old-missing-image", nil)
		client := mockContainer.NewMockClient(t)
		expectHandoff(client, old)
		client.EXPECT().RemoveImageByID(mock.Anything, old.ImageID(), "watchtower:v1").
			Return(cerrdefs.ErrNotFound).Once()

		err := orchestrateSelfUpdate(newTestLogger(t), t.Context(), client,
			"wt-old-missing-image", "watchtower:v2", "watchtower", "", true)
		require.NoError(t, err)
	})
}
