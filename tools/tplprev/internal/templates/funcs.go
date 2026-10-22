// Package templates provides template functions and named builtin catalogs.
package templates

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/nicholas-fedor/tplprev/internal/report"
)

const (
	kilobyteMultiplier int64 = 1000
	megabyteMultiplier       = 1000 * kilobyteMultiplier
	gigabyteMultiplier       = 1000 * megabyteMultiplier
	terabyteMultiplier       = 1000 * gigabyteMultiplier
	petabyteMultiplier       = 1000 * terabyteMultiplier
)

// Funcs defines a set of utility functions for use in notification templates.
var Funcs = template.FuncMap{
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

// versionInfo renders a container's OCI version transition, mirroring the
// helper of the same name in Watchtower's notifications package.
func versionInfo(containerReport report.ContainerReport) string {
	if containerReport == nil {
		return ""
	}

	oldVersion := containerReport.CurrentImageVersion()
	newVersion := containerReport.LatestImageVersion()

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

// porcelainContainer represents a single container in the porcelain JSON report.
type porcelainContainer struct {
	Name            string `json:"name"`
	Image           string `json:"image"`
	ImageID         string `json:"image_id"`
	LatestImageID   string `json:"latest_image_id"`
	State           string `json:"state"`
	UpdateAvailable bool   `json:"update_available"`
	Error           string `json:"error,omitempty"`
}

// porcelainReport is the top-level JSON structure for porcelain JSON output.
type porcelainReport struct {
	Containers []porcelainContainer `json:"containers"`
}

// toPorcelainJSON marshals a report.Report to an indented JSON string for templates.
// If marshaling fails, it returns an error message as the string.
func toPorcelainJSON(v any) string {
	if v == nil {
		return "{\n  \"containers\": []\n}"
	}

	sourceReport, ok := v.(report.Report)
	if !ok {
		return "failed to marshal porcelain JSON: input is not a report.Report"
	}

	report := porcelainReport{
		Containers: make([]porcelainContainer, 0, len(sourceReport.All())),
	}

	for _, containerReport := range sourceReport.All() {
		container := porcelainContainer{
			Name:            containerReport.Name(),
			Image:           containerReport.ImageName(),
			ImageID:         containerReport.CurrentImageID().ShortID(),
			LatestImageID:   containerReport.LatestImageID().ShortID(),
			State:           containerReport.State(),
			UpdateAvailable: containerReport.CurrentImageID() != containerReport.LatestImageID(),
		}
		if err := containerReport.Error(); err != "" {
			container.Error = err
		}

		report.Containers = append(report.Containers, container)
	}

	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal porcelain JSON: %v", err)
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

	return formatDiskSpaceBytes(bytes)
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

// formatDiskSpaceBytes formats a byte count as a compact decimal size string.
//
// Parameters:
//   - bytes: Size in bytes. Negative values are formatted with a leading minus.
//
// Returns:
//   - string: Human-readable size such as "10 GB" or "512 B".
func formatDiskSpaceBytes(bytes int64) string {
	if bytes == math.MinInt64 {
		return "-" + formatDiskSpaceBytes(math.MaxInt64)
	}

	if bytes < 0 {
		return "-" + formatDiskSpaceBytes(-bytes)
	}

	units := []struct {
		suffix string
		size   int64
	}{
		{"PB", petabyteMultiplier},
		{"TB", terabyteMultiplier},
		{"GB", gigabyteMultiplier},
		{"MB", megabyteMultiplier},
		{"KB", kilobyteMultiplier},
	}

	for _, unit := range units {
		if bytes >= unit.size {
			if bytes%unit.size == 0 {
				return strconv.FormatInt(bytes/unit.size, 10) + " " + unit.suffix
			}

			formatted := strconv.FormatFloat(float64(bytes)/float64(unit.size), 'f', 2, 64)
			formatted = strings.TrimRight(formatted, "0")
			formatted = strings.TrimRight(formatted, ".")

			return formatted + " " + unit.suffix
		}
	}

	return strconv.FormatInt(bytes, 10) + " B"
}
