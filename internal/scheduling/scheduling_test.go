package scheduling_test

import (
	"context"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// testLogger returns a discarded zerolog logger for tests that do not assert on logs.
func testLogger() *zerolog.Logger {
	nop := zerolog.Nop()

	return &nop
}

// testContainerOption configures optional fields on the InspectResponse for testing.
type testContainerOption func(*dockerContainer.InspectResponse)

// withName sets the container runtime name (stored in InspectResponse.Name).
func withName(name string) testContainerOption {
	return func(ir *dockerContainer.InspectResponse) {
		ir.Name = name
	}
}

// withPortBindings adds host-bound port bindings to the container, making
// HasExposedPorts() return true.
func withPortBindings() testContainerOption {
	return func(ir *dockerContainer.InspectResponse) {
		ir.HostConfig = &dockerContainer.HostConfig{
			PortBindings: dockerNetwork.PortMap{
				dockerNetwork.MustParsePort("8080/tcp"): {
					{
						HostIP:   netip.MustParseAddr("0.0.0.0"),
						HostPort: dockerNetwork.MustParsePort("8080/tcp").Port(),
					},
				},
			},
		}
	}
}

// createTestContainer creates a *container.Container with specified chain label for testing.
func createTestContainer(chain string, opts ...testContainerOption) *container.Container {
	labels := make(map[string]string)
	if chain != "" {
		labels[container.ContainerChainLabel] = chain
	}

	inspectResponse := &dockerContainer.InspectResponse{
		ID: "test-container-id",
		Config: &dockerContainer.Config{
			Hostname: "test-container",
			Image:    "test-image",
			Labels:   labels,
		},
	}

	for _, opt := range opts {
		opt(inspectResponse)
	}

	return container.NewContainer(nil, inspectResponse, nil)
}

// testDeps returns ScheduleDeps with common test defaults.
// Callers override ScheduleSpec, UpdateOnStart, SkipFirstRun, containers, and BaseParams as needed.
func testDeps(
	client container.Client,
	runUpdate func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric,
	writeStartup func(logging.StartupParams),
) scheduling.ScheduleDeps {
	return scheduling.ScheduleDeps{
		Logger:              testLogger(),
		Filter:              filters.NoFilter,
		FilterDesc:          "test filter",
		WriteStartupMessage: writeStartup,
		RunUpdate:           runUpdate,
		Client:              client,
		MetaVersion:         "v1.0.0",
		BaseParams:          types.UpdateParams{},
	}
}

// TestWaitForRunningUpdate verifies that the shutdown wait takes the update
// lock and keeps it, waiting for a running update to finish first, and that
// the wait ends when its timeout passes or its context is canceled.
func TestWaitForRunningUpdate(t *testing.T) {
	t.Parallel()

	t.Run("no update running", func(t *testing.T) {
		t.Parallel()

		lock := scheduling.NewUpdateLock()

		scheduling.WaitForRunningUpdate(testLogger(), t.Context(), lock)

		assert.False(t, lock.TryLock(), "an update could start after the shutdown wait")
	})

	t.Run("update running", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()
			require.True(t, lock.TryLock())

			done := make(chan struct{})

			go func() {
				scheduling.WaitForRunningUpdate(testLogger(), t.Context(), lock)
				close(done)
			}()

			synctest.Wait()

			select {
			case <-done:
				t.Fatal("the shutdown wait returned while an update was running")
			default:
			}

			lock.Unlock()
			<-done

			assert.False(t, lock.TryLock(), "an update could start after the shutdown wait")
		})
	})

	t.Run("update outlasts the wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()
			require.True(t, lock.TryLock())

			start := time.Now()

			scheduling.WaitForRunningUpdate(testLogger(), t.Context(), lock)

			assert.Equal(t, 60*time.Second, time.Since(start))
		})
	})

	t.Run("context canceled", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()
			require.True(t, lock.TryLock())

			ctx, cancel := context.WithCancel(t.Context())
			start := time.Now()

			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			scheduling.WaitForRunningUpdate(testLogger(), ctx, lock)

			assert.Equal(t, 50*time.Millisecond, time.Since(start))
		})
	})
}

// TestRunUpgradesOnSchedule_WaitingRunStopsAtShutdown verifies that a
// scheduled run waiting for the update lock gives up when the scheduler shuts
// down, instead of starting an update once the lock comes free.
func TestRunUpgradesOnSchedule_WaitingRunStopsAtShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

		var updated atomic.Bool

		deps := testDeps(client, func(context.Context, types.Filter, types.UpdateParams) *metrics.Metric {
			updated.Store(true)

			return &metrics.Metric{}
		}, func(logging.StartupParams) {})

		// Another update holds the lock while the scheduled run comes due.
		deps.Lock = scheduling.NewUpdateLock()
		require.True(t, deps.Lock.TryLock())

		deps.ScheduleSpec = "@every 1s"

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)

		go func() { done <- scheduling.RunUpgradesOnSchedule(ctx, deps) }()

		time.Sleep(1500 * time.Millisecond)
		cancel()

		require.NoError(t, <-done)

		deps.Lock.Unlock()
		synctest.Wait()

		assert.False(t, updated.Load(), "a scheduled run started an update after shutdown")
	})
}

func TestRunUpgradesOnSchedule_EmptySchedule(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Use timeout to avoid hanging
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunUpgradesOnSchedule_StartupMessageSuppressed(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	// Spy closure to detect if writeStartupMessage is called
	startupMessageCalled := false
	writeStartupMessage := func(logging.StartupParams) {
		startupMessageCalled = true
	}

	// Use timeout to avoid hanging
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.StartupMessageSent = true
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify writeStartupMessage was NOT called when startupMessageSent=true
	if startupMessageCalled {
		t.Error("writeStartupMessage should not be called when startupMessageSent is true")
	}
}

func TestRunUpgradesOnSchedule_UpdateOnStart(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	updateCalled := false
	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		updateCalled = true

		return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Use timeout to avoid hanging
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.UpdateOnStart = true

	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !updateCalled {
		t.Error("expected updateCalled to be true")
	}
}

// TestRunUpgradesOnSchedule_UpdateOnStartSkippedWhenCanceled verifies that the
// update at startup does not run when the process context is already canceled,
// for example by a stop signal during startup, so no canceled update is
// reported as a failure.
func TestRunUpgradesOnSchedule_UpdateOnStartSkippedWhenCanceled(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	updateCalled := false
	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		updateCalled = true

		return &metrics.Metric{}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	deps := testDeps(client, runUpdatesWithNotifications, func(logging.StartupParams) {})
	deps.UpdateOnStart = true

	err := scheduling.RunUpgradesOnSchedule(ctx, deps)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if updateCalled {
		t.Error("expected the update at startup to be skipped")
	}
}

func TestRunUpgradesOnSchedule_InvalidCronSpec(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		return &metrics.Metric{Scanned: 0, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.ScheduleSpec = "invalid cron spec"

	err := scheduling.RunUpgradesOnSchedule(ctx, deps)
	if err == nil {
		t.Error("expected error")
	}

	if err != nil && !strings.Contains(err.Error(), "failed to schedule updates") {
		t.Errorf("expected error to contain 'failed to schedule updates', got %v", err)
	}
}

func TestRunUpgradesOnSchedule_QuotedScheduleSpec(t *testing.T) {
	tests := []struct {
		name         string
		scheduleSpec string
		expectError  bool
	}{
		{
			name:         "double-quoted cron spec from Docker Compose",
			scheduleSpec: `"0 0 2 * * *"`,
			expectError:  false,
		},
		{
			name:         "single-quoted cron spec",
			scheduleSpec: `'0 0 2 * * *'`,
			expectError:  false,
		},
		{
			name:         "double-quoted descriptor",
			scheduleSpec: `"@hourly"`,
			expectError:  false,
		},
		{
			name:         "unquoted cron spec",
			scheduleSpec: `0 0 2 * * *`,
			expectError:  false,
		},
		{
			name:         "unquoted descriptor",
			scheduleSpec: `@hourly`,
			expectError:  false,
		},
		{
			name:         "double-quoted invalid spec still errors",
			scheduleSpec: `"invalid cron spec"`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			ctx := t.Context()

			runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
				return &metrics.Metric{Scanned: 0, Updated: 0, Failed: 0}
			}

			writeStartupMessage := func(logging.StartupParams) {}

			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer timeoutCancel()

			deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
			deps.ScheduleSpec = tt.scheduleSpec
			deps.SkipFirstRun = true
			err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestRunUpgradesOnSchedule_ContextCancellation(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		return &metrics.Metric{Scanned: 0, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Cancel immediately
	canceledCtx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)

	err := scheduling.RunUpgradesOnSchedule(canceledCtx, deps)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestRunUpgradesOnSchedule_StopsOnlyOnCancellation verifies that the scheduler
// leaves signal handling to its caller. An interrupt sent to the process does
// not stop it, and canceling its context does.
//
// The interrupt goes to the whole test process, so this test must not run in
// parallel with tests that register or depend on signal handlers. It does not
// call t.Parallel, and neither does any other test in this package.
func TestRunUpgradesOnSchedule_StopsOnlyOnCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sends an interrupt signal to the test process")
	}

	// Hold the interrupt so it does not end the test process.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	t.Cleanup(func() { signal.Stop(interrupts) })

	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)
	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		return &metrics.Metric{}
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- scheduling.RunUpgradesOnSchedule(ctx, testDeps(client, runUpdatesWithNotifications, func(logging.StartupParams) {}))
	}()

	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, process.Signal(os.Interrupt))

	select {
	case <-interrupts:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the interrupt was not delivered")
	}

	select {
	case err := <-done:
		require.FailNow(t, "the scheduler stopped on the interrupt", "error: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the scheduler did not stop after cancellation")
	}
}

func TestRunUpgradesOnSchedule_MonitorOnlyParameter(t *testing.T) {
	tests := []struct {
		name              string
		monitorOnly       bool
		expectMonitorOnly bool
	}{
		{
			name:              "monitorOnly false",
			monitorOnly:       false,
			expectMonitorOnly: false,
		},
		{
			name:              "monitorOnly true",
			monitorOnly:       true,
			expectMonitorOnly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			ctx := t.Context()

			var capturedParams types.UpdateParams

			runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
				capturedParams = params

				return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
			}

			writeStartupMessage := func(logging.StartupParams) {}

			// Use timeout to avoid hanging
			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer timeoutCancel()

			deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
			deps.UpdateOnStart = true
			deps.BaseParams.MonitorOnly = tt.monitorOnly

			err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if capturedParams.MonitorOnly != tt.expectMonitorOnly {
				t.Errorf(
					"expected MonitorOnly=%v, got %v",
					tt.expectMonitorOnly,
					capturedParams.MonitorOnly,
				)
			}
		})
	}
}

// TestShouldExitDueToInvalidRestart verifies the ShouldExitDueToInvalidRestart function
// handles various scenarios correctly.
func TestShouldExitDueToInvalidRestart(t *testing.T) {
	tests := []struct {
		name         string
		container    types.Container
		runOnceFlag  bool
		expectedExit bool
	}{
		{
			name:         "no container",
			container:    nil,
			runOnceFlag:  false,
			expectedExit: false,
		},
		{
			name:         "not a watchtower parent",
			container:    createTestContainer("other-id"),
			runOnceFlag:  false,
			expectedExit: false,
		},
		{
			name:         "is watchtower parent but run-once is true",
			container:    createTestContainer("test-container-id,parent-id"),
			runOnceFlag:  true,
			expectedExit: false,
		},
		{
			name:         "is watchtower parent and run-once is false",
			container:    createTestContainer("test-container-id,parent-id"),
			runOnceFlag:  false,
			expectedExit: true,
		},
		{
			name:         "old container with run-once false",
			container:    createTestContainer("", withName("watchtower-old-abc123")),
			runOnceFlag:  false,
			expectedExit: true,
		},
		{
			name:         "old container with run-once true",
			container:    createTestContainer("", withName("watchtower-old-abc123")),
			runOnceFlag:  true,
			expectedExit: false,
		},
		{
			name:         "old container with leading slash",
			container:    createTestContainer("", withName("/watchtower-old-def456")),
			runOnceFlag:  false,
			expectedExit: true,
		},
		{
			name:         "non-old container continues",
			container:    createTestContainer("", withName("watchtower")),
			runOnceFlag:  false,
			expectedExit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldExit := scheduling.ShouldExitDueToInvalidRestart(
				tt.container,
				tt.runOnceFlag,
			)

			assert.Equal(t, tt.expectedExit, shouldExit)
		})
	}
}

func TestRunUpgradesOnSchedule_CronWithSeconds(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	updateCallCount := 0
	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		updateCallCount++

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Use a 6-field cron spec that includes seconds (every 2 seconds)
	scheduleSpec := "*/2 * * * * *"

	// Use timeout to avoid hanging - should execute at least once within 5 seconds
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.ScheduleSpec = scheduleSpec
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should have executed at least once (depending on timing)
	if updateCallCount == 0 {
		t.Error("expected at least one update call")
	}
}

func TestRunUpgradesOnSchedule_SkipFirstRun_True(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	var capturedParams []types.UpdateParams

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
		capturedParams = append(capturedParams, params)

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Use a cron spec that runs every second
	scheduleSpec := "* * * * * *"

	// Use timeout to allow multiple executions
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 3500*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.ScheduleSpec = scheduleSpec
	deps.SkipFirstRun = true
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should have executed at least twice (first run skips self-update, second doesn't)
	if len(capturedParams) < 2 {
		t.Fatalf("expected at least 2 update calls, got %d", len(capturedParams))
	}

	// First run should have SkipSelfUpdate=true
	if !capturedParams[0].SkipSelfUpdate {
		t.Error("expected first run to skip Watchtower self-update")
	}

	// Second run should have SkipSelfUpdate=false
	if capturedParams[1].SkipSelfUpdate {
		t.Error("expected second run to not skip Watchtower self-update")
	}
}

func TestRunUpgradesOnSchedule_WatchtowerParent_Skipping(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	updateCallCount := 0
	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		updateCallCount++

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Create a mock Watchtower parent container
	parentContainer := createTestContainer("test-container-id,parent-id")

	// Use a cron spec that runs every second
	scheduleSpec := "* * * * * *"

	// Use timeout to allow multiple potential executions
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.ScheduleSpec = scheduleSpec
	deps.CurrentWatchtowerContainer = parentContainer
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should not have executed any updates (parent container skips scheduled updates)
	if updateCallCount > 0 {
		t.Errorf(
			"expected no update calls for Watchtower parent container, got %d",
			updateCallCount,
		)
	}
}

func TestRunUpgradesOnSchedule_ScheduledRuns_Execution(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	var executionTimes []time.Time

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		executionTimes = append(executionTimes, time.Now())

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Use a cron spec that runs every 1 second
	scheduleSpec := "*/1 * * * * *"

	startTime := time.Now()

	// Use timeout to allow multiple executions
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.ScheduleSpec = scheduleSpec
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	// Should complete without error when context times out (clean cancellation)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should have executed multiple times
	if len(executionTimes) < 2 {
		t.Fatalf("expected at least 2 executions, got %d", len(executionTimes))
	}

	// Verify intervals are approximately correct (within 200ms tolerance for 1-second cron)
	for i := 1; i < len(executionTimes); i++ {
		interval := executionTimes[i].Sub(executionTimes[i-1])
		if interval < 800*time.Millisecond || interval > 1200*time.Millisecond {
			t.Errorf("execution interval %v is not within expected range [800ms, 1200ms]", interval)
		}
	}

	// Verify executions happened after start time
	for _, execTime := range executionTimes {
		if execTime.Before(startTime) {
			t.Errorf("execution time %v is before start time %v", execTime, startTime)
		}
	}
}

// TestRunUpgradesOnSchedule_EphemeralSelfUpdateWithExposedPorts verifies that when
// ephemeralSelfUpdate=true, the port-conflict guard is bypassed even when the
// Watchtower container has exposed ports. This allows ephemeral self-updates to
// proceed because they remove the old container before creating the new one,
// avoiding port conflicts.
func TestRunUpgradesOnSchedule_EphemeralSelfUpdateWithExposedPorts(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	var capturedParams types.UpdateParams

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
		capturedParams = params

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Create a Watchtower container with exposed ports (host-bound port mappings).
	containerWithPorts := createTestContainer("", withPortBindings())

	require.True(t, containerWithPorts.HasExposedPorts(),
		"test container should have exposed ports",
	)

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.UpdateOnStart = true
	deps.CurrentWatchtowerContainer = containerWithPorts
	deps.BaseParams.EphemeralSelfUpdate = true
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	require.NoError(t, err)

	// With ephemeralSelfUpdate=true, the port-conflict guard should be bypassed,
	// so SkipSelfUpdate should remain false (self-update is allowed).
	assert.False(t, capturedParams.SkipSelfUpdate,
		"self-update should NOT be skipped when ephemeralSelfUpdate=true, "+
			"even with exposed ports",
	)
}

// TestRunUpgradesOnSchedule_KeepsBaseSkipSelfUpdate verifies that a run cannot
// enable self-updates that the base parameters disable.
func TestRunUpgradesOnSchedule_KeepsBaseSkipSelfUpdate(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	var capturedParams types.UpdateParams

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
		capturedParams = params

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer timeoutCancel()

	// The run at startup does not skip self-updates on its own.
	deps := testDeps(client, runUpdatesWithNotifications, func(logging.StartupParams) {})
	deps.UpdateOnStart = true
	deps.BaseParams.SkipSelfUpdate = true
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	require.NoError(t, err)

	assert.True(t, capturedParams.SkipSelfUpdate)
}

// TestRunUpgradesOnSchedule_PortConflictGuard_SkipsSelfUpdate verifies that when
// the Watchtower container has exposed ports and ephemeralSelfUpdate=false, the
// port-conflict guard forces SkipSelfUpdate to true to prevent the old container
// from holding the port while the new container tries to bind it.
func TestRunUpgradesOnSchedule_PortConflictGuard_SkipsSelfUpdate(t *testing.T) {
	client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

	ctx := t.Context()

	var capturedParams types.UpdateParams

	runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
		capturedParams = params

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	writeStartupMessage := func(logging.StartupParams) {}

	// Create a Watchtower container with exposed ports (host-bound port mappings).
	containerWithPorts := createTestContainer("", withPortBindings())

	require.True(t, containerWithPorts.HasExposedPorts(),
		"test container should have exposed ports",
	)

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer timeoutCancel()

	deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
	deps.UpdateOnStart = true
	deps.CurrentWatchtowerContainer = containerWithPorts
	err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
	require.NoError(t, err)

	// With ephemeralSelfUpdate=false and exposed ports, the port-conflict guard
	// should force SkipSelfUpdate to true to prevent port conflicts.
	assert.True(t, capturedParams.SkipSelfUpdate,
		"self-update should be skipped when container has exposed ports "+
			"and ephemeralSelfUpdate=false",
	)
}

// TestRunUpgradesOnSchedule_NoExposedPorts_AllowsSelfUpdate verifies that when
// the Watchtower container has no exposed ports, the port-conflict guard does
// not interfere regardless of the ephemeralSelfUpdate setting.
func TestRunUpgradesOnSchedule_NoExposedPorts_AllowsSelfUpdate(t *testing.T) {
	tests := []struct {
		name                string
		ephemeralSelfUpdate bool
	}{
		{
			name:                "ephemeralSelfUpdate=true",
			ephemeralSelfUpdate: true,
		},
		{
			name:                "ephemeralSelfUpdate=false",
			ephemeralSelfUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			ctx := t.Context()

			var capturedParams types.UpdateParams

			runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
				capturedParams = params

				return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
			}

			writeStartupMessage := func(logging.StartupParams) {}

			// Create a Watchtower container WITHOUT exposed ports.
			containerNoPorts := createTestContainer("")

			require.False(t, containerNoPorts.HasExposedPorts(),
				"test container should not have exposed ports",
			)

			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer timeoutCancel()

			deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
			deps.UpdateOnStart = true
			deps.CurrentWatchtowerContainer = containerNoPorts
			deps.BaseParams.EphemeralSelfUpdate = tt.ephemeralSelfUpdate
			err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
			require.NoError(t, err)

			// Without exposed ports, the port-conflict guard should not trigger,
			// so SkipSelfUpdate should remain false regardless of ephemeralSelfUpdate.
			assert.False(t, capturedParams.SkipSelfUpdate,
				"self-update should NOT be skipped when container has no exposed ports",
			)
		})
	}
}

// TestRunUpgradesOnSchedule_ReviveStoppedPropagation verifies that RunUpgradesOnSchedule
// passes the ReviveStopped flag through UpdateParams to the update function.
func TestRunUpgradesOnSchedule_ReviveStoppedPropagation(t *testing.T) {
	tests := []struct {
		name           string
		reviveStopped  bool
		expectSelected bool
	}{
		{
			name:           "reviveStopped=true",
			reviveStopped:  true,
			expectSelected: true,
		},
		{
			name:           "reviveStopped=false",
			reviveStopped:  false,
			expectSelected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			ctx := t.Context()

			var capturedParams types.UpdateParams

			runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
				capturedParams = params

				return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
			}

			writeStartupMessage := func(logging.StartupParams) {}

			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer timeoutCancel()

			deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
			deps.UpdateOnStart = true
			deps.BaseParams.ReviveStopped = tt.reviveStopped
			err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
			require.NoError(t, err)

			assert.Equal(t, tt.expectSelected, capturedParams.ReviveStopped,
				"ReviveStopped should be %v in UpdateParams", tt.expectSelected)
		})
	}
}

// TestRunUpgradesOnSchedule_UseComposeDependsOnPropagation verifies that
// RunUpgradesOnSchedule passes UseComposeDependsOn through UpdateParams so
// scheduled cycles honor Compose depends_on the same way as --run-once.
func TestRunUpgradesOnSchedule_UseComposeDependsOnPropagation(t *testing.T) {
	tests := []struct {
		name                string
		useComposeDependsOn bool
		expectSelected      bool
	}{
		{
			name:                "useComposeDependsOn=true",
			useComposeDependsOn: true,
			expectSelected:      true,
		},
		{
			name:                "useComposeDependsOn=false",
			useComposeDependsOn: false,
			expectSelected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

			ctx := t.Context()

			var capturedParams types.UpdateParams

			runUpdatesWithNotifications := func(_ context.Context, _ types.Filter, params types.UpdateParams) *metrics.Metric {
				capturedParams = params

				return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
			}

			writeStartupMessage := func(logging.StartupParams) {}

			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer timeoutCancel()

			deps := testDeps(client, runUpdatesWithNotifications, writeStartupMessage)
			deps.UpdateOnStart = true
			deps.BaseParams.UseComposeDependsOn = tt.useComposeDependsOn
			err := scheduling.RunUpgradesOnSchedule(timeoutCtx, deps)
			require.NoError(t, err)

			assert.Equal(t, tt.expectSelected, capturedParams.UseComposeDependsOn,
				"UseComposeDependsOn should be %v in UpdateParams", tt.expectSelected)
			assert.False(t, capturedParams.RunOnce,
				"scheduled updates must keep RunOnce=false")
		})
	}
}
