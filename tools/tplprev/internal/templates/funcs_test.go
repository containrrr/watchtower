package templates

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{"usage": int64(0)}
	assert.True(t, hasKey(data, "usage"))
	assert.False(t, hasKey(data, "max"))
	assert.False(t, hasKey("not-a-map", "usage"))
	assert.False(t, hasKey(nil, "usage"))
}

func TestToJSON(t *testing.T) {
	t.Parallel()

	got := toJSON(map[string]any{"title": "Title"})
	assert.Contains(t, got, `"title": "Title"`)
}

func TestFormatDiskSpace(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "10 GB", formatDiskSpace(int64(10_000_000_000)))
	assert.Equal(t, "0 B", formatDiskSpace(int64(0)))
	assert.Equal(t, "unknown", formatDiskSpace(nil))
	assert.Equal(t, "unknown", formatDiskSpace(math.NaN()))
	assert.Equal(t, "unknown", formatDiskSpace(float64(1<<63)))
	assert.Equal(t, "-9223.37 PB", formatDiskSpace(int64(math.MinInt64)))
}

func TestFormatRFC1123(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want string
	}{
		{give: "2026-01-02T03:04:05Z", want: "Fri, 02 Jan 2026 03:04:05 UTC"},
		{give: "not-a-timestamp", want: "not-a-timestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatRFC1123(tt.give))
		})
	}
}
