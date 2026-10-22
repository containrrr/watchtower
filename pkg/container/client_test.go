package container

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerSystem "github.com/moby/moby/api/types/system"
	dockerClient "github.com/moby/moby/client"

	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// mockContainerID is the ID of a container built by MockContainer.
const mockContainerID = "container_id"

// execCase configures the mocked Docker calls of one ExecuteCommand run.
type execCase struct {
	// createErr fails the exec create.
	createErr error
	// startErr fails the exec start.
	startErr error
	// containerID is the container the command runs in.
	containerID string
	// execID is the ID of the created exec instance.
	execID string
	// cmd is the command to run.
	cmd string
	// user is the exec user the command is expected to run as.
	user string
	// exitCode is the exit code the exec reports.
	exitCode int
}

// expectExec expects GetContainer's inspects of the container and its image,
// then the exec create, start, attach, and inspect of an ExecuteCommand run,
// stopping after a configured failure.
//
// The attach fails, so the run continues without captured output.
//
// Parameters:
//   - api: Mocked Docker API.
//   - tc: The run to expect.
func expectExec(api *mockContainer.MockAPIClient, tc execCase) {
	const imageName = "test-image:latest"

	api.EXPECT().ContainerInspect(mock.Anything, tc.containerID, dockerClient.ContainerInspectOptions{}).
		Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
			ID:         tc.containerID,
			Name:       "/test-container",
			Image:      imageName,
			State:      &dockerContainer.State{Status: "running"},
			HostConfig: &dockerContainer.HostConfig{},
			Config: &dockerContainer.Config{
				Image:  imageName,
				Labels: map[string]string{},
			},
		}}, nil).Once()
	api.EXPECT().ImageInspect(mock.Anything, imageName).
		Return(dockerClient.ImageInspectResult{ID: "test-image-id"}, nil).Once()

	metadata := fmt.Sprintf(
		`WT_CONTAINER={"name":"test-container","id":"%s","image_name":"%s","stop_signal":"SIGTERM","labels":{}}`,
		tc.containerID, imageName,
	)

	api.EXPECT().ExecCreate(mock.Anything, tc.containerID, dockerClient.ExecCreateOptions{
		User:         tc.user,
		TTY:          true,
		Cmd:          []string{"sh", "-c", tc.cmd},
		Env:          []string{metadata},
		AttachStdout: true,
		AttachStderr: true,
	}).Return(dockerClient.ExecCreateResult{ID: tc.execID}, tc.createErr).Once()

	if tc.createErr != nil {
		return
	}

	// ExecStart must detach, or the daemon blocks until the command finishes
	// and the attach that follows fails with "exec command is already running".
	api.EXPECT().ExecStart(mock.Anything, tc.execID, dockerClient.ExecStartOptions{Detach: true, TTY: true}).
		Return(dockerClient.ExecStartResult{}, tc.startErr).Once()

	if tc.startErr != nil {
		return
	}

	api.EXPECT().ExecAttach(mock.Anything, tc.execID, dockerClient.ExecAttachOptions{TTY: true}).
		Return(dockerClient.ExecAttachResult{}, errors.New("cannot hijack the connection")).Once()
	api.EXPECT().ExecInspect(mock.Anything, tc.execID, dockerClient.ExecInspectOptions{}).
		Return(dockerClient.ExecInspectResult{ID: tc.execID, ContainerID: tc.containerID, ExitCode: tc.exitCode}, nil).Once()
}

// expectHealth expects one inspect of the mock container reporting the given
// health status. An empty status reports a container without a health check.
//
// Parameters:
//   - api: Mocked Docker API.
//   - status: Health status the inspect reports.
func expectHealth(api *mockContainer.MockAPIClient, status dockerContainer.HealthStatus) {
	state := &dockerContainer.State{Status: "running"}
	if status != "" {
		state.Health = &dockerContainer.Health{Status: status}
	}

	api.EXPECT().ContainerInspect(mock.Anything, mockContainerID, dockerClient.ContainerInspectOptions{}).
		Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
			ID:     mockContainerID,
			State:  state,
			Config: &dockerContainer.Config{},
		}}, nil).Once()
}

// expectInfo expects one daemon info request and answers it with info.
//
// Parameters:
//   - api: Mocked Docker API.
//   - info: Daemon information to answer with.
func expectInfo(api *mockContainer.MockAPIClient, info dockerSystem.Info) {
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(dockerClient.SystemInfoResult{Info: info}, nil).Once()
}

// expectServerVersion expects one server version request and answers it with
// the given components.
//
// Parameters:
//   - api: Mocked Docker API.
//   - components: Version components the daemon reports.
func expectServerVersion(api *mockContainer.MockAPIClient, components ...dockerSystem.ComponentVersion) {
	api.EXPECT().ServerVersion(mock.Anything, dockerClient.ServerVersionOptions{}).
		Return(dockerClient.ServerVersionResult{Components: components}, nil).Once()
}

// newExecOutput returns an exec attach result whose connection streams the
// output written by write, and closes when the client closes the result.
//
// Parameters:
//   - t: Test the connection belongs to.
//   - write: Writes the command output, nil for a command that never writes.
//
// Returns:
//   - dockerClient.ExecAttachResult: Result with the client end of the connection.
func newExecOutput(t *testing.T, write func(conn net.Conn)) dockerClient.ExecAttachResult {
	t.Helper()

	client, server := net.Pipe()

	t.Cleanup(func() { _ = server.Close() })

	if write != nil {
		go func() {
			defer server.Close()

			write(server)
		}()
	}

	return dockerClient.ExecAttachResult{HijackedResponse: dockerClient.NewHijackedResponse(client, "")}
}

// newTLSStubClient returns a Docker client for a TLS stub server that answers
// the API version ping.
//
// Parameters:
//   - t: Test the server belongs to.
//
// Returns:
//   - *httpstub.Server: Server the client talks to.
//   - *dockerClient.Client: Client using the server's TLS configuration.
func newTLSStubClient(t *testing.T) (*httpstub.Server, *dockerClient.Client) {
	t.Helper()

	server := httpstub.NewTLS(t)
	server.RouteToHandler(http.MethodHead, httpstub.Path("/_ping"), httpstub.RespondWith(http.StatusOK, "", nil))

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)

	return server, api
}

// writeDockerCerts writes the ca.pem, cert.pem, and key.pem files the Docker
// SDK reads from DOCKER_CERT_PATH. The CA is the stub server's certificate, so
// the client trusts the server, and the client certificate is self-signed.
//
// Parameters:
//   - t: Test the files belong to.
//   - server: TLS stub server the client connects to.
//
// Returns:
//   - string: Directory to set DOCKER_CERT_PATH to.
func writeDockerCerts(t *testing.T, server *httpstub.Server) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "watchtower-test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	files := map[string]*pem.Block{
		"ca.pem":   {Type: "CERTIFICATE", Bytes: server.TestServer().Certificate().Raw},
		"cert.pem": {Type: "CERTIFICATE", Bytes: certDER},
		"key.pem":  {Type: "EC PRIVATE KEY", Bytes: keyDER},
	}

	for name, block := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(block), 0o600))
	}

	return dir
}

// respondToPing answers a ping with status, reporting apiVersion as the
// daemon's API version.
//
// Parameters:
//   - t: Test the handler belongs to.
//   - method: Ping method, HEAD or GET.
//   - status: Status to answer with.
//   - apiVersion: API version the daemon reports, empty for none.
//
// Returns:
//   - http.HandlerFunc: Handler verifying and answering the ping.
func respondToPing(t *testing.T, method string, status int, apiVersion string) http.HandlerFunc {
	t.Helper()

	header := http.Header{}
	if apiVersion != "" {
		header.Set("Api-Version", apiVersion)
	}

	return httpstub.CombineHandlers(
		httpstub.VerifyRequest(t, method, httpstub.Path("/_ping")),
		httpstub.RespondWith(status, "", header),
	)
}

// useDockerTLS points the Docker environment variables at a TLS connection to
// server, with certificates the client trusts.
//
// Parameters:
//   - t: Test whose environment is changed.
//   - server: Server the client connects to.
//   - certServer: TLS server whose certificate the client trusts.
func useDockerTLS(t *testing.T, server, certServer *httpstub.Server) {
	t.Helper()

	t.Setenv("DOCKER_HOST", "tcp://"+server.Addr())
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", writeDockerCerts(t, certServer))
	t.Setenv("DOCKER_API_VERSION", "")
}

// TestClient verifies the Docker client's container lifecycle, listing,
// health, runtime detection, exec, and system calls against a mocked Docker
// API.
func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("removing a running container", func(t *testing.T) {
		t.Parallel()

		// running returns a running container.
		running := func() *Container {
			return MockContainer(WithContainerState(dockerContainer.State{Running: true}))
		}

		t.Run("the container still exists after stopping", func(t *testing.T) {
			t.Parallel()

			t.Run("should attempt to remove the container", func(t *testing.T) {
				t.Parallel()

				log, logOutput := newCapturedLogger(zerolog.DebugLevel)
				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, nil)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, nil).Once()

				err := (&client{log: log, api: api}).StopAndRemoveContainer(t.Context(), running(), time.Second)
				require.NoError(t, err)
				assert.Contains(t, logOutput.String(), "Container removed successfully")
			})
		})

		t.Run("the container does not exist after stopping", func(t *testing.T) {
			t.Parallel()

			t.Run("should not cause an error", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, nil)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, notFoundError("No such container: "+mockContainerID)).Once()

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), running(), time.Second)
				require.NoError(t, err)
			})
		})

		t.Run("stopping fails with an unexpected error", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, daemonError("server error"))

				err := (&client{log: newTestLogger(t), api: api}).StopContainer(t.Context(), running(), time.Second)
				require.ErrorContains(t, err, "failed to stop container: Error response from daemon: server error")
			})
		})

		t.Run("stopping fails with an unexpected error in StopAndRemoveContainer", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error without attempting removal", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, daemonError("stop error"))

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), running(), time.Second)
				require.ErrorContains(t, err, "failed to stop container: Error response from daemon: stop error")
			})
		})

		t.Run("removal fails with an unexpected error", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, nil)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, daemonError("server error")).Once()

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), running(), time.Second)
				require.ErrorContains(t, err, "failed to remove container: Error response from daemon: server error")
			})
		})

		t.Run("removing a stopped container", func(t *testing.T) {
			t.Parallel()

			t.Run("should only call remove, not stop", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, nil).Once()

				stopped := MockContainer(WithContainerState(dockerContainer.State{Running: false}))

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), stopped, time.Second)
				require.NoError(t, err)
			})
		})

		t.Run("stopping a container with AutoRemove enabled", func(t *testing.T) {
			t.Parallel()

			t.Run("should skip removal after stopping a running container", func(t *testing.T) {
				t.Parallel()

				// Docker removes an AutoRemove container after it stops, so
				// only the stop is expected.
				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, nil)

				source := MockContainer(
					WithContainerState(dockerContainer.State{Running: true}),
					WithAutoRemove(true),
				)

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), source, time.Second)
				require.NoError(t, err)
			})

			t.Run("should remove a non-running AutoRemove container explicitly", func(t *testing.T) {
				t.Parallel()

				// AutoRemove does not apply to a container that never started.
				api := newDockerAPI(t)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, nil).Once()

				source := MockContainer(
					WithContainerState(dockerContainer.State{Running: false, Status: "created"}),
					WithAutoRemove(true),
				)

				err := (&client{log: newTestLogger(t), api: api}).StopAndRemoveContainer(t.Context(), source, time.Second)
				require.NoError(t, err)
			})
		})
	})

	t.Run("stopping a container", func(t *testing.T) {
		t.Parallel()

		t.Run("the container is running", func(t *testing.T) {
			t.Parallel()

			t.Run("should stop the container successfully", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectStop(api, mockContainerID, 1, nil)

				source := MockContainer(WithContainerState(dockerContainer.State{Running: true}))

				err := (&client{log: newTestLogger(t), api: api}).StopContainer(t.Context(), source, time.Second)
				require.NoError(t, err)
			})
		})

		t.Run("the container is already stopped", func(t *testing.T) {
			t.Parallel()

			t.Run("should not attempt to stop and return no error", func(t *testing.T) {
				t.Parallel()

				// The mock has no expectations, so any Docker call fails the test.
				api := mockContainer.NewMockAPIClient(t)
				source := MockContainer(WithContainerState(dockerContainer.State{Running: false}))

				err := (&client{log: newTestLogger(t), api: api}).StopContainer(t.Context(), source, time.Second)
				require.NoError(t, err)
			})
		})
	})

	t.Run("removing a container", func(t *testing.T) {
		t.Parallel()

		t.Run("the container exists", func(t *testing.T) {
			t.Parallel()

			t.Run("should remove the container successfully", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, nil).Once()

				err := (&client{log: newTestLogger(t), api: api}).RemoveContainer(t.Context(), MockContainer())
				require.NoError(t, err)
			})
		})

		t.Run("the container does not exist", func(t *testing.T) {
			t.Parallel()

			t.Run("should not return an error", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, notFoundError("No such container: "+mockContainerID)).Once()

				err := (&client{log: newTestLogger(t), api: api}).RemoveContainer(t.Context(), MockContainer())
				require.NoError(t, err)
			})
		})

		t.Run("removal fails with an unexpected error", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				api.EXPECT().ContainerRemove(mock.Anything, mockContainerID, dockerClient.ContainerRemoveOptions{Force: true}).
					Return(dockerClient.ContainerRemoveResult{}, daemonError("server error")).Once()

				err := (&client{log: newTestLogger(t), api: api}).RemoveContainer(t.Context(), MockContainer())
				require.ErrorContains(t, err, "failed to remove container")
			})
		})
	})

	t.Run("listing containers", func(t *testing.T) {
		t.Parallel()

		// newRunningAPI returns a mocked Docker API listing the running
		// Watchtower and Portainer containers.
		newRunningAPI := func(t *testing.T) *mockContainer.MockAPIClient {
			t.Helper()

			api := newDockerAPI(t)
			expectContainerList(t, api, "running")
			expectContainerFixtures(t, api, fixtureWatchtower, fixtureRunning)

			return api
		}

		t.Run("no filter is provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should return all available containers", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}

				containers, err := c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)
				assert.Len(t, containers, 2)
			})
		})

		t.Run("a filter matching nothing", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an empty array", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}
				filter := filters.FilterByNames(newTestLogger(t), []string{"lollercoaster"}, filters.NoFilter)

				containers, err := c.ListContainers(t.Context(), filter)
				require.NoError(t, err)
				assert.Empty(t, containers)
			})
		})

		t.Run("a watchtower filter is provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should return only the watchtower container", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}

				containers, err := c.ListContainers(t.Context(), filters.WatchtowerContainersFilter)
				require.NoError(t, err)
				require.Len(t, containers, 1)
				assert.Equal(t, "nickfedor/watchtower:latest", containers[0].ImageName())
			})
		})

		t.Run(`include stopped is enabled`, func(t *testing.T) {
			t.Parallel()

			t.Run("should return both stopped and running containers", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectContainerList(t, api, "running", "exited", "created")
				expectContainerFixtures(t, api, fixtureStopped, fixtureWatchtower, fixtureRunning)

				c := &client{log: newTestLogger(t), api: api, IncludeStopped: true}

				containers, err := c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)
				assert.True(t, slices.ContainsFunc(containers, func(container types.Container) bool {
					return !container.ContainerInfo().State.Running
				}), "a stopped container is listed")
			})
		})

		t.Run(`include restarting is enabled`, func(t *testing.T) {
			t.Parallel()

			t.Run("should return both restarting and running containers", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectContainerList(t, api, "running", "restarting")
				expectContainerFixtures(t, api, fixtureWatchtower, fixtureRunning, fixtureRestarting)

				// A Docker marker resolves the runtime without an API call.
				fsys := afero.NewMemMapFs()
				require.NoError(t, afero.WriteFile(fsys, "/.dockerenv", nil, 0o600))

				c := &client{
					log:               newTestLogger(t),
					api:               api,
					IncludeRestarting: true, Fs: fsys,
				}

				containers, err := c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)
				assert.True(t, slices.ContainsFunc(containers, func(container types.Container) bool {
					return container.ContainerInfo().State.Restarting
				}), "a restarting container is listed")
			})
		})

		t.Run(`include restarting is disabled`, func(t *testing.T) {
			t.Parallel()

			t.Run("should not return restarting containers", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}

				containers, err := c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)
				assert.False(t, slices.ContainsFunc(containers, func(container types.Container) bool {
					return container.ContainerInfo().State.Restarting
				}), "no restarting container is listed")
			})
		})

		t.Run("multiple filters are provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should combine filters with logical AND", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}
				nameFilter := filters.FilterByNames(newTestLogger(t), []string{"portainer"}, filters.NoFilter)

				containers, err := c.ListContainers(t.Context(), nameFilter, filters.NoFilter)
				require.NoError(t, err)
				require.Len(t, containers, 1)
				assert.Equal(t, "portainer", containers[0].Name())
			})

			t.Run("should return empty when filters are mutually exclusive", func(t *testing.T) {
				t.Parallel()

				c := &client{log: newTestLogger(t), api: newRunningAPI(t)}
				portainerFilter := filters.FilterByNames(newTestLogger(t), []string{"portainer"}, filters.NoFilter)
				watchtowerFilter := filters.FilterByNames(newTestLogger(t), []string{"watchtower-running"}, filters.NoFilter)

				containers, err := c.ListContainers(t.Context(), portainerFilter, watchtowerFilter)
				require.NoError(t, err)
				assert.Empty(t, containers)
			})
		})

		t.Run(`the image of a container cannot be inspected`, func(t *testing.T) {
			t.Parallel()

			t.Run("should warn, naming the container and its image", func(t *testing.T) {
				t.Parallel()

				// GetContainer is only reached for containers already being
				// acted upon, so missing image metadata is worth a warning here
				// even though getSourceContainer logs it at debug level.
				api := newDockerAPI(t)
				expectMissingImage(api, testContainerID)

				log, logOutput := newCapturedLogger(zerolog.WarnLevel)
				c := &client{log: log, api: api}

				container, err := c.GetContainer(t.Context(), types.ContainerID(testContainerID))
				require.NoError(t, err)
				assert.False(t, container.HasImageInfo())

				logged := logOutput.String()
				assert.Contains(t, logged, "Failed to retrieve image info")
				assert.Contains(t, logged, "test-container")
				assert.Contains(t, logged, "test-image:latest")
			})
		})

		t.Run(`a container uses container network mode`, func(t *testing.T) {
			t.Parallel()

			t.Run(`the network container can be resolved`, func(t *testing.T) {
				t.Parallel()

				t.Run("should return the container name instead of the ID", func(t *testing.T) {
					t.Parallel()

					api := newDockerAPI(t)
					expectContainerFixtures(t, api, fixtureNetConsumer)

					container, err := (&client{log: newTestLogger(t), api: api}).GetContainer(t.Context(), fixtureNetConsumer.id)
					require.NoError(t, err)

					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					assert.Equal(t, fixtureNetSupplierContainerName, networkMode.ConnectedContainer())
				})
			})

			t.Run(`the network container cannot be resolved`, func(t *testing.T) {
				t.Parallel()

				t.Run("should still return the container ID", func(t *testing.T) {
					t.Parallel()

					api := newDockerAPI(t)
					expectContainerFixtures(t, api, fixtureNetConsumerMissingSupplier)

					container, err := (&client{log: newTestLogger(t), api: api}).
						GetContainer(t.Context(), fixtureNetConsumerMissingSupplier.id)
					require.NoError(t, err)

					networkMode := container.ContainerInfo().HostConfig.NetworkMode
					assert.Equal(t, fixtureNetSupplierNotFoundID, networkMode.ConnectedContainer())
				})
			})
		})
	})

	t.Run("WaitForContainerHealthy", func(t *testing.T) {
		t.Parallel()

		t.Run("container has no health check", func(t *testing.T) {
			t.Parallel()

			t.Run("should return immediately without error", func(t *testing.T) {
				t.Parallel()

				synctest.Test(t, func(t *testing.T) {
					api := newDockerAPI(t)
					expectHealth(api, "")

					err := (&client{log: newTestLogger(t), api: api}).
						WaitForContainerHealthy(t.Context(), mockContainerID, 5*time.Second)
					require.NoError(t, err)
				})
			})
		})

		t.Run("container becomes healthy", func(t *testing.T) {
			t.Parallel()

			t.Run("should return without error", func(t *testing.T) {
				t.Parallel()

				synctest.Test(t, func(t *testing.T) {
					api := newDockerAPI(t)
					expectHealth(api, dockerContainer.Starting)
					expectHealth(api, dockerContainer.Starting)
					expectHealth(api, dockerContainer.Healthy)

					err := (&client{log: newTestLogger(t), api: api}).
						WaitForContainerHealthy(t.Context(), mockContainerID, 5*time.Second)
					require.NoError(t, err)
				})
			})
		})

		t.Run("container becomes unhealthy", func(t *testing.T) {
			t.Parallel()

			t.Run("should return an error", func(t *testing.T) {
				t.Parallel()

				synctest.Test(t, func(t *testing.T) {
					api := newDockerAPI(t)
					expectHealth(api, dockerContainer.Unhealthy)

					err := (&client{log: newTestLogger(t), api: api}).
						WaitForContainerHealthy(t.Context(), mockContainerID, 5*time.Second)
					require.ErrorContains(t, err, "health check failed")
				})
			})
		})
	})

	t.Run("getRuntime", func(t *testing.T) {
		t.Parallel()

		t.Run("CPUCopyMode is auto", func(t *testing.T) {
			t.Parallel()

			t.Run("should detect Podman via marker file", func(t *testing.T) {
				t.Parallel()

				memFs := afero.NewMemMapFs()
				require.NoError(t, afero.WriteFile(memFs, "/run/.containerenv", []byte{}, 0o644))

				testClient := &client{
					log:         newTestLogger(t),
					api:         mockContainer.NewMockAPIClient(t),
					CPUCopyMode: cpuCopyModeAuto, Fs: memFs,
				}
				assert.True(t, testClient.getRuntime(t.Context()))
			})

			t.Run("should detect Docker via marker file", func(t *testing.T) {
				t.Parallel()

				memFs := afero.NewMemMapFs()
				require.NoError(t, afero.WriteFile(memFs, "/.dockerenv", []byte{}, 0o644))

				testClient := &client{
					log:         newTestLogger(t),
					api:         mockContainer.NewMockAPIClient(t),
					CPUCopyMode: cpuCopyModeAuto, Fs: memFs,
				}
				assert.False(t, testClient.getRuntime(t.Context()))
			})
		})
	})

	t.Run("detectRuntimeByMarker", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			title   string
			markers []string
			want    containerRuntime
		}{
			{title: "should return runtimePodman when Podman marker file exists", markers: []string{"/run/.containerenv"}, want: runtimePodman},
			{title: "should return runtimeDocker when Docker marker file exists", markers: []string{"/.dockerenv"}, want: runtimeDocker},
			{title: "should return runtimeUnknown when neither marker file exists", want: runtimeUnknown},
			{
				title:   "should return runtimePodman when Podman marker exists alongside Docker marker",
				markers: []string{"/run/.containerenv", "/.dockerenv"},
				want:    runtimePodman,
			},
		}

		for _, tt := range tests {
			t.Run(tt.title, func(t *testing.T) {
				t.Parallel()

				memFs := afero.NewMemMapFs()
				for _, marker := range tt.markers {
					require.NoError(t, afero.WriteFile(memFs, marker, []byte{}, 0o644))
				}

				// Without a logger, the client logs through a discarding one.
				testClient := &client{Fs: memFs}

				result, err := testClient.detectRuntimeByMarker()
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			})
		}
	})

	t.Run("detectRuntimeByAPI", func(t *testing.T) {
		t.Parallel()

		t.Run("should return true when API returns Name podman", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInfo(api, dockerSystem.Info{Name: "podman"})

			result, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			require.NoError(t, err)
			assert.True(t, result)
		})

		t.Run("should return true when API returns ServerVersion containing podman", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInfo(api, dockerSystem.Info{ServerVersion: "podman/4.0.0"})

			result, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			require.NoError(t, err)
			assert.True(t, result)
		})

		t.Run("should return false when API returns neither podman Name nor ServerVersion", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInfo(api, dockerSystem.Info{Name: "docker", ServerVersion: "20.10.0", OperatingSystem: "linux"})
			expectServerVersion(api, dockerSystem.ComponentVersion{Name: "Engine", Version: "20.10.0"})

			result, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			require.NoError(t, err)
			assert.False(t, result)
		})

		t.Run("should return error when API call fails", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
				Return(dockerClient.SystemInfoResult{}, daemonError("server error")).Once()

			result, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			require.Error(t, err)
			assert.False(t, result)
		})

		t.Run("should return false when API returns empty response", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInfo(api, dockerSystem.Info{})
			expectServerVersion(api)

			result, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			require.NoError(t, err)
			assert.False(t, result)
		})
	})

	t.Run("ExecuteCommand", func(t *testing.T) {
		t.Parallel()

		t.Run("logging", func(t *testing.T) {
			t.Parallel()

			t.Run("should include container id field", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectExec(api, execCase{containerID: "ex-cont-id", execID: "ex-exec-id", cmd: "exec-cmd"})

				log, logOutput := newCapturedLogger(zerolog.DebugLevel)
				c := &client{log: log, api: api}

				container, err := c.GetContainer(t.Context(), "ex-cont-id")
				require.NoError(t, err)

				_, err = c.ExecuteCommand(t.Context(), container, "exec-cmd", 1, 0, 0)
				require.NoError(t, err)
				assert.Contains(t, logOutput.String(), "container_id=ex-cont-id")
			})

			t.Run("should skip updates when command exits with code 75", func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				expectExec(api, execCase{containerID: "ex-cont-id", execID: "ex-exec-id", cmd: "exec-cmd", exitCode: 75})

				log, logOutput := newCapturedLogger(zerolog.DebugLevel)
				c := &client{log: log, api: api}

				container, err := c.GetContainer(t.Context(), "ex-cont-id")
				require.NoError(t, err)

				skipUpdate, err := c.ExecuteCommand(t.Context(), container, "exec-cmd", 1, 0, 0)
				require.NoError(t, err)
				assert.True(t, skipUpdate)
				assert.Contains(t, logOutput.String(), "container_id=ex-cont-id")
			})
		})
	})

	t.Run("ExecuteCommand results", func(t *testing.T) {
		t.Parallel()

		// run gets the container and executes tc's command in it with the
		// given user and group IDs.
		run := func(t *testing.T, tc execCase, uid, gid int) (bool, error) {
			t.Helper()

			api := newDockerAPI(t)
			expectExec(api, tc)

			c := &client{log: newTestLogger(t), api: api}

			container, err := c.GetContainer(t.Context(), types.ContainerID(tc.containerID))
			require.NoError(t, err)

			return c.ExecuteCommand(t.Context(), container, tc.cmd, 1, uid, gid)
		}

		t.Run("command exits with code 0", func(t *testing.T) {
			t.Parallel()

			t.Run("should return false and nil error", func(t *testing.T) {
				t.Parallel()

				skipUpdate, err := run(t, execCase{containerID: "ex-cont-id", execID: "success-exec-id", cmd: "success-cmd"}, 0, 0)
				require.NoError(t, err)
				assert.False(t, skipUpdate)
			})
		})

		t.Run("command exits with non-zero code", func(t *testing.T) {
			t.Parallel()

			t.Run("should return error", func(t *testing.T) {
				t.Parallel()

				_, err := run(t, execCase{containerID: "ex-cont-id", execID: "failure-exec-id", cmd: "failure-cmd", exitCode: 1}, 0, 0)
				require.ErrorContains(t, err, "command execution failed")
			})
		})

		t.Run("ExecCreate fails", func(t *testing.T) {
			t.Parallel()

			t.Run("should return error containing 'failed to create exec'", func(t *testing.T) {
				t.Parallel()

				_, err := run(t, execCase{
					containerID: "ex-cont-id",
					cmd:         "create-fail-cmd",
					createErr:   daemonError("exec create failed"),
				}, 0, 0)
				require.ErrorContains(t, err, "failed to create exec")
			})
		})

		t.Run("ExecStart fails", func(t *testing.T) {
			t.Parallel()

			t.Run("should return error containing 'failed to start exec'", func(t *testing.T) {
				t.Parallel()

				_, err := run(t, execCase{
					containerID: "ex-cont-id",
					execID:      "start-fail-exec-id",
					cmd:         "start-fail-cmd",
					startErr:    daemonError("exec start failed"),
				}, 0, 0)
				require.ErrorContains(t, err, "failed to start exec")
			})
		})

		t.Run("uid and gid parameters are provided", func(t *testing.T) {
			t.Parallel()

			t.Run("should pass UID:GID to User field in ExecOptions", func(t *testing.T) {
				t.Parallel()

				skipUpdate, err := run(t, execCase{
					containerID: "ex-cont-id",
					execID:      "uid-gid-exec-id",
					cmd:         "uid-gid-cmd",
					user:        "1000:1000",
				}, 1000, 1000)
				require.NoError(t, err)
				assert.False(t, skipUpdate)
			})
		})

		t.Run("ExecStart is called", func(t *testing.T) {
			t.Parallel()

			t.Run("should use Detach true to prevent blocking on command execution", func(t *testing.T) {
				t.Parallel()

				// expectExec matches only an ExecStart with Detach: true.
				skipUpdate, err := run(t, execCase{
					containerID: "detach-test-cont-id",
					execID:      "detach-test-exec-id",
					cmd:         "detach-test-cmd",
				}, 0, 0)
				require.NoError(t, err)
				assert.False(t, skipUpdate)
			})
		})
	})

	t.Run("captureExecOutput", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error when attach fails", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ExecAttach(mock.Anything, "exec-id", dockerClient.ExecAttachOptions{TTY: true}).
				Return(dockerClient.ExecAttachResult{}, daemonError("attach failed")).Once()

			_, err := (&client{log: newTestLogger(t), api: api}).captureExecOutput(t.Context(), "exec-id")
			require.ErrorContains(t, err, "failed to attach")
		})

		t.Run("should handle context cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			// The output never arrives, so only the cancellation ends the read.
			api := newDockerAPI(t)
			api.EXPECT().ExecAttach(mock.Anything, "exec-id", dockerClient.ExecAttachOptions{TTY: true}).
				Return(newExecOutput(t, nil), nil).Once()

			_, err := (&client{log: newTestLogger(t), api: api}).captureExecOutput(ctx, "exec-id")
			require.ErrorIs(t, err, context.Canceled)
		})

		t.Run("should truncate output that exceeds the size cap", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ExecAttach(mock.Anything, "exec-id", dockerClient.ExecAttachOptions{TTY: true}).
				Return(newExecOutput(t, func(conn net.Conn) {
					_, _ = io.WriteString(conn, strings.Repeat("x", maxExecOutputSize+4096))
				}), nil).Once()

			out, err := (&client{log: newTestLogger(t), api: api}).captureExecOutput(t.Context(), "exec-id")
			require.NoError(t, err)
			assert.Len(t, out, maxExecOutputSize)
		})
	})

	t.Run("listing containers with 404 response", func(t *testing.T) {
		t.Parallel()

		t.Run("should return empty list and log warning", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ContainerList(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerListResult{}, notFoundError("page not found")).Once()

			log, logOutput := newCapturedLogger(zerolog.WarnLevel)

			containers, err := (&client{log: log, api: api}).ListContainers(t.Context(), filters.NoFilter)
			require.NoError(t, err)
			assert.Empty(t, containers)
			assert.Contains(t, logOutput.String(), "Docker API returned 404 for container list")
		})
	})

	t.Run("listing containers with 500 server error", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ContainerList(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerListResult{}, daemonError("server error")).Once()

			containers, err := (&client{log: newTestLogger(t), api: api}).ListContainers(t.Context(), filters.NoFilter)
			require.Error(t, err)
			assert.Nil(t, containers)
		})
	})

	t.Run("listing containers with 401 unauthorized error", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ContainerList(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerListResult{}, fmt.Errorf("unauthorized: %w", cerrdefs.ErrUnauthenticated)).Once()

			containers, err := (&client{log: newTestLogger(t), api: api}).ListContainers(t.Context(), filters.NoFilter)
			require.Error(t, err)
			assert.Nil(t, containers)
		})
	})

	t.Run("listing containers with container inspect 500 error", func(t *testing.T) {
		t.Parallel()

		t.Run("should return error", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().ContainerList(mock.Anything, mock.Anything).
				Return(dockerClient.ContainerListResult{Items: []dockerContainer.Summary{
					{ID: "container1", Names: []string{"/test1"}},
				}}, nil).Once()
			api.EXPECT().ContainerInspect(mock.Anything, "container1", dockerClient.ContainerInspectOptions{}).
				Return(dockerClient.ContainerInspectResult{}, daemonError("inspect error")).Once()

			containers, err := (&client{log: newTestLogger(t), api: api}).ListContainers(t.Context(), filters.NoFilter)
			require.Error(t, err)
			assert.Nil(t, containers)
		})
	})

	t.Run("ListContainers", func(t *testing.T) {
		t.Parallel()

		t.Run("containers disappear during enumeration", func(t *testing.T) {
			t.Parallel()

			t.Run("should gracefully handle ghost containers and continue processing", func(t *testing.T) {
				t.Parallel()

				const (
					valid1ID = "valid-container-1-id"
					valid2ID = "valid-container-2-id"
					ghostID  = "ghost-container-id"
				)

				// expectValid expects the inspects of a valid container and of
				// its image.
				expectValid := func(api *mockContainer.MockAPIClient, id, name, image string) {
					api.EXPECT().ContainerInspect(mock.Anything, id, dockerClient.ContainerInspectOptions{}).
						Return(dockerClient.ContainerInspectResult{Container: dockerContainer.InspectResponse{
							ID:         id,
							Name:       name,
							Image:      image,
							State:      &dockerContainer.State{Status: "running"},
							HostConfig: &dockerContainer.HostConfig{},
							Config:     &dockerContainer.Config{Image: image},
						}}, nil).Once()
					api.EXPECT().ImageInspect(mock.Anything, image).
						Return(dockerClient.ImageInspectResult{ID: "id-" + image}, nil).Once()
				}

				api := newDockerAPI(t)
				api.EXPECT().ContainerList(mock.Anything, mock.Anything).
					Return(dockerClient.ContainerListResult{Items: []dockerContainer.Summary{
						{ID: valid1ID, Names: []string{"/valid-container-1"}},
						{ID: ghostID, Names: []string{"/ghost-container"}},
						{ID: valid2ID, Names: []string{"/valid-container-2"}},
					}}, nil).Once()
				expectValid(api, valid1ID, "/valid-container-1", "test-image-1:latest")
				api.EXPECT().ContainerInspect(mock.Anything, ghostID, dockerClient.ContainerInspectOptions{}).
					Return(dockerClient.ContainerInspectResult{}, notFoundError("No such container: "+ghostID)).Once()
				expectValid(api, valid2ID, "/valid-container-2", "test-image-2:latest")

				log, logOutput := newCapturedLogger(zerolog.DebugLevel)

				containers, err := (&client{log: log, api: api}).ListContainers(t.Context())
				require.NoError(t, err)
				require.Len(t, containers, 2)

				containerIDs := []types.ContainerID{containers[0].ID(), containers[1].ID()}
				assert.ElementsMatch(t, []types.ContainerID{valid1ID, valid2ID}, containerIDs)
				assert.Contains(t, logOutput.String(), ghostID)
			})
		})
	})

	t.Run("GetImageDiskUsage", func(t *testing.T) {
		t.Parallel()

		t.Run("maps image usage from /system/df", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().DiskUsage(mock.Anything, dockerClient.DiskUsageOptions{Images: true}).
				Return(dockerClient.DiskUsageResult{Images: dockerClient.ImagesDiskUsage{
					TotalSize:   4096,
					Reclaimable: 1024,
					TotalCount:  2,
					ActiveCount: 1,
				}}, nil).Once()

			usage, err := (&client{log: newTestLogger(t), api: api}).GetImageDiskUsage(t.Context())
			require.NoError(t, err)
			assert.Equal(t, int64(4096), usage.TotalSize)
			assert.Equal(t, int64(1024), usage.Reclaimable)
			assert.Equal(t, int64(2), usage.TotalCount)
			assert.Equal(t, int64(1), usage.ActiveCount)
		})

		t.Run("wraps daemon errors", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			api.EXPECT().DiskUsage(mock.Anything, dockerClient.DiskUsageOptions{Images: true}).
				Return(dockerClient.DiskUsageResult{}, daemonError("df failed")).Once()

			_, err := (&client{log: newTestLogger(t), api: api}).GetImageDiskUsage(t.Context())
			require.ErrorContains(t, err, "failed to get image disk usage")
		})
	})

	t.Run("TLS client methods", func(t *testing.T) {
		t.Parallel()

		t.Run("GetVersion returns correct API version with TLS client", func(t *testing.T) {
			t.Parallel()

			_, api := newTLSStubClient(t)

			version := (&client{log: newTestLogger(t), api: api}).GetVersion()
			assert.Regexp(t, `^\d+\.\d+$`, version)
		})

		t.Run("GetInfo successfully retrieves system information over TLS", func(t *testing.T) {
			t.Parallel()

			server, api := newTLSStubClient(t)
			server.AppendHandlers(httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`^/v[0-9.]+/info$`)),
				httpstub.RespondWithJSONEncoded(t, http.StatusOK, map[string]any{
					"Name":            "docker-server",
					"ServerVersion":   "24.0.0",
					"OSType":          "linux",
					"OperatingSystem": "Ubuntu 20.04",
					"Driver":          "overlay2",
				}, nil),
			))

			info, err := (&client{log: newTestLogger(t), api: api}).GetInfo(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "docker-server", info.Name)
		})

		t.Run("GetInfo returns expected system info fields over TLS", func(t *testing.T) {
			t.Parallel()

			server, api := newTLSStubClient(t)
			server.AppendHandlers(httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.PathRegexp(`^/v[0-9.]+/info$`)),
				httpstub.RespondWithJSONEncoded(t, http.StatusOK, map[string]any{
					"Name":            "test-docker",
					"ServerVersion":   "25.0.0",
					"OSType":          "linux",
					"OperatingSystem": "Alpine Linux",
					"Driver":          "btrfs",
				}, nil),
			))

			info, err := (&client{log: newTestLogger(t), api: api}).GetInfo(t.Context())
			require.NoError(t, err)
			assert.Equal(t, types.SystemInfo{
				Name:            "test-docker",
				ServerVersion:   "25.0.0",
				OSType:          "linux",
				OperatingSystem: "Alpine Linux",
				Driver:          "btrfs",
			}, info)
		})
	})

	t.Run("daemonInitTimeout constant", func(t *testing.T) {
		t.Parallel()

		t.Run("should have a reasonable timeout value of 30 seconds", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, 30*time.Second, daemonInitTimeout)
		})

		t.Run("should not be zero", func(t *testing.T) {
			t.Parallel()

			assert.Positive(t, daemonInitTimeout)
		})

		t.Run("should be within reasonable bounds (10s to 5min)", func(t *testing.T) {
			t.Parallel()

			assert.GreaterOrEqual(t, daemonInitTimeout, 10*time.Second)
			assert.LessOrEqual(t, daemonInitTimeout, 5*time.Minute)
		})
	})
}

// TestClientEnvironment verifies the client behavior that depends on
// environment variables: runtime detection by the container variables, and
// client construction from the DOCKER_* variables.
//
// The test sets environment variables, so it does not run in parallel.
func TestClientEnvironment(t *testing.T) {
	t.Run("getRuntime", func(t *testing.T) {
		t.Run("CPUCopyMode is auto", func(t *testing.T) {
			// newAutoClient returns a client in auto CPU copy mode with no
			// runtime marker files.
			newAutoClient := func(t *testing.T, api dockerClient.APIClient) *client {
				t.Helper()

				return &client{
					log:         newTestLogger(t),
					api:         api,
					CPUCopyMode: cpuCopyModeAuto, Fs: afero.NewMemMapFs(),
				}
			}

			t.Run("should detect Podman via CONTAINER environment variable", func(t *testing.T) {
				t.Setenv("CONTAINER", "podman")

				assert.True(t, newAutoClient(t, mockContainer.NewMockAPIClient(t)).getRuntime(t.Context()))
			})

			t.Run("should detect Podman via API Name field", func(t *testing.T) {
				// Podman sets the lowercase variable, so clear both forms.
				t.Setenv("container", "")
				t.Setenv("CONTAINER", "")

				api := newDockerAPI(t)
				expectInfo(api, dockerSystem.Info{Name: "podman"})

				assert.True(t, newAutoClient(t, api).getRuntime(t.Context()))
			})

			t.Run("should detect Podman via API ServerVersion field", func(t *testing.T) {
				t.Setenv("container", "")
				t.Setenv("CONTAINER", "")

				api := newDockerAPI(t)
				expectInfo(api, dockerSystem.Info{ServerVersion: "podman/4.0.0"})

				assert.True(t, newAutoClient(t, api).getRuntime(t.Context()))
			})

			t.Run("should fall back to Docker when detection fails", func(t *testing.T) {
				t.Setenv("container", "")
				t.Setenv("CONTAINER", "")

				api := newDockerAPI(t)
				api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
					Return(dockerClient.SystemInfoResult{}, daemonError("server error")).Once()

				log, logOutput := newCapturedLogger(zerolog.DebugLevel)
				testClient := newAutoClient(t, api)
				testClient.log = log

				assert.False(t, testClient.getRuntime(t.Context()))
				assert.Contains(t, logOutput.String(), "Failed to detect container runtime, falling back to Docker")
			})
		})
	})

	t.Run("detectRuntimeByEnv", func(t *testing.T) {
		tests := []struct {
			title string
			// env lists the variables to set. A nil value unsets the variable.
			env  map[string]*string
			want bool
		}{
			{title: "should return true when CONTAINER is podman", env: map[string]*string{"CONTAINER": new("podman")}, want: true},
			{title: "should return true when CONTAINER is oci", env: map[string]*string{"CONTAINER": new("oci")}, want: true},
			{
				title: "should return false when CONTAINER is docker",
				env:   map[string]*string{"CONTAINER": new("docker"), "container": new("")},
			},
			{
				title: "should return false when CONTAINER is other",
				env:   map[string]*string{"CONTAINER": new("other"), "container": new("")},
			},
			{
				title: "should return false when CONTAINER is not set",
				env:   map[string]*string{"container": new(""), "CONTAINER": nil},
			},
			{
				title: "should return false when CONTAINER is empty",
				env:   map[string]*string{"CONTAINER": new(""), "container": new("")},
			},
		}

		for _, tt := range tests {
			t.Run(tt.title, func(t *testing.T) {
				for name, value := range tt.env {
					// t.Setenv restores the variable, so it is set before it
					// is unset.
					t.Setenv(name, "")

					if value == nil {
						require.NoError(t, os.Unsetenv(name))

						continue
					}

					t.Setenv(name, *value)
				}

				assert.Equal(t, tt.want, (&client{log: newTestLogger(t)}).detectRuntimeByEnv())
			})
		}
	})

	t.Run("TLS client methods", func(t *testing.T) {
		t.Run("GetInfo handles TLS connection failures gracefully", func(t *testing.T) {
			// The client requires TLS, but the server speaks plain HTTP, so
			// no request reaches a handler.
			server := httpstub.New(t)
			useDockerTLS(t, server, httpstub.NewTLS(t))

			_, err := NewClient(newTestLogger(t), ClientOptions{}).GetInfo(t.Context())
			require.ErrorContains(t, err, "failed to get system info")
			assert.Empty(t, server.ReceivedRequests())
		})
	})

	t.Run("NewClient", func(t *testing.T) {
		t.Run("should successfully connect with TLS when DOCKER_TLS_VERIFY=1 and DOCKER_HOST points to TLS server", func(t *testing.T) {
			server := httpstub.NewTLS(t)
			server.AppendHandlers(respondToPing(t, http.MethodHead, http.StatusOK, "1.45"))
			useDockerTLS(t, server, server)

			require.NotNil(t, NewClient(newTestLogger(t), ClientOptions{}))

			requests := server.ReceivedRequests()
			require.Len(t, requests, 1)
			assert.NotNil(t, requests[0].TLS, "the ping is sent over TLS")
		})

		t.Run("should fail when TLS is required but server is HTTP-only", func(t *testing.T) {
			server := httpstub.New(t)
			useDockerTLS(t, server, httpstub.NewTLS(t))

			assert.NotPanics(t, func() { NewClient(newTestLogger(t), ClientOptions{}) })
			assert.Empty(t, server.ReceivedRequests(), "no plain HTTP request is sent")
		})

		t.Run("should negotiate API version with TLS", func(t *testing.T) {
			server := httpstub.NewTLS(t)
			server.AppendHandlers(respondToPing(t, http.MethodHead, http.StatusOK, "1.45"))
			useDockerTLS(t, server, server)

			assert.Equal(t, "1.45", NewClient(newTestLogger(t), ClientOptions{}).GetVersion())
		})

		t.Run("should use forced API version with TLS", func(t *testing.T) {
			// A forced version skips negotiation, so the daemon's newer
			// version is not adopted.
			server := httpstub.NewTLS(t)
			server.AppendHandlers(respondToPing(t, http.MethodHead, http.StatusOK, "1.47"))
			useDockerTLS(t, server, server)
			t.Setenv("DOCKER_API_VERSION", "1.44")

			assert.Equal(t, "1.44", NewClient(newTestLogger(t), ClientOptions{}).GetVersion())
		})

		t.Run("should fall back to negotiation when the forced API version is newer than the daemon's", func(t *testing.T) {
			// The daemon answers the unversioned ping, so only the API
			// version it reports shows the forced version is unsupported.
			server := httpstub.NewTLS(t)
			server.AppendHandlers(
				respondToPing(t, http.MethodHead, http.StatusOK, "1.45"),
				respondToPing(t, http.MethodHead, http.StatusOK, "1.45"),
			)
			useDockerTLS(t, server, server)
			t.Setenv("DOCKER_API_VERSION", "1.47")

			assert.Equal(t, "1.45", NewClient(newTestLogger(t), ClientOptions{}).GetVersion())
			assert.Len(t, server.ReceivedRequests(), 2)
		})

		t.Run("should handle invalid API version with TLS and fall back to negotiation", func(t *testing.T) {
			// The pings at the forced version fail with 404, HEAD and then
			// GET, so the client is recreated and negotiates.
			server := httpstub.NewTLS(t)
			server.AppendHandlers(
				respondToPing(t, http.MethodHead, http.StatusNotFound, ""),
				respondToPing(t, http.MethodGet, http.StatusNotFound, ""),
				respondToPing(t, http.MethodHead, http.StatusOK, "1.45"),
			)
			useDockerTLS(t, server, server)
			t.Setenv("DOCKER_API_VERSION", "1.99")

			assert.Equal(t, "1.45", NewClient(newTestLogger(t), ClientOptions{}).GetVersion())
			assert.Len(t, server.ReceivedRequests(), 3)
		})
	})
}

// Test_isDaemonConnectionError verifies which errors count as a lost
// connection to the Docker daemon, including wrapped ones.
func Test_isDaemonConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil error returns false", err: nil, expected: false},
		{
			name:     "Docker daemon connection error returns true",
			err:      errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"),
			expected: true,
		},
		{
			name:     "connection refused error returns true",
			err:      errors.New("dial unix /var/run/docker.sock: connect: connection refused"),
			expected: true,
		},
		{name: "EOF error returns true", err: fmt.Errorf("failed to execute request: %w", io.EOF), expected: true},
		{
			name:     "unexpected EOF error returns true",
			err:      fmt.Errorf("failed to execute request: %w", io.ErrUnexpectedEOF),
			expected: true,
		},
		{name: "permission denied error returns false", err: errors.New("permission denied"), expected: false},
		{name: "container not found error returns false", err: errors.New("Error: No such container: abc123"), expected: false},
		{name: "generic error returns false", err: errors.New("some random error"), expected: false},
		{
			name: "wrapped Docker daemon error returns true",
			err: fmt.Errorf("failed to list containers: %w",
				errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, isDaemonConnectionError(tt.err))
		})
	}
}

// TestClientSetRestartPolicy verifies the restart policy the client sends to
// Docker, including the normalization of an empty policy to "no", and that a
// failed update is logged rather than returned.
func TestClientSetRestartPolicy(t *testing.T) {
	t.Parallel()

	const cid = "watchtower-container-id"

	// expectUpdate expects one restart policy update of cid to policy and
	// answers it with err.
	expectUpdate := func(api *mockContainer.MockAPIClient, policy dockerContainer.RestartPolicy, err error) {
		api.EXPECT().ContainerUpdate(mock.Anything, cid, dockerClient.ContainerUpdateOptions{RestartPolicy: &policy}).
			Return(dockerClient.ContainerUpdateResult{}, err).Once()
	}

	t.Run("the container is nil", func(t *testing.T) {
		t.Parallel()

		t.Run("should short-circuit without calling ContainerUpdate", func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so any Docker call fails the test.
			c := &client{log: newTestLogger(t), api: mockContainer.NewMockAPIClient(t)}
			c.SetRestartPolicy(t.Context(), nil, dockerContainer.RestartPolicy{})
		})
	})

	t.Run("the policy name is empty", func(t *testing.T) {
		t.Parallel()

		t.Run("should send restart policy Name=no to the Docker API", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectUpdate(api, dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled}, nil)

			(&client{log: newTestLogger(t), api: api}).
				SetRestartPolicy(t.Context(), MockContainer(WithID(cid)), dockerContainer.RestartPolicy{})
		})

		t.Run("should clear MaximumRetryCount when normalizing an empty name", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectUpdate(api, dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled}, nil)

			(&client{log: newTestLogger(t), api: api}).
				SetRestartPolicy(t.Context(), MockContainer(WithID(cid)), dockerContainer.RestartPolicy{MaximumRetryCount: 5})
		})
	})

	t.Run("the policy is always", func(t *testing.T) {
		t.Parallel()

		t.Run("should send restart policy Name=always to the Docker API", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectUpdate(api, dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyAlways}, nil)

			(&client{log: newTestLogger(t), api: api}).SetRestartPolicy(
				t.Context(), MockContainer(WithID(cid)), dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyAlways},
			)
		})
	})

	t.Run("the container update fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should log the failure and not panic", func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectUpdate(api, dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled}, daemonError("update failed"))

			log, logOutput := newCapturedLogger(zerolog.DebugLevel)

			(&client{log: log, api: api}).SetRestartPolicy(
				t.Context(), MockContainer(WithID(cid)), dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled},
			)
			assert.Contains(t, logOutput.String(), "Failed to set container restart policy")
		})
	})
}

// TestListContainers_RestartingFilterFollowsRuntime verifies that with
// restarting containers included, the list request asks for the restarting
// status on Docker and leaves it out on Podman, which has no such status, in
// every CPU copy mode.
func TestListContainers_RestartingFilterFollowsRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marker string
		// statuses are the statuses the list request filters by.
		statuses []string
	}{
		{name: "Docker", marker: "/.dockerenv", statuses: []string{"running", "restarting"}},
		{name: "Podman", marker: "/run/.containerenv", statuses: []string{"running"}},
	}

	for _, tt := range tests {
		for _, mode := range []string{cpuCopyModeAuto, "full", "none"} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				api := newDockerAPI(t)
				api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{Filters: statusFilters(tt.statuses...)}).
					Return(dockerClient.ContainerListResult{}, nil).Once()

				fsys := afero.NewMemMapFs()
				require.NoError(t, afero.WriteFile(fsys, tt.marker, nil, 0o600))

				c := &client{
					log:               newTestLogger(t),
					api:               api,
					CPUCopyMode:       mode,
					IncludeRestarting: true,
					Fs:                fsys,
				}

				_, err := c.ListContainers(t.Context(), filters.NoFilter)
				require.NoError(t, err)
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

	// The first info request fails and the second identifies Podman. A
	// third request would fail the test, so the detection must be cached.
	api := newDockerAPI(t)
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(dockerClient.SystemInfoResult{}, daemonError("server error")).Once()
	expectInfo(api, dockerSystem.Info{Name: "podman"})

	c := &client{
		log:         newTestLogger(t),
		api:         api,
		CPUCopyMode: cpuCopyModeAuto, Fs: afero.NewMemMapFs(),
	}

	assert.False(t, c.getRuntime(t.Context()), "a failed detection falls back to Docker")
	assert.True(t, c.getRuntime(t.Context()), "the next call detects Podman")
	assert.True(t, c.getRuntime(t.Context()), "the detection is cached")
}

// TestDetectRuntimeByEnv_PodmanVariable verifies that the lowercase container
// variable, which Podman sets, identifies Podman. It sets environment
// variables, so it runs sequentially.
func TestDetectRuntimeByEnv_PodmanVariable(t *testing.T) {
	// A nonmatching uppercase value keeps an inherited CONTAINER from passing
	// the test. It is set first because Windows treats both names as one
	// variable, so the lowercase value replaces it there.
	t.Setenv("CONTAINER", "docker")
	t.Setenv("container", "podman")

	c := &client{log: newTestLogger(t)}

	assert.True(t, c.detectRuntimeByEnv())
}

// TestDetectRuntimeByAPI verifies which daemon responses identify Podman,
// including Podman versions that only name themselves in the version
// components.
func TestDetectRuntimeByAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// versionErr fails the server version request.
		versionErr error
		name       string
		info       dockerSystem.Info
		// components are the server version components. Nil expects no
		// server version request.
		components []dockerSystem.ComponentVersion
		want       bool
		wantErr    bool
	}{
		{
			name: "Podman named in the info",
			info: dockerSystem.Info{Name: "podman", ServerVersion: "4.0.0"},
			want: true,
		},
		{
			name: "Podman named in the info server version",
			info: dockerSystem.Info{Name: "host-1", ServerVersion: "podman/4.0.0"},
			want: true,
		},
		{
			name:       "Podman named only in the version components",
			info:       dockerSystem.Info{Name: "host-1", ServerVersion: "6.1.3"},
			components: []dockerSystem.ComponentVersion{{Name: "Podman Engine", Version: "6.1.3"}},
			want:       true,
		},
		{
			name:       "Docker",
			info:       dockerSystem.Info{Name: "host-1", ServerVersion: "28.0.0"},
			components: []dockerSystem.ComponentVersion{{Name: "Engine", Version: "28.0.0"}},
			want:       false,
		},
		{
			name:       "version request fails",
			info:       dockerSystem.Info{Name: "host-1", ServerVersion: "6.1.3"},
			versionErr: daemonError("version failed"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := newDockerAPI(t)
			expectInfo(api, tt.info)

			if tt.components != nil || tt.versionErr != nil {
				api.EXPECT().ServerVersion(mock.Anything, dockerClient.ServerVersionOptions{}).
					Return(dockerClient.ServerVersionResult{Components: tt.components}, tt.versionErr).Once()
			}

			got, err := (&client{log: newTestLogger(t), api: api}).detectRuntimeByAPI(t.Context())
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestStartContainer_StartsCreatedContainer verifies that StartContainer starts
// the container it created, by the ID the create returned, and returns that ID.
func TestStartContainer_StartsCreatedContainer(t *testing.T) {
	t.Parallel()

	const createdID = "created-container-id"

	source := MockContainer(
		WithNetworkMode("bridge"),
		WithContainerState(dockerContainer.State{Running: true, Status: "running"}),
	)

	api := newDockerAPI(t)
	api.EXPECT().ContainerCreate(mock.Anything, mock.Anything).
		Return(dockerClient.ContainerCreateResult{ID: createdID}, nil).Once()
	api.EXPECT().ContainerRename(mock.Anything, createdID, dockerClient.ContainerRenameOptions{NewName: source.Name()}).
		Return(dockerClient.ContainerRenameResult{}, nil).Once()
	api.EXPECT().ContainerStart(mock.Anything, createdID, dockerClient.ContainerStartOptions{}).
		Return(dockerClient.ContainerStartResult{}, nil).Once()

	newID, err := (&client{log: newTestLogger(t), api: api}).StartContainer(t.Context(), source)
	require.NoError(t, err)
	assert.Equal(t, types.ContainerID(createdID), newID)
}

// TestClientWrappers verifies that the client methods delegating to the image
// client, the target container helpers, and the Docker API pass their
// arguments through and report the failures they wrap.
func TestClientWrappers(t *testing.T) {
	t.Parallel()

	t.Run("IsContainerStale reports a newer local image", func(t *testing.T) {
		t.Parallel()

		currentImageID := "sha256:" + randomSHA256()
		newImageID := "sha256:" + randomSHA256()

		api := newImageAPI(t)
		api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(newImageID), nil).Once()

		stale, latestID, _, err := (&client{log: newTestLogger(t), api: api}).IsContainerStale(
			t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true},
		)
		require.NoError(t, err)
		assert.True(t, stale)
		assert.Equal(t, types.ImageID(newImageID), latestID)
	})

	t.Run("IsContainerStale reports a failed check", func(t *testing.T) {
		t.Parallel()

		api := newImageAPI(t)
		api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
			Return(dockerClient.ImageInspectResult{}, daemonError("inspect failed")).Once()

		_, _, _, err := (&client{log: newTestLogger(t), api: api}).IsContainerStale(
			t.Context(), runningImage("test-image:latest", "sha256:"+randomSHA256()), types.UpdateParams{NoPull: true},
		)
		require.Error(t, err)
	})

	t.Run("CheckContainerUpdate checks the local image with no-pull", func(t *testing.T) {
		t.Parallel()

		currentImageID := "sha256:" + randomSHA256()

		api := newImageAPI(t)
		api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(currentImageID), nil).Once()

		available, latestID, _, err := (&client{log: newTestLogger(t), api: api}).CheckContainerUpdate(
			t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true},
		)
		require.NoError(t, err)
		assert.False(t, available)
		assert.Equal(t, types.ImageID(currentImageID), latestID)
	})

	t.Run("CheckContainerUpdate reports a failed check", func(t *testing.T) {
		t.Parallel()

		api := newImageAPI(t)
		api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
			Return(dockerClient.ImageInspectResult{}, daemonError("inspect failed")).Once()

		_, _, _, err := (&client{log: newTestLogger(t), api: api}).CheckContainerUpdate(
			t.Context(), runningImage("test-image:latest", "sha256:"+randomSHA256()), types.UpdateParams{NoPull: true},
		)
		require.Error(t, err)
	})

	t.Run("RemoveImageByID removes an unused image", func(t *testing.T) {
		t.Parallel()

		imageID := types.ImageID("sha256:" + randomSHA256())

		api := newDockerAPI(t)
		api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).
			Return(dockerClient.ContainerListResult{}, nil).Once()
		api.EXPECT().ImageRemove(mock.Anything, string(imageID), dockerClient.ImageRemoveOptions{Force: true, PruneChildren: true}).
			Return(dockerClient.ImageRemoveResult{Items: []dockerImage.DeleteResponse{{Deleted: string(imageID)}}}, nil).Once()

		err := (&client{log: newTestLogger(t), api: api}).RemoveImageByID(t.Context(), imageID, "test-image:latest")
		require.NoError(t, err)
	})

	t.Run("RemoveImageByID reports an image still in use", func(t *testing.T) {
		t.Parallel()

		imageID := types.ImageID("sha256:" + randomSHA256())

		api := newDockerAPI(t)
		api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).
			Return(dockerClient.ContainerListResult{Items: []dockerContainer.Summary{
				{ID: "user", ImageID: string(imageID), State: dockerContainer.StateRunning},
			}}, nil).Once()

		err := (&client{log: newTestLogger(t), api: api}).RemoveImageByID(t.Context(), imageID, "test-image:latest")
		require.ErrorIs(t, err, ErrImageInUse)
	})

	t.Run("StartContainerByID starts the container", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().ContainerStart(mock.Anything, mockContainerID, dockerClient.ContainerStartOptions{}).
			Return(dockerClient.ContainerStartResult{}, nil).Once()

		require.NoError(t, (&client{log: newTestLogger(t), api: api}).StartContainerByID(t.Context(), mockContainerID))
	})

	t.Run("StartContainerByID reports a failed start", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().ContainerStart(mock.Anything, mockContainerID, dockerClient.ContainerStartOptions{}).
			Return(dockerClient.ContainerStartResult{}, daemonError("start failed")).Once()

		err := (&client{log: newTestLogger(t), api: api}).StartContainerByID(t.Context(), mockContainerID)
		require.ErrorContains(t, err, "start failed")
	})

	t.Run("RenameContainer renames the container", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().ContainerRename(mock.Anything, mockContainerID, dockerClient.ContainerRenameOptions{NewName: "new-name"}).
			Return(dockerClient.ContainerRenameResult{}, nil).Once()

		require.NoError(t, (&client{log: newTestLogger(t), api: api}).RenameContainer(t.Context(), MockContainer(), "new-name"))
	})

	t.Run("RenameContainer reports a failed rename", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().ContainerRename(mock.Anything, mockContainerID, dockerClient.ContainerRenameOptions{NewName: "new-name"}).
			Return(dockerClient.ContainerRenameResult{}, daemonError("rename failed")).Once()

		err := (&client{log: newTestLogger(t), api: api}).RenameContainer(t.Context(), MockContainer(), "new-name")
		require.ErrorContains(t, err, "rename failed")
	})

	t.Run("Ping pings the daemon without negotiating the API version", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().Ping(mock.Anything, dockerClient.PingOptions{NegotiateAPIVersion: false}).
			Return(dockerClient.PingResult{}, nil).Once()

		require.NoError(t, (&client{log: newTestLogger(t), api: api}).Ping(t.Context()))
	})

	t.Run("Ping reports an unreachable daemon", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().Ping(mock.Anything, dockerClient.PingOptions{NegotiateAPIVersion: false}).
			Return(dockerClient.PingResult{}, errors.New("connection refused")).Once()

		err := (&client{log: newTestLogger(t), api: api}).Ping(t.Context())
		require.ErrorContains(t, err, "ping failed")
	})

	t.Run("GetCurrentWatchtowerContainer returns the container without image info", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectInspect(api, testContainerID, sourceInspect("/watchtower", true, &dockerContainer.HostConfig{}), nil)

		container, err := (&client{log: newTestLogger(t), api: api}).GetCurrentWatchtowerContainer(t.Context(), testContainerID)
		require.NoError(t, err)
		assert.Equal(t, types.ContainerID(testContainerID), container.ID())
		assert.False(t, container.HasImageInfo())
	})

	t.Run("GetCurrentWatchtowerContainer reports a failed inspect", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectInspect(api, testContainerID, dockerContainer.InspectResponse{}, notFoundError("No such container"))

		_, err := (&client{log: newTestLogger(t), api: api}).GetCurrentWatchtowerContainer(t.Context(), testContainerID)
		require.ErrorContains(t, err, "failed to inspect container")
	})

	t.Run("CreateEphemeralOrchestrator creates and starts the orchestrator", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		api.EXPECT().ContainerCreate(mock.Anything, mock.MatchedBy(func(options dockerClient.ContainerCreateOptions) bool {
			return options.Config != nil && options.Config.Image == "watchtower:v2"
		})).Return(dockerClient.ContainerCreateResult{ID: "orchestrator-id"}, nil).Once()
		api.EXPECT().ContainerStart(mock.Anything, "orchestrator-id", dockerClient.ContainerStartOptions{}).
			Return(dockerClient.ContainerStartResult{}, nil).Once()

		source := MockContainer(WithID("source123"), WithName("watchtower-source"), WithImageName("watchtower:latest"))

		id, err := (&client{log: newTestLogger(t), api: api}).
			CreateEphemeralOrchestrator(t.Context(), source, "watchtower:v2", "chain1", false)
		require.NoError(t, err)
		assert.Equal(t, types.ContainerID("orchestrator-id"), id)
	})
}

// TestClientThroughDockerSDK runs the client through the real Docker SDK
// against a stub daemon, so the behavior the mocked tests assume holds for the
// SDK itself: list filters reach the daemon, a 404 is reported as not found,
// and a daemon error keeps its message.
func TestClientThroughDockerSDK(t *testing.T) {
	t.Parallel()

	// newSDKClient returns a client using a real SDK client for server, with
	// the API version fixed so no negotiation ping is sent.
	newSDKClient := func(t *testing.T, server *httpstub.Server) *client {
		t.Helper()

		api, err := dockerClient.New(
			dockerClient.WithHost(server.URL()),
			dockerClient.WithHTTPClient(server.Client()),
			dockerClient.WithAPIVersion("1.47"),
		)
		require.NoError(t, err)

		return &client{log: newTestLogger(t), api: api}
	}

	// respondWithFixture answers with the testdata file name.
	respondWithFixture := func(t *testing.T, name string) http.HandlerFunc {
		t.Helper()

		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		return httpstub.RespondWith(http.StatusOK, string(data), http.Header{"Content-Type": []string{"application/json"}})
	}

	t.Run("lists and inspects containers", func(t *testing.T) {
		t.Parallel()

		running := []dockerContainer.Summary{}

		for _, summary := range readFixture[[]dockerContainer.Summary](t, "containers.json") {
			if summary.State == dockerContainer.StateRunning {
				running = append(running, summary)
			}
		}

		server := httpstub.New(t)
		server.AppendHandlers(
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v1.47/containers/json")),
				httpstub.VerifyQuery(t, `filters={"status":{"running":true}}`),
				httpstub.RespondWithJSONEncoded(t, http.StatusOK, running, nil),
			),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v1.47/containers/"+string(fixtureWatchtower.id)+"/json")),
				respondWithFixture(t, "container_watchtower.json"),
			),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v1.47/images/"+string(fixtureDefaultImage.id)+"/json")),
				respondWithFixture(t, "image_default.json"),
			),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v1.47/containers/"+string(fixtureRunning.id)+"/json")),
				respondWithFixture(t, "container_running.json"),
			),
			httpstub.CombineHandlers(
				httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v1.47/images/"+string(fixtureRunning.image.id)+"/json")),
				respondWithFixture(t, "image_running.json"),
			),
		)

		containers, err := newSDKClient(t, server).ListContainers(t.Context(), filters.NoFilter)
		require.NoError(t, err)
		require.Len(t, containers, 2)
		assert.ElementsMatch(t, []types.ContainerID{fixtureWatchtower.id, fixtureRunning.id},
			[]types.ContainerID{containers[0].ID(), containers[1].ID()})
		assert.True(t, containers[0].HasImageInfo())
	})

	t.Run("treats a 404 on removal as an already removed container", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodDelete, httpstub.Path("/v1.47/containers/"+mockContainerID)),
			httpstub.VerifyQuery(t, "force=1"),
			httpstub.RespondWithJSONEncoded(t, http.StatusNotFound, map[string]string{"message": "No such container: " + mockContainerID}, nil),
		))

		require.NoError(t, newSDKClient(t, server).RemoveContainer(t.Context(), MockContainer()))
	})

	t.Run("reports a daemon error with its message", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodPost, httpstub.Path("/v1.47/containers/"+mockContainerID+"/stop")),
			httpstub.RespondWithJSONEncoded(t, http.StatusInternalServerError, map[string]string{"message": "server error"}, nil),
		))

		source := MockContainer(WithContainerState(dockerContainer.State{Running: true}))

		err := newSDKClient(t, server).StopContainer(t.Context(), source, time.Second)
		require.ErrorContains(t, err, "failed to stop container: Error response from daemon: server error")
	})
}
