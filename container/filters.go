package container

// A Filter is a prototype for a function that can be used to filter the
// results from a call to the ListContainers() method on the Client.
type Filter func(FilterableContainer) bool

// A FilterableContainer is the interface which is used to filter
// containers.
type FilterableContainer interface {
	Name() string
	IsWatchtower() bool
	Enabled() (bool, bool)
}

// WatchtowerContainersFilter filters only watchtower containers
func WatchtowerContainersFilter(c FilterableContainer) bool { return c.IsWatchtower() }

// Filter no containers and returns all
func noFilter(FilterableContainer) bool { return true }

// Filters containers which don't have a specified name
func filterByNames(names []string, baseFilter Filter) Filter {
	if len(names) == 0 {
		return baseFilter
	}

	return func(c FilterableContainer) bool {
		for _, name := range names {
			if (name == c.Name()) || (name == c.Name()[1:]) {
				return baseFilter(c)
			}
		}
		return false
	}
}

// Filters out containers that don't have the 'enableLabel'
func filterByEnableLabel(baseFilter Filter) Filter {
	return func(c FilterableContainer) bool {
		// If label filtering is enabled, containers should only be considered
		// if the label is specifically set.
		_, ok := c.Enabled()
		if !ok {
			return false
		}

		return baseFilter(c)
	}
}

// Filters out containers that have a 'enableLabel' and is set to disable.
func filterByDisabledLabel(baseFilter Filter) Filter {
	return func(c FilterableContainer) bool {
		enabledLabel, ok := c.Enabled()
		if ok && !enabledLabel {
			// If the label has been set and it demands a disable
			return false
		}

		return baseFilter(c)
	}
}

// BuildFilter creates the needed filter of containers
func BuildFilter(names []string, enableLabel bool) Filter {
	filter := noFilter
	filter = filterByNames(names, filter)
	if enableLabel {
		// If label filtering is enabled, containers should only be considered
		// if the label is specifically set.
		filter = filterByEnableLabel(filter)
	}
	filter = filterByDisabledLabel(filter)
	return filter
}
