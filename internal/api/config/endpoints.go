package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Canonical HTTP API endpoint names corresponding to
// valid values for the http-api-endpoints configuration option.
const (
	endpointHealth     = "health"
	endpointUpdate     = "update"
	endpointMetrics    = "metrics"
	endpointContainers = "containers"
	endpointCheck      = "check"
	endpointHistory    = "history"
	endpointImages     = "images"
	endpointConfig     = "config"
	endpointEvents     = "events"
	endpointSwagger    = "swagger"
)

// allEndpointNames is the ordered list of every known endpoint name.
var allEndpointNames = []string{
	endpointHealth,
	endpointUpdate,
	endpointMetrics,
	endpointContainers,
	endpointCheck,
	endpointHistory,
	endpointImages,
	endpointConfig,
	endpointEvents,
	endpointSwagger,
}

var (
	// errUnknownEndpoint is returned when an endpoint name is not recognized.
	errUnknownEndpoint = errors.New("unknown HTTP API endpoint")
	// errAllMustBeAlone is returned when "all" appears with other names.
	errAllMustBeAlone = errors.New(`"all" must be the only value in http-api-endpoints`)
)

// enabledEndpoints is the map of enabled HTTP API endpoints.
type enabledEndpoints map[string]struct{}

// Contains returns whether or not the value is in the map of enabled endpoints.
func (m enabledEndpoints) Contains(name string) bool {
	if m == nil {
		return false
	}

	_, ok := m[name]

	return ok
}

// Empty reports whether or not the map of enabled endpoints is empty.
func (m enabledEndpoints) Empty() bool {
	return len(m) == 0
}

// parseAPIEndpoints parses an endpoint allowlist.
//
// Filter rules:
//   - Values may already be split on commas or spaces by flag/env parsing
//   - Each entry is still trimmed and lowercased
//   - The special value "all" (alone) expands to every known endpoint
//   - Duplicates are ignored
//   - An empty list yields an empty set
//
// Parameters:
//   - values: Endpoint names, or a single "all".
//
// Returns:
//   - EndpointMap: Parsed map of endpoint names.
//   - error: Non-nil if the value is invalid.
func parseAPIEndpoints(values []string) (enabledEndpoints, error) {
	names := make([]string, 0, len(values))

	for _, part := range values {
		// Allow a single string that still contains separators (e.g. tests or
		// callers that have not pre-split).
		for _, token := range strings.FieldsFunc(part, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		}) {
			name := strings.ToLower(strings.TrimSpace(token))
			if name == "" {
				continue
			}

			names = append(names, name)
		}
	}

	if len(names) == 0 {
		return enabledEndpoints{}, nil
	}

	if slices.Contains(names, "all") {
		if len(names) != 1 {
			return nil, errAllMustBeAlone
		}

		return allEndpoints(), nil
	}

	endpointMap := make(enabledEndpoints, len(names))
	valid := allEndpoints()

	for _, name := range names {
		if !valid.Contains(name) {
			return nil, fmt.Errorf("%w: %q (valid: %s)", errUnknownEndpoint, name, formatEndpoints(valid))
		}

		endpointMap[name] = struct{}{}
	}

	return endpointMap, nil
}

// parseLegacyOptions builds a map of endpoints from legacy
// enable configuration options (update, metrics, containers).
//
// Deprecated: Prefer parseAPIEndpoints with http-api-endpoints. Legacy flags
// will be removed in the v2 release.
//
// Parameters:
//   - update: Whether http-api-update is set.
//   - metrics: Whether http-api-metrics is set.
//   - containers: Whether http-api-containers is set.
//
// Returns:
//   - EndpointMap: Mapped endpoint names.
//
// TODO: Remove parseLegacyOptions when legacy HTTP API flags are removed in v2.
//
//nolint:godox
func parseLegacyOptions(update, metrics, containers bool) enabledEndpoints {
	endpointMap := make(enabledEndpoints)

	if update {
		endpointMap[endpointUpdate] = struct{}{}
	}

	if metrics {
		endpointMap[endpointMetrics] = struct{}{}
	}

	if containers {
		endpointMap[endpointContainers] = struct{}{}
	}

	return endpointMap
}

// formatEndpoints returns a stable comma-separated list of endpoint names
// in allEndpointNames order (only those present in the map).
//
// Parameters:
//   - endpointMap: Endpoint map to format.
//
// Returns:
//   - string: Comma-separated names, or empty if map is empty.
func formatEndpoints(endpointMap enabledEndpoints) string {
	if endpointMap.Empty() {
		return ""
	}

	parts := make([]string, 0, len(endpointMap))
	for _, name := range allEndpointNames {
		if endpointMap.Contains(name) {
			parts = append(parts, name)
		}
	}

	// Include any unexpected keys for debugging (should not happen).
	for name := range endpointMap {
		if !slices.Contains(allEndpointNames, name) {
			parts = append(parts, name)
		}
	}

	return strings.Join(parts, ",")
}

// ResolveEndpoints selects the active endpoint map from the canonical allowlist
// and/or legacy configuration options.
//
// Rules:
//  1. Parse the allowlist when non-empty (unknown names fail and "all" expands fully).
//  2. Union with any legacy update/metrics/containers flags (deduplicated).
//  3. When legacy flags are used, log one deprecation warning with the final
//     equivalent allowlist value.
//  4. Empty allowlist and no legacy flags is equivalent to an empty map (API off).
//
// Parameters:
//   - endpoints: Values from http-api-endpoints.
//   - legacyUpdate: Legacy http-api-update.
//   - legacyMetrics: Legacy http-api-metrics.
//   - legacyContainers: Legacy http-api-containers.
//
// Returns:
//   - EndpointMap: Resolved enabled endpoints.
//   - error: Non-nil if the allowlist contains invalid values.
//
// TODO: Drop legacy parameters when removing legacy HTTP API configuration options in v2.
//
//nolint:godox
func ResolveEndpoints(endpoints []string, legacyUpdate, legacyMetrics, legacyContainers bool) (enabledEndpoints, error) {
	legacyAny := legacyUpdate || legacyMetrics || legacyContainers

	enabledEndpointsMap := enabledEndpoints{}

	if len(endpoints) > 0 {
		parsed, err := parseAPIEndpoints(endpoints)
		if err != nil {
			return nil, err
		}

		enabledEndpointsMap = parsed
	}

	if legacyAny {
		legacyEndpointMap := parseLegacyOptions(
			legacyUpdate,
			legacyMetrics,
			legacyContainers,
		)
		for name := range legacyEndpointMap {
			if enabledEndpointsMap == nil {
				enabledEndpointsMap = enabledEndpoints{}
			}

			enabledEndpointsMap[name] = struct{}{}
		}
	}

	if enabledEndpointsMap == nil {
		return enabledEndpoints{}, nil
	}

	return enabledEndpointsMap, nil
}

// SetEndpointConfig populates the runtime configuration with the HTTP API
// Endpoints parsed from the http-api-endpoints configuration option.
//
// Parameters:
//   - endpointMap: Enabled endpoints.
//   - cfg: RunConfig to update.
func SetEndpointConfig(endpointMap enabledEndpoints, cfg *types.RunConfig) {
	cfg.EnableHealthAPI = endpointMap.Contains(endpointHealth)
	cfg.EnableUpdateAPI = endpointMap.Contains(endpointUpdate)
	cfg.EnableMetricsAPI = endpointMap.Contains(endpointMetrics)
	cfg.EnableContainersAPI = endpointMap.Contains(endpointContainers)
	cfg.EnableCheckAPI = endpointMap.Contains(endpointCheck)
	cfg.EnableHistoryAPI = endpointMap.Contains(endpointHistory)
	cfg.EnableImagesAPI = endpointMap.Contains(endpointImages)
	cfg.EnableConfigAPI = endpointMap.Contains(endpointConfig)
	cfg.EnableEventsAPI = endpointMap.Contains(endpointEvents)
	cfg.EnableSwaggerAPI = endpointMap.Contains(endpointSwagger)
}

func allEndpoints() enabledEndpoints {
	endpointMap := make(enabledEndpoints, len(allEndpointNames))
	for _, name := range allEndpointNames {
		endpointMap[name] = struct{}{}
	}

	return endpointMap
}
