package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerMount "github.com/moby/moby/api/types/mount"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
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

// dockerImageFixture is an image whose inspect response is read from testdata.
type dockerImageFixture struct {
	// id is the image ID.
	id types.ImageID
	// file names testdata/image_<file>.json.
	file string
}

// dockerContainerFixture is a container whose inspect response is read from
// testdata, with its image and the containers it references.
type dockerContainerFixture struct {
	// image is the container's image. Nil for a missing container.
	image *dockerImageFixture
	// id is the container ID.
	id types.ContainerID
	// name names testdata/container_<name>.json unless file is set.
	name string
	// file overrides name as the testdata file name.
	file string
	// references are containers inspected while resolving this one, such as
	// a network mode container.
	references []*dockerContainerFixture
	// missing makes the daemon report the container as not found.
	missing bool
}

const (
	// fixtureNetSupplierNotFoundID is the ID of a network supplier container
	// that no longer exists.
	fixtureNetSupplierNotFoundID = "badc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc1dbadc"
	// fixtureNetSupplierContainerName is the Docker name of the network
	// supplier container.
	fixtureNetSupplierContainerName = "/wt-contnet-producer-1"
)

// Docker fixtures served from testdata.
var (
	// fixtureDefaultImage is the Watchtower image.
	fixtureDefaultImage = &dockerImageFixture{
		id:   "sha256:4dbc5f9c07028a985e14d1393e849ea07f68804c4293050d5a641b138db72daa",
		file: "default",
	}
	// fixtureWatchtower is a running Watchtower container.
	fixtureWatchtower = &dockerContainerFixture{
		name:  "watchtower",
		id:    "3d88e0e3543281c747d88b27e246578b65ae8964ba86c7cd7522cf84e0978134",
		image: fixtureDefaultImage,
	}
	// fixtureStopped is a stopped container.
	fixtureStopped = &dockerContainerFixture{
		name:  "stopped",
		id:    "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b65",
		image: fixtureDefaultImage,
	}
	// fixtureRunning is a running Portainer container.
	fixtureRunning = &dockerContainerFixture{
		name: "running",
		id:   "b978af0b858aa8855cce46b628817d4ed58e58f2c4f66c9b9c5449134ed4c008",
		image: &dockerImageFixture{
			id:   "sha256:19d07168491a3f9e2798a9bed96544e34d57ddc4757a4ac5bb199dea896c87fd",
			file: "running",
		},
	}
	// fixtureRestarting is a restarting container.
	fixtureRestarting = &dockerContainerFixture{
		name:  "restarting",
		id:    "ae8964ba86c7cd7522cf84e09781343d88e0e3543281c747d88b27e246578b67",
		image: fixtureDefaultImage,
	}
	// fixtureNetSupplier is a container supplying its network to another.
	fixtureNetSupplier = &dockerContainerFixture{
		name: "net_supplier",
		id:   "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2",
		image: &dockerImageFixture{
			id:   "sha256:c22b543d33bfdcb9992cbef23961677133cdf09da71d782468ae2517138bad51",
			file: "net_producer",
		},
	}
	// fixtureNetSupplierNotFound is a network supplier that no longer exists.
	fixtureNetSupplierNotFound = &dockerContainerFixture{
		name:    "net_supplier",
		id:      fixtureNetSupplierNotFoundID,
		missing: true,
	}
	// fixtureNetConsumerImage is the image of the network consumers.
	fixtureNetConsumerImage = &dockerImageFixture{
		id:   "sha256:904b8cb13b932e23230836850610fa45dce9eb0650d5618c2b1487c2a4f577b8",
		file: "net_consumer",
	}
	// fixtureNetConsumer uses the network of fixtureNetSupplier.
	fixtureNetConsumer = &dockerContainerFixture{
		name:       "net_consumer",
		id:         "1f6b79d2aff23244382026c76f4995851322bed5f9c50631620162f6f9aafbd6",
		image:      fixtureNetConsumerImage,
		references: []*dockerContainerFixture{fixtureNetSupplier},
	}
	// fixtureNetConsumerMissingSupplier uses the network of a container that
	// no longer exists.
	fixtureNetConsumerMissingSupplier = &dockerContainerFixture{
		name:       "net_consumer-missing_supplier",
		id:         fixtureNetConsumer.id,
		image:      fixtureNetConsumerImage,
		references: []*dockerContainerFixture{fixtureNetSupplierNotFound},
	}
)

// newDockerAPI returns a mocked Docker API whose client version is available
// for diagnostic logs.
//
// Parameters:
//   - t: Test the mock belongs to.
//
// Returns:
//   - *mockContainer.MockAPIClient: Mocked Docker API.
func newDockerAPI(t *testing.T) *mockContainer.MockAPIClient {
	t.Helper()

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().ClientVersion().Return("1.47").Maybe()

	return api
}

// readFixture decodes testdata/name into a value of type T.
//
// Parameters:
//   - t: Test reading the fixture.
//   - name: File name within testdata.
//
// Returns:
//   - T: Decoded fixture.
func readFixture[T any](t *testing.T, name string) T {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)

	var value T

	require.NoError(t, json.Unmarshal(data, &value))

	return value
}

// daemonError returns an error formatted as the Docker SDK reports a daemon
// error response.
//
// Parameters:
//   - message: Message the daemon responded with.
//
// Returns:
//   - error: Error with the daemon's message.
func daemonError(message string) error {
	return errors.New("Error response from daemon: " + message)
}

// notFoundError returns a not-found error as the Docker SDK reports a 404.
//
// Parameters:
//   - message: Message the daemon responded with.
//
// Returns:
//   - error: Error matching [cerrdefs.IsNotFound].
func notFoundError(message string) error {
	return fmt.Errorf("Error response from daemon: %s: %w", message, cerrdefs.ErrNotFound)
}

// statusFilters returns list filters for the given container statuses.
//
// Parameters:
//   - statuses: Container statuses to list.
//
// Returns:
//   - dockerClient.Filters: Filters with one status entry per status.
func statusFilters(statuses ...string) dockerClient.Filters {
	filters := make(dockerClient.Filters)
	for _, status := range statuses {
		filters.Add("status", status)
	}

	return filters
}

// expectContainerList expects one container list request filtered by exactly
// the given statuses, and answers with the testdata containers in them.
//
// Parameters:
//   - t: Test the expectation belongs to.
//   - api: Mocked Docker API.
//   - statuses: Statuses the request filters by.
func expectContainerList(t *testing.T, api *mockContainer.MockAPIClient, statuses ...string) {
	t.Helper()

	var listed []dockerContainer.Summary

	for _, summary := range readFixture[[]dockerContainer.Summary](t, "containers.json") {
		for _, status := range statuses {
			if summary.State == dockerContainer.ContainerState(status) {
				listed = append(listed, summary)
			}
		}
	}

	api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{Filters: statusFilters(statuses...)}).
		Return(dockerClient.ContainerListResult{Items: listed}, nil).Once()
}

// expectContainerFixtures expects one inspect of each container, of each
// container it references, and of each distinct image, and answers them from
// testdata.
//
// Parameters:
//   - t: Test the expectations belong to.
//   - api: Mocked Docker API.
//   - fixtures: Containers the code under test inspects.
func expectContainerFixtures(t *testing.T, api *mockContainer.MockAPIClient, fixtures ...*dockerContainerFixture) {
	t.Helper()

	seenImages := make(map[types.ImageID]bool, len(fixtures))

	for _, fixture := range fixtures {
		expectContainerFixture(t, api, fixture)

		for _, reference := range fixture.references {
			expectContainerFixture(t, api, reference)
		}

		if seenImages[fixture.image.id] {
			continue
		}

		seenImages[fixture.image.id] = true
		image := readFixture[dockerImage.InspectResponse](t, "image_"+fixture.image.file+".json")
		require.Equal(t, string(fixture.image.id), image.ID, "image fixture file %q", fixture.image.file)
		api.EXPECT().ImageInspect(mock.Anything, string(fixture.image.id)).
			Return(dockerClient.ImageInspectResult{InspectResponse: image}, nil).Once()
	}
}

// expectContainerFixture expects one inspect of the container and answers it
// from testdata, or with a not-found error for a missing container.
//
// Parameters:
//   - t: Test the expectation belongs to.
//   - api: Mocked Docker API.
//   - fixture: Container to inspect.
func expectContainerFixture(t *testing.T, api *mockContainer.MockAPIClient, fixture *dockerContainerFixture) {
	t.Helper()

	call := api.EXPECT().ContainerInspect(mock.Anything, string(fixture.id), dockerClient.ContainerInspectOptions{})
	if fixture.missing {
		call.Return(dockerClient.ContainerInspectResult{}, notFoundError("No such container: "+string(fixture.id))).Once()

		return
	}

	file := fixture.file
	if file == "" {
		file = fixture.name
	}

	info := readFixture[dockerContainer.InspectResponse](t, "container_"+file+".json")
	call.Return(dockerClient.ContainerInspectResult{Container: info}, nil).Once()
}
