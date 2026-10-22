package history

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// FuzzParseTimeParam verifies that parseTimeParam never panics and returns
// either a valid time pointer or an error for any input string.
func FuzzParseTimeParam(f *testing.F) {
	f.Add("2024-01-01T00:00:00Z")
	f.Add("2024-06-15T12:30:45+02:00")
	f.Add("")
	f.Add("not-a-date")
	f.Add("2024-13-01T00:00:00Z")
	f.Add("2024-01-32T00:00:00Z")
	f.Add("\x00")
	f.Add(strings.Repeat("9", 1000))

	f.Fuzz(func(t *testing.T, s string) {
		result, err := parseTimeParam(s)

		if s == "" {
			assert.Nil(t, result, "empty input should return nil time")
			assert.ErrorIs(t, err, errNoTimeParameter)

			return
		}

		if err != nil {
			assert.Nil(t, result, "error should return nil time")
			assert.ErrorIs(t, err, errInvalidTimeParameter)

			return
		}

		assert.NotNil(t, result, "success should return non-nil time")
		assert.False(t, result.IsZero(), "parsed time should not be zero")
	})
}

// FuzzParseLimit verifies that parseLimit never panics and returns either a
// non-negative limit or an error for any input string.
func FuzzParseLimit(f *testing.F) {
	f.Add("0")
	f.Add("1")
	f.Add("100")
	f.Add("-1")
	f.Add("-100")
	f.Add("")
	f.Add("abc")
	f.Add("99999999999999999999999999")
	f.Add(strings.Repeat("0", 1000))

	f.Fuzz(func(t *testing.T, s string) {
		result, err := parseLimit(s)

		if s == "" {
			assert.Equal(t, 0, result, "empty input should return 0")
			assert.NoError(t, err)

			return
		}

		if err != nil {
			assert.Equal(t, 0, result, "error should return 0")

			return
		}

		assert.GreaterOrEqual(t, result, 0, "limit should be non-negative: %d", result)
	})
}

// FuzzParseTimeParamRFC3339 verifies that time.Parse with RFC3339 never
// panics for any input string.
func FuzzParseTimeParamRFC3339(f *testing.F) {
	f.Add("2024-01-01T00:00:00Z")
	f.Add("")
	f.Add("not-a-date")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = time.Parse(time.RFC3339, s)
	})
}

// FuzzParseLimitIntegerOverflow verifies that strconv.Atoi handles integer
// overflow gracefully for any input string.
func FuzzParseLimitIntegerOverflow(f *testing.F) {
	f.Add("99999999999999999999999999")
	f.Add("-99999999999999999999999999")
	f.Add(strings.Repeat("9", 1000))
	f.Add("1.5")
	f.Add("1e100")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = strconv.Atoi(s)
	})
}
