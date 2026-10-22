package container

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// testTargetID is the ID the mocked daemon gives a created target container.
const testTargetID = "new_container_id"

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

// expectCreateAndRename expects the target container to be created as
// testTargetID and renamed to the source container's name.
//
// Parameters:
//   - api: Mocked Docker API.
//   - source: Source container whose name the target takes.
func expectCreateAndRename(api *mockContainer.MockAPIClient, source *Container) {
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
		Return(dockerClient.ContainerCreateResult{ID: testTargetID}, nil).Once()
	api.EXPECT().
		ContainerRename(mock.Anything, testTargetID, dockerClient.ContainerRenameOptions{NewName: source.Name()}).
		Return(dockerClient.ContainerRenameResult{}, nil).Once()
}

// expectForceRemove expects the target container to be force-removed with a
// context that is not canceled, so cleanup survives a canceled parent.
//
// Parameters:
//   - api: Mocked Docker API.
func expectForceRemove(api *mockContainer.MockAPIClient) {
	api.EXPECT().
		ContainerRemove(
			mock.MatchedBy(func(ctx context.Context) bool { return ctx.Err() == nil }),
			testTargetID,
			dockerClient.ContainerRemoveOptions{Force: true},
		).
		Return(dockerClient.ContainerRemoveResult{}, nil).Once()
}

// expectInspectState expects the cleanup inspect of the target container and
// returns it in the given state.
//
// Parameters:
//   - api: Mocked Docker API.
//   - running: Whether the target container is running.
func expectInspectState(api *mockContainer.MockAPIClient, running bool) {
	status := dockerContainer.StateCreated
	if running {
		status = dockerContainer.StateRunning
	}

	api.EXPECT().ContainerInspect(mock.Anything, testTargetID, dockerClient.ContainerInspectOptions{}).
		Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
			ID:    testTargetID,
			State: &dockerContainer.State{Status: status, Running: running},
		}}, nil).Once()
}

// TestTargetContainerOperations verifies how replacement containers are
// created, renamed, started, and attached to networks, how failures clean up
// the created container, and how CPU settings and the Compose image label are
// carried over from the source container.
func TestTargetContainerOperations(t *testing.T) {
	t.Parallel()

	t.Run("handleCPUSettings", func(t *testing.T) {
		t.Parallel()

		var (
			defaultNanoCPUs   int64 = 2000000000
			defaultCPUShares  int64 = 1024
			defaultCPUQuota   int64 = 100000
			defaultCPUPeriod  int64 = 100000
			defaultCpusetCpus       = "0-3"
			defaultCpusetMems       = "0"
		)

		// newHostConfig returns the create host config of a container with
		// every CPU setting populated.
		newHostConfig := func() *dockerContainer.HostConfig {
			return MockContainer(
				WithCPUSettings(
					defaultNanoCPUs,
					defaultCPUShares,
					defaultCPUQuota,
					defaultCPUPeriod,
					defaultCpusetCpus,
					defaultCpusetMems,
				),
			).GetCreateHostConfig()
		}

		t.Run("should strip all CPU settings when mode is 'none'", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			hostConfig := newHostConfig()

			handleCPUSettings(hostConfig, "none", false, log)
			assert.Equal(t, int64(0), hostConfig.NanoCPUs)
			assert.Equal(t, int64(0), hostConfig.CPUShares)
			assert.Equal(t, int64(0), hostConfig.CPUQuota)
			assert.Equal(t, int64(0), hostConfig.CPUPeriod)
			assert.Empty(t, hostConfig.CpusetCpus)
			assert.Empty(t, hostConfig.CpusetMems)
			assert.Contains(t, logOutput.String(), "Stripped all CPU settings")
		})

		t.Run("should preserve all CPU settings when mode is 'full'", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			hostConfig := newHostConfig()

			handleCPUSettings(hostConfig, "full", false, log)
			assert.Equal(t, defaultNanoCPUs, hostConfig.NanoCPUs)
			assert.Equal(t, defaultCPUShares, hostConfig.CPUShares)
			assert.Equal(t, defaultCPUQuota, hostConfig.CPUQuota)
			assert.Equal(t, defaultCPUPeriod, hostConfig.CPUPeriod)
			assert.Equal(t, defaultCpusetCpus, hostConfig.CpusetCpus)
			assert.Equal(t, defaultCpusetMems, hostConfig.CpusetMems)
			assert.Contains(t, logOutput.String(), "Copied all CPU settings unchanged")
		})

		t.Run("should filter NanoCPUs when mode is 'auto' and isPodman is true", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			hostConfig := newHostConfig()

			handleCPUSettings(hostConfig, "auto", true, log)
			assert.Equal(t, int64(0), hostConfig.NanoCPUs)
			assert.Equal(t, defaultCPUShares, hostConfig.CPUShares)
			assert.Equal(t, defaultCPUQuota, hostConfig.CPUQuota)
			assert.Contains(t, logOutput.String(), "Detected Podman, filtered NanoCPUs for compatibility")
		})

		t.Run("should preserve all CPU settings when mode is 'auto' and isPodman is false", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			hostConfig := newHostConfig()

			handleCPUSettings(hostConfig, "auto", false, log)
			assert.Equal(t, defaultNanoCPUs, hostConfig.NanoCPUs)
			assert.Equal(t, defaultCPUShares, hostConfig.CPUShares)
			assert.Contains(t, logOutput.String(), "Detected Docker, copied all CPU settings")
		})

		t.Run("should default to 'full' mode for unknown CPU copy mode", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			hostConfig := newHostConfig()

			handleCPUSettings(hostConfig, "unknown", false, log)
			assert.Equal(t, defaultNanoCPUs, hostConfig.NanoCPUs)
			assert.Contains(t, logOutput.String(), "Unknown CPU copy mode, defaulting to full")
		})
	})

	t.Run("creating and starting a target container", func(t *testing.T) {
		t.Parallel()

		var defaultMem int64 = 60

		// newRunningSource returns a running source container on the bridge
		// network with a MAC address and a memory swappiness setting.
		newRunningSource := func() *Container {
			return MockContainer(
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
		}

		// newMultiNetworkSource returns a running source container attached
		// to two networks.
		newMultiNetworkSource := func() *Container {
			return MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworks("network1", "network2"),
			)
		}

		t.Run("should disable memory swappiness when flag is true", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)

			var created dockerClient.ContainerCreateOptions

			api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, options dockerClient.ContainerCreateOptions) (dockerClient.ContainerCreateResult, error) {
					created = options

					return dockerClient.ContainerCreateResult{ID: testTargetID}, nil
				},
			).Once()
			api.EXPECT().
				ContainerRename(mock.Anything, testTargetID, dockerClient.ContainerRenameOptions{NewName: source.Name()}).
				Return(dockerClient.ContainerRenameResult{}, nil).Once()

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", true, "auto", false,
			)
			require.NoError(t, err)
			assert.Equal(t, types.ContainerID(testTargetID), newID)
			require.NotNil(t, created.HostConfig)
			assert.Nil(t, created.HostConfig.MemorySwappiness)
			assert.Contains(t, logOutput.String(), "Disabled memory swappiness for Podman compatibility")
		})

		t.Run("should log MAC address details", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			expectCreateAndRename(api, source)

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			require.NoError(t, err)
			assert.Equal(t, types.ContainerID(testTargetID), newID)
			assert.Contains(t, logOutput.String(), "Found MAC address in config")
		})

		t.Run("should use full network config for API >= 1.44", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			expectCreateAndRename(api, source)

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			require.NoError(t, err)
			assert.Equal(t, types.ContainerID(testTargetID), newID)
			assert.Contains(t, logOutput.String(),
				"Using full network config for API version >= 1.44 or single network")
		})

		t.Run("should handle ContainerCreate failure", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerCreateResult{}, errors.New("create failed")).Once()

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			assert.Empty(t, newID)
			require.ErrorContains(t, err, "create failed")
			assert.Contains(t, logOutput.String(), "Failed to create new container")
		})

		t.Run("should log successful container creation", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			expectCreateAndRename(api, source)

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			require.NoError(t, err)
			assert.Equal(t, types.ContainerID(testTargetID), newID)
			assert.Contains(t, logOutput.String(), "Created container successfully")
		})

		t.Run("should skip starting stopped container when reviveStopped is false", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: false, Status: "exited"}),
			)

			err := startTargetContainer(t.Context(), mockContainer.NewMockAPIClient(t), log, source, testTargetID, false)
			require.NoError(t, err)
			assert.Contains(t, logOutput.String(), "Created container, not starting due to stopped state")
		})

		t.Run("should handle ContainerStart failure", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, errors.New("start failed")).Once()
			expectInspectState(api, false)
			expectForceRemove(api)

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.ErrorContains(t, err, "start failed")
			assert.Contains(t, logOutput.String(), "Failed to start new container")
		})

		t.Run("should skip cleanup when container is running after start timeout", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, context.DeadlineExceeded).Once()
			expectInspectState(api, true)

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			assert.Contains(t, logOutput.String(), "Skipped cleanup of running container after start error")
		})

		t.Run("should skip cleanup when inspect fails after transport error", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, errors.New("connection reset by peer")).Once()
			api.EXPECT().ContainerInspect(mock.Anything, testTargetID, dockerClient.ContainerInspectOptions{}).
				Return(dockerClient.ContainerInspectResult{}, errors.New("inspect transport failure")).Once()

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.ErrorContains(t, err, "connection reset by peer")
			assert.Contains(t, logOutput.String(), "Failed to inspect container after start error")
		})

		t.Run("should handle ContainerCreate Conflict error", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerCreateResult{}, cerrdefs.ErrConflict).Once()

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			assert.Empty(t, newID)
			require.ErrorContains(t, err, "failed to create container")
			assert.True(t, cerrdefs.IsConflict(err), "wrapped error should be detectable via cerrdefs.IsConflict")
			require.ErrorIs(t, err, cerrdefs.ErrConflict, "wrapped error should be detectable via errors.Is")
			assert.Contains(t, logOutput.String(), "Failed to create new container")
		})

		t.Run("should handle ContainerCreate InvalidArgument error", func(t *testing.T) {
			t.Parallel()

			log, _ := newCapturedLogger(zerolog.DebugLevel)
			source := newRunningSource()
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerCreateResult{}, cerrdefs.ErrInvalidArgument).Once()

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.44"),
				"1.44", false, "auto", false,
			)
			assert.Empty(t, newID)
			require.ErrorContains(t, err, "failed to create container")
		})

		t.Run("should handle ContainerStart Conflict error", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, cerrdefs.ErrConflict).Once()
			expectInspectState(api, false)
			expectForceRemove(api)

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.ErrorContains(t, err, "failed to start container")
			assert.True(t, cerrdefs.IsConflict(err), "wrapped error should be detectable via cerrdefs.IsConflict")
			require.ErrorIs(t, err, cerrdefs.ErrConflict, "wrapped error should be detectable via errors.Is")
			assert.Contains(t, logOutput.String(), "Failed to start new container")
		})

		t.Run("should handle ContainerStart NotFound error", func(t *testing.T) {
			t.Parallel()

			log, _ := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, cerrdefs.ErrNotFound).Once()
			expectInspectState(api, false)
			expectForceRemove(api)

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.ErrorContains(t, err, "failed to start container")
		})

		t.Run("should log successful container start", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerStart(mock.Anything, testTargetID, dockerClient.ContainerStartOptions{}).
				Return(dockerClient.ContainerStartResult{}, nil).Once()

			err := startTargetContainer(t.Context(), api, log, newRunningSource(), testTargetID, true)
			require.NoError(t, err)
			assert.Contains(t, logOutput.String(), "Started new container")
			assert.Contains(t, logOutput.String(), "new_id="+types.ContainerID(testTargetID).ShortID())
		})

		t.Run("should attach multiple networks for legacy API and handle success", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newMultiNetworkSource()
			api := mockContainer.NewMockAPIClient(t)
			expectCreateAndRename(api, source)
			api.EXPECT().NetworkConnect(mock.Anything, mock.Anything, mock.MatchedBy(
				func(options dockerClient.NetworkConnectOptions) bool { return options.Container == testTargetID },
			)).Return(dockerClient.NetworkConnectResult{}, nil).Once()

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.23"),
				"1.23", false, "auto", false,
			)
			require.NoError(t, err)
			assert.Equal(t, types.ContainerID(testTargetID), newID)
			assert.Contains(t, logOutput.String(), "Selected first network for container creation")
			assert.Contains(t, logOutput.String(), "Attaching additional network to container")
			assert.Contains(t, logOutput.String(), "Successfully attached additional network")
		})

		t.Run("should attach multiple networks for legacy API and handle failure", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			source := newMultiNetworkSource()
			api := mockContainer.NewMockAPIClient(t)
			expectCreateAndRename(api, source)
			api.EXPECT().NetworkConnect(mock.Anything, mock.Anything, mock.Anything).
				Return(dockerClient.NetworkConnectResult{}, errors.New("network connect failed")).Once()
			expectForceRemove(api)

			newID, err := createTargetContainer(log, t.Context(), api, source, getNetworkConfig(log, source, "1.23"),
				"1.23", false, "auto", false,
			)
			assert.Empty(t, newID)
			require.ErrorContains(t, err, "network connect failed")
			assert.Contains(t, logOutput.String(), "Failed to attach additional network")
		})

		t.Run("Context Propagation for Cleanup", func(t *testing.T) {
			t.Parallel()

			t.Run("should perform cleanup with non-canceled context after rename failure", func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				log, _ := newCapturedLogger(zerolog.DebugLevel)
				source := newRunningSource()
				api := mockContainer.NewMockAPIClient(t)
				api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
					Return(dockerClient.ContainerCreateResult{ID: testTargetID}, nil).Once()
				api.EXPECT().ContainerRename(mock.Anything, testTargetID, mock.Anything).
					Return(dockerClient.ContainerRenameResult{}, errors.New("rename failed")).Once()
				expectForceRemove(api)

				newID, err := createTargetContainer(log, ctx, api, source, getNetworkConfig(log, source, "1.44"),
					"1.44", false, "auto", false,
				)
				assert.Empty(t, newID)
				require.ErrorContains(t, err, "rename failed")
			})

			t.Run("should perform cleanup with non-canceled context after network attachment failure", func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				log, _ := newCapturedLogger(zerolog.DebugLevel)
				source := newMultiNetworkSource()
				api := mockContainer.NewMockAPIClient(t)
				expectCreateAndRename(api, source)
				api.EXPECT().NetworkConnect(mock.Anything, mock.Anything, mock.Anything).
					Return(dockerClient.NetworkConnectResult{}, errors.New("network connect failed")).Once()
				expectForceRemove(api)

				newID, err := createTargetContainer(log, ctx, api, source, getNetworkConfig(log, source, "1.23"),
					"1.23", false, "auto", false,
				)
				assert.Empty(t, newID)
				require.ErrorContains(t, err, "network connect failed")
			})
		})
	})

	t.Run("renameTargetContainer", func(t *testing.T) {
		t.Parallel()

		t.Run("should rename container successfully", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerRename(mock.Anything, "test-id", dockerClient.ContainerRenameOptions{NewName: "new-name"}).
				Return(dockerClient.ContainerRenameResult{}, nil).Once()

			err := renameTargetContainer(log, t.Context(), api, MockContainer(WithName("old-name"), WithID("test-id")), "new-name")
			require.NoError(t, err)
			assert.Contains(t, logOutput.String(), "Renamed container successfully")
		})

		t.Run("should return error when rename fails", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().ContainerRename(mock.Anything, "test-id", dockerClient.ContainerRenameOptions{NewName: "new-name"}).
				Return(dockerClient.ContainerRenameResult{}, errors.New("rename failed")).Once()

			err := renameTargetContainer(log, t.Context(), api, MockContainer(WithName("old-name"), WithID("test-id")), "new-name")
			require.ErrorContains(t, err, "rename failed")
			assert.Contains(t, logOutput.String(), "Failed to rename container")
		})
	})

	t.Run("attachNetworks", func(t *testing.T) {
		t.Parallel()

		// networks returns a networking config with an endpoint for each
		// named network, whose ID is the name with a "-id" suffix.
		networks := func(names ...string) *dockerNetwork.NetworkingConfig {
			config := &dockerNetwork.NetworkingConfig{EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{}}
			for _, name := range names {
				config.EndpointsConfig[name] = &dockerNetwork.EndpointSettings{NetworkID: name + "-id"}
			}

			return config
		}

		t.Run("should attach additional networks successfully", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().NetworkConnect(mock.Anything, "network2", dockerClient.NetworkConnectOptions{
				Container:      "container-id",
				EndpointConfig: &dockerNetwork.EndpointSettings{NetworkID: "network2-id"},
			}).Return(dockerClient.NetworkConnectResult{}, nil).Once()

			err := attachNetworks(t.Context(), api, "container-id", networks("network1", "network2"), networks("network1"), log)
			require.NoError(t, err)
			assert.Contains(t, logOutput.String(), "Attaching additional network to container")
			assert.Contains(t, logOutput.String(), "Successfully attached additional network")
		})

		t.Run("should return error when network connect fails", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)
			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().NetworkConnect(mock.Anything, "network2", mock.Anything).
				Return(dockerClient.NetworkConnectResult{}, errors.New("connect failed")).Once()

			err := attachNetworks(t.Context(), api, "container-id", networks("network1", "network2"), networks("network1"), log)
			require.ErrorContains(t, err, "failed to attach network")
			assert.Contains(t, logOutput.String(), "Failed to attach additional network")
		})

		t.Run("should skip initial network and only attach additional ones", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)

			err := attachNetworks(t.Context(), mockContainer.NewMockAPIClient(t), "container-id",
				networks("network1"), networks("network1"), log)
			require.NoError(t, err)
			assert.NotContains(t, logOutput.String(), "Attaching additional network")
		})
	})

	t.Run("syncComposeImageLabel", func(t *testing.T) {
		t.Parallel()

		const (
			oldImageID = "sha256:old-index"
			newImageID = "sha256:new-index"
			oldAmd64   = "sha256:old-amd64"
			newAmd64   = "sha256:new-amd64"
			imageRef   = "example/app:nightly"
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

		// labelAPI is a mocked Docker API that records the config sent to
		// ContainerCreate and the image references inspected.
		type labelAPI struct {
			*mockContainer.MockAPIClient

			// created is the config the target container was created with.
			created *dockerContainer.Config
			// inspected lists the image references inspected, in order.
			inspected []string
		}

		// newLabelAPI returns a labelAPI that creates and renames any target
		// container and serves the old and new images, with their manifests
		// when the inspect asks for them. inspectErr, when set, fails every
		// image inspect instead.
		newLabelAPI := func(t *testing.T, inspectErr error) *labelAPI {
			t.Helper()

			api := &labelAPI{MockAPIClient: mockContainer.NewMockAPIClient(t)}
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
			inspect := func(_ context.Context, ref string, options ...dockerClient.ImageInspectOption) (dockerClient.ImageInspectResult, error) {
				api.inspected = append(api.inspected, ref)

				if inspectErr != nil {
					return dockerClient.ImageInspectResult{}, inspectErr
				}

				image, ok := images[ref]
				if !ok {
					return dockerClient.ImageInspectResult{}, cerrdefs.ErrNotFound
				}

				if len(options) == 0 {
					image.Manifests = nil
				}

				return dockerClient.ImageInspectResult{InspectResponse: image}, nil
			}

			api.EXPECT().ImageInspect(mock.Anything, mock.Anything).RunAndReturn(inspect).Maybe()
			api.EXPECT().ImageInspect(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(inspect).Maybe()
			api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, options dockerClient.ContainerCreateOptions) (dockerClient.ContainerCreateResult, error) {
					api.created = options.Config

					return dockerClient.ContainerCreateResult{ID: testTargetID}, nil
				},
			)
			api.EXPECT().ContainerRename(mock.Anything, testTargetID, mock.Anything).
				Return(dockerClient.ContainerRenameResult{}, nil)

			return api
		}

		// create runs createTargetContainer at the given API version and
		// returns the label value sent to ContainerCreate.
		create := func(t *testing.T, log *zerolog.Logger, api *labelAPI, source *Container, clientVersion string) string {
			t.Helper()

			networkConfig := &dockerNetwork.NetworkingConfig{EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{}}

			_, err := createTargetContainer(log, t.Context(), api, source, networkConfig,
				clientVersion, false, "auto", false,
			)
			require.NoError(t, err)
			require.NotNil(t, api.created)

			return api.created.Labels["com.docker.compose.image"]
		}

		t.Run("should leave containers without the label untouched", func(t *testing.T) {
			t.Parallel()

			log, _ := newCapturedLogger(zerolog.DebugLevel)
			api := newLabelAPI(t, nil)

			assert.Empty(t, create(t, log, api, MockContainer(WithImageName(imageRef)), "1.48"))
			assert.NotContains(t, api.created.Labels, "com.docker.compose.image")
			assert.Empty(t, api.inspected)
		})

		t.Run("should create the container with the label pointing at the new image", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)

			assert.Equal(t, newImageID, create(t, log, newLabelAPI(t, nil), composeContainer(oldImageID), "1.44"))
			assert.Contains(t, logOutput.String(), "Updated Compose image label")
		})

		t.Run("should pass the client API version through for manifest digest labels", func(t *testing.T) {
			t.Parallel()

			log, _ := newCapturedLogger(zerolog.DebugLevel)
			api := newLabelAPI(t, nil)

			assert.Equal(t, newAmd64, create(t, log, api, composeContainer(oldAmd64), "1.48"))
			assert.Equal(t, oldAmd64, create(t, log, api, composeContainer(oldAmd64), "1.47"))
		})

		t.Run("should keep a label with no matching value for the new image", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)

			assert.Equal(t, "sha256:unknown", create(t, log, newLabelAPI(t, nil), composeContainer("sha256:unknown"), "1.48"))
			assert.Contains(t, logOutput.String(), "Found no matching value for Compose image label, keeping it unchanged")
		})

		t.Run("should keep the label and still create the container when the image inspect fails", func(t *testing.T) {
			t.Parallel()

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)

			assert.Equal(t, oldImageID, create(t, log, newLabelAPI(t, cerrdefs.ErrNotFound), composeContainer(oldImageID), "1.48"))
			assert.Contains(t, logOutput.String(), "Failed to resolve Compose image label, keeping it unchanged")
		})

		t.Run("should not modify the source container's labels", func(t *testing.T) {
			t.Parallel()

			log, _ := newCapturedLogger(zerolog.DebugLevel)
			source := composeContainer(oldImageID, func(_ *dockerContainer.InspectResponse, i *dockerImage.InspectResponse) {
				i.Config = nil
			})

			assert.Equal(t, newImageID, create(t, log, newLabelAPI(t, nil), source, "1.48"))
			assert.Equal(t, oldImageID, source.ContainerInfo().Config.Labels["com.docker.compose.image"])
		})
	})
}
