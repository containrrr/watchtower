package actions

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// copyFileAwareClient implements container.Client and container.CopyFileStore
// by composing mockery-generated mocks.
type copyFileAwareClient struct {
	container.Client
	container.CopyFileStore
}

// staleMockContainer returns a stale container for the copy-file recreate tests.
//
// Returns:
//   - types.Container: The stale container.
func staleMockContainer() types.Container {
	source := mockActions.CreateMockContainer("container-id", "/app", "app:latest", time.Now())
	source.(*container.Container).SetStale(true)

	return source
}

// newCopyFileAwareClient returns a client that is also a copy-file store, with
// the mocks behind each side.
//
// Parameters:
//   - t: Test the mocks belong to.
//
// Returns:
//   - *copyFileAwareClient: The combined client.
//   - *mockContainer.MockClient: Mock behind the client methods.
//   - *mockContainer.MockCopyFileStore: Mock behind the copy-file store methods.
func newCopyFileAwareClient(t *testing.T) (
	*copyFileAwareClient,
	*mockContainer.MockClient,
	*mockContainer.MockCopyFileStore,
) {
	t.Helper()

	mockClient := mockContainer.NewMockClient(t)
	mockStore := mockContainer.NewMockCopyFileStore(t)

	return &copyFileAwareClient{
		Client:        mockClient,
		CopyFileStore: mockStore,
	}, mockClient, mockStore
}

// TestCopyFileRecreateHelpers verifies that labeled files are snapshotted and
// leftovers discarded only on a client that stores copy files, and that
// snapshot failures are wrapped.
func TestCopyFileRecreateHelpers(t *testing.T) {
	t.Parallel()

	t.Run("skips snapshot when the client is not a CopyFileStore", func(t *testing.T) {
		t.Parallel()

		client := mockContainer.NewMockClient(t)
		source := staleMockContainer()

		require.NoError(t, snapshotCopyFilesForRecreate(t.Context(), client, source))
		discardCopyFilesForRecreate(client, source.ID())
	})

	t.Run("wraps SnapshotCopyFiles errors", func(t *testing.T) {
		t.Parallel()

		client, _, mockStore := newCopyFileAwareClient(t)
		source := staleMockContainer()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(errors.New("denied"))

		err := snapshotCopyFilesForRecreate(t.Context(), client, source)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "snapshot copy-file paths")
	})

	t.Run("discards leftovers on a CopyFileStore client", func(t *testing.T) {
		t.Parallel()

		client, _, mockStore := newCopyFileAwareClient(t)
		source := staleMockContainer()
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		discardCopyFilesForRecreate(client, source.ID())
	})
}

// TestStopStaleContainerCopyFile verifies that a stale container's labeled
// files are snapshotted before it stops and kept on success, that a failed
// snapshot prevents the stop, and that the snapshot is discarded when the stop
// fails or the container is already gone.
func TestStopStaleContainerCopyFile(t *testing.T) {
	t.Parallel()

	t.Run("snapshots before stop and does not discard on success", func(t *testing.T) {
		t.Parallel()

		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient(t)
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(nil)

		stopped, err := stopStaleContainer(
			newTestLogger(t),
			t.Context(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		require.NoError(t, err)
		assert.True(t, stopped)
	})

	t.Run("does not stop when snapshot fails", func(t *testing.T) {
		t.Parallel()

		source := staleMockContainer()
		client, _, mockStore := newCopyFileAwareClient(t)
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(errors.New("denied"))

		_, err := stopStaleContainer(
			newTestLogger(t),
			t.Context(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		require.Error(t, err)
	})

	t.Run("discards the snapshot when stop fails", func(t *testing.T) {
		t.Parallel()

		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient(t)
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(errors.New("busy"))
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		_, err := stopStaleContainer(
			newTestLogger(t),
			t.Context(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		require.Error(t, err)
	})

	t.Run("skips and discards the snapshot when the container is already gone", func(t *testing.T) {
		t.Parallel()

		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient(t)
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(cerrdefs.ErrNotFound)
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		stopped, err := stopStaleContainer(
			newTestLogger(t),
			t.Context(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		assert.False(t, stopped)
		assert.True(t, isSkip(err))
		assert.ErrorIs(t, err, errContainerGone)
	})
}
