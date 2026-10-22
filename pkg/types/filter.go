package types

// Filter defines a function to filter containers.
//
// Parameters:
//   - c: Container to evaluate.
//
// Returns:
//   - bool: True if container passes filter, false otherwise.
type Filter func(FilterableContainer) bool
