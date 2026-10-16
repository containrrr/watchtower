package actions_test

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("the update action", func() {
	ginkgo.When("watchtower has been instructed to clean up", func() {
		ginkgo.When("there are multiple containers using the same image", func() {
			ginkgo.It("should collect one entry per container for deferred cleanup", func() {
				client := mockActions.CreateMockClient(getCommonTestData(), false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
					"test-container-03": true,
				}
				report, cleanupImageInfos, err := actions.Update(testLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(3))
				// Each container using the same image gets its own entry so that
				// split-by-container notifications report correctly. Image-level
				// deduplication happens later in performImageCleanup.
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(3))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image:latest"))))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ContainerName", "test-container-01")))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ContainerName", "test-container-02")))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ContainerName", "test-container-03")))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("there are multiple containers using different images", func() {
			ginkgo.It("should collect one entry per container for each image", func() {
				testData := getCommonTestData()
				testData.Containers = append(
					testData.Containers,
					mockActions.CreateMockContainer(
						"unique-test-container",
						"unique-test-container",
						"unique-fake-image:latest",
						time.Now(),
					),
				)
				client := mockActions.CreateMockClient(testData, false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01":     true,
					"test-container-02":     true,
					"test-container-03":     true,
					"unique-test-container": true,
				}
				report, cleanupImageInfos, err := actions.Update(testLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(4))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image:latest"))))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("unique-fake-image:latest"))))
				// 3 containers for fake-image:latest + 1 container for unique-fake-image:latest
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(4))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("there are linked containers being updated", func() {
			ginkgo.It("should collect only the stale container's image ID", func() {
				client := mockActions.CreateMockClient(getLinkedTestData(true), false, false)
				client.TestData.Staleness["test-container-01"] = true
				report, cleanupImageInfos, err := actions.Update(testLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image1:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("performing a rolling restart update", func() {
			ginkgo.It("should collect one entry per container for deferred cleanup", func() {
				client := mockActions.CreateMockClient(getCommonTestData(), false, false)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
					"test-container-03": true,
				}
				report, cleanupImageInfos, err := actions.Update(testLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true, RollingRestart: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(3))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image:latest"))))
				// Each container using the same image gets its own entry for split notifications.
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(3))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
				gomega.Expect(client.TestData.WaitForContainerHealthyCount.Load()).
					To(gomega.Equal(int32(3)), "WaitForContainerHealthy should be called for each updated container")
			})
		})

		ginkgo.When("updating a linked container with missing image info", func() {
			ginkgo.It("should gracefully fail and collect no image IDs", func() {
				client := mockActions.CreateMockClient(getLinkedTestData(false), false, false)
				client.TestData.Staleness["test-container-01"] = true
				report, cleanupImageInfos, err := actions.Update(testLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(report.Fresh()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image1:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})
	})
})
