package metrics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzGetHistory verifies that GetHistory never panics and correctly filters
// history entries by time range and limit.
func FuzzGetHistory(f *testing.F) {
	baseTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	f.Add(5, 10)
	f.Add(0, 0)
	f.Add(1, 1)
	f.Add(100, 0)
	f.Add(0, 100)

	f.Fuzz(func(t *testing.T, sinceOffsetMin, limit int) {
		m := &Metrics{}

		m.history = []HistoryEntry{
			{Timestamp: baseTime.Add(-2 * time.Hour), Scanned: 10, Updated: 5},
			{Timestamp: baseTime.Add(-1 * time.Hour), Scanned: 8, Updated: 3},
			{Timestamp: baseTime, Scanned: 12, Updated: 7},
			{Timestamp: baseTime.Add(1 * time.Hour), Scanned: 15, Updated: 9},
		}

		if limit < 0 {
			limit = 0
		}

		if sinceOffsetMin < 0 {
			sinceOffsetMin = 0
		}

		var since *time.Time

		if sinceOffsetMin > 0 {
			s := baseTime.Add(-time.Duration(sinceOffsetMin) * time.Minute)
			since = &s
		}

		result := m.GetHistory(since, nil, limit)

		assert.NotNil(t, result, "GetHistory should never return nil")

		if limit > 0 {
			assert.LessOrEqual(t, len(result), limit,
				"result length %d should not exceed limit %d", len(result), limit)
		}

		for _, entry := range result {
			if since != nil {
				assert.False(t, entry.Timestamp.Before(*since),
					"entry should not be before since time")
			}
		}
	})
}

// FuzzMetricJSON verifies that Metric struct can be safely round-tripped
// through encoding/json with arbitrary integer values.
func FuzzMetricJSON(f *testing.F) {
	f.Add(0, 0, 0, 0, 0)
	f.Add(100, 50, 1, 2, 97)
	f.Add(-1, -1, -1, -1, -1)

	f.Fuzz(func(t *testing.T, scanned, updated, failed, restarted, skipped int) {
		m := &Metric{
			Scanned:   scanned,
			Updated:   updated,
			Failed:    failed,
			Restarted: restarted,
			Skipped:   skipped,
		}

		data, err := json.Marshal(m)
		require.NoError(t, err)

		var unmarshaled Metric
		require.NoError(t, json.Unmarshal(data, &unmarshaled))

		assert.Equal(t, m.Scanned, unmarshaled.Scanned)
		assert.Equal(t, m.Updated, unmarshaled.Updated)
		assert.Equal(t, m.Failed, unmarshaled.Failed)
		assert.Equal(t, m.Restarted, unmarshaled.Restarted)
		assert.Equal(t, m.Skipped, unmarshaled.Skipped)
	})
}
