package actions_test

import (
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/filters"
	"github.com/containrrr/watchtower/pkg/types"
	cli "github.com/docker/docker/client"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the setResourceLimit action", func() {
	var dockerClient cli.CommonAPIClient
	var client MockClient

	BeforeEach(func() {

		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
	})

	When("watchtower has been instructed to apply resource limit", func() {
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

		When("Setting memory limit on empty list", func() {
			It("Shouldn´t throw error when list is empty", func() {
				client.TestData.Containers = client.TestData.Containers[:0] // clear the container list
				limit := int64(2147483648)
				err := actions.SetResourceLimit(client, types.UpdateParams{MaxMemoryPerContainer: limit})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("Reducing memory to limit ", func() {
			It("should try to set limit to 2g", func() {
				limit := int64(2147483648)
				err := actions.SetResourceLimit(client, types.UpdateParams{MaxMemoryPerContainer: limit})
				Expect(err).NotTo(HaveOccurred())
				containers, error := client.ListContainers(filters.NoFilter)
				Expect(error).NotTo(HaveOccurred())
				memory := containers[0].ContainerInfo().HostConfig.Memory
				Expect(memory).To(Equal(limit))
			})
		})
	})
})
