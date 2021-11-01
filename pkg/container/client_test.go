package container

import (
	"github.com/containrrr/watchtower/pkg/container/mocks"
	"github.com/containrrr/watchtower/pkg/filters"
	t "github.com/containrrr/watchtower/pkg/types"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	cli "github.com/docker/docker/client"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"

	"net/http"
)

var _ = Describe("the client", func() {
	var docker *cli.Client
	var mockServer *ghttp.Server
	BeforeSuite(func() {
		mockServer = ghttp.NewServer()
		docker, _ = cli.NewClientWithOpts(
			cli.WithHost(mockServer.URL()),
			cli.WithHTTPClient(mockServer.HTTPTestServer.Client()))
	})
	AfterEach(func() {
		mockServer.Reset()
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
	When("listing containers", func() {
		When("no filter is provided", func() {
			It("should return all available containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("watchtower", "running")...)
				client := dockerClient{
					api:        docker,
					pullImages: false,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(2))
			})
		})
		When("a filter matching nothing", func() {
			It("should return an empty array", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("watchtower", "running")...)
				filter := filters.FilterByNames([]string{"lollercoaster"}, filters.NoFilter)
				client := dockerClient{
					api:        docker,
					pullImages: false,
				}
				containers, err := client.ListContainers(filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(BeEmpty())
			})
		})
		When("a watchtower filter is provided", func() {
			It("should return only the watchtower container", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("watchtower", "running")...)
				client := dockerClient{
					api:        docker,
					pullImages: false,
				}
				containers, err := client.ListContainers(filters.WatchtowerContainersFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ConsistOf(withContainerImageName(Equal("containrrr/watchtower:latest"))))
			})
		})
		When(`include stopped is enabled`, func() {
			It("should return both stopped and running containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running", "exited", "created"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("stopped", "watchtower", "running")...)
				client := dockerClient{
					api:            docker,
					pullImages:     false,
					includeStopped: true,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ContainElement(havingRunningState(false)))
			})
		})
		When(`include restarting is enabled`, func() {
			It("should return both restarting and running containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running", "restarting"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("watchtower", "running", "restarting")...)
				client := dockerClient{
					api:               docker,
					pullImages:        false,
					includeRestarting: true,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(ContainElement(havingRestartingState(true)))
			})
		})
		When(`include restarting is disabled`, func() {
			It("should not return restarting containers", func() {
				mockServer.AppendHandlers(mocks.ListContainersHandler("running"))
				mockServer.AppendHandlers(mocks.GetContainerHandlers("watchtower", "running")...)
				client := dockerClient{
					api:               docker,
					pullImages:        false,
					includeRestarting: false,
				}
				containers, err := client.ListContainers(filters.NoFilter)
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).NotTo(ContainElement(havingRestartingState(true)))
			})
		})
	})
	Describe(`ExecuteCommand`, func() {
		When(`logging`, func() {
			It("should include container id field", func() {
				server := ghttp.NewServer()
				apiClient, err := cli.NewClientWithOpts(
					cli.WithHost(server.URL()),
					cli.WithHTTPClient(server.HTTPTestServer.Client()),
				)
				Expect(err).ShouldNot(HaveOccurred())

				client := dockerClient{
					api:        apiClient,
					pullImages: false,
				}

				// Capture logrus output in buffer
				logbuf := gbytes.NewBuffer()
				origOut := logrus.StandardLogger().Out
				defer logrus.SetOutput(origOut)
				logrus.SetOutput(logbuf)

				user := "exec-user"
				containerID := t.ContainerID("ex-cont-id")
				execID := "ex-exec-id"
				cmd := "exec-cmd"

				server.AppendHandlers(
					// API.ContainerExecCreate
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("containers/%v/exec", containerID)),
						ghttp.VerifyJSONRepresenting(types.ExecConfig{
							User:   user,
							Detach: false,
							Tty:    true,
							Cmd: []string{
								"sh",
								"-c",
								cmd,
							},
						}),
						ghttp.RespondWithJSONEncoded(http.StatusOK, types.IDResponse{ID: execID}),
					),
					// API.ContainerExecStart
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", HaveSuffix("exec/%v/start", execID)),
						ghttp.VerifyJSONRepresenting(types.ExecStartCheck{
							Detach: false,
							Tty:    true,
						}),
						ghttp.RespondWith(http.StatusOK, nil),
					),
					// API.ContainerExecInspect
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", HaveSuffix("exec/ex-exec-id/json")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, backend.ExecInspect{
							ID:       execID,
							Running:  false,
							ExitCode: nil,
							ProcessConfig: &backend.ExecProcessConfig{
								Entrypoint: "sh",
								Arguments:  []string{"-c", cmd},
								User:       user,
							},
							ContainerID: string(containerID),
						}),
					),
				)

				_, err = client.ExecuteCommand(containerID, cmd, user, 1)
				Expect(err).NotTo(HaveOccurred())
				// Note: Since Execute requires opening up a raw TCP stream to the daemon for the output, this will fail
				// when using the mock API server. Regardless of the outcome, the log should include the container ID
				Eventually(logbuf).Should(gbytes.Say(`containerID="?ex-cont-id"?`))
			})
		})
	})
})

// Gomega matcher helpers

func withContainerImageName(matcher GomegaMatcher) GomegaMatcher {
	return WithTransform(containerImageName, matcher)
}

func containerImageName(container Container) string {
	return container.ImageName()
}

func havingRestartingState(expected bool) GomegaMatcher {
	return WithTransform(func(container Container) bool {
		return container.containerInfo.State.Restarting
	}, Equal(expected))
}

func havingRunningState(expected bool) GomegaMatcher {
	return WithTransform(func(container Container) bool {
		return container.containerInfo.State.Running
	}, Equal(expected))
}
