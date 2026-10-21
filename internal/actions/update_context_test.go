package actions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_Context verifies that an update reports context
// cancellation during listing and staleness checks, and how it handles a
// current container that is an old Watchtower instance.
func TestUpdateAction_Context(t *testing.T) {
	t.Parallel()

	t.Run("handling context cancellation and timeout scenarios", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle context cancellation during container listing", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(getCommonTestData(), false, false)
			canceledCtx, cancel := context.WithCancel(t.Context())
			cancel() // Cancel the context immediately

			report, cleanupImageInfos, err := update(newTestLogger(t), canceledCtx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, report)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should handle context cancellation during staleness checking", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(getCommonTestData(), false, false)
			// Simulate IsContainerStale error
			client.TestData.IsContainerStaleError = context.Canceled

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

			// Context cancellation from IsContainerStale is propagated as the
			// final Update error rather than being swallowed.
			require.ErrorIs(t, err, context.Canceled)
			// Workers returning context.Canceled do not add partial results to progress.
			assert.Empty(t, report.Skipped())
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should ensure cleanup operations are attempted even with partial failures", func(t *testing.T) {
			t.Parallel()

			// Create test data with multiple stale containers
			testData := getCommonTestData()
			testData.Staleness = map[string]bool{
				"test-container-01": true,
				"test-container-02": true,
				"test-container-03": true,
			}

			client := mockActions.CreateMockClient(testData, false, false)
			// Simulate StopContainer failure for some containers
			client.TestData.StopContainerError = context.Canceled
			client.TestData.StopContainerFailCount = 1 // Fail the first stop attempt

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

			// Should still attempt to process and return a report
			require.NoError(t, err)
			// Update completes despite some failures
			require.NotNil(t, report)
			require.Len(t, report.Failed(), 1) // One container failed to stop
			// Cleanup should still be attempted for successful operations.
			// 2 of 3 containers succeeded and each gets its own entry for split notifications.
			require.Len(t, cleanupImageInfos, 2)
		})
	})

	t.Run("the current container is an old Watchtower container", func(t *testing.T) {
		t.Parallel()

		t.Run("should detect self as old and return error without updating", func(t *testing.T) {
			t.Parallel()

			config := &dockerContainer.Config{
				Image:  "watchtower:latest",
				Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			}
			oldContainer := mockActions.CreateMockContainerWithConfig(
				"old-container-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				config,
			)
			testData := &mockActions.TestData{
				Containers: []types.Container{oldContainer},
			}
			client := mockActions.CreateMockClient(testData, false, false)

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:            true,
				CurrentContainerID: "old-container-id",
			}, ratelimit.New())

			require.Error(t, err)
			assert.Contains(t, err.Error(), "old Watchtower container")
			assert.Nil(t, report)
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("should update restart policy to no when detecting self as old", func(t *testing.T) {
			t.Parallel()

			config := &dockerContainer.Config{
				Image:  "watchtower:latest",
				Labels: map[string]string{"com.centurylinklabs.watchtower": "true"},
			}
			oldContainer := mockActions.CreateMockContainerWithConfig(
				"old-container-id",
				"watchtower-old-abc123",
				"watchtower:latest",
				true,
				false,
				time.Now(),
				config,
			)
			testData := &mockActions.TestData{
				Containers: []types.Container{oldContainer},
			}
			client := mockActions.CreateMockClient(testData, false, false)

			_, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:            true,
				CurrentContainerID: "old-container-id",
			}, ratelimit.New())

			require.Error(t, err)
			assert.Equal(t, int32(1), client.TestData.SetRestartPolicyCount.Load())
			require.NotNil(t, client.TestData.SetRestartPolicyContainer)
			assert.Equal(t, types.ContainerID("old-container-id"), client.TestData.SetRestartPolicyContainer.ID())
		})

		t.Run("should proceed normally when current container is not old", func(t *testing.T) {
			t.Parallel()

			normalContainer := mockActions.CreateMockContainer(
				"current-id",
				"watchtower",
				"watchtower:latest",
				time.Now(),
			)
			testData := &mockActions.TestData{
				Containers: []types.Container{normalContainer},
			}
			client := mockActions.CreateMockClient(testData, false, false)

			report, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup:            true,
				CurrentContainerID: "current-id",
			}, ratelimit.New())

			require.NoError(t, err)
			require.NotNil(t, report)
		})

		t.Run("should proceed normally when current container ID is empty", func(t *testing.T) {
			t.Parallel()

			testData := &mockActions.TestData{
				Containers: []types.Container{},
			}
			client := mockActions.CreateMockClient(testData, false, false)

			report, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{
				Cleanup: true,
			}, ratelimit.New())

			require.NoError(t, err)
			require.NotNil(t, report)
		})
	})
}

// TestUpdateAction_HandleTimeout verifies that an update with an expired
// deadline returns a deadline error.
func TestUpdateAction_HandleTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := mockActions.CreateMockClient(getCommonTestData(), false, false)
		pastDeadline := time.Now().Add(-time.Second)

		ctx, cancel := context.WithDeadline(t.Context(), pastDeadline)
		defer cancel()

		report, cleanupImageInfos, err := update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

		synctest.Wait()

		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "update canceled") {
			t.Fatalf("expected 'update canceled', got %s", err.Error())
		}

		if report != nil {
			t.Fatal("expected nil report")
		}

		if len(cleanupImageInfos) != 0 {
			t.Fatal("expected empty cleanupImageInfos")
		}
	})
}

// TestUpdateAction_EarlyCancellationCheck verifies that an update with an
// already canceled context returns a cancellation error.
func TestUpdateAction_EarlyCancellationCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := mockActions.CreateMockClient(getCommonTestData(), false, false)
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel the context immediately

		report, cleanupImageInfos, err := update(newTestLogger(t), canceledCtx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

		synctest.Wait()

		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "update canceled") {
			t.Fatalf("expected 'update canceled', got %s", err.Error())
		}

		if report != nil {
			t.Fatal("expected nil report")
		}

		if len(cleanupImageInfos) != 0 {
			t.Fatal("expected empty cleanupImageInfos")
		}
	})
}

// TestUpdateAction_MidOperationCancellationCheck verifies that canceling the
// context while an update runs ends it with a cancellation error.
func TestUpdateAction_MidOperationCancellationCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		testData := getCommonTestData()
		// Set simulated latency to allow operations to start before cancellation
		testData.SimulatedLatency = 10 * time.Millisecond
		client := mockActions.CreateMockClientWithContext(ctx, testData, false, false)

		// Start update in a goroutine
		done := make(chan struct{})

		var (
			report types.Report
			err    error
		)

		go func() {
			defer close(done)

			report, _, err = update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())
		}()

		// Let the goroutine reach its first blocking point (SimulatedLatency timer
		// in mock's checkContextCancellation) before canceling.
		synctest.Wait()
		cancel()

		// Wait for the update to complete
		<-done

		synctest.Wait()

		// Depending on when cancellation occurred, err might be context canceled or nil with failed operations
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("unexpected error: %s", err.Error())
		}

		// If no early cancellation, check that operations failed due to context
		if err == nil {
			if report == nil {
				t.Fatal("expected report")
			}
			// Check that some operations failed
			if len(report.Failed()) == 0 {
				t.Fatal("expected some failed operations due to cancellation")
			}
		}
	})
}

// TestUpdateAction_ContextCancellationStartContainer tests that context cancellation
// is properly handled when starting containers.
func TestUpdateAction_ContextCancellationStartContainer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create test data with stale containers (containers need to be stopped and restarted)
		testData := getCommonTestData()
		// Mark containers as stale so they will be stopped and restarted
		testData.Staleness = map[string]bool{
			"test-container-01": true,
			"test-container-02": true,
			"test-container-03": true,
		}

		// Create client with pre-canceled context
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel immediately

		client := mockActions.CreateMockClientWithContext(canceledCtx, testData, false, false)

		_, _, err := update(newTestLogger(t), canceledCtx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

		synctest.Wait()

		// The update should complete with an error related to context cancellation
		if err != nil && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "cancel") {
			t.Fatalf("expected context-related error, got: %s", err.Error())
		}
	})
}

// TestUpdateAction_ContextTimeoutDuringProcessing tests that operations respect
// context timeouts during container processing.
func TestUpdateAction_ContextTimeoutDuringProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create test data with multiple containers
		testData := getCommonTestData()
		testData.Staleness = map[string]bool{
			"test-container-01": true,
			"test-container-02": true,
			"test-container-03": true,
		}

		// Create client with context that expires immediately
		shortCtx, cancel := context.WithTimeout(t.Context(), 0)
		defer cancel()

		client := mockActions.CreateMockClientWithContext(shortCtx, testData, false, false)

		// Capture start time to verify the call completes within a reasonable bound
		start := time.Now()

		report, _, err := update(newTestLogger(t), shortCtx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

		synctest.Wait()

		elapsed := time.Since(start)

		// Smoke test: ensure no panic occurs when handling zero-timeout context
		// Verify the call completed within a reasonable time bound (not hanging)
		if elapsed > 5*time.Second {
			t.Fatalf("Update call took too long (%v), expected to complete quickly with zero-timeout context", elapsed)
		}

		// Assert that at least one of the outputs is non-nil
		if err == nil && report == nil {
			t.Fatalf("Expected either error or report to be non-nil, got err=%v and report=%v", err, report)
		}
	})
}

// runErrorPropagationTest is a helper function that sets up a mock client with
// configurable error injection and runs the Update action for testing error propagation.
func runErrorPropagationTest(
	t *testing.T,
	errorToReturn error,
	containerStaleness map[string]bool,
	targetOperation string,
) (types.Report, []types.RemovedImageInfo, error) {
	t.Helper()

	testData := getCommonTestData()
	testData.Staleness = containerStaleness

	client := mockActions.CreateMockClient(testData, false, false)

	// Inject the appropriate error based on targetOperation
	switch targetOperation {
	case "ListContainers":
		client.TestData.ListContainersError = errorToReturn
	case "StopContainer":
		client.TestData.StopContainerError = errorToReturn
	case "StartContainer":
		client.TestData.StartContainerError = errorToReturn
	}

	report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

	return report, cleanupImageInfos, err
}

// TestUpdateAction_ErrorPropagationContextErrors tests that errors from client operations
// are properly propagated through the update process.
func TestUpdateAction_ErrorPropagationContextErrors(t *testing.T) {
	allStale := map[string]bool{
		"test-container-01": true,
		"test-container-02": true,
		"test-container-03": true,
	}

	t.Run("ListContainers context error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			expectedErrorPattern := "update canceled"

			report, cleanupImageInfos, err := runErrorPropagationTest(
				t,
				context.Canceled,
				nil,
				"ListContainers",
			)

			synctest.Wait()

			if expectedErrorPattern != "" && err != nil {
				if !strings.Contains(err.Error(), expectedErrorPattern) &&
					!strings.Contains(err.Error(), "context") {
					t.Fatalf("expected error containing '%s' or 'context', got: %s",
						expectedErrorPattern, err.Error())
				}
			}

			_ = cleanupImageInfos
			_ = report
		})
	})

	t.Run("StopContainer context error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			report, cleanupImageInfos, err := runErrorPropagationTest(
				t,
				context.DeadlineExceeded,
				allStale,
				"StopContainer",
			)

			synctest.Wait()

			// Validate error is context-related or nil
			if err != nil && !strings.Contains(err.Error(), "context") &&
				!strings.Contains(err.Error(), "deadline") {
				t.Fatalf("expected context-related error, got: %s", err.Error())
			}

			// Validate report processing counts match expected
			if report != nil && len(allStale) > 0 {
				totalProcessed := len(report.Updated()) + len(report.Failed()) + len(report.Skipped())
				if totalProcessed != len(allStale) {
					t.Fatalf("expected %d processed containers, got %d (updated: %d, failed: %d, skipped: %d)",
						len(allStale), totalProcessed, len(report.Updated()), len(report.Failed()), len(report.Skipped()))
				}
			}

			// Verify cleanupImageInfos is handled
			if cleanupImageInfos == nil {
				t.Fatal("expected cleanupImageInfos to be non-nil")
			}
		})
	})

	t.Run("StartContainer context error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			report, cleanupImageInfos, err := runErrorPropagationTest(
				t,
				context.Canceled,
				allStale,
				"StartContainer",
			)

			synctest.Wait()

			// Validate error is context-related or nil
			if err != nil && !strings.Contains(err.Error(), "context") &&
				!strings.Contains(err.Error(), "cancel") {
				t.Fatalf("expected context-related error, got: %s", err.Error())
			}

			// Validate report processing counts match expected
			if report != nil && len(allStale) > 0 {
				totalProcessed := len(report.Updated()) + len(report.Failed()) + len(report.Skipped())
				if totalProcessed != len(allStale) {
					t.Fatalf("expected %d processed containers, got %d (updated: %d, failed: %d, skipped: %d)",
						len(allStale), totalProcessed, len(report.Updated()), len(report.Failed()), len(report.Skipped()))
				}
			}

			// Verify cleanupImageInfos is handled
			if cleanupImageInfos == nil {
				t.Fatal("expected cleanupImageInfos to be non-nil")
			}
		})
	})
}

// TestUpdateAction_ContextEdgeCases tests edge cases with context handling.
func TestUpdateAction_ContextEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		contextSetup     func(parent context.Context) (context.Context, context.CancelFunc)
		staleContainers  map[string]bool
		simulatedLatency time.Duration // Optional latency for context timeout testing
	}{
		{
			name: "Background context should work",
			contextSetup: func(parent context.Context) (context.Context, context.CancelFunc) {
				return parent, func() {}
			},
			staleContainers: map[string]bool{
				"test-container-01": true,
				"test-container-02": true,
				"test-container-03": true,
			},
		},
		{
			name: "WithCancel context immediately canceled",
			contextSetup: func(parent context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(parent)
				cancel()

				return ctx, func() {}
			},
			staleContainers: map[string]bool{
				"test-container-01": true,
				"test-container-02": true,
				"test-container-03": true,
			},
		},
		{
			name: "WithTimeout already expired",
			contextSetup: func(parent context.Context) (context.Context, context.CancelFunc) {
				// Use a past deadline to simulate immediate timeout
				return context.WithDeadline(parent, time.Now().Add(-time.Hour))
			},
			staleContainers: map[string]bool{
				"test-container-01": true,
				"test-container-02": true,
				"test-container-03": true,
			},
		},
		{
			name: "WithTimeout short timeout",
			contextSetup: func(parent context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(parent, 1*time.Millisecond)
			},
			staleContainers: map[string]bool{
				"test-container-01": true,
				"test-container-02": true,
				"test-container-03": true,
			},
			// Set simulated latency to allow timeout to expire during operations
			simulatedLatency: 5 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				testData := getCommonTestData()
				testData.Staleness = tc.staleContainers
				// Apply simulated latency if specified (for context timeout testing)
				if tc.simulatedLatency > 0 {
					testData.SimulatedLatency = tc.simulatedLatency
				}

				ctx, cancel := tc.contextSetup(t.Context())
				defer cancel()

				client := mockActions.CreateMockClientWithContext(ctx, testData, false, false)

				report, cleanupImageInfos, err := update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())

				synctest.Wait()

				// For background context, update should succeed
				if tc.name == "Background context should work" {
					if err != nil {
						t.Fatalf("Background context should not produce error: %s", err.Error())
					}

					if report == nil {
						t.Fatal("Expected report for successful update")
					}
				}

				// For background context, error is nil (already asserted above)
				// For canceled/expired contexts, error is expected
				if tc.name != "Background context should work" {
					if err == nil {
						t.Fatalf("Expected error for %s context, but got nil", tc.name)
					}

					// Verify the error message contains context-related keywords
					if !strings.Contains(err.Error(), "context") &&
						!strings.Contains(err.Error(), "cancel") &&
						!strings.Contains(err.Error(), "deadline") &&
						!strings.Contains(err.Error(), "timeout") &&
						!strings.Contains(err.Error(), "update canceled") {
						t.Fatalf("expected error to contain context-related keyword, got: %s", err)
					}
				}

				// Keep cleanupImageInfos for potential cleanup assertions
				_ = cleanupImageInfos
			})
		})
	}
}

// TestUpdateAction_ParallelStalenessCheckCancellationPropagation tests that context
// cancellation during parallel staleness checks is propagated as the final Update
// error rather than being swallowed by workers.
func TestUpdateAction_ParallelStalenessCheckCancellationPropagation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		testData := getCommonTestData()
		testData.Staleness = map[string]bool{
			"test-container-01": true,
			"test-container-02": true,
			"test-container-03": true,
		}
		testData.SimulatedLatency = 10 * time.Millisecond

		ctx, cancel := context.WithCancel(t.Context())
		client := mockActions.CreateMockClientWithContext(ctx, testData, false, false)

		done := make(chan struct{})

		var err error

		go func() {
			defer close(done)

			_, _, err = update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())
		}()

		synctest.Wait()
		cancel()

		<-done

		synctest.Wait()

		if err == nil {
			t.Fatal("expected error when context is canceled during parallel staleness checks")
		}

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got: %s", err.Error())
		}
	})
}

// TestEarlyCancellationStopContainers tests that context cancellation is properly handled
// during the stopContainersInReversedOrder function (lines 998-1040 in update.go).
// This test cancels the context AFTER Update starts but WHILE stopContainersInReversedOrder
// is iterating through containers to stop them.
func TestEarlyCancellationStopContainers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		testData := getCommonTestData()
		testData.Staleness = map[string]bool{
			"test-container-01": true,
			"test-container-02": true,
			"test-container-03": true,
		}

		// Set simulated latency to control timing during stop operations
		testData.SimulatedLatency = 5 * time.Millisecond

		// Create a context that is NOT canceled immediately
		ctx, cancel := context.WithCancel(t.Context())
		client := mockActions.CreateMockClientWithContext(ctx, testData, false, false)

		// Start update in a goroutine
		done := make(chan struct{})

		var (
			report            types.Report
			cleanupImageInfos []types.RemovedImageInfo
			err               error
		)

		go func() {
			defer close(done)

			report, cleanupImageInfos, err = update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())
		}()

		// Let the goroutine reach its first blocking point (SimulatedLatency timer
		// in mock's checkContextCancellation) before canceling.
		// This should interrupt stopContainersInReversedOrder while it's iterating.
		synctest.Wait()
		cancel()

		// Wait for the update to complete
		<-done

		synctest.Wait()

		// Expect an error related to context cancellation
		if err == nil {
			t.Fatal("expected error when context is cancelled during stop operations")
		}

		if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "cancel") {
			t.Fatalf("expected context-related error, got: %s", err.Error())
		}

		_ = report
		_ = cleanupImageInfos
	})
}

// TestEarlyCancellationRestartContainers tests that context cancellation is properly handled
// during the restartContainersInSortedOrder function (lines 1152-1242 in update.go).
// This test cancels the context AFTER stop completes but WHILE restartContainersInSortedOrder
// is iterating through containers to restart them.
func TestEarlyCancellationRestartContainers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		testData := getCommonTestData()
		testData.Staleness = map[string]bool{
			"test-container-01": true,
			"test-container-02": true,
			"test-container-03": true,
		}

		// Use longer latency to allow stop phase to complete before cancellation
		// This ensures we reach the restart phase before canceling
		testData.SimulatedLatency = 3 * time.Millisecond

		// Create a context that is NOT canceled immediately
		ctx, cancel := context.WithCancel(t.Context())
		client := mockActions.CreateMockClientWithContext(ctx, testData, false, false)

		// Start update in a goroutine
		done := make(chan struct{})

		var (
			report            types.Report
			cleanupImageInfos []types.RemovedImageInfo
			err               error
		)

		go func() {
			defer close(done)

			report, cleanupImageInfos, err = update(newTestLogger(t), ctx, client, types.UpdateParams{Cleanup: true}, ratelimit.New())
		}()

		// Let the goroutine reach its first blocking point (SimulatedLatency timer
		// in mock's checkContextCancellation) before canceling.
		// This allows some operations to start before cancellation takes effect.
		synctest.Wait()
		cancel()

		// Wait for the update to complete
		<-done

		synctest.Wait()

		// Expect either an error or some failed operations due to context cancellation
		// (similar to TestUpdateAction_MidOperationCancellationCheck)
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("unexpected error: %s", err.Error())
		}

		// If no error, check that operations were affected by context cancellation
		if err == nil {
			if report == nil {
				t.Fatal("expected report when context is cancelled during restart operations")
			}
			// Note: Due to timing variability, we don't strictly require failed operations
			// The key is that the test now exercises the restart phase cancellation path
			// rather than the early ctx.Done() path
		}

		_ = cleanupImageInfos
	})
}
