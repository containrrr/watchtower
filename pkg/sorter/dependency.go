package sorter

import (
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// DependencySorter handles topological sorting by dependencies.
type DependencySorter struct{}

// Sort sorts containers in place by dependencies, placing Watchtower containers last.
//
// This function implements a two-phase sorting strategy to ensure proper update order:
//  1. Separate Watchtower containers from regular containers, as Watchtower instances
//     should always be updated last to avoid disrupting the update process itself.
//  2. Perform topological sorting on non-Watchtower containers using Kahn's algorithm
//     to respect dependency relationships (containers that depend on others must be
//     updated after their dependencies).
//
// The sorting ensures that:
//   - Dependent containers are updated after their dependencies
//   - Watchtower containers are processed last to maintain monitoring capability
//   - Circular dependencies are detected and reported as errors
//
// Time Complexity: O(V + E) where V is containers and E is dependency links
// Space Complexity: O(V + E) for graph structures
//
// Parameters:
//   - containers: Slice to sort in place. Modified directly.
//   - useComposeDependsOn: Whether to include Docker Compose depends_on label in dependency resolution.
//
// Returns:
//   - error: Non-nil if circular reference detected, nil on success.
//     Error includes the container name and cycle path for debugging.
func (ds DependencySorter) Sort(log *zerolog.Logger, containers []types.Container, useComposeDependsOn bool) error {
	if containers == nil {
		return nil
	}

	log.Debug().
		Int("container_count", len(containers)).
		Msg("Starting dependency sort")

	// Separate Watchtower containers from non-Watchtower containers
	var (
		nonWatchtowerContainers []types.Container
		watchtowerContainers    []types.Container
	)

	for _, c := range containers {
		if c.IsWatchtower() {
			watchtowerContainers = append(watchtowerContainers, c)
		} else {
			nonWatchtowerContainers = append(nonWatchtowerContainers, c)
		}
	}

	log.Debug().
		Int("non_watchtower_count", len(nonWatchtowerContainers)).
		Int("watchtower_count", len(watchtowerContainers)).
		Msg("Separated containers by Watchtower status")

	// Sort non-Watchtower containers by dependencies using internal sorter
	sortedNonWatchtower, err := sortByDependencies(log, nonWatchtowerContainers, useComposeDependsOn)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Dependency sort failed for non-Watchtower containers")

		return err
	}

	// Copy sorted results back to original slice
	copy(containers, sortedNonWatchtower)

	for i, wt := range watchtowerContainers {
		containers[len(sortedNonWatchtower)+i] = wt
	}

	sortedNames := make([]string, len(containers))
	for i, c := range containers {
		sortedNames[i] = c.Name()
	}

	log.Debug().
		Int("sorted_count", len(containers)).
		Strs("sorted_order", sortedNames).
		Msg("Completed dependency sort with Watchtower containers last")

	return nil
}

// sortByDependencies performs topological sort on containers using Kahn's algorithm.
//
// Kahn's algorithm is a breadth-first approach to topological sorting that works by:
// 1. Building a directed graph where nodes are containers and edges represent dependencies
// 2. Calculating indegree (number of incoming edges) for each node
// 3. Starting with nodes that have zero indegree (no dependencies)
// 4. Processing nodes in order, reducing indegree of dependents, and adding them to queue when indegree becomes zero
// 5. Detecting cycles if not all nodes are processed
//
// This implementation uses normalized container identifiers to handle Docker Compose service names
// and container names consistently. The queue is sorted in reverse alphabetical order to match
// the behavior of previous DFS-based implementations for consistency.
//
// Time Complexity: O(V + E) where V = number of containers, E = number of dependency links
// Space Complexity: O(V + E) for maps and adjacency lists
//
// Edge Cases:
//   - Empty container list: returns empty list
//   - Single container: returns that container
//   - Circular dependencies: detected and reported with cycle path
//   - Containers with no dependencies: processed first
//   - Missing dependency targets: ignored (only considers existing containers)
//
// Parameters:
//   - containers: List to sort. Should not be nil.
//
// Returns:
//   - []types.Container: Sorted list in dependency order (dependencies first).
//   - error: Non-nil if circular reference detected, nil on success.
//     Error includes container name and cycle path for debugging.
func sortByDependencies(log *zerolog.Logger, containers []types.Container, useComposeDependsOn bool) ([]types.Container, error) {
	// Phase 1: Build the dependency graph data structures
	containerMap, indegree, adjacency, normalizedMap, err := buildDependencyGraph(log, containers, useComposeDependsOn)
	if err != nil {
		return nil, err
	}

	return kahnSort(log, containers, containerMap, indegree, adjacency, normalizedMap)
}

// kahnSort orders the containers of a dependency graph with Kahn's algorithm,
// dependencies first. Ties are broken in reverse alphabetical order of the
// container identifiers, so the order is deterministic.
//
// Parameters:
//   - log: Process logger.
//   - containers: The containers of the graph.
//   - containerMap: Container for each identifier.
//   - indegree: Number of dependencies of each identifier. Consumed by the sort.
//   - adjacency: Dependents of each identifier.
//   - normalizedMap: Identifier of each container.
//
// Returns:
//   - []types.Container: Sorted list in dependency order (dependencies first).
//   - error: A CircularReferenceError when not every container can be ordered.
func kahnSort(log *zerolog.Logger,
	containers []types.Container,
	containerMap map[string]types.Container,
	indegree map[string]int,
	adjacency map[string][]string,
	normalizedMap map[types.Container]string,
) ([]types.Container, error) {
	// Phase 2: Initialize processing queue with containers that have no dependencies
	queue := initializeQueue(indegree)

	// Phase 3: Process the queue using Kahn's algorithm
	// While there are containers with no remaining dependencies:
	//   - Remove a container from queue and add it to sorted result
	//   - For each container that depends on it, decrement their indegree
	//   - If a dependent's indegree becomes 0, add it to queue
	sorted := make([]types.Container, 0, len(containers))
	for len(queue) > 0 {
		// Dequeue the next container with no dependencies
		current := queue[0]
		queue = queue[1:]

		// Add to sorted result - this container can be updated now
		sorted = append(sorted, containerMap[current])
		log.Debug().
			Str("container_id", containerMap[current].ID().ShortID()).
			Str("name", containerMap[current].Name()).
			Msg("Added container to sorted list")

		// Update all containers that depend on this one
		for _, dependent := range adjacency[current] {
			indegree[dependent]--
			// If this dependent now has no remaining dependencies, add to queue
			if indegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Phase 4: Cycle detection
	err := detectAndReportCycle(log,
		sorted,
		containers,
		containerMap,
		adjacency,
		normalizedMap,
	)
	if err != nil {
		return nil, err
	}

	return sorted, nil
}

// buildDependencyGraph constructs the dependency graph data structures for topological sorting.
//
// This function builds three key data structures:
//   - containerMap: Maps canonical normalized identifiers to container objects for O(1) lookup
//   - indegree: Tracks number of incoming dependencies for each container (nodes with indegree 0 have no dependencies)
//   - adjacency: Lists containers that depend on each container (outgoing edges and keys are canonical)
//
// Graph nodes are always keyed by ResolveContainerIdentifier (project-service, service, or name).
// Links from c.Links() may use Watchtower depends-on container names, Compose service names,
// Docker links, or network_mode targets (including explicit container_name values). Those
// strings often differ from the canonical graph key when Compose labels are present.
//
// Link Matching Strategy:
// Matching uses findMatchingIdentifiersInSet against the canonical keys plus unique bare
// Name() aliases, then maps every hit back to a canonical key before recording edges.
// Strategies:
//  1. Exact match on canonical identifier or bare container name
//  2. Prefix match for Docker Compose replica suffixes (e.g., "db" matches "db-1", "db-2")
//  3. Project-qualified suffix match (id ends with "-"+link) for multi-segment links, or
//     ExtractServiceName equality only for unhyphenated bare service names
//
// Parameters:
//   - containers: List of containers to build graph for.
//   - useComposeDependsOn: Whether Links() should include Compose depends_on labels.
//
// Returns:
//   - map[string]types.Container: Container lookup map (canonical keys only)
//   - map[string]int: Indegree count for each container
//   - map[string][]string: Adjacency list (canonical dependency -> dependents)
//   - map[types.Container]string: Reverse map from container to normalized identifier
//   - error: IdentifierCollisionError if duplicate identifiers detected, nil otherwise
func buildDependencyGraph(log *zerolog.Logger,
	containers []types.Container,
	useComposeDependsOn bool,
) (map[string]types.Container, map[string]int, map[string][]string, map[types.Container]string, error) {
	graph, err := newDependencyGraph(
		log,
		containers,
		nil,
		nil,
		useComposeDependsOn,
		false,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return graph.subgraph(containers)
}

// identifierIndex maps each container's normalized dependency identifier to
// the container.
//
// Parameters:
//   - log: Process logger.
//   - containers: Containers to index.
//
// Returns:
//   - map[string]types.Container: Container for each normalized identifier.
//   - error: IdentifierCollisionError if containers share an identifier.
func identifierIndex(log *zerolog.Logger, containers []types.Container) (map[string]types.Container, error) {
	// Use temporary map to collect all containers per normalized identifier
	tempMap := make(map[string][]types.Container, len(containers))

	for _, c := range containers {
		normalizedIdentifier := util.NormalizeContainerName(container.ResolveContainerIdentifier(c))
		tempMap[normalizedIdentifier] = append(tempMap[normalizedIdentifier], c)
	}

	index := make(map[string]types.Container, len(tempMap))

	// Check for identifier collisions
	for identifier, dupContainers := range tempMap {
		if len(dupContainers) > 1 {
			log.Error().Msgf(
				"Identifier collision detected: '%s' used by multiple containers: %v",
				identifier,
				dupContainers,
			)

			return nil, IdentifierCollisionError{
				DuplicateIdentifier: identifier,
				AffectedContainers:  dupContainers,
			}
		}

		index[identifier] = dupContainers[0]
	}

	return index, nil
}

// ContainerDependencies maps each container to the containers in the set that it depends on.
//
// Link matching is the same graph used by SortByDependencies. Callers that group
// containers into larger units can order those units from this map.
//
// Parameters:
//   - log: Process logger. Required and must be non-nil.
//   - containers: Containers to inspect.
//   - useComposeDependsOn: Whether Links() should include Compose depends_on labels.
//
// Returns:
//   - map[types.ContainerID][]types.ContainerID: Dependent ID to dependency IDs.
//   - error: Non-nil when dependency identifiers collide.
func ContainerDependencies(
	log *zerolog.Logger,
	containers []types.Container,
	useComposeDependsOn bool,
) (map[types.ContainerID][]types.ContainerID, error) {
	if len(containers) == 0 {
		return map[types.ContainerID][]types.ContainerID{}, nil
	}

	containerMap, _, adjacency, _, err := buildDependencyGraph(log, containers, useComposeDependsOn)
	if err != nil {
		return nil, err
	}

	dependencies := make(map[types.ContainerID][]types.ContainerID, len(containers))
	seen := make(map[types.ContainerID]map[types.ContainerID]struct{}, len(containers))

	for dependencyKey, dependents := range adjacency {
		dependency, ok := containerMap[dependencyKey]
		if !ok || dependency == nil {
			continue
		}

		dependencyID := dependency.ID()

		for _, dependentKey := range dependents {
			dependent, ok := containerMap[dependentKey]
			if !ok || dependent == nil {
				continue
			}

			dependentID := dependent.ID()
			if dependentID == "" || dependentID == dependencyID {
				continue
			}

			if seen[dependentID] == nil {
				seen[dependentID] = make(map[types.ContainerID]struct{})
			}

			if _, exists := seen[dependentID][dependencyID]; exists {
				continue
			}

			seen[dependentID][dependencyID] = struct{}{}
			dependencies[dependentID] = append(dependencies[dependentID], dependencyID)
		}
	}

	return dependencies, nil
}

// buildLinkMatchIndexes builds the identifier set and alias→canonical map used when
// resolving dependency links against graph nodes.
//
// Each canonical ResolveContainerIdentifier is always included and never overwritten.
// Bare container names are registered as aliases only when they uniquely identify one
// container and do not collide with another container's canonical key, so ambiguous
// names do not create non-deterministic edges. Docker IDs are also registered because
// inspect stores network_mode as container:<id> until Watchtower rewrites it to a name.
//
// Parameters:
//   - containerMap: Canonical identifier → container map from graph construction.
//
// Returns:
//   - map[string]bool: Precomputed set of matchable identifiers (canonical keys and unique bare names).
//   - map[string]string: Maps every matchable identifier to its canonical graph key.
func buildLinkMatchIndexes(log *zerolog.Logger,
	containerMap map[string]types.Container,
) (map[string]bool, map[string]string) {
	// Capacity covers canonical key, optional bare-name alias, and Docker ID per container.
	const aliasCapacityFactor = 3

	aliasToCanonical := make(map[string]string, len(containerMap)*aliasCapacityFactor)

	// Canonical ResolveContainerIdentifier keys always map to themselves and must never
	// be removed or overwritten by bare-name alias cleanup.
	for identifier := range containerMap {
		aliasToCanonical[identifier] = identifier
	}

	// bareOwners collects distinct canonical owners for each bare container name.
	// A bare name becomes an alias only when exactly one owner claims it and the name
	// does not collide with a different container's canonical graph key.
	bareOwners := make(map[string]map[string]struct{})

	for identifier, c := range containerMap {
		if concrete, ok := c.(*container.Container); ok {
			containerID := string(concrete.ID())
			if containerID != "" && containerID != identifier {
				if _, exists := aliasToCanonical[containerID]; !exists {
					aliasToCanonical[containerID] = identifier
				}
			}
		}

		bareName := util.NormalizeContainerName(c.Name())
		if bareName == "" || bareName == identifier {
			continue
		}

		// Never use a bare name that is already another container's canonical key.
		_, isCanonicalKey := containerMap[bareName]
		if isCanonicalKey {
			continue
		}

		owners, ok := bareOwners[bareName]
		if !ok {
			owners = make(map[string]struct{})
			bareOwners[bareName] = owners
		}

		owners[identifier] = struct{}{}
	}

	for bareName, owners := range bareOwners {
		if len(owners) != 1 {
			log.Debug().
				Str("bare_name", bareName).
				Msg("Skipped ambiguous bare container name alias for dependency matching")

			continue
		}

		if _, exists := aliasToCanonical[bareName]; exists {
			continue
		}

		for owner := range owners {
			aliasToCanonical[bareName] = owner
		}
	}

	matchIDSet := make(map[string]bool, len(aliasToCanonical))
	for id := range aliasToCanonical {
		matchIDSet[id] = true
	}

	return matchIDSet, aliasToCanonical
}

// resolveLinkToCanonicalKeys resolves a single dependency link to sorted unique canonical
// graph keys using the precomputed identifier set and alias index.
//
// Parameters:
//   - link: Normalized link from Container.Links().
//   - matchIDSet: Indexed set of matchable identifiers (canonical + unique bare names).
//   - aliasToCanonical: Maps match hits to canonical graph keys.
//
// Returns:
//   - []string: Sorted unique canonical identifiers the link refers to (empty if none).
func resolveLinkToCanonicalKeys(
	link string,
	matchIDSet *matchSet,
	aliasToCanonical map[string]string,
) []string {
	if link == "" {
		return nil
	}

	matches := matchIDSet.match(link)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(matches))
	canonicalKeys := make([]string, 0, len(matches))

	for _, match := range matches {
		canonical, ok := aliasToCanonical[match]
		if !ok || canonical == "" || seen[canonical] {
			continue
		}

		seen[canonical] = true
		canonicalKeys = append(canonicalKeys, canonical)
	}

	sort.Strings(canonicalKeys)

	return canonicalKeys
}

// IsPositiveInteger checks if a string represents a positive integer (1 or greater).
//
// This validation is critical for distinguishing Docker Compose-style replica suffixes
// (e.g., "db-1", "db-2") from other hyphenated container names (e.g., "db-backup",
// "db-temp"). Docker Compose uses sequential positive integers starting from 1 to
// identify replica instances. By requiring a positive integer suffix, we ensure that:
//
//   - "db" correctly matches "db-1" and "db-2" as replicas
//   - "db" does NOT match "dbase" (no hyphen) or "db-backup" (non-numeric suffix)
//   - "database" does NOT match "database2" (no separator)
//
// This prevents false dependency relationships between unrelated containers with
// similar names, which could cause incorrect update ordering or circular dependencies.
func IsPositiveInteger(s string) bool {
	if s == "" {
		return false
	}

	n, err := strconv.Atoi(s)

	return err == nil && n > 0
}

// ExtractServiceName extracts the service name from a container identifier.
//
// Container identifiers from ResolveContainerIdentifier() follow the pattern:
//   - "project-service" when both project and service labels exist
//   - "project-service-N" for Docker Compose replicas (N is replica number)
//   - "servicename" when only service name is available (no project context)
//
// This function extracts just the service name by:
//  1. If there's no hyphen, return the whole string (it's already just a service name)
//  2. If there are hyphens, the service name is the last segment (or last two segments
//     if the last segment is a replica number)
//
// Examples:
//   - "postgresql-postgres" -> "postgres"
//   - "postgresql-postgres-1" -> "postgres" (strips replica suffix)
//   - "myapp" -> "myapp"
//   - "my-app-service" -> "service" (last segment before any replica number)
//   - "my-app-service-2" -> "service" (strips replica suffix)
func ExtractServiceName(identifier string) string {
	if identifier == "" {
		return ""
	}

	// If no hyphen, the identifier is already just a service name
	if !strings.Contains(identifier, "-") {
		return identifier
	}

	last := strings.LastIndex(identifier, "-")

	// Check if the last part is a replica number (positive integer)
	// If so, we need to skip it and take the second-to-last part as service name
	if IsPositiveInteger(identifier[last+1:]) {
		// Return the part before the replica number (e.g., "service" from "project-service-1")
		rest := identifier[:last]

		return rest[strings.LastIndex(rest, "-")+1:]
	}

	// No replica number, service name is the last part
	return identifier[last+1:]
}

// FindMatchingIdentifiers returns the identifiers from the given list that match
// the provided link. It applies the same strategies used when building the
// dependency graph:
//
//  1. Exact match.
//  2. Replica prefix match: the identifier starts with "<link>-" and the suffix
//     after the hyphen is a positive integer (Docker Compose replica numbering).
//  3. Project-qualified suffix match (the identifier, without a replica
//     number, ends with "-"+link), and for unhyphenated links only,
//     ExtractServiceName equality on both sides. This strategy only succeeds
//     when every candidate belongs to the same service, and then returns all
//     of its replicas. Candidates from more than one service (e.g. the same
//     service name in different projects) are treated as ambiguous and return
//     no results.
//
// Parameters:
//   - link: Dependency link to resolve (typically from Container.Links()).
//   - identifiers: List of known container identifiers to search within.
//
// Returns:
//   - []string: Matching identifiers. Returns nil or an empty slice when there
//     is no match or when the service-only strategy finds more than one service.
func FindMatchingIdentifiers(link string, identifiers []string) []string {
	if link == "" || len(identifiers) == 0 {
		return nil
	}

	idSet := make(map[string]bool, len(identifiers))
	for _, id := range identifiers {
		idSet[id] = true
	}

	return findMatchingIdentifiersInSet(link, idSet)
}

// findMatchingIdentifiersInSet returns identifiers from a set that match the
// provided link. See FindMatchingIdentifiers for matching strategy details.
// Callers that match many links against one set should build a matchSet once.
//
// Parameters:
//   - link: Dependency link to resolve.
//   - idSet: Set of known container identifiers.
//
// Returns:
//   - []string: Matching identifiers, or nil/empty when none or ambiguous.
func findMatchingIdentifiersInSet(link string, idSet map[string]bool) []string {
	return newMatchSet(idSet).match(link)
}

// matchSet is a set of container identifiers indexed for link matching.
//
// The identifiers are kept in sorted order, so the replicas of a link are one
// contiguous range, together with the replica-free service and service name of
// each identifier, so matching a link computes nothing per identifier.
type matchSet struct {
	// names holds every identifier.
	names map[string]bool
	// sorted lists the identifiers in sorted order.
	sorted []string
	// services holds the identifier of sorted[i] without its replica number.
	services []string
	// serviceNames holds ExtractServiceName of sorted[i].
	serviceNames []string
}

// newMatchSet indexes a set of container identifiers for link matching.
//
// Parameters:
//   - idSet: Set of known container identifiers.
//
// Returns:
//   - *matchSet: The indexed set.
func newMatchSet(idSet map[string]bool) *matchSet {
	sorted := slices.Sorted(maps.Keys(idSet))
	services := make([]string, len(sorted))
	serviceNames := make([]string, len(sorted))

	for i, identifier := range sorted {
		services[i] = replicaService(identifier)
		serviceNames[i] = ExtractServiceName(identifier)
	}

	return &matchSet{names: idSet, sorted: sorted, services: services, serviceNames: serviceNames}
}

// match returns the identifiers of the set that match the provided link. See
// FindMatchingIdentifiers for matching strategy details.
//
// Parameters:
//   - link: Dependency link to resolve.
//
// Returns:
//   - []string: Matching identifiers in sorted order, or nil when none match or
//     the service-only strategy finds more than one service.
func (s *matchSet) match(link string) []string {
	if s == nil || link == "" || len(s.sorted) == 0 {
		return nil
	}

	// 1. Exact match
	if s.names[link] {
		return []string{link}
	}

	// 2. Replica prefix match (only if suffix is positive integer). Identifiers
	// starting with the prefix are contiguous in sorted order.
	prefix := link + "-"

	var replicaMatches []string

	for i := sort.SearchStrings(s.sorted, prefix); i < len(s.sorted); i++ {
		identifier := s.sorted[i]
		if !strings.HasPrefix(identifier, prefix) {
			break
		}

		if IsPositiveInteger(identifier[len(prefix):]) {
			replicaMatches = append(replicaMatches, identifier)
		}
	}

	if len(replicaMatches) > 0 {
		return replicaMatches
	}

	// 3. Project-qualified / service-only match: the link names exactly one
	// service, and resolves to every replica of it. Candidates from more than
	// one service are ambiguous and resolve to nothing.
	// Multi-segment links (containing "-") only use the precise "-"+link suffix,
	// ignoring a replica number, so trailing-token ExtractServiceName equality
	// cannot select an unrelated peer (e.g. link "net-proxy" must not match
	// "myproject-other-proxy").
	// Unhyphenated bare service names may still match via ExtractServiceName
	// (e.g. link "db" → "myproject-db" or "myproject-db-1").
	serviceSuffix := "-" + link
	linkHasHyphen := strings.Contains(link, "-")

	var (
		serviceMatches []string
		matchedService string
	)

	for i, identifier := range s.sorted {
		service := s.services[i]

		if !strings.HasSuffix(identifier, serviceSuffix) && !strings.HasSuffix(service, serviceSuffix) &&
			(linkHasHyphen || s.serviceNames[i] != link) {
			continue
		}

		if serviceMatches != nil && service != matchedService {
			return nil
		}

		matchedService = service

		serviceMatches = append(serviceMatches, identifier)
	}

	return serviceMatches
}

// replicaService returns the service an identifier belongs to: the identifier
// without a trailing replica number (e.g. "project-db" for "project-db-2").
//
// Parameters:
//   - identifier: A container identifier.
//
// Returns:
//   - string: The identifier without its replica number, or the identifier
//     itself when it has none.
func replicaService(identifier string) string {
	i := strings.LastIndex(identifier, "-")
	if i > 0 && IsPositiveInteger(identifier[i+1:]) {
		return identifier[:i]
	}

	return identifier
}

// initializeQueue creates the initial processing queue for Kahn's algorithm.
//
// This function identifies all containers with no dependencies (indegree 0) and
// sorts them in reverse alphabetical order to ensure deterministic ordering
// when multiple containers have zero indegree. This provides consistent,
// reproducible sorting order for containers with no dependencies.
//
// Parameters:
//   - indegree: Map of container identifiers to their indegree count.
//
// Returns:
//   - []string: Sorted queue of container identifiers with zero indegree.
func initializeQueue(indegree map[string]int) []string {
	var queue []string

	for identifier, deg := range indegree {
		if deg == 0 {
			queue = append(queue, identifier)
		}
	}

	// Sort queue in reverse alphabetical order to ensure deterministic ordering
	sort.Sort(sort.Reverse(sort.StringSlice(queue)))

	return queue
}

// detectAndReportCycle checks for circular dependencies and reports detailed error information.
//
// This function implements cycle detection for Kahn's algorithm. If not all containers
// were processed during topological sorting, there must be a cycle in the dependency graph.
// It identifies unprocessed containers, selects the first one, and uses DFS to find
// the actual cycle path for detailed error reporting.
//
// Parameters:
//   - sorted: List of successfully sorted containers
//   - containers: Original list of all containers
//   - containerMap: Map of container identifiers to container objects
//   - adjacency: Graph adjacency list (container -> dependents)
//   - normalizedMap: Map of containers to their normalized identifiers
//
// Returns:
//   - error: CircularReferenceError if cycle detected, nil otherwise
func detectAndReportCycle(log *zerolog.Logger,
	sorted []types.Container,
	containers []types.Container,
	containerMap map[string]types.Container,
	adjacency map[string][]string,
	normalizedMap map[types.Container]string,
) error {
	if len(sorted) != len(containers) {
		// Identify which containers were not processed (part of cycles)
		processed := make(map[string]bool)

		for _, c := range sorted {
			normalizedIdentifier := normalizedMap[c]
			processed[normalizedIdentifier] = true
		}

		var unprocessed []string

		for id := range containerMap {
			if !processed[id] {
				unprocessed = append(unprocessed, id)
			}
		}
		// Sort for deterministic error reporting
		sort.Strings(unprocessed)

		// Find and report cycle details for the first unprocessed container
		cycleContainer := ""

		var cyclePath []string

		if len(unprocessed) > 0 {
			cycleContainer = unprocessed[0]
			// Use DFS to find the actual cycle path starting from this container
			visited := make(map[string]bool)
			cyclePath = findCyclePath(cycleContainer, adjacency, visited)
		}

		log.Debug().
			Str("cycle_container", cycleContainer).
			Strs("cycle_path", cyclePath).
			Msg("Detected circular reference in dependency graph")

		// Return detailed error with cycle information for debugging
		return CircularReferenceError{ContainerName: cycleContainer, CyclePath: cyclePath}
	}

	return nil
}

// findCyclePath performs DFS to find a cycle path starting from the given node.
//
// This function implements cycle detection using Depth-First Search with three states:
//   - visited: nodes that have been fully explored (no cycles through them)
//   - visiting: nodes currently in the recursion stack (potential cycle)
//
// When a node is encountered that is already in 'visiting' state, a cycle is detected.
// The path from the current node back to the start of the cycle is returned.
//
// Algorithm:
// 1. Start DFS from given node, tracking current path
// 2. Mark node as visiting when entering
// 3. Recurse on neighbors
// 4. If neighbor is visiting, extract cycle path from current path
// 5. Mark node as visited when backtracking
//
// Time Complexity: O(V + E) in worst case, but typically much faster for cycle detection
// Space Complexity: O(V) for recursion stack and maps
//
// Parameters:
//   - start: Container identifier to start cycle detection from
//   - adjacency: Graph adjacency list (container -> list of containers that depend on it)
//   - visited: Map tracking fully explored nodes (modified in place)
//
// Returns:
//   - []string: Cycle path if found (empty slice if no cycle)
//     Path includes the starting node at both ends to show the cycle
func findCyclePath(start string, adjacency map[string][]string, visited map[string]bool) []string {
	visiting := make(map[string]bool) // Nodes currently in recursion stack

	// Recursive DFS function to detect cycles
	var dfs func(string, []string) []string

	dfs = func(node string, path []string) []string {
		// If node is already in visiting set, we found a back edge (cycle)
		if visiting[node] {
			// Extract the cycle path: from first occurrence of node to current position
			idx := -1

			for i, p := range path {
				if p == node {
					idx = i

					break
				}
			}

			if idx >= 0 {
				// Return path segment that forms the cycle, including node at both ends
				return append(path[idx:], node)
			}

			return nil
		}

		// If already fully visited, no cycle through this path
		if visited[node] {
			return nil
		}

		// Mark as visiting and add to current path
		visiting[node] = true
		path = append(path, node)

		// Explore all neighbors (containers that depend on this one)
		for _, neighbor := range adjacency[node] {
			cycle := dfs(neighbor, path)
			if cycle != nil {
				return cycle // Propagate cycle up the call stack
			}
		}

		// Backtrack: remove from path and visiting set, mark as fully visited
		visiting[node] = false
		visited[node] = true

		return nil // No cycle found from this path
	}

	return dfs(start, []string{})
}
