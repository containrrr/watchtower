package container

import (
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/filters"
	cli "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/sirupsen/logrus"
)

var _ = Describe("the client", func() {
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
	Describe("WarnOnHeadPullFailed", func() {
		containerUnknown := *mockContainerWithImageName("unknown.repo/prefix/imagename:latest")
		containerKnown := *mockContainerWithImageName("docker.io/prefix/imagename:latest")

		When("warn on head failure is set to \"always\"", func() {
			c := newClientNoAPI(false, false, false, false, false, "always")
			It("should always return true", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeTrue())
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeTrue())
			})
		})
		When("warn on head failure is set to \"auto\"", func() {
			c := newClientNoAPI(false, false, false, false, false, "auto")
			It("should always return true", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeFalse())
			})
			It("should", func() {
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeTrue())
			})
		})
		When("warn on head failure is set to \"never\"", func() {
			c := newClientNoAPI(false, false, false, false, false, "never")
			It("should never return true", func() {
				Expect(c.WarnOnHeadPullFailed(containerUnknown)).To(BeFalse())
				Expect(c.WarnOnHeadPullFailed(containerKnown)).To(BeFalse())
			})
		})
	})

	When("listing containers without any filter", func() {
		It("should return all available containers", func() {
			containers, err := client.ListContainers(filters.NoFilter)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(containers) == 2).To(BeTrue())
		})
	})
	When("listing containers with a filter matching nothing", func() {
		It("should return an empty array", func() {
			filter := filters.FilterByNames([]string{"lollercoaster"}, filters.NoFilter)
			containers, err := client.ListContainers(filter)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(containers) == 0).To(BeTrue())
		})
	})
	When("listing containers with a watchtower filter", func() {
		It("should return only the watchtower container", func() {
			containers, err := client.ListContainers(filters.WatchtowerContainersFilter)
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
			containers, err := client.ListContainers(filters.NoFilter)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(containers) > 0).To(BeTrue())
		})
	})
	When(`listing containers with the "include restart" option`, func() {
		It("should return both stopped, restarting and running containers", func() {
			client = dockerClient{
				api:               docker,
				pullImages:        false,
				includeRestarting: true,
			}
			containers, err := client.ListContainers(filters.NoFilter)
			Expect(err).NotTo(HaveOccurred())
			RestartingContainerFound := false
			for _, ContainerRunning := range containers {
				if ContainerRunning.containerInfo.State.Restarting {
					RestartingContainerFound = true
				}
			}
			Expect(RestartingContainerFound).To(BeTrue())
			Expect(RestartingContainerFound).NotTo(BeFalse())
		})
	})
	When(`listing containers without restarting ones`, func() {
		It("should not return restarting containers", func() {
			client = dockerClient{
				api:               docker,
				pullImages:        false,
				includeRestarting: false,
			}
			containers, err := client.ListContainers(filters.NoFilter)
			Expect(err).NotTo(HaveOccurred())
			RestartingContainerFound := false
			for _, ContainerRunning := range containers {
				if ContainerRunning.containerInfo.State.Restarting {
					RestartingContainerFound = true
				}
			}
			Expect(RestartingContainerFound).To(BeFalse())
			Expect(RestartingContainerFound).NotTo(BeTrue())
		})
	})
	Describe(`ExecuteCommand`, func() {
		When(`logging`, func() {
			It("should include container id field", func() {
				// Capture logrus output in buffer
				logbuf := gbytes.NewBuffer()
				origOut := logrus.StandardLogger().Out
				defer logrus.SetOutput(origOut)
				logrus.SetOutput(logbuf)

				_, err := client.ExecuteCommand("ex-cont-id", "exec-cmd", 1)
				Expect(err).NotTo(HaveOccurred())
				// Note: Since Execute requires opening up a raw TCP stream to the daemon for the output, this will fail
				// when using the mock API server. Regardless of the outcome, the log should include the container ID
				Eventually(logbuf).Should(gbytes.Say(`containerID="?ex-cont-id"?`))
			})
		})
	})
})
