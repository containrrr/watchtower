package actions_test

import (
	"time"

	"github.com/containrrr/watchtower/internal/actions"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/types"
	cli "github.com/docker/docker/client"

	. "github.com/containrrr/watchtower/internal/actions/mocks"
	container2 "github.com/docker/docker/api/types/container"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the setResourceLimit action", func() {
	var dockerClient cli.CommonAPIClient
	var client MockClient
	var dummyContainer container.Container
	BeforeEach(func() {

		server := mocks.NewMockAPIServer()
		dockerClient, _ = cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))
		dummyContainer = CreateMockContainer(
			"test-container-01",
			"test-container-01",
			"fake-image:latest",
			time.Now().AddDate(0, 0, -1))
		//hostConfig := &container2.HostConfig{}
		//hostConfig.Memory = 8589934592
		dummyContainer.ContainerInfo().HostConfig = &container2.HostConfig{}

		pullImages := false
		removeVolumes := false
		client = CreateMockClient(
			&TestData{
				NameOfContainerToKeep: "test-container-02",
				Containers:            []container.Container{dummyContainer},
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

	When("Reducing memory ", func() {
		It("should set limit to given limit", func() {
			limit := int64(2147483648)
			err := actions.SetResourceLimit(client, types.UpdateParams{MaxMemoryPerContainer: limit})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
