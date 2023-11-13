package filters

import (
	"regexp"
	"strings"

	t "github.com/containrrr/watchtower/pkg/types"
)

// WatchtowerContainersFilter filters only watchtower containers
func WatchtowerContainersFilter(c t.FilterableContainer) bool { return c.IsWatchtower() }

// NoFilter will not filter out any containers
func NoFilter(t.FilterableContainer) bool { return true }

// FilterByNames returns all containers that match one of the specified names
func FilterByNames(names []string, baseFilter t.Filter) t.Filter {
	if len(names) == 0 {
		return baseFilter
	}

	return func(c t.FilterableContainer) bool {
		for _, name := range names {
			if name == c.Name() || name == c.Name()[1:] {
				return baseFilter(c)
			}

			if re, err := regexp.Compile(name); err == nil {
				indices := re.FindStringIndex(c.Name())
				if indices == nil {
					continue
				}
				start := indices[0]
				end := indices[1]
				if start <= 1 && end >= len(c.Name())-1 {
					return baseFilter(c)
				}
			}
		}
		return false
	}
}

// FilterByDisableNames returns all containers that don't match any of the specified names
func FilterByDisableNames(disableNames []string, baseFilter t.Filter) t.Filter {
	if len(disableNames) == 0 {
		return baseFilter
	}

	return func(c t.FilterableContainer) bool {
		for _, name := range disableNames {
			if name == c.Name() || name == c.Name()[1:] {
				return false
			}
		}
		return baseFilter(c)
	}
}

// FilterByEnableLabel returns all containers that have the enabled label set
func FilterByEnableLabel(baseFilter t.Filter) t.Filter {
	return func(c t.FilterableContainer) bool {
		// If label filtering is enabled, containers should only be considered
		// if the label is specifically set.
		_, ok := c.Enabled()
		if !ok {
			return false
		}

		return baseFilter(c)
	}
}

// FilterByDisabledLabel returns all containers that have the enabled label set to disable
func FilterByDisabledLabel(baseFilter t.Filter) t.Filter {
	return func(c t.FilterableContainer) bool {
		enabledLabel, ok := c.Enabled()
		if ok && !enabledLabel {
			// If the label has been set and it demands a disable
			return false
		}

		return baseFilter(c)
	}
}

// FilterByScope returns all containers that belongs to a specific scope
func FilterByScope(scope string, baseFilter t.Filter) t.Filter {
	return func(c t.FilterableContainer) bool {
		containerScope, containerHasScope := c.Scope()

		if !containerHasScope || containerScope == "" {
			containerScope = "none"
		}

		if containerScope == scope {
			return baseFilter(c)
		}

		return false
	}
}

// FilterByImage returns all containers that have a specific image
func FilterByImage(images []string, baseFilter t.Filter) t.Filter {
	if images == nil {
		return baseFilter
	}

	return func(c t.FilterableContainer) bool {
		image := strings.Split(c.ImageName(), ":")[0]
		for _, targetImage := range images {
			if image == targetImage {
				return baseFilter(c)
			}
		}

		return false
	}
}

// BuildFilter creates the needed filter of containers
func BuildFilter(names []string, disableNames []string, enableLabel bool, scope string) (t.Filter, string) {
	sb := strings.Builder{}
	filter := NoFilter
	filter = FilterByNames(names, filter)
	filter = FilterByDisableNames(disableNames, filter)

	if len(names) > 0 {
		sb.WriteString("which name matches \"")
		for i, n := range names {
			sb.WriteString(n)
			if i < len(names)-1 {
				sb.WriteString(`" or "`)
			}
		}
		sb.WriteString(`", `)
	}
	if len(disableNames) > 0 {
		sb.WriteString("not named one of \"")
		for i, n := range disableNames {
			sb.WriteString(n)
			if i < len(disableNames)-1 {
				sb.WriteString(`" or "`)
			}
		}
		sb.WriteString(`", `)
	}

	if enableLabel {
		// If label filtering is enabled, containers should only be considered
		// if the label is specifically set.
		filter = FilterByEnableLabel(filter)
		sb.WriteString("using enable label, ")
	}

	if scope == "none" {
		// If a scope has explicitly defined as "none", containers should only be considered
		// if they do not have a scope defined, or if it's explicitly set to "none".
		filter = FilterByScope(scope, filter)
		sb.WriteString(`without a scope, "`)
	} else if scope != "" {
		// If a scope has been defined, containers should only be considered
		// if the scope is specifically set.
		filter = FilterByScope(scope, filter)
		sb.WriteString(`in scope "`)
		sb.WriteString(scope)
		sb.WriteString(`", `)
	}
	filter = FilterByDisabledLabel(filter)

	filterDesc := "Checking all containers (except explicitly disabled with label)"
	if sb.Len() > 0 {
		filterDesc = "Only checking containers " + sb.String()

		// Remove the last ", "
		filterDesc = filterDesc[:len(filterDesc)-2]
	}

	return filter, filterDesc
}
