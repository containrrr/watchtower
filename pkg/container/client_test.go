package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerClient "github.com/moby/moby/client"
	gomegaTypes "github.com/onsi/gomega/types"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	methodPost   = "POST"
	methodDelete = "DELETE"
	pingPath     = "/_ping"
)

var _ = ginkgo.Describe("the client", func() {
	var (
		docker     *dockerClient.Client
		mockServer *ghttp.Server
	)

	// Set up a mock Docker server before each test.

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	// Clean up the mock server after each test.
	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	// Test suite for stopping and removing a running container.
	ginkgo.When("removing a running container", func() {
		ginkgo.When("the container still exists after stopping", func() {
			ginkgo.It("should attempt to remove the container", func() {
				// Create a mock mockedContainer in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handlers for stop and remove operations.
				mockServer.AppendHandlers(
					StopContainerHandler(
						cid,
						mockContainer.Found,
					), // Simulate successful stop
					mockContainer.RemoveContainerHandler(
						cid,
						mockContainer.Found,
					), // Simulate successful removal
				)
				// Execute StopAndRemoveContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("the container does not exist after stopping", func() {
			ginkgo.It("should not cause an error", func() {
				// Create a mock container in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handlers for stop and removal.
				mockServer.AppendHandlers(
					StopContainerHandler(
						cid,
						mockContainer.Found,
					), // Simulate successful stop
					mockContainer.RemoveContainerHandler(
						cid,
						mockContainer.Missing,
					), // Removal fails gracefully
				)
				// Execute StopAndRemoveContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("stopping fails with an unexpected error", func() {
			ginkgo.It("should return an error", func() {
				// Create a mock container in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for stop failure.
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							methodPost,
							gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
						),
						ghttp.RespondWith(http.StatusInternalServerError, "server error"),
					),
				)
				// Execute StopContainer and verify the error is propagated.
				err := (&client{log: testLog(), api: docker}).StopContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("failed to stop container: Error response from daemon: server error")))
			})
		})

		ginkgo.When("stopping fails with an unexpected error in StopAndRemoveContainer", func() {
			ginkgo.It("should return an error without attempting removal", func() {
				// Create a mock container in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for stop failure (no remove handler needed).
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							methodPost,
							gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
						),
						ghttp.RespondWith(http.StatusInternalServerError, "stop error"),
					),
				)
				// Execute StopAndRemoveContainer and verify the stop error is propagated.
				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("failed to stop container: Error response from daemon: stop error")))
			})
		})

		ginkgo.When("removal fails with an unexpected error", func() {
			ginkgo.It("should return an error", func() {
				// Create a mock container in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handlers for stop and removal failure.
				mockServer.AppendHandlers(
					StopContainerHandler(cid, mockContainer.Found), // Simulate successful stop
					ghttp.CombineHandlers( // Removal fails
						ghttp.VerifyRequest(methodDelete, gomega.HaveSuffix(cid)),
						ghttp.RespondWith(http.StatusInternalServerError, "server error"),
					),
				)
				// Execute StopAndRemoveContainer and verify the removal error is propagated.
				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("failed to remove container: Error response from daemon: server error")))
			})
		})

		ginkgo.When("removing a stopped container", func() {
			ginkgo.It("should only call remove, not stop", func() {
				// Create a mock container in stopped state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: false}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for removal only.
				mockServer.AppendHandlers(
					mockContainer.RemoveContainerHandler(
						cid,
						mockContainer.Found,
					), // Simulate successful removal
				)
				// Execute StopAndRemoveContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("stopping a container with AutoRemove enabled", func() {
			ginkgo.It("should skip removal after stopping a running container", func() {
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
					WithAutoRemove(true),
				)
				cid := mockedContainer.ContainerInfo().ID
				mockServer.AppendHandlers(
					StopContainerHandler(cid, mockContainer.Found),
				)

				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				// API version ping + stop. No DELETE. Docker AutoRemove handles cleanup after stop.
				gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(2))
			})

			ginkgo.It("should remove a non-running AutoRemove container explicitly", func() {
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: false, Status: "created"}),
					WithAutoRemove(true),
				)
				cid := mockedContainer.ContainerInfo().ID
				mockServer.AppendHandlers(
					mockContainer.RemoveContainerHandler(cid, mockContainer.Found),
				)

				err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				// API version ping + DELETE. AutoRemove does not apply to never-started containers.
				gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(2))
			})
		})
	})

	// Test suite for stopping containers.
	ginkgo.When("stopping a container", func() {
		ginkgo.When("the container is running", func() {
			ginkgo.It("should stop the container successfully", func() {
				// Create a mock container in running state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for stop operation.
				mockServer.AppendHandlers(
					StopContainerHandler(
						cid,
						mockContainer.Found,
					),
				)
				// Execute StopContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).StopContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("the container is already stopped", func() {
			ginkgo.It("should not attempt to stop and return no error", func() {
				// Create a mock container in stopped state.
				mockedContainer := MockContainer(
					WithContainerState(dockerContainer.State{Running: false}),
				)
				// Execute StopContainer and verify no error occurs (no API calls expected).
				err := (&client{log: testLog(), api: docker}).StopContainer(context.Background(), mockedContainer, time.Second)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				// Verify no requests were made to the mock server.
				gomega.Expect(mockServer.ReceivedRequests()).To(gomega.BeEmpty())
			})
		})
	})

	// Test suite for removing containers.
	ginkgo.When("removing a container", func() {
		ginkgo.When("the container exists", func() {
			ginkgo.It("should remove the container successfully", func() {
				// Create a mock container.
				mockedContainer := MockContainer()
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for removal.
				mockServer.AppendHandlers(
					mockContainer.RemoveContainerHandler(
						cid,
						mockContainer.Found,
					),
				)
				// Execute RemoveContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).RemoveContainer(context.Background(), mockedContainer)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("the container does not exist", func() {
			ginkgo.It("should not return an error", func() {
				// Create a mock container.
				mockedContainer := MockContainer()
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for removal failure (container not found).
				mockServer.AppendHandlers(
					mockContainer.RemoveContainerHandler(
						cid,
						mockContainer.Missing,
					),
				)
				// Execute RemoveContainer and verify no error occurs.
				err := (&client{log: testLog(), api: docker}).RemoveContainer(context.Background(), mockedContainer)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		})

		ginkgo.When("removal fails with an unexpected error", func() {
			ginkgo.It("should return an error", func() {
				// Create a mock container.
				mockedContainer := MockContainer()
				cid := mockedContainer.ContainerInfo().ID
				// Set up mock server handler for removal failure.
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(methodDelete, gomega.HaveSuffix(cid)),
						ghttp.RespondWith(http.StatusInternalServerError, "server error"),
					),
				)
				// Execute RemoveContainer and verify the error is propagated.
				err := (&client{log: testLog(), api: docker}).RemoveContainer(context.Background(), mockedContainer)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("failed to remove container")))
			})
		})
	})

	// Test suite for listing containers with various filters and options.
	ginkgo.When("listing containers", func() {
		ginkgo.When("no filter is provided", func() {
			ginkgo.It("should return all available containers", func() {
				// Set up mock server to return running containers.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Execute ListContainers and verify results.
				containers, err := client.ListContainers(
					context.Background(),
					filters.NoFilter,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(2))
			})
		})

		ginkgo.When("a filter matching nothing", func() {
			ginkgo.It("should return an empty array", func() {
				// Set up mock server to return running containers.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				filter := filters.FilterByNames(testLog(), []string{"lollercoaster"}, filters.NoFilter)
				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Execute ListContainers and verify empty result.
				containers, err := client.ListContainers(context.Background(), filter)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.BeEmpty())
			})
		})

		ginkgo.When("a watchtower filter is provided", func() {
			ginkgo.It("should return only the watchtower container", func() {
				// Set up mock server to return running containers.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Execute ListContainers with Watchtower filter and verify result.
				containers, err := client.ListContainers(
					context.Background(),
					filters.WatchtowerContainersFilter,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).
					To(gomega.ConsistOf(withContainerImageName(gomega.Equal("nickfedor/watchtower:latest"))))
			})
		})

		ginkgo.When(`include stopped is enabled`, func() {
			ginkgo.It("should return both stopped and running containers", func() {
				// Set up mock server to return running, stopped, and created containers.
				mockServer.AppendHandlers(
					mockContainer.ListContainersHandler("running", "exited", "created"),
				)
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Stopped,
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:            docker,
					IncludeStopped: true,
				}
				// Execute ListContainers and verify stopped containers are included.
				containers, err := client.ListContainers(
					context.Background(),
					filters.NoFilter,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.ContainElement(havingRunningState(false)))
			})
		})

		ginkgo.When(`include restarting is enabled`, func() {
			ginkgo.It("should return both restarting and running containers", func() {
				// Set up mock server to return running and restarting containers.
				mockServer.AppendHandlers(
					mockContainer.ListContainersHandler("running", "restarting"),
				)
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
						&mockContainer.Restarting,
					)...,
				)

				// A Docker marker resolves the runtime without an API call.
				fsys := afero.NewMemMapFs()
				gomega.Expect(afero.WriteFile(fsys, "/.dockerenv", nil, 0o600)).To(gomega.Succeed())

				client := &client{
					api:               docker,
					IncludeRestarting: true,
					Fs:                fsys,
				}
				// Execute ListContainers and verify restarting containers are included.
				containers, err := client.ListContainers(
					context.Background(),
					filters.NoFilter,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.ContainElement(havingRestartingState(true)))
			})
		})

		ginkgo.When(`include restarting is disabled`, func() {
			ginkgo.It("should not return restarting containers", func() {
				// Set up mock server to return running containers only.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:               docker,
					IncludeRestarting: false,
				}
				// Execute ListContainers and verify no restarting containers are included.
				containers, err := client.ListContainers(
					context.Background(),
					filters.NoFilter,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).NotTo(gomega.ContainElement(havingRestartingState(true)))
			})
		})

		ginkgo.When("multiple filters are provided", func() {
			ginkgo.It("should combine filters with logical AND", func() {
				// Set up mock server to return running containers.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Apply two filters: one for name "portainer" and one that always passes
				nameFilter := filters.FilterByNames(testLog(), []string{"portainer"}, filters.NoFilter)
				containers, err := client.ListContainers(context.Background(), nameFilter, filters.NoFilter)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Should return only the "portainer" container
				gomega.Expect(containers).To(gomega.HaveLen(1))
				gomega.Expect(containers[0].Name()).To(gomega.Equal("portainer"))
			})

			ginkgo.It("should return empty when filters are mutually exclusive", func() {
				// Set up mock server to return running containers.
				mockServer.AppendHandlers(mockContainer.ListContainersHandler("running"))
				mockServer.AppendHandlers(
					mockContainer.GetContainerHandlers(
						&mockContainer.Watchtower,
						&mockContainer.Running,
					)...,
				)

				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Apply two mutually exclusive name filters
				portainerFilter := filters.FilterByNames(testLog(), []string{"portainer"}, filters.NoFilter)
				watchtowerFilter := filters.FilterByNames(testLog(),
					[]string{"watchtower-running"},
					filters.NoFilter,
				)
				containers, err := client.ListContainers(context.Background(), portainerFilter, watchtowerFilter)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Should return empty since no container can be both "portainer" and "watchtower-running" named
				gomega.Expect(containers).To(gomega.BeEmpty())
			})
		})

		ginkgo.When(`the image of a container cannot be inspected`, func() {
			ginkgo.It("should warn, naming the container and its image", func() {
				// GetContainer is only reached for containers already being acted
				// upon, so missing image metadata is worth a warning here even
				// though GetSourceContainer logs it at debug level.
				mockServer.AppendHandlers(missingImageHandlers(testContainerID)...)

				log, logBuf := captureLog(zerolog.WarnLevel)
				client := &client{
					api:           docker,
					log:           log,
					ClientOptions: ClientOptions{},
				}

				container, err := client.GetContainer(
					context.Background(),
					types.ContainerID(testContainerID),
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(container.HasImageInfo()).To(gomega.BeFalse())

				logged := string(logBuf.Contents())
				gomega.Expect(logged).To(gomega.ContainSubstring("Failed to retrieve image info"))
				gomega.Expect(logged).To(gomega.ContainSubstring("test-container"))
				gomega.Expect(logged).To(gomega.ContainSubstring("test-image:latest"))
			})
		})

		ginkgo.When(`a container uses container network mode`, func() {
			ginkgo.When(`the network container can be resolved`, func() {
				ginkgo.It("should return the container name instead of the ID", func() {
					// Set up mock server for a container with network mode.
					consumerContainerRef := mockContainer.NetConsumerOK
					mockServer.AppendHandlers(
						mockContainer.GetContainerHandlers(&consumerContainerRef)...,
					)

					client := &client{
						api:           docker,
						ClientOptions: ClientOptions{},
					}
					// Execute GetContainer and verify network mode resolution.
					container, err := client.GetContainer(context.Background(), consumerContainerRef.ContainerID())
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					gomega.Expect(networkMode.ConnectedContainer()).
						To(gomega.Equal(mockContainer.NetSupplierContainerName))
				})
			})

			ginkgo.When(`the network container cannot be resolved`, func() {
				ginkgo.It("should still return the container ID", func() {
					// Set up mock server for a container with invalid network supplier.
					consumerContainerRef := mockContainer.NetConsumerInvalidSupplier
					mockServer.AppendHandlers(
						mockContainer.GetContainerHandlers(&consumerContainerRef)...,
					)

					client := &client{
						api:           docker,
						ClientOptions: ClientOptions{},
					}
					// Execute GetContainer and verify fallback to container ID.
					container, err := client.GetContainer(context.Background(), consumerContainerRef.ContainerID())
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					gomega.Expect(networkMode.ConnectedContainer()).
						To(gomega.Equal(mockContainer.NetSupplierNotFoundID))
				})
			})

			// Test suite for waiting for container health.
			ginkgo.Describe("WaitForContainerHealthy", func() {
				ginkgo.When("container has no health check", func() {
					ginkgo.It("should return immediately without error", func() {
						mockedContainer := MockContainer()
						cid := mockedContainer.ContainerInfo().ID
						// Mock inspect response with no health check
						mockServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest(
									"GET",
									gomega.MatchRegexp(
										fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, cid),
									),
								),
								ghttp.RespondWithJSONEncoded(
									http.StatusOK,
									dockerContainer.InspectResponse{
										ID:     cid,
										State:  &dockerContainer.State{Status: "running"},
										Config: &dockerContainer.Config{},
									},
								),
							),
						)

						client := &client{log: testLog(), api: docker}
						err := client.WaitForContainerHealthy(
							context.Background(),
							types.ContainerID(cid),
							5*time.Second,
						)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
					})
				})

				ginkgo.When("container becomes healthy", func() {
					ginkgo.It("should return without error", func() {
						mockedContainer := MockContainer()
						cid := mockedContainer.ContainerInfo().ID
						// Mock inspect responses: first two starting, then healthy
						mockServer.AppendHandlers(
							inspectHandler(cid, dockerContainer.HealthStatus("starting")),
							inspectHandler(cid, dockerContainer.HealthStatus("starting")),
							inspectHandler(cid, dockerContainer.HealthStatus("healthy")),
						)

						client := &client{log: testLog(), api: docker}
						err := client.WaitForContainerHealthy(
							context.Background(),
							types.ContainerID(cid),
							5*time.Second,
						)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
					})
				})

				ginkgo.When("container becomes unhealthy", func() {
					ginkgo.It("should return an error", func() {
						mockedContainer := MockContainer()
						cid := mockedContainer.ContainerInfo().ID
						// Mock inspect response with unhealthy status
						mockServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest(
									"GET",
									gomega.MatchRegexp(
										fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, cid),
									),
								),
								ghttp.RespondWithJSONEncoded(
									http.StatusOK,
									dockerContainer.InspectResponse{
										ID: cid,
										State: &dockerContainer.State{
											Status: "running",
											Health: &dockerContainer.Health{
												Status: "unhealthy",
											},
										},
										Config: &dockerContainer.Config{},
									},
								),
							),
						)

						client := &client{log: testLog(), api: docker}
						err := client.WaitForContainerHealthy(
							context.Background(),
							types.ContainerID(cid),
							5*time.Second,
						)
						gomega.Expect(err).To(gomega.HaveOccurred())
						gomega.Expect(err.Error()).
							To(gomega.ContainSubstring("health check failed"))
					})
				})
			})
		})

		ginkgo.Describe("getRuntime", func() {
			ginkgo.When("CPUCopyMode is auto", func() {
				ginkgo.It("should detect Podman via marker file", func() {
					memFs := afero.NewMemMapFs()
					afero.WriteFile(memFs, "/run/.containerenv", []byte{}, 0o644)
					testClient := &client{
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeTrue())
				})

				ginkgo.It("should detect Podman via CONTAINER environment variable", func() {
					memFs := afero.NewMemMapFs()

					restore := withEnvVars(map[string]string{"CONTAINER": "podman"})
					defer restore()

					testClient := &client{
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeTrue())
				})

				ginkgo.It("should detect Podman via API Name field", func() {
					memFs := afero.NewMemMapFs()

					mockServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
								"Name": "podman",
							}),
						),
					)

					testClient := &client{
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeTrue())
				})

				ginkgo.It("should detect Podman via API ServerVersion field", func() {
					memFs := afero.NewMemMapFs()

					mockServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
								"ServerVersion": "podman/4.0.0",
							}),
						),
					)

					testClient := &client{
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeTrue())
				})

				ginkgo.It("should detect Docker via marker file", func() {
					memFs := afero.NewMemMapFs()
					afero.WriteFile(memFs, "/.dockerenv", []byte{}, 0o644)
					mockServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
								"Name": "docker",
							}),
						),
					)

					testClient := &client{
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeFalse())
				})

				ginkgo.It("should fall back to Docker when detection fails", func() {
					memFs := afero.NewMemMapFs()

					// Podman sets the lowercase variable, so clear both forms.
					restore := withEnvVars(map[string]string{"container": "", "CONTAINER": ""})
					defer restore()

					mockServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
							ghttp.RespondWith(http.StatusInternalServerError, "server error"),
						),
					)

					log, logbuf := captureLog(zerolog.DebugLevel)

					testClient := &client{
						log:         log,
						api:         docker,
						CPUCopyMode: CPUCopyModeAuto,
						Fs:          memFs,
					}
					result := testClient.getRuntime(context.Background())
					gomega.Expect(result).To(gomega.BeFalse())
					gomega.Eventually(logbuf).
						Should(gbytes.Say("Failed to detect container runtime, falling back to Docker"))
				})
			})
		})
	})

	// Test suite for detectRuntimeByMarker helper function.
	ginkgo.Describe("detectRuntimeByMarker", func() {
		ginkgo.It("should return RuntimePodman when Podman marker file exists", func() {
			memFs := afero.NewMemMapFs()
			afero.WriteFile(memFs, "/run/.containerenv", []byte{}, 0o644)
			testClient := &client{
				log: testLog(),
				Fs:  memFs,
			}
			result, err := testClient.detectRuntimeByMarker()
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(RuntimePodman))
		})

		ginkgo.It("should return RuntimeDocker when Docker marker file exists", func() {
			memFs := afero.NewMemMapFs()
			afero.WriteFile(memFs, "/.dockerenv", []byte{}, 0o644)
			testClient := &client{
				Fs: memFs,
			}
			result, err := testClient.detectRuntimeByMarker()
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(RuntimeDocker))
		})

		ginkgo.It("should return RuntimeUnknown when neither marker file exists", func() {
			memFs := afero.NewMemMapFs()
			testClient := &client{
				Fs: memFs,
			}
			result, err := testClient.detectRuntimeByMarker()
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(RuntimeUnknown))
		})

		ginkgo.It("should return RuntimePodman when Podman marker exists alongside Docker marker", func() {
			memFs := afero.NewMemMapFs()
			afero.WriteFile(memFs, "/run/.containerenv", []byte{}, 0o644)
			afero.WriteFile(memFs, "/.dockerenv", []byte{}, 0o644)
			testClient := &client{
				Fs: memFs,
			}
			result, err := testClient.detectRuntimeByMarker()
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(RuntimePodman))
		})
	})

	ginkgo.Describe("detectRuntimeByEnv", func() {
		ginkgo.It("should return true when CONTAINER is podman", func() {
			restore := withEnvVars(map[string]string{"CONTAINER": "podman"})
			defer restore()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when CONTAINER is oci", func() {
			restore := withEnvVars(map[string]string{"CONTAINER": "oci"})
			defer restore()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return false when CONTAINER is docker", func() {
			restore := withEnvVars(map[string]string{"CONTAINER": "docker", "container": ""})
			defer restore()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when CONTAINER is other", func() {
			restore := withEnvVars(map[string]string{"CONTAINER": "other", "container": ""})
			defer restore()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when CONTAINER is not set", func() {
			// Podman sets the lowercase variable, so clear it as well.
			restoreLower := withEnvVars(map[string]string{"container": ""})
			defer restoreLower()

			// Save original state and ensure CONTAINER is not set
			orig, exists := os.LookupEnv("CONTAINER")

			os.Unsetenv("CONTAINER")

			defer func() {
				if exists {
					os.Setenv("CONTAINER", orig)
				}
			}()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when CONTAINER is empty", func() {
			restore := withEnvVars(map[string]string{"CONTAINER": "", "container": ""})
			defer restore()

			testClient := &client{log: testLog()}
			result := testClient.detectRuntimeByEnv()
			gomega.Expect(result).To(gomega.BeFalse())
		})
	})

	// Test suite for detectRuntimeByAPI helper function.
	ginkgo.Describe("detectRuntimeByAPI", func() {
		ginkgo.It("should return true when API returns Name podman", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/info$")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"Name": "podman",
					}),
				),
			)

			testClient := &client{log: testLog(), api: docker}
			result, err := testClient.detectRuntimeByAPI(context.Background())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return true when API returns ServerVersion containing podman", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/info$")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"ServerVersion": "podman/4.0.0",
					}),
				),
			)

			testClient := &client{log: testLog(), api: docker}
			result, err := testClient.detectRuntimeByAPI(context.Background())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeTrue())
		})

		ginkgo.It("should return false when API returns neither podman Name nor ServerVersion", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/info$")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"Name":            "docker",
						"ServerVersion":   "20.10.0",
						"OperatingSystem": "linux",
					}),
				),
			)

			testClient := &client{log: testLog(), api: docker}
			result, err := testClient.detectRuntimeByAPI(context.Background())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return error when API call fails", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/info$")),
					ghttp.RespondWith(http.StatusInternalServerError, "server error"),
				),
			)

			testClient := &client{log: testLog(), api: docker}
			result, err := testClient.detectRuntimeByAPI(context.Background())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when API returns empty response", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/info$")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{}),
				),
			)

			testClient := &client{log: testLog(), api: docker}
			result, err := testClient.detectRuntimeByAPI(context.Background())
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.BeFalse())
		})
	})

	// Test suite for executing commands in a container.
	ginkgo.Describe("ExecuteCommand", func() {
		ginkgo.When("logging", func() {
			ginkgo.It("should include container id field", func() {
				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Capture log output in buffer.
				log, logbuf := captureLog(zerolog.DebugLevel)
				client.log = log

				containerID := types.ContainerID("ex-cont-id")
				execID := "ex-exec-id"
				cmd := "exec-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 0, false, false)

				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify log output includes container id.
				_, err = client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Eventually(logbuf).Should(gbytes.Say("container_id=ex-cont-id"))
			})

			ginkgo.It("should skip updates when command exits with code 75", func() {
				client := &client{
					log:           testLog(),
					api:           docker,
					ClientOptions: ClientOptions{},
				}
				// Capture log output in buffer.
				log, logbuf := captureLog(zerolog.DebugLevel)
				client.log = log

				containerID := types.ContainerID("ex-cont-id")
				execID := "ex-exec-id"
				cmd := "exec-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 75, false, false)

				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify skip update is true
				skipUpdate, err := client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(skipUpdate).To(gomega.BeTrue())
				gomega.Eventually(logbuf).Should(gbytes.Say("container_id=ex-cont-id"))
			})
		})
	})

	// Test suite for ExecuteCommand results.
	ginkgo.Describe("ExecuteCommand results", func() {
		ginkgo.When("command exits with code 0", func() {
			ginkgo.It("should return false and nil error", func() {
				client := &client{
					log:           testLog(),
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("ex-cont-id")
				execID := "success-exec-id"
				cmd := "success-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 0, false, false)

				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify skipUpdate is false and no error
				skipUpdate, err := client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(skipUpdate).To(gomega.BeFalse())
			})
		})

		ginkgo.When("command exits with non-zero code", func() {
			ginkgo.It("should return error", func() {
				client := &client{
					log:           testLog(),
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("ex-cont-id")
				execID := "failure-exec-id"
				cmd := "failure-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 1, false, false)
				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify error is returned
				_, err = client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("command execution failed"))
			})
		})

		ginkgo.When("ContainerExecCreate fails", func() {
			ginkgo.It("should return error containing 'failed to create exec'", func() {
				client := &client{
					log:           testLog(),
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("ex-cont-id")
				cmd := "create-fail-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), "", cmd, -1, -1, 0, true, false)
				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify error is returned
				_, err = client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to create exec"))
			})
		})

		ginkgo.When("ContainerExecStart fails", func() {
			ginkgo.It("should return error containing 'failed to start exec'", func() {
				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("ex-cont-id")
				execID := "start-fail-exec-id"
				cmd := "start-fail-cmd"
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 0, false, true)
				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command and verify error is returned
				_, err = client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to start exec"))
			})
		})

		ginkgo.When("uid and gid parameters are provided", func() {
			ginkgo.It("should pass UID:GID to User field in ExecOptions", func() {
				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("ex-cont-id")
				execID := "uid-gid-exec-id"
				cmd := "uid-gid-cmd"
				uid := 1000
				gid := 1000
				// Set up mock server handlers for ExecuteCommand.
				setupExecMockHandlers(
					mockServer,
					string(containerID),
					execID,
					cmd,
					uid,
					gid,
					0,
					false,
					false,
				)
				// Get the container first
				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Execute command with uid/gid and verify no error
				skipUpdate, err := client.ExecuteCommand(context.Background(), container, cmd, 1, uid, gid)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(skipUpdate).To(gomega.BeFalse())
			})
		})
		ginkgo.When("ExecStart is called", func() {
			ginkgo.It("should use Detach true to prevent blocking on command execution", func() {
				client := &client{
					api:           docker,
					ClientOptions: ClientOptions{},
				}

				containerID := types.ContainerID("detach-test-cont-id")
				execID := "detach-test-exec-id"
				cmd := "detach-test-cmd"

				// Set up standard handlers
				setupExecMockHandlers(mockServer, string(containerID), execID, cmd, -1, -1, 0, false, false)

				container, err := client.GetContainer(context.Background(), containerID)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				skipUpdate, err := client.ExecuteCommand(context.Background(), container, cmd, 1, 0, 0)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(skipUpdate).To(gomega.BeFalse())

				// Verify the ExecStart request contained Detach: true
				// by inspecting the last request body received by the mock server.
				// The setupExecMockHandlers already verifies this via VerifyJSONRepresenting,
				// but we add this test to explicitly document the requirement:
				// ExecStart must use Detach: true so that the daemon does not block
				// until the command finishes. Without Detach: true, ExecStart blocks,
				// and a subsequent ExecAttach receives "exec command is already running"
				// which causes exit code 126 and aborts the update.
			})
		})
	})

	// Test suite for captureExecOutput.
	ginkgo.Describe("captureExecOutput", func() {
		ginkgo.It("should return error when attach fails", func() {
			client := &client{log: testLog(), api: docker}
			ctx := context.Background()
			_, err := client.captureExecOutput(ctx, "exec-id")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to attach"))
		})

		ginkgo.It("should handle context cancellation", func() {
			client := &client{log: testLog(), api: docker}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := client.captureExecOutput(ctx, "exec-id")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should truncate output that exceeds the size cap", func() {
			mockServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", gomega.MatchRegexp(`^/v[0-9.]+/exec/exec-id/start$`)),
				func(writer http.ResponseWriter, req *http.Request) {
					_, _ = io.Copy(io.Discard, req.Body)

					hijacker, ok := writer.(http.Hijacker)
					gomega.Expect(ok).To(gomega.BeTrue())

					conn, bufWriter, err := hijacker.Hijack()
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					defer conn.Close()

					_, _ = bufWriter.WriteString("HTTP/1.1 101 UPGRADED\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
					_, _ = bufWriter.WriteString(strings.Repeat("x", maxExecOutputSize+4096))
					_ = bufWriter.Flush()

					if tcpConn, ok := conn.(*net.TCPConn); ok {
						_ = tcpConn.CloseWrite()
					}
				},
			))

			// ExecAttach hijacks the TCP connection. The suite client uses an http://
			// host, which the Docker dialer cannot hijack. Use tcp:// against the same
			// ghttp server.
			serverURL, err := url.Parse(mockServer.URL())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			hijackClient, err := dockerClient.New(
				dockerClient.WithHost("tcp://"+serverURL.Host),
				dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			client := &client{log: testLog(), api: hijackClient}
			out, err := client.captureExecOutput(context.Background(), "exec-id")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(out).To(gomega.HaveLen(maxExecOutputSize))
		})
	})

	// Test suite for handling 404 responses when listing containers.
	ginkgo.When("listing containers with 404 response", func() {
		ginkgo.It("should return empty list and log warning", func() {
			// Capture log output.
			log, logbuf := captureLog(zerolog.WarnLevel)

			// Set up mock server to return 404 for /containers/json.
			mockServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest(
					"GET",
					gomega.MatchRegexp(`^/v[0-9.]+/containers/json$`),
				),
				ghttp.RespondWith(http.StatusNotFound, "page not found"),
			))

			// Create client instance.
			client := &client{
				log: log, api: docker, ClientOptions: ClientOptions{},
			}
			// Execute ListContainers and verify empty result with warning log.
			containers, err := client.ListContainers(
				context.Background(),
				filters.NoFilter,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeEmpty())
			gomega.Eventually(logbuf).
				Should(gbytes.Say("Docker API returned 404 for container list"))
		})
	})

	// Test suite for listing containers with 500 server error.
	ginkgo.When("listing containers with 500 server error", func() {
		ginkgo.It("should return error", func() {
			// Set up mock server to return 500 for /containers/json.
			mockServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/containers/json$")),
				ghttp.RespondWith(http.StatusInternalServerError, "server error"),
			))

			// Create client instance.
			client := &client{
				log: testLog(), api: docker, ClientOptions: ClientOptions{},
			}
			// Execute ListContainers and verify error is returned.
			containers, err := client.ListContainers(
				context.Background(),
				filters.NoFilter,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeNil())
		})
	})

	// Test suite for listing containers with 401 unauthorized error.
	ginkgo.When("listing containers with 401 unauthorized error", func() {
		ginkgo.It("should return error", func() {
			// Set up mock server to return 401 for /containers/json.
			mockServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/containers/json$")),
				ghttp.RespondWith(http.StatusUnauthorized, "unauthorized"),
			))

			// Create client instance.
			client := &client{
				log: testLog(), api: docker, ClientOptions: ClientOptions{},
			}
			// Execute ListContainers and verify error is returned.
			containers, err := client.ListContainers(
				context.Background(),
				filters.NoFilter,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeNil())
		})
	})

	// Test suite for listing containers with container inspect 500 error.
	ginkgo.When("listing containers with container inspect 500 error", func() {
		ginkgo.It("should return error", func() {
			// Set up mock server to return containers for list, then 500 for inspect.
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("^/v[0-9.]+/containers/json$")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []dockerContainer.Summary{
						{ID: "container1", Names: []string{"/test1"}},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/containers/container1/json$"),
					),
					ghttp.RespondWith(http.StatusInternalServerError, "inspect error"),
				),
			)

			// Create client instance.
			client := &client{api: docker, ClientOptions: ClientOptions{}}
			// Execute ListContainers and verify error is returned.
			containers, err := client.ListContainers(
				context.Background(),
				filters.NoFilter,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeNil())
		})
	})

	// Test suite for listing all containers with ghost container handling.
	ginkgo.Describe("ListContainers", func() {
		ginkgo.When("containers disappear during enumeration", func() {
			ginkgo.It("should gracefully handle ghost containers and continue processing", func() {
				// Create mock containers: two valid ones and one that will disappear
				validContainer1 := MockContainer()
				validContainer1ID := validContainer1.ContainerInfo().ID
				validContainer2 := MockContainer()
				validContainer2ID := validContainer2.ContainerInfo().ID
				ghostContainerID := "ghost-container-id"

				// Set up mock server handlers
				mockServer.AppendHandlers(
					// Handler for ContainerList - returns all three containers
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.MatchRegexp(`^/v[0-9.]+/containers/json$`),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []dockerContainer.Summary{
							{ID: validContainer1ID, Names: []string{"/valid-container-1"}},
							{ID: ghostContainerID, Names: []string{"/ghost-container"}},
							{ID: validContainer2ID, Names: []string{"/valid-container-2"}},
						}),
					),
					// Handler for first valid container inspect
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.MatchRegexp(
								fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, validContainer1ID),
							),
						),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							dockerContainer.InspectResponse{
								ID:    validContainer1ID,
								Name:  "/valid-container-1",
								Image: "test-image-1:latest",
								State: &dockerContainer.State{
									Status: "running",
								},
								HostConfig: &dockerContainer.HostConfig{},
								Config: &dockerContainer.Config{
									Image: "test-image-1:latest",
								},
							},
						),
					),
					// Handler for image inspect for first container
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/images/.*json$`)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID: "image-id-1",
						}),
					),
					// Handler for ghost container inspect - returns "No such container" error
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.MatchRegexp(
								fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, ghostContainerID),
							),
						),
						ghttp.RespondWith(
							http.StatusNotFound,
							`{"message":"No such container: `+ghostContainerID+`"}`,
						),
					),
					// Handler for second valid container inspect
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.MatchRegexp(
								fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, validContainer2ID),
							),
						),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							dockerContainer.InspectResponse{
								ID:    validContainer2ID,
								Name:  "/valid-container-2",
								Image: "test-image-2:latest",
								State: &dockerContainer.State{
									Status: "running",
								},
								HostConfig: &dockerContainer.HostConfig{},
								Config: &dockerContainer.Config{
									Image: "test-image-2:latest",
								},
							},
						),
					),
					// Handler for image inspect for second container
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/images/.*json$`)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID: "image-id-2",
						}),
					),
				)

				// Execute ListContainers
				log, logbuf := captureLog(zerolog.DebugLevel)

				client := &client{
					log: log, api: docker, ClientOptions: ClientOptions{},
				}
				containers, err := client.ListContainers(context.Background())

				// Verify no error is returned and only valid containers are included
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(containers).To(gomega.HaveLen(2))

				// Verify the ghost container is not in the result
				containerIDs := make([]string, len(containers))
				for i, c := range containers {
					containerIDs[i] = string(c.ID())
				}

				gomega.Expect(containerIDs).To(gomega.ContainElement(validContainer1ID))
				gomega.Expect(containerIDs).To(gomega.ContainElement(validContainer2ID))
				gomega.Expect(containerIDs).NotTo(gomega.ContainElement(ghostContainerID))
				gomega.Eventually(logbuf).Should(gbytes.Say(ghostContainerID))
			})
		})
	})

	ginkgo.Describe("GetImageDiskUsage", func() {
		ginkgo.It("maps image usage from /system/df", func() {
			mockServer.Reset()

			api, err := dockerClient.New(
				dockerClient.WithHost(mockServer.URL()),
				dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
				dockerClient.WithAPIVersion("1.52"),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(`^/v[0-9.]+/system/df$`),
						"type=image",
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"ImageUsage": map[string]any{
							"TotalSize":   4096,
							"Reclaimable": 1024,
							"TotalCount":  2,
							"ActiveCount": 1,
						},
					}),
				),
			)

			usage, usageErr := (&client{log: testLog(), api: api}).GetImageDiskUsage(
				context.Background(),
			)
			gomega.Expect(usageErr).NotTo(gomega.HaveOccurred())
			gomega.Expect(usage.TotalSize).To(gomega.Equal(int64(4096)))
			gomega.Expect(usage.Reclaimable).To(gomega.Equal(int64(1024)))
			gomega.Expect(usage.TotalCount).To(gomega.Equal(int64(2)))
			gomega.Expect(usage.ActiveCount).To(gomega.Equal(int64(1)))
		})

		ginkgo.It("wraps daemon errors", func() {
			mockServer.Reset()

			api, err := dockerClient.New(
				dockerClient.WithHost(mockServer.URL()),
				dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
				dockerClient.WithAPIVersion("1.52"),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(`^/v[0-9.]+/system/df$`),
						"type=image",
					),
					ghttp.RespondWith(http.StatusInternalServerError, "df failed"),
				),
			)

			_, usageErr := (&client{log: testLog(), api: api}).GetImageDiskUsage(
				context.Background(),
			)
			gomega.Expect(usageErr).To(gomega.HaveOccurred())
			gomega.Expect(usageErr.Error()).To(gomega.ContainSubstring("failed to get image disk usage"))
		})
	})

	ginkgo.Describe("TLS client methods", func() {
		var (
			tlsServer  *ghttp.Server
			testClient Client
		)

		ginkgo.BeforeEach(func() {
			tlsServer = ghttp.NewTLSServer()
			docker, _ := dockerClient.New(
				dockerClient.WithHost(tlsServer.URL()),
				dockerClient.WithHTTPClient(tlsServer.HTTPTestServer.Client()),
			)
			testClient = &client{log: testLog(), api: docker}
			gomega.Expect(testClient).NotTo(gomega.BeNil())
			tlsServer.AppendHandlers(APIVersionPingHandler())
		})

		ginkgo.AfterEach(func() {
			tlsServer.Close()
		})

		ginkgo.It("GetVersion returns correct API version with TLS client", func() {
			version := testClient.GetVersion()
			gomega.Expect(version).To(gomega.MatchRegexp(`^\d+\.\d+$`))
		})

		ginkgo.It("GetInfo successfully retrieves system information over TLS", func() {
			tlsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"Name":            "docker-server",
						"ServerVersion":   "24.0.0",
						"OSType":          "linux",
						"OperatingSystem": "Ubuntu 20.04",
						"Driver":          "overlay2",
					}),
				),
			)

			info, err := testClient.GetInfo(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(info.Name).To(gomega.Equal("docker-server"))
		})

		ginkgo.It("GetInfo handles TLS connection failures gracefully", func() {
			// Create a non-TLS server to simulate TLS failure
			httpServer := ghttp.NewServer()
			defer httpServer.Close()

			httpServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusOK, "OK"),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
					ghttp.RespondWith(http.StatusInternalServerError, "TLS connection failed"),
				),
			)
			// Override DOCKER_HOST to point to HTTP server while TLS is required
			restore := withEnvVars(map[string]string{
				"DOCKER_TLS_VERIFY": "1",
				"DOCKER_HOST":       httpServer.URL(),
			})
			defer restore()
			// Create client that expects TLS but gets HTTP
			failingClient := NewClient(testLog(), ClientOptions{})
			_, err := failingClient.GetInfo(context.Background())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to get system info"))
		})

		ginkgo.It("GetInfo returns expected system info fields over TLS", func() {
			tlsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp(`^/v[0-9.]+/info$`)),
					ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{
						"Name":            "test-docker",
						"ServerVersion":   "25.0.0",
						"OSType":          "linux",
						"OperatingSystem": "Alpine Linux",
						"Driver":          "btrfs",
					}),
				),
			)

			info, err := testClient.GetInfo(context.Background())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(info).To(gomega.Equal(types.SystemInfo{
				Name:            "test-docker",
				ServerVersion:   "25.0.0",
				OSType:          "linux",
				OperatingSystem: "Alpine Linux",
				Driver:          "btrfs",
			}))
		})
	})

	ginkgo.Describe("DaemonInitTimeout constant", func() {
		ginkgo.It("should have a reasonable timeout value of 30 seconds", func() {
			expectedTimeout := 30 * time.Second
			gomega.Expect(DaemonInitTimeout).To(gomega.Equal(expectedTimeout))
		})

		ginkgo.It("should not be zero", func() {
			gomega.Expect(DaemonInitTimeout).To(gomega.BeNumerically(">", 0))
		})

		ginkgo.It("should be within reasonable bounds (10s to 5min)", func() {
			minTimeout := 10 * time.Second
			maxTimeout := 5 * time.Minute

			gomega.Expect(DaemonInitTimeout).To(gomega.BeNumerically(">=", minTimeout))
			gomega.Expect(DaemonInitTimeout).To(gomega.BeNumerically("<=", maxTimeout))
		})
	})

	ginkgo.Describe("NewClient", func() {
		ginkgo.It(
			"should successfully connect with TLS when DOCKER_TLS_VERIFY=1 and DOCKER_HOST points to TLS server",
			func() {
				tlsServer := ghttp.NewTLSServer()
				defer tlsServer.Close()

				tlsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/_ping"),
						ghttp.RespondWith(http.StatusOK, "OK"),
					),
				)

				restore := withEnvVars(map[string]string{
					"DOCKER_TLS_VERIFY": "1",
					"DOCKER_HOST":       tlsServer.URL(),
				})
				defer restore()

				client := NewClient(testLog(), ClientOptions{})
				gomega.Expect(client).NotTo(gomega.BeNil())
			},
		)

		ginkgo.It("should fail when TLS is required but server is HTTP-only", func() {
			httpServer := ghttp.NewServer()
			defer httpServer.Close()

			httpServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusOK, "OK"),
				),
			)

			restore := withEnvVars(map[string]string{
				"DOCKER_TLS_VERIFY": "1",
				"DOCKER_HOST":       httpServer.URL(),
			})
			defer restore()

			gomega.Expect(func() { NewClient(testLog(), ClientOptions{}) }).ToNot(gomega.Panic())
		})

		ginkgo.It("should negotiate API version with TLS", func() {
			tlsServer := ghttp.NewTLSServer()
			defer tlsServer.Close()

			tlsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/_ping"),
					ghttp.RespondWith(http.StatusOK, "OK"),
				),
			)

			restore := withEnvVars(map[string]string{
				"DOCKER_TLS_VERIFY": "1",
				"DOCKER_HOST":       tlsServer.URL(),
			})
			defer restore()

			client := NewClient(testLog(), ClientOptions{})
			gomega.Expect(client).NotTo(gomega.BeNil())
		})

		ginkgo.It("should use forced API version with TLS", func() {
			tlsServer := ghttp.NewTLSServer()
			defer tlsServer.Close()

			tlsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/_ping"),
					ghttp.RespondWith(http.StatusOK, "OK"),
				),
			)

			restore := withEnvVars(map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        tlsServer.URL(),
				"DOCKER_API_VERSION": "1.40",
			})
			defer restore()

			client := NewClient(testLog(), ClientOptions{})
			gomega.Expect(client).NotTo(gomega.BeNil())
		})

		ginkgo.It(
			"should handle invalid API version with TLS and fall back to negotiation",
			func() {
				tlsServer := ghttp.NewTLSServer()
				defer tlsServer.Close()

				// First ping fails with 404 (forced version too high).
				// Fallback client retries and succeeds.
				tlsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/_ping"),
						ghttp.RespondWith(http.StatusNotFound, "page not found"),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/_ping"),
						ghttp.RespondWith(http.StatusOK, "OK"),
					),
				)

				restore := withEnvVars(map[string]string{
					"DOCKER_TLS_VERIFY":  "1",
					"DOCKER_HOST":        tlsServer.URL(),
					"DOCKER_API_VERSION": "1.99",
				})
				defer restore()

				client := NewClient(testLog(), ClientOptions{})
				gomega.Expect(client).NotTo(gomega.BeNil())
			},
		)
	})
})

func TestStopAndRemoveContainer_ContainerStillExistsAfterStopping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
				w.WriteHeader(http.StatusNoContent)
			} else if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/") {
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer server.Close()

		docker, _ := dockerClient.New(
			dockerClient.WithHost(server.URL),
			dockerClient.WithHTTPClient(server.Client()),
		)

		// Create a mock container in running state.
		mockedContainer := MockContainer(
			WithContainerState(dockerContainer.State{Running: true}),
		)
		// Execute StopAndRemoveContainer and verify no error occurs.
		err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(
			context.Background(),
			mockedContainer,
			time.Second,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestStopAndRemoveContainer_ContainerDoesNotExistAfterStopping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
				w.WriteHeader(http.StatusNoContent)
			} else if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/") {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		docker, _ := dockerClient.New(
			dockerClient.WithHost(server.URL),
			dockerClient.WithHTTPClient(server.Client()),
		)

		// Create a mock container in running state.
		mockedContainer := MockContainer(
			WithContainerState(dockerContainer.State{Running: true}),
		)
		// Execute StopAndRemoveContainer and verify no error occurs.
		err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(
			context.Background(),
			mockedContainer,
			time.Second,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestStopContainer_StoppingFailsWithUnexpectedError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pingPath {
				w.WriteHeader(http.StatusOK)

				return
			}

			if strings.Contains(r.URL.Path, "/version") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ApiVersion": "1.40", "Version": "20.10.0"}`))

				return
			}

			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"message": "server error"}`))

				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		docker, _ := dockerClient.New(
			dockerClient.WithHost(server.URL),
			dockerClient.WithHTTPClient(server.Client()),
		)

		// Create a mock container in running state.
		mockedContainer := MockContainer(
			WithContainerState(dockerContainer.State{Running: true}),
		)
		// Execute StopContainer and verify the error is propagated.
		err := (&client{log: testLog(), api: docker}).StopContainer(context.Background(), mockedContainer, time.Second)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expectedMsg := "failed to stop container: Error response from daemon: server error"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Fatalf("expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})
}

func TestStopAndRemoveContainer_RemovalFailsWithUnexpectedError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pingPath {
				w.WriteHeader(http.StatusOK)

				return
			}

			if strings.Contains(r.URL.Path, "/version") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ApiVersion": "1.40", "Version": "20.10.0"}`))

				return
			}

			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"message": "server error"}`))

				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		docker, _ := dockerClient.New(
			dockerClient.WithHost(server.URL),
			dockerClient.WithHTTPClient(server.Client()),
		)

		// Create a mock container in running state.
		mockedContainer := MockContainer(
			WithContainerState(dockerContainer.State{Running: true}),
		)
		// Execute StopAndRemoveContainer and verify the removal error is propagated.
		err := (&client{log: testLog(), api: docker}).StopAndRemoveContainer(
			context.Background(),
			mockedContainer,
			time.Second,
		)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expectedMsg := "failed to remove container: Error response from daemon: server error"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Fatalf("expected error to contain %q, got %q", expectedMsg, err.Error())
		}
	})
}

func TestStopContainer_ContainerFailsToStopWithinTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == pingPath {
				w.WriteHeader(http.StatusOK)

				return
			}

			if strings.Contains(r.URL.Path, "/version") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"ApiVersion": "1.40", "Version": "20.10.0"}`))

				return
			}

			if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/stop") {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/") {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		docker, _ := dockerClient.New(
			dockerClient.WithHost(server.URL),
			dockerClient.WithHTTPClient(server.Client()),
		)

		// Create a mock container in running state.
		mockedContainer := MockContainer(
			WithContainerState(dockerContainer.State{Running: true}),
		)
		// Capture log output for verification.
		log, logbuf := captureLog(zerolog.DebugLevel)
		// Execute StopAndRemoveContainer with a realistic timeout.
		err := (&client{log: log, api: docker}).StopAndRemoveContainer(
			context.Background(),
			mockedContainer,
			1*time.Second,
		)
		// Verify no error occurs, as removal should succeed.
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// Verify log output includes expected message from container_source.go.
		if !strings.Contains(string(logbuf.Contents()), "Container removed successfully") {
			t.Fatalf(
				"expected log to contain 'Container removed successfully', got %q",
				string(logbuf.Contents()),
			)
		}
	})
}

// setupExecMockHandlers sets up mock handlers for ExecuteCommand tests.
// It registers 5 handlers: ContainerInspect, ImageInspect, ContainerExecCreate,
// ContainerExecStart, and ContainerExecInspect.
//
// Parameters:
//   - mockServer: The test HTTP server
//   - containerID: Container ID for ContainerInspect
//   - execID: Exec ID returned by ContainerExecCreate
//   - cmd: Command to execute
//   - uid: UID value (negative values indicate no UID specified)
//   - gid: GID value (negative values indicate no GID specified)
//   - exitCode: Exit code to return (0 for success case)
//   - execCreateError: If true, make ContainerExecCreate return 500 error
//   - execStartError: If true, make ContainerExecStart return 500 error
//
// The uid/gid parameters use negative values as sentinels to indicate "no user specified".
// This allows 0:0 (root) to be properly tested as a valid user specification.
// When both uid and gid are non-negative, they are formatted as "uid:gid" for the exec user.
func setupExecMockHandlers(
	mockServer *ghttp.Server,
	containerID string,
	execID string,
	cmd string,
	uid int,
	gid int,
	exitCode int,
	execCreateError bool,
	execStartError bool,
) {
	// Hardcoded image name used by all tests
	imageName := "test-image:latest"

	// Determine the user string - use uid/gid if provided (non-negative values)
	execUser := ""
	if uid >= 0 && gid >= 0 {
		execUser = fmt.Sprintf("%d:%d", uid, gid)
	}

	// Build the WT_CONTAINER environment variable
	wgContainerEnv := fmt.Sprintf(
		"WT_CONTAINER={\"name\":\"test-container\",\"id\":\"%s\",\"image_name\":\"%s\",\"stop_signal\":\"SIGTERM\",\"labels\":{}}",
		containerID, imageName,
	)

	mockServer.AppendHandlers(
		// Handler for ContainerInspect (GetContainer)
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp(
					fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
				),
			),
			ghttp.RespondWithJSONEncoded(
				http.StatusOK,
				dockerContainer.InspectResponse{
					ID:    containerID,
					Name:  "/test-container",
					Image: imageName,
					State: &dockerContainer.State{
						Status: "running",
					},
					HostConfig: &dockerContainer.HostConfig{},
					Config: &dockerContainer.Config{
						Image:  imageName,
						Labels: map[string]string{},
					},
				},
			),
		),
		// Handler for ImageInspect
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/images/%s/json$", imageName)),
			),
			ghttp.RespondWithJSONEncoded(
				http.StatusOK,
				dockerImage.InspectResponse{
					ID: "test-image-id",
				},
			),
		),
	)

	// Handle execCreateError case - only 3 handlers needed
	if execCreateError {
		mockServer.AppendHandlers(
			// Handler for ContainerExecCreate - returns error
			ghttp.CombineHandlers(
				ghttp.VerifyRequest(
					"POST",
					gomega.MatchRegexp(
						fmt.Sprintf("^/v[0-9.]+/containers/%s/exec$", containerID),
					),
				),
				ghttp.RespondWith(http.StatusInternalServerError, "exec create failed"),
			),
		)

		return
	}

	// ContainerExecCreate handler (success case)
	mockServer.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"POST",
				gomega.MatchRegexp(
					fmt.Sprintf("^/v[0-9.]+/containers/%s/exec$", containerID),
				),
			),
			// Only verify JSON if user is specified
			ghttp.VerifyJSONRepresenting(dockerContainer.ExecCreateRequest{
				User: execUser,
				Tty:  true,
				Cmd: []string{
					"sh",
					"-c",
					cmd,
				},
				Env: []string{
					wgContainerEnv,
				},
				AttachStdout: true,
				AttachStderr: true,
			}),
			ghttp.RespondWithJSONEncoded(
				http.StatusOK,
				dockerContainer.CommitResponse{ID: execID},
			),
		),
	)

	// Handle execStartError case
	if execStartError {
		mockServer.AppendHandlers(
			// Handler for ContainerExecStart - returns error
			ghttp.CombineHandlers(
				ghttp.VerifyRequest(
					"POST",
					gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/exec/%s/start$", execID)),
				),
				ghttp.RespondWith(http.StatusInternalServerError, "exec start failed"),
			),
		)

		return
	}

	// ContainerExecStart and ContainerExecInspect handlers (success case)
	mockServer.AppendHandlers(
		// Handler for ContainerExecStart
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"POST",
				gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/exec/%s/start$", execID)),
			),
			ghttp.VerifyJSONRepresenting(dockerContainer.ExecStartRequest{
				Detach: true,
				Tty:    true,
			}),
			ghttp.RespondWith(http.StatusOK, nil),
		),
		// Handler for ContainerExecInspect
		ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/exec/%s/json$", execID)),
			),
			ghttp.RespondWithJSONEncoded(
				http.StatusOK,
				dockerContainer.ExecInspectResponse{
					ID:       execID,
					Running:  false,
					ExitCode: &exitCode,
					ProcessConfig: &dockerContainer.ExecProcessConfig{
						Entrypoint: "sh",
						Arguments:  []string{"-c", cmd},
						User:       execUser,
					},
					ContainerID: containerID,
				},
			),
		),
	)
}

// inspectHandler creates a ghttp handler for container inspect responses.
// This helper function builds a CombineHandlers that verifies the request
// and returns a Docker container inspect response with the specified health status.
//
// Parameters:
//   - cid: Container ID to include in the response
//   - healthStatus: Health status to include in the response (e.g., dockerContainer.Starting, dockerContainer.Healthy, dockerContainer.Unhealthy)
//
// Returns:
//   - http.HandlerFunc: A combined handler for container inspect requests
func inspectHandler(cid string, healthStatus dockerContainer.HealthStatus) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest(
			"GET",
			gomega.MatchRegexp(fmt.Sprintf(`^/v[0-9.]+/containers/%s/json$`, cid)),
		),
		ghttp.RespondWithJSONEncoded(
			http.StatusOK,
			dockerContainer.InspectResponse{
				ID: cid,
				State: &dockerContainer.State{
					Status: "running",
					Health: &dockerContainer.Health{
						Status: healthStatus,
					},
				},
				Config: &dockerContainer.Config{},
			},
		),
	)
}

// havingRestartingState creates a Gomega matcher for container restarting state.
//
// Parameters:
//   - expected: Expected restarting state (true or false).
//
// Returns:
//   - gomegaTypes.GomegaMatcher: Matcher for verifying restarting state.
func havingRestartingState(expected bool) gomegaTypes.GomegaMatcher {
	return gomega.WithTransform(func(container types.Container) bool {
		return container.ContainerInfo().State.Restarting
	}, gomega.Equal(expected))
}

// havingRunningState creates a Gomega matcher for container running state.
//
// Parameters:
//   - expected: Expected running state (true or false).
//
// Returns:
//   - gomegaTypes.GomegaMatcher: Matcher for verifying running state.
func havingRunningState(expected bool) gomegaTypes.GomegaMatcher {
	return gomega.WithTransform(func(container types.Container) bool {
		return container.ContainerInfo().State.Running
	}, gomega.Equal(expected))
}

// withEnvVars sets environment variables and returns a restore function.
//
// Parameters:
//   - vars: Map of environment variables to set.
//
// Returns:
//   - func(): Function to restore original environment variables.
func withEnvVars(vars map[string]string) func() {
	type envState struct {
		value  string
		exists bool
	}

	original := make(map[string]envState)

	for k, v := range vars {
		orig, exists := os.LookupEnv(k)
		original[k] = envState{value: orig, exists: exists}
		os.Setenv(k, v)
	}

	return func() {
		for k, state := range original {
			if !state.exists {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, state.value)
			}
		}
	}
}

var _ = ginkgo.Describe("isDaemonConnectionError", func() {
	ginkgo.DescribeTable("error detection",
		func(err error, expected bool) {
			gomega.Expect(isDaemonConnectionError(err)).To(gomega.Equal(expected))
		},
		ginkgo.Entry("nil error returns false", nil, false),
		ginkgo.Entry("Docker daemon connection error returns true",
			errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"),
			true),
		ginkgo.Entry("connection refused error returns true",
			errors.New("dial unix /var/run/docker.sock: connect: connection refused"),
			true),
		ginkgo.Entry("EOF error returns true",
			fmt.Errorf("failed to execute request: %w", io.EOF),
			true),
		ginkgo.Entry("unexpected EOF error returns true",
			fmt.Errorf("failed to execute request: %w", io.ErrUnexpectedEOF),
			true),
		ginkgo.Entry("permission denied error returns false",
			errors.New("permission denied"),
			false),
		ginkgo.Entry("container not found error returns false",
			errors.New("Error: No such container: abc123"),
			false),
		ginkgo.Entry("generic error returns false",
			errors.New("some random error"),
			false),
		ginkgo.Entry("wrapped Docker daemon error returns true",
			fmt.Errorf("failed to list containers: %w", errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")),
			true),
	)
})

var _ = ginkgo.Describe("SetRestartPolicy", func() {
	var (
		mockServer *ghttp.Server
		docker     *dockerClient.Client
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("the container is nil", func() {
		ginkgo.It("should short-circuit without calling ContainerUpdate", func() {
			c := &client{log: testLog()}

			c.SetRestartPolicy(
				context.Background(),
				nil,
				dockerContainer.RestartPolicy{},
			)
		})
	})

	ginkgo.When("the policy name is empty", func() {
		ginkgo.It("should send restart policy Name=no to the Docker API", func() {
			cid := "watchtower-container-id"
			mockedContainer := MockContainer(
				WithID(cid),
			)

			mockServer.AppendHandlers(
				ContainerUpdateHandler(cid, http.StatusOK, "no"),
			)

			c := &client{log: testLog(), api: docker}

			c.SetRestartPolicy(
				context.Background(),
				mockedContainer,
				dockerContainer.RestartPolicy{},
			)
		})

		ginkgo.It("should clear MaximumRetryCount when normalizing an empty name", func() {
			cid := "watchtower-container-id"
			mockedContainer := MockContainer(
				WithID(cid),
			)

			mockServer.AppendHandlers(
				ContainerUpdateHandler(cid, http.StatusOK, "no"),
			)

			c := &client{log: testLog(), api: docker}

			c.SetRestartPolicy(
				context.Background(),
				mockedContainer,
				dockerContainer.RestartPolicy{MaximumRetryCount: 5},
			)
		})
	})

	ginkgo.When("the policy is always", func() {
		ginkgo.It("should send restart policy Name=always to the Docker API", func() {
			cid := "watchtower-container-id"
			mockedContainer := MockContainer(
				WithID(cid),
			)

			mockServer.AppendHandlers(
				ContainerUpdateHandler(cid, http.StatusOK, "always"),
			)

			c := &client{log: testLog(), api: docker}

			c.SetRestartPolicy(
				context.Background(),
				mockedContainer,
				dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyAlways},
			)
		})
	})

	ginkgo.When("the container update fails", func() {
		ginkgo.It("should log the failure and not panic", func() {
			log, logbuf := captureLog(zerolog.DebugLevel)

			cid := "watchtower-container-id"
			mockedContainer := MockContainer(
				WithID(cid),
			)

			mockServer.AppendHandlers(
				ContainerUpdateHandler(cid, http.StatusInternalServerError, ""),
			)

			c := &client{log: log, api: docker}

			c.SetRestartPolicy(
				context.Background(),
				mockedContainer,
				dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
			)

			gomega.Expect(logbuf).To(gbytes.Say("Failed to set container restart policy"))
		})
	})
})

// TestListContainers_RestartingFilterFollowsRuntime verifies that with
// restarting containers included, the list request asks for the restarting
// status on Docker and leaves it out on Podman, which has no such status, in
// every CPU copy mode.
func TestListContainers_RestartingFilterFollowsRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		marker         string
		wantRestarting bool
	}{
		{name: "Docker", marker: "/.dockerenv", wantRestarting: true},
		{name: "Podman", marker: "/run/.containerenv", wantRestarting: false},
	}

	for _, tt := range tests {
		for _, mode := range []string{CPUCopyModeAuto, "full", "none"} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				var listFilters atomic.Value

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.URL.Path == pingPath:
						w.Header().Set("Api-Version", "1.47")
					case strings.HasSuffix(r.URL.Path, "/containers/json"):
						listFilters.Store(r.URL.Query().Get("filters"))
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte("[]"))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer server.Close()

				api, err := dockerClient.New(
					dockerClient.WithHost("tcp://"+strings.TrimPrefix(server.URL, "http://")),
					dockerClient.WithHTTPClient(server.Client()),
				)
				require.NoError(t, err)

				fsys := afero.NewMemMapFs()
				require.NoError(t, afero.WriteFile(fsys, tt.marker, nil, 0o600))

				c := &client{
					log:               testLog(),
					api:               api,
					CPUCopyMode:       mode,
					IncludeRestarting: true,
					Fs:                fsys,
				}

				_, err = c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)

				sent, ok := listFilters.Load().(string)
				require.True(t, ok, "the list request was sent")
				assert.Equal(t, tt.wantRestarting, strings.Contains(sent, "restarting"), "filters: %s", sent)
			})
		}
	}
}

// TestGetRuntime_RetriesFailedDetection verifies that a failed detection is
// not cached, so a later call detects the runtime, and that a successful
// detection is cached. It sets environment variables, so it runs sequentially.
func TestGetRuntime_RetriesFailedDetection(t *testing.T) {
	t.Setenv("container", "")
	t.Setenv("CONTAINER", "")

	var infoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pingPath {
			w.Header().Set("Api-Version", "1.47")
			w.WriteHeader(http.StatusOK)

			return
		}

		if strings.HasSuffix(r.URL.Path, "/info") {
			if infoCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Name":"podman"}`))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	api, err := dockerClient.New(
		dockerClient.WithHost("tcp://"+strings.TrimPrefix(server.URL, "http://")),
		dockerClient.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)

	c := &client{
		log:         testLog(),
		api:         api,
		CPUCopyMode: CPUCopyModeAuto,
		Fs:          afero.NewMemMapFs(),
	}

	assert.False(t, c.getRuntime(t.Context()), "a failed detection falls back to Docker")
	assert.True(t, c.getRuntime(t.Context()), "the next call detects Podman")
	assert.True(t, c.getRuntime(t.Context()), "the detection is cached")
	assert.Equal(t, int32(2), infoCalls.Load())
}

// TestDetectRuntimeByEnv_PodmanVariable verifies that the lowercase container
// variable, which Podman sets, identifies Podman. It sets environment
// variables, so it runs sequentially.
func TestDetectRuntimeByEnv_PodmanVariable(t *testing.T) {
	t.Setenv("container", "podman")
	t.Setenv("CONTAINER", "")

	c := &client{log: testLog()}

	assert.True(t, c.detectRuntimeByEnv())
}
