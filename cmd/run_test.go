package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"

	appConfig "github.com/nicholas-fedor/watchtower/internal/config"
	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/notifications"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// runExitHelperEnv selects a run exit case when the test binary is re-executed
// as a subprocess. Empty means the parent process.
const runExitHelperEnv = "WATCHTOWER_CMD_RUN_EXIT_HELPER"

// Container labels used by the run fixtures.
const (
	watchtowerLabelKey   = "com.centurylinklabs.watchtower"
	scopeLabelKey        = "com.centurylinklabs.watchtower.scope"
	orchestratorLabelKey = "com.centurylinklabs.watchtower.ephemeral-orchestrator"
)

// errRunTestDocker is returned by the Docker client in the run tests.
var errRunTestDocker = errors.New("docker daemon error")

// withFilter matches the filter argument of a filtered ListContainers call.
// The mock records an unfiltered call with the context alone, and mock.Anything
// would also match that missing argument.
var withFilter = mock.MatchedBy(func([]types.Filter) bool { return true })

// runTestState is the process state run and runMain read from package
// variables, set by setRunState.
type runTestState struct {
	client    container.Client
	current   types.Container
	currentID types.ContainerID
}

// setRunState sets the package variables run and runMain read, and restores
// every one of them when the test ends. The Docker readiness sleep is skipped,
// and the notifier has no services, so it sends nothing.
func setRunState(t *testing.T, state runTestState) {
	t.Helper()

	savedCfg := appCfg
	savedClient := client
	savedRateLimiter := rateLimiter
	savedNotifier := notifier
	savedCurrent := currentWatchtowerContainer
	savedCurrentID := currentWatchtowerContainerID
	savedCurrentUnknown := currentWatchtowerContainerUnknown
	savedRunUpdates := runUpdatesWithNotifications
	savedSleep := sleepFunc
	savedSignalContext := createSignalContext

	t.Cleanup(func() {
		appCfg = savedCfg
		client = savedClient
		rateLimiter = savedRateLimiter
		notifier = savedNotifier
		currentWatchtowerContainer = savedCurrent
		currentWatchtowerContainerID = savedCurrentID
		currentWatchtowerContainerUnknown = savedCurrentUnknown
		runUpdatesWithNotifications = savedRunUpdates
		sleepFunc = savedSleep
		createSignalContext = savedSignalContext
	})

	appCfg = appConfig.Config{}
	client = state.client
	rateLimiter = ratelimit.New()
	currentWatchtowerContainer = state.current
	currentWatchtowerContainerID = state.currentID
	currentWatchtowerContainerUnknown = false
	sleepFunc = func(time.Duration) {}

	notifier = notifications.NewNotifier(new(zerolog.Nop()), notifyConfig.Notify{Level: "info"})
	t.Cleanup(notifier.Close)
}

// runFixtureContainer returns a running container with the given ID, name,
// creation time, and labels.
func runFixtureContainer(id, name, created string, labels map[string]string) types.Container {
	return container.NewContainer(nil,
		&dockerContainer.InspectResponse{
			ID:      id,
			Name:    name,
			Image:   "sha256:" + id,
			Created: created,
			State:   &dockerContainer.State{Running: true},
			Config: &dockerContainer.Config{
				Image:  "nickfedor/watchtower:latest",
				Labels: labels,
			},
			HostConfig: &dockerContainer.HostConfig{},
		},
		&dockerImage.InspectResponse{ID: "sha256:" + id},
	)
}

// currentWatchtower returns the Watchtower container the tests run as.
func currentWatchtower() types.Container {
	return runFixtureContainer("c0c0", "/watchtower", "2026-10-08T12:00:00Z",
		map[string]string{watchtowerLabelKey: "true"})
}

// runCalls returns the Docker client calls in the order they were made, each
// with the container, image, or listing it targeted.
func runCalls(client *mockContainer.MockClient) []string {
	calls := make([]string, 0, len(client.Calls))

	for _, call := range client.Calls {
		entry := call.Method

		switch call.Method {
		case "ListContainers":
			if len(call.Arguments) > 1 {
				entry += " filtered"
			} else {
				entry += " all"
			}
		case "SetRestartPolicy":
			entry += " " + containerLabel(call.Arguments.Get(1)) + " " +
				string(call.Arguments.Get(2).(dockerContainer.RestartPolicy).Name)
		case "StopAndRemoveContainer":
			entry += fmt.Sprintf(" %s %s", containerLabel(call.Arguments.Get(1)), call.Arguments.Get(2))
		case "RemoveImageByID":
			entry += " " + string(call.Arguments.Get(1).(types.ImageID))
		case "GetCurrentWatchtowerContainer":
			entry += " " + call.Arguments.Get(1).(types.ContainerID).ShortID()
		}

		calls = append(calls, entry)
	}

	return calls
}

// containerLabel names a container argument, or "none" when it is nil.
func containerLabel(arg any) string {
	c, ok := arg.(types.Container)
	if !ok || c == nil {
		return "none"
	}

	return strings.TrimPrefix(c.Name(), "/")
}

// runUntilCanceled runs the command in a synctest bubble until the scheduler
// is idle, then cancels the process context the way a signal would.
func runUntilCanceled(t *testing.T, args ...string) {
	t.Helper()

	cmd := newTestRootCommand(t, args...)

	synctest.Test(t, func(t *testing.T) {
		cancels := make(chan context.CancelFunc, 1)
		createSignalContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			cancels <- cancel

			return ctx, cancel
		}

		done := make(chan struct{})

		go func() {
			newRunProcess().run(cmd, nil)
			close(done)
		}()

		synctest.Wait()

		select {
		case <-done:
			require.FailNow(t, "run returned before the scheduler started waiting")
		default:
		}

		select {
		case cancel := <-cancels:
			cancel()
		default:
			require.FailNow(t, "run did not create the process context")
		}

		<-done
	})
}

// newRunProcess returns a process that logs nothing.
func newRunProcess() *process {
	return &process{log: new(zerolog.Nop())}
}

// TestRun_RunOnce records the Docker calls of a run-once update with no
// containers to update. The restart policy is disabled at the end whether or
// not Watchtower runs in a container.
func TestRun_RunOnce(t *testing.T) {
	tests := []struct {
		name    string
		current types.Container
		want    []string
	}{
		{
			name: "outside a container",
			want: []string{"GetVersion", "ListContainers filtered", "SetRestartPolicy none no"},
		},
		{
			name:    "in a container",
			current: currentWatchtower(),
			want:    []string{"GetVersion", "ListContainers filtered", "SetRestartPolicy watchtower no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockContainer.NewMockClient(t)
			mockClient.EXPECT().GetVersion().Return("1.47")
			mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return(nil, nil)
			mockClient.EXPECT().SetRestartPolicy(mock.Anything, mock.Anything, mock.Anything)

			state := runTestState{client: mockClient, current: tt.current}
			if tt.current != nil {
				state.currentID = tt.current.ID()
			}

			setRunState(t, state)

			newRunProcess().run(newTestRootCommand(t, "--run-once"), nil)

			assert.Equal(t, tt.want, runCalls(mockClient))
		})
	}
}

// TestRun_Continuous records the Docker calls of continuous mode from startup
// until the process context is canceled, including the cleanup of older
// Watchtower instances and of orchestrator containers.
func TestRun_Continuous(t *testing.T) {
	older := runFixtureContainer("01d0", "/watchtower-older", "2026-10-01T12:00:00Z",
		map[string]string{watchtowerLabelKey: "true"})
	// The tests run in a synctest bubble, whose clock starts at midnight UTC on
	// 2000-01-01. One orchestrator was created a minute earlier and may still be
	// handing off, and the other an hour earlier and is orphaned.
	inFlight := runFixtureContainer("0c41", "/watchtower-orchestrator-new", "1999-12-31T23:59:00Z",
		map[string]string{orchestratorLabelKey: "true", scopeLabelKey: "other"})
	orchestrator := runFixtureContainer("0c40", "/watchtower-orchestrator", "1999-12-31T23:00:00Z",
		map[string]string{orchestratorLabelKey: "true", scopeLabelKey: "other"})

	tests := []struct {
		name string
		args []string
		// knownID keeps the current container's ID while leaving the container
		// itself unknown, as when its lookup failed.
		knownID bool
		// uncached runs without a current container, as outside a container or
		// after a failed lookup.
		uncached bool
		setup    func(client *mockContainer.MockClient, current types.Container)
		want     []string
	}{
		{
			name: "no other containers",
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current}, nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current}, nil)
			},
			want: []string{"ListContainers filtered", "ListContainers all", "GetVersion"},
		},
		{
			// The removed image is reported to a slice that is then discarded.
			name: "older instance and its image are removed",
			args: []string{"--cleanup"},
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current, older}, nil)
				client.EXPECT().StopAndRemoveContainer(mock.Anything, older, 10*time.Minute).Return(nil)
				client.EXPECT().RemoveImageByID(mock.Anything, older.ImageID(), older.ImageName()).Return(nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current}, nil)
			},
			want: []string{
				"ListContainers filtered",
				"StopAndRemoveContainer watchtower-older 10m0s",
				"RemoveImageByID sha256:01d0",
				"ListContainers all",
				"GetVersion",
			},
		},
		{
			// Without the current container, the cleanup cannot tell which
			// instance to keep, so it removes none of them.
			name:     "older instance is kept outside a container",
			args:     []string{"--cleanup"},
			uncached: true,
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current, older}, nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current, older}, nil)
			},
			want: []string{"ListContainers filtered", "ListContainers all", "GetVersion"},
		},
		{
			name:     "older instance is kept when the current container lookup failed",
			args:     []string{"--cleanup"},
			uncached: true,
			knownID:  true,
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current, older}, nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current, older}, nil)
			},
			want: []string{"ListContainers filtered", "ListContainers all", "GetVersion"},
		},
		{
			name: "orchestrator still handing off is kept",
			args: []string{"--scope", "prod"},
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current}, nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current, inFlight}, nil)
			},
			want: []string{"ListContainers filtered", "ListContainers all", "GetVersion"},
		},
		{
			name: "orphaned orchestrator from another scope is removed",
			args: []string{"--scope", "prod"},
			setup: func(client *mockContainer.MockClient, current types.Container) {
				client.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current}, nil)
				client.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current, orchestrator}, nil)
				client.EXPECT().StopAndRemoveContainer(mock.Anything, orchestrator, time.Duration(0)).Return(nil)
			},
			want: []string{
				"ListContainers filtered",
				"ListContainers all",
				"StopAndRemoveContainer watchtower-orchestrator 0s",
				"GetVersion",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := currentWatchtower()

			mockClient := mockContainer.NewMockClient(t)
			tt.setup(mockClient, current)
			mockClient.EXPECT().GetVersion().Return("1.47")

			state := runTestState{client: mockClient, current: current, currentID: current.ID()}
			if tt.uncached {
				state.current = nil
				state.currentID = ""
			}

			if tt.knownID {
				state.currentID = current.ID()
			}

			setRunState(t, state)

			runUntilCanceled(t, tt.args...)

			assert.Equal(t, tt.want, runCalls(mockClient))
		})
	}
}

// TestRun_WaitsForAPIDrain verifies that continuous mode with the HTTP API in
// the background does not return until the API has finished a request it was
// serving when the process was told to stop. The request lists containers, so
// it does not hold the update lock that the scheduler also waits for.
func TestRun_WaitsForAPIDrain(t *testing.T) {
	// stopTimeout bounds the wait for run to return after its context is
	// canceled. It covers the HTTP API's shutdown grace period.
	const stopTimeout = 10 * time.Second

	current := currentWatchtower()

	entered := make(chan struct{})
	release := make(chan struct{})

	// Release the request even when the test fails early, so the API and run
	// do not stay blocked after the test ends.
	releaseRequest := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseRequest)

	mockClient := mockContainer.NewMockClient(t)
	// The startup cleanup of older instances lists the containers first.
	mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current}, nil).Once()
	mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current}, nil)
	// The API request then lists them, and stays open until released.
	mockClient.EXPECT().ListContainers(mock.Anything, withFilter).
		RunAndReturn(func(context.Context, ...types.Filter) ([]types.Container, error) {
			close(entered)
			<-release

			return []types.Container{}, nil
		}).Once()
	mockClient.EXPECT().GetVersion().Return("1.47").Maybe()

	setRunState(t, runTestState{client: mockClient, current: current, currentID: current.ID()})

	// run's process context, which the test cancels the way a signal would.
	runCtx, cancelRun := context.WithCancel(context.Background())
	createSignalContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
		return runCtx, cancelRun
	}

	var listenConfig net.ListenConfig

	ln, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	cmd := newTestRootCommand(t,
		"--http-api-endpoints=containers",
		"--http-api-periodic-polls",
		"--http-api-token", "test-token",
		"--http-api-host", "127.0.0.1",
		"--http-api-port", port,
	)

	done := make(chan struct{})

	go func() {
		defer close(done)

		newRunProcess().run(cmd, nil)
	}()

	// Stop run before setRunState restores the package state it reads, so a
	// test that fails early does not leave run behind.
	t.Cleanup(func() {
		cancelRun()
		releaseRequest()

		select {
		case <-done:
		case <-time.After(stopTimeout):
			t.Error("run did not stop during cleanup")
		}
	})

	var dialer net.Dialer

	require.Eventually(t, func() bool {
		conn, dialErr := dialer.DialContext(t.Context(), "tcp", addr)
		if dialErr != nil {
			return false
		}

		_ = conn.Close()

		return true
	}, 5*time.Second, 20*time.Millisecond, "the HTTP API did not start listening")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/v1/containers", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer test-token")

	statuses := make(chan int, 1)

	go func() {
		resp, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			statuses <- 0

			return
		}

		_ = resp.Body.Close()
		statuses <- resp.StatusCode
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the API request did not reach the Docker client")
	}

	// Stop the process the way a signal would. The scheduler returns, and run
	// then waits for the API to finish the request.
	cancelRun()

	select {
	case <-done:
		t.Fatal("run returned while the HTTP API was still serving a request")
	case <-time.After(300 * time.Millisecond):
	}

	releaseRequest()

	select {
	case status := <-statuses:
		assert.Equal(t, http.StatusOK, status)
	case <-time.After(5 * time.Second):
		t.Fatal("the API request did not finish after it was released")
	}

	select {
	case <-done:
	case <-time.After(stopTimeout):
		t.Fatal("run did not return after the HTTP API finished its request")
	}
}

// TestRun_HealthCheck verifies that the health check mode returns without
// touching Docker when Watchtower is not the container's main process.
func TestRun_HealthCheck(t *testing.T) {
	mockClient := mockContainer.NewMockClient(t)
	setRunState(t, runTestState{client: mockClient})

	newRunProcess().run(newTestRootCommand(t, "--health-check"), nil)

	assert.Empty(t, runCalls(mockClient))
}

// TestRun_Exit verifies how run ends the process when it cannot continue, and
// that the restart policy of the Watchtower container is disabled first.
func TestRun_Exit(t *testing.T) {
	if helperCase := os.Getenv(runExitHelperEnv); helperCase != "" {
		runExitHelper(t, helperCase)

		// Every helper case must end the process. Reaching here means it did not.
		os.Exit(2)
	}

	tests := []struct {
		helperCase string
		wantOutput string
	}{
		{
			// Loading the configuration rejects the combination, so runMain's own
			// check for it is never reached through run.
			helperCase: "rolling-restart-with-monitor-only",
			wantOutput: "rolling-restart and monitor-only cannot both be enabled",
		},
		{helperCase: "rolling-restart-validation-fails", wantOutput: "Rolling restart compatibility validation failed"},
		{helperCase: "api-without-token", wantOutput: "API setup failed"},
	}

	for _, tt := range tests {
		t.Run(tt.helperCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRun_Exit$", "-test.v=false")

			cmd.Env = append(os.Environ(), runExitHelperEnv+"="+tt.helperCase)

			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError

			require.ErrorAs(t, err, &exitErr, "output:\n%s", out)
			assert.Equal(t, 1, exitErr.ExitCode(), "output:\n%s", out)
			assert.Contains(t, string(out), tt.wantOutput)
			assert.Contains(t, string(out), "SetRestartPolicy watchtower no",
				"the restart policy is disabled before the process ends")
		})
	}
}

// runExitHelper runs the named exit case in the re-executed test binary. The
// Docker client prints each restart policy change so the parent can check it.
func runExitHelper(t *testing.T, helperCase string) {
	t.Helper()

	current := currentWatchtower()

	mockClient := mockContainer.NewMockClient(t)
	mockClient.EXPECT().GetVersion().Return("1.47").Maybe()
	mockClient.EXPECT().SetRestartPolicy(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, c types.Container, policy dockerContainer.RestartPolicy) {
			fmt.Fprintf(os.Stderr, "SetRestartPolicy %s %s\n", containerLabel(c), policy.Name)
		})

	var args []string

	switch helperCase {
	case "rolling-restart-with-monitor-only":
		args = []string{"--rolling-restart", "--monitor-only"}
	case "rolling-restart-validation-fails":
		args = []string{"--rolling-restart"}

		mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return(nil, errRunTestDocker)
	case "api-without-token":
		args = []string{"--http-api-update"}

		mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{current}, nil)
		mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{current}, nil)
	default:
		panic("unknown run exit helper case " + helperCase)
	}

	setRunState(t, runTestState{client: mockClient, current: current, currentID: current.ID()})

	proc := &process{log: logging.New(os.Stderr, logging.InfoLevel)}
	proc.run(newTestRootCommand(t, args...), nil)
}

// TestRun_OldInstanceStops verifies that continuous mode stops on its own
// when an update finds that Watchtower runs in an old instance's container,
// after disabling that container's restart policy, instead of continuing to
// run until it receives a signal.
func TestRun_OldInstanceStops(t *testing.T) {
	oldSelf := runFixtureContainer("01d0", "/watchtower-old-01d0", "2026-10-01T12:00:00Z",
		map[string]string{watchtowerLabelKey: "true"})

	mockClient := mockContainer.NewMockClient(t)
	mockClient.EXPECT().ListContainers(mock.Anything, withFilter).Return([]types.Container{oldSelf}, nil)
	mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{oldSelf}, nil)
	mockClient.EXPECT().GetVersion().Return("1.47").Maybe()
	mockClient.EXPECT().SetRestartPolicy(mock.Anything, oldSelf,
		dockerContainer.RestartPolicy{Name: dockerContainer.RestartPolicyDisabled})

	setRunState(t, runTestState{client: mockClient, current: oldSelf, currentID: oldSelf.ID()})

	cmd := newTestRootCommand(t, "--update-on-start")

	synctest.Test(t, func(t *testing.T) {
		cancels := make(chan context.CancelFunc, 1)
		createSignalContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			cancels <- cancel

			return ctx, cancel
		}

		done := make(chan struct{})

		go func() {
			newRunProcess().run(cmd, nil)
			close(done)
		}()

		synctest.Wait()

		select {
		case <-done:
		default:
			// Stop the process the way a signal would so the test can end.
			(<-cancels)()
			<-done

			require.FailNow(t, "run kept running after detecting an old instance")
		}
	})
}
