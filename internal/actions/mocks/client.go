// Package mocks provides mock implementations for testing Watchtower components.
package mocks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// MockClient must satisfy the client interface. This mock is hand written
// rather than generated, so nothing else would report a missing or renamed
// method until a test happened to pass it where the interface is required.
var _ container.Client = MockClient{}

// MockClient is a mock implementation of a Watchtower Client for testing purposes.
// It simulates container operations with configurable behavior defined by TestData.
type MockClient struct {
	TestData      *TestData
	pullImages    bool
	removeVolumes bool
	Stopped       map[string]bool // Track stopped containers by ID.
	ctx           context.Context // Context for cancellation simulation
}

// TestData holds configuration data for MockClient's test behavior.
// It defines container states, staleness, and mock operation results.
// Counter fields use atomic.Int32 for thread-safe concurrent access.
type TestData struct {
	TriedToRemoveImageCount      atomic.Int32                          // Number of times RemoveImageByID was called.
	RenameContainerCount         atomic.Int32                          // Number of times RenameContainer was called.
	StopContainerCount           atomic.Int32                          // Number of times StopContainer was called.
	StopAndRemoveContainerCount  atomic.Int32                          // Number of times StopAndRemoveContainer was called.
	StartContainerCount          atomic.Int32                          // Number of times StartContainer was called.
	CreateContainerCount         atomic.Int32                          // Number of times CreateContainer was called.
	LastStopAndRemoveID          types.ContainerID                     // ID of the last container passed to StopAndRemoveContainer.
	LastCreatedContainerID       types.ContainerID                     // ID returned by the last successful CreateContainer call.
	LastRenameTarget             string                                // Last new name passed to RenameContainer.
	RenameTargets                []string                              // Ordered list of rename targets.
	SetRestartPolicyCount        atomic.Int32                          // Number of times SetRestartPolicy was called.
	IsContainerStaleCount        atomic.Int32                          // Number of times IsContainerStale was called.
	WaitForContainerHealthyCount atomic.Int32                          // Number of times WaitForContainerHealthy was called.
	ListContainersCount          atomic.Int32                          // Number of times ListContainers was called.
	NameOfContainerToKeep        string                                // Name of the container to avoid stopping.
	Containers                   []types.Container                     // List of mock containers.
	ContainersByID               map[types.ContainerID]types.Container // Map of containers by ID.
	Staleness                    map[string]bool                       // Map of container names to staleness status.
	IsContainerStaleError        error                                 // Error to return from IsContainerStale (for testing).
	ListContainersError          error                                 // Error to return from ListContainers (for testing).
	ListContainersFailCount      int                                   // Number of times ListContainers should fail before succeeding.
	StopContainerError           error                                 // Error to return from StopContainer (for testing).
	StartContainerError          error                                 // Error to return from StartContainer (for testing).
	StartContainerByIDError      error                                 // Error to return from StartContainerByID (for testing).
	CreateContainerError         error                                 // Error to return from CreateContainer (for testing).
	StopContainerFailCount       int                                   // Number of times StopContainer should fail before succeeding.
	RemoveImageError             error                                 // Error to return from RemoveImageByID (for testing).
	FailedImageIDs               []types.ImageID                       // List of image IDs that should fail removal.
	StopOrder                    []string                              // Order in which containers were stopped.
	CreateOrder                  []string                              // Order in which containers were created.
	StartOrder                   []string                              // Order in which containers were started.
	// OperationOrder records client method names in call order for handoff sequencing assertions.
	OperationOrder            []string
	RemoveContainerCount      atomic.Int32                  // Number of times RemoveContainer was called.
	SimulatedLatency          time.Duration                 // Simulated latency for operations (default 0 for fast tests, set for context cancellation tests).
	LastContainerChain        string                        // Last container chain passed to CreateEphemeralOrchestrator.
	LastCleanup               bool                          // Last cleanup flag passed to CreateEphemeralOrchestrator.
	LastStartedContainer      types.Container               // Last container passed to StartContainer.
	LastStartedContainerID    types.ContainerID             // ID returned by the last successful StartContainer call.
	SetRestartPolicyContainer types.Container               // Last container passed to SetRestartPolicy.
	SetRestartPolicyCtx       context.Context               // Last context passed to SetRestartPolicy.
	LastRestartPolicy         dockerContainer.RestartPolicy // Last policy passed to SetRestartPolicy.
	CreateContainerCtx        context.Context               // Last context passed to CreateContainer.
	RenameContainerCtx        context.Context               // Last context passed to RenameContainer.
	StartContainerByIDCtx     context.Context               // Last context passed to StartContainerByID.
	GetContainerCtx           context.Context               // Last context passed to GetContainer.
	StopAndRemoveContainerCtx context.Context               // Last context passed to StopAndRemoveContainer.
	GetImageDiskUsageCount    atomic.Int32                  // Number of times GetImageDiskUsage was called.
	ImageDiskUsage            types.ImageDiskUsage          // Usage returned by GetImageDiskUsage.
	GetImageDiskUsageError    error                         // Error to return from GetImageDiskUsage.
	ImageAnnotations          map[string]oci.Annotations    // OCI annotations returned per image reference.
	LastExecUID               int                           // UID passed to the last ExecuteCommand call.
	LastExecGID               int                           // GID passed to the last ExecuteCommand call.
}

// recordOperation appends an operation name to OperationOrder for sequencing tests.
//
// Parameters:
//   - name: Client method name to record (for example, "StopContainer").
func (testdata *TestData) recordOperation(name string) {
	if testdata == nil {
		return
	}

	testdata.OperationOrder = append(testdata.OperationOrder, name)
}

// TriedToRemoveImage checks if RemoveImageByID has been invoked.
// It returns true if the count is greater than zero, aiding test assertions.
func (testdata *TestData) TriedToRemoveImage() bool {
	return testdata.TriedToRemoveImageCount.Load() > 0
}

// CreateMockClient constructs a new MockClient instance for testing.
// It initializes the client with provided test data, pull and volume removal flags,
// and an empty map for tracking stopped containers.
func CreateMockClient(data *TestData, pullImages, removeVolumes bool) MockClient {
	return CreateMockClientWithContext(context.Background(), data, pullImages, removeVolumes)
}

// CreateMockClientWithContext constructs a new MockClient instance with context for testing.
// It initializes the client with provided context, test data, pull and volume removal flags,
// and an empty map for tracking stopped containers.
func CreateMockClientWithContext(
	ctx context.Context,
	data *TestData,
	pullImages bool,
	removeVolumes bool,
) MockClient {
	if data.ContainersByID == nil {
		data.ContainersByID = make(map[types.ContainerID]types.Container)
	}
	for _, c := range data.Containers {
		data.ContainersByID[c.ID()] = c
	}

	return MockClient{
		TestData:      data,
		pullImages:    pullImages,
		removeVolumes: removeVolumes,
		Stopped:       make(map[string]bool),
		ctx:           ctx,
	}
}

// checkContextCancellation checks both the passed context and client context for cancellation.
// It simulates latency when SimulatedLatency is configured for testing context cancellation.
// Returns the cancellation error if either context is done, nil otherwise.
func (client MockClient) checkContextCancellation(ctx context.Context) error {
	// Simulate latency for context cancellation testing when configured
	if client.TestData.SimulatedLatency > 0 {
		select {
		case <-time.After(client.TestData.SimulatedLatency):
		case <-ctx.Done():
			return ctx.Err()
		case <-client.ctx.Done():
			return client.ctx.Err()
		}
	}

	// Check passed context for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Also check client.ctx for backward compatibility
	if client.ctx != nil {
		select {
		case <-client.ctx.Done():
			return client.ctx.Err()
		default:
		}
	}

	return nil
}

// ListContainers returns containers from TestData, optionally filtered.
func (client MockClient) ListContainers(ctx context.Context, filter ...types.Filter) ([]types.Container, error) {
	client.TestData.ListContainersCount.Add(1)

	if err := client.checkContextCancellation(ctx); err != nil {
		return nil, err
	}

	if client.TestData.ListContainersError != nil &&
		(client.TestData.ListContainersFailCount == 0 ||
			int(client.TestData.ListContainersCount.Load()) <= client.TestData.ListContainersFailCount) {
		return nil, client.TestData.ListContainersError
	}

	containers := client.TestData.Containers

	if len(filter) > 0 && filter[0] != nil {
		filtered := []types.Container{}

		for _, c := range containers {
			if filter[0](c) {
				filtered = append(filtered, c)
			}
		}

		return filtered, nil
	}

	return containers, nil
}

// StopContainer simulates stopping a container by marking it in the Stopped map.
// It records the container's ID as stopped, increments the StopContainerCount,
// and returns nil for simplicity.
func (client MockClient) StopContainer(ctx context.Context, c types.Container, _ time.Duration) error {
	client.TestData.StopContainerCount.Add(1)
	client.TestData.recordOperation("StopContainer")

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	if client.TestData.StopContainerError != nil &&
		int(client.TestData.StopContainerCount.Load()) <= client.TestData.StopContainerFailCount {
		return client.TestData.StopContainerError
	}

	client.Stopped[string(c.ID())] = true
	client.TestData.StopOrder = append(client.TestData.StopOrder, c.Name())

	return nil
}

// StopAndRemoveContainer simulates stopping and removing a container by calling StopContainer followed by RemoveContainer.
// It properly simulates the stop-and-remove operation sequence while respecting error conditions.
func (client MockClient) StopAndRemoveContainer(ctx context.Context, c types.Container, timeout time.Duration) error {
	client.TestData.StopAndRemoveContainerCount.Add(1)
	client.TestData.LastStopAndRemoveID = c.ID()
	client.TestData.StopAndRemoveContainerCtx = ctx
	client.TestData.recordOperation("StopAndRemoveContainer")

	err := client.StopContainer(ctx, c, timeout)
	if err != nil {
		return err
	}

	return client.RemoveContainer(ctx, c)
}

// IsContainerRunning checks if a container is running based on the Stopped map.
// It returns true if the container's ID is not marked as stopped, false otherwise.
func (client MockClient) IsContainerRunning(c types.Container) bool {
	return !client.Stopped[string(c.ID())]
}

// CreateContainer simulates creating a new container from a source container's
// configuration without starting it.
// It provides a minimal implementation for testing purposes.
// Returns the configured CreateContainerError if set.
// On success it returns a distinct container ID so callers can distinguish the
// new instance from the source (required for self-update failure cleanup tests).
func (client MockClient) CreateContainer(ctx context.Context, c types.Container) (types.ContainerID, error) {
	client.TestData.CreateContainerCount.Add(1)
	client.TestData.CreateContainerCtx = ctx

	if err := client.checkContextCancellation(ctx); err != nil {
		return "", err
	}

	if client.TestData.CreateContainerError != nil {
		return "", client.TestData.CreateContainerError
	}

	client.TestData.CreateOrder = append(client.TestData.CreateOrder, c.Name())

	newID := types.ContainerID(string(c.ID()) + "-recreated")
	client.TestData.LastCreatedContainerID = newID

	// Register a lookup entry so GetContainer(newID) succeeds for cleanup paths.
	if client.TestData.ContainersByID == nil {
		client.TestData.ContainersByID = make(map[types.ContainerID]types.Container)
	}

	if _, exists := client.TestData.ContainersByID[newID]; !exists {
		client.TestData.ContainersByID[newID] = CreateMockContainer(
			string(newID),
			c.Name(),
			c.ImageName(),
			time.Now(),
		)
	}

	return newID, nil
}

// StartContainer simulates starting a container, returning the container's ID.
// It provides a minimal implementation for testing purposes.
// Returns the configured StartContainerError if set.
func (client MockClient) StartContainer(ctx context.Context, c types.Container) (types.ContainerID, error) {
	client.TestData.StartContainerCount.Add(1)
	client.TestData.recordOperation("StartContainer")
	client.TestData.LastStartedContainer = c

	if err := client.checkContextCancellation(ctx); err != nil {
		return "", err
	}

	if client.TestData.StartContainerError != nil {
		return "", client.TestData.StartContainerError
	}

	client.TestData.StartOrder = append(client.TestData.StartOrder, c.Name())

	// Return a distinct ID so handoff verification can distinguish the replacement
	// instance from the source, matching real Docker create+start behavior.
	newID := types.ContainerID(string(c.ID()) + "-started")
	client.TestData.LastStartedContainerID = newID

	if client.TestData.ContainersByID == nil {
		client.TestData.ContainersByID = make(map[types.ContainerID]types.Container)
	}

	if _, exists := client.TestData.ContainersByID[newID]; !exists {
		// Register as running so ensureContainerRunning accepts the handoff.
		labels := map[string]string{}
		if info := c.ContainerInfo(); info != nil && info.Config != nil && info.Config.Labels != nil {
			for k, v := range info.Config.Labels {
				labels[k] = v
			}
		}

		client.TestData.ContainersByID[newID] = CreateMockContainerWithConfig(
			string(newID),
			c.Name(),
			c.ImageName(),
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Image:  c.ImageName(),
				Labels: labels,
			},
		)
	}

	return newID, nil
}

// StartContainerByID simulates starting a container by its ID directly.
// It provides a minimal implementation for testing purposes.
// Returns the configured StartContainerByIDError if set.
func (client MockClient) StartContainerByID(ctx context.Context, containerID types.ContainerID) error {
	client.TestData.StartContainerCount.Add(1)
	client.TestData.recordOperation("StartContainerByID")
	client.TestData.StartContainerByIDCtx = ctx

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	if client.TestData.StartContainerByIDError != nil {
		return client.TestData.StartContainerByIDError
	}

	// Mark the container running when present so re-inspect after start succeeds.
	delete(client.Stopped, string(containerID))

	if c, ok := client.TestData.ContainersByID[containerID]; ok {
		if info := c.ContainerInfo(); info != nil && info.State != nil {
			info.State.Running = true
		}
	}

	return nil
}

// RenameContainer simulates renaming a container, incrementing the RenameContainerCount.
// It returns nil to indicate success without modifying any state.
func (client MockClient) RenameContainer(ctx context.Context, _ types.Container, newName string) error {
	client.TestData.RenameContainerCount.Add(1)
	client.TestData.LastRenameTarget = newName
	client.TestData.RenameTargets = append(client.TestData.RenameTargets, newName)
	client.TestData.recordOperation("RenameContainer")
	client.TestData.RenameContainerCtx = ctx

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	return nil
}

// SetRestartPolicy simulates updating a container's restart policy.
func (client MockClient) SetRestartPolicy(
	ctx context.Context,
	container types.Container,
	policy dockerContainer.RestartPolicy,
) {
	client.TestData.SetRestartPolicyCount.Add(1)
	client.TestData.SetRestartPolicyContainer = container
	client.TestData.SetRestartPolicyCtx = ctx
	client.TestData.LastRestartPolicy = policy
	client.TestData.recordOperation("SetRestartPolicy")
}

// RemoveImageByID increments the count of image removal attempts in TestData.
// It simulates image cleanup and returns configured error if set or if image ID is in FailedImageIDs, nil otherwise.
func (client MockClient) RemoveImageByID(ctx context.Context, imageID types.ImageID, _ string) error {
	client.TestData.TriedToRemoveImageCount.Add(1)

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	if slices.Contains(client.TestData.FailedImageIDs, imageID) {
		return client.TestData.RemoveImageError
	}

	return nil
}

// GetContainer returns the container with the specified ID from TestData.
// It provides a mock response for testing container retrieval.
func (client MockClient) GetContainer(ctx context.Context, containerID types.ContainerID) (types.Container, error) {
	client.TestData.GetContainerCtx = ctx
	client.TestData.recordOperation("GetContainer")

	if c, ok := client.TestData.ContainersByID[containerID]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("container not found: %s", containerID)
}

// GetCurrentWatchtowerContainer returns the container with the specified ID from TestData with imageInfo set to nil.
// It provides a mock response for testing current Watchtower container retrieval.
func (client MockClient) GetCurrentWatchtowerContainer(_ context.Context, containerID types.ContainerID) (types.Container, error) {
	if c, ok := client.TestData.ContainersByID[containerID]; ok {
		// Return a copy with imageInfo nil
		// Since it's a mock, just return the same container
		return c, nil
	}
	return nil, fmt.Errorf("container not found: %s", containerID)
}

// GetVersion returns a mock Docker API client version.
// It provides a static version string for testing purposes.
func (client MockClient) GetVersion() string {
	return "1.50"
}

// errCommandFailed is a static error indicating a command exited with a non-zero code.
// It is used in ExecuteCommand to provide consistent error reporting for test scenarios.
var errCommandFailed = errors.New("command exited with non-zero code")

// ExecuteCommand simulates executing a command in a container for testing lifecycle hooks.
// It returns a SkipUpdate boolean indicating whether to skip the update and an error if the command fails.
// The method uses predefined command behaviors to mimic real execution outcomes.
func (client MockClient) ExecuteCommand(
	ctx context.Context,
	_ types.Container,
	command string,
	_ int,
	uid int,
	gid int,
) (bool, error) {
	if err := client.checkContextCancellation(ctx); err != nil {
		return false, err
	}

	if client.TestData != nil {
		client.TestData.LastExecUID = uid
		client.TestData.LastExecGID = gid
	}

	switch command {
	case "/PreUpdateReturn0.sh":
		return false, nil // Command succeeds (exit 0), no skip.
	case "/PreUpdateReturn1.sh":
		return false, fmt.Errorf(
			"%w: %s",
			errCommandFailed,
			"code 1",
		) // Command fails (exit 1), no skip.
	case "/PreUpdateReturn75.sh":
		return true, nil // Command succeeds (exit 75), signals skip.
	default:
		return false, nil // Unknown commands succeed (exit 0), no skip.
	}
}

// IsContainerStale determines if a container is stale based on TestData's Staleness map.
// It returns true if the container's name isn't explicitly marked as fresh, along with an empty ImageID and no error.
// If IsContainerStaleError is set, it returns that error instead.
func (client MockClient) IsContainerStale(
	ctx context.Context,
	container types.Container,
	_ types.UpdateParams,
) (bool, types.ImageID, string, error) {
	client.TestData.IsContainerStaleCount.Add(1)

	if err := client.checkContextCancellation(ctx); err != nil {
		return false, "", "", err
	}

	// Return configured error if set (for testing error conditions)
	if client.TestData.IsContainerStaleError != nil {
		return false, "", "", client.TestData.IsContainerStaleError
	}

	stale, found := client.TestData.Staleness[container.Name()]
	if !found {
		stale = true // Default to stale if not specified.
	}

	return stale, "", "", nil
}

// CheckContainerUpdate reports update availability using the same staleness map as IsContainerStale.
func (client MockClient) CheckContainerUpdate(
	ctx context.Context,
	container types.Container,
	params types.UpdateParams,
) (bool, types.ImageID, string, error) {
	return client.IsContainerStale(ctx, container, params)
}

// GetImageAnnotations returns the annotations configured for an image reference.
//
// Parameters:
//   - ctx: Context for operation control.
//   - imageRef: Image reference, tag or digest, to inspect.
//
// Returns:
//   - oci.Annotations: Configured annotations, or empty when none are set.
func (client MockClient) GetImageAnnotations(
	_ context.Context,
	imageRef string,
) oci.Annotations {
	if client.TestData == nil {
		return oci.Annotations{}
	}

	return client.TestData.ImageAnnotations[imageRef]
}

// WaitForContainerHealthy simulates waiting for a container to become healthy.
// It increments the count and returns nil to indicate success.
func (client MockClient) WaitForContainerHealthy(ctx context.Context, _ types.ContainerID, _ time.Duration) error {
	client.TestData.WaitForContainerHealthyCount.Add(1)

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	return nil
}

// RemoveContainer simulates removing a container.
// It returns nil to indicate success.
func (client MockClient) RemoveContainer(ctx context.Context, _ types.Container) error {
	client.TestData.RemoveContainerCount.Add(1)
	client.TestData.recordOperation("RemoveContainer")

	if err := client.checkContextCancellation(ctx); err != nil {
		return err
	}

	return nil
}

// CreateEphemeralOrchestrator simulates creating an ephemeral orchestrator container.
// It records the container chain parameter for test verification and returns a mock
// container ID for the orchestrator and nil error to indicate success.
func (client MockClient) CreateEphemeralOrchestrator(
	ctx context.Context,
	_ types.Container,
	_ string,
	containerChain string,
	cleanup bool,
) (types.ContainerID, error) {
	if err := client.checkContextCancellation(ctx); err != nil {
		return "", err
	}

	client.TestData.LastContainerChain = containerChain
	client.TestData.LastCleanup = cleanup

	return types.ContainerID("mock-ephemeral-orchestrator"), nil
}

// GetImageDiskUsage returns configured mock image usage for testing.
func (client MockClient) GetImageDiskUsage(ctx context.Context) (types.ImageDiskUsage, error) {
	if client.TestData == nil {
		return types.ImageDiskUsage{}, nil
	}

	if err := client.checkContextCancellation(ctx); err != nil {
		return types.ImageDiskUsage{}, err
	}

	client.TestData.GetImageDiskUsageCount.Add(1)

	if client.TestData.GetImageDiskUsageError != nil {
		return types.ImageDiskUsage{}, client.TestData.GetImageDiskUsageError
	}

	return client.TestData.ImageDiskUsage, nil
}

// GetInfo returns mock system information for testing.
func (client MockClient) GetInfo(ctx context.Context) (types.SystemInfo, error) {
	if err := client.checkContextCancellation(ctx); err != nil {
		return types.SystemInfo{}, err
	}

	return types.SystemInfo{
		Name:          "docker",
		ServerVersion: "1.50",
		OSType:        "linux",
	}, nil
}

// Ping returns nil to simulate a healthy Docker daemon.
func (client MockClient) Ping(ctx context.Context) error {
	return client.checkContextCancellation(ctx)
}

// BuildRemoteImage simulates building an image from a Git URL context.
func (client MockClient) BuildRemoteImage(ctx context.Context, _, _ string, tags []string) (types.ImageID, error) {
	if err := client.checkContextCancellation(ctx); err != nil {
		return "", err
	}

	if client.TestData != nil {
		client.TestData.recordOperation("BuildRemoteImage")
	}

	if len(tags) > 0 {
		return types.ImageID("sha256:gitbuild" + tags[0]), nil
	}

	return types.ImageID("sha256:gitbuild"), nil
}
