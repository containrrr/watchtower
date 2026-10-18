package lifecycle

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// newDebugLog returns a debug-level logger that writes logfmt lines to the
// returned buffer, so tests can assert on what was logged.
//
// Returns:
//   - *zerolog.Logger: Logger writing to the returned buffer.
//   - *bytes.Buffer: Buffer holding the logged lines.
func newDebugLog() (*zerolog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}

	return logging.New(logging.LogfmtWriter(buf), logging.DebugLevel), buf
}

// mockedContainer creates a *container.Container for testing.
func mockedContainer(options ...func(*container.Container)) *container.Container {
	c := container.NewContainer(nil,
		&dockerContainer.InspectResponse{
			ID:         "container_id",
			HostConfig: &dockerContainer.HostConfig{},
			Name:       "/test-container",
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		},
		nil, // No image info needed for these tests
	)
	// Apply default state to avoid nil pointer issues
	c.ContainerInfo().State = &dockerContainer.State{Running: false}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// withLabels sets labels on a container.
func withLabels(labels map[string]string) func(*container.Container) {
	return func(c *container.Container) {
		c.ContainerInfo().Config.Labels = labels
	}
}

// withContainerState sets the state on a container.
func withContainerState(state dockerContainer.State) func(*container.Container) {
	return func(c *container.Container) {
		c.ContainerInfo().State = &state
	}
}

var (
	errListingFailed = errors.New("listing failed")
	errExecFailed    = errors.New("exec failed")
	errNotFound      = errors.New("not found")
)

// TestExecutePreChecks tests the ExecutePreChecks function.
func TestExecutePreChecks(t *testing.T) {
	tests := []struct {
		name           string
		setupClient    func(*mockContainer.MockClient)
		expectedLogMsg string
	}{
		{
			name: "successful execution",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{
					mockedContainer(withLabels(map[string]string{
						"com.centurylinklabs.watchtower.lifecycle.pre-check": "pre-check",
					})),
					mockedContainer(),
				}, nil)
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-check", 1, 0, 0).
					Return(true, nil)
			},
			expectedLogMsg: "Listing containers for pre-checks",
		},
		{
			name: "listing error",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ListContainers", mock.Anything, mock.Anything).Return(nil, errListingFailed)
			},
			expectedLogMsg: "Listing containers for pre-checks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := newDebugLog()
			client := mockContainer.NewMockClient(t)
			tt.setupClient(client)
			ExecutePreChecks(log, context.Background(), client, types.UpdateParams{
				Filter:         func(types.FilterableContainer) bool { return true },
				LifecycleHooks: true,
				LifecycleUID:   0,
				LifecycleGID:   0,
			}, nil)

			output := logBuf.String()
			assert.NotEmpty(t, output, "expected log output")
			assert.Contains(t, output, tt.expectedLogMsg,
				"first log message mismatch",
			)
		})
	}
}

// TestExecutePostChecks tests the ExecutePostChecks function.
func TestExecutePostChecks(t *testing.T) {
	tests := []struct {
		name           string
		setupClient    func(*mockContainer.MockClient)
		expectedLogMsg string
	}{
		{
			name: "successful execution",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{
					mockedContainer(withLabels(map[string]string{
						"com.centurylinklabs.watchtower.lifecycle.post-check": "post-check",
					})),
					mockedContainer(),
				}, nil)
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-check", 1, 0, 0).
					Return(true, nil)
			},
			expectedLogMsg: "Listing containers for post-checks",
		},
		{
			name: "listing error",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ListContainers", mock.Anything, mock.Anything).Return(nil, errListingFailed)
			},
			expectedLogMsg: "Listing containers for post-checks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := newDebugLog()
			client := mockContainer.NewMockClient(t)
			tt.setupClient(client)
			ExecutePostChecks(log, context.Background(), client, types.UpdateParams{
				Filter:         func(types.FilterableContainer) bool { return true },
				LifecycleHooks: true,
				LifecycleUID:   0,
				LifecycleGID:   0,
			}, nil)

			output := logBuf.String()
			assert.NotEmpty(t, output, "expected log output")
			assert.Contains(t, output, tt.expectedLogMsg)
		})
	}
}

func TestExecutePreChecks_UsesListedContainers(t *testing.T) {
	log, logBuf := newDebugLog()
	client := mockContainer.NewMockClient(t)
	listed := []types.Container{
		mockedContainer(withLabels(map[string]string{
			"com.centurylinklabs.watchtower.lifecycle.pre-check": "pre-check",
		})),
	}

	client.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-check", 1, 0, 0).
		Return(true, nil)

	ExecutePreChecks(log, context.Background(), client, types.UpdateParams{
		Filter:         func(types.FilterableContainer) bool { return true },
		LifecycleHooks: true,
	}, listed)

	client.AssertNotCalled(t, "ListContainers", mock.Anything, mock.Anything)
	assert.Contains(t, logBuf.String(), "Found containers for pre-checks")
}

func TestExecutePostChecks_UsesListedContainers(t *testing.T) {
	log, logBuf := newDebugLog()
	client := mockContainer.NewMockClient(t)
	listed := []types.Container{
		mockedContainer(withLabels(map[string]string{
			"com.centurylinklabs.watchtower.lifecycle.post-check": "post-check",
		})),
	}

	client.On("ExecuteCommand", mock.Anything, mock.Anything, "post-check", 1, 0, 0).
		Return(true, nil)

	ExecutePostChecks(log, context.Background(), client, types.UpdateParams{
		Filter:         func(types.FilterableContainer) bool { return true },
		LifecycleHooks: true,
	}, listed)

	client.AssertNotCalled(t, "ListContainers", mock.Anything, mock.Anything)
	assert.Contains(t, logBuf.String(), "Found containers for post-checks")
}

func TestExecutePreChecks_EmptyListedDoesNotRelist(t *testing.T) {
	log, _ := newDebugLog()
	client := mockContainer.NewMockClient(t)
	listed := []types.Container{}

	ExecutePreChecks(log, context.Background(), client, types.UpdateParams{
		Filter:         func(types.FilterableContainer) bool { return true },
		LifecycleHooks: true,
	}, listed)

	client.AssertNotCalled(t, "ListContainers", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestExecutePostChecks_EmptyListedDoesNotRelist(t *testing.T) {
	log, _ := newDebugLog()
	client := mockContainer.NewMockClient(t)
	listed := []types.Container{}

	ExecutePostChecks(log, context.Background(), client, types.UpdateParams{
		Filter:         func(types.FilterableContainer) bool { return true },
		LifecycleHooks: true,
	}, listed)

	client.AssertNotCalled(t, "ListContainers", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestExecutePreCheckCommand tests the executePreCheckCommand function.
func TestExecutePreCheckCommand(t *testing.T) {
	tests := []struct {
		name           string
		container      types.Container
		setupClient    func(*mockContainer.MockClient)
		expectedLogMsg string
	}{
		{
			name: "command present",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.pre-check": "pre-check",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-check", 1, 0, 0).
					Return(true, nil)
			},
			expectedLogMsg: "Executing pre-check command",
		},
		{
			name:           "no command",
			container:      mockedContainer(),
			expectedLogMsg: "No pre-check command supplied",
		},
		{
			name: "command error",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.pre-check": "pre-check",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-check", 1, 0, 0).
					Return(false, errExecFailed)
			},
			expectedLogMsg: "Pre-check command failed",
		},
		{
			name: "container UID/GID override",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.pre-check": "pre-check",
				"com.centurylinklabs.watchtower.lifecycle.uid":       "1000",
				"com.centurylinklabs.watchtower.lifecycle.gid":       "1001",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-check", 1, 1000, 1001).
					Return(true, nil)
			},
			expectedLogMsg: "Executing pre-check command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := newDebugLog()

			client := mockContainer.NewMockClient(t)
			if tt.setupClient != nil {
				tt.setupClient(client)
			}

			executePreCheckCommand(log, context.Background(), client, tt.container, 0, 0)

			output := logBuf.String()
			assert.NotEmpty(t, output, "expected log output")
			assert.Contains(t, output, tt.expectedLogMsg)
		})
	}
}

// TestExecutePostCheckCommand tests the executePostCheckCommand function.
func TestExecutePostCheckCommand(t *testing.T) {
	tests := []struct {
		name           string
		container      types.Container
		setupClient    func(*mockContainer.MockClient)
		expectedLogMsg string
	}{
		{
			name: "command present",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.post-check": "post-check",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-check", 1, 0, 0).
					Return(true, nil)
			},
			expectedLogMsg: "Executing post-check command",
		},
		{
			name:           "no command",
			container:      mockedContainer(),
			expectedLogMsg: "No post-check command supplied",
		},
		{
			name: "command error",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.post-check": "post-check",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-check", 1, 0, 0).
					Return(false, errExecFailed)
			},
			expectedLogMsg: "Post-check command failed",
		},
		{
			name: "container UID/GID override",
			container: mockedContainer(withLabels(map[string]string{
				"com.centurylinklabs.watchtower.lifecycle.post-check": "post-check",
				"com.centurylinklabs.watchtower.lifecycle.uid":        "2000",
				"com.centurylinklabs.watchtower.lifecycle.gid":        "2001",
			})),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-check", 1, 2000, 2001).
					Return(true, nil)
			},
			expectedLogMsg: "Executing post-check command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := newDebugLog()

			client := mockContainer.NewMockClient(t)
			if tt.setupClient != nil {
				tt.setupClient(client)
			}

			executePostCheckCommand(log, context.Background(), client, tt.container, 0, 0)

			output := logBuf.String()
			assert.NotEmpty(t, output, "expected log output")
			assert.Contains(t, output, tt.expectedLogMsg)
		})
	}
}

// TestExecutePreUpdateCommand tests the ExecutePreUpdateCommand function.
func TestExecutePreUpdateCommand(t *testing.T) {
	tests := []struct {
		name           string
		container      types.Container
		setupClient    func(*mockContainer.MockClient)
		expectedResult bool
		expectedErr    bool
		expectedLogMsg string
	}{
		{
			name: "command present and running",
			container: mockedContainer(
				withContainerState(dockerContainer.State{Running: true}),
				withLabels(map[string]string{
					"com.centurylinklabs.watchtower.lifecycle.pre-update":         "pre-update",
					"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "2",
				}),
			),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-update", 2, 0, 0).
					Return(true, nil)
			},
			expectedResult: true,
			expectedLogMsg: "Pre-update command executed",
		},
		{
			name: "no command",
			container: mockedContainer(
				withContainerState(dockerContainer.State{Running: true}),
			),
			expectedResult: false,
			expectedLogMsg: "No pre-update command supplied",
		},
		{
			name: "not running",
			container: mockedContainer(
				withContainerState(dockerContainer.State{Running: false}),
				withLabels(map[string]string{
					"com.centurylinklabs.watchtower.lifecycle.pre-update":         "pre-update",
					"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "2",
				}),
			),
			expectedResult: false,
			expectedLogMsg: "Container is not running",
		},
		{
			name: "command error",
			container: mockedContainer(
				withContainerState(dockerContainer.State{Running: true}),
				withLabels(map[string]string{
					"com.centurylinklabs.watchtower.lifecycle.pre-update":         "pre-update",
					"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "2",
				}),
			),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-update", 2, 0, 0).
					Return(false, errExecFailed)
			},
			expectedResult: true,
			expectedErr:    true,
			expectedLogMsg: "Pre-update command failed",
		},
		{
			name: "container UID/GID override",
			container: mockedContainer(
				withContainerState(dockerContainer.State{Running: true}),
				withLabels(map[string]string{
					"com.centurylinklabs.watchtower.lifecycle.pre-update":         "pre-update",
					"com.centurylinklabs.watchtower.lifecycle.pre-update-timeout": "2",
					"com.centurylinklabs.watchtower.lifecycle.uid":                "3000",
					"com.centurylinklabs.watchtower.lifecycle.gid":                "3001",
				}),
			),
			setupClient: func(c *mockContainer.MockClient) {
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "pre-update", 2, 3000, 3001).
					Return(true, nil)
			},
			expectedResult: true,
			expectedLogMsg: "Pre-update command executed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPreUpdateTest(t, tt)
		})
	}
}

// runPreUpdateTest executes a single pre-update command test case and validates its results.
func runPreUpdateTest(t *testing.T, tt struct {
	name           string
	container      types.Container
	setupClient    func(*mockContainer.MockClient)
	expectedResult bool
	expectedErr    bool
	expectedLogMsg string
},
) {
	t.Helper()

	log, logBuf := newDebugLog()

	client := mockContainer.NewMockClient(t)
	if tt.setupClient != nil {
		tt.setupClient(client)
	}

	result, err := ExecutePreUpdateCommand(log, context.Background(), client, tt.container, 0, 0)

	assert.Equal(t, tt.expectedResult, result)

	if tt.expectedErr {
		require.Error(t, err, "expected an error but got none")
		assert.Contains(
			t,
			err.Error(),
			"pre-update command execution failed",
			"error message mismatch",
		)
	} else {
		require.NoError(t, err)
	}

	output := logBuf.String()
	assert.NotEmpty(t, output, "expected log output")
	assert.Contains(t, output, tt.expectedLogMsg)
}

// TestExecutePostUpdateCommand tests the ExecutePostUpdateCommand function.
func TestExecutePostUpdateCommand(t *testing.T) {
	tests := []struct {
		name           string
		containerID    types.ContainerID
		setupClient    func(*mockContainer.MockClient)
		expectedLogMsg string
	}{
		{
			name:        "command present",
			containerID: "test",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("GetContainer", mock.Anything, types.ContainerID("test")).
					Return(mockedContainer(withLabels(map[string]string{
						"com.centurylinklabs.watchtower.lifecycle.post-update": "post-update",
					})), nil)
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-update", 1, 0, 0).
					Return(true, nil)
			},
			expectedLogMsg: "Executing post-update command",
		},
		{
			name:        "no command",
			containerID: "test",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("GetContainer", mock.Anything, types.ContainerID("test")).Return(mockedContainer(), nil)
			},
			expectedLogMsg: "No post-update command supplied",
		},
		{
			name:        "container retrieval error",
			containerID: "test",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("GetContainer", mock.Anything, types.ContainerID("test")).Return(nil, errNotFound)
			},
			expectedLogMsg: "Failed to get container",
		},
		{
			name:        "command error",
			containerID: "test",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("GetContainer", mock.Anything, types.ContainerID("test")).
					Return(mockedContainer(withLabels(map[string]string{
						"com.centurylinklabs.watchtower.lifecycle.post-update": "post-update",
					})), nil)
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-update", 1, 0, 0).
					Return(false, errExecFailed)
			},
			expectedLogMsg: "Post-update command failed",
		},
		{
			name:        "container UID/GID override",
			containerID: "test",
			setupClient: func(c *mockContainer.MockClient) {
				c.On("GetContainer", mock.Anything, types.ContainerID("test")).
					Return(mockedContainer(withLabels(map[string]string{
						"com.centurylinklabs.watchtower.lifecycle.post-update": "post-update",
						"com.centurylinklabs.watchtower.lifecycle.uid":         "4000",
						"com.centurylinklabs.watchtower.lifecycle.gid":         "4001",
					})), nil)
				c.On("ExecuteCommand", mock.Anything, mock.Anything, "post-update", 1, 4000, 4001).
					Return(true, nil)
			},
			expectedLogMsg: "Executing post-update command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, logBuf := newDebugLog()
			client := mockContainer.NewMockClient(t)
			tt.setupClient(client)
			ExecutePostUpdateCommand(log, context.Background(), client, tt.containerID, 0, 0)

			output := logBuf.String()
			assert.NotEmpty(t, output, "expected log output")
			assert.Contains(t, output, tt.expectedLogMsg)
		})
	}
}
