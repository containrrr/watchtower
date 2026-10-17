package ratelimit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLog returns a discarded logger for retry unit tests.
//
// Returns:
//   - *zerolog.Logger: Nop logger that writes nothing.
func testLog() *zerolog.Logger {
	logger := zerolog.Nop()

	return &logger
}

func TestJitterWaitDoesNotShortenDecisionWait(t *testing.T) {
	t.Parallel()

	wait := 100 * time.Millisecond
	seen := map[time.Duration]struct{}{}
	maxExtra := wait / equalJitterDivisor

	for range 200 {
		got := jitterWait(wait)
		assert.GreaterOrEqual(t, got, wait)
		assert.LessOrEqual(t, got, wait+maxExtra)
		seen[got] = struct{}{}
	}

	assert.Greater(t, len(seen), 1)
}

func TestJitterWaitLeavesZeroUnchanged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, time.Duration(0), jitterWait(0))
}

func TestJitterWaitDoesNotExceedHonorWindow(t *testing.T) {
	t.Parallel()

	assert.Equal(t, maxHonorWait, jitterWait(maxHonorWait))
}

func TestDoLogsRetryAtDebug(t *testing.T) {
	l := New()

	var buf bytes.Buffer

	log := zerolog.New(&buf)

	var attempts atomic.Int32

	err := l.Do(t.Context(), &log, "registry.example", func() error {
		if attempts.Add(1) < 2 {
			return &Error{RetryAfter: minHonorWait}
		}

		return nil
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"level":"debug"`)
	assert.Contains(t, buf.String(), "Registry rate limited. Retrying after delay")
	assert.Contains(t, buf.String(), `"token_bucket":false`)
	assert.NotContains(t, buf.String(), `"level":"warn"`)
}

func TestDoLogsExhaustionAtWarn(t *testing.T) {
	l := New()

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.DebugLevel)

	err := l.Do(t.Context(), &log, "registry.example", func() error {
		return &Error{RetryAfter: 2 * time.Hour}
	})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, buf.String(), `"level":"warn"`)
	assert.Contains(t, buf.String(), "Registry rate limited. Stopping retries")
	assert.Contains(t, buf.String(), `"attempts":1`)
	assert.NotContains(t, buf.String(), "Retrying after delay")
}

func TestDoLogsTinyTokenBucketExhaustionAtDebug(t *testing.T) {
	l := New()

	l.bucketRetryElapsed = 250 * time.Millisecond

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.DebugLevel)

	err := l.Do(t.Context(), &log, "ghcr.io", func() error {
		return &Error{
			RetryAfter:    975 * time.Microsecond,
			Allowed:       44000,
			AllowedWindow: time.Minute,
			Host:          "ghcr.io",
		}
	})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, buf.String(), "Registry rate limited. Stopping retries")
	assert.Contains(t, buf.String(), `"token_bucket":true`)
	assert.Contains(t, buf.String(), `"level":"debug"`)
	assert.NotContains(t, buf.String(), `"level":"warn"`)
}

func TestDoRetriesPastFiveWhenRetryAfterIsTiny(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	err := l.Do(t.Context(), testLog(), "ghcr.io", func() error {
		if attempts.Add(1) < 6 {
			return &Error{
				RetryAfter:    23722 * time.Nanosecond,
				Allowed:       44000,
				AllowedWindow: time.Minute,
				Host:          "ghcr.io",
			}
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(6), attempts.Load())
}

// TestDoBacksOffExponentiallyForTinyRetryAfter covers a registry that keeps
// answering with a sub-floor Retry-After: the waits grow instead of repeating
// the floor, and the bucket budget ends the loop at debug.
func TestDoBacksOffExponentiallyForTinyRetryAfter(t *testing.T) {
	l := New()

	l.bucketRetryElapsed = time.Second

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.DebugLevel)

	var attempts atomic.Int32

	err := l.Do(t.Context(), &log, "ghcr.io", func() error {
		attempts.Add(1)

		return &Error{
			RetryAfter:    23722 * time.Nanosecond,
			Allowed:       44000,
			AllowedWindow: time.Minute,
		}
	})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, buf.String(), "Registry rate limited. Stopping retries")
	assert.Contains(t, buf.String(), `"token_bucket":true`)
	assert.Contains(t, buf.String(), `"level":"debug"`)
	assert.NotContains(t, buf.String(), `"level":"warn"`)

	// Growing waits of 100, 150, 225, 337 and 506ms fit four or five attempts
	// into one second, and a slow host only lowers that count. Flat 100 to
	// 150ms waits would fit at least seven.
	assert.GreaterOrEqual(t, attempts.Load(), int32(3))
	assert.LessOrEqual(t, attempts.Load(), int32(5))

	waits := loggedRetryWaits(t, buf.String())
	require.NotEmpty(t, waits)
	assert.GreaterOrEqual(t, waits[0], float64(minHonorWait/time.Millisecond))

	for i := 1; i < len(waits); i++ {
		assert.GreaterOrEqual(t, waits[i], waits[i-1], "wait %d shrank", i)
	}
}

// TestDoBudgetStartsAtFirstThrottle covers an attempt that spends longer than
// the whole bucket budget before its first 429, such as a pull queued behind a
// sibling on the same slot. The budget starts at that 429, so the retry runs.
func TestDoBudgetStartsAtFirstThrottle(t *testing.T) {
	l := New()

	l.bucketRetryElapsed = 200 * time.Millisecond

	var attempts atomic.Int32

	err := l.Do(t.Context(), testLog(), "ghcr.io", func() error {
		if attempts.Add(1) == 1 {
			time.Sleep(300 * time.Millisecond)

			return &Error{
				RetryAfter:    23722 * time.Nanosecond,
				Allowed:       44000,
				AllowedWindow: time.Minute,
			}
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
}

// TestDoKeepsHonorWindowForUsableRetryAfter covers a Retry-After at the floor:
// it is honored as sent and stops at the shorter honor budget, not the bucket
// budget.
func TestDoKeepsHonorWindowForUsableRetryAfter(t *testing.T) {
	l := New()

	l.retryElapsed = 250 * time.Millisecond
	l.bucketRetryElapsed = 5 * time.Second

	var buf bytes.Buffer

	log := zerolog.New(&buf).Level(zerolog.DebugLevel)

	var attempts atomic.Int32

	started := time.Now()
	err := l.Do(t.Context(), &log, "ghcr.io", func() error {
		attempts.Add(1)

		return &Error{RetryAfter: minHonorWait}
	})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, buf.String(), "Registry rate limited. Stopping retries")
	assert.Contains(t, buf.String(), `"token_bucket":false`)
	assert.NotContains(t, buf.String(), `"level":"warn"`)
	assert.GreaterOrEqual(t, attempts.Load(), int32(2))
	assert.LessOrEqual(t, attempts.Load(), int32(3))
	assert.Less(t, time.Since(started), time.Second)
}

// TestExceedsHonorWindow covers which rate limits the retry loop already
// reports at warn when it stops.
func TestExceedsHonorWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "plain error", err: errors.New("connection refused"), want: false},
		{name: "bare sentinel", err: ErrRateLimited, want: false},
		{name: "token bucket", err: &Error{RetryAfter: 347 * time.Microsecond}, want: false},
		{name: "at the honor window", err: &Error{RetryAfter: maxHonorWait}, want: false},
		{name: "beyond the honor window", err: &Error{RetryAfter: 2 * time.Hour}, want: true},
		{
			name: "wrapped beyond the honor window",
			err:  fmt.Errorf("image pull: %w", &Error{RetryAfter: 2 * time.Hour}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ExceedsHonorWindow(tt.err))
		})
	}
}

// TestMaxRetryAttemptsDerivesFromLargerBudget covers the spin breaker following
// whichever budget is longer.
func TestMaxRetryAttemptsDerivesFromLargerBudget(t *testing.T) {
	l := New()

	l.retryElapsed = minHonorWait
	l.bucketRetryElapsed = 3 * minHonorWait

	assert.Equal(t, uint(4), l.maxRetryAttempts())

	l.retryElapsed = 5 * minHonorWait
	l.bucketRetryElapsed = minHonorWait

	assert.Equal(t, uint(6), l.maxRetryAttempts())
}

// TestNewBucketPolicyStartsAtFloorAndCapsAtHonorWindow covers the policy
// bounds without sleeping.
func TestNewBucketPolicyStartsAtFloorAndCapsAtHonorWindow(t *testing.T) {
	t.Parallel()

	policy := newBucketPolicy()
	assert.Equal(t, minHonorWait, policy.NextBackOff())

	last := time.Duration(0)
	for range 30 {
		last = policy.NextBackOff()
		assert.LessOrEqual(t, last, maxHonorWait)
	}

	assert.Equal(t, maxHonorWait, last)
}

// TestBucketWaitGrowsMonotonically covers the jittered waits never shrinking
// and settling at the honor window.
func TestBucketWaitGrowsMonotonically(t *testing.T) {
	t.Parallel()

	policy := newBucketPolicy()
	prev := time.Duration(0)

	for i := range 25 {
		wait := bucketWait(policy)
		assert.GreaterOrEqual(t, wait, minHonorWait, "wait %d below the floor", i)
		assert.LessOrEqual(t, wait, maxHonorWait, "wait %d above the honor window", i)
		assert.GreaterOrEqual(t, wait, prev, "wait %d shrank", i)
		prev = wait
	}

	assert.Equal(t, maxHonorWait, prev)
}

// loggedRetryWaits extracts the retry_after values, in milliseconds, from the
// "Retrying after delay" lines of a JSON log buffer.
//
// Parameters:
//   - t: Test for fatal decode failures.
//   - logs: Newline-separated zerolog JSON output.
//
// Returns:
//   - []float64: Waits in log order.
func loggedRetryWaits(t *testing.T, logs string) []float64 {
	t.Helper()

	var waits []float64

	for line := range strings.SplitSeq(strings.TrimSpace(logs), "\n") {
		var entry struct {
			Message    string  `json:"message"`
			RetryAfter float64 `json:"retry_after"`
		}

		require.NoError(t, json.Unmarshal([]byte(line), &entry))

		if entry.Message == "Registry rate limited. Retrying after delay" {
			waits = append(waits, entry.RetryAfter)
		}
	}

	return waits
}

func TestDoRetriesRateLimitedOperations(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	err := l.Do(t.Context(), testLog(), "ghcr.io", func() error {
		if attempts.Add(1) < 3 {
			return &Error{RetryAfter: minHonorWait, Allowed: 44000, AllowedWindow: time.Minute}
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load())
}

func TestDoDoesNotRetryPermanentErrors(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	permanent := errors.New("manifest not found")

	err := l.Do(t.Context(), testLog(), "ghcr.io", func() error {
		attempts.Add(1)

		return permanent
	})
	require.ErrorIs(t, err, permanent)
	assert.Equal(t, int32(1), attempts.Load())
}

func TestDoGivesUpWhenRetryAfterExceedsHonorWindow(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	err := l.Do(t.Context(), testLog(), "ghcr.io", func() error {
		attempts.Add(1)

		return &Error{RetryAfter: 2 * time.Hour}
	})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, int32(1), attempts.Load())
}

func TestDoValueReturnsSuccessfulResult(t *testing.T) {
	l := New()

	got, err := l.DoValue(t.Context(), testLog(), "ghcr.io", func() (string, error) {
		return "digest", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "digest", got)
}

func TestDoTreatsSentinelTextWithoutWrappingAsPermanent(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	err := l.Do(t.Context(), nil, "ghcr.io", func() error {
		if attempts.Add(1) < 2 {
			return errors.New("registry rate limited: " + ErrRateLimited.Error())
		}

		return nil
	})
	// A plain string is not errors.Is(ErrRateLimited), so it must be permanent.
	require.Error(t, err)
	assert.Equal(t, int32(1), attempts.Load())
}

func TestDoRetriesWrappedErrRateLimited(t *testing.T) {
	l := New()

	var attempts atomic.Int32

	err := l.Do(t.Context(), nil, "ghcr.io", func() error {
		if attempts.Add(1) < 2 {
			return errors.Join(ErrRateLimited)
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
}

func TestDoValueConsumesOneTokenPerAttempt(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{
		Allowed:       2,
		AllowedWindow: 200 * time.Millisecond,
	})

	got, err := l.DoValue(t.Context(), testLog(), "ghcr.io", func() (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", got)

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	assert.GreaterOrEqual(t, time.Since(started), 80*time.Millisecond)
}

func TestDoValueHonorsHostWaitCancellation(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 5 * time.Second})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := l.DoValue(ctx, testLog(), "ghcr.io", func() (string, error) {
		return "unused", nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestLastOperationErrorPassesThroughPlainErrors(t *testing.T) {
	t.Parallel()

	plain := errors.New("not a retry error")
	assert.Equal(t, plain, lastOperationError(plain))
	assert.NoError(t, lastOperationError(nil))
}

func TestDoStopsWhenContextCanceled(t *testing.T) {
	l := New()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := l.Do(ctx, testLog(), "ghcr.io", func() error {
		return &Error{RetryAfter: minHonorWait}
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
