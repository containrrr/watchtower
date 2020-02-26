package container

import (
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/docker/docker/api/types"
	container2 "github.com/docker/docker/api/types/container"
	cli "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	docker *cli.Client
	client Client
	c      Container
)

var _ = Describe("Client-SetMaxMemoryLimit", func() {
	BeforeEach(func() {
		server := mocks.NewMockAPIServer()
		docker, _ := cli.NewClientWithOpts(
			cli.WithHost(server.URL),
			cli.WithHTTPClient(server.Client()))

		client = dockerClient{
			api:        docker,
			pullImages: false,
		}
		c = CreateMockContainer(
			"test-container-01",
			"test-container-01",
			"fake-image:latest",
		)

	})

	When("When container memory limit is too high", func() {
		It("should reduce it to the configured limit", func() {
			limit := int64(2147483648)
			c.ContainerInfo().HostConfig.Memory = 8589934592 //8G
			apply, err := client.SetMaxMemoryLimit(c, limit)

			Expect(err).NotTo(HaveOccurred())
			Expect(apply).To(BeTrue())
			Expect(limit == c.ContainerInfo().HostConfig.Memory).To(BeTrue())
		})
	})
	When("When container has no memory limit", func() {
		It("should set it to the configured limit", func() {
			limit := int64(8589934592) // limit 8G

			c.ContainerInfo().HostConfig.Memory = 0 // has no limit, will use amount host memory if needed
			apply, err := client.SetMaxMemoryLimit(c, limit)

			Expect(err).NotTo(HaveOccurred())
			Expect(apply).To(BeTrue())
			Expect(limit == c.ContainerInfo().HostConfig.Memory).To(BeTrue())
		})
	})

	When("When container memory limit is then or egal configured limit", func() {
		It("should do nothing", func() {
			limit := int64(9663676416)                       // limit 9G
			c.ContainerInfo().HostConfig.Memory = 8589934592 // 8G
			apply, err := client.SetMaxMemoryLimit(c, limit)

			Expect(err).NotTo(HaveOccurred())
			Expect(apply).To(BeFalse())
			Expect(limit > c.ContainerInfo().HostConfig.Memory).To(BeTrue())
		})
	})
})

// CreateMockContainer creates a container substitute valid for testing
func CreateMockContainer(id string, name string, image string) Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         id,
			Name:       name,
			HostConfig: &container2.HostConfig{},
		},
	}
	return *NewContainer(
		&content,
		&types.ImageInspect{
			ID: image,
		},
	)
}
