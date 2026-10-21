package container

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/moby/moby/api/types/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
	dockerClient "github.com/moby/moby/client"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	testContainerID = "test-container-id"
	testShortID     = "abc123def456"
)

// Test_listSourceContainers verifies the status filters the container list
// request carries for each client option, that a 404 from the list counts as
// no containers, and that a container whose image cannot be inspected is
// warned about only when it passes the filter.
func Test_listSourceContainers(t *testing.T) {
	t.Parallel()

	// expectListed expects one container list request filtered by exactly
	// the given statuses, and answers it with the test container.
	expectListed := func(api *mockContainer.MockAPIClient, statuses ...string) {
		api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{Filters: statusFilters(statuses...)}).
			Return(dockerClient.ContainerListResult{Items: []dockerContainer.Summary{
				{ID: testContainerID, Names: []string{"/test-container"}, State: "running"},
			}}, nil).Once()
	}

	// expectInspects expects the inspects of the test container and its image.
	expectInspects := func(api *mockContainer.MockAPIClient) {
		api.EXPECT().ContainerInspect(mock.Anything, testContainerID, dockerClient.ContainerInspectOptions{}).
			Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
				ID:    testContainerID,
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
			}}, nil).Once()
		api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
			Return(dockerClient.ImageInspectResult{ID: "test-image-id"}, nil).Once()
	}

	t.Run("no custom filter with default ClientOptions", func(t *testing.T) {
		t.Parallel()

		t.Run("should list only running containers", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running")
			expectInspects(api)

			containers, err := listSourceContainers(newTestLogger(t), t.Context(), api, ClientOptions{}, nil)
			require.NoError(t, err)
			require.Len(t, containers, 1)
		})
	})

	t.Run("no custom filter with IncludeStopped=true", func(t *testing.T) {
		t.Parallel()

		t.Run("should list running, created, and exited containers", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running", "created", "exited")
			expectInspects(api)

			containers, err := listSourceContainers(newTestLogger(t), t.Context(), api, ClientOptions{IncludeStopped: true}, nil)
			require.NoError(t, err)
			require.Len(t, containers, 1)
		})
	})

	t.Run("no custom filter with IncludeRestarting=true", func(t *testing.T) {
		t.Parallel()

		t.Run("should list running and restarting containers", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running", "restarting")
			expectInspects(api)

			containers, err := listSourceContainers(newTestLogger(t), t.Context(), api, ClientOptions{IncludeRestarting: true}, nil)
			require.NoError(t, err)
			require.Len(t, containers, 1)
		})
	})

	t.Run("with custom filter (filters.NoFilter)", func(t *testing.T) {
		t.Parallel()

		t.Run("should list containers and apply custom filter", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running")
			expectInspects(api)

			containers, err := listSourceContainers(newTestLogger(t), t.Context(), api, ClientOptions{}, filters.NoFilter)
			require.NoError(t, err)
			require.Len(t, containers, 1)
		})
	})

	t.Run("Docker API returns 404 NotFound for container list", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty container list without error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ContainerList(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerListResult{}, notFoundError("page not found")).Once()

			containers, err := listSourceContainers(newTestLogger(t), t.Context(), api, ClientOptions{}, nil)
			require.NoError(t, err)
			assert.Empty(t, containers)
		})
	})

	t.Run("image inspection fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should not warn for containers excluded by the filter", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running")
			expectMissingImage(api, testContainerID)

			log, logBuf := newCapturedLogger(zerolog.WarnLevel)
			excludeAll := filters.FilterByDisableNames(log, []string{"test-container"}, filters.NoFilter)

			containers, err := listSourceContainers(log, t.Context(), api, ClientOptions{}, excludeAll)
			require.NoError(t, err)
			assert.Empty(t, containers)
			assert.NotContains(t, logBuf.String(), "Failed to retrieve image info")
		})

		t.Run("should warn for containers that pass the filter", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectListed(api, "running")
			expectMissingImage(api, testContainerID)

			log, logBuf := newCapturedLogger(zerolog.WarnLevel)

			containers, err := listSourceContainers(log, t.Context(), api, ClientOptions{}, nil)
			require.NoError(t, err)
			require.Len(t, containers, 1)

			logged := logBuf.String()
			assert.Contains(t, logged, "Failed to retrieve image info")
			// The warning must identify the container by name and image, not just by ID.
			assert.Contains(t, logged, "test-container")
			assert.Contains(t, logged, "test-image:latest")
		})
	})
}

// Test_buildListFilterArgs verifies which container statuses the list filter
// requests for each combination of client options and runtime, including that
// Podman never gets the restarting status.
func Test_buildListFilterArgs(t *testing.T) {
	t.Parallel()

	t.Run("includes running status always", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.Contains(t, statuses, "running")
	})

	t.Run("includes created and exited when IncludeStopped is true", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeStopped: true}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.Contains(t, statuses, "created")
		assert.Contains(t, statuses, "exited")
		assert.Contains(t, statuses, "running")
	})

	t.Run("does not include created and exited when IncludeStopped is false", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeStopped: false}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.NotContains(t, statuses, "created")
		assert.NotContains(t, statuses, "exited")
		assert.Contains(t, statuses, "running")
	})

	t.Run("includes restarting when IncludeRestarting is true and isPodman is false", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeRestarting: true}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.Contains(t, statuses, "restarting")
		assert.Contains(t, statuses, "running")
	})

	t.Run("does not include restarting when IncludeRestarting is false", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeRestarting: false}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.NotContains(t, statuses, "restarting")
		assert.Contains(t, statuses, "running")
	})

	t.Run("does not include restarting when isPodman is true regardless of IncludeRestarting", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeRestarting: true}
		filterArgs := buildListFilterArgs(opts, true)
		statuses := getStatusFilterKeys(filterArgs)
		assert.NotContains(t, statuses, "restarting")
		assert.Contains(t, statuses, "running")
	})

	t.Run("includes all statuses when IncludeStopped and IncludeRestarting are true and isPodman is false", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeStopped: true, IncludeRestarting: true}
		filterArgs := buildListFilterArgs(opts, false)
		statuses := getStatusFilterKeys(filterArgs)
		assert.Contains(t, statuses, "running")
		assert.Contains(t, statuses, "created")
		assert.Contains(t, statuses, "exited")
		assert.Contains(t, statuses, "restarting")
	})

	t.Run("includes running, created, exited but not restarting when IncludeStopped is true, IncludeRestarting is true, and isPodman is true", func(t *testing.T) {
		t.Parallel()

		opts := ClientOptions{IncludeStopped: true, IncludeRestarting: true}
		filterArgs := buildListFilterArgs(opts, true)
		statuses := getStatusFilterKeys(filterArgs)
		assert.Contains(t, statuses, "running")
		assert.Contains(t, statuses, "created")
		assert.Contains(t, statuses, "exited")
		assert.NotContains(t, statuses, "restarting")
	})
}

// sourceInspect returns the inspect response of a source container on
// test-image:latest.
//
// Parameters:
//   - name: Docker name of the container.
//   - running: Whether the container is running.
//   - hostConfig: Host configuration of the container.
//
// Returns:
//   - dockerContainer.InspectResponse: Inspect response of testContainerID.
func sourceInspect(name string, running bool, hostConfig *dockerContainer.HostConfig) dockerContainer.InspectResponse {
	status := dockerContainer.StateRunning
	if !running {
		status = dockerContainer.StateExited
	}

	return dockerContainer.InspectResponse{
		ID:         testContainerID,
		Name:       name,
		Image:      "test-image:latest",
		State:      &dockerContainer.State{Status: status, Running: running},
		HostConfig: hostConfig,
		Config:     &dockerContainer.Config{Image: "test-image:latest"},
	}
}

// expectInspect expects one inspect of the container and answers it with info
// and err.
//
// Parameters:
//   - api: Mocked Docker API.
//   - containerID: ID of the container to inspect.
//   - info: Inspect response.
//   - err: Error the inspect returns.
func expectInspect(api *mockContainer.MockAPIClient, containerID string, info dockerContainer.InspectResponse, err error) {
	api.EXPECT().ContainerInspect(mock.Anything, containerID, dockerClient.ContainerInspectOptions{}).
		Return(dockerClient.ContainerInspectResult{Container: info}, err).Once()
}

// expectTestImage expects one inspect of test-image:latest and answers it with
// the image test-image-id.
//
// Parameters:
//   - api: Mocked Docker API.
func expectTestImage(api *mockContainer.MockAPIClient) {
	api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
		Return(dockerClient.ImageInspectResult{ID: "test-image-id"}, nil).Once()
}

// TestGetSourceContainer verifies that a source container is inspected with
// its image, keeps working without image info, has its network mode and
// volumes-from references rewritten from IDs to names where they resolve, and
// fails when the container inspect fails.
func TestGetSourceContainer(t *testing.T) {
	t.Parallel()

	const (
		appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"
		dbID  = "a1b2c3d4e5f60718293a4b5c6d7e8f901234567890abcdef1234567890abcd"
		// missingID is a volumes-from source that no longer exists.
		missingID = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	)

	// expectNamed expects one inspect of a referenced container and answers
	// it with the container's name.
	expectNamed := func(api *mockContainer.MockAPIClient, id, name string) {
		expectInspect(api, id, dockerContainer.InspectResponse{ID: id, Name: name}, nil)
	}

	// expectMissing expects one inspect of a referenced container that no
	// longer exists.
	expectMissing := func(api *mockContainer.MockAPIClient, id string) {
		expectInspect(api, id, dockerContainer.InspectResponse{}, notFoundError("No such container"))
	}

	// volumesFrom returns the volumes-from host config of the source.
	volumesFrom := func(entries ...string) *dockerContainer.HostConfig {
		return &dockerContainer.HostConfig{VolumesFrom: entries}
	}

	t.Run("retrieving a running container successfully", func(t *testing.T) {
		t.Parallel()

		t.Run("should return container with image info", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/test-watchtower", true, &dockerContainer.HostConfig{}), nil)
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			require.NotNil(t, container)
			assert.Equal(t, types.ContainerID(testContainerID), container.ID())
			assert.Equal(t, "test-watchtower", container.Name())
			assert.Equal(t, types.ImageID("test-image-id"), container.ImageID())
		})
	})

	t.Run("retrieving a stopped container successfully", func(t *testing.T) {
		t.Parallel()

		t.Run("should return container without image info when ImageInspect fails", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/test-watchtower", false, &dockerContainer.HostConfig{}), nil)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
				Return(dockerClient.ImageInspectResult{}, daemonError("image not found")).Once()

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			require.NotNil(t, container)
			assert.Equal(t, types.ContainerID(testContainerID), container.ID())
			assert.Equal(t, types.ImageID(""), container.ImageID())
		})
	})

	t.Run("container has network mode requiring resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve network container name", func(t *testing.T) {
			t.Parallel()

			const parentID = "parent-container-id"

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/test-watchtower", true, &dockerContainer.HostConfig{
				NetworkMode: dockerContainer.NetworkMode("container:" + parentID),
			}), nil)
			expectNamed(api, parentID, "/parent-container")
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			require.NotNil(t, container)
			assert.Equal(t, dockerContainer.NetworkMode("container:/parent-container"), container.ContainerInfo().HostConfig.NetworkMode)
		})
	})

	t.Run("container has volumes-from requiring resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should rewrite volumes-from container ID to name", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/nextcloud-nginx", true, volumesFrom(appID+":rw")), nil)
			expectNamed(api, appID, "/nextcloud-app")
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{"nextcloud-app:rw"}, container.ContainerInfo().HostConfig.VolumesFrom)
			assert.Equal(t, []string{"nextcloud-app:rw"}, container.GetCreateHostConfig().VolumesFrom)
		})

		t.Run("should leave volumes-from ID when the source cannot be resolved", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/nextcloud-nginx", true, volumesFrom(appID)), nil)
			expectMissing(api, appID)
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{appID}, container.ContainerInfo().HostConfig.VolumesFrom)
		})

		t.Run("should rewrite volumes-from IDs and preserve ro mode", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/nextcloud-nginx", true, volumesFrom(appID+":ro")), nil)
			expectNamed(api, appID, "/nextcloud-app")
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{"nextcloud-app:ro"}, container.ContainerInfo().HostConfig.VolumesFrom)
		})

		t.Run("should rewrite multiple volumes-from entries independently", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/nextcloud-nginx", true, volumesFrom(appID+":rw", dbID)), nil)
			expectNamed(api, appID, "/nextcloud-app")
			expectNamed(api, dbID, "/nextcloud-db")
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{"nextcloud-app:rw", "nextcloud-db"}, container.ContainerInfo().HostConfig.VolumesFrom)
		})

		t.Run("should rewrite resolved volumes-from entries and leave unresolved IDs", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, sourceInspect("/nextcloud-nginx", true, volumesFrom(appID+":rw", missingID)), nil)
			expectNamed(api, appID, "/nextcloud-app")
			expectMissing(api, missingID)
			expectTestImage(api)

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.NoError(t, err)
			assert.Equal(t, []string{"nextcloud-app:rw", missingID}, container.ContainerInfo().HostConfig.VolumesFrom)
		})
	})

	t.Run("container ID is invalid", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error for 404 not found", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, "invalid-container-id", dockerContainer.InspectResponse{}, notFoundError("No such container"))

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, "invalid-container-id", nil)
			require.ErrorContains(t, err, "failed to inspect container")
			assert.Nil(t, container)
		})
	})

	t.Run("API connection fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error for connection timeout", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInspect(api, testContainerID, dockerContainer.InspectResponse{}, daemonError("gateway timeout"))

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.ErrorContains(t, err, "failed to inspect container")
			assert.Nil(t, container)
		})
	})

	t.Run("response is malformed", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error for invalid JSON", func(t *testing.T) {
			t.Parallel()

			// The SDK reports a response it cannot decode as an error.
			api := newDockerAPI(t)
			expectInspect(api, testContainerID, dockerContainer.InspectResponse{},
				errors.New("invalid character 'j' looking for beginning of value"))

			container, err := getSourceContainer(newTestLogger(t), t.Context(), api, testContainerID, nil)
			require.ErrorContains(t, err, "failed to inspect container")
			assert.Nil(t, container)
		})
	})
}

// Test_stopAndRemoveSourceContainer verifies that a running container is
// stopped with its signal and timeout and then force-removed, with volumes
// when asked, that a container Docker removes on exit is only stopped, and
// that stop and remove failures are reported.
func Test_stopAndRemoveSourceContainer(t *testing.T) {
	t.Parallel()

	// expectRemove expects one forced removal of the mock container and
	// answers it with err.
	expectRemove := func(api *mockContainer.MockAPIClient, removeVolumes bool, err error) {
		api.EXPECT().
			ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true, RemoveVolumes: removeVolumes}).
			Return(dockerClient.ContainerRemoveResult{}, err).Once()
	}

	// running returns a running container with the given updates.
	running := func(updates ...MockContainerUpdate) *Container {
		return MockContainer(append([]MockContainerUpdate{WithContainerState(dockerContainer.State{Running: true})}, updates...)...)
	}

	t.Run("stopping and removing a running container successfully with removeVolumes=false", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop and remove the container without error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)
			expectRemove(api, false, nil)

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, false)
			require.NoError(t, err)
		})
	})

	t.Run("stopping and removing a running container successfully with removeVolumes=true", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop and remove the container with volume removal", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)
			expectRemove(api, true, nil)

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, true)
			require.NoError(t, err)
		})
	})

	t.Run("stopping fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error without attempting removal", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, daemonError("stop failed"))

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, false)
			require.ErrorContains(t, err, "failed to stop container")
		})
	})

	t.Run("removing fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error after successful stop", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)
			expectRemove(api, false, daemonError("remove failed"))

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, false)
			require.ErrorContains(t, err, "failed to remove container")
		})
	})

	t.Run("container has AutoRemove enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop but skip removal when running", func(t *testing.T) {
			t.Parallel()

			// Docker removes an AutoRemove container after it stops, so only
			// the stop is expected.
			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(WithAutoRemove(true)), 10*time.Second, true)
			require.NoError(t, err)
		})

		t.Run("should remove explicitly when not running", func(t *testing.T) {
			t.Parallel()

			// AutoRemove does not apply to a container that never started.
			api := newDockerAPI(t)
			expectRemove(api, false, nil)

			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false, Status: "created"}),
				WithAutoRemove(true),
			)

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, container, 10*time.Second, false)
			require.NoError(t, err)
		})
	})

	t.Run("container HostConfig is nil", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop and remove without panicking", func(t *testing.T) {
			t.Parallel()

			// A nil HostConfig is not AutoRemove, so the container is removed.
			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)
			expectRemove(api, true, nil)

			container := running(func(c *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
				c.HostConfig = nil
			})

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, container, 10*time.Second, true)
			require.NoError(t, err)
		})
	})

	t.Run("container is not running", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip stop and remove successfully", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectRemove(api, true, nil)

			container := MockContainer(WithContainerState(dockerContainer.State{Running: false}))

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, container, 10*time.Second, true)
			require.NoError(t, err)
		})
	})

	t.Run("timeout is specified", func(t *testing.T) {
		t.Parallel()

		t.Run("should pass timeout to stop operation", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 30, nil)
			expectRemove(api, false, nil)

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 30*time.Second, false)
			require.NoError(t, err)
		})
	})

	t.Run("container is not found during removal", func(t *testing.T) {
		t.Parallel()

		t.Run("should succeed as container is already gone", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)
			expectRemove(api, false, notFoundError("No such container"))

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, false)
			require.NoError(t, err)
		})
	})

	t.Run("API connection fails during stop", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, daemonError("gateway timeout"))

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second, false)
			require.ErrorContains(t, err, "failed to stop container")
		})
	})
}

// Test_getNetworkConfig verifies the networking config built for a replacement
// container across network modes, client versions, and missing or partial
// network settings.
func Test_getNetworkConfig(t *testing.T) {
	t.Parallel()

	t.Run("with bridge network mode", func(t *testing.T) {
		t.Parallel()

		t.Run("should return network config with processed endpoints", func(t *testing.T) {
			t.Parallel()

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

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			require.NotNil(t, config)
			assert.Contains(t, config.EndpointsConfig, "bridge")
			endpoint := config.EndpointsConfig["bridge"]
			assert.Equal(t, "bridge_network_id", endpoint.NetworkID)
			assert.Equal(t, dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"), endpoint.MacAddress)
			assert.Equal(t, netip.MustParseAddr("172.17.0.2"), endpoint.IPAddress)
			assert.ElementsMatch(t, []string{"test.example.com"}, endpoint.DNSNames)
			require.NotNil(t, endpoint.IPAMConfig)
			assert.Equal(t, netip.MustParseAddr("172.17.0.2"), endpoint.IPAMConfig.IPv4Address)
			// Aliases should be filtered to remove container short ID
			assert.ElementsMatch(t, []string{"test-alias"}, endpoint.Aliases)
		})

		t.Run("should handle multiple networks", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithNetworks("bridge", "custom_network"),
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			require.Len(t, config.EndpointsConfig, 2)
			assert.Contains(t, config.EndpointsConfig, "bridge")
			assert.Contains(t, config.EndpointsConfig, "custom_network")
		})

		t.Run("should clear engine-generated MAC for running container without validation error", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithNetworkMode("bridge"),
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						NetworkID:  "bridge_network_id",
						MacAddress: mustParseMAC(t, "02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
					},
				}),
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			require.NotNil(t, config)
			assert.Contains(t, config.EndpointsConfig, "bridge")
			endpoint := config.EndpointsConfig["bridge"]
			assert.Equal(t, dockerNetwork.HardwareAddr{}, endpoint.MacAddress)
		})
	})

	t.Run("with host network mode", func(t *testing.T) {
		t.Parallel()

		t.Run("should return network config with cleared host-specific settings", func(t *testing.T) {
			t.Parallel()

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

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			assert.Contains(t, config.EndpointsConfig, "host")
			endpoint := config.EndpointsConfig["host"]
			assert.Equal(t, dockerNetwork.HardwareAddr{}, endpoint.MacAddress)
			assert.Equal(t, netip.Addr{}, endpoint.IPAddress)
			assert.Nil(t, endpoint.DNSNames)
			assert.Nil(t, endpoint.IPAMConfig)
			assert.Nil(t, endpoint.Aliases)
		})
	})

	t.Run("with legacy client version (< 1.44)", func(t *testing.T) {
		t.Parallel()

		t.Run("should clear MAC addresses and IP info", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						IPAddress:  netip.MustParseAddr("172.17.0.2"),
						DNSNames:   []string{"test.example.com"},
					},
				}),
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.40")

			endpoint := config.EndpointsConfig["bridge"]
			assert.Equal(t, dockerNetwork.HardwareAddr{}, endpoint.MacAddress)
			assert.Equal(t, netip.Addr{}, endpoint.IPAddress)
			assert.Nil(t, endpoint.DNSNames)
		})
	})

	t.Run("with nil network settings", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty network config", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				func(container *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					container.NetworkSettings = nil
				},
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			require.NotNil(t, config)
			assert.Empty(t, config.EndpointsConfig)
		})
	})

	t.Run("with stopped container (exited state)", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle MAC address validation for non-running containers", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: false, Status: "exited"}),
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						// No MAC address for stopped container - this is expected
					},
				}),
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			require.NotNil(t, config)
			assert.Contains(t, config.EndpointsConfig, "bridge")
		})
	})

	t.Run("with custom network settings", func(t *testing.T) {
		t.Parallel()

		t.Run("should preserve custom network configurations", func(t *testing.T) {
			t.Parallel()

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

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			endpoint := config.EndpointsConfig["my_custom_network"]
			assert.Equal(t, "custom_net_id", endpoint.NetworkID)
			assert.Equal(t, dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"), endpoint.MacAddress)
			assert.Equal(t, netip.MustParseAddr("10.0.0.5"), endpoint.IPAddress)
			assert.ElementsMatch(t, []string{"custom-alias"}, endpoint.Aliases)
			assert.ElementsMatch(t, []string{"other_container:alias"}, endpoint.Links)
		})
	})

	t.Run("with Podman-style client version", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle Podman API version strings", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				}),
			)

			// Test with Podman version format
			config := getNetworkConfig(newTestLogger(t), container, "4.0.0")

			endpoint := config.EndpointsConfig["bridge"]
			// Should preserve MAC for modern versions
			assert.Equal(t, dockerNetwork.HardwareAddr("02:42:ac:11:00:02"), endpoint.MacAddress)
		})
	})

	t.Run("with nil endpoint in network settings", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip nil endpoints", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
					"bridge":    nil,
					"valid_net": {NetworkID: "valid_id"},
				}),
			)

			config := getNetworkConfig(newTestLogger(t), container, "1.50")

			assert.Contains(t, config.EndpointsConfig, "valid_net")
			assert.NotContains(t, config.EndpointsConfig, "bridge")
		})
	})
}

// Test_newEmptyNetworkConfig verifies that an empty networking config has a
// usable, empty endpoints map.
func Test_newEmptyNetworkConfig(t *testing.T) {
	t.Parallel()

	t.Run("should create a non-nil network config", func(t *testing.T) {
		t.Parallel()

		config := newEmptyNetworkConfig()
		require.NotNil(t, config)
	})

	t.Run("should initialize EndpointsConfig as an empty map", func(t *testing.T) {
		t.Parallel()

		config := newEmptyNetworkConfig()
		require.NotNil(t, config.EndpointsConfig)
		assert.Empty(t, config.EndpointsConfig)
		assert.Equal(t, make(map[string]*dockerNetwork.EndpointSettings), config.EndpointsConfig)
	})

	t.Run("should return a properly structured NetworkingConfig", func(t *testing.T) {
		t.Parallel()

		config := newEmptyNetworkConfig()
		assert.IsType(t, &dockerNetwork.NetworkingConfig{}, config)
		assert.IsType(t, make(map[string]*dockerNetwork.EndpointSettings), config.EndpointsConfig)
	})

	t.Run("should be ready for use - can add endpoints to the map", func(t *testing.T) {
		t.Parallel()

		config := newEmptyNetworkConfig()
		endpoint := &dockerNetwork.EndpointSettings{
			NetworkID: "test-network",
		}
		config.EndpointsConfig["test"] = endpoint
		require.Len(t, config.EndpointsConfig, 1)
		assert.Equal(t, endpoint, config.EndpointsConfig["test"])
	})
}

// Test_processEndpoint verifies which endpoint settings are copied to a
// replacement container, including when an engine-generated MAC address is
// cleared, for modern, legacy, and host-network configurations.
func Test_processEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("with modern API version (>= 1.44)", func(t *testing.T) {
		t.Parallel()

		clientVersion := "1.50"

		t.Run("and non-host network mode", func(t *testing.T) {
			t.Parallel()

			isHostNetwork := false

			t.Run("should preserve MAC address, IP address, and DNS names", func(t *testing.T) {
				t.Parallel()

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

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, "bridge_network_id", result.NetworkID)
				assert.Equal(t, dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"), result.MacAddress)
				assert.Equal(t, netip.MustParseAddr("172.17.0.2"), result.IPAddress)
				assert.ElementsMatch(t, []string{"test.example.com"}, result.DNSNames)
				require.NotNil(t, result.IPAMConfig)
				assert.Equal(t, netip.MustParseAddr("172.17.0.2"), result.IPAMConfig.IPv4Address)
				// Aliases should be filtered to remove container short ID
				assert.ElementsMatch(t, []string{"test-alias"}, result.Aliases)
			})

			t.Run("should clear engine-generated MAC when it matches the endpoint IP", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC(t, "02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, dockerNetwork.HardwareAddr{}, result.MacAddress)
			})

			t.Run("should preserve user-configured MAC that does not match the endpoint IP", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, dockerNetwork.HardwareAddr("aa:bb:cc:dd:ee:ff"), result.MacAddress)
			})

			t.Run("should preserve engine-generated MAC when endpoint IP is empty", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC(t, "02:42:ac:11:00:02"),
					IPAddress:  netip.Addr{},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, mustParseMAC(t, "02:42:ac:11:00:02"), result.MacAddress)
			})

			t.Run("should preserve engine-generated MAC when endpoint IP is IPv6", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC(t, "02:42:ac:11:00:02"),
					IPAddress:  netip.MustParseAddr("::1"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, mustParseMAC(t, "02:42:ac:11:00:02"), result.MacAddress)
			})

			t.Run("should clear empty MAC without error", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: dockerNetwork.HardwareAddr{},
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, dockerNetwork.HardwareAddr{}, result.MacAddress)
			})

			t.Run("should preserve user-configured MAC matching engine-generated prefix with different IP", func(t *testing.T) {
				t.Parallel()

				// A user-configured MAC that happens to start with 02:42 but whose last
				// four bytes do not match the endpoint's IP must not be cleared.
				sourceEndpoint := &dockerNetwork.EndpointSettings{
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC(t, "02:42:de:ad:be:ef"),
					IPAddress:  netip.MustParseAddr("172.17.0.2"),
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, mustParseMAC(t, "02:42:de:ad:be:ef"), result.MacAddress)
			})

			t.Run("should handle multiple aliases correctly", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Aliases: []string{"container_id", "alias1", "alias2", "other_id"},
				}
				containerID := types.ContainerID("container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.ElementsMatch(t, []string{"alias1", "alias2", "other_id"}, result.Aliases)
			})

			t.Run("should copy all IPAM config fields", func(t *testing.T) {
				t.Parallel()

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

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				require.NotNil(t, result.IPAMConfig)
				assert.Equal(t, netip.MustParseAddr("192.168.1.100"), result.IPAMConfig.IPv4Address)
				assert.Equal(t, netip.MustParseAddr("2001:db8::1"), result.IPAMConfig.IPv6Address)
				assert.ElementsMatch(t, []netip.Addr{netip.MustParseAddr("169.254.1.1"), netip.MustParseAddr("169.254.1.2")}, result.IPAMConfig.LinkLocalIPs)
			})

			t.Run("should preserve empty aliases list", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Aliases: []string{},
				}
				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Empty(t, result.Aliases)
			})

			t.Run("should preserve other endpoint fields", func(t *testing.T) {
				t.Parallel()

				sourceEndpoint := &dockerNetwork.EndpointSettings{
					Gateway:    netip.MustParseAddr("172.17.0.1"),
					Links:      []string{"other_container:alias"},
					EndpointID: "endpoint_id",
				}
				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, netip.MustParseAddr("172.17.0.1"), result.Gateway)
				assert.ElementsMatch(t, []string{"other_container:alias"}, result.Links)
				assert.Equal(t, "endpoint_id", result.EndpointID)
			})
		})
	})

	t.Run("with legacy API version (< 1.44)", func(t *testing.T) {
		t.Parallel()

		clientVersion := "1.40"

		t.Run("and non-host network mode", func(t *testing.T) {
			t.Parallel()

			isHostNetwork := false

			t.Run("should clear MAC address, IP address, and DNS names", func(t *testing.T) {
				t.Parallel()

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

				result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
					containerID,
					clientVersion,
					isHostNetwork,
				)
				require.NoError(t, err)

				assert.Equal(t, dockerNetwork.HardwareAddr{}, result.MacAddress)
				assert.Equal(t, netip.Addr{}, result.IPAddress)
				assert.Nil(t, result.DNSNames)
				require.NotNil(t, result.IPAMConfig)
				assert.Equal(t, netip.MustParseAddr("172.17.0.2"), result.IPAMConfig.IPv4Address)
				// Aliases should still be filtered
				assert.ElementsMatch(t, []string{"test-alias"}, result.Aliases)
			})
		})
	})

	t.Run("with host network mode", func(t *testing.T) {
		t.Parallel()

		isHostNetwork := true

		t.Run("should clear aliases regardless of API version", func(t *testing.T) {
			t.Parallel()

			sourceEndpoint := &dockerNetwork.EndpointSettings{
				Aliases: []string{"container_id", "test-alias"},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			require.NoError(t, err)

			assert.Nil(t, result.Aliases)
		})

		t.Run("should clear IPAM config regardless of API version", func(t *testing.T) {
			t.Parallel()

			sourceEndpoint := &dockerNetwork.EndpointSettings{
				IPAMConfig: &dockerNetwork.EndpointIPAMConfig{
					IPv4Address: netip.MustParseAddr("192.168.1.100"),
				},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			require.NoError(t, err)

			assert.Nil(t, result.IPAMConfig)
		})

		t.Run("should clear MAC and IP addresses even with modern API", func(t *testing.T) {
			t.Parallel()

			sourceEndpoint := &dockerNetwork.EndpointSettings{
				MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
				IPAddress:  netip.MustParseAddr("192.168.1.100"),
				DNSNames:   []string{"host.example.com"},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
				containerID,
				"1.50",
				isHostNetwork,
			)
			require.NoError(t, err)

			assert.Equal(t, dockerNetwork.HardwareAddr{}, result.MacAddress)
			assert.Equal(t, netip.Addr{}, result.IPAddress)
			assert.Nil(t, result.DNSNames)
		})
	})

	t.Run("with edge cases", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle nil IPAM config gracefully", func(t *testing.T) {
			t.Parallel()

			sourceEndpoint := &dockerNetwork.EndpointSettings{
				IPAMConfig: nil,
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
				containerID,
				"1.50",
				false,
			)
			require.NoError(t, err)

			assert.Nil(t, result.IPAMConfig)
		})

		t.Run("should handle empty DNS names", func(t *testing.T) {
			t.Parallel()

			sourceEndpoint := &dockerNetwork.EndpointSettings{
				DNSNames: []string{},
			}
			containerID := types.ContainerID("test_container_id")

			result, err := processEndpoint(newTestLogger(t), sourceEndpoint,
				containerID,
				"1.50",
				false,
			)
			require.NoError(t, err)

			assert.Empty(t, result.DNSNames)
		})

		t.Run("with nil source endpoint", func(t *testing.T) {
			t.Parallel()

			t.Run("should return ErrNilSourceEndpoint when sourceEndpoint is nil", func(t *testing.T) {
				t.Parallel()

				containerID := types.ContainerID("test_container_id")

				result, err := processEndpoint(newTestLogger(t), nil,
					containerID,
					"1.50",
					false,
				)
				require.Error(t, err)
				require.ErrorIs(t, err, errNilSourceEndpoint)
				assert.Nil(t, result)
			})
		})
	})
}

// Test_isEngineGeneratedMAC verifies that only a MAC address derived from the
// endpoint's IPv4 address counts as generated by the engine.
func Test_isEngineGeneratedMAC(t *testing.T) {
	t.Parallel()

	t.Run("should return true for MAC derived from the endpoint IPv4 address", func(t *testing.T) {
		t.Parallel()

		mac := mustParseMAC(t, "02:42:ac:11:00:02")
		ip := netip.MustParseAddr("172.17.0.2")

		assert.True(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return true for a MAC unmarshaled from Docker inspect JSON", func(t *testing.T) {
		t.Parallel()

		var endpoint dockerNetwork.EndpointSettings

		err := json.Unmarshal(
			[]byte(`{"MacAddress":"02:42:ac:11:00:02","IPAddress":"172.17.0.2"}`),
			&endpoint,
		)
		require.NoError(t, err)
		require.Len(t, endpoint.MacAddress, 6)
		assert.True(t, isEngineGeneratedMAC(endpoint.MacAddress, endpoint.IPAddress))
	})

	t.Run("should return false for colon-separated ASCII bytes that are not a parsed HardwareAddr", func(t *testing.T) {
		t.Parallel()

		mac := dockerNetwork.HardwareAddr("02:42:ac:11:00:02")
		ip := netip.MustParseAddr("172.17.0.2")

		require.Len(t, mac, 17)
		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return false for user-configured MAC with matching prefix but different IP", func(t *testing.T) {
		t.Parallel()

		mac := mustParseMAC(t, "02:42:de:ad:be:ef")
		ip := netip.MustParseAddr("172.17.0.2")

		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return false when MAC is empty", func(t *testing.T) {
		t.Parallel()

		mac := dockerNetwork.HardwareAddr{}
		ip := netip.MustParseAddr("172.17.0.2")

		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return false when IP is empty", func(t *testing.T) {
		t.Parallel()

		mac := mustParseMAC(t, "02:42:ac:11:00:02")
		ip := netip.Addr{}

		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return false when IP is IPv6", func(t *testing.T) {
		t.Parallel()

		mac := mustParseMAC(t, "02:42:ac:11:00:02")
		ip := netip.MustParseAddr("::1")

		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return false when MAC length is not 6", func(t *testing.T) {
		t.Parallel()

		mac := dockerNetwork.HardwareAddr("02:42:ac:11")
		ip := netip.MustParseAddr("172.17.0.2")

		assert.False(t, isEngineGeneratedMAC(mac, ip))
	})

	t.Run("should return true for the reproduction case from issue #2104", func(t *testing.T) {
		t.Parallel()

		mac := mustParseMAC(t, "02:42:0a:63:00:02")
		ip := netip.MustParseAddr("10.99.0.2")

		assert.True(t, isEngineGeneratedMAC(mac, ip))
	})
}

// Test_onlyGeneratedMacs verifies that a container counts as having only
// engine-generated MAC addresses when every original endpoint is covered and
// none has a user-configured MAC.
func Test_onlyGeneratedMacs(t *testing.T) {
	t.Parallel()

	t.Run("should return true when all original MACs are engine-generated and all endpoints are covered", func(t *testing.T) {
		t.Parallel()

		container := MockContainer(
			WithNetworkSettings(map[string]*dockerNetwork.EndpointSettings{
				"bridge": {
					NetworkID:  "bridge_network_id",
					MacAddress: mustParseMAC(t, "02:42:ac:11:00:02"),
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

		assert.True(t, onlyGeneratedMacs(container, processedConfig))
	})

	t.Run("should return false when a non-nil original endpoint is missing from the processed config", func(t *testing.T) {
		t.Parallel()

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

		assert.False(t, onlyGeneratedMacs(container, processedConfig))
	})

	t.Run("should return false when there is a user-configured MAC", func(t *testing.T) {
		t.Parallel()

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

		assert.False(t, onlyGeneratedMacs(container, processedConfig))
	})

	t.Run("should return false when there are no MACs at all", func(t *testing.T) {
		t.Parallel()

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

		assert.False(t, onlyGeneratedMacs(container, processedConfig))
	})

	t.Run("should return false when container info is nil", func(t *testing.T) {
		t.Parallel()

		processedConfig := &dockerNetwork.NetworkingConfig{}

		assert.False(t, onlyGeneratedMacs(nil, processedConfig))
	})
}

// Test_validateMacAddresses verifies where a MAC address in the networking
// config is expected or an error, by client version, network mode, and
// container state.
func Test_validateMacAddresses(t *testing.T) {
	t.Parallel()

	t.Run("with legacy API version (< 1.44)", func(t *testing.T) {
		t.Parallel()

		clientVersion := "1.40"

		t.Run("and non-host network mode", func(t *testing.T) {
			t.Parallel()

			isHostNetwork := false

			t.Run("should succeed when no MAC address is present", func(t *testing.T) {
				t.Parallel()

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

				err := validateMacAddresses(newTestLogger(t), config,
					container.ID(),
					clientVersion,
					isHostNetwork,
					container,
				)
				require.NoError(t, err)
			})

			t.Run("should return error when unexpected MAC address is present", func(t *testing.T) {
				t.Parallel()

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

				err := validateMacAddresses(newTestLogger(t), config,
					container.ID(),
					clientVersion,
					isHostNetwork,
					container,
				)
				require.Error(t, err)
				require.ErrorContains(t, err, "unexpected MAC address in legacy config")
				require.ErrorContains(t, err, "API version 1.40")
			})
		})
	})

	t.Run("with host network mode", func(t *testing.T) {
		t.Parallel()

		isHostNetwork := true

		t.Run("should succeed when no MAC address is present", func(t *testing.T) {
			t.Parallel()

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

			err := validateMacAddresses(newTestLogger(t), config,
				container.ID(),
				"1.50",
				isHostNetwork,
				container,
			)
			require.NoError(t, err)
		})

		t.Run("should return error when unexpected MAC address is present", func(t *testing.T) {
			t.Parallel()

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

			err := validateMacAddresses(newTestLogger(t), config,
				container.ID(),
				"1.50",
				isHostNetwork,
				container,
			)
			require.Error(t, err)
			require.ErrorIs(t, err, errUnexpectedMacInHost)
		})
	})

	t.Run("with modern API version (>= 1.44)", func(t *testing.T) {
		t.Parallel()

		clientVersion := "1.50"

		t.Run("and non-host network mode", func(t *testing.T) {
			t.Parallel()

			isHostNetwork := false

			t.Run("and running container", func(t *testing.T) {
				t.Parallel()

				t.Run("should succeed when MAC address is present", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.NoError(t, err)
				})

				t.Run("should return error when MAC address is missing", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.Error(t, err)
					require.ErrorIs(t, err, errNoMacInNonHost)
				})

				t.Run("should succeed with multiple networks all having MAC addresses", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.NoError(t, err)
				})

				t.Run("should succeed when at least one of multiple networks has MAC address", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.NoError(t, err)
				})
			})

			t.Run("and non-running container", func(t *testing.T) {
				t.Parallel()

				t.Run("should succeed when MAC address is missing (created state)", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.NoError(t, err)
				})

				t.Run("should succeed when MAC address is missing (exited state)", func(t *testing.T) {
					t.Parallel()

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

					err := validateMacAddresses(newTestLogger(t), config,
						container.ID(),
						clientVersion,
						isHostNetwork,
						container,
					)
					require.NoError(t, err)
				})
			})
		})
	})

	t.Run("with edge cases", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle empty network configuration", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
			)
			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
			}

			err := validateMacAddresses(newTestLogger(t), config,
				container.ID(),
				"1.50",
				false,
				container,
			)
			require.Error(t, err)
			require.ErrorIs(t, err, errNoMacInNonHost)
		})

		t.Run("should handle nil network configuration", func(t *testing.T) {
			t.Parallel()

			container := MockContainer(
				WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
			)

			err := validateMacAddresses(newTestLogger(t), nil,
				container.ID(),
				"1.50",
				false,
				container,
			)
			require.Error(t, err)
			require.ErrorIs(t, err, errNoMacInNonHost)
		})
	})
}

// Test_filterAliases verifies that only exact matches of the container's short
// ID are removed from its network aliases.
func Test_filterAliases(t *testing.T) {
	t.Parallel()

	t.Run("removes container short ID from aliases", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{testShortID, "custom-alias", "another-alias"}
		result := filterAliases(aliases, shortID)
		assert.ElementsMatch(t, []string{"custom-alias", "another-alias"}, result)
	})

	t.Run("preserves all aliases when short ID is not present", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{"custom-alias", "another-alias", "third-alias"}
		result := filterAliases(aliases, shortID)
		assert.ElementsMatch(t, []string{"custom-alias", "another-alias", "third-alias"}, result)
	})

	t.Run("handles empty alias list", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{}
		result := filterAliases(aliases, shortID)
		assert.Empty(t, result)
	})

	t.Run("handles nil alias list", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID

		var aliases []string

		result := filterAliases(aliases, shortID)
		assert.Empty(t, result)
	})

	t.Run("removes multiple instances of short ID", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{testShortID, "custom-alias", testShortID, "another-alias"}
		result := filterAliases(aliases, shortID)
		assert.ElementsMatch(t, []string{"custom-alias", "another-alias"}, result)
	})

	t.Run("returns empty list when all aliases are the short ID", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{testShortID, testShortID}
		result := filterAliases(aliases, shortID)
		assert.Empty(t, result)
	})

	t.Run("does not remove partial matches", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{"abc123def4567", "abc123def45", testShortID, "custom-alias"}
		result := filterAliases(aliases, shortID)
		assert.ElementsMatch(t, []string{"abc123def4567", "abc123def45", "custom-alias"}, result)
	})

	t.Run("handles case-sensitive matching", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{"ABC123DEF456", testShortID, "Abc123Def456"}
		result := filterAliases(aliases, shortID)
		assert.ElementsMatch(t, []string{"ABC123DEF456", "Abc123Def456"}, result)
	})

	t.Run("preserves order of remaining aliases", func(t *testing.T) {
		t.Parallel()

		shortID := testShortID
		aliases := []string{"first", testShortID, "second", "third", testShortID}
		result := filterAliases(aliases, shortID)
		assert.Equal(t, []string{"first", "second", "third"}, result)
	})
}

// Test_stopSourceContainer verifies that a running container is stopped with
// its signal and timeout, that a stopped one is left alone, and that stop
// failures, including a container already gone, are reported.
func Test_stopSourceContainer(t *testing.T) {
	t.Parallel()

	// running returns a running container.
	running := func() *Container {
		return MockContainer(WithContainerState(dockerContainer.State{Running: true}))
	}

	t.Run("stopping a linked restarting container", func(t *testing.T) {
		t.Parallel()

		t.Run("should log 'Stopping linked container' and succeed", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)

			container := MockContainer()
			container.SetLinkedToRestarting(true)

			log, logbuf := newCapturedLogger(zerolog.InfoLevel)

			err := stopSourceContainer(log, t.Context(), api, container, 10*time.Second)
			require.NoError(t, err)
			assert.Contains(t, logbuf.String(), "Stopping linked container")
		})
	})

	t.Run("stopping a running container successfully", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop the container without error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, nil)

			err := stopSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second)
			require.NoError(t, err)
		})
	})

	t.Run("stopping a container that fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, daemonError("stop failed"))

			err := stopSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second)
			require.ErrorContains(t, err, "failed to stop container")
		})
	})

	t.Run("stopping a container with timeout parameter", func(t *testing.T) {
		t.Parallel()

		t.Run("should pass the timeout to the Docker API", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 30, nil)

			err := stopSourceContainer(newTestLogger(t), t.Context(), api, running(), 30*time.Second)
			require.NoError(t, err)
		})
	})

	t.Run("stopping a container that is not running", func(t *testing.T) {
		t.Parallel()

		t.Run("should not attempt to stop and return no error", func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so any Docker call fails the test.
			api := mockContainer.NewMockAPIClient(t)
			container := MockContainer(WithContainerState(dockerContainer.State{Running: false}))

			err := stopSourceContainer(newTestLogger(t), t.Context(), api, container, 10*time.Second)
			require.NoError(t, err)
		})
	})

	t.Run("stopping a container that was removed between check and stop", func(t *testing.T) {
		t.Parallel()

		t.Run("should return a not-found error when Docker returns 404 NotFound", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectStop(api, mockContainerID, 10, notFoundError("No such container"))

			err := stopSourceContainer(newTestLogger(t), t.Context(), api, running(), 10*time.Second)
			require.Error(t, err)
			assert.True(t, cerrdefs.IsNotFound(err))
		})
	})
}

// Test_debugLogMacAddress verifies the MAC address diagnostics logged for each
// client version, network mode, and networking config shape.
func Test_debugLogMacAddress(t *testing.T) {
	t.Parallel()

	t.Run("with legacy API version (< minimum supported version)", func(t *testing.T) {
		t.Parallel()

		minSupportedVersion := "1.44"

		t.Run("and MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'Unexpected MAC address in legacy config'", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				assert.Contains(t, logbuf.String(), "Unexpected MAC address in legacy config")
			})
		})

		t.Run("and no MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'No MAC address in legacy config, Docker will handle'", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {NetworkID: "bridge_network_id"},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				assert.Contains(t, logbuf.String(), "No MAC address in legacy config, Docker will handle")
			})
		})

		t.Run("and host network mode", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'Unexpected MAC address in legacy config' when MAC present", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"host": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					true,
				)

				assert.Contains(t, logbuf.String(), "Unexpected MAC address in legacy config")
			})
		})
	})

	t.Run("with API version < 1.44 and not host network", func(t *testing.T) {
		t.Parallel()

		minSupportedVersion := "1.39" // Lower than 1.40 so first case doesn't match

		t.Run("and MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'Unexpected MAC address in legacy config'", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				assert.Contains(t, logbuf.String(), "Unexpected MAC address in legacy config")
			})
		})

		t.Run("and no MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'No MAC address in legacy config, as expected'", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {NetworkID: "bridge_network_id"},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.40",
					minSupportedVersion,
					false,
				)

				assert.Contains(t, logbuf.String(), "No MAC address in legacy config, as expected")
			})
		})
	})

	t.Run("with modern API version (>= 1.44)", func(t *testing.T) {
		t.Parallel()

		minSupportedVersion := "1.44"

		t.Run("and MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("should log 'Verified MAC address configuration'", func(t *testing.T) {
				t.Parallel()

				config := &dockerNetwork.NetworkingConfig{
					EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
						"bridge": {
							MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
						},
					},
				}
				containerID := types.ContainerID("test-container")

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.50",
					minSupportedVersion,
					false,
				)

				assert.Contains(t, logbuf.String(), "Verified MAC address configuration")
			})

			t.Run("should log MAC address details for each network", func(t *testing.T) {
				t.Parallel()

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

				log, logbuf := newCapturedLogger(zerolog.DebugLevel)

				debugLogMacAddress(log, config,
					containerID,
					"1.50",
					minSupportedVersion,
					false,
				)

				assert.Regexp(t, `(?s)Found MAC address in config.*network.*mac_address`, logbuf.String())
			})
		})

		t.Run("and no MAC address present", func(t *testing.T) {
			t.Parallel()

			t.Run("and non-host network mode", func(t *testing.T) {
				t.Parallel()

				t.Run("should log 'No MAC address found in config'", func(t *testing.T) {
					t.Parallel()

					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"bridge": {NetworkID: "bridge_network_id"},
						},
					}
					containerID := types.ContainerID("test-container")

					log, logbuf := newCapturedLogger(zerolog.DebugLevel)

					debugLogMacAddress(log, config, containerID, "1.50", minSupportedVersion, false)

					assert.Contains(t, logbuf.String(), "No MAC address found in config")
				})
			})

			t.Run("and host network mode", func(t *testing.T) {
				t.Parallel()

				t.Run("should log 'No MAC address in host network mode, as expected'", func(t *testing.T) {
					t.Parallel()

					config := &dockerNetwork.NetworkingConfig{
						EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
							"host": {NetworkID: "host_network_id"},
						},
					}
					containerID := types.ContainerID("test-container")

					log, logbuf := newCapturedLogger(zerolog.DebugLevel)

					debugLogMacAddress(log, config,
						containerID,
						"1.50",
						minSupportedVersion,
						true,
					)

					assert.Contains(t, logbuf.String(), "No MAC address in host network mode, as expected")
				})
			})
		})
	})

	t.Run("with nil network config", func(t *testing.T) {
		t.Parallel()

		t.Run("should execute without panicking and log appropriate message", func(t *testing.T) {
			t.Parallel()

			containerID := types.ContainerID("test-container")

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)

			assert.NotPanics(t, func() {
				debugLogMacAddress(log, nil, containerID, "1.50", "1.44", false)
			})

			// Should log that no MAC address was found
			assert.Contains(t, logbuf.String(), "No MAC address found in config")
		})
	})

	t.Run("with empty endpoints config", func(t *testing.T) {
		t.Parallel()

		t.Run("should log appropriate message for non-host network", func(t *testing.T) {
			t.Parallel()

			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.50", "1.44", false)

			assert.Contains(t, logbuf.String(), "No MAC address found in config")
		})
	})

	t.Run("with multiple networks", func(t *testing.T) {
		t.Parallel()

		t.Run("should check all networks for MAC addresses", func(t *testing.T) {
			t.Parallel()

			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
					"custom_network": {NetworkID: "custom_net_id"},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.50", "1.44", false)

			assert.Regexp(t, `(?s)Found MAC address in config.*Verified MAC address configuration`, logbuf.String())
		})
	})

	t.Run("with different client version formats", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle Podman-style version strings", func(t *testing.T) {
			t.Parallel()

			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "4.0.0", "1.44", false)

			assert.Contains(t, logbuf.String(), "Verified MAC address configuration")
		})

		t.Run("should handle version strings with patch levels", func(t *testing.T) {
			t.Parallel()

			config := &dockerNetwork.NetworkingConfig{
				EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{
					"bridge": {
						MacAddress: dockerNetwork.HardwareAddr("02:42:ac:11:00:02"),
					},
				},
			}
			containerID := types.ContainerID("test-container")

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)

			debugLogMacAddress(log, config, containerID, "1.45.0", "1.44", false)

			assert.Contains(t, logbuf.String(), "Verified MAC address configuration")
		})
	})
}

// TestIsWatchtowerParent verifies that a container ID is found anywhere in a
// comma-separated container chain, with or without spaces.
func TestIsWatchtowerParent(t *testing.T) {
	t.Parallel()

	t.Run("should return false for empty chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("test-id"), "")
		assert.False(t, result)
	})

	t.Run("should return false for empty ID", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID(""), "id1,id2")
		assert.False(t, result)
	})

	t.Run("should return true when ID is at start of chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("test-id"), "test-id,id2,id3")
		assert.True(t, result)
	})

	t.Run("should return true when ID is in middle of chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("id2"), "id1,id2,id3")
		assert.True(t, result)
	})

	t.Run("should return true when ID is at end of chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("id3"), "id1,id2,id3")
		assert.True(t, result)
	})

	t.Run("should return true for single ID chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("test-id"), "test-id")
		assert.True(t, result)
	})

	t.Run("should return false when ID is not in chain", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("test-id"), "id1,id2,id3")
		assert.False(t, result)
	})

	t.Run("should handle chains with spaces around commas", func(t *testing.T) {
		t.Parallel()

		result := IsWatchtowerParent(types.ContainerID("id2"), "id1, id2 , id3")
		assert.True(t, result)
	})
}

// getStatusFilterKeys returns the statuses a container list filter requests.
//
// Parameters:
//   - f: Filter to read.
//
// Returns:
//   - []string: Requested statuses, in no particular order.
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

// newImageInspectFailAPI returns a mocked Docker API that lists one running
// container and fails the inspect of its image with err.
//
// Parameters:
//   - t: Test the mock belongs to.
//   - err: Error the image inspect fails with.
//
// Returns:
//   - *mockContainer.MockAPIClient: Mocked Docker API.
func newImageInspectFailAPI(t *testing.T, err error) *mockContainer.MockAPIClient {
	t.Helper()

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().ClientVersion().Return("1.44").Maybe()
	api.EXPECT().ContainerList(mock.Anything, mock.Anything).
		Return(dockerClient.ContainerListResult{Items: []dockerContainer.Summary{{ID: testContainerID}}}, nil).Once()
	api.EXPECT().ContainerInspect(mock.Anything, testContainerID, dockerClient.ContainerInspectOptions{}).
		Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
			ID:         testContainerID,
			Name:       "/app",
			Image:      "sha256:image",
			State:      &dockerContainer.State{Running: true},
			Config:     &dockerContainer.Config{Image: "app:latest"},
			HostConfig: &dockerContainer.HostConfig{},
		}}, nil).Once()
	api.EXPECT().ImageInspect(mock.Anything, "sha256:image").
		Return(dockerClient.ImageInspectResult{}, err).Once()

	return api
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
			api := newImageInspectFailAPI(t, tt.inspectErr)

			containers, err := listSourceContainers(&log, t.Context(), api, ClientOptions{}, nil)

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

// expectStop expects one stop of the container with the default stop signal
// and the given timeout, and answers it with err.
//
// Parameters:
//   - api: Mocked Docker API.
//   - containerID: ID of the container to stop.
//   - timeout: Stop timeout in seconds.
//   - err: Error the stop returns, nil for a successful stop.
func expectStop(api *mockContainer.MockAPIClient, containerID string, timeout int, err error) {
	api.EXPECT().ContainerStop(mock.Anything, containerID, dockerClient.ContainerStopOptions{
		Signal:  defaultStopSignal,
		Timeout: &timeout,
	}).Return(dockerClient.ContainerStopResult{}, err).Once()
}

// stopRemoveContainer returns a container in the given state for the stop and
// remove tests.
func stopRemoveContainer(running, autoRemove bool) *Container {
	return NewContainer(nil, &dockerContainer.InspectResponse{
		ID:         testContainerID,
		Name:       "/app",
		State:      &dockerContainer.State{Running: running},
		Config:     &dockerContainer.Config{Image: "app:latest"},
		HostConfig: &dockerContainer.HostConfig{AutoRemove: autoRemove},
	}, nil)
}

// TestStopAndRemoveSourceContainer_NotFound verifies that a container which no
// longer exists before Watchtower stops or removes it is reported as not
// found, while a container that disappears only after Watchtower stopped it,
// such as one Docker removes on exit, counts as stopped and removed.
func TestStopAndRemoveSourceContainer_NotFound(t *testing.T) {
	t.Parallel()

	notFound := fmt.Errorf("no such container: %w", cerrdefs.ErrNotFound)

	tests := []struct {
		name       string
		running    bool
		autoRemove bool
		stopErr    error
		removeErr  error
		// wantNotFound expects a not-found error. Otherwise no error is expected.
		wantNotFound bool
		// wantRemoved expects the container to be removed.
		wantRemoved bool
	}{
		{name: "running container gone before stop", running: true, stopErr: notFound, wantNotFound: true},
		{name: "running container removed after stop", running: true, removeErr: notFound, wantRemoved: true},
		{name: "stopped container gone before remove", removeErr: notFound, wantNotFound: true, wantRemoved: true},
		{name: "stopped container removed", wantRemoved: true},
		{name: "running auto-remove container", running: true, autoRemove: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := mockContainer.NewMockAPIClient(t)
			if tt.running {
				expectStop(api, testContainerID, 1, tt.stopErr)
			}

			if tt.wantRemoved {
				api.EXPECT().
					ContainerRemove(mock.Anything, testContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, tt.removeErr).Once()
			}

			err := stopAndRemoveSourceContainer(newTestLogger(t), t.Context(), api,
				stopRemoveContainer(tt.running, tt.autoRemove), time.Second, false)

			if tt.wantNotFound {
				require.Error(t, err)
				assert.True(t, cerrdefs.IsNotFound(err), "error: %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestStopSourceContainer_NotFound verifies that stopping a container which no
// longer exists reports it as not found.
func TestStopSourceContainer_NotFound(t *testing.T) {
	t.Parallel()

	api := mockContainer.NewMockAPIClient(t)
	expectStop(api, testContainerID, 1, fmt.Errorf("no such container: %w", cerrdefs.ErrNotFound))

	err := stopSourceContainer(newTestLogger(t), t.Context(), api, stopRemoveContainer(true, false), time.Second)
	require.Error(t, err)
	assert.True(t, cerrdefs.IsNotFound(err), "error: %v", err)
}
