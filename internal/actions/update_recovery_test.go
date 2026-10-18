package actions_test

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("TryRecoverOrphanedContainer", func() {
	ginkgo.When("listing containers fails", func() {
		ginkgo.It("should return false without error", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{
				ListContainersError: context.DeadlineExceeded,
			}, false, false)

			current := mockActions.CreateMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				time.Now(),
			)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
		})
	})

	ginkgo.When("no orphaned container exists", func() {
		ginkgo.It("should return false", func() {
			oldContainer := mockActions.CreateMockContainerWithConfig(
				"old-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image:  "watchtower:latest",
					Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
				},
			)
			current := mockActions.CreateMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				time.Now(),
			)

			client := mockActions.CreateMockClient(&mockActions.TestData{
				Containers: []types.Container{oldContainer, current},
			}, false, false)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
		})
	})

	ginkgo.When("an orphaned created-state Watchtower container exists", func() {
		ginkgo.It("should start it and return the recovered container", func() {
			config := &dockerContainer.Config{
				Image:  "watchtower:latest",
				Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			}
			orphaned := mockActions.CreateMockContainerWithConfig(
				"orphaned-id",
				"watchtower-xyz",
				"watchtower:latest",
				false,
				false,
				time.Now(),
				config,
			)
			orphaned.ContainerInfo().State.Status = dockerContainer.StateCreated

			current := mockActions.CreateMockContainerWithConfig(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				config,
			)

			client := mockActions.CreateMockClient(&mockActions.TestData{
				Containers: []types.Container{orphaned, current},
			}, false, false)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(recovered).NotTo(gomega.BeNil())
			gomega.Expect(recovered.ID()).To(gomega.Equal(types.ContainerID("orphaned-id")))
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(1)))
		})
	})

	ginkgo.When("start fails for the orphaned container", func() {
		ginkgo.It("should continue searching and return false", func() {
			config := &dockerContainer.Config{
				Image:  "watchtower:latest",
				Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			}
			orphaned := mockActions.CreateMockContainerWithConfig(
				"orphaned-id",
				"watchtower-xyz",
				"watchtower:latest",
				false,
				false,
				time.Now(),
				config,
			)
			orphaned.ContainerInfo().State.Status = dockerContainer.StateCreated

			current := mockActions.CreateMockContainerWithConfig(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				config,
			)

			client := mockActions.CreateMockClient(&mockActions.TestData{
				Containers:              []types.Container{orphaned, current},
				StartContainerByIDError: context.DeadlineExceeded,
			}, false, false)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(1)))
		})
	})

	ginkgo.When("the current container is nil", func() {
		ginkgo.It("should not panic and return false", func() {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			gomega.Expect(func() {
				actions.TryRecoverOrphanedContainer(newNopLogger(),
					context.Background(),
					client,
					nil,
				)
			}).NotTo(gomega.Panic())

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				nil,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
		})
	})

	ginkgo.When("the orphaned container is in a different scope", func() {
		ginkgo.It("should not start it", func() {
			orphaned := mockActions.CreateMockContainerWithConfig(
				"orphaned-id",
				"watchtower-xyz",
				"watchtower:latest",
				false,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-b",
					},
				},
			)
			orphaned.ContainerInfo().State.Status = dockerContainer.StateCreated

			current := mockActions.CreateMockContainerWithConfig(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-a",
					},
				},
			)

			client := mockActions.CreateMockClient(&mockActions.TestData{
				Containers: []types.Container{orphaned, current},
			}, false, false)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))
		})
	})

	ginkgo.When("the current container has no scope and the orphaned container has an explicit scope", func() {
		ginkgo.It("should not start it", func() {
			orphaned := mockActions.CreateMockContainerWithConfig(
				"orphaned-id",
				"watchtower-xyz",
				"watchtower:latest",
				false,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower":       "true",
						"com.centurylinklabs.watchtower.scope": "scope-b",
					},
				},
			)
			orphaned.ContainerInfo().State.Status = dockerContainer.StateCreated

			current := mockActions.CreateMockContainerWithConfig(
				"current-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "watchtower:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower": "true",
					},
				},
			)

			client := mockActions.CreateMockClient(&mockActions.TestData{
				Containers: []types.Container{orphaned, current},
			}, false, false)

			recovered, ok := actions.TryRecoverOrphanedContainer(newNopLogger(),
				context.Background(),
				client,
				current,
			)

			gomega.Expect(ok).To(gomega.BeFalse())
			gomega.Expect(recovered).To(gomega.BeNil())
			gomega.Expect(client.TestData.StartContainerCount.Load()).To(gomega.Equal(int32(0)))
		})
	})
})
