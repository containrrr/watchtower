package util

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Errors for duration parsing.
var (
	// errNegativeDuration indicates a negative duration value was provided.
	errNegativeDuration = errors.New("negative values are not supported")

	// errInvalidChar indicates an invalid character in a duration string.
	errInvalidChar = errors.New("invalid character in duration string")

	// errInvalidUnit indicates an invalid unit suffix in a duration string.
	errInvalidUnit = errors.New("invalid unit in duration string")
)

// timeUnit represents a single unit of time (hours, minutes, or seconds) with its value and labels.
type timeUnit struct {
	value    int64  // The numeric value of the unit (e.g., 2 for 2 hours)
	singular string // The singular form of the unit (e.g., "hour")
	plural   string // The plural form of the unit (e.g., "hours")
}

// SliceSubtract returns elements in the first slice that are not in the second slice.
//
// Parameters:
//   - slice: Source slice.
//   - subtractFrom: Slice containing elements to exclude.
//
// Returns:
//   - []string: Slice containing elements unique to the source slice.
func SliceSubtract(slice, subtractFrom []string) []string {
	result := []string{}

	for _, element1 := range slice {
		found := slices.Contains(subtractFrom, element1)

		if !found {
			result = append(result, element1)
		}
	}

	return result
}

// StringMapSubtract removes matching key-value pairs.
//
// Parameters:
//   - map1: Source map.
//   - map2: Map to subtract.
//
// Returns:
//   - map[string]string: New map with unique or differing entries.
func StringMapSubtract(map1, map2 map[string]string) map[string]string {
	result := map[string]string{}

	for key1, value1 := range map1 {
		if value2, ok := map2[key1]; ok {
			if value2 != value1 {
				result[key1] = value1
			}
		} else {
			result[key1] = value1
		}
	}

	return result
}

// StructMapSubtract removes matching keys.
//
// Parameters:
//   - map1: Source map.
//   - map2: Map to subtract.
//
// Returns:
//   - map[string]struct{}: New map with unique keys.
func StructMapSubtract(map1, map2 map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}

	for key1, value1 := range map1 {
		if _, ok := map2[key1]; !ok {
			result[key1] = value1
		}
	}

	return result
}

// FormatDuration converts a time.Duration into a human-readable string representation.
//
// It breaks down the duration into months, weeks, days, hours, minutes, and seconds, formatting each unit
// with appropriate grammar (singular or plural) and returning a string like
// "1 week, 2 days, 3 hours" or "0 seconds" if the duration is zero.
//
// Parameters:
//   - duration: The time.Duration to convert into a readable string.
//
// Returns:
//   - string: A formatted string representing the duration, always including at least "0 seconds".
func FormatDuration(duration time.Duration) string {
	const (
		hoursPerDay      = 24
		daysPerMonth     = 30
		daysPerWeek      = 7
		minutesPerHour   = 60
		secondsPerMinute = 60
		timeUnitCount    = 6
	)

	// Handle negative durations by recording the sign and working with absolute value.
	negative := duration < 0
	if negative {
		duration = duration.Abs()
	}

	totalHours := duration.Hours()
	totalDays := int64(totalHours) / hoursPerDay

	months := totalDays / daysPerMonth
	remainingAfterMonths := totalDays % daysPerMonth
	weeks := remainingAfterMonths / daysPerWeek
	remainingAfterWeeks := remainingAfterMonths % daysPerWeek
	hours := int64(totalHours) % hoursPerDay

	// Define units with calculated values, preserving order for display.
	units := []timeUnit{
		{months, "month", "months"},
		{weeks, "week", "weeks"},
		{remainingAfterWeeks, "day", "days"},
		{hours, "hour", "hours"},
		{int64(math.Mod(duration.Minutes(), minutesPerHour)), "minute", "minutes"},
		{int64(math.Mod(duration.Seconds(), secondsPerMinute)), "second", "seconds"},
	}

	parts := make([]string, 0, timeUnitCount)
	// Format each unit, forcing inclusion of seconds if no other parts exist to avoid empty output.
	for i, unit := range units {
		parts = append(
			parts,
			FormatTimeUnit(
				unit.value,
				unit.singular,
				unit.plural,
				i == len(units)-1 && len(parts) == 0,
			),
		)
	}

	// Join non-empty parts with spaces for compact output.
	joined := strings.Join(FilterEmpty(parts), " ")
	if joined == "" {
		return "0 seconds" // Default output when duration is zero or all units are skipped.
	}

	// Prepend negative sign if the original duration was negative.
	if negative {
		return "-" + joined
	}

	return joined
}

// FormatTimeUnit formats a single time unit into a string based on its value and context.
//
// It applies singular or plural grammar, skipping leading zeros unless forced (e.g., for seconds as the last unit),
// returning an empty string for skippable zeros to maintain a concise output.
//
// Parameters:
//   - value: The numeric value of the unit (e.g., 2 for 2 hours).
//   - singular: The singular form of the unit (e.g., "hour").
//   - plural: The plural form of the unit (e.g., "hours").
//   - forceInclude: A boolean indicating whether to include the unit even if zero (e.g., for seconds as fallback).
//
// Returns:
//   - string: The formatted unit (e.g., "1 hour", "2 minutes") or empty string if skipped.
func FormatTimeUnit(value int64, singular, plural string, forceInclude bool) string {
	switch {
	case value == 1:
		return "1 " + singular
	case value > 1 || forceInclude:
		return fmt.Sprintf("%d %s", value, plural)
	default:
		return "" // Skip zero values unless forced.
	}
}

// FilterEmpty removes empty strings from a slice, returning only non-empty elements.
//
// It ensures the final formatted duration string excludes unnecessary parts, maintaining readability
// by filtering out zero-value units that were not explicitly included.
//
// Parameters:
//   - parts: A slice of strings representing formatted time units (e.g., "1 hour", "").
//
// Returns:
//   - []string: A new slice containing only the non-empty strings from the input.
func FilterEmpty(parts []string) []string {
	var filtered []string

	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}

	return filtered
}

// NormalizeContainerName trims the leading "/" from container names.
//
// Parameters:
//   - name: Container name, potentially with leading "/".
//
// Returns:
//   - string: Normalized name without leading "/".
func NormalizeContainerName(name string) string {
	return strings.TrimLeft(name, "/")
}

// ParseDuration parses a duration string with extended unit support.
//
// It supports all standard Go duration units (h, m, s, ms, us, ns) plus:
//   - d: days (24 hours)
//   - w: weeks (7 days)
//   - M: months (30 days)
//
// Units can be combined (e.g., "1w2d", "2M3w", "1M15d12h").
// An empty string or "0" returns a zero duration without error.
//
// Parameters:
//   - s: The duration string to parse.
//
// Returns:
//   - time.Duration: The parsed duration.
//   - error: Non-nil if the string cannot be parsed.
func ParseDuration(duration string) (time.Duration, error) {
	if duration == "" || duration == "0" {
		return 0, nil
	}

	// Reject negative values — all durations in Watchtower are non-negative.
	if duration[0] == '-' {
		return 0, fmt.Errorf("invalid duration %q: %w", duration, errNegativeDuration)
	}

	expanded, err := expandDurationUnits(duration)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", duration, err)
	}

	d, err := time.ParseDuration(expanded)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", duration, err)
	}

	return d, nil
}

// isValidDurationChar reports whether ch is a valid duration unit character
// that may appear without a preceding numeric value during parsing.
//
// Parameters:
//   - ch: The byte to validate.
//
// Returns:
//   - bool: True if ch is a recognized duration unit (h, m, s, d, w, M, n, u).
func isValidDurationChar(ch byte) bool {
	switch ch {
	case 'h', 'm', 's', 'd', 'w', 'M', 'n', 'u':
		return true
	default:
		return false
	}
}

// expandDurationUnits converts extended unit suffixes (d, w, M) into their
// hour equivalents so that time.ParseDuration can handle them.
//
// For example:
//   - "1d" → "24h"
//   - "1w" → "168h"
//   - "1M" → "720h"
//   - "1M2w3d12h" → "720h336h72h12h"
//
// Standard units (h, m, s, ms, us, ns) are passed through unchanged.
//
// Parameters:
//   - s: Duration string potentially containing d, w, or M units.
//
// Returns:
//   - string: Duration string with only standard Go units.
//   - error: Non-nil if the string is malformed.
func expandDurationUnits(durationStr string) (string, error) {
	const (
		hoursPerDay    = 24
		daysPerWeek    = 7
		daysPerMonth   = 30
		growMultiplier = 4 // Pre-allocate space for expanded output.
	)

	// processUnit handles the unit character for a parsed numeric string.
	// For extended units (d, w, M) it converts to hours via multiplyUnit.
	// For standard Go units it validates and passes through unchanged.
	processUnit := func(numStr string, unit byte) (string, error) {
		switch unit {
		case 'd':
			return multiplyUnit(numStr, hoursPerDay)
		case 'w':
			return multiplyUnit(numStr, hoursPerDay*daysPerWeek)
		case 'M':
			return multiplyUnit(numStr, hoursPerDay*daysPerMonth)
		default:
			// Standard Go unit — validate and pass through unchanged.
			if !isValidDurationChar(unit) {
				return "", fmt.Errorf(
					"%w: %q in %q",
					errInvalidUnit, unit, durationStr,
				)
			}

			return numStr + string(unit), nil
		}
	}

	var result strings.Builder

	result.Grow(len(durationStr) * growMultiplier)

	i := 0

	for i < len(durationStr) {
		// Find the numeric part (digits and optional decimal point).
		numStart := i

		for i < len(durationStr) && (durationStr[i] >= '0' && durationStr[i] <= '9' || durationStr[i] == '.') {
			i++
		}

		if i == numStart {
			// No number found — validate that the character is a known
			// duration unit or separator. Reject anything else.
			char := durationStr[i]
			if !isValidDurationChar(char) {
				return "", fmt.Errorf(
					"%w: %q in %q at position %d",
					errInvalidChar, char, durationStr, i,
				)
			}

			result.WriteByte(char)

			i++

			continue
		}

		numStr := durationStr[numStart:i]

		if i >= len(durationStr) {
			// Trailing number with no unit — pass through.
			result.WriteString(numStr)

			break
		}

		// Read the unit character.
		unit := durationStr[i]
		i++

		expanded, err := processUnit(numStr, unit)
		if err != nil {
			return "", err
		}

		result.WriteString(expanded)
	}

	return result.String(), nil
}

// multiplyUnit multiplies a numeric string by a factor and returns the result
// as a duration string in hours (e.g., "1" * 24 → "24h").
//
// Parameters:
//   - numStr: Numeric string (integer or decimal).
//   - factor: Multiplier (e.g., 24 for days-to-hours).
//
// Returns:
//   - string: Resulting duration string (e.g., "24h").
//   - error: Non-nil if numStr cannot be parsed as a float.
func multiplyUnit(numStr string, factor float64) (string, error) {
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return "", fmt.Errorf("invalid number %q: %w", numStr, err)
	}

	result := num * factor

	// Use 'f' format to avoid scientific notation, then trim trailing zeros.
	formatted := fmt.Sprintf("%.10f", result)
	// Remove trailing zeros after decimal point.
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")

	return formatted + "h", nil
}
