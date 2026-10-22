package actions

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// warnMessages returns the messages of the warn-level entries in buf.
//
// Parameters:
//   - buf: JSON log output from newCaptureLogger.
//
// Returns:
//   - []string: Messages logged at warn level, in order.
func warnMessages(buf *bytes.Buffer) []string {
	var messages []string

	for _, entry := range parseJSONLogEntries(buf) {
		if entry["level"] == "warn" {
			if message, ok := entry["message"].(string); ok {
				messages = append(messages, message)
			}
		}
	}

	return messages
}

// TestImageDiskSpaceGate verifies that an update warns about image disk usage
// at the warning threshold, blocks at the maximum, and fails closed when the
// usage cannot be read.
func TestImageDiskSpaceGate(t *testing.T) {
	t.Parallel()

	t.Run("skips the check when both thresholds are unset", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{}, false, false)

		_, _, err := update(
			newTestLogger(t),
			t.Context(),
			client,
			defaultTestUpdateParams(filters.NoFilter),
			ratelimit.New(),
		)
		require.NoError(t, err)
		assert.Equal(t, int32(0), client.TestData.GetImageDiskUsageCount.Load())
		assert.Positive(t, client.TestData.ListContainersCount.Load())
	})

	t.Run("continues when usage is below the warning threshold", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 1_000, TotalCount: 1},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		_, _, err := update(newTestLogger(t), t.Context(), client, params, ratelimit.New())
		require.NoError(t, err)
		assert.Equal(t, int32(1), client.TestData.GetImageDiskUsageCount.Load())
		assert.Positive(t, client.TestData.ListContainersCount.Load())
	})

	t.Run("warns and continues when usage reaches the warning threshold", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 8_000, TotalCount: 2},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		log, buf := newCaptureLogger()

		_, _, err := update(log, t.Context(), client, params, ratelimit.New())
		require.NoError(t, err)
		assert.Contains(t, warnMessages(buf), msgImageDiskSpaceWarn)
		assert.Positive(t, client.TestData.ListContainersCount.Load())
	})

	t.Run("allows warn-only mode without blocking", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{TotalSize: 50_000, TotalCount: 3},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceWarn = 1_000

		log, buf := newCaptureLogger()

		_, _, err := update(log, t.Context(), client, params, ratelimit.New())
		require.NoError(t, err)
		assert.Contains(t, warnMessages(buf), msgImageDiskSpaceWarn)
		assert.Positive(t, client.TestData.ListContainersCount.Load())
	})

	t.Run("blocks the session when usage reaches the maximum", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{
			ImageDiskUsage: types.ImageDiskUsage{
				TotalSize:   10_000,
				Reclaimable: 2_000,
				TotalCount:  4,
			},
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000
		params.DiskSpaceWarn = 8_000

		_, _, err := update(newTestLogger(t), t.Context(), client, params, ratelimit.New())
		require.ErrorIs(t, err, errImageDiskSpaceExceeded)
		assert.Equal(t, int32(0), client.TestData.ListContainersCount.Load())
	})

	t.Run("fails closed when image disk usage cannot be queried", func(t *testing.T) {
		t.Parallel()

		client := mockActions.CreateMockClient(&mockActions.TestData{
			GetImageDiskUsageError: errors.New("df unavailable"),
		}, false, false)

		params := defaultTestUpdateParams(filters.NoFilter)
		params.DiskSpaceMax = 10_000

		_, _, err := update(newTestLogger(t), t.Context(), client, params, ratelimit.New())
		require.ErrorIs(t, err, errImageDiskUsageFailed)
		assert.Equal(t, int32(0), client.TestData.ListContainersCount.Load())
	})
}
