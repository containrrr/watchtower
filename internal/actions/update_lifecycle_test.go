package actions_test

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("the update action", func() {
	ginkgo.When("watchtower has been instructed to run lifecycle hooks", func() {
		ginkgo.When("pre-update script returns 1", func() {
			ginkgo.It("should not update those containers and collect no image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.BeEmpty())
				gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("lifecycle UID and GID are specified", func() {
			ginkgo.It("should pass UID and GID to lifecycle hook execution", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-uid-gid",
								"test-container-uid-gid",
								"fake-image:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update": "/PreUpdateReturn0.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-uid-gid": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
						LifecycleUID:   1000,
						LifecycleGID:   1001,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("preupdate script returns 75", func() {
			ginkgo.It("should skip those containers and collect no image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn75.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.BeEmpty())
				gomega.Expect(report.Failed()).To(gomega.BeEmpty())
				gomega.Expect(report.Skipped()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("preupdate script returns 0", func() {
			ginkgo.It("should update those containers and collect image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								true,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn0.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image2:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("container is linked to restarting containers", func() {
			ginkgo.It("should be marked for restart and collect image IDs", func() {
				provider := mockActions.CreateMockContainerWithConfig(
					"test-container-provider",
					"/test-container-provider",
					"fake-image2:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				provider.SetStale(true)

				consumer := mockActions.CreateMockContainerWithConfig(
					"test-container-consumer",
					"/test-container-consumer",
					"fake-image3:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "test-container-provider",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{
					provider,
					consumer,
				}

				gomega.Expect(provider.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(consumer.ToRestart()).To(gomega.BeFalse())

				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containers[1].ToRestart()).To(gomega.BeTrue())
			})

			ginkgo.It(
				"should propagate restart in Docker Compose with service name mismatch",
				func() {
					testData := getComposeTestData()
					containers := testData.Containers

					// db container is stale
					containers[0].SetStale(true)

					gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue())  // db
					gomega.Expect(containers[1].ToRestart()).To(gomega.BeFalse()) // web

					actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

					// web should be marked for restart because it depends on db
					gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue())
					gomega.Expect(containers[1].ToRestart()).To(gomega.BeTrue())
				},
			)

			ginkgo.It(
				"should propagate restart through multi-hop chains in Docker Compose",
				func() {
					testData := getComposeMultiHopTestData()
					containers := testData.Containers

					// cache container is stale
					containers[0].SetStale(true)

					gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue())  // cache
					gomega.Expect(containers[1].ToRestart()).To(gomega.BeFalse()) // db
					gomega.Expect(containers[2].ToRestart()).To(gomega.BeFalse()) // app

					actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

					// All should be marked for restart: cache -> db -> app
					gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue())
					gomega.Expect(containers[1].ToRestart()).To(gomega.BeTrue())
					gomega.Expect(containers[2].ToRestart()).To(gomega.BeTrue())
				},
			)
			ginkgo.It("should propagate restart through chained dependencies", func() {
				// Create a transitive dependency chain: A depends on B, B depends on C
				containerC := mockActions.CreateMockContainerWithConfig(
					"test-container-c",
					"/test-container-c",
					"fake-image-c:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels:       map[string]string{},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerB := mockActions.CreateMockContainerWithConfig(
					"test-container-b",
					"/test-container-b",
					"fake-image-b:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "test-container-c",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containerA := mockActions.CreateMockContainerWithConfig(
					"test-container-a",
					"/test-container-a",
					"fake-image-a:latest",
					true,
					false,
					time.Now(),
					&dockerContainer.Config{
						Labels: map[string]string{
							"com.centurylinklabs.watchtower.depends-on": "test-container-b",
						},
						ExposedPorts: dockerNetwork.PortSet{},
					},
				)

				containers := []types.Container{
					containerC,
					containerB,
					containerA,
				}

				// Initially, only C should be marked for restart
				containerC.SetStale(true)
				gomega.Expect(containerC.ToRestart()).To(gomega.BeTrue())
				gomega.Expect(containerB.ToRestart()).To(gomega.BeFalse())
				gomega.Expect(containerA.ToRestart()).To(gomega.BeFalse())

				// Run UpdateImplicitRestart to propagate restart through the chain
				actions.UpdateImplicitRestart(newNopLogger(), containers, containers, true)

				// Verify that restart propagates: A and B should now be marked for restart
				gomega.Expect(containers[0].ToRestart()).To(gomega.BeTrue()) // C
				gomega.Expect(containers[1].ToRestart()).To(gomega.BeTrue()) // B
				gomega.Expect(containers[2].ToRestart()).To(gomega.BeTrue()) // A
			})
		})

		ginkgo.When("container is not running", func() {
			ginkgo.It("should skip running preupdate and collect image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								false,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image2:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})

		ginkgo.When("container is restarting", func() {
			ginkgo.It("should skip running preupdate and collect image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								false,
								true,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "190",
										"com.centurylinklabs.watchtower.lifecycle.pre-update":         "/PreUpdateReturn1.sh",
									},
									ExposedPorts: dockerNetwork.PortSet{},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{
						Cleanup:        true,
						LifecycleHooks: true,
					},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.HaveLen(1))
				gomega.Expect(cleanupImageInfos).
					To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("fake-image2:latest"))))
				gomega.Expect(cleanupImageInfos).To(gomega.HaveLen(1))
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})
		})
	})
})
