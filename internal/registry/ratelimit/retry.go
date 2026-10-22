package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/rs/zerolog"
)

// Do retries operation when the registry returns a 429 that is worth retrying.
//
// Permanent errors are not retried. A Retry-After longer than the honor window
// stops retries so the next Watchtower cycle can try again. A usable Retry-After
// is honored as sent for up to the honor window. A token-bucket 429, whose
// Retry-After is below the floor, is retried with exponential backoff from the
// floor toward the honor window for up to the bucket budget. Both budgets are
// counted from the first throttle, so time spent queued on a pull slot or inside
// a long attempt before that throttle does not consume them.
//
// Parameters:
//   - ctx: Context that bounds the retry loop.
//   - log: Logger for retry notices. May be nil.
//   - host: Registry host used for shared cooldown and quota.
//   - operation: Function to run. It should return an [Error] on 429.
//
// Returns:
//   - error: The last operation error, or [context.Context.Err] when canceled.
func (l *Limiter) Do(ctx context.Context, log *zerolog.Logger, host string, operation func() error) error {
	_, err := l.DoValue(ctx, log, host, func() (struct{}, error) {
		return struct{}{}, operation()
	})

	return err
}

// DoValue is [Limiter.Do] with a successful result.
//
// Parameters:
//   - ctx: Context that bounds the retry loop.
//   - log: Logger for retry notices. May be nil.
//   - host: Registry host used for shared cooldown and quota.
//   - operation: Function to run.
//
// Returns:
//   - T: Value from a successful attempt.
//   - error: The last operation error, or [context.Context.Err] when canceled.
func (l *Limiter) DoValue[T any](
	ctx context.Context,
	log *zerolog.Logger,
	host string,
	operation func() (T, error),
) (T, error) {
	var zero T

	ctxErr := ctx.Err()
	if ctxErr != nil {
		return zero, fmt.Errorf("rate-limit retry canceled: %w", ctxErr)
	}

	policy := newBucketPolicy()

	var firstLimitedAt time.Time

	attempt := 0
	lastHonoredWait := time.Duration(0)
	lastRawRetryAfter := time.Duration(0)

	result, err := backoff.Retry(ctx, func() (T, error) {
		waitErr := l.Wait(ctx, host)
		if waitErr != nil {
			return zero, backoff.Permanent(waitErr)
		}

		value, opErr := operation()
		if opErr == nil {
			l.observeSuccess(host)

			return value, nil
		}

		info, ok := errors.AsType[*Error](opErr)
		if !ok && !Is(opErr) {
			return zero, backoff.Permanent(opErr)
		}

		if ok {
			if info.Host == "" {
				info.Host = host
			}

			l.Observe(host, info)
			lastRawRetryAfter = info.RetryAfter
		}

		now := time.Now()
		if firstLimitedAt.IsZero() {
			firstLimitedAt = now
		}

		wait, giveUp := decision(info)
		bucket := isTokenBucket(info)
		budget := l.retryElapsed

		if bucket {
			wait = bucketWait(policy)
			budget = l.bucketRetryElapsed
		} else {
			wait = jitterWait(wait)
		}

		attempt++
		lastHonoredWait = wait

		if giveUp || now.Sub(firstLimitedAt)+wait > budget {
			return zero, backoff.Permanent(opErr)
		}

		if log != nil {
			log.Debug().
				Str("host", host).
				Bool("token_bucket", bucket).
				Dur("retry_after", wait).
				Msg("Registry rate limited. Retrying after delay")
		}

		return zero, backoff.RetryAfter(wait, opErr)
	},
		// The budgets are enforced inside the closure from the first throttle, so
		// the library's elapsed limit is off. RetryAfter resets the policy the
		// library holds on every retry, which is why the bucket policy lives
		// outside it.
		backoff.WithMaxTries(l.maxRetryAttempts()),
		backoff.WithMaxElapsedTime(0),
	)
	if err == nil {
		return result, nil
	}

	err = lastOperationError(err)
	if log != nil && Is(err) {
		log.WithLevel(exhaustionLevel(err)).
			Err(err).
			Str("host", host).
			Int("attempts", attempt).
			Dur("honored_wait", lastHonoredWait).
			Dur("retry_after", lastRawRetryAfter).
			Msg("Registry rate limited. Stopping retries")
	}

	return result, err
}

// ExceedsHonorWindow reports whether err is a rate limit whose Retry-After is
// longer than the honor window.
//
// [Limiter.Do] and [Limiter.DoValue] stop at once on such a limit and log it at warn, so
// callers use this to avoid warning a second time about the same failure.
//
// Parameters:
//   - err: Error to inspect. May be wrapped.
//
// Returns:
//   - bool: True when err carries a Retry-After beyond the honor window.
func ExceedsHonorWindow(err error) bool {
	info, ok := errors.AsType[*Error](err)

	return ok && info != nil && info.RetryAfter > maxHonorWait
}

// exhaustionLevel chooses the log level when in-cycle 429 retries stop.
//
// A Retry-After longer than the honor window is a real registry backoff and
// is logged at warn. Waits that exhaust a retry budget stay at debug, because
// the caller reports the failed container once instead.
//
// Parameters:
//   - err: Last rate-limit error from the retry loop.
//
// Returns:
//   - zerolog.Level: Warn for long Retry-After, debug otherwise.
func exhaustionLevel(err error) zerolog.Level {
	if ExceedsHonorWindow(err) {
		return zerolog.WarnLevel
	}

	return zerolog.DebugLevel
}

// maxRetryAttempts is a circuit breaker derived from the larger retry budget.
//
// Retries run until their budget elapses, not a fixed handful of tries. This
// cap only stops a zero-wait loop from spinning.
//
// Returns:
//   - uint: Maximum attempts passed to [backoff.WithMaxTries].
func (l *Limiter) maxRetryAttempts() uint {
	n := max(l.retryElapsed, l.bucketRetryElapsed)/minHonorWait + 1
	if n < 1 {
		return 1
	}

	return uint(n)
}

// lastOperationError unwraps a cenkalti RetryError to the last operation error.
//
// Context cancellation on the retry loop is preferred over the last 429 so
// callers can distinguish a canceled wait from a rate limit.
//
// Parameters:
//   - err: Error returned by backoff.Retry. May be a *backoff.RetryError.
//
// Returns:
//   - error: Last operation error, the context cause, or err unchanged.
func lastOperationError(err error) error {
	retryErr := backoff.AsRetryError(err)
	if retryErr == nil {
		return err
	}

	if retryErr.Cause != nil &&
		(errors.Is(retryErr.Cause, context.Canceled) || errors.Is(retryErr.Cause, context.DeadlineExceeded)) {
		return retryErr.Cause
	}

	if retryErr.LastErr != nil {
		return retryErr.LastErr
	}

	return err
}

// jitterWait adds nonnegative jitter so concurrent retries do not wake together.
//
// The decision wait is the minimum. Extra delay is in
// [0, wait/equalJitterDivisor], capped so the result does not exceed
// maxHonorWait. Zero and negative waits are unchanged.
//
// Parameters:
//   - wait: decision wait before the next attempt.
//
// Returns:
//   - time.Duration: Jittered wait, never shorter than wait when wait > 0.
func jitterWait(wait time.Duration) time.Duration {
	if wait <= 0 {
		return wait
	}

	room := maxHonorWait - wait
	if room <= 0 {
		return wait
	}

	spread := min(wait/equalJitterDivisor, room)

	//nolint:gosec // Jitter only needs to desynchronize retries, not be cryptographic.
	return wait + time.Duration(rand.Int64N(int64(spread)+1))
}

// newBucketPolicy returns the exponential policy that paces token-bucket 429s.
//
// Growth starts at minHonorWait and is capped at maxHonorWait. The policy does
// not randomize, because jitterWait adds equal jitter and keeps the floor and
// the cap. It is private to one DoValue call: backoff.RetryAfter resets the
// policy the library holds, so growth has to live on one the library never sees.
//
// Returns:
//   - *backoff.ExponentialBackOff: Reset policy ready for NextBackOff.
func newBucketPolicy() *backoff.ExponentialBackOff {
	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = minHonorWait
	policy.MaxInterval = maxHonorWait
	policy.RandomizationFactor = 0
	policy.Reset()

	return policy
}

// bucketWait returns the next jittered token-bucket wait from policy.
//
// Equal jitter on the default 1.5 multiplier keeps the sequence monotonic: one
// step's largest value is the next step's smallest.
//
// Parameters:
//   - policy: Policy from newBucketPolicy. Each call advances it.
//
// Returns:
//   - time.Duration: Wait in [minHonorWait, maxHonorWait], never shorter than the previous call.
func bucketWait(policy *backoff.ExponentialBackOff) time.Duration {
	return jitterWait(policy.NextBackOff())
}
