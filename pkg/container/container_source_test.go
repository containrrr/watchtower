package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/moby/moby/api/types/storage"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	testContainerID = "test-container-id"
	testShortID     = "abc123def456"
)

var _ = ginkgo.Describe("ListSourceContainers", func() {
	var (
		docker     *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		require.NoError(ginkgo.GinkgoT(), err)

		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	// Helper function to verify filters in request
	verifyFilters := func(expectedStatuses []string) http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest(
				"GET",
				gomega.MatchRegexp(
					"^/v[0-9.]+/containers/json$",
				),
			),
			func(w http.ResponseWriter, r *http.Request) {
				filtersParam := r.URL.Query().Get("filters")

				var filters map[string]map[string]bool

				err := json.Unmarshal([]byte(filtersParam), &filters)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				statusMap, exists := filters["status"]
				if len(expectedStatuses) == 0 {
					gomega.Expect(exists).To(gomega.BeFalse())
				} else {
					gomega.Expect(exists).To(gomega.BeTrue())

					actualStatuses := make([]string, 0, len(statusMap))
					for status := range statusMap {
						actualStatuses = append(actualStatuses, status)
					}

					gomega.Expect(actualStatuses).To(gomega.ConsistOf(expectedStatuses))
				}

				// Return a running container for the test
				containers := []dockerContainer.Summary{
					{ID: testContainerID, Names: []string{"/test-container"}, State: "running"},
				}
				ghttp.RespondWithJSONEncoded(http.StatusOK, containers)(w, r)
			},
		)
	}

	// Helper to mock container and image inspect
	mockInspects := func(containerID string) []http.HandlerFunc {
		return []http.HandlerFunc{
			ghttp.CombineHandlers(
				ghttp.VerifyRequest(
					"GET",
					gomega.MatchRegexp(
						fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
					),
				),
				ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
					ID:    containerID,
					Name:  "/test-container",
					Image: "test-image:latest",
					State: &dockerContainer.State{
						Status:  "running",
						Running: true,
					},
					HostConfig: &dockerContainer.HostConfig{},
					Config: &dockerContainer.Config{
						Image: "test-image:latest",
					},
				}),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest(
					"GET",
					gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
				),
				ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
					ID: "test-image-id",
				}),
			),
		}
	}

	ginkgo.When("no custom filter with default ClientOptions", func() {
		ginkgo.It("should list only running containers", func() {
			mockServer.AppendHandlers(
				verifyFilters([]string{"running"}),
			)
			mockServer.AppendHandlers(mockInspects(testContainerID)...)

			containers, err := ListSourceContainers(testLog(), context.Background(),
				docker,
				ClientOptions{},
				nil,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))
		})
	})

	ginkgo.When("no custom filter with IncludeStopped=true", func() {
		ginkgo.It("should list running, created, and exited containers", func() {
			mockServer.AppendHandlers(
				verifyFilters([]string{"running", "created", "exited"}),
			)
			mockServer.AppendHandlers(mockInspects(testContainerID)...)

			containers, err := ListSourceContainers(testLog(), context.Background(),
				docker,
				ClientOptions{IncludeStopped: true},
				nil,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))
		})
	})

	ginkgo.When("no custom filter with IncludeRestarting=true", func() {
		ginkgo.It("should list running and restarting containers", func() {
			mockServer.AppendHandlers(
				verifyFilters([]string{"running", "restarting"}),
			)
			mockServer.AppendHandlers(mockInspects(testContainerID)...)

			containers, err := ListSourceContainers(testLog(), context.Background(),
				docker,
				ClientOptions{IncludeRestarting: true},
				nil,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))
		})
	})

	ginkgo.When("with custom filter (filters.NoFilter)", func() {
		ginkgo.It("should list containers and apply custom filter", func() {
			mockServer.AppendHandlers(
				verifyFilters([]string{"running"}),
			)
			mockServer.AppendHandlers(mockInspects(testContainerID)...)

			containers, err := ListSourceContainers(testLog(), context.Background(),
				docker,
				ClientOptions{},
				filters.NoFilter,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))
		})
	})

	ginkgo.When("Docker API returns 404 NotFound for container list", func() {
		ginkgo.It("should return empty container list without error", func() {
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/containers/json$"),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message":"page not found"}`),
				),
			)

			containers, err := ListSourceContainers(testLog(), context.Background(),
				docker,
				ClientOptions{},
				nil,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("image inspection fails", func() {
		ginkgo.It("should not warn for containers excluded by the filter", func() {
			mockServer.AppendHandlers(verifyFilters([]string{"running"}))
			mockServer.AppendHandlers(missingImageHandlers(testContainerID)...)

			log, logBuf := captureLog(zerolog.WarnLevel)
			excludeAll := filters.FilterByDisableNames(
				log,
				[]string{"test-container"},
				filters.NoFilter,
			)

			containers, err := ListSourceContainers(log, context.Background(),
				docker,
				ClientOptions{},
				excludeAll,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.BeEmpty())
			gomega.Expect(string(logBuf.Contents())).
				ToNot(gomega.ContainSubstring("Failed to retrieve image info"))
		})

		ginkgo.It("should warn for containers that pass the filter", func() {
			mockServer.AppendHandlers(verifyFilters([]string{"running"}))
			mockServer.AppendHandlers(missingImageHandlers(testContainerID)...)

			log, logBuf := captureLog(zerolog.WarnLevel)

			containers, err := ListSourceContainers(log, context.Background(),
				docker,
				ClientOptions{},
				nil,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(containers).To(gomega.HaveLen(1))

			logged := string(logBuf.Contents())
			gomega.Expect(logged).To(gomega.ContainSubstring("Failed to retrieve image info"))
			// The warning must identify the container by name and image, not just by ID.
			gomega.Expect(logged).To(gomega.ContainSubstring("test-container"))
			gomega.Expect(logged).To(gomega.ContainSubstring("test-image:latest"))
		})
	})
})

var _ = ginkgo.Describe("buildListFilterArgs", func() {
	ginkgo.It("includes running status always", func() {
		opts := ClientOptions{}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		gomega.Expect(statuses).To(gomega.ContainElement("running"))
	})

	ginkgo.It("includes created and exited when IncludeStopped is true", func() {
		opts := ClientOptions{IncludeStopped: true}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		gomega.Expect(statuses).To(gomega.ContainElement("created"))
		gomega.Expect(statuses).To(gomega.ContainElement("exited"))
		gomega.Expect(statuses).To(gomega.ContainElement("running"))
	})

	ginkgo.It("does not include created and exited when IncludeStopped is false", func() {
		opts := ClientOptions{IncludeStopped: false}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		gomega.Expect(statuses).ToNot(gomega.ContainElement("created"))
		gomega.Expect(statuses).ToNot(gomega.ContainElement("exited"))
		gomega.Expect(statuses).To(gomega.ContainElement("running"))
	})

	ginkgo.It(
		"includes restarting when IncludeRestarting is true and isPodman is false",
		func() {
			opts := ClientOptions{IncludeRestarting: true}
			filterArgs := buildListFilterArgs(opts, false)
			statuses := getStatusFilterKeys(filterArgs)
			gomega.Expect(statuses).To(gomega.ContainElement("restarting"))
			gomega.Expect(statuses).To(gomega.ContainElement("running"))
		},
	)

	ginkgo.It("does not include restarting when IncludeRestarting is false", func() {
		opts := ClientOptions{IncludeRestarting: false}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		gomega.Expect(statuses).ToNot(gomega.ContainElement("restarting"))
		gomega.Expect(statuses).To(gomega.ContainElement("running"))
	})

	ginkgo.It(
		"does not include restarting when isPodman is true regardless of IncludeRestarting",
		func() {
			opts := ClientOptions{IncludeRestarting: true}
			filterArgs := buildListFilterArgs(opts, true)
			statuses := getStatusFilterKeys(filterArgs)
			gomega.Expect(statuses).ToNot(gomega.ContainElement("restarting"))
			gomega.Expect(statuses).To(gomega.ContainElement("running"))
		},
	)

	ginkgo.It(
		"includes all statuses when IncludeStopped and IncludeRestarting are true and isPodman is false",
		func() {
			opts := ClientOptions{IncludeStopped: true, IncludeRestarting: true}
			filterArgs := buildListFilterArgs(opts, false)
			statuses := getStatusFilterKeys(filterArgs)
			gomega.Expect(statuses).To(gomega.ContainElement("running"))
			gomega.Expect(statuses).To(gomega.ContainElement("created"))
			gomega.Expect(statuses).To(gomega.ContainElement("exited"))
			gomega.Expect(statuses).To(gomega.ContainElement("restarting"))
		},
	)

	ginkgo.It(
		"includes running, created, exited but not restarting when IncludeStopped is true, IncludeRestarting is true, and isPodman is true",
		func() {
			opts := ClientOptions{IncludeStopped: true, IncludeRestarting: true}
			filterArgs := buildListFilterArgs(opts, true)
			statuses := getStatusFilterKeys(filterArgs)
			gomega.Expect(statuses).To(gomega.ContainElement("running"))
			gomega.Expect(statuses).To(gomega.ContainElement("created"))
			gomega.Expect(statuses).To(gomega.ContainElement("exited"))
			gomega.Expect(statuses).ToNot(gomega.ContainElement("restarting"))
		},
	)
})

var _ = ginkgo.Describe("GetSourceContainer", func() {
	var (
		docker     *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		require.NoError(ginkgo.GinkgoT(), err)

		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("retrieving a running container successfully", func() {
		ginkgo.It("should return container with image info", func() {
			containerID := testContainerID
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/test-watchtower",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container).ToNot(gomega.BeNil())
			gomega.Expect(container.ID()).To(gomega.Equal(types.ContainerID(containerID)))
			gomega.Expect(container.Name()).To(gomega.Equal("test-watchtower"))
			gomega.Expect(container.ImageID()).To(gomega.Equal(types.ImageID("test-image-id")))
		})
	})

	ginkgo.When("retrieving a stopped container successfully", func() {
		ginkgo.It("should return container without image info when ImageInspect fails", func() {
			containerID := testContainerID
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/test-watchtower",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "exited",
							Running: false,
						},
						HostConfig: &dockerContainer.HostConfig{},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWith(
						http.StatusInternalServerError,
						`{"message": "image not found"}`,
					),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container).ToNot(gomega.BeNil())
			gomega.Expect(container.ID()).To(gomega.Equal(types.ContainerID(containerID)))
			gomega.Expect(container.ImageID()).To(gomega.Equal(types.ImageID("")))
		})
	})

	ginkgo.When("container has network mode requiring resolution", func() {
		ginkgo.It("should resolve network container name", func() {
			containerID := testContainerID
			parentID := "parent-container-id"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/test-watchtower",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							NetworkMode: dockerContainer.NetworkMode("container:" + parentID),
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", parentID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   parentID,
						Name: "/parent-container",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container).ToNot(gomega.BeNil())
			gomega.Expect(container.ContainerInfo().HostConfig.NetworkMode).
				To(gomega.Equal(dockerContainer.NetworkMode("container:/parent-container")))
		})
	})

	ginkgo.When("container has volumes-from requiring resolution", func() {
		ginkgo.It("should rewrite volumes-from container ID to name", func() {
			containerID := testContainerID
			appID := "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/nextcloud-nginx",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							VolumesFrom: []string{appID + ":rw"},
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", appID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   appID,
						Name: "/nextcloud-app",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container.ContainerInfo().HostConfig.VolumesFrom).
				To(gomega.Equal([]string{"nextcloud-app:rw"}))
			gomega.Expect(container.GetCreateHostConfig().VolumesFrom).
				To(gomega.Equal([]string{"nextcloud-app:rw"}))
		})

		ginkgo.It("should leave volumes-from ID when the source cannot be resolved", func() {
			containerID := testContainerID
			appID := "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/nextcloud-nginx",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							VolumesFrom: []string{appID},
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", appID),
						),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message":"No such container"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container.ContainerInfo().HostConfig.VolumesFrom).
				To(gomega.Equal([]string{appID}))
		})

		ginkgo.It("should rewrite volumes-from IDs and preserve ro mode", func() {
			containerID := testContainerID
			appID := "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/nextcloud-nginx",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							VolumesFrom: []string{appID + ":ro"},
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", appID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   appID,
						Name: "/nextcloud-app",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container.ContainerInfo().HostConfig.VolumesFrom).
				To(gomega.Equal([]string{"nextcloud-app:ro"}))
		})

		ginkgo.It("should rewrite multiple volumes-from entries independently", func() {
			containerID := testContainerID
			appID := "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
			dbID := "a1b2c3d4e5f60718293a4b5c6d7e8f901234567890abcdef1234567890abcd"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/nextcloud-nginx",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							VolumesFrom: []string{appID + ":rw", dbID},
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", appID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   appID,
						Name: "/nextcloud-app",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", dbID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   dbID,
						Name: "/nextcloud-db",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container.ContainerInfo().HostConfig.VolumesFrom).
				To(gomega.Equal([]string{"nextcloud-app:rw", "nextcloud-db"}))
		})

		ginkgo.It("should rewrite resolved volumes-from entries and leave unresolved IDs", func() {
			containerID := testContainerID
			appID := "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
			missingID := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:    containerID,
						Name:  "/nextcloud-nginx",
						Image: "test-image:latest",
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
						HostConfig: &dockerContainer.HostConfig{
							VolumesFrom: []string{appID + ":rw", missingID},
						},
						Config: &dockerContainer.Config{
							Image: "test-image:latest",
						},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", appID),
						),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerContainer.InspectResponse{
						ID:   appID,
						Name: "/nextcloud-app",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", missingID),
						),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message":"No such container"}`),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp("^/v[0-9.]+/images/test-image:latest/json$"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: "test-image-id",
					}),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(container.ContainerInfo().HostConfig.VolumesFrom).
				To(gomega.Equal([]string{"nextcloud-app:rw", missingID}))
		})
	})

	ginkgo.When("container ID is invalid", func() {
		ginkgo.It("should return error for 404 not found", func() {
			containerID := "invalid-container-id"
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message": "No such container"}`),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to inspect container"))
			gomega.Expect(container).To(gomega.BeNil())
		})
	})

	ginkgo.When("API connection fails", func() {
		ginkgo.It("should return error for connection timeout", func() {
			containerID := testContainerID
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWith(http.StatusGatewayTimeout, ""),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to inspect container"))
			gomega.Expect(container).To(gomega.BeNil())
		})
	})

	ginkgo.When("response is malformed", func() {
		ginkgo.It("should return error for invalid JSON", func() {
			containerID := testContainerID
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.MatchRegexp(
							fmt.Sprintf("^/v[0-9.]+/containers/%s/json$", containerID),
						),
					),
					ghttp.RespondWith(http.StatusOK, `{"invalid": json}`),
				),
			)

			container, err := GetSourceContainer(testLog(), context.Background(),
				docker,
				types.ContainerID(containerID),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to inspect container"))
			gomega.Expect(container).To(gomega.BeNil())
		})
	})
})

var _ = ginkgo.Describe("StopAndRemoveSourceContainer", func() {
	var (
		docker     *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		require.NoError(ginkgo.GinkgoT(), err)

		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When(
		"stopping and removing a running container successfully with removeVolumes=false",
		func() {
			ginkgo.It("should stop and remove the container without error", func() {
				container := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := container.ContainerInfo().ID

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"POST",
							gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
						),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"DELETE",
							gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
						),
						func(w http.ResponseWriter, r *http.Request) {
							// Verify query parameters
							gomega.Expect(r.URL.Query().Get("force")).To(gomega.Equal("1"))
							gomega.Expect(r.URL.Query().Get("v")).
								To(gomega.Equal(""))
								// removeVolumes=false
							w.WriteHeader(http.StatusNoContent)
						},
					),
				)

				err := StopAndRemoveSourceContainer(testLog(), context.Background(),
					docker,
					container,
					10*time.Second,
					false,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		},
	)

	ginkgo.When(
		"stopping and removing a running container successfully with removeVolumes=true",
		func() {
			ginkgo.It("should stop and remove the container with volume removal", func() {
				container := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
				)
				cid := container.ContainerInfo().ID

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"POST",
							gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
						),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"DELETE",
							gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
						),
						func(w http.ResponseWriter, r *http.Request) {
							// Verify query parameters
							gomega.Expect(r.URL.Query().Get("force")).To(gomega.Equal("1"))
							gomega.Expect(r.URL.Query().Get("v")).
								To(gomega.Equal("1"))
								// removeVolumes=true
							w.WriteHeader(http.StatusNoContent)
						},
					),
				)

				err := StopAndRemoveSourceContainer(testLog(), context.Background(),
					docker,
					container,
					10*time.Second,
					true,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})
		},
	)

	ginkgo.When("stopping fails", func() {
		ginkgo.It("should return an error without attempting removal", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusInternalServerError, `{"message": "stop failed"}`),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				false,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to stop container"))
		})
	})

	ginkgo.When("removing fails", func() {
		ginkgo.It("should return an error after successful stop", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					ghttp.RespondWith(
						http.StatusInternalServerError,
						`{"message": "remove failed"}`,
					),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				false,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to remove container"))
		})
	})

	ginkgo.When("container has AutoRemove enabled", func() {
		ginkgo.It("should stop but skip removal when running", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
				WithAutoRemove(true),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				true,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// API version ping + stop. No DELETE. Docker AutoRemove handles cleanup after stop.
			gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(2))
		})

		ginkgo.It("should remove explicitly when not running", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false, Status: "created"}),
				WithAutoRemove(true),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					func(w http.ResponseWriter, r *http.Request) {
						gomega.Expect(r.URL.Query().Get("force")).To(gomega.Equal("1"))
						w.WriteHeader(http.StatusNoContent)
					},
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// API version ping + DELETE. AutoRemove does not apply to never-started containers.
			gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(2))
		})
	})

	ginkgo.When("container HostConfig is nil", func() {
		ginkgo.It("should stop and remove without panicking", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					c.HostConfig = nil
				},
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				true,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Stop + DELETE (nil HostConfig is not AutoRemove).
			// ReceivedRequests includes the initial API version ping.
			gomega.Expect(mockServer.ReceivedRequests()).To(gomega.HaveLen(3))
		})
	})

	ginkgo.When("container is not running", func() {
		ginkgo.It("should skip stop and remove successfully", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					func(w http.ResponseWriter, r *http.Request) {
						gomega.Expect(r.URL.Query().Get("force")).To(gomega.Equal("1"))
						gomega.Expect(r.URL.Query().Get("v")).To(gomega.Equal("1"))
						w.WriteHeader(http.StatusNoContent)
					},
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				true,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.When("timeout is specified", func() {
		ginkgo.It("should pass timeout to stop operation", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID
			timeout := 30 * time.Second

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					func(w http.ResponseWriter, r *http.Request) {
						query := r.URL.Query()
						signal := query.Get("signal")
						timeoutStr := query.Get("t")

						gomega.Expect(signal).To(gomega.Equal("SIGTERM"))
						gomega.Expect(timeoutStr).To(gomega.Equal("30"))
						w.WriteHeader(http.StatusNoContent)
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				timeout,
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.When("container is not found during removal", func() {
		ginkgo.It("should succeed as container is already gone", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"DELETE",
						gomega.MatchRegexp(fmt.Sprintf("^/v[0-9.]+/containers/%s$", cid)),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message": "No such container"}`),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.When("API connection fails during stop", func() {
		ginkgo.It("should return error", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusGatewayTimeout, ""),
				),
			)

			err := StopAndRemoveSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
				false,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to stop container"))
		})
	})
})

var _ = ginkgo.Describe("getNetworkConfig", func() {
	ginkgo.Context("with bridge network mode", func() {
		ginkgo.It("should return network config with processed endpoints", func() {
			container := MockContainer(
				WithNetworkMode("bridge"),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						NetworkID:  "bridge_network_id",
						MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
						Aliases:    []string{"container_id", "test-alias"},
						DNSNames:   []string{"test.example.com"},
						IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
							IPv4Address: netip.MustParseAddr("172.17.0.2"),
						},
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config).ToNot(gomega.BeNil())
			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("bridge"))
			endpoint := config.EndpointsConfig["bridge"]
			gomega.Expect(endpoint.NetworkID).To(gomega.Equal("bridge_network_id"))
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(
				dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
			))
			gomega.Expect(endpoint.IPAddress).To(gomega.Equal(
				netip.MustParseAddr("172.17.0.2"),
			))
			gomega.Expect(endpoint.DNSNames).To(gomega.ConsistOf("test.example.com"))
			gomega.Expect(endpoint.IPAMConfig).ToNot(gomega.BeNil())
			gomega.Expect(endpoint.IPAMConfig.IPv4Address).To(gomega.Equal(
				netip.MustParseAddr("172.17.0.2"),
			))
			// Aliases should be filtered to remove container short ID
			gomega.Expect(endpoint.Aliases).To(gomega.ConsistOf("test-alias"))
		})

		ginkgo.It("should handle multiple networks", func() {
			container := MockContainer(
				WithNetworks("bridge", "custom_network"),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config.EndpointsConfig).To(gomega.HaveLen(2))
			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("bridge"))
			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("custom_network"))
		})

		ginkgo.It("should clear engine-generated MAC for running container without validation error", func() {
			container := MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						NetworkID:  "bridge_network_id",
						MacAddress: mustParseMAC("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config).ToNot(gomega.BeNil())
			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("bridge"))
			endpoint := config.EndpointsConfig["bridge"]
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
		})
	})

	ginkgo.Context("with host network mode", func() {
		ginkgo.It("should return network config with cleared host-specific settings", func() {
			container := MockContainer(
				WithNetworkMode("host"),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"host": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("192.168.1.100"),
						Aliases:    []string{"container_id", "host-alias"},
						DNSNames:   []string{"host.example.com"},
						IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
							IPv4Address: netip.MustParseAddr("192.168.1.100"),
						},
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("host"))
			endpoint := config.EndpointsConfig["host"]
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
			gomega.Expect(endpoint.IPAddress).To(gomega.Equal(netip.Addr{}))
			gomega.Expect(endpoint.DNSNames).To(gomega.BeNil())
			gomega.Expect(endpoint.IPAMConfig).To(gomega.BeNil())
			gomega.Expect(endpoint.Aliases).To(gomega.BeNil())
		})
	})

	ginkgo.Context("with legacy client version (< 1.44)", func() {
		ginkgo.It("should clear MAC addresses and IP info", func() {
			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
						DNSNames:   []string{"test.example.com"},
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.40")

			endpoint := config.EndpointsConfig["bridge"]
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
			gomega.Expect(endpoint.IPAddress).To(gomega.Equal(netip.Addr{}))
			gomega.Expect(endpoint.DNSNames).To(gomega.BeNil())
		})
	})

	ginkgo.Context("with nil network settings", func() {
		ginkgo.It("should return empty network config", func() {
			container := MockContainer(
				func(container *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					container.NetworkSettings = nil
				},
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config).ToNot(gomega.BeNil())
			gomega.Expect(config.EndpointsConfig).To(gomega.BeEmpty())
		})
	})

	ginkgo.Context("with stopped container (exited state)", func() {
		ginkgo.It("should handle MAC address validation for non-running containers", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false, Status: "exited"}),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						// No MAC address for stopped container - this is expected
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config).ToNot(gomega.BeNil())
			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("bridge"))
		})
	})

	ginkgo.Context("with custom network settings", func() {
		ginkgo.It("should preserve custom network configurations", func() {
			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"my_custom_network": {
						NetworkID:  "custom_net_id",
						MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
						IPAddress:  netip.MustParseAddr("10.0.0.5"),
						Aliases:    []string{"container_id", "custom-alias"},
						Links:      []string{"other_container:alias"},
					},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			endpoint := config.EndpointsConfig["my_custom_network"]
			gomega.Expect(endpoint.NetworkID).To(gomega.Equal("custom_net_id"))
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(
				dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
			))
			gomega.Expect(endpoint.IPAddress).To(gomega.Equal(
				netip.MustParseAddr("10.0.0.5"),
			))
			gomega.Expect(endpoint.Aliases).To(gomega.ConsistOf("custom-alias"))
			gomega.Expect(endpoint.Links).To(gomega.ConsistOf("other_container:alias"))
		})
	})

	ginkgo.Context("with Podman-style client version", func() {
		ginkgo.It("should handle Podman API version strings", func() {
			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				}),
			)

			// Test with Podman version format
			config := getNetworkConfig(testLog(), container, "4.0.0")

			endpoint := config.EndpointsConfig["bridge"]
			// Should preserve MAC for modern versions
			gomega.Expect(endpoint.MacAddress).To(gomega.Equal(
				dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
			))
		})
	})

	ginkgo.Context("with nil endpoint in network settings", func() {
		ginkgo.It("should skip nil endpoints", func() {
			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge":    nil,
					"valid_net": {NetworkID: "valid_id"},
				}),
			)

			config := getNetworkConfig(testLog(), container, "1.50")

			gomega.Expect(config.EndpointsConfig).To(gomega.HaveKey("valid_net"))
			gomega.Expect(config.EndpointsConfig).ToNot(gomega.HaveKey("bridge"))
		})
	})
})

var _ = ginkgo.Describe("newEmptyNetworkConfig", func() {
	ginkgo.It("should create a non-nil network config", func() {
		config := newEmptyNetworkConfig()
		gomega.Expect(config).ToNot(gomega.BeNil())
	})

	ginkgo.It("should initialize EndpointsConfig as an empty map", func() {
		config := newEmptyNetworkConfig()
		gomega.Expect(config.EndpointsConfig).ToNot(gomega.BeNil())
		gomega.Expect(config.EndpointsConfig).To(gomega.BeEmpty())
		gomega.Expect(config.EndpointsConfig).
			To(gomega.Equal(make(map[string]*dockerNetwork.EndpointSettings)))
	})

	ginkgo.It("should return a properly structured NetworkingConfig", func() {
		config := newEmptyNetworkConfig()
		gomega.Expect(config).To(gomega.BeAssignableToTypeOf(&dockerNetwork.NetworkingConfig{}))
		gomega.Expect(config.EndpointsConfig).
			To(gomega.BeAssignableToTypeOf(make(map[string]*dockerNetwork.EndpointSettings)))
	})

	ginkgo.It("should be ready for use - can add endpoints to the map", func() {
		config := newEmptyNetworkConfig()
		endpoint := &dockerNetwork.EndpointSettings{
			NetworkID: "test-network",
		}
		config.EndpointsConfig["test"] = endpoint
		gomega.Expect(config.EndpointsConfig).To(gomega.HaveLen(1))
		gomega.Expect(config.EndpointsConfig["test"]).To(gomega.Equal(endpoint))
	})
})

var _ = ginkgo.Describe("processEndpoint", func() {
	ginkgo.Context("with modern API version (>= 1.44)", func() {
		clientVersion := "1.50"

		ginkgo.Context("and non-host network mode", func() {
			isHostNetwork := false

			ginkgo.It("should preserve MAC address, IP address, and DNS names", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
					DNSNames:   []string{"test.example.com"},
					Aliases:    []string{"container_id", "test-alias"},
					IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
						IPv4Address: netip.MustParseAddr("172.17.0.2"),
					},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.NetworkID).To(gomega.Equal("bridge_network_id"))
				gomega.Expect(result.MacAddress).To(gomega.Equal(
					dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
				))
				gomega.Expect(result.IPAddress).To(gomega.Equal(
					netip.MustParseAddr("172.17.0.2"),
				))
				gomega.Expect(result.DNSNames).To(gomega.ConsistOf("test.example.com"))
				gomega.Expect(result.IPAMConfig).ToNot(gomega.BeNil())
				gomega.Expect(result.IPAMConfig.IPv4Address).To(gomega.Equal(
					netip.MustParseAddr("172.17.0.2"),
				))
				// Aliases should be filtered to remove container short ID
				gomega.Expect(result.Aliases).To(gomega.ConsistOf("test-alias"))
			})

			ginkgo.It("should clear engine-generated MAC when it matches the endpoint IP", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC("02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
			})

			ginkgo.It("should preserve user-configured MAC that does not match the endpoint IP", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(
					dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
				))
			})

			ginkgo.It("should preserve engine-generated MAC when endpoint IP is empty", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC("02:42:ac:11:00:02"),
					IPAddress:  netip.Addr{},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(
					mustParseMAC("02:42:ac:11:00:02"),
				))
			})

			ginkgo.It("should preserve engine-generated MAC when endpoint IP is IPv6", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC("02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("::1"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(
					mustParseMAC("02:42:ac:11:00:02"),
				))
			})

			ginkgo.It("should clear empty MAC without error", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr{},
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
			})

			ginkgo.It("should preserve user-configured MAC matching engine-generated prefix with different IP", func() {
				// A user-configured MAC that happens to start with 02:42 but whose last
				// four bytes do not match the endpoint's IP must not be cleared.
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC("02:42:de:ad:be:ef"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(
					mustParseMAC("02:42:de:ad:be:ef"),
				))
			})

			ginkgo.It("should handle multiple aliases correctly", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Aliases: []string{"container_id", "alias1", "alias2", "other_id"},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.Aliases).To(gomega.ConsistOf(
					"alias1", "alias2", "other_id",
				),
				)
			})

			ginkgo.It("should copy all IPAM config fields", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
						IPv4Address: netip.MustParseAddr("192.168.1.100"),
						IPv6Address: netip.MustParseAddr("2001:db8::1"),
						LinkLocalIPs: []netip.Addr{
							netip.MustParseAddr("169.254.1.1"),
							netip.MustParseAddr("169.254.1.2"),
						},
					},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.IPAMConfig).ToNot(gomega.BeNil())
				gomega.Expect(result.IPAMConfig.IPv4Address).To(gomega.Equal(
					netip.MustParseAddr("192.168.1.100"),
				))
				gomega.Expect(result.IPAMConfig.IPv6Address).To(gomega.Equal(
					netip.MustParseAddr("2001:db8::1"),
				))
				gomega.Expect(result.IPAMConfig.LinkLocalIPs).
					To(gomega.ConsistOf(
						netip.MustParseAddr("169.254.1.1"),
						netip.MustParseAddr("169.254.1.2"),
					))
			})

			ginkgo.It("should preserve empty aliases list", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Aliases: []string{},
				}
				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.Aliases).To(gomega.BeEmpty())
			})

			ginkgo.It("should preserve other endpoint fields", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Gateway:    netip.MustParseAddr("172.17.0.1"),
					Links:      []string{"other_container:alias"},
					EndpointID: "endpoint_id",
				}
				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.Gateway).To(gomega.Equal(
					netip.MustParseAddr("172.17.0.1"),
				))
				gomega.Expect(result.Links).To(gomega.ConsistOf("other_container:alias"))
				gomega.Expect(result.EndpointID).To(gomega.Equal("endpoint_id"))
			})
		})
	})

	ginkgo.Context("with legacy API version (< 1.44)", func() {
		clientVersion := "1.40"

		ginkgo.Context("and non-host network mode", func() {
			isHostNetwork := false

			ginkgo.It("should clear MAC address, IP address, and DNS names", func() {
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
					DNSNames:   []string{"test.example.com"},
					Aliases:    []string{"container_id", "test-alias"},
					IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
						IPv4Address: netip.MustParseAddr("172.17.0.2"),
					},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(testLog(), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				gomega.Expect(result.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
				gomega.Expect(result.IPAddress).To(gomega.Equal(netip.Addr{}))
				gomega.Expect(result.DNSNames).To(gomega.BeNil())
				gomega.Expect(result.IPAMConfig).ToNot(gomega.BeNil())
				gomega.Expect(result.IPAMConfig.IPv4Address).To(gomega.Equal(
					netip.MustParseAddr("172.17.0.2"),
				))
				// Aliases should still be filtered
				gomega.Expect(result.Aliases).To(gomega.ConsistOf("test-alias"))
			})
		})
	})

	ginkgo.Context("with host network mode", func() {
		isHostNetwork := true

		ginkgo.It("should clear aliases regardless of API version", func() {
			sourceEndpoint := &dockerNetwork.EndpointSettings{
				Aliases: []string{"container_id", "test-alias"},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(testLog(), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(result.Aliases).To(gomega.BeNil())
		})

		ginkgo.It("should clear IPAM config regardless of API version", func() {
			sourceEndpoint := &dockerNetwork.EndpointSettings{
				IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
					IPv4Address: netip.MustParseAddr("192.168.1.100"),
				},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(testLog(), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(result.IPAMConfig).To(gomega.BeNil())
		})

		ginkgo.It("should clear MAC and IP addresses even with modern API", func() {
			sourceEndpoint := &dockerNetwork.EndpointSettings{
				MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
				IPAddress:  netip.MustParseAddr("192.168.1.100"),
				DNSNames:   []string{"host.example.com"},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(testLog(), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(result.MacAddress).To(gomega.Equal(dockerNetwork.HardwareAddr{}))
			gomega.Expect(result.IPAddress).To(gomega.Equal(netip.Addr{}))
			gomega.Expect(result.DNSNames).To(gomega.BeNil())
		})
	})

	ginkgo.Context("with edge cases", func() {
		ginkgo.It("should handle nil IPAM config gracefully", func() {
			sourceEndpoint := &dockerNetwork.EndpointSettings{
				IPAMConfig: nil,
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(testLog(), sourceEndpoint,
				containerID,
				"1.50",
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(result.IPAMConfig).To(gomega.BeNil())
		})

		ginkgo.It("should handle empty DNS names", func() {
			sourceEndpoint := &dockerNetwork.EndpointSettings{
				DNSNames: []string{},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(testLog(), sourceEndpoint,
				containerID,
				"1.50",
				false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(result.DNSNames).To(gomega.BeEmpty())
		})

		ginkgo.Context("with nil source endpoint", func() {
			ginkgo.It("should return ErrNilSourceEndpoint when sourceEndpoint is nil", func() {
				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(testLog(), nil,
					containerID,
					"1.50",
					false,
				)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError(errNilSourceEndpoint))
				gomega.Expect(result).To(gomega.BeNil())
			})
		})
	})
})

var _ = ginkgo.Describe("isEngineGeneratedMAC", func() {
	ginkgo.It("should return true for MAC derived from the endpoint IPv4 address", func() {
		mac := mustParseMAC("02:42:ac:11:00:02")
		ip := netip.MustParseAddr("172.17.0.2")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeTrue())
	})

	ginkgo.It("should return true for a MAC unmarshaled from Docker inspect JSON", func() {
		var endpoint dockerNetwork.EndpointSettings

		err := json.Unmarshal(
			[]byte(`{"MacAddress":"02:42:ac:11:00:02","IPAddress":"172.17.0.2"}`),
			&endpoint,
		)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(endpoint.MacAddress).To(gomega.HaveLen(6))
		gomega.Expect(isEngineGeneratedMAC(endpoint.MacAddress, endpoint.IPAddress)).
			To(gomega.BeTrue())
	})

	ginkgo.It("should return false for colon-separated ASCII bytes that are not a parsed HardwareAddr", func() {
		mac := dockerNetwork.HardwareAddr("02:42:ac:11:00:02")
		ip := netip.MustParseAddr("172.17.0.2")

		gomega.Expect(mac).To(gomega.HaveLen(17))
		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false for user-configured MAC with matching prefix but different IP", func() {
		mac := mustParseMAC("02:42:de:ad:be:ef")
		ip := netip.MustParseAddr("172.17.0.2")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when MAC is empty", func() {
		mac := dockerNetwork.HardwareAddr{}
		ip := netip.MustParseAddr("172.17.0.2")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when IP is empty", func() {
		mac := mustParseMAC("02:42:ac:11:00:02")
		ip := netip.Addr{}

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when IP is IPv6", func() {
		mac := mustParseMAC("02:42:ac:11:00:02")
		ip := netip.MustParseAddr("::1")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when MAC length is not 6", func() {
		mac := dockerNetwork.HardwareAddr("02:42:ac:11")
		ip := netip.MustParseAddr("172.17.0.2")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeFalse())
	})

	ginkgo.It("should return true for the reproduction case from issue #2104", func() {
		mac := mustParseMAC("02:42:0a:63:00:02")
		ip := netip.MustParseAddr("10.99.0.2")

		gomega.Expect(isEngineGeneratedMAC(mac, ip)).To(gomega.BeTrue())
	})
})

var _ = ginkgo.Describe("onlyGeneratedMacs", func() {
	ginkgo.It("should return true when all original MACs are engine-generated and all endpoints are covered", func() {
		container := MockContainer(
			WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC("02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				},
			}),
		)
		processedConfig := &dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID: "bridge_network_id",
				},
			},
		}

		gomega.Expect(onlyGeneratedMacs(container, processedConfig)).To(gomega.BeTrue())
	})

	ginkgo.It("should return false when a non-nil original endpoint is missing from the processed config", func() {
		container := MockContainer(
			WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				},
				"custom": {
					NetworkID:  "custom_network_id",
					MacAddress: dockerNetwork.HardwareAddr("02:42:0a:00:00:05"),
					IPAddress:  netip.MustParseAddr("10.0.0.5"),
				},
			}),
		)
		processedConfig := &dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID: "bridge_network_id",
				},
			},
		}

		gomega.Expect(onlyGeneratedMacs(container, processedConfig)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when there is a user-configured MAC", func() {
		container := MockContainer(
			WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				},
			}),
		)
		processedConfig := &dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID: "bridge_network_id",
				},
			},
		}

		gomega.Expect(onlyGeneratedMacs(container, processedConfig)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when there are no MACs at all", func() {
		container := MockContainer(
			WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID: "bridge_network_id",
					IPAddress: netip.MustParseAddr("172.17.0.2"),
				},
			}),
		)
		processedConfig := &dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID: "bridge_network_id",
				},
			},
		}

		gomega.Expect(onlyGeneratedMacs(container, processedConfig)).To(gomega.BeFalse())
	})

	ginkgo.It("should return false when container info is nil", func() {
		processedConfig := &dockerNetwork.NetworkingConfig{}

		gomega.Expect(onlyGeneratedMacs(nil, processedConfig)).To(gomega.BeFalse())
	})
})

var _ = ginkgo.Describe("validateMacAddresses", func() {
	ginkgo.Context("with legacy API version (< 1.44)", func() {
		clientVersion := "1.40"

		ginkgo.Context("and non-host network mode", func() {
			isHostNetwork := false

			ginkgo.It("should succeed when no MAC address is present", func() {
				container := MockContainer(
					WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							NetworkID: "bridge_network_id",
							IPAddress: netip.MustParseAddr("172.17.0.2"),
						},
					}),
				)
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							NetworkID: "bridge_network_id",
							IPAddress: netip.MustParseAddr("172.17.0.2"),
						},
					},
				}

				err := validateMacAddresses(testLog(), config,
					container.ID(),
					clientVersion,
					isHostNetwork,
					container,
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
			})

			ginkgo.It("should return error when unexpected MAC address is present", func() {
				container := MockContainer(
					WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							NetworkID:  "bridge_network_id",
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
							IPAddress:  netip.MustParseAddr("172.17.0.2"),
						},
					}),
				)
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							NetworkID:  "bridge_network_id",
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
							IPAddress:  netip.MustParseAddr("172.17.0.2"),
						},
					},
				}

				err := validateMacAddresses(testLog(), config,
					container.ID(),
					clientVersion,
					isHostNetwork,
					container,
				)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring(
						"unexpected MAC address in legacy config",
					)))
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("API version 1.40")))
			})
		})
	})

	ginkgo.Context("with host network mode", func() {
		isHostNetwork := true

		ginkgo.It("should succeed when no MAC address is present", func() {
			container := MockContainer(
				WithNetworkMode("host"),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"host": {
						NetworkID: "host_network_id",
						IPAddress: netip.MustParseAddr("192.168.1.100"),
					},
				}),
			)
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"host": {
						NetworkID: "host_network_id",
						IPAddress: netip.MustParseAddr("192.168.1.100"),
					},
				},
			}

			err := validateMacAddresses(testLog(), config,
				container.ID(),
				"1.50",
				isHostNetwork,
				container,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should return error when unexpected MAC address is present", func() {
			container := MockContainer(
				WithNetworkMode("host"),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"host": {
						NetworkID:  "host_network_id",
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("192.168.1.100"),
					},
				}),
			)
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"host": {
						NetworkID:  "host_network_id",
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("192.168.1.100"),
					},
				},
			}

			err := validateMacAddresses(testLog(), config,
				container.ID(),
				"1.50",
				isHostNetwork,
				container,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(errUnexpectedMacInHost))
		})
	})

	ginkgo.Context("with modern API version (>= 1.44)", func() {
		clientVersion := "1.50"

		ginkgo.Context("and non-host network mode", func() {
			isHostNetwork := false

			ginkgo.Context("and running container", func() {
				ginkgo.It("should succeed when MAC address is present", func() {
					container := MockContainer(
						WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
						WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID:  "bridge_network_id",
								MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
								IPAddress:  netip.MustParseAddr("172.17.0.2"),
							},
						}),
					)
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID:  "bridge_network_id",
								MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
								IPAddress:  netip.MustParseAddr("172.17.0.2"),
							},
						},
					}

					err := validateMacAddresses(testLog(), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})

				ginkgo.It("should return error when MAC address is missing", func() {
					container := MockContainer(
						WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
						WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						}),
					)
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						},
					}

					err := validateMacAddresses(testLog(), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					gomega.Expect(err).To(gomega.HaveOccurred())
					gomega.Expect(err).To(gomega.MatchError(errNoMacInNonHost))
				})

				ginkgo.It("should succeed with multiple networks all having MAC addresses", func() {
					container := MockContainer(
						WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
						WithNetworks("bridge", "custom_network"),
						WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID:  "bridge_network_id",
								MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
								IPAddress:  netip.MustParseAddr("172.17.0.2"),
							},
							"custom_network": {
								NetworkID:  "custom_network_id",
								MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
								IPAddress:  netip.MustParseAddr("10.0.0.5"),
							},
						}),
					)
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID:  "bridge_network_id",
								MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
								IPAddress:  netip.MustParseAddr("172.17.0.2"),
							},
							"custom_network": {
								NetworkID:  "custom_network_id",
								MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
								IPAddress:  netip.MustParseAddr("10.0.0.5"),
							},
						},
					}

					err := validateMacAddresses(testLog(), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})

				ginkgo.It(
					"should succeed when at least one of multiple networks has MAC address",
					func() {
						container := MockContainer(
							WithContainerState(
								dockerContainer.State{Running: true, Status: "running"},
							),
							WithNetworks("bridge", "custom_network"),
							WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
								"bridge": {
									NetworkID:  "bridge_network_id",
									MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
									IPAddress:  netip.MustParseAddr("172.17.0.2"),
								},
								"custom_network": {
									NetworkID: "custom_network_id",
									IPAddress: netip.MustParseAddr("10.0.0.5"),
								},
							}),
						)
						config := &dockerNetwork.NetworkingConfig{
							EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
								"bridge": {
									NetworkID:  "bridge_network_id",
									MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
									IPAddress:  netip.MustParseAddr("172.17.0.2"),
								},
								"custom_network": {
									NetworkID: "custom_network_id",
									IPAddress: netip.MustParseAddr("10.0.0.5"),
								},
							},
						}

						err := validateMacAddresses(testLog(), config,
							container.ID(),
							clientVersion,
							isHostNetwork,
							container,
						)
						gomega.Expect(err).ToNot(gomega.HaveOccurred())
					},
				)
			})

			ginkgo.Context("and non-running container", func() {
				ginkgo.It("should succeed when MAC address is missing (created state)", func() {
					container := MockContainer(
						WithContainerState(
							dockerContainer.State{Running: false, Status: "created"},
						),
						WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						}),
					)
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						},
					}

					err := validateMacAddresses(testLog(), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})

				ginkgo.It("should succeed when MAC address is missing (exited state)", func() {
					container := MockContainer(
						WithContainerState(dockerContainer.State{Running: false, Status: "exited"}),
						WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						}),
					)
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {
								NetworkID: "bridge_network_id",
								IPAddress: netip.MustParseAddr("172.17.0.2"),
							},
						},
					}

					err := validateMacAddresses(testLog(), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})
	})

	ginkgo.Context("with edge cases", func() {
		ginkgo.It("should handle empty network configuration", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
			)
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
			}

			err := validateMacAddresses(testLog(), config,
				container.ID(),
				"1.50",
				false,
				container,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(errNoMacInNonHost))
		})

		ginkgo.It("should handle nil network configuration", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
			)

			err := validateMacAddresses(testLog(), nil,
				container.ID(),
				"1.50",
				false,
				container,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(errNoMacInNonHost))
		})
	})
})

var _ = ginkgo.Describe("filterAliases", func() {
	ginkgo.It("removes container short ID from aliases", func() {
		shortID := testShortID
		aliases := []string{testShortID, "custom-alias", "another-alias"}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.ConsistOf("custom-alias", "another-alias"))
	})

	ginkgo.It("preserves all aliases when short ID is not present", func() {
		shortID := testShortID
		aliases := []string{"custom-alias", "another-alias", "third-alias"}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.ConsistOf(
			"custom-alias", "another-alias", "third-alias",
		))
	})

	ginkgo.It("handles empty alias list", func() {
		shortID := testShortID
		aliases := []string{}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.BeEmpty())
	})

	ginkgo.It("handles nil alias list", func() {
		shortID := testShortID

		var aliases []string

		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.BeEmpty())
	})

	ginkgo.It("removes multiple instances of short ID", func() {
		shortID := testShortID
		aliases := []string{testShortID, "custom-alias", testShortID, "another-alias"}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.ConsistOf("custom-alias", "another-alias"))
	})

	ginkgo.It("returns empty list when all aliases are the short ID", func() {
		shortID := testShortID
		aliases := []string{testShortID, testShortID}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.BeEmpty())
	})

	ginkgo.It("does not remove partial matches", func() {
		shortID := testShortID
		aliases := []string{"abc123def4567", "abc123def45", testShortID, "custom-alias"}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.ConsistOf(
			"abc123def4567", "abc123def45", "custom-alias",
		))
	})

	ginkgo.It("handles case-sensitive matching", func() {
		shortID := testShortID
		aliases := []string{"ABC123DEF456", testShortID, "Abc123Def456"}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.ConsistOf("ABC123DEF456", "Abc123Def456"))
	})

	ginkgo.It("preserves order of remaining aliases", func() {
		shortID := testShortID
		aliases := []string{"first", testShortID, "second", "third", testShortID}
		result := filterAliases(aliases, shortID)
		gomega.Expect(result).To(gomega.Equal([]string{"first", "second", "third"}))
	})
})

var _ = ginkgo.Describe("StopSourceContainer", func() {
	var (
		docker     *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		docker, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		require.NoError(ginkgo.GinkgoT(), err)

		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("stopping a linked restarting container", func() {
		ginkgo.It("should log 'Stopping linked container' and succeed", func() {
			container := MockContainer()
			container.SetLinkedToRestarting(true)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			log, logbuf := captureLog(zerolog.InfoLevel)

			err := StopSourceContainer(log, context.Background(),
				docker,
				container,
				10*time.Second,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Eventually(logbuf).Should(gbytes.Say("Stopping linked container"))
		})
	})

	ginkgo.When("stopping a running container successfully", func() {
		ginkgo.It("should stop the container without error", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNoContent, nil),
				),
			)

			err := StopSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.When("stopping a container that fails", func() {
		ginkgo.It("should return an error", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusInternalServerError, `{"message": "stop failed"}`),
				),
			)

			err := StopSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to stop container"))
		})
	})

	ginkgo.When("stopping a container with timeout parameter", func() {
		ginkgo.It("should pass the timeout to the Docker API", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID
			timeout := 30 * time.Second

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					func(w http.ResponseWriter, r *http.Request) {
						// Verify the timeout is in the query parameters
						query := r.URL.Query()
						signal := query.Get("signal")
						timeoutStr := query.Get("t")

						gomega.Expect(signal).To(gomega.Equal("SIGTERM"))
						gomega.Expect(timeoutStr).To(gomega.Equal("30"))
						w.WriteHeader(http.StatusNoContent)
					},
				),
			)

			err := StopSourceContainer(testLog(), context.Background(), docker, container, timeout)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.When("stopping a container that is not running", func() {
		ginkgo.It("should not attempt to stop and return no error", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false}),
			)

			err := StopSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockServer.ReceivedRequests()).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("stopping a container that was removed between check and stop", func() {
		ginkgo.It("should return nil when Docker returns 404 NotFound", func() {
			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true}),
			)
			cid := container.ContainerInfo().ID

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"POST",
						gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", cid)),
					),
					ghttp.RespondWith(http.StatusNotFound, `{"message":"No such container"}`),
				),
			)

			err := StopSourceContainer(testLog(), context.Background(),
				docker,
				container,
				10*time.Second,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
})

var _ = ginkgo.Describe("debugLogMacAddress", func() {
	ginkgo.Context("with legacy API version (< minimum supported version)", func() {
		minSupportedVersion := "1.44"

		ginkgo.Context("and MAC address present", func() {
			ginkgo.It("should log 'Unexpected MAC address in legacy config'", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).
					Should(gbytes.Say("Unexpected MAC address in legacy config"))
			})
		})

		ginkgo.Context("and no MAC address present", func() {
			ginkgo.It("should log 'No MAC address in legacy config, Docker will handle'", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {NetworkID: "bridge_network_id"},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).
					Should(gbytes.Say("No MAC address in legacy config, Docker will handle"))
			})
		})

		ginkgo.Context("and host network mode", func() {
			ginkgo.It(
				"should log 'Unexpected MAC address in legacy config' when MAC present",
				func() {
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"host": {
								MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
							},
						},
					}
					containerID := types.ContainerID("test-container")

					log, logbuf := captureLog(zerolog.DebugLevel)

					debugLogMacAddress(log, config,
						containerID,
						"1.40",
						minSupportedVersion,
						true,
					)

					gomega.Eventually(logbuf).
						Should(gbytes.Say("Unexpected MAC address in legacy config"))
				},
			)
		})
	})

	ginkgo.Context("with API version < 1.44 and not host network", func() {
		minSupportedVersion := "1.39" // Lower than 1.40 so first case doesn't match

		ginkgo.Context("and MAC address present", func() {
			ginkgo.It("should log 'Unexpected MAC address in legacy config'", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).
					Should(gbytes.Say("Unexpected MAC address in legacy config"))
			})
		})

		ginkgo.Context("and no MAC address present", func() {
			ginkgo.It("should log 'No MAC address in legacy config, as expected'", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {NetworkID: "bridge_network_id"},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).
					Should(gbytes.Say("No MAC address in legacy config, as expected"))
			})
		})
	})

	ginkgo.Context("with modern API version (>= 1.44)", func() {
		minSupportedVersion := "1.44"

		ginkgo.Context("and MAC address present", func() {
			ginkgo.It("should log 'Verified MAC address configuration'", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.50",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).Should(gbytes.Say(
					"Verified MAC address configuration",
				))
			})

			ginkgo.It("should log MAC address details for each network", func() {
				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
						"custom_network": {
							MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := captureLog(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.50",
					minSupportedVersion,
					false,
				)

				gomega.Eventually(logbuf).Should(gbytes.Say("Found MAC address in config"))
				gomega.Eventually(logbuf).Should(gbytes.Say("network"))
				gomega.Eventually(logbuf).Should(gbytes.Say("mac_address"))
			})
		})

		ginkgo.Context("and no MAC address present", func() {
			ginkgo.Context("and non-host network mode", func() {
				ginkgo.It("should log 'No MAC address found in config'", func() {
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {NetworkID: "bridge_network_id"},
						},
					}
					containerID := types.ContainerID("test-container")

					log, logbuf := captureLog(zerolog.DebugLevel)

					debugLogMacAddress(log, config, containerID, "1.50", minSupportedVersion, false)

					gomega.Eventually(logbuf).Should(gbytes.Say(
						"No MAC address found in config",
					))
				})
			})

			ginkgo.Context("and host network mode", func() {
				ginkgo.It("should log 'No MAC address in host network mode, as expected'", func() {
					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"host": {NetworkID: "host_network_id"},
						},
					}
					containerID := types.ContainerID("test-container")

					log, logbuf := captureLog(zerolog.DebugLevel)

					debugLogMacAddress(log, config,
						containerID,
						"1.50",
						minSupportedVersion,
						true,
					)

					gomega.Eventually(logbuf).
						Should(gbytes.Say("No MAC address in host network mode, as expected"))
				})
			})
		})
	})

	ginkgo.Context("with nil network config", func() {
		ginkgo.It("should execute without panicking and log appropriate message", func() {
			containerID := types.ContainerID("test-container")

			log, logbuf := captureLog(zerolog.DebugLevel)

			gomega.Expect(func() {
				debugLogMacAddress(log, nil, containerID, "1.50", "1.44", false)
			}).ToNot(gomega.Panic())

			// Should log that no MAC address was found
			gomega.Eventually(logbuf).Should(gbytes.Say("No MAC address found in config"))
		})
	})

	ginkgo.Context("with empty endpoints config", func() {
		ginkgo.It("should log appropriate message for non-host network", func() {
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := captureLog(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.50", "1.44", false)

			gomega.Eventually(logbuf).Should(gbytes.Say("No MAC address found in config"))
		})
	})

	ginkgo.Context("with multiple networks", func() {
		ginkgo.It("should check all networks for MAC addresses", func() {
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
					"custom_network": {NetworkID: "custom_net_id"},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := captureLog(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.50", "1.44", false)

			gomega.Eventually(logbuf).Should(gbytes.Say("Found MAC address in config"))
			gomega.Eventually(logbuf).Should(gbytes.Say("Verified MAC address configuration"))
		})
	})

	ginkgo.Context("with different client version formats", func() {
		ginkgo.It("should handle Podman-style version strings", func() {
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := captureLog(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "4.0.0", "1.44", false)

			gomega.Eventually(logbuf).Should(gbytes.Say("Verified MAC address configuration"))
		})

		ginkgo.It("should handle version strings with patch levels", func() {
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := captureLog(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.45.0", "1.44", false)

			gomega.Eventually(logbuf).Should(gbytes.Say("Verified MAC address configuration"))
		})
	})
})

var _ = ginkgo.Describe("IsWatchtowerParent", func() {
	ginkgo.It("should return false for empty chain", func() {
		result := IsWatchtowerParent(types.ContainerID("test-id"), "")
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return false for empty ID", func() {
		result := IsWatchtowerParent(types.ContainerID(""), "id1,id2")
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should return true when ID is at start of chain", func() {
		result := IsWatchtowerParent(types.ContainerID("test-id"), "test-id,id2,id3")
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should return true when ID is in middle of chain", func() {
		result := IsWatchtowerParent(types.ContainerID("id2"), "id1,id2,id3")
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should return true when ID is at end of chain", func() {
		result := IsWatchtowerParent(types.ContainerID("id3"), "id1,id2,id3")
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should return true for single ID chain", func() {
		result := IsWatchtowerParent(types.ContainerID("test-id"), "test-id")
		gomega.Expect(result).To(gomega.BeTrue())
	})

	ginkgo.It("should return false when ID is not in chain", func() {
		result := IsWatchtowerParent(types.ContainerID("test-id"), "id1,id2,id3")
		gomega.Expect(result).To(gomega.BeFalse())
	})

	ginkgo.It("should handle chains with spaces around commas", func() {
		result := IsWatchtowerParent(types.ContainerID("id2"), "id1, id2 , id3")
		gomega.Expect(result).To(gomega.BeTrue())
	})
})

func getStatusFilterKeys(f dockerClient.Filters) []string {
	inner, ok := f["status"]
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(inner))
	for k := range inner {
		keys = append(keys, k)
	}

	return keys
}

func TestCloneImageInspect_IsolatesMutations(t *testing.T) {
	t.Parallel()

	src := &dockerImage.InspectResponse{
		ID:          "sha256:abc",
		RepoDigests: []string{"app@sha256:abc"},
		RepoTags:    []string{"app:latest"},
		Config: &dockerspec.DockerOCIImageConfig{
			Env:        []string{"PATH=/usr/bin"},
			Cmd:        []string{"app"},
			Entrypoint: []string{"/bin/sh"},
			Labels:     map[string]string{"app": "web"},
			Volumes:    map[string]struct{}{"/data": {}},
			ExposedPorts: map[string]struct{}{
				"80/tcp": {},
			},
			Healthcheck: &dockerspec.HealthcheckConfig{Test: []string{"CMD", "true"}},
		},
		GraphDriver: &storage.DriverData{
			Name: "overlay2",
			Data: map[string]string{"MergedDir": "/var/lib/docker/overlay2/abc"},
		},
	}

	cloned := cloneImageInspect(src)
	require.NotNil(t, cloned)
	require.NotSame(t, src, cloned)

	cloned.RepoDigests[0] = "app@sha256:mutated"
	cloned.RepoTags = append(cloned.RepoTags, "app:dev")
	cloned.Config.Env[0] = "PATH=/mutated"
	cloned.Config.Cmd[0] = "mutated"
	cloned.Config.Entrypoint[0] = "/bin/mutated"
	cloned.Config.Labels["app"] = "mutated"
	cloned.Config.Volumes["/tmp"] = struct{}{}
	delete(cloned.Config.ExposedPorts, "80/tcp")
	cloned.Config.Healthcheck.Test[0] = "NONE"
	cloned.GraphDriver.Data["MergedDir"] = "/mutated"

	require.Equal(t, []string{"app@sha256:abc"}, src.RepoDigests)
	require.Equal(t, []string{"app:latest"}, src.RepoTags)
	require.Equal(t, []string{"PATH=/usr/bin"}, src.Config.Env)
	require.Equal(t, []string{"app"}, src.Config.Cmd)
	require.Equal(t, []string{"/bin/sh"}, src.Config.Entrypoint)
	require.Equal(t, map[string]string{"app": "web"}, src.Config.Labels)
	require.Equal(t, map[string]struct{}{"/data": {}}, src.Config.Volumes)
	require.Equal(t, map[string]struct{}{"80/tcp": {}}, src.Config.ExposedPorts)
	require.Equal(t, []string{"CMD", "true"}, src.Config.Healthcheck.Test)
	require.Equal(t, "/var/lib/docker/overlay2/abc", src.GraphDriver.Data["MergedDir"])
	require.Nil(t, cloneImageInspect(nil))
}

// imageInspectFailAPI is a Docker API client that lists one running container
// and fails the inspect of its image. Methods it does not override panic.
type imageInspectFailAPI struct {
	dockerClient.APIClient

	// err is the error ImageInspect returns.
	err error
}

// ClientVersion returns a fixed API version for diagnostic logs.
func (imageInspectFailAPI) ClientVersion() string {
	return "1.44"
}

// ContainerList returns one container.
func (imageInspectFailAPI) ContainerList(
	context.Context,
	dockerClient.ContainerListOptions,
) (dockerClient.ContainerListResult, error) {
	return dockerClient.ContainerListResult{Items: []dockerContainer.Summary{{ID: testContainerID}}}, nil
}

// ContainerInspect returns a running container without links or shared namespaces.
func (imageInspectFailAPI) ContainerInspect(
	context.Context,
	string,
	dockerClient.ContainerInspectOptions,
) (dockerClient.ContainerInspectResult, error) {
	return dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
		ID:         testContainerID,
		Name:       "/app",
		Image:      "sha256:image",
		State:      &dockerContainer.State{Running: true},
		Config:     &dockerContainer.Config{Image: "app:latest"},
		HostConfig: &dockerContainer.HostConfig{},
	}}, nil
}

// ImageInspect fails with the configured error.
func (a imageInspectFailAPI) ImageInspect(
	context.Context,
	string,
	...dockerClient.ImageInspectOption,
) (dockerClient.ImageInspectResult, error) {
	return dockerClient.ImageInspectResult{}, a.err
}

// TestListSourceContainers_ImageInspectFailure verifies that a canceled or
// timed-out image inspect ends the listing, while other failures leave the
// container without image info. Ending the listing keeps an interrupted scan
// from warning about every remaining container.
func TestListSourceContainers_ImageInspectFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// inspectErr is the error the image inspect fails with.
		inspectErr error
		// wantErr is the error the listing returns. Nil expects the container
		// without image info.
		wantErr error
	}{
		{
			name:       "canceled",
			inspectErr: fmt.Errorf("request canceled: %w", context.Canceled),
			wantErr:    context.Canceled,
		},
		{
			name:       "timed out",
			inspectErr: fmt.Errorf("request timed out: %w", context.DeadlineExceeded),
			wantErr:    context.DeadlineExceeded,
		},
		{
			name:       "image not found",
			inspectErr: fmt.Errorf("no such image: %w", cerrdefs.ErrNotFound),
		},
		{
			name:       "daemon error",
			inspectErr: errors.New("daemon error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer

			log := zerolog.New(&out).Level(zerolog.WarnLevel)
			api := imageInspectFailAPI{err: tt.inspectErr}

			containers, err := ListSourceContainers(&log, t.Context(), api, ClientOptions{}, nil)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, containers)
				assert.NotContains(t, out.String(), "Failed to retrieve image info")

				return
			}

			require.NoError(t, err)
			require.Len(t, containers, 1)
			assert.False(t, containers[0].HasImageInfo())
			assert.Contains(t, out.String(), "Failed to retrieve image info")
		})
	}
}
