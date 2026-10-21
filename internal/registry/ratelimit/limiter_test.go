package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
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

// recoveryHost is the registry host the recovery tests throttle.
const recoveryHost = "registry.example"

// waitTime returns how long l.Wait blocks for recoveryHost on the synctest
// clock.
//
// Parameters:
//   - t: Test inside a synctest bubble.
//   - l: Limiter to wait on.
//
// Returns:
//   - time.Duration: Time the wait took.
func waitTime(t *testing.T, l *Limiter) time.Duration {
	t.Helper()

	started := time.Now()

	require.NoError(t, l.Wait(t.Context(), recoveryHost))

	return time.Since(started)
}

// TestObserve_RecoveryEndsPacing verifies that the pacing a 429 starts lasts
// until throttleRecovery after the cooldown ends, then stops, and that a quota
// made up from the cooldown goes with it.
func TestObserve_RecoveryEndsPacing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		l := New()

		l.Observe(recoveryHost, &Error{RetryAfter: time.Second})
		assert.Equal(t, time.Second, waitTime(t, l), "the cooldown holds the first request")

		time.Sleep(throttleRecovery - 2*time.Second)
		assert.Zero(t, waitTime(t, l), "one token is available during recovery")
		assert.Equal(t, time.Second, waitTime(t, l), "the next request waits for the made-up quota")

		time.Sleep(time.Second)

		for range 20 {
			assert.Zero(t, waitTime(t, l), "requests are not paced after recovery")
		}

		state := l.hostStates[recoveryHost]
		assert.Zero(t, state.allowed)
		assert.Zero(t, state.burst)
		assert.False(t, state.synthesized)
		assert.True(t, state.recoverAt.IsZero())
	})
}

// TestObserve_RecoveryKeepsAdvertisedQuota verifies that recovery lifts the
// one-token cap but keeps a quota the registry advertised, so the host refills
// to its full budget and is then paced at the advertised rate.
func TestObserve_RecoveryKeepsAdvertisedQuota(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		l := New()

		l.Observe(recoveryHost, &Error{Allowed: 2, AllowedWindow: time.Minute})

		time.Sleep(throttleRecovery)

		assert.Zero(t, waitTime(t, l))
		assert.Zero(t, waitTime(t, l), "the full advertised budget is available")
		assert.Equal(t, 30*time.Second, waitTime(t, l), "the advertised rate paces the rest")

		state := l.hostStates[recoveryHost]
		assert.Equal(t, 2, state.allowed)
		assert.Equal(t, time.Minute, state.window)
		assert.Zero(t, state.burst)
	})
}

// TestObserve_ThrottleDuringRecoveryExtendsIt verifies that a 429 while the
// host is recovering moves the end of recovery later.
func TestObserve_ThrottleDuringRecoveryExtendsIt(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		l := New()

		l.Observe(recoveryHost, &Error{RetryAfter: time.Second})
		time.Sleep(throttleRecovery / 2)
		l.Observe(recoveryHost, &Error{RetryAfter: time.Second})

		// Past the first throttle's recovery, within the second's.
		time.Sleep(throttleRecovery/2 + 10*time.Second)

		assert.Zero(t, waitTime(t, l))
		assert.Equal(t, time.Second, waitTime(t, l), "the host is still paced")
	})
}

// TestObserve_ThrottleDuringRecoveryUpdatesMadeUpQuota verifies that a 429
// while the host is recovering paces it by that 429's cooldown when the quota
// was made up, and keeps an advertised quota.
func TestObserve_ThrottleDuringRecoveryUpdatesMadeUpQuota(t *testing.T) {
	t.Parallel()

	t.Run("made-up quota follows the latest cooldown", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			l := New()

			l.Observe(recoveryHost, &Error{RetryAfter: time.Second})
			l.Observe(recoveryHost, &Error{RetryAfter: 5 * time.Second})

			assert.Equal(t, 5*time.Second, waitTime(t, l), "the longer cooldown holds the next request")
			assert.Equal(t, 5*time.Second, waitTime(t, l), "the latest cooldown paces the rest")
		})
	})

	t.Run("advertised quota is kept", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			l := New()

			l.Observe(recoveryHost, &Error{Allowed: 2, AllowedWindow: time.Minute})
			l.Observe(recoveryHost, &Error{RetryAfter: 5 * time.Second})

			assert.Equal(t, 5*time.Second, waitTime(t, l), "the cooldown holds the next request")
			assert.Equal(t, 30*time.Second, waitTime(t, l), "the advertised rate paces the rest")
		})
	})
}

// TestObserve_ThrottleAfterRecoveryPacesAgain verifies that a 429 after the host
// recovered starts pacing again with a quota made up from the new cooldown.
func TestObserve_ThrottleAfterRecoveryPacesAgain(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		l := New()

		l.Observe(recoveryHost, &Error{RetryAfter: time.Second})
		time.Sleep(time.Second + throttleRecovery)
		assert.Zero(t, waitTime(t, l), "the host recovered")

		l.Observe(recoveryHost, &Error{RetryAfter: 2 * time.Second})

		assert.Equal(t, 2*time.Second, waitTime(t, l), "the new cooldown holds the next request")
		assert.Equal(t, 2*time.Second, waitTime(t, l), "the new made-up quota paces the rest")
	})
}
