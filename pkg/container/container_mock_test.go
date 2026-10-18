package container

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerMount "github.com/moby/moby/api/types/mount"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
)

// MockContainerUpdate defines a function to update mock container or image metadata for testing.
type MockContainerUpdate func(*dockerContainer.InspectResponse, *dockerImage.InspectResponse)

// MockContainer creates a mock Container instance with customizable metadata.
//
// Parameters:
//   - updates: Optional functions to modify container or image metadata.
//
// Returns:
//   - *Container: A configured mock container instance.
func MockContainer(updates ...MockContainerUpdate) *Container {
	// Initialize default container metadata with running state.
	containerInfo := dockerContainer.InspectResponse{
		ID:         "container_id",
		Image:      "image",
		Name:       "test-watchtower",
		HostConfig: &dockerContainer.HostConfig{},
		State: &dockerContainer.State{
			Running: true,
			Status:  "running",
		},
		Config: &dockerContainer.Config{
			Labels: map[string]string{},
		},
		NetworkSettings: &dockerContainer.NetworkSettings{
			Networks: map[string]*dockerNetwork.EndpointSettings{},
		},
	}
	// Initialize default image metadata.
	image := dockerImage.InspectResponse{
		ID:     "image_id",
		Config: &dockerspec.DockerOCIImageConfig{},
	}

	// Apply provided updates to container or image metadata.
	for _, update := range updates {
		update(&containerInfo, &image)
	}

	// Create and return a new Container instance.
	return NewContainer(nil, &containerInfo, &image)
}

// WithPortBindings configures port bindings for the mock container.
//
// Parameters:
//   - portBindingSources: List of port binding sources (e.g., "80/tcp").
//
// Returns:
//   - MockContainerUpdate: Function to apply port bindings to container metadata.
func WithPortBindings(portBindingSources ...string) MockContainerUpdate {
	return func(container *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		portBindings := dockerNetwork.PortMap{}

		for _, pbs := range portBindingSources {
			port, err := dockerNetwork.ParsePort(pbs)
			if err != nil {
				panic(fmt.Sprintf("failed to parse port %s: %v", pbs, err))
			}

			portBindings[port] = []dockerNetwork.PortBinding{}
		}

		container.HostConfig.PortBindings = portBindings
	}
}

// WithImageName sets the image name for the mock container and image.
//
// Parameters:
//   - name: Image name to set (e.g., "test-image:latest").
//
// Returns:
//   - MockContainerUpdate: Function to set image name in container and image metadata.
func WithImageName(name string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, i *dockerImage.InspectResponse) {
		c.Config.Image = name
		i.RepoTags = append(i.RepoTags, name)
	}
}

// WithRepoDigests sets the RepoDigests for the mock image.
//
// Parameters:
//   - digests: List of repo digests (e.g., ["repo@sha256:abc"]).
//
// Returns:
//   - MockContainerUpdate: Function to set RepoDigests on image metadata.
func WithRepoDigests(digests []string) MockContainerUpdate {
	return func(_ *dockerContainer.InspectResponse, i *dockerImage.InspectResponse) {
		i.RepoDigests = digests
	}
}

// WithLinks sets dependency links for the mock container.
//
// Parameters:
//   - links: List of links in "name:alias" format.
//
// Returns:
//   - MockContainerUpdate: Function to set links in container HostConfig.
func WithLinks(links []string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.HostConfig.Links = links
	}
}

// WithVolumesFrom sets HostConfig.VolumesFrom for the mock container.
//
// Parameters:
//   - volumesFrom: List of volumes-from specs (name or ID, optional :ro/:rw).
//
// Returns:
//   - MockContainerUpdate: Function to set VolumesFrom in container HostConfig.
func WithVolumesFrom(volumesFrom []string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.VolumesFrom = volumesFrom
	}
}

// WithLabels sets labels for the mock container.
//
// Parameters:
//   - labels: Map of label key-value pairs.
//
// Returns:
//   - MockContainerUpdate: Function to set labels in container Config.
func WithLabels(labels map[string]string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.Config.Labels = labels
	}
}

// WithImageLabels sets labels on the mock image config (OCI annotations).
func WithImageLabels(labels map[string]string) MockContainerUpdate {
	return func(_ *dockerContainer.InspectResponse, i *dockerImage.InspectResponse) {
		if i.Config == nil {
			i.Config = &dockerspec.DockerOCIImageConfig{}
		}

		i.Config.Labels = labels
	}
}

// WithContainerState sets the state for the mock container.
//
// Parameters:
//   - state: Container state (e.g., running, stopped).
//
// Returns:
//   - MockContainerUpdate: Function to set state in container metadata.
func WithContainerState(state dockerContainer.State) MockContainerUpdate {
	return func(cnt *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		cnt.State = &state
	}
}

// WithHealthcheck sets the healthcheck configuration for the mock container.
//
// Parameters:
//   - healthConfig: Healthcheck configuration to set.
//
// Returns:
//   - MockContainerUpdate: Function to set healthcheck in container Config.
func WithHealthcheck(healthConfig dockerContainer.HealthConfig) MockContainerUpdate {
	return func(cnt *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		cnt.Config.Healthcheck = &healthConfig
	}
}

// WithImageHealthcheck sets the healthcheck configuration for the mock image.
//
// Parameters:
//   - healthConfig: Healthcheck configuration to set.
//
// Returns:
//   - MockContainerUpdate: Function to set healthcheck in image Config.
func WithImageHealthcheck(healthConfig dockerContainer.HealthConfig) MockContainerUpdate {
	return func(_ *dockerContainer.InspectResponse, img *dockerImage.InspectResponse) {
		img.Config.Healthcheck = &healthConfig
	}
}

// WithNetworkMode sets the network mode for the mock container.
//
// Parameters:
//   - mode: Network mode (e.g., "bridge", "host").
//
// Returns:
//   - MockContainerUpdate: Function to set network mode in container HostConfig.
func WithNetworkMode(mode string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.NetworkMode = dockerContainer.NetworkMode(mode)
		nopLog().Debug().
			Str("mode", mode).
			Bool("is_host", mode == "host").
			Msg("MockContainer set NetworkMode")
	}
}

// WithNetworkSettings sets network settings for the mock container.
//
// Parameters:
//   - networks: Map of network names to endpoint settings.
//
// Returns:
//   - MockContainerUpdate: Function to set network settings in container NetworkSettings.
func WithNetworkSettings(
	networks map[string]*dockerNetwork.EndpointSettings,
) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.NetworkSettings == nil {
			c.NetworkSettings = &dockerContainer.NetworkSettings{}
		}

		c.NetworkSettings.Networks = networks
	}
}

// WithMounts sets mounts for the mock container.
//
// Parameters:
//   - mounts: List of mount configurations.
//
// Returns:
//   - MockContainerUpdate: Function to set mounts in container HostConfig.
func WithMounts(mounts []dockerMount.Mount) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.HostConfig.Mounts = mounts
	}
}

// WithInspectMounts sets runtime mount points on the mock container inspect response.
//
// Parameters:
//   - mounts: Runtime mount points reported by container inspect.
//
// Returns:
//   - MockContainerUpdate: Function to set inspect Mounts.
func WithInspectMounts(mounts []dockerContainer.MountPoint) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.Mounts = mounts
	}
}

// WithReadonlyRootfs sets HostConfig.ReadonlyRootfs on the mock container.
//
// Parameters:
//   - readOnly: Whether the container root filesystem is read-only.
//
// Returns:
//   - MockContainerUpdate: Function to set ReadonlyRootfs.
func WithReadonlyRootfs(readOnly bool) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.ReadonlyRootfs = readOnly
	}
}

// WithNetworks adds multiple networks to the mock container.
//
// Parameters:
//   - networkNames: List of network names to add.
//
// Returns:
//   - MockContainerUpdate: Function to add networks to container NetworkSettings.
func WithNetworks(networkNames ...string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.NetworkSettings == nil {
			c.NetworkSettings = &dockerContainer.NetworkSettings{}
		}

		if c.NetworkSettings.Networks == nil {
			c.NetworkSettings.Networks = make(map[string]*dockerNetwork.EndpointSettings)
		}

		for _, name := range networkNames {
			c.NetworkSettings.Networks[name] = &dockerNetwork.EndpointSettings{
				NetworkID: fmt.Sprintf("network_%s_id", name),
				Aliases:   []string{c.Name},
			}
			nopLog().Debug().
				Str("container", c.Name).
				Str("network", name).
				Msg("MockContainer added network")
		}
	}
}

// WithUTSMode sets the UTS mode for the mock container.
//
// Parameters:
//   - mode: UTS mode to set (e.g., "host").
//
// Returns:
//   - MockContainerUpdate: Function to set UTS mode in container HostConfig.
func WithUTSMode(mode string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.UTSMode = dockerContainer.UTSMode(mode)
	}
}

// WithHostname sets the hostname for the mock container.
//
// Parameters:
//   - hostname: Hostname to set.
//
// Returns:
//   - MockContainerUpdate: Function to set hostname in container Config.
func WithHostname(hostname string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.Config == nil {
			c.Config = &dockerContainer.Config{}
		}

		c.Config.Hostname = hostname
	}
}

// WithAutoRemove sets the AutoRemove flag for the mock container.
//
// Parameters:
//   - autoRemove: Whether to auto-remove the container on stop.
//
// Returns:
//   - MockContainerUpdate: Function to set AutoRemove in container HostConfig.
func WithAutoRemove(autoRemove bool) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.AutoRemove = autoRemove
	}
}

// WithName sets the container name for the mock container.
//
// Parameters:
//   - name: Container name to set (e.g., "my-app").
//
// Returns:
//   - MockContainerUpdate: Function to set the container name in metadata.
func WithName(name string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.Name = name
	}
}

// WithID sets the container ID for the mock container.
//
// Parameters:
//   - id: Container ID to set (e.g., "abc123").
//
// Returns:
//   - MockContainerUpdate: Function to set the container ID in metadata.
func WithID(id string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		c.ID = id
	}
}

// WithEnv sets environment variables on the mock container's Config.
//
// Parameters:
//   - env: Environment variables in "KEY=VALUE" format.
//
// Returns:
//   - MockContainerUpdate: Function to set environment variables in container Config.
func WithEnv(env []string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.Config == nil {
			c.Config = &dockerContainer.Config{}
		}

		c.Config.Env = env
	}
}

// WithBinds sets bind mounts on the mock container's HostConfig.
//
// Parameters:
//   - binds: Bind mount strings in "host_path:container_path" format.
//
// Returns:
//   - MockContainerUpdate: Function to set bind mounts in container HostConfig.
func WithBinds(binds []string) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.Binds = binds
	}
}

// WithDevices sets device mappings on the mock container's HostConfig.
//
// This is primarily used to test device permission normalization (empty
// CgroupPermissions defaulting to "rwm") for Podman compatibility.
//
// Parameters:
//   - devices: Slice of DeviceMapping structs.
//
// Returns:
//   - MockContainerUpdate: Function to set devices in container HostConfig.
func WithDevices(devices []dockerContainer.DeviceMapping) MockContainerUpdate {
	return func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
		if c.HostConfig == nil {
			c.HostConfig = &dockerContainer.HostConfig{}
		}

		c.HostConfig.Devices = devices
	}
}

// MockClient is a mock implementation of the Operations interface for testing container operations.
type MockClient struct {
	createFunc        func(context.Context, *dockerContainer.Config, *dockerContainer.HostConfig, *dockerNetwork.NetworkingConfig, *ocispec.Platform, string) (dockerClient.ContainerCreateResult, error)
	startFunc         func(context.Context, string, dockerClient.ContainerStartOptions) (dockerClient.ContainerStartResult, error)
	inspectFunc       func(context.Context, string, dockerClient.ContainerInspectOptions) (dockerClient.ContainerInspectResult, error)
	removeFunc        func(context.Context, string, dockerClient.ContainerRemoveOptions) (dockerClient.ContainerRemoveResult, error)
	connectFunc       func(context.Context, string, string, *dockerNetwork.EndpointSettings) (dockerClient.NetworkConnectResult, error)
	renameFunc        func(context.Context, string, string) (dockerClient.ContainerRenameResult, error)
	imageInspectFunc  func(context.Context, string, bool) (dockerClient.ImageInspectResult, error)
	removeFuncCalled  atomic.Bool
	inspectFuncCalled atomic.Bool
	createFuncCalled  atomic.Bool
	startFuncCalled   atomic.Bool
	connectFuncCalled atomic.Bool
	renameFuncCalled  atomic.Bool
}

// ContainerCreate mocks the creation of a new container.
//
// Parameters:
//   - ctx: Context for the operation.
//   - config: Container configuration.
//   - hostConfig: Host configuration.
//   - networkingConfig: Network configuration.
//   - platform: Platform specification.
//   - containerName: Name for the new container.
//
// Returns:
//   - dockerContainerType.CreateResponse: Mocked response with container ID.
//   - error: Error if the mock create function is set to fail, nil otherwise.
func (m *MockClient) ContainerCreate(
	ctx context.Context,
	options dockerClient.ContainerCreateOptions,
) (dockerClient.ContainerCreateResult, error) {
	m.createFuncCalled.Store(true)

	if m.createFunc != nil {
		return m.createFunc(ctx, options.Config, options.HostConfig, options.NetworkingConfig, options.Platform, options.Name)
	}

	return dockerClient.ContainerCreateResult{ID: "new_container_id"}, nil
}

// ContainerStart mocks the start of a container.
//
// Parameters:
//   - ctx: Context for the operation.
//   - containerID: ID of the container to start.
//   - options: Start options.
//
// Returns:
//   - error: Error if the mock start function is set to fail, nil otherwise.
func (m *MockClient) ContainerStart(
	ctx context.Context,
	containerID string,
	options dockerClient.ContainerStartOptions,
) (dockerClient.ContainerStartResult, error) {
	m.startFuncCalled.Store(true)

	if m.startFunc != nil {
		return m.startFunc(ctx, containerID, options)
	}

	return dockerClient.ContainerStartResult{}, nil
}

// ContainerInspect mocks inspection of a container.
//
// Parameters:
//   - ctx: Context for the operation.
//   - containerID: ID of the container to inspect.
//   - options: Inspect options.
//
// Returns:
//   - dockerClient.ContainerInspectResult: Mocked inspect result.
//   - error: Error if the mock inspect function is set to fail, nil otherwise.
func (m *MockClient) ContainerInspect(
	ctx context.Context,
	containerID string,
	options dockerClient.ContainerInspectOptions,
) (dockerClient.ContainerInspectResult, error) {
	m.inspectFuncCalled.Store(true)

	if m.inspectFunc != nil {
		return m.inspectFunc(ctx, containerID, options)
	}

	// Default: created but not running (typical after a start failure).
	return dockerClient.ContainerInspectResult{
		Container: dockerContainer.InspectResponse{
			ID: containerID,
			State: &dockerContainer.State{
				Status:  "created",
				Running: false,
			},
		},
	}, nil
}

// ContainerRemove mocks the removal of a container.
//
// Parameters:
//   - ctx: Context for the operation.
//   - containerID: ID of the container to remove.
//   - options: Removal options (e.g., force, remove volumes).
//
// Returns:
//   - error: Error if the mock remove function is set to fail, nil otherwise.
func (m *MockClient) ContainerRemove(
	ctx context.Context,
	containerID string,
	options dockerClient.ContainerRemoveOptions,
) (dockerClient.ContainerRemoveResult, error) {
	m.removeFuncCalled.Store(true)

	if m.removeFunc != nil {
		return m.removeFunc(ctx, containerID, options)
	}

	return dockerClient.ContainerRemoveResult{}, nil
}

// NetworkConnect mocks connecting a container to a network.
//
// Parameters:
//   - ctx: Context for the operation.
//   - networkID: ID of the network.
//   - containerID: ID of the container.
//   - config: Endpoint settings for the network connection.
//
// Returns:
//   - error: Error if the mock connect function is set to fail, nil otherwise.
func (m *MockClient) NetworkConnect(
	ctx context.Context,
	networkID string,
	options dockerClient.NetworkConnectOptions,
) (dockerClient.NetworkConnectResult, error) {
	m.connectFuncCalled.Store(true)

	if m.connectFunc != nil {
		return m.connectFunc(ctx, networkID, options.Container, options.EndpointConfig)
	}

	return dockerClient.NetworkConnectResult{}, nil
}

// ContainerRename mocks renaming a container.
//
// Parameters:
//   - ctx: Context for the operation.
//   - containerID: ID of the container to rename.
//   - options: Rename options carrying the new name.
//
// Returns:
//   - dockerClient.ContainerRenameResult: Empty rename result.
//   - error: Error if the mock rename function is set to fail, nil otherwise.
func (m *MockClient) ContainerRename(
	ctx context.Context,
	containerID string,
	options dockerClient.ContainerRenameOptions,
) (dockerClient.ContainerRenameResult, error) {
	m.renameFuncCalled.Store(true)

	if m.renameFunc != nil {
		return m.renameFunc(ctx, containerID, options.NewName)
	}

	return dockerClient.ContainerRenameResult{}, nil
}

// ImageInspect mocks the inspection of an image.
//
// Parameters:
//   - ctx: Context for the operation.
//   - imageID: ID or reference of the image to inspect.
//   - inspectOpts: Inspect options. Any option is taken as a request for manifests.
//
// Returns:
//   - dockerClient.ImageInspectResult: Mocked image metadata.
//   - error: Error if the mock image inspect function is set to fail, nil otherwise.
func (m *MockClient) ImageInspect(
	ctx context.Context,
	imageID string,
	inspectOpts ...dockerClient.ImageInspectOption,
) (dockerClient.ImageInspectResult, error) {
	if m.imageInspectFunc != nil {
		return m.imageInspectFunc(ctx, imageID, len(inspectOpts) > 0)
	}

	return dockerClient.ImageInspectResult{}, nil
}

// APIVersionPingHandler returns a handler that responds to the Docker client's
// HEAD /_ping request used for API version negotiation.
func APIVersionPingHandler() http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("HEAD", "/_ping"),
		ghttp.RespondWith(http.StatusOK, nil),
	)
}

// ContainerUpdateHandler returns a handler that responds to the Docker API's
// POST /containers/{id}/update endpoint. When expectedPolicy is non-empty, it
// verifies RestartPolicy.Name matches that value.
//
// Parameters:
//   - containerID: The container ID for the update endpoint.
//   - status: HTTP status code to return (204 for success, 500 for error).
//   - expectedPolicy: Restart policy name to verify. Empty skips body checks.
//
// Returns:
//   - http.HandlerFunc: Handler for the container update endpoint.
func ContainerUpdateHandler(
	containerID string,
	status int,
	expectedPolicy string,
) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("POST", gomega.HaveSuffix(fmt.Sprintf("containers/%s/update", containerID))),
		func(w http.ResponseWriter, r *http.Request) {
			if expectedPolicy != "" {
				var body map[string]any

				err := json.NewDecoder(r.Body).Decode(&body)
				if err != nil {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				restartPolicy, ok := body["RestartPolicy"].(map[string]any)
				gomega.Expect(ok).To(gomega.BeTrue(), "RestartPolicy should be present in update body")

				name, ok := restartPolicy["Name"].(string)
				gomega.Expect(ok).To(gomega.BeTrue(), "RestartPolicy.Name should be a string")
				gomega.Expect(name).To(gomega.Equal(expectedPolicy), "RestartPolicy.Name should match")

				if expectedPolicy == "no" {
					retryCount, hasRetryCount := restartPolicy["MaximumRetryCount"]
					if hasRetryCount {
						gomega.Expect(retryCount).To(gomega.BeNumerically("==", 0),
							"disabled policy should not keep a retry count")
					}
				}
			}

			body := []byte(nil)
			if status < http.StatusBadRequest {
				body = []byte("{}")
			}

			ghttp.RespondWith(status, body)(w, r)
		},
	)
}

func StopContainerHandler(
	containerID string,
	status mockContainer.FoundStatus,
) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest(
			"POST",
			gomega.HaveSuffix(fmt.Sprintf("containers/%s/stop", containerID)),
		),
		func(w http.ResponseWriter, r *http.Request) {
			code := 404
			if status {
				code = 204
			}

			ghttp.RespondWith(code, nil)(w, r)
		},
	)
}
