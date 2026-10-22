package utils

import (
	"math"
	"strconv"
	"testing"
)

// FuzzIsPureNumeric verifies that IsPureNumeric never panics and that
// when it returns true, strconv.ParseFloat either succeeds with a finite
// value or fails only with a range error.
func FuzzIsPureNumeric(f *testing.F) {
	// Valid bare numbers.
	f.Add("0")
	f.Add("42")
	f.Add("300")
	f.Add("1.5")
	f.Add(".5")
	f.Add("1.")
	f.Add("-10")
	f.Add("+3")
	f.Add("-1.5")
	f.Add("+.5")
	f.Add("-.5")
	f.Add("+0")
	f.Add("-0")

	// Invalid: multiple dots and misplaced signs.
	f.Add("1.2.3")
	f.Add("1..2")
	f.Add("1-2")
	f.Add("12+")
	f.Add("1+2")
	f.Add("5-")
	f.Add("+-1")
	f.Add("-+5")
	f.Add("++3")

	// Other invalid cases (units, letters, control chars, long inputs, etc.).
	f.Add("")
	f.Add(".")
	f.Add("+")
	f.Add("-")
	f.Add("+.")
	f.Add("-.5")
	f.Add("1a")
	f.Add("1e3")
	f.Add("1E-3")
	f.Add("Inf")
	f.Add("NaN")
	f.Add("\x00")
	f.Add("1\x002")
	f.Add("¹²³") // Unicode digits — must be rejected
	f.Add("1,000")
	f.Add(" 42")
	f.Add("1.2.3.4.5")
	f.Add(string(make([]byte, 1024))) // long string of zeros (will be filled in fuzzer)

	// Duration-like inputs.
	f.Add("30s")
	f.Add("2m")
	f.Add("1h")
	f.Add("1d")

	f.Fuzz(func(t *testing.T, input string) {
		result := IsPureNumeric(input)

		if result {
			// When true, ParseFloat must succeed with finite value or fail only on range.
			val, err := strconv.ParseFloat(input, 64)
			if err != nil {
				numErr, ok := err.(*strconv.NumError)
				if ok && numErr.Err == strconv.ErrRange {
					return // out-of-range magnitude is acceptable
				}

				t.Errorf("IsPureNumeric(%q) = true, but strconv.ParseFloat returned non-range error: %v", input, err)

				return
			}

			if math.IsNaN(val) || math.IsInf(val, 0) {
				t.Errorf("IsPureNumeric(%q) = true, but parsed value %v is NaN or Inf", input, val)
			}
		}

		// We do not assert the inverse (ParseFloat success ⇒ IsPureNumeric true)
		// because we intentionally reject scientific notation, Inf, NaN, etc.
		// Those should fall through to normal duration parsing.
	})
}
