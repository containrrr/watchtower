package filters

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// noScope is the default scope value when none is specified.
	noScope = "none"

	// Expected number of parts in a key=value label pair.
	labelPairPartsCount = 2

	// maxLabelPairBytes is the maximum combined byte length of a label key and
	// value, consistent with containerd's label validation.
	maxLabelPairBytes = 4096

	// maxLabelPairs is the maximum number of label pairs accepted from user
	// input, preventing memory exhaustion from unbounded slice lengths.
	maxLabelPairs = 100

	// enableLabelKey is the Docker label key used by Watchtower to control whether
	// a container is managed.
	enableLabelKey = "com.centurylinklabs.watchtower.enable"
)

var (
	// errTooManyLabelPairs is returned when the number of provided label pairs
	// exceeds the allowed maximum.
	errTooManyLabelPairs = errors.New("too many label pairs")

	// errLabelPairMissingEquals is returned when a label pair does not contain
	// exactly one '=' separator.
	errLabelPairMissingEquals = errors.New("label pair missing '=' separator")

	// errLabelPairEmptyKey is returned when a label pair has an empty key.
	errLabelPairEmptyKey = errors.New("label pair has empty key")

	// errLabelPairTooLong is returned when a label pair exceeds the maximum
	// combined byte length.
	errLabelPairTooLong = errors.New("label pair exceeds maximum combined length")
)

// WatchtowerContainersFilter selects only Watchtower containers.
//
// Returns:
//   - bool: True if container is Watchtower, false otherwise.
func WatchtowerContainersFilter(c types.FilterableContainer) bool {
	return c.IsWatchtower()
}

// UnscopedWatchtowerContainersFilter selects only unscoped Watchtower containers.
//
// Returns:
//   - bool: True if container is Watchtower and has no scope or scope "none", false otherwise.
func UnscopedWatchtowerContainersFilter(c types.FilterableContainer) bool {
	if !c.IsWatchtower() {
		return false
	}

	containerScope, containerHasScope := c.Scope()
	if !containerHasScope || containerScope == "" {
		containerScope = noScope
	}

	return containerScope == noScope
}

// excludeOldWatchtowerFilter rejects containers that are old Watchtower
// containers renamed during self-update (prefixed with "watchtower-old-").
//
// These containers are predecessors that should only be removed, never updated
// or included in update cycles. This filter ensures they are excluded from
// regular container processing regardless of scope.
//
// Returns:
//   - bool: True if the container should be included, false if it is an old
//     Watchtower container that should be excluded.
func excludeOldWatchtowerFilter(c types.FilterableContainer) bool {
	if !c.IsWatchtower() {
		return true
	}

	if IsOldWatchtower(c) {
		return false
	}

	return true
}

// IsOldWatchtower reports whether the container is an old Watchtower
// container renamed during self-update (prefixed with "watchtower-old-").
//
// This is the positive counterpart to excludeOldWatchtowerFilter and is
// intended for guard clauses where a positive check reads more naturally.
//
// Returns:
//   - bool: True if the container is an old Watchtower container.
func IsOldWatchtower(c types.FilterableContainer) bool {
	return c.IsWatchtower() && strings.HasPrefix(strings.TrimLeft(c.Name(), "/"), types.WatchtowerOldPrefix)
}

// excludeOldWatchtowerFilterChain wraps a base filter with old
// Watchtower exclusion. It chains the exclusion check before the base filter,
// ensuring old Watchtower containers are rejected early in the pipeline.
//
// Parameters:
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function that excludes old Watchtower containers
//     and applies the base filter.
func excludeOldWatchtowerFilterChain(baseFilter types.Filter) types.Filter {
	return func(c types.FilterableContainer) bool {
		if !excludeOldWatchtowerFilter(c) {
			return false
		}

		return baseFilter(c)
	}
}

// NoFilter allows all containers through.
//
// Returns:
//   - bool: Always true.
func NoFilter(c types.FilterableContainer) bool {
	return true
}

// FilterByNames selects containers matching specified names.
//
// Name patterns are compiled once when the filter is built.
//
// Parameters:
//   - log: Logger for debug output.
//   - normalizedNames: List of normalized names or regex patterns to match.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function combining name check with base filter.
func FilterByNames(log *zerolog.Logger, normalizedNames []string, baseFilter types.Filter) types.Filter {
	if len(normalizedNames) == 0 {
		return baseFilter
	}

	// Compile once. The returned closure only evaluates the compiled matchers.
	compiled := compileNamePatterns(normalizedNames, true)

	return func(c types.FilterableContainer) bool {
		containerName := c.Name()

		if compiled.match(containerName) {
			if debugEnabled(log) {
				log.Debug().
					Str("container", containerName).
					Strs("names", normalizedNames).
					Msg("Matched container by name/pattern")
			}

			return baseFilter(c)
		}

		if debugEnabled(log) {
			log.Debug().
				Str("container", containerName).
				Strs("names", normalizedNames).
				Msg("Container name did not match any filter")
		}

		return false
	}
}

// FilterByDisableNames excludes containers matching specified names.
//
// Name patterns are compiled once when the filter is built.
//
// Parameters:
//   - log: Logger for debug output.
//   - normalizedDisableNames: Names or regex patterns to exclude.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function excluding names and applying base filter.
func FilterByDisableNames(log *zerolog.Logger, normalizedDisableNames []string, baseFilter types.Filter) types.Filter {
	if len(normalizedDisableNames) == 0 {
		return baseFilter
	}

	// Compile once. The returned closure only evaluates the compiled matchers.
	compiled := compileNamePatterns(normalizedDisableNames, true)

	return func(c types.FilterableContainer) bool {
		containerName := c.Name()

		if compiled.match(containerName) {
			if debugEnabled(log) {
				log.Debug().
					Str("container", containerName).
					Strs("disableNames", normalizedDisableNames).
					Msg("Container excluded by disable name/pattern")
			}

			return false
		}

		if debugEnabled(log) {
			log.Debug().
				Str("container", containerName).
				Strs("disableNames", normalizedDisableNames).
				Msg("Container not excluded by disable names")
		}

		return baseFilter(c)
	}
}

// filterByMonitoredImageNamePatterns restricts monitoring to containers whose image name
// matches specified patterns. When set, only matching containers are monitored.
//
// Image patterns are compiled once when the filter is built.
//
// Parameters:
//   - log: Logger for debug output.
//   - namePatterns: Image name regex patterns.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function restricting to matching image names, then
//     applying base filter.
func filterByMonitoredImageNamePatterns(log *zerolog.Logger, namePatterns []string, baseFilter types.Filter) types.Filter {
	if len(namePatterns) == 0 {
		return baseFilter
	}

	// Compile once. The returned closure only evaluates the compiled matchers.
	compiled := compileNamePatterns(namePatterns, false)

	return func(c types.FilterableContainer) bool {
		imageName := c.ImageName()

		if compiled.match(imageName) {
			if debugEnabled(log) {
				log.Debug().
					Str("container", c.Name()).
					Str("image", imageName).
					Strs("namePatterns", namePatterns).
					Msg("Container image matched pattern")
			}

			return baseFilter(c)
		}

		if debugEnabled(log) {
			log.Debug().
				Str("container", c.Name()).
				Str("image", imageName).
				Strs("namePatterns", namePatterns).
				Msg("Container image did not match any pattern")
		}

		return false
	}
}

// filterBySkippedImageNamePatterns prevents monitoring of containers whose image name
// matches specified patterns. Matching containers are excluded from monitoring.
//
// Image patterns are compiled once when the filter is built.
//
// Parameters:
//   - log: Logger for debug output.
//   - namePatterns: Image name regex patterns.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function excluding matching image names, then applying
//     base filter.
func filterBySkippedImageNamePatterns(log *zerolog.Logger, namePatterns []string, baseFilter types.Filter) types.Filter {
	if len(namePatterns) == 0 {
		return baseFilter
	}

	// Compile once. The returned closure only evaluates the compiled matchers.
	compiled := compileNamePatterns(namePatterns, false)

	return func(c types.FilterableContainer) bool {
		imageName := c.ImageName()

		if compiled.match(imageName) {
			if debugEnabled(log) {
				log.Debug().
					Str("container", c.Name()).
					Str("image", imageName).
					Strs("namePatterns", namePatterns).
					Msg("Container image matched skip pattern")
			}

			return false
		}

		if debugEnabled(log) {
			log.Debug().
				Str("container", c.Name()).
				Str("image", imageName).
				Strs("namePatterns", namePatterns).
				Msg("Container not skipped by image name patterns")
		}

		return baseFilter(c)
	}
}

// parseLabelPairs parses a slice of "key=value" strings into a map.
//
// Values are trimmed of surrounding whitespace for consistency with key
// trimming. Only the first '=' separates key from value and any additional '='
// characters are retained as part of the value. Entries without at least one
// '=', empty keys, or pairs exceeding the maximum combined byte length are
// rejected with an error rather than silently skipped, so callers can surface
// malformed input to the user.
//
// Parameters:
//   - pairs: Raw label pair strings from flag/env input.
//
// Returns:
//   - map[string]string: Parsed label key-value pairs.
//   - error: Non-nil if any pair is malformed or exceeds size limits.
func parseLabelPairs(pairs []string) (map[string]string, error) {
	if len(pairs) > maxLabelPairs {
		return nil, fmt.Errorf("%w: %d > %d", errTooManyLabelPairs, len(pairs), maxLabelPairs)
	}

	result := make(map[string]string)

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", labelPairPartsCount)
		if len(parts) != labelPairPartsCount {
			return nil, fmt.Errorf("%w: %q", errLabelPairMissingEquals, pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("%w: %q", errLabelPairEmptyKey, pair)
		}

		if len(key)+len(value) > maxLabelPairBytes {
			return nil, fmt.Errorf("%w: key=%q value=%q combined=%d bytes", errLabelPairTooLong, key, value, len(key)+len(value))
		}

		result[key] = value
	}

	return result, nil
}

// filterByEnabledLabels restricts processing to containers that have at least
// one of the specified label key-value pairs.
//
// An empty label list disables the filter entirely.
//
// A label entry with an empty value (key="") performs a presence check: the
// label must exist on the container with any value. A non-empty value requires
// an exact match.
//
// Parameters:
//   - labels: Map of label key-value pairs to require.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function matching labels and applying base filter.
func filterByEnabledLabels(log *zerolog.Logger, labels map[string]string, baseFilter types.Filter) types.Filter {
	if len(labels) == 0 {
		return baseFilter
	}

	return func(c types.FilterableContainer) bool {
		clogVal := log.With().
			Str("container", c.Name()).
			Interface("labels", labels).
			Logger()
		clog := &clogVal

		for key, expectedValue := range labels {
			labelValue, ok := c.GetLabel(key)
			if !ok {
				continue
			}

			if expectedValue != "" && labelValue != expectedValue {
				continue
			}

			clog.Debug().
				Str("matched_label", key).
				Msg("Container matched enabled label")

			return baseFilter(c)
		}

		clog.Debug().Msg("Container did not match any enabled label")

		return false
	}
}

// filterByDisabledLabels excludes containers that have any of the specified
// label key-value pairs.
//
// An empty label list disables the filter entirely.
//
// A label entry with an empty value (key="") performs a presence check: the
// label must exist on the container with any value. A non-empty value requires
// an exact match.
//
// Parameters:
//   - labels: Map of label key-value pairs to exclude.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function excluding matching labels and applying base filter.
func filterByDisabledLabels(log *zerolog.Logger, labels map[string]string, baseFilter types.Filter) types.Filter {
	if len(labels) == 0 {
		return baseFilter
	}

	return func(c types.FilterableContainer) bool {
		clogVal := log.With().
			Str("container", c.Name()).
			Interface("labels", labels).
			Logger()
		clog := &clogVal

		for key, expectedValue := range labels {
			labelValue, ok := c.GetLabel(key)
			if !ok {
				continue
			}

			if expectedValue != "" && labelValue != expectedValue {
				continue
			}

			clog.Debug().
				Str("matched_label", key).
				Msg("Container excluded by disabled label")

			return false
		}

		clog.Debug().Msg("Container not excluded by disabled labels")

		return baseFilter(c)
	}
}

// filterByEnableLabel applies the boolean --label-enable filter.
//
// When enable is true, only containers whose enable label is present with a
// value that parses as true are included. When enable is false, containers
// whose enable label is present with a value that parses as false are excluded.
// Absent labels and invalid values are both treated as unset, matching
// Enabled() returning ok=false.
//
// Parameters:
//   - enable: require the enable label when true, exclude false values when false.
//   - baseFilter: base filter to chain.
//
// Returns:
//   - types.Filter: filter function applying enable-label semantics and baseFilter.
func filterByEnableLabel(enable bool, baseFilter types.Filter) types.Filter {
	return func(c types.FilterableContainer) bool {
		enabled, ok := c.Enabled()
		if ok && !enabled {
			return false
		}

		if !ok && enable {
			return false
		}

		return baseFilter(c)
	}
}

// FilterByScope selects containers in a specific scope.
//
// Parameters:
//   - scope: Scope to match.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function matching scope and applying base filter.
func FilterByScope(log *zerolog.Logger, scope string, baseFilter types.Filter) types.Filter {
	return func(c types.FilterableContainer) bool {
		clogVal := log.With().
			Str("container", c.Name()).
			Str("scope", scope).
			Logger()
		clog := &clogVal

		containerScope, containerHasScope := c.Scope()
		if !containerHasScope || containerScope == "" {
			containerScope = noScope // Default to "none" if unset.
		}

		if containerScope == scope {
			clog.Debug().
				Str("container_scope", containerScope).
				Msg("Container matched scope")

			return baseFilter(c)
		}

		clog.Debug().
			Str("container_scope", containerScope).
			Msg("Container scope mismatch")

		return false
	}
}

// FilterByImage selects containers with specific images, optionally including tags.
//
// Parameters:
//   - images: List of image names (with optional tags) to match.
//   - baseFilter: Base filter to chain.
//
// Returns:
//   - types.Filter: Filter function matching images and applying base filter.
func FilterByImage(log *zerolog.Logger, images []string, baseFilter types.Filter) types.Filter {
	if len(images) == 0 {
		return baseFilter // No images specified, apply base filter only.
	}

	return func(c types.FilterableContainer) bool {
		clogVal := log.With().
			Str("container", c.Name()).
			Strs("images", images).
			Logger()
		clog := &clogVal

		for _, targetImage := range images {
			if matchImageAndTag(c.ImageName(), targetImage) {
				clog.Debug().
					Str("image", c.ImageName()).
					Msg("Container matched image")

				return baseFilter(c) // Image matches, proceed with base filter.
			}
		}

		clog.Debug().
			Str("image", c.ImageName()).
			Msg("Container image did not match")

		return false // No matching image found.
	}
}

// compiledPatterns holds precompiled name or image matchers.
type compiledPatterns struct {
	exact   []string
	regexes []*regexp.Regexp
}

// compileNamePatterns compiles name or image filter patterns once.
//
// Patterns without regex metacharacters are stored as exact literals.
// Invalid regex patterns are also kept as exact literals.
//
// Parameters:
//   - patterns: Name or image patterns.
//   - trimSlash: Strip a leading slash from each pattern for container names.
//
// Returns:
//   - compiledPatterns: Exact literals and compiled regexes.
func compileNamePatterns(patterns []string, trimSlash bool) compiledPatterns {
	compiled := compiledPatterns{
		exact:   make([]string, 0, len(patterns)),
		regexes: make([]*regexp.Regexp, 0, len(patterns)),
	}

	for _, pattern := range patterns {
		// Container names are stored without a leading slash.
		if trimSlash {
			pattern = strings.TrimPrefix(pattern, "/")
		}

		// Literal names skip compilation. Only patterns with metacharacters become regexes.
		if regexp.QuoteMeta(pattern) == pattern {
			compiled.exact = append(compiled.exact, pattern)

			continue
		}

		// Invalid regex stays an exact literal, matching matchesName behavior.
		patternRe, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			compiled.exact = append(compiled.exact, pattern)

			continue
		}

		compiled.regexes = append(compiled.regexes, patternRe)
	}

	return compiled
}

// match reports whether value matches an exact literal or compiled regex.
//
// Parameters:
//   - value: Container name or image name to test.
//
// Returns:
//   - bool: True when value matches a literal or regex.
func (p compiledPatterns) match(value string) bool {
	if slices.Contains(p.exact, value) {
		return true
	}

	for _, re := range p.regexes {
		if re.MatchString(value) {
			return true
		}
	}

	return false
}

// debugEnabled reports whether debug logging is active on log.
//
// Parameters:
//   - log: Logger to inspect. Nil is treated as disabled.
//
// Returns:
//   - bool: True when debug-level logging is enabled.
func debugEnabled(log *zerolog.Logger) bool {
	return log != nil && log.GetLevel() <= zerolog.DebugLevel
}

// matchImageAndTag checks if a container's image matches a target image, including optional tag.
//
// Image references are parsed with distribution/reference so host:port registries
// (for example localhost:5000/nginx:alpine) are not split on the port colon.
// When the target omits a tag, any tag on the container image is accepted.
// When the target includes a tag, both name and tag must match. Digest-only
// references compare the familiar name without requiring a matching tag.
//
// Parameters:
//   - containerImage: The container's image name (e.g., "registry:develop").
//   - targetImage: The target image name or image:tag to match (e.g., "registry").
//
// Returns:
//   - bool: True if the image (and tag, if specified) matches, false otherwise.
func matchImageAndTag(containerImage, targetImage string) bool {
	containerRef, err := reference.ParseAnyReference(containerImage)
	if err != nil {
		return false
	}

	targetRef, err := reference.ParseAnyReference(targetImage)
	if err != nil {
		return false
	}

	containerNamed, ok := containerRef.(reference.Named)
	if !ok {
		return false
	}

	targetNamed, ok := targetRef.(reference.Named)
	if !ok {
		return false
	}

	if reference.FamiliarName(containerNamed) != reference.FamiliarName(targetNamed) {
		return false
	}

	targetTagged, targetHasTag := targetRef.(reference.NamedTagged)
	if !targetHasTag {
		return true
	}

	containerTagged, containerHasTag := containerRef.(reference.NamedTagged)
	if !containerHasTag {
		return false
	}

	return containerTagged.Tag() == targetTagged.Tag()
}

// BuildFilter constructs a composite filter for containers.
//
// Parameters:
//   - normalizedNames: Normalized container names or regex patterns. When set, only
//     containers matching these patterns are monitored.
//   - normalizedDisableNames: Container names or regex patterns to skip. Matching
//     containers are excluded from monitoring.
//   - monitoredImageNamePatterns: Image name regex patterns. When set, only containers whose
//     image matches one of these patterns are monitored.
//   - skippedImageNamePatterns: Image name regex patterns. Containers whose image
//     matches one of these patterns are excluded from monitoring.
//   - enabledLabels: Label key-value pairs. When set, only containers matching at least
//     one pair are monitored.
//   - disabledLabels: Label key-value pairs. Containers matching any pair are excluded.
//   - enableLabel: Require enable label if true.
//   - scope: Scope to match.
//
// Returns:
//   - types.Filter: Combined filter function.
//   - string: Description of the filter.
//   - error: Non-nil if any enabled or disabled label pair is malformed.
func BuildFilter(log *zerolog.Logger,
	normalizedNames []string,
	normalizedDisableNames []string,
	monitoredImageNamePatterns []string,
	skippedImageNamePatterns []string,
	enabledLabels []string,
	disabledLabels []string,
	enableLabel bool,
	scope string,
) (types.Filter, string, error) {
	enabledLabelMap, err := parseLabelPairs(enabledLabels)
	if err != nil {
		return nil, "", err
	}

	disabledLabelMap, err := parseLabelPairs(disabledLabels)
	if err != nil {
		return nil, "", err
	}

	clogVal := log.With().
		Strs("names", normalizedNames).
		Strs("disableNames", normalizedDisableNames).
		Strs("monitoredImageNamePatterns", monitoredImageNamePatterns).
		Strs("skippedImageNamePatterns", skippedImageNamePatterns).
		Interface("enabledLabels", enabledLabelMap).
		Interface("disabledLabels", disabledLabelMap).
		Bool("enableLabel", enableLabel).
		Str("scope", scope).
		Logger()
	clog := &clogVal
	clog.Debug().Msg("Building container filter")

	stringBuilder := strings.Builder{}
	filter := NoFilter
	filter = FilterByNames(log, normalizedNames, filter)
	filter = FilterByDisableNames(log, normalizedDisableNames, filter)
	filter = filterByMonitoredImageNamePatterns(log, monitoredImageNamePatterns, filter)
	filter = filterBySkippedImageNamePatterns(log, skippedImageNamePatterns, filter)
	filter = filterByEnabledLabels(log, enabledLabelMap, filter)
	filter = filterByDisabledLabels(log, disabledLabelMap, filter)
	filter = filterByEnableLabel(enableLabel, filter)

	// Add name-based filter description.
	if len(normalizedNames) > 0 {
		stringBuilder.WriteString("which name matches \"")

		for i, n := range normalizedNames {
			stringBuilder.WriteString(n)

			if i < len(normalizedNames)-1 {
				stringBuilder.WriteString(`" or "`)
			}
		}

		stringBuilder.WriteString(`", `)
	}

	// Add disable-name-based filter description.
	if len(normalizedDisableNames) > 0 {
		stringBuilder.WriteString("not named one of \"")

		for i, n := range normalizedDisableNames {
			stringBuilder.WriteString(n)

			if i < len(normalizedDisableNames)-1 {
				stringBuilder.WriteString(`" or "`)
			}
		}

		stringBuilder.WriteString(`", `)
	}

	if len(monitoredImageNamePatterns) > 0 {
		stringBuilder.WriteString("which image matches \"")

		for i, n := range monitoredImageNamePatterns {
			stringBuilder.WriteString(n)

			if i < len(monitoredImageNamePatterns)-1 {
				stringBuilder.WriteString(`" or "`)
			}
		}

		stringBuilder.WriteString(`", `)
	}

	if len(skippedImageNamePatterns) > 0 {
		stringBuilder.WriteString("whose image is not one of \"")

		for i, n := range skippedImageNamePatterns {
			stringBuilder.WriteString(n)

			if i < len(skippedImageNamePatterns)-1 {
				stringBuilder.WriteString(`" or "`)
			}
		}

		stringBuilder.WriteString(`", `)
	}

	if enableLabel {
		stringBuilder.WriteString("with label ")
		stringBuilder.WriteString(enableLabelKey)
		stringBuilder.WriteString(`, `)
	}

	if len(enabledLabelMap) > 0 {
		stringBuilder.WriteString("with label ")

		first := true
		for key, value := range enabledLabelMap {
			if !first {
				stringBuilder.WriteString(" or ")
			}

			if value == "" {
				stringBuilder.WriteString(key)
			} else {
				stringBuilder.WriteString(key)
				stringBuilder.WriteString(`="`)
				stringBuilder.WriteString(value)
				stringBuilder.WriteString(`"`)
			}

			first = false
		}

		stringBuilder.WriteString(`, `)
	}

	if len(disabledLabelMap) > 0 {
		stringBuilder.WriteString("without label ")

		first := true
		for key, value := range disabledLabelMap {
			if !first {
				stringBuilder.WriteString(" or ")
			}

			if value == "" {
				stringBuilder.WriteString(key)
			} else {
				stringBuilder.WriteString(key)
				stringBuilder.WriteString(`="`)
				stringBuilder.WriteString(value)
				stringBuilder.WriteString(`"`)
			}

			first = false
		}

		stringBuilder.WriteString(`, `)
	}

	if scope == noScope || scope == "" {
		filter = FilterByScope(log, noScope, filter)

		stringBuilder.WriteString(`without a scope`)
	} else if scope != "" {
		filter = FilterByScope(log, scope, filter)

		stringBuilder.WriteString(`in scope `)
		stringBuilder.WriteString(scope)
	}

	// Exclude old Watchtower containers (predecessors from self-update).
	// Applied last so it wraps the entire chain and short-circuits first.
	filter = excludeOldWatchtowerFilterChain(filter)

	// Build filter description.
	filterDesc := "Checking all containers (except explicitly disabled with label)"
	if stringBuilder.Len() > 0 {
		filterDesc = "Only checking containers " + stringBuilder.String()
		// Trim trailing ", " if present (for name/enable filters)
		filterDesc = strings.TrimSuffix(filterDesc, ", ")
	}

	clog.Debug().
		Str("filter_desc", filterDesc).
		Msg("Filter built")

	return filter, filterDesc, nil
}
