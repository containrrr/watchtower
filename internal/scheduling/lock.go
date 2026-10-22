package scheduling

import (
	"context"
	"fmt"

	"golang.org/x/sync/semaphore"
)

// UpdateLock serializes update sessions, so a scheduled run and an update
// requested through the HTTP API never run at the same time.
//
// The zero value is not usable. Construct one with NewUpdateLock and share
// that one lock with every caller that starts updates.
type UpdateLock struct {
	// sem admits one holder at a time.
	sem *semaphore.Weighted
}

// NewUpdateLock returns an unlocked UpdateLock.
//
// Returns:
//   - *UpdateLock: Lock ready for use.
func NewUpdateLock() *UpdateLock {
	return &UpdateLock{sem: semaphore.NewWeighted(1)}
}

// TryLock takes the lock if no update holds it.
//
// Returns:
//   - bool: True when the lock was taken, false when another update holds it.
func (l *UpdateLock) TryLock() bool {
	return l.sem.TryAcquire(1)
}

// Lock waits until the lock is free and takes it.
//
// It fails when ctx is done before the lock is taken, even if the lock is
// free, and leaves the lock unchanged.
//
// Parameters:
//   - ctx: Context bounding the wait.
//
// Returns:
//   - error: The context error when ctx is done first, nil once the lock is taken.
func (l *UpdateLock) Lock(ctx context.Context) error {
	err := l.sem.Acquire(ctx, 1)
	if err != nil {
		return fmt.Errorf("wait for the update lock: %w", err)
	}

	return nil
}

// Unlock releases the lock.
//
// It panics when the lock is not held, because releasing a free lock would
// let two updates run at once.
func (l *UpdateLock) Unlock() {
	l.sem.Release(1)
}
