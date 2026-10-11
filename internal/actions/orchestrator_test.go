package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Containers and settings used by the orchestrator tests.
const (
	orchestratorOldID        = "01d0000000000000000000000000000000000000000000000000000000000000"
	orchestratorNewID        = "0e00000000000000000000000000000000000000000000000000000000000000"
	orchestratorOldName      = "/watchtower"
	orchestratorRenamedName  = "/watchtower-old-01d000000000"
	orchestratorOriginalName = "watchtower"
	orchestratorNewImage     = "watchtower:v2"
	orchestratorOldImage     = "watchtower:v1"
	orchestratorHelperEnv    = "WATCHTOWER_ACTIONS_ORCHESTRATOR_HELPER"
)

// Errors returned by the Docker client in the orchestrator tests.
var (
	errOrchestratorTestDocker   = errors.New("docker daemon error")
	errOrchestratorTestNotFound = cerrdefs.ErrNotFound.WithMessage("no such container")
)

// orchestratorContainer returns a Watchtower container with the given ID,
// name, and running state.
func orchestratorContainer(id, name string, running bool) types.Container {
	return mockActions.CreateMockContainerWithConfig(
		id,
		name,
		orchestratorOldImage,
		running,
		false,
		time.Now(),
		&dockerContainer.Config{
			Image:  orchestratorOldImage,
			Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
		},
	)
}

// orchestratorCalls returns the Docker client calls in the order they were
// made, each with the container or image it targeted.
func orchestratorCalls(client *mockContainer.MockClient) []string {
	calls := make([]string, 0, len(client.Calls))

	for _, call := range client.Calls {
		var target string

		switch call.Method {
		case "GetContainer", "StartContainerByID":
			target = string(call.Arguments.Get(1).(types.ContainerID))[:3]
		case "RenameContainer":
			target = call.Arguments.Get(1).(types.Container).ID().ShortID()[:3] + " to " + call.Arguments.String(2)
		case "StopContainer", "RemoveContainer", "StartContainer", "StopAndRemoveContainer":
			target = call.Arguments.Get(1).(types.Container).ID().ShortID()[:3]
		case "RemoveImageByID":
			target = call.Arguments.String(2)
		}

		calls = append(calls, call.Method+" "+target)
	}

	return calls
}

// TestReadOrchestratorEnv verifies how the orchestrator reads its settings
// from the environment, including the values it accepts without validation.
func TestReadOrchestratorEnv(t *testing.T) {
	complete := map[string]string{
		"WT_ORCHESTRATOR_OLD_ID":          orchestratorOldID,
		"WT_ORCHESTRATOR_NEW_IMAGE":       orchestratorNewImage,
		"WT_ORCHESTRATOR_ORIGINAL_NAME":   orchestratorOriginalName,
		"WT_ORCHESTRATOR_CONTAINER_CHAIN": "chain-a,chain-b",
		"WT_ORCHESTRATOR_CLEANUP":         "true",
	}

	tests := []struct {
		name        string
		override    map[string]string
		wantMissing string
		wantOldID   string
		wantChain   string
		wantCleanup bool
	}{
		{
			name:        "all variables set",
			wantOldID:   orchestratorOldID,
			wantChain:   "chain-a,chain-b",
			wantCleanup: true,
		},
		{name: "old ID missing", override: map[string]string{"WT_ORCHESTRATOR_OLD_ID": ""}, wantMissing: "WT_ORCHESTRATOR_OLD_ID"},
		{name: "new image missing", override: map[string]string{"WT_ORCHESTRATOR_NEW_IMAGE": ""}, wantMissing: "WT_ORCHESTRATOR_NEW_IMAGE"},
		{
			name:        "original name missing",
			override:    map[string]string{"WT_ORCHESTRATOR_ORIGINAL_NAME": ""},
			wantMissing: "WT_ORCHESTRATOR_ORIGINAL_NAME",
		},
		{
			name:        "container chain is optional",
			override:    map[string]string{"WT_ORCHESTRATOR_CONTAINER_CHAIN": ""},
			wantOldID:   orchestratorOldID,
			wantCleanup: true,
		},
		{
			name:      "malformed cleanup value reads as false",
			override:  map[string]string{"WT_ORCHESTRATOR_CLEANUP": "yes please"},
			wantOldID: orchestratorOldID,
			wantChain: "chain-a,chain-b",
		},
		{
			name:        "old ID is not validated",
			override:    map[string]string{"WT_ORCHESTRATOR_OLD_ID": "not-a-container-id"},
			wantOldID:   "not-a-container-id",
			wantChain:   "chain-a,chain-b",
			wantCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range complete {
				if override, ok := tt.override[key]; ok {
					value = override
				}

				t.Setenv(key, value)
			}

			oldID, newImage, originalName, chain, cleanup, err := readOrchestratorEnv()
			if tt.wantMissing != "" {
				require.ErrorIs(t, err, errOrchestratorMissingEnv)
				assert.Contains(t, err.Error(), tt.wantMissing)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOldID, oldID)
			assert.Equal(t, orchestratorNewImage, newImage)
			assert.Equal(t, orchestratorOriginalName, originalName)
			assert.Equal(t, tt.wantChain, chain)
			assert.Equal(t, tt.wantCleanup, cleanup)
		})
	}
}

// TestOrchestrateSelfUpdate_Handoff records the Docker calls the orchestrator
// makes for each outcome of the handoff, showing what it restores, starts,
// cleans up, or leaves behind when a step fails.
func TestOrchestrateSelfUpdate_Handoff(t *testing.T) {
	tests := []struct {
		name string
		// oldName is the old container's name. Empty means the original name.
		oldName      string
		originalName string
		cleanup      bool
		setup        func(client *mockContainer.MockClient, old types.Container)
		wantErr      error
		wantCalls    []string
	}{
		{
			name: "old container not found",
			setup: func(client *mockContainer.MockClient, _ types.Container) {
				client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorOldID)).
					Return(nil, errOrchestratorTestNotFound)
			},
			wantErr:   errOrchestratorOldContainerNotFound,
			wantCalls: []string{"GetContainer 01d"},
		},
		{
			name: "old container inspect fails",
			setup: func(client *mockContainer.MockClient, _ types.Container) {
				client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorOldID)).
					Return(nil, errOrchestratorTestDocker)
			},
			wantErr:   errOrchestratorInspectFailed,
			wantCalls: []string{"GetContainer 01d"},
		},
		{
			name: "success removes the renamed old container",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"RemoveContainer 01d",
			},
		},
		{
			name:    "success removes the old image when cleanup is set",
			cleanup: true,
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
				client.EXPECT().RemoveImageByID(mock.Anything, types.ImageID(orchestratorOldImage), orchestratorOldImage).
					Return(nil)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"RemoveContainer 01d",
				"RemoveImageByID " + orchestratorOldImage,
			},
		},
		{
			name:    "old container already renamed is not renamed again",
			oldName: orchestratorRenamedName,
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"StartContainer 01d",
				"GetContainer 0e0",
				"RemoveContainer 01d",
			},
		},
		{
			name: "old container gone before stop skips rename and removal",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(errOrchestratorTestNotFound)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"StartContainer 01d",
				"GetContainer 0e0",
			},
		},
		{
			name: "stop error with the old container already stopped continues",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(errOrchestratorTestDocker)
				client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorOldID)).
					Return(orchestratorContainer(orchestratorOldID, orchestratorOldName, false), nil).Once()
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"GetContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"RemoveContainer 01d",
			},
		},
		{
			name: "stop error with the old container still running stops the handoff",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(errOrchestratorTestDocker)
				client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorOldID)).
					Return(orchestratorContainer(orchestratorOldID, orchestratorOldName, true), nil).Once()
			},
			wantErr:   errOrchestratorStopFailed,
			wantCalls: []string{"GetContainer 01d", "StopContainer 01d", "GetContainer 01d"},
		},
		{
			name: "rename failure restarts the old container",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(errOrchestratorTestDocker)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorOldID)).Return(nil)
			},
			wantErr: errOrchestratorRenameFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainerByID 01d",
			},
		},
		{
			name: "rename failure still reports the rename when the restart fails",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(errOrchestratorTestDocker)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorOldID)).
					Return(errOrchestratorTestDocker)
			},
			wantErr: errOrchestratorRenameFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainerByID 01d",
			},
		},
		{
			name: "create failure restores and restarts the old container",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return("", errOrchestratorTestDocker)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorOriginalName).Return(nil)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorOldID)).Return(nil)
			},
			wantErr: errOrchestratorCreateFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"RenameContainer 01d to watchtower",
				"StartContainerByID 01d",
			},
		},
		{
			name:         "create failure without an original name leaves the old container renamed and stopped",
			originalName: "-",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return("", errOrchestratorTestDocker)
			},
			wantErr: errOrchestratorCreateFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
			},
		},
		{
			name: "create failure stops restoring when the rename back fails",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return("", errOrchestratorTestDocker)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorOriginalName).Return(errOrchestratorTestDocker)
			},
			wantErr: errOrchestratorCreateFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"RenameContainer 01d to watchtower",
			},
		},
		{
			name: "stopped new container is started",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, false)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorNewID)).Return(nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"StartContainerByID 0e0",
				"GetContainer 0e0",
				"RemoveContainer 01d",
			},
		},
		{
			name: "new container that fails to start is removed and the old one restored",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, false)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorNewID)).Return(errOrchestratorTestDocker)
				newContainer := expectNewRunning(client, false)
				client.EXPECT().StopAndRemoveContainer(mock.Anything, newContainer, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorOriginalName).Return(nil)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorOldID)).Return(nil)
			},
			wantErr: errOrchestratorStartFailed,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"StartContainerByID 0e0",
				"GetContainer 0e0",
				"StopAndRemoveContainer 0e0",
				"RenameContainer 01d to watchtower",
				"StartContainerByID 01d",
			},
		},
		{
			name: "new container that stops after starting is removed and the old one restored",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, false)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorNewID)).Return(nil)
				expectNewRunning(client, false)
				newContainer := expectNewRunning(client, false)
				client.EXPECT().StopAndRemoveContainer(mock.Anything, newContainer, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorOriginalName).Return(nil)
				client.EXPECT().StartContainerByID(mock.Anything, types.ContainerID(orchestratorOldID)).Return(nil)
			},
			wantErr: errNewContainerNotRunning,
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"StartContainerByID 0e0",
				"GetContainer 0e0",
				"GetContainer 0e0",
				"StopAndRemoveContainer 0e0",
				"RenameContainer 01d to watchtower",
				"StartContainerByID 01d",
			},
		},
		{
			name: "failure to remove the old container still succeeds",
			setup: func(client *mockContainer.MockClient, old types.Container) {
				expectOldInspect(client, old)
				client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
				client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
				client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
				expectNewRunning(client, true)
				client.EXPECT().RemoveContainer(mock.Anything, old).Return(errOrchestratorTestDocker)
			},
			wantCalls: []string{
				"GetContainer 01d",
				"StopContainer 01d",
				"RenameContainer 01d to watchtower-old-01d000000000",
				"StartContainer 01d",
				"GetContainer 0e0",
				"RemoveContainer 01d",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldName := tt.oldName
			if oldName == "" {
				oldName = orchestratorOldName
			}

			originalName := tt.originalName
			switch originalName {
			case "":
				originalName = orchestratorOriginalName
			case "-":
				originalName = ""
			}

			old := orchestratorContainer(orchestratorOldID, oldName, true)
			client := mockContainer.NewMockClient(t)
			tt.setup(client, old)

			err := orchestrateSelfUpdate(testLogger(), t.Context(), client,
				orchestratorOldID, orchestratorNewImage, originalName, "", tt.cleanup)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantCalls, orchestratorCalls(client))
		})
	}
}

// expectOldInspect expects the orchestrator's first inspection of the old
// container.
func expectOldInspect(client *mockContainer.MockClient, old types.Container) {
	client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorOldID)).Return(old, nil).Once()
}

// expectNewRunning expects one inspection of the new container, reporting it
// running or stopped, and returns the container it reports.
func expectNewRunning(client *mockContainer.MockClient, running bool) types.Container {
	newContainer := orchestratorContainer(orchestratorNewID, "/watchtower", running)
	client.EXPECT().GetContainer(mock.Anything, types.ContainerID(orchestratorNewID)).Return(newContainer, nil).Once()

	return newContainer
}

// TestRunOrchestrator_Exit verifies how the orchestrator process ends: a
// missing setting ends it with a fatal log, a failed handoff exits with 1, and
// a completed handoff exits with 0.
func TestRunOrchestrator_Exit(t *testing.T) {
	if helperCase := os.Getenv(orchestratorHelperEnv); helperCase != "" {
		runOrchestratorHelper(t, helperCase)

		// RunOrchestrator must exit. Reaching here means it did not.
		os.Exit(2)
	}

	tests := []struct {
		helperCase   string
		wantExitCode int
		wantOutput   string
	}{
		{helperCase: "missing-env", wantExitCode: 1, wantOutput: "Failed to read orchestrator environment variables"},
		{helperCase: "handoff-fails", wantExitCode: 1, wantOutput: "Orchestration failed"},
		{helperCase: "handoff-succeeds", wantExitCode: 0, wantOutput: "Started new container"},
	}

	for _, tt := range tests {
		t.Run(tt.helperCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunOrchestrator_Exit$", "-test.v=false")

			cmd.Env = append(os.Environ(), orchestratorHelperEnv+"="+tt.helperCase)

			out, err := cmd.CombinedOutput()

			exitCode := 0

			var exitErr *exec.ExitError

			switch {
			case errors.As(err, &exitErr):
				exitCode = exitErr.ExitCode()
			default:
				require.NoError(t, err, "output:\n%s", out)
			}

			assert.Equal(t, tt.wantExitCode, exitCode, "output:\n%s", out)
			assert.Contains(t, string(out), tt.wantOutput)
		})
	}
}

// runOrchestratorHelper runs RunOrchestrator in the re-executed test binary
// for the named case, logging to stderr so the parent can read the outcome.
func runOrchestratorHelper(t *testing.T, helperCase string) {
	t.Helper()

	log := logging.New(os.Stderr, logging.DebugLevel)
	client := mockContainer.NewMockClient(t)

	if helperCase != "missing-env" {
		t.Setenv("WT_ORCHESTRATOR_OLD_ID", orchestratorOldID)
		t.Setenv("WT_ORCHESTRATOR_NEW_IMAGE", orchestratorNewImage)
		t.Setenv("WT_ORCHESTRATOR_ORIGINAL_NAME", orchestratorOriginalName)
	}

	old := orchestratorContainer(orchestratorOldID, orchestratorOldName, true)

	switch helperCase {
	case "missing-env":
		t.Setenv("WT_ORCHESTRATOR_OLD_ID", "")
	case "handoff-fails":
		client.EXPECT().GetContainer(mock.Anything, mock.Anything).Return(nil, errOrchestratorTestDocker)
	case "handoff-succeeds":
		expectOldInspect(client, old)
		client.EXPECT().StopContainer(mock.Anything, old, orchestratorStopTimeout).Return(nil)
		client.EXPECT().RenameContainer(mock.Anything, old, orchestratorRenamedName[1:]).Return(nil)
		client.EXPECT().StartContainer(mock.Anything, old).Return(types.ContainerID(orchestratorNewID), nil)
		expectNewRunning(client, true)
		client.EXPECT().RemoveContainer(mock.Anything, old).Return(nil)
	default:
		panic(fmt.Sprintf("unknown orchestrator helper case %q", helperCase))
	}

	RunOrchestrator(log, context.Background(), client)
}
