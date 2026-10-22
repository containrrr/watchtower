package spec

import (
	"strings"

	"github.com/nicholas-fedor/watchtower/internal/flags/utils"
)

// ParseList splits a raw list string using the FlagSpec ListParse strategy.
//
// Parameters:
//   - raw: Raw env or default list string.
//   - parse: List parse kind from FlagSpec.
//
// Returns:
//   - []string: Parsed tokens for the given strategy.
func ParseList(raw string, parse ListParseKind) []string {
	switch parse {
	case ListCommaOnly:
		return utils.SplitCommaOnly(raw)
	case ListNotificationURLs:
		return utils.FilterEmptyStrings(utils.SplitNotificationValues(raw))
	case ListNewline:
		return utils.FilterEmptyStrings(splitNewlines(raw))
	case ListCommaOrSpace, ListNative, ListNone:
		return utils.SplitCommaOrSpace(raw)
	default:
		return utils.SplitCommaOrSpace(raw)
	}
}

// splitNewlines splits raw on newlines and trims surrounding space.
func splitNewlines(raw string) []string {
	if raw == "" {
		return nil
	}

	lines := make([]string, 0)

	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lines = append(lines, line)
	}

	return lines
}
