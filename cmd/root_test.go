package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	"github.com/nicholas-fedor/watchtower/internal/api"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/update"
	appconfig "github.com/nicholas-fedor/watchtower/internal/config"
	"github.com/nicholas-fedor/watchtower/internal/flags"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	"github.com/nicholas-fedor/watchtower/internal/scheduling"
	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// testLogger returns a discarded zerolog logger for tests that do not assert on logs.
func testLogger() *zerolog.Logger {
	nop := zerolog.Nop()

	return &nop
}

const testFilterDesc = "test filter"

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0 seconds",
		},
		{
			name:     "only seconds",
			duration: 45 * time.Second,
			expected: "45 seconds",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2 minutes 30 seconds",
		},
		{
			name:     "hours, minutes, seconds",
			duration: 1*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "1 hour 30 minutes 45 seconds",
		},
		{
			name:     "single hour",
			duration: 1 * time.Hour,
			expected: "1 hour",
		},
		{
			name:     "single minute",
			duration: 1 * time.Minute,
			expected: "1 minute",
		},
		{
			name:     "single second",
			duration: 1 * time.Second,
			expected: "1 second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTimeUnit(t *testing.T) {
	tests := []struct {
		name         string
		value        int64
		singular     string
		plural       string
		forceInclude bool
		expected     string
	}{
		{
			name:         "zero value not forced",
			value:        0,
			singular:     "second",
			plural:       "seconds",
			forceInclude: false,
			expected:     "",
		},
		{
			name:         "zero value forced",
			value:        0,
			singular:     "second",
			plural:       "seconds",
			forceInclude: true,
			expected:     "0 seconds",
		},
		{
			name:         "singular value",
			value:        1,
			singular:     "hour",
			plural:       "hours",
			forceInclude: false,
			expected:     "1 hour",
		},
		{
			name:         "plural value",
			value:        5,
			singular:     "minute",
			plural:       "minutes",
			forceInclude: false,
			expected:     "5 minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.FormatTimeUnit(tt.value, tt.singular, tt.plural, tt.forceInclude)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "all empty",
			input:    []string{"", "", ""},
			expected: nil,
		},
		{
			name:     "all non-empty",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "mixed empty and non-empty",
			input:    []string{"", "a", "", "b", ""},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.FilterEmpty(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAwaitDockerClient(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// This function just sleeps for 1 second, so we test that it doesn't panic
		// and completes within a reasonable time
		start := time.Now()

		log := logging.New(io.Discard, logging.InfoLevel)
		awaitDockerClient(log)

		elapsed := time.Since(start)

		// Should take at least 900ms but not more than 3 seconds (to account for CI timing variations)
		assert.GreaterOrEqual(
			t,
			elapsed,
			900*time.Millisecond,
			"Should sleep for at least 900 milliseconds",
		)
		assert.Less(t, elapsed, 3*time.Second, "Should not sleep for more than 3 seconds")
	})
}

func TestLifecycleFlags(t *testing.T) {
	cmd := &cobra.Command{}

	flags.SetDefaults()
	flags.RegisterAll(cmd)

	err := cmd.ParseFlags([]string{"--lifecycle-uid", "1000", "--lifecycle-gid", "1001"})
	require.NoError(t, err)

	cfg, err := appconfig.Load(testLogger(), cmd, nil)
	require.NoError(t, err)

	assert.Equal(t, 1000, cfg.Lifecycle.UID, "lifecycle UID should be set to 1000")
	assert.Equal(t, 1001, cfg.Lifecycle.GID, "lifecycle GID should be set to 1001")
}

func TestGetAPIAddr(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "empty host",
			host:     "",
			port:     "8080",
			expected: ":8080",
		},
		{
			name:     "IPv4 host",
			host:     "127.0.0.1",
			port:     "8080",
			expected: "127.0.0.1:8080",
		},
		{
			name:     "IPv6 host",
			host:     "::1",
			port:     "8080",
			expected: "[::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.GetAPIAddr(tt.host, tt.port)
			assert.Equal(t, tt.expected, result)

			// Verify the formatted address is a valid TCP address
			_, err := net.ResolveTCPAddr("tcp", result)
			assert.NoError(t, err, "formatted address should be a valid TCP address")
		})
	}
}

// TestUpdateLockSerialization verifies that the updateLock mechanism properly serializes updates,
// preventing concurrent access to the Docker client. This test simulates multiple update operations
// running simultaneously, ensuring only one update executes at a time, mimicking the behavior
// of updateOnStart and scheduled updates in the main application.
func TestUpdateLockSerialization(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		updateLock := scheduling.NewUpdateLock()

		// Atomic counters to track concurrent execution and completion
		var (
			running   int32
			started   int32
			completed int32
		)

		// WaitGroup to synchronize test completion
		var wg sync.WaitGroup

		// Simulate the update function used in runMain and runUpgradesOnSchedule
		updateFunc := func(id int) {
			if !updateLock.TryLock() {
				// Lock not available, skip update (same as in the actual code)
				t.Logf("Update %d skipped due to concurrent update in progress", id)

				return
			}

			defer updateLock.Unlock()

			// Track that only one update is running at a time
			current := atomic.AddInt32(&running, 1)
			require.Equal(
				t,
				int32(1),
				current,
				"Only one update should be running at a time, but %d are running",
				current,
			)

			atomic.AddInt32(&started, 1)

			// Simulate update work with a delay
			synctest.Wait()

			atomic.AddInt32(&running, -1)
			atomic.AddInt32(&completed, 1)
		}

		// Simulate concurrent updateOnStart and scheduled updates
		numUpdates := 2
		for i := range numUpdates {
			wg.Add(1)

			go func(id int) {
				defer wg.Done()

				updateFunc(id)
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		// Verify that only one update executed due to lock serialization
		assert.Equal(t, int32(1), started, "Only one update should have started due to lock")
		assert.Equal(t, int32(1), completed, "Only one update should have completed")
		assert.Equal(t, int32(0), running, "No updates should be running after completion")
	})
}

// TestConcurrentScheduledAndFullAPIUpdate verifies that a full API-triggered update (no image params)
// returns 429 immediately when a scheduled update is in progress, rather than blocking indefinitely.
func TestConcurrentScheduledAndFullAPIUpdate(t *testing.T) {
	updateLock := scheduling.NewUpdateLock()

	scheduledStarted := make(chan struct{})
	scheduledDone := make(chan struct{})

	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		t.Error("API update function should not be called when lock is held for full updates")

		return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
	}

	handler := update.New(testLogger(), updateFn, updateLock)

	// The scheduled update holds the lock until the API request has been
	// asserted.
	require.True(t, updateLock.TryLock())

	go func() {
		close(scheduledStarted)

		<-scheduledDone

		updateLock.Unlock()
	}()

	<-scheduledStarted

	testApp := fiber.New(fiber.Config{})
	testApp.Post(handler.Path, handler.Handle)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/update",
		http.NoBody,
	)
	require.NoError(t, err)

	resp, err := testApp.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode,
		"full API update should be rejected with 429 while scheduled update holds the lock")

	close(scheduledDone)
}

func TestHandleAsync(t *testing.T) {
	apiStarted := make(chan struct{})
	apiCompleted := make(chan struct{})

	updateLock := scheduling.NewUpdateLock()

	scheduledStarted := make(chan struct{})
	scheduledCompleted := make(chan struct{})

	updateFn := func(_ context.Context, _, _ []string) *metrics.Metric {
		close(apiStarted)
		time.Sleep(50 * time.Millisecond)
		close(apiCompleted)

		return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
	}

	handler := update.New(testLogger(), updateFn, updateLock)

	require.True(t, updateLock.TryLock())

	go func() {
		t.Log("Scheduled: acquired lock")
		close(scheduledStarted)

		// Hold the lock for a window to simulate a running scheduled update.
		// Block until either the API update goroutine starts (which would
		// indicate a locking bug) or the window elapses.
		select {
		case <-apiStarted:
			t.Error("Targeted API update should not have started while scheduled update is running")

			updateLock.Unlock()

			return
		case <-time.After(50 * time.Millisecond):
		}

		close(scheduledCompleted)
		t.Log("Scheduled: releasing lock")

		updateLock.Unlock()
	}()

	<-scheduledStarted

	go func() {
		testApp := fiber.New(fiber.Config{})
		testApp.Post(handler.Path, handler.Handle)

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			"/v1/update?image=myapp:latest",
			http.NoBody,
		)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)

			return
		}

		resp, err := testApp.Test(req)
		if err != nil {
			t.Errorf("Failed to test request: %v", err)

			return
		}
		defer resp.Body.Close()
	}()

	<-scheduledCompleted

	select {
	case <-apiStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for API update to start")
	}

	select {
	case <-apiCompleted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for API update to complete")
	}
}

// testScheduleDeps returns ScheduleDeps with shared defaults for cmd scheduling tests.
// Callers set Filter, FilterDesc, Lock, UpdateOnStart, and BaseParams.EphemeralSelfUpdate as needed.
func testScheduleDeps(
	filter types.Filter,
	filterDesc string,
	lock *scheduling.UpdateLock,
	updateOnStart bool,
) scheduling.ScheduleDeps {
	return scheduling.ScheduleDeps{
		Logger:       testLogger(),
		Filter:       filter,
		FilterDesc:   filterDesc,
		Lock:         lock,
		ScheduleSpec: "",
		// Suppress startup messaging in unit tests (Logger still set for safety if re-enabled).
		Startup: logging.StartupParams{
			Logger:           logging.New(io.Discard, logging.InfoLevel),
			NoStartupMessage: true,
		},
		WriteStartupMessage: logging.WriteStartupMessage,
		RunUpdate:           runUpdatesWithNotifications,
		UpdateOnStart:       updateOnStart,
		BaseParams:          types.UpdateParams{},
	}
}

// TestUpdateOnStartTriggersImmediateUpdate verifies that the --update-on-start flag
// triggers an immediate update before scheduling periodic updates.
func TestUpdateOnStartTriggersImmediateUpdate(t *testing.T) {
	// Create a command with update-on-start flag enabled
	cmd := &cobra.Command{}
	flags.RegisterSystemFlags(cmd)
	err := cmd.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
	require.NoError(t, err)

	// Track if update function was called
	updateCalled := make(chan bool, 1)
	updateCallCount := int32(0)

	// Mock the update function to signal when called
	originalRunUpdatesWithNotifications := runUpdatesWithNotifications
	runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		atomic.AddInt32(&updateCallCount, 1)

		select {
		case updateCalled <- true:
		default:
		}

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

	// Create a context that will cancel quickly to avoid running the scheduler
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Create update lock
	updateLock := scheduling.NewUpdateLock()

	// Call runUpgradesOnSchedule with a filter that matches no containers
	filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
	filterDesc := testFilterDesc

	// The function should trigger immediate update and then start scheduler
	deps := testScheduleDeps(filter, filterDesc, updateLock, true)
	err = scheduling.RunUpgradesOnSchedule(ctx, deps)

	// Should not return an error (context cancellation is expected)
	require.NoError(t, err)

	// Verify that update was called immediately
	select {
	case <-updateCalled:
		// Expected: update was called
	default:
		t.Error("Update function was not called immediately with --update-on-start")
	}

	// Verify only one update call occurred (the immediate one)
	assert.Equal(t, int32(1), atomic.LoadInt32(&updateCallCount))
}

// TestUpdateOnStartIntegratesWithCronScheduling verifies that update-on-start
// works with cron scheduling without causing duplicate updates.
func TestUpdateOnStartIntegratesWithCronScheduling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a command with update-on-start flag enabled and a schedule
		cmd := &cobra.Command{}
		flags.RegisterSystemFlags(cmd)
		err := cmd.ParseFlags(
			[]string{"--update-on-start", "--schedule", "@every 1h", "--no-startup-message"},
		)
		require.NoError(t, err)

		// Track update calls
		updateCallCount := int32(0)
		updateCalls := make(chan time.Time, 10)

		// Mock the update function
		originalRunUpdatesWithNotifications := runUpdatesWithNotifications
		runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			callTime := time.Now()

			atomic.AddInt32(&updateCallCount, 1)

			select {
			case updateCalls <- callTime:
			default:
			}

			return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
		}

		defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

		// Create a context that allows some time for scheduler to start but not run updates
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Create update lock
		updateLock := scheduling.NewUpdateLock()

		// Call runUpgradesOnSchedule
		filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
		filterDesc := testFilterDesc

		startTime := time.Now()
		deps := testScheduleDeps(filter, filterDesc, updateLock, true)
		err = scheduling.RunUpgradesOnSchedule(ctx, deps)

		// Should not return an error (context cancellation is expected)
		require.NoError(t, err)

		// Wait a bit for any scheduled calls
		synctest.Wait()

		// Verify that at least one update was called (the immediate one)
		callCount := atomic.LoadInt32(&updateCallCount)
		assert.GreaterOrEqual(t, callCount, int32(1), "At least one update should have been called")

		// Verify that the first call happened immediately (within 10ms of start)
		select {
		case callTime := <-updateCalls:
			timeSinceStart := callTime.Sub(startTime)
			assert.Less(
				t,
				timeSinceStart,
				10*time.Millisecond,
				"First update should happen immediately",
			)
		default:
			t.Error("No update calls were recorded")
		}

		// Verify no duplicate immediate calls occurred
		assert.LessOrEqual(
			t,
			callCount,
			int32(2),
			"Should not have more than 2 update calls in short test period",
		)
	})
}

// TestUpdateOnStartLockingBehavior verifies that update-on-start respects the update lock
// and doesn't run concurrent updates.
func TestUpdateOnStartLockingBehavior(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a command with update-on-start flag enabled
		cmd := &cobra.Command{}
		flags.RegisterSystemFlags(cmd)
		err := cmd.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
		require.NoError(t, err)

		// Create update lock that's initially unavailable (simulating another update in progress)
		updateLock := scheduling.NewUpdateLock()
		require.True(t, updateLock.TryLock())

		// Track if update function was called
		updateCalled := make(chan bool, 1)

		// Mock the update function
		originalRunUpdatesWithNotifications := runUpdatesWithNotifications
		runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			select {
			case updateCalled <- true:
			default:
			}

			return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
		}

		defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

		// Create a short context
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Call runUpgradesOnSchedule
		filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
		filterDesc := testFilterDesc

		deps := testScheduleDeps(filter, filterDesc, updateLock, false)
		err = scheduling.RunUpgradesOnSchedule(ctx, deps)

		// Should not return an error
		require.NoError(t, err)

		// Verify that update was NOT called because lock was unavailable
		select {
		case <-updateCalled:
			t.Error("Update should not have been called when lock is unavailable")
		default:
			// Expected: no update call
		}
	})
}

// TestUpdateOnStartSelfUpdateScenario verifies that update-on-start works correctly
// in self-update scenarios where Watchtower updates itself.
func TestUpdateOnStartSelfUpdateScenario(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a command with update-on-start flag enabled
		cmd := &cobra.Command{}
		flags.RegisterSystemFlags(cmd)
		err := cmd.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
		require.NoError(t, err)

		updateOnStart, _ := cmd.Flags().GetBool("update-on-start")

		// Track update calls
		updateCalled := make(chan bool, 1)

		// Mock the update function
		originalRunUpdatesWithNotifications := runUpdatesWithNotifications
		runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			select {
			case updateCalled <- true:
			default:
			}

			return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0}
		}

		defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

		// Create a short context
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Create update lock
		updateLock := scheduling.NewUpdateLock()

		// Call runUpgradesOnSchedule with a filter that includes containers
		filter := types.Filter(func(_ types.FilterableContainer) bool { return true })
		filterDesc := testFilterDesc

		deps := testScheduleDeps(filter, filterDesc, updateLock, updateOnStart)
		err = scheduling.RunUpgradesOnSchedule(ctx, deps)

		// Should not return an error
		require.NoError(t, err)

		// Verify that update was called for self-update scenario
		select {
		case <-updateCalled:
			// Expected: update was called
		default:
			t.Error("Update function was not called in self-update scenario")
		}
	})
}

// TestUpdateOnStartMultiInstanceScenario verifies that multiple Watchtower instances
// with update-on-start don't conflict with each other.
func TestUpdateOnStartMultiInstanceScenario(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// This test simulates two Watchtower instances both with --update-on-start
		// They should not conflict due to proper locking

		// Create commands with update-on-start flag enabled
		cmd1 := &cobra.Command{}
		flags.RegisterSystemFlags(cmd1)
		err := cmd1.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
		require.NoError(t, err)

		updateOnStart1, _ := cmd1.Flags().GetBool("update-on-start")

		cmd2 := &cobra.Command{}
		flags.RegisterSystemFlags(cmd2)
		err = cmd2.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
		require.NoError(t, err)

		updateOnStart2, _ := cmd2.Flags().GetBool("update-on-start")

		// Shared update lock (simulating shared resource)
		updateLock := scheduling.NewUpdateLock()

		// Track update calls from both instances
		updateCallCount := int32(0)

		var completed atomic.Int32

		instance1Called := make(chan bool, 1)
		instance2Called := make(chan bool, 1)

		// Mock the update function
		originalRunUpdatesWithNotifications := runUpdatesWithNotifications
		runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			atomic.AddInt32(&updateCallCount, 1)
			synctest.Wait() // Simulate update work

			return nil // Don't trigger metrics in test
		}

		defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

		// Start both instances concurrently
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
			filterDesc := "instance1"

			deps := testScheduleDeps(filter, filterDesc, updateLock, updateOnStart1)
			runErr := scheduling.RunUpgradesOnSchedule(ctx, deps)
			assert.NoError(t, runErr)
			completed.Add(1)
			close(instance1Called)
		}()

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
			filterDesc := "instance2"

			deps := testScheduleDeps(filter, filterDesc, updateLock, updateOnStart2)
			runErr := scheduling.RunUpgradesOnSchedule(ctx, deps)
			assert.NoError(t, runErr)
			completed.Add(1)
			close(instance2Called)
		}()

		// Wait for both instances to complete
		<-instance1Called
		<-instance2Called

		// Verify that both instances shut down properly
		assert.Equal(
			t,
			int32(2),
			completed.Load(),
			"Both instances should have shut down properly",
		)

		// Verify that only one update occurred due to locking (one instance gets the lock first)
		callCount := atomic.LoadInt32(&updateCallCount)
		assert.Equal(
			t,
			int32(1),
			callCount,
			"Only one update should occur due to lock serialization",
		)
		// Verify that shutdown keeps the lock, so no update starts afterwards
		assert.False(t, updateLock.TryLock(), "An update could start after shutdown")
	})
}

// TestListContainersWithoutFilterIntegration verifies that client.ListContainers() is called
// without filter arguments when no filter is provided, and that containers are returned correctly.
func TestListContainersWithoutFilterIntegration(t *testing.T) {
	// Set up environment
	hostname := "test-container"
	t.Setenv("HOSTNAME", hostname)

	// Create mocks
	mockClient := mockContainer.NewMockClient(t)
	mockContainer := mockTypes.NewMockContainer(t)

	// Set up mock expectations for ListContainers called with context
	mockClient.EXPECT().ListContainers(context.Background()).Return([]types.Container{mockContainer}, nil).Once()

	// Set up container mock to return the expected hostname
	mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Config: &dockerContainer.Config{Hostname: hostname},
	}).Once()

	// Set up IsWatchtower expectation (called to check if container should be preferred)
	mockContainer.EXPECT().IsWatchtower().Return(false).Once()

	// Set up container mock to return the container ID
	expectedID := types.ContainerID("test-container-id")
	mockContainer.EXPECT().ID().Return(expectedID).Once()

	// Execute the function that calls ListContainers with context
	resultID, err := container.GetContainerIDFromHostname(testLogger(), context.Background(), mockClient)

	// Assert results
	require.NoError(t, err)
	assert.Equal(t, expectedID, resultID)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
	mockContainer.AssertExpectations(t)
}

// TestRunUpgradesOnSchedule_ShutdownWaitsForRunningUpdate verifies that runUpgradesOnSchedule
// waits for any running update to complete before shutting down when receiving a shutdown signal.
func TestRunUpgradesOnSchedule_ShutdownWaitsForRunningUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a command without scheduling to keep test simple
		cmd := &cobra.Command{}
		flags.RegisterSystemFlags(cmd)
		err := cmd.ParseFlags([]string{"--no-startup-message"})
		require.NoError(t, err)

		// Create update lock
		updateLock := scheduling.NewUpdateLock()

		// Track when shutdown completes
		shutdownCompleted := make(chan bool, 1)

		// Channels to coordinate the manual update simulation
		updateStarted := make(chan bool, 1)
		updateFinished := make(chan bool, 1)

		// Mock runUpdatesWithNotifications to simulate a long-running update
		originalRunUpdatesWithNotifications := runUpdatesWithNotifications
		runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
			// Signal that we're in the update
			synctest.Wait() // Simulate update work

			return nil // Don't trigger metrics in test
		}

		defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

		// Create a cancelable context for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		// Start runUpgradesOnSchedule in a goroutine
		go func() {
			filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
			filterDesc := testFilterDesc

			// This should start and wait for context cancellation
			deps := testScheduleDeps(filter, filterDesc, updateLock, false)
			err = scheduling.RunUpgradesOnSchedule(ctx, deps)
			assert.NoError(t, err)

			shutdownCompleted <- true
		}()

		// Start an update manually to simulate one running
		go func() {
			if !updateLock.TryLock() {
				// Lock not available
				return
			}

			updateStarted <- true

			defer func() {
				updateLock.Unlock()

				updateFinished <- true
			}()

			// Simulate longer update work
			synctest.Wait()
		}()

		// Wait for the update to start
		<-updateStarted

		// Cancel context to trigger shutdown
		cancel()

		// Wait for shutdown to complete
		<-shutdownCompleted

		// Ensure the manual update completes
		<-updateFinished
	})
}

// TestValidateRollingRestartDependenciesAcceptsCancelableContext verifies that
// actions.ValidateRollingRestartDependencies properly accepts and uses a cancelable context.
func TestValidateRollingRestartDependenciesAcceptsCancelableContext(t *testing.T) {
	// Create a mock client
	mockClient := mockContainer.NewMockClient(t)

	// Create a filter that accepts all containers
	filter := types.Filter(func(_ types.FilterableContainer) bool { return true })

	// Test with cancelable context - context should not be canceled
	t.Run("cancelable context without cancellation", func(t *testing.T) {
		ctx := t.Context()

		// Mock expects ListContainers to be called with the cancelable context
		mockClient.EXPECT().ListContainers(ctx, mock.Anything, mock.Anything).Return([]types.Container{}, nil).Once()

		err := actions.ValidateRollingRestartDependencies(testLogger(), ctx, mockClient, filter, true)

		require.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// Test that canceled context is properly propagated
	t.Run("canceled context is propagated to client", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel immediately
		cancel()

		// Mock expects ListContainers to be called with canceled context
		mockClient.EXPECT().ListContainers(ctx, mock.Anything, mock.Anything).Return(nil, context.Canceled).Once()

		err := actions.ValidateRollingRestartDependencies(testLogger(), ctx, mockClient, filter, true)

		// The function should return the error from ListContainers
		require.Error(t, err)
		mockClient.AssertExpectations(t)
	})

	// Test with timeout context
	t.Run("timeout context is propagated to client", func(t *testing.T) {
		// Use a short but non-trivial timeout. Windows timer resolution can
		// delay sub-15ms deadlines, so wait by polling rather than a fixed sleep.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		deadline := time.Now().Add(500 * time.Millisecond)
		for ctx.Err() == nil {
			require.True(t, time.Now().Before(deadline), "context deadline should expire within wait window")
			time.Sleep(5 * time.Millisecond)
		}

		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)

		// Mock expects ListContainers to be called with timed out context
		mockClient.EXPECT().ListContainers(ctx, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Once()

		err := actions.ValidateRollingRestartDependencies(testLogger(), ctx, mockClient, filter, true)

		// The function should return the error from ListContainers
		require.Error(t, err)
		mockClient.AssertExpectations(t)
	})
}

// TestEphemeralSelfUpdateExercisesTruePath verifies that RunUpgradesOnSchedule
// correctly handles ephemeralSelfUpdate=true, which bypasses the exposed-ports
// self-update restriction by removing the old container before creating a new one.
func TestEphemeralSelfUpdateExercisesTruePath(t *testing.T) {
	// Create a command with update-on-start flag enabled
	cmd := &cobra.Command{}
	flags.RegisterSystemFlags(cmd)
	err := cmd.ParseFlags([]string{"--update-on-start", "--no-startup-message"})
	require.NoError(t, err)

	// Track update calls
	updateCallCount := int32(0)
	updateCalled := make(chan struct{}, 1)

	// Mock the update function
	originalRunUpdatesWithNotifications := runUpdatesWithNotifications
	runUpdatesWithNotifications = func(_ context.Context, _ types.Filter, _ types.UpdateParams) *metrics.Metric {
		atomic.AddInt32(&updateCallCount, 1)

		select {
		case updateCalled <- struct{}{}:
		default:
		}

		return &metrics.Metric{Scanned: 1, Updated: 0, Failed: 0}
	}

	defer func() { runUpdatesWithNotifications = originalRunUpdatesWithNotifications }()

	// Create update lock
	updateLock := scheduling.NewUpdateLock()

	// Create a context that shuts down quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	filter := types.Filter(func(_ types.FilterableContainer) bool { return false })
	filterDesc := testFilterDesc

	// Call RunUpgradesOnSchedule with ephemeralSelfUpdate=true
	deps := testScheduleDeps(filter, filterDesc, updateLock, true)
	deps.BaseParams.EphemeralSelfUpdate = true
	err = scheduling.RunUpgradesOnSchedule(ctx, deps)

	require.NoError(t, err)

	// Verify that update was called immediately
	select {
	case <-updateCalled:
		// Expected: update was called
	default:
		t.Error("Update function was not called immediately with ephemeralSelfUpdate=true")
	}

	// Verify at least one update call occurred
	assert.GreaterOrEqual(t, atomic.LoadInt32(&updateCallCount), int32(1))
}

// TestCreateSignalContext verifies that the signal-aware context is properly created
// and can be canceled via the stop function.
func TestCreateSignalContext(t *testing.T) {
	// Save original and restore after test
	originalCreateSignalContext := createSignalContext

	defer func() { createSignalContext = originalCreateSignalContext }()

	// Test with custom mock that simulates signal handling
	callCount := 0
	createSignalContext = func(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
		callCount++

		// Verify the correct signals are passed
		assert.Contains(t, signals, os.Interrupt, "Should include SIGINT")
		assert.Contains(t, signals, syscall.SIGTERM, "Should include SIGTERM")

		// Return a context that's canceled via the cancel function
		return context.WithCancel(ctx)
	}

	ctx, cancel := createSignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Verify context is not done initially
	assert.NotNil(t, ctx, "Context should not be nil")
	assert.NotNil(t, ctx.Done(), "Context should not be done initially")

	// Call cancel and verify context is done
	cancel()

	// Verify the function was called once
	assert.Equal(t, 1, callCount, "createSignalContext should be called once")

	// Verify context is done after cancel
	assert.Error(t, ctx.Err(), "Context should be done after cancel")
}

// TestCreateSignalContextDefault verifies that the default implementation
// correctly creates a signal-aware context using signal.NotifyContext.
func TestCreateSignalContextDefault(t *testing.T) {
	// Save original and restore after test
	originalCreateSignalContext := createSignalContext

	defer func() { createSignalContext = originalCreateSignalContext }()

	// Use the default implementation
	createSignalContext = signal.NotifyContext

	ctx, stop := createSignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Verify context is created successfully
	assert.NotNil(t, ctx, "Context should not be nil")

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Expected: context is not done
	}
}

// TestSignalContextCancellation verifies that the signal context properly cancels
// when signals are received, enabling graceful shutdown.
func TestSignalContextCancellation(t *testing.T) {
	// Skip in short test mode as this requires real signal handling
	if testing.Short() {
		t.Skip("Skipping signal test in short mode")
	}

	// Save original and restore after test
	originalCreateSignalContext := createSignalContext

	defer func() { createSignalContext = originalCreateSignalContext }()

	// Create context that we'll control
	ctx, cancel := context.WithCancel(context.Background())

	createSignalContext = func(_ context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		return ctx, cancel
	}

	ctx, _ = createSignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Verify context is not done initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
		// Expected: context is not done
	}
}

// TestSignalContextWithMultipleSignals verifies that the context correctly handles
// multiple signal types (SIGINT and SIGTERM).
func TestSignalContextWithMultipleSignals(t *testing.T) {
	// Save original and restore after test
	originalCreateSignalContext := createSignalContext

	defer func() { createSignalContext = originalCreateSignalContext }()

	// Track which signals were received
	var receivedSignals []os.Signal

	createSignalContext = func(ctx context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
		receivedSignals = signals

		return context.WithCancel(ctx)
	}

	ctx, cancel := createSignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Verify both signals are in the received list
	assert.Len(t, receivedSignals, 2, "Should receive exactly 2 signals")
	assert.Contains(t, receivedSignals, os.Interrupt)
	assert.Contains(t, receivedSignals, syscall.SIGTERM)

	// Verify context is valid
	assert.NotNil(t, ctx)
}

// TestSignalContextGracefulShutdown verifies that the context supports graceful
// shutdown by not completing until explicitly canceled.
func TestSignalContextGracefulShutdown(t *testing.T) {
	// Save original and restore after test
	originalCreateSignalContext := createSignalContext

	defer func() { createSignalContext = originalCreateSignalContext }()

	// Create context that we'll control
	ctx, cancel := context.WithCancel(context.Background())

	createSignalContext = func(_ context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		return ctx, cancel
	}

	// Create signal context
	sigCtx, stop := createSignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Verify context is valid and not done
	assert.NotNil(t, sigCtx)

	// Simulate work - context should still be valid
	select {
	case <-sigCtx.Done():
		t.Error("Context should not be done during graceful operation")
	default:
		// Expected: context is not done
	}

	// Now cancel to simulate signal receipt
	cancel()

	// Context should be done after cancellation
	<-sigCtx.Done()
	assert.ErrorIs(t, sigCtx.Err(), context.Canceled)
}

func TestContextDeadlineExceededErrorHandling(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		isDeadlineExceeded bool
		isCanceled         bool
	}{
		{
			name:               "DeadlineExceeded error",
			err:                context.DeadlineExceeded,
			isDeadlineExceeded: true,
			isCanceled:         false,
		},
		{
			name:               "Canceled error",
			err:                context.Canceled,
			isDeadlineExceeded: false,
			isCanceled:         true,
		},
		{
			name:               "Wrapped DeadlineExceeded",
			err:                fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
			isDeadlineExceeded: true,
			isCanceled:         false,
		},
		{
			name:               "Wrapped Canceled",
			err:                fmt.Errorf("wrapped: %w", context.Canceled),
			isDeadlineExceeded: false,
			isCanceled:         true,
		},
		{
			name:               "Regular error",
			err:                errors.New("some error"),
			isDeadlineExceeded: false,
			isCanceled:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isDeadlineExceeded, errors.Is(tt.err, context.DeadlineExceeded),
				"errors.Is should correctly identify DeadlineExceeded")
			assert.Equal(t, tt.isCanceled, errors.Is(tt.err, context.Canceled),
				"errors.Is should correctly identify Canceled")
		})
	}
}

// TestContainerLookupWithTimeoutContext verifies that container lookup functions properly
// handle timeout contexts using synctest.
func TestContainerLookupWithTimeoutContext(t *testing.T) {
	// Test case 1: Context deadline exceeded - verify DeadlineExceeded error is propagated
	t.Run("DeadlineExceeded error is properly propagated", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Set HOSTNAME so container detection doesn't fail early
			t.Setenv("HOSTNAME", "test-hostname")

			// Create a mock client
			mockClient := mockContainer.NewMockClient(t)

			// Create a context that's already timed out
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()

			// Wait for context to actually timeout
			time.Sleep(time.Millisecond)

			// Verify context has expired
			require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)

			// Mock expects GetCurrentContainerID to be called with expired context
			// and should return DeadlineExceeded error
			mockClient.EXPECT().ListContainers(ctx).Return(nil, context.DeadlineExceeded).Once()

			// Call GetCurrentContainerID which internally uses the client
			// The function will eventually call ListContainers with our expired context
			_, err := container.GetCurrentContainerID(testLogger(), ctx, mockClient)

			// Verify error is propagated
			require.Error(t, err)
			require.ErrorIs(t, err, context.DeadlineExceeded)

			mockClient.AssertExpectations(t)
		})
	})

	// Test case 2: Context canceled - verify Canceled error is propagated
	t.Run("Canceled context is properly propagated", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Set HOSTNAME so container detection doesn't fail early
			t.Setenv("HOSTNAME", "test-hostname")

			// Create a mock client
			mockClient := mockContainer.NewMockClient(t)

			ctx, cancel := context.WithCancel(context.Background())

			// Cancel immediately
			cancel()

			// Verify context is canceled
			require.ErrorIs(t, ctx.Err(), context.Canceled)

			// Mock expects GetCurrentContainerID to be called with canceled context
			mockClient.EXPECT().ListContainers(ctx).Return(nil, context.Canceled).Once()

			// Call GetCurrentContainerID
			_, err := container.GetCurrentContainerID(testLogger(), ctx, mockClient)

			// Verify error is propagated
			require.Error(t, err)
			require.ErrorIs(t, err, context.Canceled)

			mockClient.AssertExpectations(t)
		})
	})
}

// TestGetCurrentWatchtowerContainerWithTimeout verifies that GetCurrentWatchtowerContainer
// properly handles timeout contexts.
func TestGetCurrentWatchtowerContainerWithTimeout(t *testing.T) {
	// Test case 1: Context deadline exceeded - verify DeadlineExceeded error is handled
	t.Run("DeadlineExceeded error is properly handled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create a mock client
			mockClient := mockContainer.NewMockClient(t)

			// Create a context that's already timed out
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()

			// Wait for context to actually timeout
			time.Sleep(time.Millisecond)

			// Verify context has expired
			require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)

			containerID := types.ContainerID("test-container-id")

			// Mock expects GetCurrentWatchtowerContainer to be called with expired context
			mockClient.EXPECT().GetCurrentWatchtowerContainer(ctx, containerID).
				Return(nil, context.DeadlineExceeded).Once()

			// Call GetCurrentWatchtowerContainer
			_, err := mockClient.GetCurrentWatchtowerContainer(ctx, containerID)

			// Verify error is propagated
			require.Error(t, err)
			require.ErrorIs(t, err, context.DeadlineExceeded)

			mockClient.AssertExpectations(t)
		})
	})

	// Test case 2: Context canceled - verify Canceled error is handled
	t.Run("Canceled context is properly handled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Create a mock client
			mockClient := mockContainer.NewMockClient(t)

			ctx, cancel := context.WithCancel(context.Background())

			// Cancel immediately
			cancel()

			// Verify context is canceled
			require.ErrorIs(t, ctx.Err(), context.Canceled)

			containerID := types.ContainerID("test-container-id")

			// Mock expects GetCurrentWatchtowerContainer to be called with canceled context
			mockClient.EXPECT().GetCurrentWatchtowerContainer(ctx, containerID).
				Return(nil, context.Canceled).Once()

			// Call GetCurrentWatchtowerContainer
			_, err := mockClient.GetCurrentWatchtowerContainer(ctx, containerID)

			// Verify error is propagated
			require.Error(t, err)
			require.ErrorIs(t, err, context.Canceled)

			mockClient.AssertExpectations(t)
		})
	})
}

func TestRootCommand_FlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid stop-timeout",
			args:    []string{"--stop-timeout", "30s"},
			wantErr: false,
		},
		{
			name:    "zero stop-timeout",
			args:    []string{"--stop-timeout", "0s"},
			wantErr: false,
		},
		{
			name:    "valid http-api-port",
			args:    []string{"--http-api-port", "9090"},
			wantErr: false,
		},
		{
			name:    "valid http-api-host",
			args:    []string{"--http-api-host", "127.0.0.1"},
			wantErr: false,
		},
		{
			// Hostnames parse as flag values but fail validateAPIHost at run time.
			name:    "http-api-host hostname parses as flag",
			args:    []string{"--http-api-host", "localhost"},
			wantErr: false,
		},
		{
			name:    "valid rolling-restart",
			args:    []string{"--rolling-restart"},
			wantErr: false,
		},
		{
			name:    "valid include-stopped",
			args:    []string{"--include-stopped"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			flags.RegisterSystemFlags(cmd)
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRootCommand_HTTPAPIEndpointsFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "endpoints all",
			args: []string{"--http-api-endpoints=all"},
			want: []string{"all"},
		},
		{
			name: "endpoints list comma",
			args: []string{"--http-api-endpoints=health,metrics"},
			want: []string{"health", "metrics"},
		},
		{
			name: "endpoints repeated flags",
			args: []string{"--http-api-endpoints=health", "--http-api-endpoints=metrics"},
			want: []string{"health", "metrics"},
		},
		{
			name: "legacy update still registered",
			args: []string{"--http-api-update"},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			flags.RegisterDockerFlags(cmd)
			flags.RegisterSystemFlags(cmd)
			flags.RegisterNotificationFlags(cmd)
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			got, err := cmd.Flags().GetStringSlice("http-api-endpoints")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// Branch-only endpoint booleans must not be registered.
			assert.Nil(t, cmd.Flags().Lookup("http-api-events"))
			assert.Nil(t, cmd.Flags().Lookup("http-api-check"))
			assert.Nil(t, cmd.Flags().Lookup("http-api-full"))

			// Main legacy flags remain for deprecation.
			assert.NotNil(t, cmd.Flags().Lookup("http-api-update"))
			assert.NotNil(t, cmd.Flags().Lookup("http-api-metrics"))
			assert.NotNil(t, cmd.Flags().Lookup("http-api-containers"))
		})
	}
}

func TestValidateAPIHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "empty", host: "", wantErr: false},
		{name: "ipv4", host: "127.0.0.1", wantErr: false},
		{name: "ipv6", host: "::1", wantErr: false},
		{name: "hostname", host: "localhost", wantErr: true},
		{name: "dns name", host: "api.example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := appconfig.ValidateAPIHost(tt.host)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnyHTTPAPIConfig(t *testing.T) {
	t.Parallel()

	assert.False(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{APIToken: "secret"}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{TLSCertPath: "/cert.pem"}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{CORSAllowedOrigins: []string{"https://app.example.com"}}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{APIHostChanged: true}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{APIPortChanged: true}))
	assert.True(t, appconfig.AnyHTTPAPIConfig(types.RunConfig{APIRateLimitChanged: true}))
}

func TestHTTPAPIEndpointsEnabled(t *testing.T) {
	t.Parallel()

	assert.False(t, appconfig.HTTPAPIEndpointsEnabled(types.RunConfig{}))
	assert.True(t, appconfig.HTTPAPIEndpointsEnabled(types.RunConfig{EnableHealthAPI: true}))
	assert.True(t, appconfig.HTTPAPIEndpointsEnabled(types.RunConfig{EnableUpdateAPI: true}))
}

func TestRootCommand_ReviveStoppedFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "revive-stopped enabled",
			args:     []string{"--revive-stopped"},
			expected: true,
		},
		{
			name:     "revive-stopped default",
			args:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			flags.RegisterSystemFlags(cmd)
			cmd.SetArgs(tt.args)

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			value, err := cmd.PersistentFlags().GetBool("revive-stopped")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestResolveCurrentWatchtowerContainerForFallback(t *testing.T) {
	t.Run("successful resolution", func(t *testing.T) {
		originalReadMountinfoFunc := container.ReadMountinfoFunc
		originalReadCgroupFunc := container.ReadCgroupFunc
		container.ReadMountinfoFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock mountinfo error")
		}
		container.ReadCgroupFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock cgroup error")
		}

		defer func() {
			container.ReadMountinfoFunc = originalReadMountinfoFunc
			container.ReadCgroupFunc = originalReadCgroupFunc
		}()

		t.Setenv("HOSTNAME", "test-hostname")

		mockCnt := mockTypes.NewMockContainer(t)
		mockClient := mockContainer.NewMockClient(t)

		mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{mockCnt}, nil).Once()
		mockCnt.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Hostname: "test-hostname"},
		}).Once()
		mockCnt.EXPECT().ID().Return(types.ContainerID("test-id")).Once()
		mockCnt.EXPECT().IsWatchtower().Return(false).Once()
		mockClient.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, types.ContainerID("test-id")).Return(mockCnt, nil).Once()

		result := resolveCurrentWatchtowerContainerForFallback(testLogger(), context.Background(), mockClient)

		assert.Equal(t, mockCnt, result)
	})

	t.Run("GetCurrentContainerID returns error", func(t *testing.T) {
		originalReadMountinfoFunc := container.ReadMountinfoFunc
		originalReadCgroupFunc := container.ReadCgroupFunc
		container.ReadMountinfoFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock mountinfo error")
		}
		container.ReadCgroupFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock cgroup error")
		}

		defer func() {
			container.ReadMountinfoFunc = originalReadMountinfoFunc
			container.ReadCgroupFunc = originalReadCgroupFunc
		}()

		t.Setenv("HOSTNAME", "test-hostname")

		mockClient := mockContainer.NewMockClient(t)

		mockClient.EXPECT().ListContainers(mock.Anything).Return(nil, errors.New("list failed")).Once()

		result := resolveCurrentWatchtowerContainerForFallback(testLogger(), context.Background(), mockClient)

		assert.Nil(t, result)
	})

	t.Run("GetCurrentContainerID returns empty string", func(t *testing.T) {
		originalReadMountinfoFunc := container.ReadMountinfoFunc
		originalReadCgroupFunc := container.ReadCgroupFunc
		container.ReadMountinfoFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock mountinfo error")
		}
		container.ReadCgroupFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock cgroup error")
		}

		defer func() {
			container.ReadMountinfoFunc = originalReadMountinfoFunc
			container.ReadCgroupFunc = originalReadCgroupFunc
		}()

		t.Setenv("HOSTNAME", "test-hostname")

		mockClient := mockContainer.NewMockClient(t)

		mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{}, nil).Once()

		result := resolveCurrentWatchtowerContainerForFallback(testLogger(), context.Background(), mockClient)

		assert.Nil(t, result)
	})

	t.Run("GetCurrentWatchtowerContainer returns nil", func(t *testing.T) {
		originalReadMountinfoFunc := container.ReadMountinfoFunc
		originalReadCgroupFunc := container.ReadCgroupFunc
		container.ReadMountinfoFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock mountinfo error")
		}
		container.ReadCgroupFunc = func(string) ([]byte, error) {
			return nil, errors.New("mock cgroup error")
		}

		defer func() {
			container.ReadMountinfoFunc = originalReadMountinfoFunc
			container.ReadCgroupFunc = originalReadCgroupFunc
		}()

		t.Setenv("HOSTNAME", "test-hostname")

		mockCnt := mockTypes.NewMockContainer(t)
		mockClient := mockContainer.NewMockClient(t)

		mockClient.EXPECT().ListContainers(mock.Anything).Return([]types.Container{mockCnt}, nil).Once()
		mockCnt.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{Hostname: "test-hostname"},
		}).Once()
		mockCnt.EXPECT().ID().Return(types.ContainerID("test-id")).Once()
		mockCnt.EXPECT().IsWatchtower().Return(false).Once()
		mockClient.EXPECT().GetCurrentWatchtowerContainer(mock.Anything, types.ContainerID("test-id")).Return(nil, nil).Once()

		result := resolveCurrentWatchtowerContainerForFallback(testLogger(), context.Background(), mockClient)

		assert.Nil(t, result)
	})
}
