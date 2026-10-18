package actions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("CheckForMultipleWatchtowerInstances", func() {
	ginkgo.When("no scope is specified", func() {
		ginkgo.It("should return nil when only one instance exists", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockContainer := createMockContainer(
				"watchtower",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{mockContainer}, nil)

			var cleanupImageInfo []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				&cleanupImageInfo,
				mockContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageInfo).To(gomega.BeEmpty())
		})

		ginkgo.It(
			"should stop excess instances and collect image IDs when cleanup enabled",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				oldContainer := createMockContainer(
					"watchtower-old",
					"watchtower-old",
					"watchtower:old",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{oldContainer, newContainer}, nil)
				mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
				gomega.Expect(cleanupImageIDs).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageIDs[0].ImageID).
					To(gomega.Equal(types.ImageID("watchtower:old")))
			},
		)
	})

	ginkgo.When("scope is specified", func() {
		ginkgo.It("should only clean up instances in the same scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			scopedOldContainer := createMockContainer(
				"watchtower-scoped",
				"watchtower-scoped",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopedOldContainer}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"prod",
				&cleanupImageIDs,
				scopedOldContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})

		ginkgo.It("should clean up multiple instances within the same scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"watchtower-prod-old",
				"watchtower-prod-old",
				"watchtower:1.11.0",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)
			newContainer := createMockContainer(
				"watchtower-prod-new",
				"watchtower-prod-new",
				"watchtower:1.12.0",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"prod",
				&cleanupImageIDs,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageIDs).
				To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("watchtower:1.11.0"))))
			gomega.Expect(cleanupImageIDs).To(gomega.HaveLen(1))
		})

		ginkgo.It("should return false when only one instance exists in scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			scopedContainer := createMockContainer(
				"watchtower-prod",
				"watchtower-prod",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopedContainer}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"prod",
				&cleanupImageIDs,
				scopedContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("cleanup is disabled", func() {
		ginkgo.It("should stop excess instances but not collect image IDs", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"watchtower:old",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("error scenarios", func() {
		ginkgo.It("should not perform cleanup when currentContainer is nil", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{}, nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})

		ginkgo.It("should return error when stopping container fails", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"watchtower:old",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(errors.New("stop container failed")).
				Times(maxRemovalAttempts)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("1 of 1 instances failed to stop"))
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})

		ginkgo.It("should fail completely when some containers fail to stop after retries", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			old1Container := createMockContainer(
				"watchtower-old1",
				"watchtower-old1",
				"watchtower:old1",
				true,
				false,
				time.Now().Add(-2*time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			old2Container := createMockContainer(
				"watchtower-old2",
				"watchtower-old2",
				"watchtower:old2",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{old1Container, old2Container, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
				Return(errors.New("stop container failed")).
				Times(maxRemovalAttempts)
			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			gomega.Expect(err).
				To(gomega.HaveOccurred())
				// Partial success: one container was removed before failure
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})

		ginkgo.It(
			"should retry 'already in progress' errors and fail if they persist",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				old1Container := createMockContainer(
					"watchtower-old1",
					"watchtower-old1",
					"watchtower:old1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				old2Container := createMockContainer(
					"watchtower-old2",
					"watchtower-old2",
					"watchtower:old2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{old1Container, old2Container, newContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
					Return(errors.New("removal of container watchtower-old1 is already in progress")).
					Times(maxRemovalAttempts)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).
					Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
				gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
			},
		)

		ginkgo.It(
			"should treat 'no such container' errors as non-errors and continue cleanup",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				old1Container := createMockContainer(
					"watchtower-old1",
					"watchtower-old1",
					"watchtower:old1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				old2Container := createMockContainer(
					"watchtower-old2",
					"watchtower-old2",
					"watchtower:old2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:new",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{old1Container, old2Container, newContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old1Container, 10*time.Minute).
					Return(cerrdefs.ErrNotFound)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, old2Container, 10*time.Minute).
					Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).To(gomega.Equal(2))
				gomega.Expect(cleanupImageIDs).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("watchtower:old2"))))
				gomega.Expect(cleanupImageIDs).To(gomega.HaveLen(1))
			},
		)
	})

	ginkgo.When("image ID handling", func() {
		ginkgo.It(
			"should not collect image ID when excess container shares image with newest",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				oldContainer := createMockContainer(
					"watchtower-old",
					"watchtower-old",
					"watchtower:latest",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)
				newContainer := createMockContainer(
					"watchtower-new",
					"watchtower-new",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{oldContainer, newContainer}, nil)
				mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).Return(nil)

				var cleanupImageIDs []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"",
					&cleanupImageIDs,
					newContainer,
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
				gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
			},
		)

		ginkgo.It("should skip empty image IDs", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"watchtower-old",
				"watchtower-old",
				"",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			newContainer := createMockContainer(
				"watchtower-new",
				"watchtower-new",
				"watchtower:new",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).Return(nil)

			var cleanupImageIDs []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				&cleanupImageIDs,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageIDs).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("container chain cleanup", func() {
		ginkgo.It("should cleanup old containers in the chain", func() {
			oldID := types.ContainerID("old123")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			newID := types.ContainerID("new456")

			newContainer := createMockContainer(
				string(newID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"",
				&cleanupImageInfos,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It("should cleanup old container via name belt even when current detection points to the old one", func() {
			oldID := types.ContainerID("old123")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old-old123",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			newID := types.ContainerID("new456")

			newContainer := createMockContainer(
				string(newID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			// Belt case: current detection selected the old predecessor.
			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"",
				&cleanupImageInfos,
				oldContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It("should select same-scope successor when resolving old current container", func() {
			// Old current in scope-a with two non-old successors in different scopes.
			// Must pick the scope-a successor, not the scope-b one.
			oldID := types.ContainerID("old-scoped-a")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old-scoped-a",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-2*time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-a",
				},
			)

			scopeANewID := types.ContainerID("new-scoped-a")

			scopeANewContainer := createMockContainer(
				string(scopeANewID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "scope-a",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			scopeBNewID := types.ContainerID("new-scoped-b")

			scopeBNewContainer := createMockContainer(
				string(scopeBNewID),
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-b",
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// scope-b container appears first in the list to test that it is skipped
			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopeBNewContainer, oldContainer, scopeANewContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"scope-a",
				&cleanupImageInfos,
				oldContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1))
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It("should respect scope boundaries when cleaning up container chains", func() {
			oldID := types.ContainerID("old-scoped")

			oldContainer := createMockContainer(
				string(oldID),
				"watchtower-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			newID := types.ContainerID("new-scoped")

			newContainer := createMockContainer(
				string(newID),
				"watchtower-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(oldID),
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(context.Background(), oldContainer, 10*time.Minute).
				Return(nil).
				Times(1)
			// Should not clean up different scope container

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"prod", // scope specified
				&cleanupImageInfos,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).To(gomega.Equal(1)) // Only oldContainer cleaned
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It("should not clean up chain containers from different scopes", func() {
			// Chain references container from different scope - should not clean it
			oldDifferentScopeID := types.ContainerID("old-different-scope")

			oldDifferentScopeContainer := createMockContainer(
				string(oldDifferentScopeID),
				"watchtower-old-different",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "different-scope",
				},
			)

			newID := types.ContainerID("new-scoped")

			newContainer := createMockContainer(
				string(newID),
				"watchtower-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(oldDifferentScopeID),
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldDifferentScopeContainer, newContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldDifferentScopeContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			// Should attempt to clean up the different scope container
			// Even though it's in the chain, scope isolation does not prevent cleanup when no scope filter

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"prod", // scope specified
				&cleanupImageInfos,
				newContainer,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).
				To(gomega.Equal(1))
				// Should clean up chain container even across scopes when no scope filter
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It("should validate chain cleanup isolation with multiple scopes", func() {
			// Multiple containers in different scopes with chains
			prodOldID := types.ContainerID("prod-old")

			prodOldContainer := createMockContainer(
				string(prodOldID),
				"watchtower-prod-old",
				"watchtower:latest",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			prodNewContainer := createMockContainer(
				"prod-new",
				"watchtower-prod-new",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":                 "true",
					"com.centurylinklabs.watchtower.scope":           "prod",
					"com.centurylinklabs.watchtower.container-chain": string(prodOldID),
				},
			)

			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{prodOldContainer, prodNewContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, prodOldContainer, 10*time.Minute).
				Return(nil).
				Times(1)
			// devOldContainer is not cleaned because unscoped filter excludes scoped containers

			// Test cleanup without scope filter - should clean chained container only
			// Note: unscoped filter excludes containers with scopes, so only chained container is cleaned
			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"", // no scope filter
				&cleanupImageInfos,
				prodNewContainer, // Use prod new as current for prod scope
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cleanupOccurred).
				To(gomega.Equal(1))
				// Only chained container cleaned (scoped containers excluded from unscoped filter)
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})

		ginkgo.It(
			"should clean cross-scope chained containers as parent containers must be removed",
			func() {
				// Container with chain referencing different scope - chained containers are parent containers that must be removed
				crossScopeChainContainer := createMockContainer(
					"cross-chain",
					"watchtower-cross",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":                 "true",
						"com.centurylinklabs.watchtower.scope":           "scope-a",
						"com.centurylinklabs.watchtower.container-chain": "id-from-scope-b",
					},
				)

				// Referenced container from different scope - should be cleaned as chained parent
				referencedContainer := createMockContainer(
					"id-from-scope-b",
					"watchtower-referenced",
					"watchtower:old",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-b",
					},
				)

				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{referencedContainer, crossScopeChainContainer}, nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, referencedContainer, 10*time.Minute).
					Return(nil).
					Times(1)

				// Cleanup should clean the referenced container as it's a chained parent container
				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"scope-a", // Only clean scope-a
					&cleanupImageInfos,
					crossScopeChainContainer,
				)

				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).
					To(gomega.Equal(1))
					// Referenced container cleaned as chained parent
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos[0].ImageID).
					To(gomega.Equal(types.ImageID("watchtower:old")))
			},
		)
	})

	ginkgo.When("error scenarios in scoped operations", func() {
		ginkgo.It(
			"should handle partial failure during scoped cleanup with image removal errors",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				// Create containers in the same scope
				container1 := createMockContainer(
					"scoped-1",
					"watchtower-scoped-1",
					"watchtower:v1",
					true,
					false,
					time.Now().Add(-2*time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)
				container2 := createMockContainer(
					"scoped-2",
					"watchtower-scoped-2",
					"watchtower:v2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)
				currentContainer := createMockContainer(
					"current",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "test-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{container1, container2, currentContainer}, nil)
				// Mock partial failures: container1 stops successfully,
				// container2 fails to stop entirely (should prevent all image cleanup)
				mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, container1, 10*time.Minute).Return(nil)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, container2, 10*time.Minute).
					Return(errors.New("container stop failed"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"test-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("1 of 2 instances failed to stop"))
				gomega.Expect(cleanupOccurred).
					To(gomega.Equal(1))
					// Partial success: one container removed before failure
				gomega.Expect(cleanupImageInfos).
					To(gomega.BeEmpty())
				// Image info cleared on failure
			},
		)

		ginkgo.It(
			"should maintain state consistency when scoped cleanup encounters mixed errors",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				// Create containers with different error scenarios in same scope
				notFoundContainer := createMockContainer(
					"not-found",
					"watchtower-not-found",
					"watchtower:missing",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)
				stopErrorContainer := createMockContainer(
					"stop-error",
					"watchtower-stop-error",
					"watchtower:error",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)
				currentContainer := createMockContainer(
					"current-scoped",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "error-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{notFoundContainer, stopErrorContainer, currentContainer}, nil)
				// Mock different error types
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, notFoundContainer, 10*time.Minute).
					Return(cerrdefs.ErrNotFound)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, stopErrorContainer, 10*time.Minute).
					Return(errors.New("stop failed"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					false, // No image cleanup to focus on container removal
					"error-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("1 of 2 instances failed to stop"))
				gomega.Expect(cleanupOccurred).
					To(gomega.Equal(1))
					// Partial success: one container removed before failure
				gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
			},
		)

		ginkgo.It(
			"should handle scoped cleanup when image removal is interrupted by container failure",
			func() {
				mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

				successContainer := createMockContainer(
					"success",
					"watchtower-success",
					"watchtower:v1",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)
				failureContainer := createMockContainer(
					"failure",
					"watchtower-failure",
					"watchtower:v2",
					true,
					false,
					time.Now().Add(-time.Hour),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)
				currentContainer := createMockContainer(
					"current-interrupt",
					"watchtower-current",
					"watchtower:latest",
					true,
					false,
					time.Now(),
					map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "interrupt-scope",
					},
				)

				mockClient.EXPECT().
					ListContainers(mock.Anything, mock.Anything).
					Return([]types.Container{successContainer, failureContainer, currentContainer}, nil)
				// Success container stops successfully
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, successContainer, 10*time.Minute).
					Return(nil)
				// Failure container causes interruption (should prevent all image cleanup)
				mockClient.EXPECT().
					StopAndRemoveContainer(mock.Anything, failureContainer, 10*time.Minute).
					Return(errors.New("interruption error"))

				var cleanupImageInfos []types.RemovedImageInfo

				cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
					context.Background(),
					mockClient,
					true,
					"interrupt-scope",
					&cleanupImageInfos,
					currentContainer,
				)

				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(cleanupOccurred).To(gomega.Equal(1)) // Partial success before failure
				gomega.Expect(cleanupImageInfos).
					To(gomega.BeEmpty())
				// Image info cleared on any failure
			},
		)

		ginkgo.It("should ensure scope isolation prevents error propagation across scopes", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			// Scope A container that should fail
			scopeAContainer := createMockContainer(
				"scope-a-fail",
				"watchtower-scope-a",
				"watchtower:fail",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-a",
				},
			)

			currentContainer := createMockContainer(
				"current-a",
				"watchtower-current-a",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "scope-a",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{scopeAContainer, currentContainer}, nil)
			// Only scope-a operations should occur
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, scopeAContainer, 10*time.Minute).
				Return(errors.New("scope-a failure"))

			var cleanupImageInfos []types.RemovedImageInfo

			cleanupOccurred, err := RemoveExcessWatchtowerInstances(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"scope-a", // Only clean scope-a
				&cleanupImageInfos,
				currentContainer,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("1 of 1 instances failed to stop"))
			gomega.Expect(cleanupOccurred).To(gomega.Equal(0))
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
		})
	})
})

var _ = ginkgo.Describe("CleanupImages", func() {
	ginkgo.It("should do nothing when no images are provided", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.BeEmpty())
	})

	ginkgo.It("should attempt to remove each image ID", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
			{ImageID: ""}, // empty ID should be skipped
		}

		mockClient.EXPECT().RemoveImageByID(context.Background(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image2"), "").Return(nil)

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, cleanedImages)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.HaveLen(2))
		gomega.Expect(cleaned[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
		gomega.Expect(cleaned[1].ImageID).To(gomega.Equal(types.ImageID("image2")))
	})

	ginkgo.It("should return error when image removal fails", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(context.Background(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(context.Background(), types.ImageID("image2"), "").
			Return(errors.New("image removal failed"))

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, cleanedImages)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).
			To(gomega.ContainSubstring("errors occurred during image cleanup"))
		gomega.Expect(cleaned).To(gomega.HaveLen(1))
		gomega.Expect(cleaned[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
	})

	ginkgo.It("should treat 'not found' errors as non-errors and not add to cleaned", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1"},
			{ImageID: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(context.Background(), types.ImageID("image1"), "").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(context.Background(), types.ImageID("image2"), "").
			Return(cerrdefs.ErrNotFound)

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, cleanedImages)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.HaveLen(1))
		gomega.Expect(cleaned[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
	})

	ginkgo.It("should treat context cancellation as a non-error and not add to cleaned", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
			{ImageID: "image2", ImageName: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(context.Background(), types.ImageID("image2"), "image2").
			Return(context.Canceled)

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, cleanedImages)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.HaveLen(1))
		gomega.Expect(cleaned[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
	})

	ginkgo.It("should treat a canceled context cause that is not context.Canceled as a non-error", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
		}

		canceledCtx, cancel := context.WithCancelCause(context.Background())
		cancel(errors.New("terminated signal received"))

		mockClient.EXPECT().
			RemoveImageByID(canceledCtx, types.ImageID("image1"), "image1").
			Return(errors.New("cannot verify image usage: terminated signal received"))

		cleaned, err := removeImages(newNopLogger(), canceledCtx, mockClient, cleanedImages)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.BeEmpty())
	})

	ginkgo.It("should treat 'conflict' errors as non-errors and not add to cleaned", func() {
		mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

		cleanedImages := []types.RemovedImageInfo{
			{ImageID: "image1", ImageName: "image1"},
			{ImageID: "image2", ImageName: "image2"},
		}

		mockClient.EXPECT().RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1").Return(nil)
		mockClient.EXPECT().
			RemoveImageByID(context.Background(), types.ImageID("image2"), "image2").
			Return(cerrdefs.ErrConflict)

		cleaned, err := removeImages(newNopLogger(), context.Background(), mockClient, cleanedImages)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(cleaned).To(gomega.HaveLen(1))
		gomega.Expect(cleaned[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
	})
})

var _ = ginkgo.Describe("removeExcessContainers", func() {
	ginkgo.When("removeImageInfos is nil", func() {
		ginkgo.It("should remove the images of removed instances right away", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().StopAndRemoveContainer(context.Background(), excessContainer, 10*time.Minute).Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(context.Background(), types.ImageID("image1"), "image1:latest").
				Return(nil)

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				true,
				currentContainer,
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should log removeImages errors and still report the removed instances", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer1 := createMockContainer(
				"excess1",
				"excess1",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			excessContainer2 := createMockContainer(
				"excess2",
				"excess2",
				"image2",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image3",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer1, 10*time.Minute).Return(nil)
			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer2, 10*time.Minute).Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(mock.Anything, types.ImageID("image1"), "image1:latest").
				Return(nil)
			mockClient.EXPECT().
				RemoveImageByID(mock.Anything, types.ImageID("image2"), "image2:latest").
				Return(errors.New("remove failed"))

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer1, excessContainer2},
				true,
				currentContainer,
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(2))
		})
	})

	ginkgo.When("removeImageInfos is not nil", func() {
		ginkgo.It("should append to provided slice without removing the images", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			var removeInfos []types.RemovedImageInfo

			mockClient.EXPECT().StopAndRemoveContainer(context.Background(), excessContainer, 10*time.Minute).Return(nil)

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				true,
				currentContainer,
				&removeInfos,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
			gomega.Expect(removeInfos).To(gomega.HaveLen(1))
			gomega.Expect(removeInfos[0].ImageID).To(gomega.Equal(types.ImageID("image1")))
		})
	})

	ginkgo.When("cleanupImages is false", func() {
		ginkgo.It("should not call removeImages", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			var removeInfos []types.RemovedImageInfo

			mockClient.EXPECT().StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).Return(nil)
			// No RemoveImageByID expected

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				&removeInfos,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
			gomega.Expect(removeInfos).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("context cancellation during retry delay", func() {
		ginkgo.It("should return immediately when context is canceled during retry delay", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// Create a canceled context
			ctx, cancel := context.WithCancel(context.Background())
			// Cancel immediately to simulate cancellation during retry delay
			cancel()

			// First call fails, triggering retry with context cancellation
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(1)

			removed, err := removeExcessContainers(newNopLogger(),
				ctx,
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("context canceled during retry delay"))
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should return immediately when context times out during retry delay", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// Create an already-expired context to ensure it is done
			// before the retry loop's select statement runs.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// First call fails, triggering retry with expired context
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(1)

			removed, err := removeExcessContainers(newNopLogger(),
				ctx,
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("context canceled during retry delay"))
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should proceed with retry after delay when context is not canceled", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// First call fails, second call succeeds after retry delay
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(1)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should retry multiple times before succeeding when context is not canceled", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// First two attempts fail, third attempt succeeds
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(2)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(nil).
				Times(1)

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should fail after max attempts when context is never canceled", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			excessContainer := createMockContainer(
				"excess",
				"excess",
				"image1",
				true,
				false,
				time.Now().Add(-time.Hour),
				map[string]string{},
			)
			currentContainer := createMockContainer(
				"current",
				"current",
				"image2",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			// All attempts fail
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, excessContainer, 10*time.Minute).
				Return(errors.New("container stop failed")).
				Times(maxRemovalAttempts)

			removed, err := removeExcessContainers(newNopLogger(),
				context.Background(),
				mockClient,
				[]types.Container{excessContainer},
				false,
				currentContainer,
				nil,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("1 of 1 instances failed to stop"))
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})
})

var _ = ginkgo.Describe("containerNames", func() {
	ginkgo.It("should return empty slice for empty container list", func() {
		containers := []types.Container{}
		result := containerNames(containers)
		gomega.Expect(result).To(gomega.BeEmpty())
	})

	ginkgo.It("should return container names", func() {
		container1 := createMockContainer("id1", "name1", "image1", true, false, time.Now(), nil)
		container2 := createMockContainer("id2", "name2", "image2", true, false, time.Now(), nil)

		containers := []types.Container{container1, container2}
		result := containerNames(containers)
		gomega.Expect(result).To(gomega.Equal([]string{"name1", "name2"}))
	})
})

// createMockContainer is a helper function to create a mock container for testing.
//
//nolint:unparam // running parameter is intentionally fixed for test purposes
func createMockContainer(
	id, name, image string,
	running, restarting bool,
	created time.Time,
	labels map[string]string,
) types.Container {
	if labels == nil {
		labels = make(map[string]string)
	}

	content := dockerContainer.InspectResponse{
		ID:    id,
		Image: image,
		Name:  name,
		State: &dockerContainer.State{
			Running:    running,
			Restarting: restarting,
		},
		Created: created.Format(time.RFC3339Nano),
		HostConfig: &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{},
		},
		Config: &dockerContainer.Config{
			Image:        image,
			Labels:       labels,
			ExposedPorts: dockerNetwork.PortSet{},
		},
	}

	imageInfo := &dockerImage.InspectResponse{
		ID: image,
		RepoDigests: []string{
			image + "@sha256:" + strings.ReplaceAll(image, ":", "_"),
		},
	}

	return container.NewContainer(nil, &content, imageInfo)
}

// createMockCreatedContainer creates a Watchtower container in the "created" state,
// simulating a container that was created but never started during a failed self-update.
func createMockCreatedContainer(
	id, name, image string,
	created time.Time,
	labels map[string]string,
) types.Container {
	c := createMockContainer(id, name, image, false, false, created, labels)

	info := c.ContainerInfo()
	if info != nil && info.State != nil {
		info.State.Status = dockerContainer.StateCreated
	}

	return c
}

var _ = ginkgo.Describe("cleanupOldWatchtowerContainers", func() {
	ginkgo.When("no old containers exist", func() {
		ginkgo.It("should return nil when no containers exist", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should return nil when only non-Watchtower containers exist", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			nonWT := createMockContainer(
				"other-id",
				"other-app",
				"other:latest",
				true,
				false,
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{nonWT}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should return nil when only current non-old Watchtower exists", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			current := createMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{current}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})

	ginkgo.When("scope normalization applies", func() {
		ginkgo.It("should match unscoped old containers when scope is empty", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})
	})

	ginkgo.When("scope filtering applies", func() {
		ginkgo.It("should skip old containers in different scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldScopedContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "other-scope",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldScopedContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"my-scope",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should skip old containers with explicit scope when current is unscoped", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldScopedContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "some-scope",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldScopedContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})

	ginkgo.When("old containers are found and removed", func() {
		ginkgo.It("should remove old container in same scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should remove old container with matching explicit scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"prod",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should skip old container with same ID as current container", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should remove multiple old containers in same scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			old1 := createMockContainer(
				"old-1",
				"watchtower-old-aaa",
				"watchtower:old1",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)
			old2 := createMockContainer(
				"old-2",
				"watchtower-old-bbb",
				"watchtower:old2",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{old1, old2}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old1, 10*time.Minute).
				Return(nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, old2, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"prod",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(2))
		})
	})

	ginkgo.When("cleanup images is enabled", func() {
		ginkgo.It("should remove old container and collect its image", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			currentContainer := createMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{currentContainer, oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			var imageInfos []types.RemovedImageInfo

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				true,
				"none",
				"current-id",
				&imageInfos,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
			gomega.Expect(imageInfos).To(gomega.HaveLen(1))
		})

		ginkgo.It("should handle removal failure and return partial count", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(errors.New("stop container failed")).
				Times(maxRemovalAttempts)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})

	ginkgo.When("list containers fails", func() {
		ginkgo.It("should return an error", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return(nil, errors.New("docker daemon unreachable"))

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to list containers"))
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})

	ginkgo.When("current container not found in list", func() {
		ginkgo.It("should still remove old containers with nil current", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"nonexistent-current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})
	})

	ginkgo.When("orphaned created-state containers exist", func() {
		ginkgo.It("should remove Watchtower containers stuck in created state", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			orphaned := createMockCreatedContainer(
				"orphaned-id",
				"watchtower-abc123",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{orphaned}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, orphaned, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(1))
		})

		ginkgo.It("should skip created-state containers that are not Watchtower", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			nonWT := createMockCreatedContainer(
				"non-wt-id",
				"other-app",
				"other:latest",
				time.Now(),
				map[string]string{},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{nonWT}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should skip the current container even if it is in created state", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			current := createMockCreatedContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{current}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})

		ginkgo.It("should remove both old and orphaned created-state containers together", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			oldContainer := createMockContainer(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:old",
				true,
				false,
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)
			orphaned := createMockCreatedContainer(
				"orphaned-id",
				"watchtower-xyz789",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower": "true",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{oldContainer, orphaned}, nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, oldContainer, 10*time.Minute).
				Return(nil)
			mockClient.EXPECT().
				StopAndRemoveContainer(mock.Anything, orphaned, 10*time.Minute).
				Return(nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(2))
		})

		ginkgo.It("should skip created-state containers from a different scope", func() {
			mockClient := mockContainer.NewMockClient(ginkgo.GinkgoT())

			differentScopeOrphaned := createMockCreatedContainer(
				"orphaned-scoped-id",
				"watchtower-scoped",
				"watchtower:latest",
				time.Now(),
				map[string]string{
					"com.centurylinklabs.watchtower":       "true",
					"com.centurylinklabs.watchtower.scope": "prod",
				},
			)

			mockClient.EXPECT().
				ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{differentScopeOrphaned}, nil)

			removed, err := cleanupOldWatchtowerContainers(newNopLogger(),
				context.Background(),
				mockClient,
				false,
				"none",
				"current-id",
				nil,
			)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(removed).To(gomega.Equal(0))
		})
	})
})
