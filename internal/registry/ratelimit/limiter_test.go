package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitRespectsHostCooldown(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 80 * time.Millisecond})

	started := time.Now()
	err := l.Wait(t.Context(), "ghcr.io")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 70*time.Millisecond)
}

func TestWaitDoesNotBlockOtherHosts(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: time.Hour})

	started := time.Now()
	err := l.Wait(t.Context(), "index.docker.io")
	require.NoError(t, err)
	assert.Less(t, time.Since(started), 50*time.Millisecond)
}

func TestWaitHonorsAllowedBudget(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{
		Allowed:       2,
		AllowedWindow: 200 * time.Millisecond,
	})

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))

	started := time.Now()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err := l.Wait(ctx, "ghcr.io")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 80*time.Millisecond)
}

func TestWaitCooldownRespectsHostCooldown(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 80 * time.Millisecond})

	started := time.Now()
	err := l.WaitCooldown(t.Context(), "ghcr.io")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(started), 70*time.Millisecond)
}

func TestWaitCooldownDoesNotConsumeQuotaToken(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{
		Allowed:       2,
		AllowedWindow: 200 * time.Millisecond,
	})

	require.NoError(t, l.WaitCooldown(t.Context(), "ghcr.io"))

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	assert.Less(t, time.Since(started), 50*time.Millisecond)
}

func TestWaitCooldownEmptyHostReturnsImmediately(t *testing.T) {
	l := New()

	started := time.Now()
	err := l.WaitCooldown(t.Context(), "")
	require.NoError(t, err)
	assert.Less(t, time.Since(started), 20*time.Millisecond)
}

func TestWaitCooldownCancelsWithContext(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 5 * time.Second})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	err := l.WaitCooldown(ctx, "ghcr.io")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitEmptyHostReturnsImmediately(t *testing.T) {
	l := New()

	started := time.Now()
	err := l.Wait(t.Context(), "")
	require.NoError(t, err)
	assert.Less(t, time.Since(started), 20*time.Millisecond)
}

func TestObserveIgnoresEmptyHostAndNilInfo(t *testing.T) {
	l := New()

	l.Observe("", &Error{RetryAfter: time.Hour})
	l.Observe("ghcr.io", nil)

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	assert.Less(t, time.Since(started), 20*time.Millisecond)
}

func TestObserveDoesNotShrinkExistingCooldown(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 80 * time.Millisecond})
	l.Observe("ghcr.io", &Error{RetryAfter: time.Millisecond})

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	assert.GreaterOrEqual(t, time.Since(started), 70*time.Millisecond)
}

func TestObserveSuccessIgnoresUnknownHost(t *testing.T) {
	l := New()

	l.observeSuccess("")
	l.observeSuccess("ghcr.io")

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
}

func TestObserveSuccessDoesNotGiftExtraToken(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{
		Allowed:       2,
		AllowedWindow: 200 * time.Millisecond,
	})

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	l.observeSuccess("ghcr.io")

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), "ghcr.io"))
	assert.GreaterOrEqual(t, time.Since(started), 80*time.Millisecond)
}

func TestRefillLocked(t *testing.T) {
	t.Parallel()

	t.Run("missing budget is a no-op", func(t *testing.T) {
		t.Parallel()

		state := &hostState{}
		state.refillLocked(time.Now())
		assert.InDelta(t, 0.0, state.tokens, 0.001)
	})

	t.Run("zero last refill starts at full budget", func(t *testing.T) {
		t.Parallel()

		state := &hostState{allowed: 5, window: time.Second}
		state.refillLocked(time.Now())
		assert.InDelta(t, 5.0, state.tokens, 0.001)
	})

	t.Run("future last refill does not change tokens", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		state := &hostState{
			allowed:    5,
			window:     time.Second,
			tokens:     2,
			lastRefill: now.Add(time.Second),
		}
		state.refillLocked(now)
		assert.InDelta(t, 2.0, state.tokens, 0.001)
	})

	t.Run("caps tokens at the advertised budget", func(t *testing.T) {
		t.Parallel()

		state := &hostState{
			allowed:    10,
			window:     time.Second,
			tokens:     9,
			lastRefill: time.Now().Add(-time.Hour),
		}
		state.refillLocked(time.Now())
		assert.InDelta(t, 10.0, state.tokens, 0.001)
	})

	t.Run("caps tokens at burst after a throttle", func(t *testing.T) {
		t.Parallel()

		state := &hostState{
			allowed:    44000,
			window:     time.Minute,
			burst:      1,
			tokens:     0,
			lastRefill: time.Now().Add(-time.Second),
		}
		state.refillLocked(time.Now())
		assert.InDelta(t, 1.0, state.tokens, 0.001)
	})
}

func TestWaitCancelsWithContext(t *testing.T) {
	l := New()

	l.Observe("ghcr.io", &Error{RetryAfter: 5 * time.Second})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	err := l.Wait(ctx, "ghcr.io")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitSerializesAfterQuotaThrottle(t *testing.T) {
	l := New()

	l.Observe("registry.example", &Error{
		RetryAfter:    521600 * time.Nanosecond,
		Allowed:       44000,
		AllowedWindow: time.Minute,
	})

	require.NoError(t, l.Wait(t.Context(), "registry.example"))

	started := time.Now()

	const waiters = 20

	var wg sync.WaitGroup

	for range waiters {
		wg.Go(func() {
			assert.NoError(t, l.Wait(t.Context(), "registry.example"))
		})
	}

	wg.Wait()

	elapsed := time.Since(started)
	t.Logf("20 concurrent l.Wait() after quota throttle completed in %s", elapsed)
	assert.GreaterOrEqual(t, elapsed, 15*time.Millisecond)
}

func TestWaitSerializesAfterRetryAfterOnlyThrottle(t *testing.T) {
	l := New()

	l.Observe("registry.example", &Error{RetryAfter: 80 * time.Millisecond})

	require.NoError(t, l.Wait(t.Context(), "registry.example"))

	started := time.Now()

	const waiters = 4

	var wg sync.WaitGroup

	for range waiters {
		wg.Go(func() {
			assert.NoError(t, l.Wait(t.Context(), "registry.example"))
		})
	}

	wg.Wait()

	elapsed := time.Since(started)
	t.Logf("4 concurrent l.Wait() after Retry-After-only throttle completed in %s", elapsed)
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond)
}

func TestWaitUnthrottledHostStaysImmediate(t *testing.T) {
	l := New()

	started := time.Now()

	const waiters = 20

	var wg sync.WaitGroup

	for range waiters {
		wg.Go(func() {
			assert.NoError(t, l.Wait(t.Context(), "registry.example"))
		})
	}

	wg.Wait()

	assert.Less(t, time.Since(started), 20*time.Millisecond)
}

func TestWaitDoesNotAccrueBurstDuringCooldown(t *testing.T) {
	l := New()

	l.Observe("registry.example", &Error{
		RetryAfter:    80 * time.Millisecond,
		Allowed:       44000,
		AllowedWindow: time.Minute,
	})

	require.NoError(t, l.Wait(t.Context(), "registry.example"))

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()

	granted := 0

	for {
		err := l.Wait(ctx, "registry.example")
		if err != nil {
			break
		}

		granted++
		if granted > 5 {
			break
		}
	}

	assert.LessOrEqual(t, granted, 1)
}

func TestScopeSeparatesAnonymousGHCR(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "ghcr.io|anon", Scope("ghcr.io", false))
	assert.Equal(t, "ghcr.io", Scope("ghcr.io", true))
	assert.Equal(t, "ghcr.io|anon", Scope("lscr.io", false))
	assert.Equal(t, "ghcr.io", Scope("lscr.io", true))
	assert.Equal(t, "index.docker.io", Scope("index.docker.io", false))
	assert.Empty(t, Scope("", false))
	assert.True(t, isAnonymousScope(Scope("ghcr.io", false)))
	assert.True(t, isAnonymousScope(Scope("lscr.io", false)))
	assert.False(t, isAnonymousScope(Scope("ghcr.io", true)))
	assert.False(t, isAnonymousScope(Scope("lscr.io", true)))
}

func TestAnonymousGHCRSerializes(t *testing.T) {
	l := New()

	key := Scope("ghcr.io", false)

	var (
		inFlight atomic.Int32
		peak     atomic.Int32
	)

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			require.NoError(t, l.acquireSerial(t.Context(), key))
			defer l.releaseSerial(key)

			n := inFlight.Add(1)

			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}

			time.Sleep(20 * time.Millisecond)
			inFlight.Add(-1)
		})
	}

	wg.Wait()
	assert.Equal(t, int32(1), peak.Load())
}

func TestAuthenticatedGHCRDoesNotTakeAnonLock(t *testing.T) {
	l := New()

	anonKey := Scope("ghcr.io", false)

	require.NoError(t, l.acquireSerial(t.Context(), anonKey))
	defer l.releaseSerial(anonKey)

	started := time.Now()
	authKey := Scope("ghcr.io", true)
	require.Equal(t, "ghcr.io", authKey)
	require.NoError(t, l.Wait(t.Context(), authKey))
	assert.Less(t, time.Since(started), 50*time.Millisecond)
}

// TestSerialSlotForConcurrent verifies that concurrent callers asking for the
// same key get one shared slot, so the slot serializes them.
func TestSerialSlotForConcurrent(t *testing.T) {
	l := New()

	slots := make(chan chan struct{}, 32)

	var wg sync.WaitGroup

	for range 32 {
		wg.Go(func() {
			slots <- l.serialSlotFor("serial-race")
		})
	}

	wg.Wait()
	close(slots)

	first := <-slots
	for slot := range slots {
		assert.Equal(t, first, slot)
	}
}

func TestObserveAnonDoesNotPaceAuth(t *testing.T) {
	l := New()

	l.Observe(Scope("ghcr.io", false), &Error{RetryAfter: time.Hour})

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), Scope("ghcr.io", true)))
	assert.Less(t, time.Since(started), 50*time.Millisecond)
}

// TestNewUsesProductionRetryBudgets verifies that a new limiter starts with
// the production retry budgets.
func TestNewUsesProductionRetryBudgets(t *testing.T) {
	t.Parallel()

	l := New()

	assert.Equal(t, maxRetryElapsed, l.retryElapsed)
	assert.Equal(t, maxBucketRetryElapsed, l.bucketRetryElapsed)
}
