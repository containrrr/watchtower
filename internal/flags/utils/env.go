package utils

import (
	"math"
	"time"
)

// DurationFromSeconds converts a floating-point second count to time.Duration.
//
// The nanosecond product is clamped to MaxInt64/MinInt64 when it would overflow
// time.Duration's integer range.
//
// Parameters:
//   - seconds: Duration length in seconds (may be fractional).
//
// Returns:
//   - time.Duration: Clamped duration value.
func DurationFromSeconds(seconds float64) time.Duration {
	nanos := seconds * float64(time.Second)

	if nanos > float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}

	if nanos < float64(math.MinInt64) {
		return time.Duration(math.MinInt64)
	}

	return time.Duration(nanos)
}

// IsPureNumeric reports whether str is a bare number (integer or float,
// possibly signed) with no duration unit characters.
func IsPureNumeric(str string) bool {
	if str == "" {
		return false
	}

	sawDigit := false
	sawDot := false

	for i, char := range str {
		switch {
		case char >= '0' && char <= '9':
			sawDigit = true
		case char == '.':
			if sawDot {
				return false
			}

			sawDot = true
		case char == '-' || char == '+':
			if i != 0 {
				return false
			}
		default:
			return false
		}
	}

	return sawDigit
}
