package update

import (
	"regexp"
	"slices"
	"strings"
)

// ContainerFilter creates a filter function that matches containers by name
// pattern using Go regex syntax. Invalid regex patterns are treated as literal
// strings for exact matching. Leading slashes are stripped from patterns and
// names to match CLI filter behavior. Returns a pass-through filter when
// patterns is empty.
//
// Parameters:
//   - patterns: Container name patterns to match. Supports Go regex syntax.
//
// Returns:
//   - func(string, bool) bool: Filter function that returns true when the
//     container name matches any pattern.
func ContainerFilter(patterns []string) func(string, bool) bool {
	if len(patterns) == 0 {
		return func(_ string, _ bool) bool { return true }
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	exact := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(pattern, "/")

		re, err := regexp.Compile("^" + pattern + "$")
		if err == nil {
			compiled = append(compiled, re)
		} else {
			exact = append(exact, pattern)
		}
	}

	return func(name string, _ bool) bool {
		name = strings.TrimPrefix(name, "/")

		for _, re := range compiled {
			if re.MatchString(name) {
				return true
			}
		}

		return slices.Contains(exact, name)
	}
}
