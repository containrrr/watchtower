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

var _ = ginkgo.Describe("the update action", func() {
	ginkgo.When("watchtower has been instructed to monitor only", func() {
		ginkgo.When("certain containers are set to monitor only", func() {
			ginkgo.It("should not update those containers and collect no image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						NameOfContainerToKeep: "test-container-02",
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image1:latest",
								time.Now(),
							),
							mockActions.CreateMockContainerWithConfig(
								"test-container-02",
								"/test-container-02",
								"fake-image2:latest",
								false,
								false,
								time.Now(),
								&dockerContainer.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.monitor-only": "true",
									},
								},
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
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

		ginkgo.When("monitor only is set globally", func() {
			ginkgo.It("should not update any containers and collect no image IDs", func() {
				client := mockActions.CreateMockClient(
					&mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image:latest",
								time.Now(),
							),
							mockActions.CreateMockContainer(
								"test-container-02",
								"test-container-02",
								"fake-image:latest",
								time.Now(),
							),
						},
					},
					false,
					false,
				)
				client.TestData.Staleness = map[string]bool{
					"test-container-01": true,
					"test-container-02": true,
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					types.UpdateParams{Cleanup: true, MonitorOnly: true},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Updated()).To(gomega.BeEmpty())
				gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
			})

			ginkgo.When("watchtower has been instructed to have label take precedence", func() {
				ginkgo.It("it should update containers when monitor only is set to false", func() {
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
											"com.centurylinklabs.watchtower.monitor-only": "false",
										},
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
							Cleanup:         true,
							MonitorOnly:     true,
							LabelPrecedence: true,
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

				ginkgo.It(
					"it should not update containers when monitor only is set to true",
					func() {
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
												"com.centurylinklabs.watchtower.monitor-only": "true",
											},
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
								Cleanup:         true,
								MonitorOnly:     true,
								LabelPrecedence: true,
							},
						)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						gomega.Expect(report.Updated()).To(gomega.BeEmpty())
						gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
						gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
							To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
					},
				)

				ginkgo.It("it should not update containers when monitor only is not set", func() {
					client := mockActions.CreateMockClient(
						&mockActions.TestData{
							Containers: []types.Container{
								mockActions.CreateMockContainer(
									"test-container-01",
									"test-container-01",
									"fake-image:latest",
									time.Now(),
								),
							},
						},
						false,
						false,
					)
					client.TestData.Staleness = map[string]bool{
						"test-container-01": true,
					}
					report, cleanupImageInfos, err := actions.Update(newNopLogger(),
						context.Background(),
						client,
						types.UpdateParams{
							Cleanup:         true,
							MonitorOnly:     true,
							LabelPrecedence: true,
						},
					)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(report.Updated()).To(gomega.BeEmpty())
					gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
					gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
						To(gomega.Equal(int32(0)), "RemoveImageByID should not be called during Update")
				})
			})
		})
	})
})
