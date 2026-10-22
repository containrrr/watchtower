package notifications

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// templateFuncs defines utility functions for notification templates.
var templateFuncs = template.FuncMap{
	"ToUpper":         strings.ToUpper,
	"ToLower":         strings.ToLower,
	"ToJSON":          toJSON,
	"ToPorcelainJSON": toPorcelainJSON,
	"Title":           cases.Title(language.AmericanEnglish).String,
	"RFC1123":         formatRFC1123,
	"HasKey":          hasKey,
	"FormatDiskSpace": formatDiskSpace,
	"VersionInfo":     versionInfo,
}

// versionInfo renders a container's OCI version transition for report templates.
//
// It returns "v1 -> v2" for a real transition, a single version when only one side
// is labelled, and an empty string when neither is, so templates can guard on it
// with {{with}} and omit the field entirely for unlabelled images.
func versionInfo(report types.ContainerReport) string {
	if report == nil {
		return ""
	}

	oldVersion, newVersion := report.CurrentImageVersion(), report.LatestImageVersion()

	switch {
	case oldVersion != "" && newVersion != "" && oldVersion != newVersion:
		return oldVersion + " -> " + newVersion
	case newVersion != "":
		return newVersion
	default:
		return oldVersion
	}
}

// hasKey reports whether key is present in m.
//
// Parameters:
//   - m: Map to inspect. Non-map values are treated as missing.
//   - key: Map key to look up.
//
// Returns:
//   - bool: True when key is present.
func hasKey(m any, key string) bool {
	data, ok := m.(map[string]any)
	if !ok {
		return false
	}

	_, exists := data[key]

	return exists
}

// toJSON marshals a value to a formatted JSON string for use in templates.
// If marshaling fails, it returns an error message as the string.
func toJSON(v any) string {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal JSON in notification template: %v", err)
	}

	return string(bytes)
}

// formatRFC1123 parses an RFC3339 timestamp string and formats it as RFC1123.
// If parsing fails, it returns the original string.
func formatRFC1123(value string) string {
	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}

	return timestamp.Format(time.RFC1123)
}

// formatDiskSpace formats a template value as a human-readable disk size.
//
// Numeric values use decimal units from util.FormatDiskSpace. Non-numeric
// values, including nil, return "unknown".
//
// Parameters:
//   - value: Template value, typically an int64 byte count from log data.
//
// Returns:
//   - string: Human-readable size such as "10 GB", or "unknown".
func formatDiskSpace(value any) string {
	bytes, ok := int64FromAny(value)
	if !ok {
		return "unknown"
	}

	return util.FormatDiskSpace(bytes)
}

// int64FromAny converts common numeric template values to int64.
//
// Parameters:
//   - value: Value from log data or JSON decoding.
//
// Returns:
//   - int64: Converted integer when the value is a supported numeric type.
//   - bool: False when the value is missing, non-numeric, or out of range.
func int64FromAny(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint64:
		if typed > math.MaxInt64 {
			return 0, false
		}

		return int64(typed), true
	case float64:
		if math.IsNaN(typed) || typed >= float64(math.MaxInt64) || typed < float64(math.MinInt64) {
			return 0, false
		}

		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}

		return parsed, true
	default:
		return 0, false
	}
}
