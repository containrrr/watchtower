package actions_test

import (
	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/types"
	container2 "github.com/docker/docker/api/types/container"
	cli "github.com/docker/docker/client"
	"time"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the update action", func() {
	var dockerClient cli.CommonAPIClient
	var client MockClient

	BeforeEach(func() {
		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
	})

	When("watchtower has been instructed to clean up", func() {
		BeforeEach(func() {
			pullImages := false
			removeVolumes := false
			client = CreateMockClient(
				&TestData{
					NameOfContainerToKeep: "test-container-02",
					Containers: []container.Container{
						CreateMockContainer(
							"test-container-01",
							"test-container-01",
							"fake-image:latest",
							time.Now().AddDate(0, 0, -1)),
						CreateMockContainer(
							"test-container-02",
							"test-container-02",
							"fake-image:latest",
							time.Now()),
						CreateMockContainer(
							"test-container-02",
							"test-container-02",
							"fake-image:latest",
							time.Now()),
					},
				},
				dockerClient,
				pullImages,
				removeVolumes,
			)
		})

		When("there are multiple containers using the same image", func() {
			It("should only try to remove the image once", func() {

				_, err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(1))
			})
		})
		When("there are multiple containers using different images", func() {
			It("should try to remove each of them", func() {
				client.TestData.Containers = append(
					client.TestData.Containers,
					CreateMockContainer(
						"unique-test-container",
						"unique-test-container",
						"unique-fake-image:latest",
						time.Now(),
					),
				)
				_, err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(2))
			})
		})
		When("performing a rolling restart update", func() {
			It("should try to remove the image once", func() {

				_, err := actions.Update(client, types.UpdateParams{Cleanup: true, RollingRestart: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(1))
			})
		})
	})

	When("watchtower has been instructed to monitor only", func() {
		When("certain containers are set to monitor only", func() {
			BeforeEach(func() {
				client = CreateMockClient(
					&TestData{
						NameOfContainerToKeep: "test-container-02",
						Containers: []container.Container{
							CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image1:latest",
								time.Now()),
							CreateMockContainerWithConfig(
								"test-container-02",
								"test-container-02",
								"fake-image2:latest",
								time.Now(),
								&container2.Config{
									Labels: map[string]string{
										"com.centurylinklabs.watchtower.monitor-only": "true",
									},
								}),
						},
					},
					dockerClient,
					false,
					false,
				)
			})

			It("should not update those containers", func() {
				_, err := actions.Update(client, types.UpdateParams{Cleanup: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(1))
			})
		})

		When("monitor only is set globally", func() {
			BeforeEach(func() {
				client = CreateMockClient(
					&TestData{
						Containers: []container.Container{
							CreateMockContainer(
								"test-container-01",
								"test-container-01",
								"fake-image:latest",
								time.Now()),
							CreateMockContainer(
								"test-container-02",
								"test-container-02",
								"fake-image:latest",
								time.Now()),
						},
					},
					dockerClient,
					false,
					false,
				)
			})

			It("should not update any containers", func() {
				_, err := actions.Update(client, types.UpdateParams{MonitorOnly: true})
				Expect(err).NotTo(HaveOccurred())
				Expect(client.TestData.TriedToRemoveImageCount).To(Equal(0))
			})
		})

	})
})
