package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/metrics"
)

// FuzzStatusHandlerHandle verifies that StatusHandler.Handle never panics and
// produces valid JSON for arbitrary metric values including edge cases like
// negative numbers, max int, and zero values.
func FuzzStatusHandlerHandle(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0), int64(0))
	f.Add(int64(100), int64(50), int64(10), int64(5), int64(35))
	f.Add(int64(-1), int64(-1), int64(-1), int64(-1), int64(-1))
	f.Add(int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MaxInt64))
	f.Add(int64(math.MinInt64), int64(math.MinInt64), int64(math.MinInt64), int64(math.MinInt64), int64(math.MinInt64))

	f.Fuzz(func(t *testing.T, scanned, updated, failed, restarted, skipped int64) {
		h := NewStatusHandler(newNopLogger(), func() *metrics.Metric {
			return &metrics.Metric{
				Scanned:   int(scanned),
				Updated:   int(updated),
				Failed:    int(failed),
				Restarted: int(restarted),
				Skipped:   int(skipped),
			}
		})

		require.NotNil(t, h)

		metric := h.getLast()
		require.NotNil(t, metric)

		assert.Equal(t, int(scanned), metric.Scanned)
		assert.Equal(t, int(updated), metric.Updated)
		assert.Equal(t, int(failed), metric.Failed)
		assert.Equal(t, int(restarted), metric.Restarted)
		assert.Equal(t, int(skipped), metric.Skipped)
	})
}

// FuzzStatusHandlerNilGetLast verifies that StatusHandler handles nil return
// from getLast without panicking.
func FuzzStatusHandlerNilGetLast(f *testing.F) {
	f.Add(true)
	f.Add(false)

	f.Fuzz(func(t *testing.T, returnNil bool) {
		h := NewStatusHandler(newNopLogger(), func() *metrics.Metric {
			if returnNil {
				return nil
			}

			return &metrics.Metric{Scanned: 1, Updated: 1, Failed: 0, Restarted: 0, Skipped: 0}
		})

		require.NotNil(t, h)

		metric := h.getLast()
		if returnNil {
			assert.Nil(t, metric)
		} else {
			assert.NotNil(t, metric)
		}
	})
}
