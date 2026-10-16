// Package sorter provides sorting functionality for Watchtower containers.
// It implements dependency-based topological sorting.
//
// Key components:
//   - SortByDependencies: Sorts containers in place by links, detecting circular references.
//
// Usage example:
//
//	// log is the process *zerolog.Logger.
//	err := sorter.SortByDependencies(log, containers, useComposeDependsOn)
//	if err != nil {
//	    log.Error().Err(err).Msg("Dependency sort failed")
//	}
//
// The package uses zerolog for logging sort operations and errors.
package sorter
