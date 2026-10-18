package container

import (
	"bytes"
	"context"
	"errors"
	"net/netip"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// WithCPUSettings configures CPU settings for the mock container.
func WithCPUSettings(nanoCPUs, cpuShares, cpuQuota, cpuPeriod int64, cpusetCpus, cpusetMems string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.NanoCPUs = nanoCPUs
		c.HostConfig.CPUShares = cpuShares
		c.HostConfig.CPUQuota = cpuQuota
		c.HostConfig.CPUPeriod = cpuPeriod
		c.HostConfig.CpusetCpus = cpusetCpus
		c.HostConfig.CpusetMems = cpusetMems
	}
}

var _ = ginkgo.Describe("Target Container Operations", func() {
	var (
		logOutput *bytes.Buffer
		targetLog *zerolog.Logger
	)

	ginkgo.BeforeEach(func() {
		logOutput = &bytes.Buffer{}
		w := logging.LogfmtWriter(logOutput)
		l := zerolog.New(w).Level(zerolog.DebugLevel).With().Timestamp().Logger()
		targetLog = &l
	})

	ginkgo.Describe("handleCPUSettings", func() {
		var (
			defaultNanoCPUs   int64 = 2000000000
			defaultCPUShares  int64 = 1024
			defaultCPUQuota   int64 = 100000
			defaultCPUPeriod  int64 = 100000
			defaultCpusetCpus       = "0-3"
			defaultCpusetMems       = "0"
		)

		var hostConfig *dockerContainer.HostConfig

		ginkgo.BeforeEach(func() {
			mockCont := MockContainer(
				WithCPUSettings(
					defaultNanoCPUs,
					defaultCPUShares,
					defaultCPUQuota,
					defaultCPUPeriod,
					defaultCpusetCpus,
					defaultCpusetMems,
				),
			)
			hostConfig = mockCont.GetCreateHostConfig()
		})

		ginkgo.It("should strip all CPU settings when mode is 'none'", func() {
			handleCPUSettings(hostConfig, "none", false, targetLog)
			gomega.Expect(hostConfig.NanoCPUs).To(gomega.Equal(int64(0)))
			gomega.Expect(hostConfig.CPUShares).To(gomega.Equal(int64(0)))
			gomega.Expect(hostConfig.CPUQuota).To(gomega.Equal(int64(0)))
			gomega.Expect(hostConfig.CPUPeriod).To(gomega.Equal(int64(0)))
			gomega.Expect(hostConfig.CpusetCpus).To(gomega.BeEmpty())
			gomega.Expect(hostConfig.CpusetMems).To(gomega.BeEmpty())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Stripped all CPU settings"))
		})

		ginkgo.It("should preserve all CPU settings when mode is 'full'", func() {
			handleCPUSettings(hostConfig, "full", false, targetLog)
			gomega.Expect(hostConfig.NanoCPUs).To(gomega.Equal(defaultNanoCPUs))
			gomega.Expect(hostConfig.CPUShares).To(gomega.Equal(defaultCPUShares))
			gomega.Expect(hostConfig.CPUQuota).To(gomega.Equal(defaultCPUQuota))
			gomega.Expect(hostConfig.CPUPeriod).To(gomega.Equal(defaultCPUPeriod))
			gomega.Expect(hostConfig.CpusetCpus).To(gomega.Equal(defaultCpusetCpus))
			gomega.Expect(hostConfig.CpusetMems).To(gomega.Equal(defaultCpusetMems))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Copied all CPU settings unchanged"))
		})

		ginkgo.It("should filter NanoCPUs when mode is 'auto' and isPodman is true", func() {
			handleCPUSettings(hostConfig, "auto", true, targetLog)
			gomega.Expect(hostConfig.NanoCPUs).To(gomega.Equal(int64(0)))
			gomega.Expect(hostConfig.CPUShares).To(gomega.Equal(defaultCPUShares))
			gomega.Expect(hostConfig.CPUQuota).To(gomega.Equal(defaultCPUQuota))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Detected Podman, filtered NanoCPUs for compatibility"))
		})

		ginkgo.It("should preserve all CPU settings when mode is 'auto' and isPodman is false", func() {
			handleCPUSettings(hostConfig, "auto", false, targetLog)
			gomega.Expect(hostConfig.NanoCPUs).To(gomega.Equal(defaultNanoCPUs))
			gomega.Expect(hostConfig.CPUShares).To(gomega.Equal(defaultCPUShares))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Detected Docker, copied all CPU settings"))
		})

		ginkgo.It("should default to 'full' mode for unknown CPU copy mode", func() {
			handleCPUSettings(hostConfig, "unknown", false, targetLog)
			gomega.Expect(hostConfig.NanoCPUs).To(gomega.Equal(defaultNanoCPUs))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Unknown CPU copy mode, defaulting to full"))
		})
	})

	ginkgo.Describe("creating and starting a target container", func() {
		var (
			client        *MockClient
			mockCont      *Container
			networkConfig *dockerNetwork.NetworkingConfig
			defaultMem    int64 = 60
		)

		ginkgo.BeforeEach(func() {
			client = &MockClient{}
			mockCont = MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						NetworkID:  "network_bridge_id",
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
						MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
						Aliases:    []string{"test-watchtower"},
					},
				}),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					if c.HostConfig == nil {
						c.HostConfig = &dockerContainer.HostConfig{}
					}

					c.HostConfig.MemorySwappiness = &defaultMem
				},
			)
			networkConfig = getNetworkConfig(targetLog, mockCont, "1.44")
		})

		ginkgo.It("should disable memory swappiness when flag is true", func() {
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", true, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.Equal(types.ContainerID("new_container_id")))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Disabled memory swappiness for Podman compatibility"))
		})

		ginkgo.It("should log MAC address details", func() {
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.Equal(types.ContainerID("new_container_id")))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Found MAC address in config"))
		})

		ginkgo.It("should use full network config for API >= 1.44", func() {
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.Equal(types.ContainerID("new_container_id")))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Using full network config for API version >= 1.44 or single network"))
		})

		ginkgo.It("should handle ContainerCreate failure", func() {
			createErr := errors.New("create failed")
			client.createFunc = func(_ context.Context, _ *dockerContainer.Config, _ *dockerContainer.HostConfig, _ *dockerNetwork.NetworkingConfig, _ *ocispec.Platform, _ string) (dockerClient.ContainerCreateResult, error) {
				return dockerClient.ContainerCreateResult{}, createErr
			}
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("create failed"))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to create new container"))
		})

		ginkgo.It("should log successful container creation", func() {
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.Equal(types.ContainerID("new_container_id")))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Created container successfully"))
		})

		ginkgo.It("should skip starting stopped container when reviveStopped is false", func() {
			mockCont = MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: false, Status: "exited"}),
			)
			networkConfig = getNetworkConfig(targetLog, mockCont, "1.44")
			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", false)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Created container, not starting due to stopped state"))
		})

		ginkgo.It("should handle ContainerStart failure", func() {
			startErr := errors.New("start failed")
			client.startFunc = func(_ context.Context, _ string, _ dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
				return dockerClient.ContainerStartResult{}, startErr
			}

			var removedID string

			client.removeFunc = func(_ context.Context, id string, _ dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error) {
				removedID = id

				client.removeFuncCalled.Store(true)

				return dockerClient.ContainerRemoveResult{}, nil
			}
			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("start failed"))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to start new container"))
			gomega.Expect(client.inspectFuncCalled.Load()).To(gomega.BeTrue())
			gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeTrue())
			gomega.Expect(removedID).To(gomega.Equal("new_container_id"))
		})

		ginkgo.It("should skip cleanup when container is running after start timeout", func() {
			client.startFunc = func(_ context.Context, _ string, _ dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
				return dockerClient.ContainerStartResult{}, context.DeadlineExceeded
			}
			client.inspectFunc = func(_ context.Context, id string, _ dockerClient.ContainerInspectOptions) (dockerClient.ContainerInspectResult, error) {
				return dockerClient.ContainerInspectResult{
					Container: dockerContainer.InspectResponse{
						ID: id,
						State: &dockerContainer.State{
							Status:  "running",
							Running: true,
						},
					},
				}, nil
			}

			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, context.DeadlineExceeded)).To(gomega.BeTrue())
			gomega.Expect(client.inspectFuncCalled.Load()).To(gomega.BeTrue())
			gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeFalse())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Skipped cleanup of running container after start error"))
		})

		ginkgo.It("should skip cleanup when inspect fails after transport error", func() {
			client.startFunc = func(_ context.Context, _ string, _ dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
				return dockerClient.ContainerStartResult{}, errors.New("connection reset by peer")
			}
			client.inspectFunc = func(_ context.Context, _ string, _ dockerClient.ContainerInspectOptions) (dockerClient.ContainerInspectResult, error) {
				return dockerClient.ContainerInspectResult{}, errors.New("inspect transport failure")
			}

			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("connection reset by peer"))
			gomega.Expect(client.inspectFuncCalled.Load()).To(gomega.BeTrue())
			gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeFalse())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to inspect container after start error"))
		})

		ginkgo.It("should handle ContainerCreate Conflict error", func() {
			client.createFunc = func(_ context.Context, _ *dockerContainer.Config, _ *dockerContainer.HostConfig, _ *dockerNetwork.NetworkingConfig, _ *ocispec.Platform, _ string) (dockerClient.ContainerCreateResult, error) {
				return dockerClient.ContainerCreateResult{}, cerrdefs.ErrConflict
			}
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to create container"))
			gomega.Expect(cerrdefs.IsConflict(err)).To(gomega.BeTrue(),
				"wrapped error should be detectable via cerrdefs.IsConflict")
			gomega.Expect(errors.Is(err, cerrdefs.ErrConflict)).To(gomega.BeTrue(),
				"wrapped error should be detectable via errors.Is")
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to create new container"))
		})

		ginkgo.It("should handle ContainerCreate InvalidArgument error", func() {
			client.createFunc = func(_ context.Context, _ *dockerContainer.Config, _ *dockerContainer.HostConfig, _ *dockerNetwork.NetworkingConfig, _ *ocispec.Platform, _ string) (dockerClient.ContainerCreateResult, error) {
				return dockerClient.ContainerCreateResult{}, cerrdefs.ErrInvalidArgument
			}
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.44", false, "auto", false,
			)
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to create container"))
		})

		ginkgo.It("should handle ContainerStart Conflict error", func() {
			client.startFunc = func(_ context.Context, _ string, _ dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
				return dockerClient.ContainerStartResult{}, cerrdefs.ErrConflict
			}
			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to start container"))
			gomega.Expect(cerrdefs.IsConflict(err)).To(gomega.BeTrue(),
				"wrapped error should be detectable via cerrdefs.IsConflict")
			gomega.Expect(errors.Is(err, cerrdefs.ErrConflict)).To(gomega.BeTrue(),
				"wrapped error should be detectable via errors.Is")
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to start new container"))
			gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeTrue())
		})

		ginkgo.It("should handle ContainerStart NotFound error", func() {
			client.startFunc = func(_ context.Context, _ string, _ dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error) {
				return dockerClient.ContainerStartResult{}, cerrdefs.ErrNotFound
			}
			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to start container"))
			gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeTrue())
		})

		ginkgo.It("should log successful container start", func() {
			err := startTargetContainer(context.Background(), client, targetLog, mockCont, "new_container_id", true)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Started new container"))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("new_id=" + types.ContainerID("new_container_id").ShortID()))
		})

		ginkgo.It("should attach multiple networks for legacy API and handle success", func() {
			mockCont = MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworks("network1", "network2"),
			)
			networkConfig = getNetworkConfig(targetLog, mockCont, "1.23")
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.23", false, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(newID).To(gomega.Equal(types.ContainerID("new_container_id")))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Selected first network for container creation"))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Attaching additional network to container"))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Successfully attached additional network"))
		})

		ginkgo.It("should attach multiple networks for legacy API and handle failure", func() {
			mockCont = MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworks("network1", "network2"),
			)
			networkConfig = getNetworkConfig(targetLog, mockCont, "1.23")
			connectErr := errors.New("network connect failed")
			client.connectFunc = func(_ context.Context, _, _ string, _ *dockerNetwork.EndpointSettings) (dockerClient.NetworkConnectResult, error) {
				return dockerClient.NetworkConnectResult{}, connectErr
			}
			client.removeFunc = func(_ context.Context, _ string, _ dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error) {
				return dockerClient.ContainerRemoveResult{}, nil
			}
			newID, err := createTargetContainer(targetLog, context.Background(), client, mockCont, networkConfig,
				"1.23", false, "auto", false,
			)
			gomega.Expect(newID).To(gomega.BeEmpty())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("network connect failed"))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Failed to attach additional network"))
		})

		ginkgo.Describe("Context Propagation for Cleanup", func() {
			ginkgo.It("should perform cleanup with non-canceled context after rename failure", func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				client.renameFunc = func(_ context.Context, _, _ string) (dockerClient.ContainerRenameResult, error) {
					return dockerClient.ContainerRenameResult{}, errors.New("rename failed")
				}

				client.removeFunc = func(cleanupCtx context.Context, _ string, _ dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error) {
					client.removeFuncCalled.Store(true)

					select {
					case <-cleanupCtx.Done():
						ginkgo.Fail("Cleanup context should not be canceled")
					default:
					}

					return dockerClient.ContainerRemoveResult{}, nil
				}

				newID, err := createTargetContainer(targetLog, ctx, client, mockCont, networkConfig,
					"1.44", false, "auto", false,
				)
				gomega.Expect(newID).To(gomega.BeEmpty())
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("rename failed"))
				gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeTrue())
			})

			ginkgo.It("should perform cleanup with non-canceled context after network attachment failure", func() {
				mockCont = MockContainer(
					WithNetworkMode("bridge"),
					WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
					WithNetworks("network1", "network2"),
				)
				networkConfig = getNetworkConfig(targetLog, mockCont, "1.23")

				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				client.connectFunc = func(_ context.Context, _, _ string, _ *dockerNetwork.EndpointSettings) (dockerClient.NetworkConnectResult, error) {
					return dockerClient.NetworkConnectResult{}, errors.New("network connect failed")
				}

				client.removeFunc = func(cleanupCtx context.Context, _ string, _ dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error) {
					client.removeFuncCalled.Store(true)

					select {
					case <-cleanupCtx.Done():
						ginkgo.Fail("Cleanup context should not be canceled")
					default:
					}

					return dockerClient.ContainerRemoveResult{}, nil
				}

				newID, err := createTargetContainer(targetLog, ctx, client, mockCont, networkConfig,
					"1.23", false, "auto", false,
				)
				gomega.Expect(newID).To(gomega.BeEmpty())
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("network connect failed"))
				gomega.Expect(client.removeFuncCalled.Load()).To(gomega.BeTrue())
			})
		})
	})

	ginkgo.Describe("renameTargetContainer", func() {
		var client *MockClient

		ginkgo.BeforeEach(func() {
			client = &MockClient{}
		})

		ginkgo.It("should rename container successfully", func() {
			mockCont := MockContainer(WithName("old-name"), WithID("test-id"))
			err := renameTargetContainer(targetLog, context.Background(), client, mockCont, "new-name")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Renamed container successfully"))
		})

		ginkgo.It("should return error when rename fails", func() {
			client.renameFunc = func(_ context.Context, _, _ string) (dockerClient.ContainerRenameResult, error) {
				return dockerClient.ContainerRenameResult{}, errors.New("rename failed")
			}
			mockCont := MockContainer(WithName("old-name"), WithID("test-id"))
			err := renameTargetContainer(targetLog, context.Background(), client, mockCont, "new-name")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("rename failed"))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to rename container"))
		})
	})

	ginkgo.Describe("attachNetworks", func() {
		var client *MockClient

		ginkgo.BeforeEach(func() {
			client = &MockClient{}
		})

		ginkgo.It("should attach additional networks successfully", func() {
			fullConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
					"network2": {NetworkID: "net2"},
				},
			}
			initialConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
				},
			}

			err := attachNetworks(context.Background(), client, "container-id", fullConfig, initialConfig, targetLog)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Attaching additional network to container"))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Successfully attached additional network"))
		})

		ginkgo.It("should return error when network connect fails", func() {
			client.connectFunc = func(_ context.Context, _, _ string, _ *dockerNetwork.EndpointSettings) (dockerClient.NetworkConnectResult, error) {
				return dockerClient.NetworkConnectResult{}, errors.New("connect failed")
			}
			fullConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
					"network2": {NetworkID: "net2"},
				},
			}
			initialConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
				},
			}

			err := attachNetworks(context.Background(), client, "container-id", fullConfig, initialConfig, targetLog)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to attach network"))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Failed to attach additional network"))
		})

		ginkgo.It("should skip initial network and only attach additional ones", func() {
			fullConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
				},
			}
			initialConfig := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"network1": {NetworkID: "net1"},
				},
			}

			err := attachNetworks(context.Background(), client, "container-id", fullConfig, initialConfig, targetLog)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(logOutput.String()).ToNot(gomega.ContainSubstring("Attaching additional network"))
		})
	})

	ginkgo.Describe("syncComposeImageLabel", func() {
		const (
			oldImageID = "sha256:old-index"
			newImageID = "sha256:new-index"
			oldAmd64   = "sha256:old-amd64"
			newAmd64   = "sha256:new-amd64"
			imageRef   = "example/app:nightly"
		)

		var (
			client        *MockClient
			networkConfig *dockerNetwork.NetworkingConfig
			createdConfig *dockerContainer.Config
			inspectedRefs []string
		)

		// amd64Manifest builds an available linux/amd64 image manifest summary.
		amd64Manifest := func(id string) dockerImage.ManifestSummary {
			return dockerImage.ManifestSummary{
				ID:        id,
				Kind:      dockerImage.ManifestKindImage,
				Available: true,
				ImageData: &dockerImage.ImageProperties{
					Platform: ocispec.Platform{OS: "linux", Architecture: "amd64"},
				},
			}
		}

		// composeContainer builds a source container running oldImageID with
		// the given Compose image label.
		composeContainer := func(label string, extra ...MockContainerUpdate) *Container {
			updates := append([]MockContainerUpdate{
				WithImageName(imageRef),
				WithLabels(map[string]string{
					"com.docker.compose.project": "stack",
					"com.docker.compose.image":   label,
				}),
				func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					c.Image = oldImageID
				},
			}, extra...)

			return MockContainer(updates...)
		}

		// create runs createTargetContainer at the given API version and
		// returns the label value sent to ContainerCreate.
		create := func(source *Container, clientVersion string) string {
			_, err := createTargetContainer(targetLog, context.Background(), client, source, networkConfig,
				clientVersion, false, "auto", false,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(createdConfig).ToNot(gomega.BeNil())

			return createdConfig.Labels["com.docker.compose.image"]
		}

		ginkgo.BeforeEach(func() {
			client = &MockClient{}
			createdConfig = nil
			inspectedRefs = nil
			networkConfig = &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
			}
			client.createFunc = func(_ context.Context, config *dockerContainer.Config, _ *dockerContainer.HostConfig, _ *dockerNetwork.NetworkingConfig, _ *ocispec.Platform, _ string) (dockerClient.ContainerCreateResult, error) {
				createdConfig = config

				return dockerClient.ContainerCreateResult{ID: "new_container_id"}, nil
			}

			images := map[string]dockerImage.InspectResponse{
				imageRef: {
					ID:        newImageID,
					Manifests: []dockerImage.ManifestSummary{amd64Manifest(newAmd64)},
				},
				oldImageID: {
					ID:        oldImageID,
					Manifests: []dockerImage.ManifestSummary{amd64Manifest(oldAmd64)},
				},
			}
			client.imageInspectFunc = func(_ context.Context, ref string, withManifests bool) (dockerClient.ImageInspectResult, error) {
				inspectedRefs = append(inspectedRefs, ref)

				image, ok := images[ref]
				if !ok {
					return dockerClient.ImageInspectResult{}, cerrdefs.ErrNotFound
				}

				if !withManifests {
					image.Manifests = nil
				}

				return dockerClient.ImageInspectResult{InspectResponse: image}, nil
			}
		})

		ginkgo.It("should leave containers without the label untouched", func() {
			source := MockContainer(WithImageName(imageRef))

			gomega.Expect(create(source, "1.48")).To(gomega.BeEmpty())
			gomega.Expect(createdConfig.Labels).ToNot(gomega.HaveKey("com.docker.compose.image"))
			gomega.Expect(inspectedRefs).To(gomega.BeEmpty())
		})

		ginkgo.It("should create the container with the label pointing at the new image", func() {
			gomega.Expect(create(composeContainer(oldImageID), "1.44")).To(gomega.Equal(newImageID))
			gomega.Expect(logOutput.String()).To(gomega.ContainSubstring("Updated Compose image label"))
		})

		ginkgo.It("should pass the client API version through for manifest digest labels", func() {
			gomega.Expect(create(composeContainer(oldAmd64), "1.48")).To(gomega.Equal(newAmd64))
			gomega.Expect(create(composeContainer(oldAmd64), "1.47")).To(gomega.Equal(oldAmd64))
		})

		ginkgo.It("should keep a label with no matching value for the new image", func() {
			gomega.Expect(create(composeContainer("sha256:unknown"), "1.48")).To(gomega.Equal("sha256:unknown"))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Found no matching value for Compose image label, keeping it unchanged"))
		})

		ginkgo.It("should keep the label and still create the container when the image inspect fails", func() {
			client.imageInspectFunc = func(context.Context, string, bool) (dockerClient.ImageInspectResult, error) {
				return dockerClient.ImageInspectResult{}, cerrdefs.ErrNotFound
			}

			gomega.Expect(create(composeContainer(oldImageID), "1.48")).To(gomega.Equal(oldImageID))
			gomega.Expect(logOutput.String()).
				To(gomega.ContainSubstring("Failed to resolve Compose image label, keeping it unchanged"))
		})

		ginkgo.It("should not modify the source container's labels", func() {
			source := composeContainer(oldImageID, func(_ *dockerContainer.InspectResponse, i *dockerImage.InspectResponse) {
				i.Config = nil
			})

			gomega.Expect(create(source, "1.48")).To(gomega.Equal(newImageID))
			gomega.Expect(source.ContainerInfo().Config.Labels).
				To(gomega.HaveKeyWithValue("com.docker.compose.image", oldImageID))
		})
	})
})
