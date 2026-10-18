package actions

import (
	"context"
	"errors"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

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

func staleMockContainer() types.Container {
	source := mockActions.CreateMockContainer("container-id", "/app", "app:latest", time.Now())
	source.(*container.Container).SetStale(true)

	return source
}

func newCopyFileAwareClient() (
	*copyFileAwareClient,
	*mockContainer.MockClient,
	*mockContainer.MockCopyFileStore,
) {
	mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())
	mockStore := mockContainer.NewMockCopyFileStore(ginkgo.GinkgoT())

	return &copyFileAwareClient{
		Client:        mockClient,
		CopyFileStore: mockStore,
	}, mockClient, mockStore
}

var _ = ginkgo.Describe("copy-file recreate helpers", func() {
	ginkgo.It("skips snapshot when the client is not a CopyFileStore", func() {
		client := mockContainer.NewMockClient(ginkgo.GinkgoT())
		source := staleMockContainer()

		gomega.Expect(snapshotCopyFilesForRecreate(context.Background(), client, source)).To(gomega.Succeed())
		discardCopyFilesForRecreate(client, source.ID())
	})

	ginkgo.It("wraps SnapshotCopyFiles errors", func() {
		client, _, mockStore := newCopyFileAwareClient()
		source := staleMockContainer()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(errors.New("denied"))

		err := snapshotCopyFilesForRecreate(context.Background(), client, source)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("snapshot copy-file paths"))
	})

	ginkgo.It("discards leftovers on a CopyFileStore client", func() {
		client, _, mockStore := newCopyFileAwareClient()
		source := staleMockContainer()
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		discardCopyFilesForRecreate(client, source.ID())
	})
})

var _ = ginkgo.Describe("stopStaleContainer copy-file", func() {
	ginkgo.It("snapshots before stop and does not discard on success", func() {
		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(nil)

		stopped, err := stopStaleContainer(
			newNopLogger(),
			context.Background(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(stopped).To(gomega.BeTrue())
	})

	ginkgo.It("does not stop when snapshot fails", func() {
		source := staleMockContainer()
		client, _, mockStore := newCopyFileAwareClient()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(errors.New("denied"))

		_, err := stopStaleContainer(
			newNopLogger(),
			context.Background(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		gomega.Expect(err).To(gomega.HaveOccurred())
	})

	ginkgo.It("discards the snapshot when stop fails", func() {
		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(errors.New("busy"))
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		_, err := stopStaleContainer(
			newNopLogger(),
			context.Background(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		gomega.Expect(err).To(gomega.HaveOccurred())
	})

	ginkgo.It("skips and discards the snapshot when the container is already gone", func() {
		source := staleMockContainer()
		client, mockClient, mockStore := newCopyFileAwareClient()
		mockStore.EXPECT().
			SnapshotCopyFiles(mock.Anything, source).
			Return(nil)
		mockClient.EXPECT().
			StopAndRemoveContainer(mock.Anything, source, time.Second).
			Return(cerrdefs.ErrNotFound)
		mockStore.EXPECT().DiscardCopyFiles(source.ID())

		stopped, err := stopStaleContainer(
			newNopLogger(),
			context.Background(),
			source,
			client,
			types.UpdateParams{Timeout: time.Second},
		)
		gomega.Expect(stopped).To(gomega.BeFalse())
		gomega.Expect(isSkip(err)).To(gomega.BeTrue())
		gomega.Expect(errors.Is(err, errContainerGone)).To(gomega.BeTrue())
	})
})
