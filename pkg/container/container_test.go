package container

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("the container", func() {
	Describe("VerifyConfiguration", func() {
		When("verifying a container with no image info", func() {
			It("should return an error", func() {
				c := mockContainerWithPortBindings()
				c.imageInfo = nil
				err := c.VerifyConfiguration()
				Expect(err).To(Equal(errorNoImageInfo))
			})
		})
		When("verifying a container with no container info", func() {
			It("should return an error", func() {
				c := mockContainerWithPortBindings()
				c.containerInfo = nil
				err := c.VerifyConfiguration()
				Expect(err).To(Equal(errorNoContainerInfo))
			})
		})
		When("verifying a container with no config", func() {
			It("should return an error", func() {
				c := mockContainerWithPortBindings()
				c.containerInfo.Config = nil
				err := c.VerifyConfiguration()
				Expect(err).To(Equal(errorInvalidConfig))
			})
		})
		When("verifying a container with no host config", func() {
			It("should return an error", func() {
				c := mockContainerWithPortBindings()
				c.containerInfo.HostConfig = nil
				err := c.VerifyConfiguration()
				Expect(err).To(Equal(errorInvalidConfig))
			})
		})
		When("verifying a container with no port bindings", func() {
			It("should not return an error", func() {
				c := mockContainerWithPortBindings()
				err := c.VerifyConfiguration()
				Expect(err).ToNot(HaveOccurred())
			})
		})
		When("verifying a container with port bindings, but no exposed ports", func() {
			It("should make the config compatible with updating", func() {
				c := mockContainerWithPortBindings("80/tcp")
				c.containerInfo.Config.ExposedPorts = nil
				Expect(c.VerifyConfiguration()).To(Succeed())

				Expect(c.containerInfo.Config.ExposedPorts).ToNot(BeNil())
				Expect(c.containerInfo.Config.ExposedPorts).To(BeEmpty())
			})
		})
		When("verifying a container with port bindings and exposed ports is non-nil", func() {
			It("should return an error", func() {
				c := mockContainerWithPortBindings("80/tcp")
				c.containerInfo.Config.ExposedPorts = map[nat.Port]struct{}{"80/tcp": {}}
				err := c.VerifyConfiguration()
				Expect(err).ToNot(HaveOccurred())
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

			Expect(id).To(BeEquivalentTo("container_id"))
			Expect(id).NotTo(BeEquivalentTo("wrong-id"))
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

		When("fetching container links", func() {
			When("the depends on label is present", func() {
				It("should fetch depending containers from it", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("postgres"), HaveLen(1)))
				})
				It("should fetch depending containers if there are many", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres,redis",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("postgres"), ContainElement("redis"), HaveLen(2)))
				})
				It("should fetch depending containers if label is blank", func() {
					c = mockContainerWithLabels(map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "",
					})
					links := c.Links()
					Expect(links).To(HaveLen(0))
				})
			})
			When("the depends on label is not present", func() {
				It("should fetch depending containers from host config links", func() {
					c = mockContainerWithLinks([]string{
						"redis:test-containrrr",
						"postgres:test-containrrr",
					})
					links := c.Links()
					Expect(links).To(SatisfyAll(ContainElement("redis"), ContainElement("postgres"), HaveLen(2)))
				})
			})
		})
		
		When("there is a pre or post update timeout", func() {
 				It("should return minute values", func() {
 					c = mockContainerWithLabels(map[string]string{
 						"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout":  "3",
 						"com.centurylinklabs.watchtower.lifecycle.post-update-timeout": "5",
 					})
 					preTimeout := c.PreUpdateTimeout()
 					Expect(preTimeout).To(Equal(3))
 					postTimeout := c.PostUpdateTimeout()
 					Expect(postTimeout).To(Equal(5))
 				})
 			})
		
	})
})

func mockContainerWithPortBindings(portBindingSources ...string) *Container {
	mockContainer := mockContainerWithLabels(nil)
	mockContainer.imageInfo = &types.ImageInspect{}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{},
	}
	for _, pbs := range portBindingSources {
		hostConfig.PortBindings[nat.Port(pbs)] = []nat.PortBinding{}
	}
	mockContainer.containerInfo.HostConfig = hostConfig
	return mockContainer
}

func mockContainerWithImageName(name string) *Container {
	mockContainer := mockContainerWithLabels(nil)
	mockContainer.containerInfo.Config.Image = name
	return mockContainer
}

func mockContainerWithLinks(links []string) *Container {
	content := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    "container_id",
			Image: "image",
			Name:  "test-containrrr",
			HostConfig: &container.HostConfig{
				Links: links,
			},
		},
		Config: &container.Config{
			Labels: map[string]string{},
		},
	}
	return NewContainer(&content, nil)
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

func newClientNoAPI(pullImages, includeStopped, reviveStopped, removeVolumes, includeRestarting bool, warnOnHeadFailed string) Client {
	return dockerClient{
		api:               nil,
		pullImages:        pullImages,
		removeVolumes:     removeVolumes,
		includeStopped:    includeStopped,
		reviveStopped:     reviveStopped,
		includeRestarting: includeRestarting,
		warnOnHeadFailed:  warnOnHeadFailed,
	}
}
