package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nicholas-fedor/watchtower/internal/registry/hosts"
)

// hostState holds the cooldown and token-bucket budget for one registry host.
type hostState struct {
	// cooldownUntil is when the next request to this host may proceed.
	cooldownUntil time.Time
	// allowed is the advertised fill rate numerator. Zero when unknown.
	allowed int
	// window is the period for allowed. Zero when unknown.
	window time.Duration
	// burst is the maximum tokens held. Zero uses the advertised allowance.
	burst int
	// tokens is the remaining budget in the current window.
	tokens float64
	// lastRefill is when tokens were last replenished.
	lastRefill time.Time
}

// Limiter paces registry requests per host and retries the ones a registry
// rate limits.
//
// It records each host's 429 cooldown and advertised quota, and serializes
// unauthenticated GHCR requests. Requests share a host's budget only when they
// go through the same Limiter, so a process uses one Limiter for all of its
// registry traffic.
type Limiter struct {
	// hostsMu guards hostStates.
	hostsMu sync.Mutex
	// hostStates holds the cooldown and token budget of each host.
	hostStates map[string]*hostState
	// serialMu guards serialSlots.
	serialMu sync.Mutex
	// serialSlots holds the one-at-a-time slot of each serialized limiter key.
	serialSlots map[string]chan struct{}
	// retryElapsed bounds how long one operation keeps honoring a usable Retry-After.
	retryElapsed time.Duration
	// bucketRetryElapsed bounds how long one operation keeps retrying token-bucket 429s.
	bucketRetryElapsed time.Duration
}

const (
	// anonSuffix marks limiter keys for unauthenticated GHCR traffic.
	anonSuffix = "|anon"
)

// New returns a Limiter with no recorded throttles and the production retry
// budgets.
//
// Returns:
//   - *Limiter: Limiter ready for use.
func New() *Limiter {
	return &Limiter{
		hostsMu:            sync.Mutex{},
		hostStates:         map[string]*hostState{},
		serialMu:           sync.Mutex{},
		serialSlots:        map[string]chan struct{}{},
		retryElapsed:       maxRetryElapsed,
		bucketRetryElapsed: maxBucketRetryElapsed,
	}
}

// Scope returns the limiter key for host.
//
// GHCR and lscr.io are the same registry. Both use ghcr.io so an authenticated
// lscr.io check does not keep a separate slot from GHCR. Unauthenticated
// traffic uses a distinct key so anonymous 429s cannot pace authenticated
// requests.
//
// Parameters:
//   - host: Registry host, such as ghcr.io. Empty values return empty.
//   - authenticated: True when the caller has registry credentials.
//
// Returns:
//   - string: Limiter key. Unauthenticated GHCR is "ghcr.io|anon".
func Scope(host string, authenticated bool) string {
	if host == "" {
		return ""
	}

	if hosts.IsGitHubRegistry(host) {
		if !authenticated {
			return hosts.GitHubRegistryDomain + anonSuffix
		}

		return hosts.GitHubRegistryDomain
	}

	return host
}

// isAnonymousScope reports whether key is the unauthenticated GHCR limiter key.
//
// Parameters:
//   - key: Limiter key from [Scope].
//
// Returns:
//   - bool: True when key is the anonymous GHCR scope.
func isAnonymousScope(key string) bool {
	return strings.HasSuffix(key, anonSuffix)
}

// HoldAnonymous returns the limiter key for host and, for unauthenticated GHCR,
// acquires the one-at-a-time serial slot with acquireSerial.
//
// The release function is always non-nil. Callers should defer it. Authenticated
// GHCR and other registries do not take the serial slot.
//
// Parameters:
//   - ctx: Context that can cancel the serial wait.
//   - host: Canonical registry host. Empty hosts skip the hold.
//   - authenticated: True when the caller has registry credentials.
//
// Returns:
//   - string: Limiter key from [Scope].
//   - func(): Release callback. A no-op when no slot was acquired.
//   - error: [context.Context.Err] when canceled while waiting for the serial slot.
func (l *Limiter) HoldAnonymous(ctx context.Context, host string, authenticated bool) (string, func(), error) {
	key := Scope(host, authenticated)
	release := func() {}

	if !isAnonymousScope(key) {
		return key, release, nil
	}

	err := l.acquireSerial(ctx, key)
	if err != nil {
		return key, release, err
	}

	return key, func() { l.releaseSerial(key) }, nil
}

// Observe records a 429 against host so later Wait calls honor it.
//
// Advertised "allowed: N/window" values are treated as a fill rate, not a
// burst size. After a throttle, the host is limited to one outstanding token
// so concurrent waiters cannot dump the advertised budget at once.
// Cooldown time does not accrue tokens.
//
// Parameters:
//   - host: Registry host. Empty values are ignored.
//   - info: Parsed 429. Nil values are ignored.
func (l *Limiter) Observe(host string, info *Error) {
	if host == "" || info == nil {
		return
	}

	l.hostsMu.Lock()
	defer l.hostsMu.Unlock()

	state := l.hostLocked(host)
	now := time.Now()

	cooldown := min(info.RetryAfter, maxHonorWait)

	if cooldown > 0 && cooldown < minHonorWait {
		cooldown = minHonorWait
	}

	if cooldown > 0 {
		until := now.Add(cooldown)
		if until.After(state.cooldownUntil) {
			state.cooldownUntil = until
		}
	}

	state.burst = 1
	state.tokens = 1

	if state.cooldownUntil.After(now) {
		state.lastRefill = state.cooldownUntil
	} else {
		state.lastRefill = now
	}

	if info.Allowed > 0 && info.AllowedWindow > 0 {
		state.allowed = info.Allowed
		state.window = info.AllowedWindow
	} else if state.allowed <= 0 || state.window <= 0 {
		window := cooldown
		if window <= 0 {
			window = minHonorWait
		}

		state.allowed = 1
		state.window = window
	}
}

// Wait blocks until host is outside its 429 cooldown and has budget.
//
// Parameters:
//   - ctx: Context that can cancel the wait.
//   - host: Registry host. Empty hosts return immediately.
//
// Returns:
//   - error: ctx.Err() when canceled. Nil when the caller may proceed.
func (l *Limiter) Wait(ctx context.Context, host string) error {
	if host == "" {
		return nil
	}

	for {
		wait := l.nextWait(host)
		if wait <= 0 {
			return nil
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("registry rate-limit wait canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// WaitCooldown blocks until host is outside its 429 cooldown.
//
// Unlike Wait, this does not reserve a quota token. Callers that already
// passed Wait use it to recheck cooldown after a lock or slot handoff.
//
// Parameters:
//   - ctx: Context that can cancel the wait.
//   - host: Registry host. Empty hosts return immediately.
//
// Returns:
//   - error: ctx.Err() when canceled. Nil when the caller may proceed.
func (l *Limiter) WaitCooldown(ctx context.Context, host string) error {
	if host == "" {
		return nil
	}

	for {
		wait := l.nextCooldownWait(host)
		if wait <= 0 {
			return nil
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("registry rate-limit wait canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

// acquireSerial blocks until no other caller of this limiter runs an operation on key.
//
// Empty keys return immediately. releaseSerial must be called for the same key.
//
// Parameters:
//   - ctx: Context that can cancel the wait.
//   - key: Serial slot key. Empty values return immediately.
//
// Returns:
//   - error: [context.Context.Err] when canceled. Nil when the caller may proceed.
func (l *Limiter) acquireSerial(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}

	ctxErr := ctx.Err()
	if ctxErr != nil {
		return fmt.Errorf("registry serial wait canceled: %w", ctxErr)
	}

	select {
	case l.serialSlotFor(key) <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("registry serial wait canceled: %w", ctx.Err())
	}
}

// releaseSerial returns a serial slot acquired with acquireSerial.
//
// Empty keys and slots that are not held are ignored.
//
// Parameters:
//   - key: Serial slot key passed to acquireSerial.
func (l *Limiter) releaseSerial(key string) {
	if key == "" {
		return
	}

	select {
	case <-l.serialSlotFor(key):
	default:
	}
}

// serialSlotFor returns the capacity-1 slot for key, creating it when missing.
//
// Parameters:
//   - key: Serial slot key.
//
// Returns:
//   - chan struct{}: Buffered channel used as a mutex. Capacity is 1.
func (l *Limiter) serialSlotFor(key string) chan struct{} {
	l.serialMu.Lock()
	defer l.serialMu.Unlock()

	slot := l.serialSlots[key]
	if slot == nil {
		slot = make(chan struct{}, 1)
		l.serialSlots[key] = slot
	}

	return slot
}

// observeSuccess refreshes host's fill-rate tokens after a successful request.
//
// Token reservation happens in Wait. This only refills up to the burst cap so
// a success does not spend a second token or restore a pre-throttle dump.
//
// Parameters:
//   - host: Registry host. Empty values are ignored.
func (l *Limiter) observeSuccess(host string) {
	if host == "" {
		return
	}

	l.hostsMu.Lock()
	defer l.hostsMu.Unlock()

	state := l.hostStates[host]
	if state == nil || state.allowed <= 0 || state.window <= 0 {
		return
	}

	state.refillLocked(time.Now())
}

// nextWait computes how long the caller must wait before contacting host.
//
// Parameters:
//   - host: Registry host whose cooldown and quota should be inspected.
//
// Returns:
//   - time.Duration: Sleep before the next request. Zero when the caller may proceed.
func (l *Limiter) nextWait(host string) time.Duration {
	l.hostsMu.Lock()
	defer l.hostsMu.Unlock()

	state := l.hostLocked(host)
	now := time.Now()

	if now.Before(state.cooldownUntil) {
		return time.Until(state.cooldownUntil)
	}

	if state.allowed <= 0 || state.window <= 0 {
		return 0
	}

	state.refillLocked(now)

	if state.tokens >= 1 {
		state.tokens--

		return 0
	}

	perToken := max(state.window/time.Duration(state.allowed), time.Millisecond)

	return perToken
}

// nextCooldownWait returns remaining 429 cooldown for host without taking a token.
//
// Parameters:
//   - host: Registry host whose cooldown should be inspected.
//
// Returns:
//   - time.Duration: Sleep before the next request. Zero when there is no cooldown.
func (l *Limiter) nextCooldownWait(host string) time.Duration {
	l.hostsMu.Lock()
	defer l.hostsMu.Unlock()

	state := l.hostStates[host]
	if state == nil || !time.Now().Before(state.cooldownUntil) {
		return 0
	}

	return time.Until(state.cooldownUntil)
}

// hostLocked returns the per-host limiter state, creating it when missing.
//
// The caller must hold hostsMu.
//
// Parameters:
//   - host: Registry host key.
//
// Returns:
//   - *hostState: Existing or newly created state for host.
func (l *Limiter) hostLocked(host string) *hostState {
	state := l.hostStates[host]
	if state == nil {
		state = &hostState{lastRefill: time.Now()}
		l.hostStates[host] = state
	}

	return state
}

// refillLocked adds tokens earned since the last refill.
//
// The caller must hold hostsMu.
//
// Parameters:
//   - now: Current time used to compute earned tokens.
func (s *hostState) refillLocked(now time.Time) {
	if s.allowed <= 0 || s.window <= 0 {
		return
	}

	if s.lastRefill.IsZero() {
		s.lastRefill = now
		s.tokens = s.tokenCap()

		return
	}

	elapsed := now.Sub(s.lastRefill)
	if elapsed <= 0 {
		return
	}

	s.tokens += float64(elapsed) / float64(s.window) * float64(s.allowed)
	if ceiling := s.tokenCap(); s.tokens > ceiling {
		s.tokens = ceiling
	}

	s.lastRefill = now
}

// tokenCap is the maximum tokens this host may hold.
//
// After a throttle, burst is 1 so an advertised fill rate cannot rebuild a
// large dump of tokens. Before any throttle, burst is 0 and the cap is the
// advertised budget when a quota is present.
//
// Returns:
//   - float64: Token ceiling used by refillLocked.
func (s *hostState) tokenCap() float64 {
	if s.burst > 0 {
		return float64(s.burst)
	}

	return float64(s.allowed)
}
