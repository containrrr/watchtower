package scheduling_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/scheduling"
)

// TestUpdateLock_TryLock verifies that TryLock admits one holder at a time and
// admits the next one after Unlock.
func TestUpdateLock_TryLock(t *testing.T) {
	t.Parallel()

	lock := scheduling.NewUpdateLock()

	require.True(t, lock.TryLock())
	assert.False(t, lock.TryLock(), "a second holder took a held lock")

	lock.Unlock()

	assert.True(t, lock.TryLock(), "the lock stayed held after Unlock")
}

// TestUpdateLock_Lock verifies that Lock waits for the holder to unlock, and
// that a context ending first stops the wait without taking the lock.
func TestUpdateLock_Lock(t *testing.T) {
	t.Parallel()

	t.Run("waits for the holder", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()
			require.True(t, lock.TryLock())

			taken := make(chan error, 1)

			go func() { taken <- lock.Lock(t.Context()) }()

			synctest.Wait()

			select {
			case err := <-taken:
				t.Fatalf("Lock returned while the lock was held: %v", err)
			default:
			}

			lock.Unlock()

			require.NoError(t, <-taken)
			assert.False(t, lock.TryLock(), "Lock did not take the lock")
		})
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			lock := scheduling.NewUpdateLock()
			require.True(t, lock.TryLock())

			ctx, cancel := context.WithCancel(t.Context())
			taken := make(chan error, 1)

			go func() { taken <- lock.Lock(ctx) }()

			synctest.Wait()
			cancel()

			require.ErrorIs(t, <-taken, context.Canceled)

			lock.Unlock()

			assert.True(t, lock.TryLock(), "a canceled Lock left the lock held")
		})
	})
}

// TestUpdateLock_UnlockFree verifies that releasing a lock nobody holds
// panics instead of admitting a second update.
func TestUpdateLock_UnlockFree(t *testing.T) {
	t.Parallel()

	lock := scheduling.NewUpdateLock()

	assert.Panics(t, lock.Unlock)
}
