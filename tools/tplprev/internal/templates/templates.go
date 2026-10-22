package templates

import (
	"maps"
	"slices"
)

// Lookup returns a named builtin template.
//
// Parameters:
//   - name: Builtin template name.
//
// Returns:
//   - string: Template source when found.
//   - bool: True when name is a known builtin.
func Lookup(name string) (string, bool) {
	tpl, found := Templates[name]

	return tpl, found
}

// Names returns the builtin template names in sorted order.
//
// Returns:
//   - []string: Sorted builtin names.
func Names() []string {
	return slices.Sorted(maps.Keys(Templates))
}
