// Package utils provides shared helpers for flag registration and value parsing.
package utils

import (
	"regexp"
	"strings"
)

var (
	// commaOrSpace splits on commas, spaces, or runs of either.
	commaOrSpace = regexp.MustCompile("[, ]+")
	// commaOnly splits on commas only (spaces inside tokens are kept).
	commaOnly = regexp.MustCompile(",")
)

// FilterEmptyStrings removes empty or whitespace-only strings from a slice.
//
// Parameters:
//   - values: Slice of strings.
//
// Returns:
//   - []string: Filtered slice without empty or whitespace-only strings.
func FilterEmptyStrings(values []string) []string {
	var result []string

	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}

	return result
}

// SplitCommaOrSpace splits on commas and/or spaces and drops empty tokens.
//
// Parameters:
//   - value: Raw list string from env or defaults.
//
// Returns:
//   - []string: Parsed non-empty tokens.
func SplitCommaOrSpace(value string) []string {
	return FilterEmptyStrings(commaOrSpace.Split(value, -1))
}

// SplitCommaOnly splits on commas only and drops empty tokens.
//
// Parameters:
//   - value: Raw list string from env or defaults.
//
// Returns:
//   - []string: Parsed non-empty tokens.
func SplitCommaOnly(value string) []string {
	return FilterEmptyStrings(commaOnly.Split(value, -1))
}

// SplitNotificationValues parses notification URLs separated by commas or spaces.
//
// Comma-delimited fragments without "://" are recombined so commas inside URLs
// (query params, tenant IDs) are preserved.
//
// Parameters:
//   - value: A string containing one or more notification URLs.
//
// Returns:
//   - []string: Parsed notification URLs. Invalid URLs are included but logged.
func SplitNotificationValues(value string) []string {
	type delimiterType int

	const (
		delimiterComma delimiterType = iota
		delimiterSpace
	)

	type splitPart struct {
		text      string
		delimiter delimiterType
	}

	var (
		parts   []splitPart
		current strings.Builder
	)

	lastDelimiter := delimiterSpace

	for _, char := range value {
		switch char {
		case ',':
			if current.Len() > 0 {
				parts = append(parts, splitPart{text: current.String(), delimiter: lastDelimiter})
				current.Reset()
			}

			lastDelimiter = delimiterComma
		case ' ':
			if current.Len() > 0 {
				parts = append(parts, splitPart{text: current.String(), delimiter: lastDelimiter})
				current.Reset()
			}

			lastDelimiter = delimiterSpace
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, splitPart{text: current.String(), delimiter: lastDelimiter})
	}

	var result []string

	for _, part := range parts {
		part.text = strings.TrimSpace(part.text)
		if part.text == "" {
			continue
		}

		if part.delimiter == delimiterComma && len(result) > 0 &&
			!strings.Contains(part.text, "://") {
			result[len(result)-1] = result[len(result)-1] + "," + part.text
		} else {
			result = append(result, part.text)
		}
	}

	// Preserve order. Callers (shoutrrr) validate URLs when building notifiers.
	// Do not log values here — notification URLs often embed tokens.
	return append([]string(nil), result...)
}
