package ratelimit

import (
	"errors"
	"fmt"
	"time"
)

// ErrRateLimited indicates a registry rejected a request for exceeding its rate limit.
var ErrRateLimited = errors.New("registry rate limited")

const (
	// minHonorWait is the floor applied to tiny Retry-After values such as 331µs.
	minHonorWait = 100 * time.Millisecond
	// maxHonorWait is the longest Retry-After this process will sleep in one update cycle.
	maxHonorWait = 30 * time.Second
	// maxRetryElapsed is the production in-cycle bound for honoring a usable
	// Retry-After, counted from the first throttle.
	maxRetryElapsed = 30 * time.Second
	// maxBucketRetryElapsed is the production in-cycle bound for retrying
	// token-bucket 429s, counted from the first throttle. Shared GHCR org buckets
	// stay empty for up to two minutes past the hour, and one daemon pull attempt
	// can take most of a minute to report a 429.
	maxBucketRetryElapsed = 3 * time.Minute
	// DefaultBodyLimit is how many response bytes we read when parsing a 429.
	DefaultBodyLimit = 4096
	// retryAfterCaptureCount is the expected regex group count for retry-after.
	retryAfterCaptureCount = 2
	// allowedCaptureCount is the expected regex group count for allowed quotas.
	allowedCaptureCount = 3
	// equalJitterDivisor splits a wait into the equal-jitter half range.
	equalJitterDivisor = 2
)

// Retry budgets. Tests shorten them so budget exhaustion does not sleep for real.
var (
	// retryElapsed bounds how long one operation keeps honoring a usable Retry-After.
	retryElapsed = maxRetryElapsed
	// bucketRetryElapsed bounds how long one operation keeps retrying token-bucket 429s.
	bucketRetryElapsed = maxBucketRetryElapsed
)

// Error is a registry 429 with the wait and quota the registry advertised.
type Error struct {
	// StatusCode is the HTTP status when the limit came from an HTTP response.
	StatusCode int
	// RetryAfter is the wait parsed from Retry-After or the response body.
	RetryAfter time.Duration
	// Allowed is the advertised request budget. Zero when the registry omitted it.
	Allowed int
	// AllowedWindow is the period for Allowed. Zero when the registry omitted it.
	AllowedWindow time.Duration
	// Host is the registry host that produced the limit when known.
	Host string
	// Message is the raw registry or Docker-stream text.
	Message string
}

// Error describes the rate limit for logs and error wrapping.
//
// Returns:
//   - string: Human-readable rate-limit error.
func (e *Error) Error() string {
	if e == nil {
		return ErrRateLimited.Error()
	}

	if e.Allowed > 0 && e.AllowedWindow > 0 {
		return fmt.Sprintf(
			"%s: retry-after %s allowed %d per %s",
			ErrRateLimited.Error(),
			e.RetryAfter,
			e.Allowed,
			e.AllowedWindow,
		)
	}

	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s: retry-after %s", ErrRateLimited.Error(), e.RetryAfter)
	}

	if e.Message != "" {
		return fmt.Sprintf("%s: %s", ErrRateLimited.Error(), e.Message)
	}

	return ErrRateLimited.Error()
}

// Unwrap exposes ErrRateLimited so errors.Is matches.
//
// Returns:
//   - error: The sentinel rate-limit error.
func (e *Error) Unwrap() error {
	return ErrRateLimited
}

// Is reports whether err is a registry rate-limit error.
//
// Parameters:
//   - err: Error to inspect. May be wrapped.
//
// Returns:
//   - bool: True when err unwraps to ErrRateLimited.
func Is(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// decision converts a parsed 429 into a sleep or a give-up.
//
// Tiny Retry-After values are raised to minHonorWait so a 331µs GHCR token
// does not cause an immediate retry. Waits longer than maxHonorWait are not
// slept in-process. The next scheduled Watchtower run can try again.
//
// Parameters:
//   - info: Parsed rate-limit details. Nil uses the minimum wait.
//
// Returns:
//   - time.Duration: How long to wait before the next attempt.
//   - bool: True when the caller should stop retrying this cycle.
func decision(info *Error) (time.Duration, bool) {
	wait := minHonorWait
	if info != nil && info.RetryAfter > 0 {
		wait = info.RetryAfter
	}

	if wait > maxHonorWait {
		return 0, true
	}

	if wait < minHonorWait {
		wait = minHonorWait
	}

	return wait, false
}

// isTokenBucket reports whether info is a bucket refill signal rather than a
// backoff instruction.
//
// A Retry-After below minHonorWait, or none at all, carries no usable wait. The
// registry is saying its shared bucket is empty right now, not how long to stay
// away, so the caller chooses its own backoff.
//
// Parameters:
//   - info: Parsed rate-limit details. Nil counts as a token bucket.
//
// Returns:
//   - bool: True when the Retry-After is absent or below the floor.
func isTokenBucket(info *Error) bool {
	return info == nil || info.RetryAfter < minHonorWait
}
