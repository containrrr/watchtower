package container

import (
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	cli "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Container Suite")
}

var _ = Describe("the container", func() {
	Describe("the client", func() {
		var docker *cli.Client
		var client Client
		BeforeSuite(func() {
			server := mocks.NewMockAPIServer()
			docker, _ = cli.NewClientWithOpts(
				cli.WithHost(server.URL),
				cli.WithHTTPClient(server.Client()))
			client = dockerClient{
				api:        docker,
				pullImages: false,
			}
		})
		It("should return a client for the api", func() {
			Expect(client).NotTo(BeNil())
		})
		When("listing containers without any filter", func() {
			It("should return all available containers", func() {
				containers, err := client.ListContainers(noFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 2).To(BeTrue())
			})
		})
		When("listing containers with a filter matching nothing", func() {
			It("should return an empty array", func() {
				filter := filterByNames([]string{"lollercoaster"}, noFilter)
				containers, err := client.ListContainers(filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 0).To(BeTrue())
			})
		})
		When("listing containers with a watchtower filter", func() {
			It("should return only the watchtower container", func() {
				containers, err := client.ListContainers(WatchtowerContainersFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) == 1).To(BeTrue())
				Expect(containers[0].ImageName()).To(Equal("containrrr/watchtower:latest"))
			})
		})
		When(`listing containers with the "include stopped" option`, func() {
			It("should return both stopped and running containers", func() {
				client = dockerClient{
					api:            docker,
					pullImages:     false,
					includeStopped: true,
				}
				containers, err := client.ListContainers(noFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(containers) > 0).To(BeTrue())
			})
		})
	})
	When("asked for metadata", func() {
		var c *Container
		BeforeEach(func() {
			c = mockContainerWithLabels(map[string]string{
				"com.centurylinklabs.watchtower.enable": "true",
				"com.centurylinklabs.watchtower":        "true",
			})
		})
		It("should return its name on calls to .Name()", func() {
			name := c.Name()
			Expect(name).To(Equal("test-containrrr"))
			Expect(name).NotTo(Equal("wrong-name"))
		})
		It("should return its ID on calls to .ID()", func() {
			id := c.ID()

			Expect(id).To(Equal("container_id"))
			Expect(id).NotTo(Equal("wrong-id"))
		})
		It("should return true, true if enabled on calls to .Enabled()", func() {
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeTrue())
			Expect(enabled).NotTo(BeFalse())
			Expect(exists).To(BeTrue())
			Expect(exists).NotTo(BeFalse())
		})
		It("should return false, true if present but not true on calls to .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower.enable": "false"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeTrue())
			Expect(exists).NotTo(BeFalse())
		})
		It("should return false, false if not present on calls to .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"lol": "false"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeFalse())
			Expect(exists).NotTo(BeTrue())
		})
		It("should return false, false if present but not parsable .Enabled()", func() {
			c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower.enable": "falsy"})
			enabled, exists := c.Enabled()

			Expect(enabled).To(BeFalse())
			Expect(enabled).NotTo(BeTrue())
			Expect(exists).To(BeFalse())
			Expect(exists).NotTo(BeTrue())
		})
		When("checking if its a watchtower instance", func() {
			It("should return true if the label is set to true", func() {
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeTrue())
			})
			It("should return false if the label is present but set to false", func() {
				c = mockContainerWithLabels(map[string]string{"com.centurylinklabs.watchtower": "false"})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
			It("should return false if the label is not present", func() {
				c = mockContainerWithLabels(map[string]string{"funny.label": "false"})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
			It("should return false if there are no labels", func() {
				c = mockContainerWithLabels(map[string]string{})
				isWatchtower := c.IsWatchtower()
				Expect(isWatchtower).To(BeFalse())
			})
		})
		When("fetching the custom stop signal", func() {
			It("should return the signal if its set", func() {
				c = mockContainerWithLabels(map[string]string{
					"com.centurylinklabs.watchtower.stop-signal": "SIGKILL",
				})
				stopSignal := c.StopSignal()
				Expect(stopSignal).To(Equal("SIGKILL"))
			})
			It("should return an empty string if its not set", func() {
				c = mockContainerWithLabels(map[string]string{})
				stopSignal := c.StopSignal()
				Expect(stopSignal).To(Equal(""))
			})
		})
		When("fetching the image name", func() {
			When("the zodiac label is present", func() {
				It("should fetch the image name from it", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.zodiac.original-image": "the-original-image",
					})
					imageName := c.ImageName()
					Expect(imageName).To(Equal(imageName))
				})
			})
			It("should return the image name", func() {
				name := "image-name:3"
				c = mockContainerWithImageName(name)
				imageName := c.ImageName()
				Expect(imageName).To(Equal(name))
			})
			It("should assume latest if no tag is supplied", func() {
				name := "image-name"
				c = mockContainerWithImageName(name)
				imageName := c.ImageName()
				Expect(imageName).To(Equal(name + ":latest"))
			})
		})
	})
})

func mockContainerWithImageName(name string) *Container {
	container := mockContainerWithLabels(nil)
	container.containerInfo.Config.Image = name
	return container
}

func mockContainerWithLabels(labels map[string]string) *Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    "container_id",
			Image: "image",
			Name:  "test-containrrr",
		},
		Config: &container.Config{
			Labels: labels,
		},
	}
	return NewContainer(&content, nil)
}
