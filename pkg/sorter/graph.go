package sorter

import (
	"fmt"
	"slices"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// DependencyGraph is the dependency graph of a set of containers, with every
// container's links resolved once when the graph is built.
//
// Links are matched to containers as SortByDependencies matches them,
// including Compose service names, replica names, container-name aliases, and
// Docker IDs. Sorting a subset of the containers keeps only the dependencies
// between the containers in the subset, so a link always names the same
// container whichever subset is sorted, and a subset of containers outside
// every cycle always sorts.
//
// Watchtower containers are not part of the graph and are always sorted last,
// but the graph still records which containers depend on them, and which
// containers they depend on, for Dependents. A link that names an unmonitored
// container exactly resolves to nothing, rather than to a different container
// with a similar name.
//
// A service name shared by several Compose projects names the service of the
// linking container's own project when it has one, and every monitored one of
// them otherwise. When the service of its own project is unmonitored, the link
// resolves to nothing.
//
// A graph reflects the labels and host config of its containers when it is
// built. Build a new graph after they change.
type DependencyGraph struct {
	// containers lists the containers of the graph in the order given.
	containers []types.Container
	// identifiers maps each container to its normalized dependency identifier.
	identifiers map[types.Container]string
	// dependencies lists, for each container, the identifiers its links
	// resolve to, in link order. A link to the container itself is left out.
	dependencies map[types.Container][]string
	// dependents lists, for each container, the containers whose links name
	// it: members in the order given, then Watchtower containers.
	dependents map[types.Container][]types.Container
	// unresolved lists the depends-on label entries that name no container.
	unresolved []UnresolvedLink
}

// UnresolvedLink is a Watchtower depends-on label entry that names no
// container.
type UnresolvedLink struct {
	// Container is the container whose label holds the entry.
	Container types.Container
	// Link is the entry, normalized as the dependency resolution reads it.
	Link string
}

// NewDependencyGraph builds the dependency graph of containers.
//
// Watchtower containers are set aside, as SortByDependencies sets them aside,
// so links never resolve to them for ordering. Their own links still resolve,
// and a link that names one exactly by name, identifier, or ID, or without its
// replica number when it is the only such Watchtower container and no
// unmonitored container has that name, makes the linking container one of its
// dependents.
//
// Unmonitored containers are not part of the graph. A link that names one
// exactly resolves to nothing, so it is not matched to a monitored container
// instead.
//
// Parameters:
//   - log: Process logger.
//   - containers: Monitored containers to build the graph of.
//   - unmonitored: Other containers that links may name.
//   - useComposeDependsOn: Whether to include Compose depends_on labels as links.
//
// Returns:
//   - *DependencyGraph: The graph.
//   - error: An IdentifierCollisionError when monitored containers other than
//     Watchtower containers share an identifier.
func NewDependencyGraph(
	log *zerolog.Logger,
	containers, unmonitored []types.Container,
	useComposeDependsOn bool,
) (*DependencyGraph, error) {
	members := make([]types.Container, 0, len(containers))
	watchtowers := make([]types.Container, 0, 1)

	for _, c := range containers {
		if c.IsWatchtower() {
			watchtowers = append(watchtowers, c)
		} else {
			members = append(members, c)
		}
	}

	return newDependencyGraph(log, members, watchtowers, unmonitored, useComposeDependsOn)
}

// newDependencyGraph builds the dependency graph of members.
//
// Parameters:
//   - log: Process logger.
//   - members: Containers of the graph.
//   - watchtowers: Monitored Watchtower containers, outside the graph.
//   - unmonitored: Containers that links may name exactly but that are
//     outside the graph.
//   - useComposeDependsOn: Whether to include Compose depends_on labels as links.
//
// Returns:
//   - *DependencyGraph: The graph.
//   - error: An IdentifierCollisionError when members share an identifier.
func newDependencyGraph(
	log *zerolog.Logger,
	members, watchtowers, unmonitored []types.Container,
	useComposeDependsOn bool,
) (*DependencyGraph, error) {
	containerMap, err := identifierIndex(log, members)
	if err != nil {
		return nil, err
	}

	identifiers := make(map[types.Container]string, len(containerMap))
	for identifier, c := range containerMap {
		identifiers[c] = identifier
	}

	// Lookup identifiers for link resolution: canonical keys plus unique bare names.
	// Docs specify Watchtower depends-on and network_mode targets use container names,
	// while Compose depends_on uses service names. Aliases bridge those forms to the
	// canonical project-service graph keys without inventing extra Kahn nodes.
	matchIDSet, aliasToCanonical := buildLinkMatchIndexes(log, containerMap)

	watchtowerExact := exactNames(watchtowers)

	resolver := &linkResolver{
		matchIDSet:        matchIDSet,
		aliasToCanonical:  aliasToCanonical,
		projects:          memberProjects(containerMap),
		services:          memberServices(containerMap),
		watchtowers:       watchtowerExact,
		unmonitored:       exactNames(unmonitored),
		unmonitoredNames:  projectNames(unmonitored),
		watchtowerAliases: watchtowerAliases(watchtowers, watchtowerExact),
	}

	graph := &DependencyGraph{
		containers:   members,
		identifiers:  identifiers,
		dependencies: make(map[types.Container][]string, len(members)),
		dependents:   make(map[types.Container][]types.Container, len(members)),
	}

	for _, c := range slices.Concat(members, watchtowers) {
		self, isMember := identifiers[c]

		for _, normalizedLink := range c.Links(useComposeDependsOn) {
			keys, watchtower, _ := resolver.resolve(c, normalizedLink)
			if watchtower != nil && watchtower != c {
				graph.addDependent(watchtower, c)
			}

			for _, key := range keys {
				if key == self {
					// Self-reference: skip so the container stays indegree 0 for this link.
					continue
				}

				if isMember {
					graph.dependencies[c] = append(graph.dependencies[c], key)
				}

				graph.addDependent(containerMap[key], c)
			}
		}

		graph.unresolved = append(graph.unresolved, resolver.unresolvedDependsOn(log, c, self)...)
	}

	return graph, nil
}

// CycleMembers returns the containers that are part of a circular dependency.
//
// Every member of every cycle is reported, in the order the containers were
// given to the graph. A container that only depends on a cycle member is not
// part of the cycle, and a container that lists itself is a self-dependency
// rather than a cycle, so neither is reported.
//
// Returns:
//   - []types.Container: Containers in a cycle.
func (g *DependencyGraph) CycleMembers() []types.Container {
	edges := make(map[string][]string, len(g.containers))
	for _, c := range g.containers {
		edges[g.identifiers[c]] = g.dependencies[c]
	}

	finder := &cycleFinder{
		adjacency: edges,
		index:     make(map[string]int, len(g.containers)),
		lowLink:   make(map[string]int, len(g.containers)),
		onStack:   make(map[string]bool, len(g.containers)),
		stack:     make([]string, 0, len(g.containers)),
		inCycle:   make(map[string]bool),
	}

	// Start from the containers in the order given so the search is deterministic.
	for _, c := range g.containers {
		if _, visited := finder.index[g.identifiers[c]]; !visited {
			finder.visit(g.identifiers[c])
		}
	}

	members := make([]types.Container, 0, len(finder.inCycle))

	for _, c := range g.containers {
		if finder.inCycle[g.identifiers[c]] {
			members = append(members, c)
		}
	}

	return members
}

// Dependents returns the containers that depend directly on c: members of the
// graph in the order they were given, then Watchtower containers. Watchtower
// containers are included both as c and among the dependents.
//
// Parameters:
//   - c: A monitored container.
//
// Returns:
//   - []types.Container: The containers whose links name c. Nil when none do
//     or c is not monitored.
func (g *DependencyGraph) Dependents(c types.Container) []types.Container {
	return g.dependents[c]
}

// UnresolvedDependsOn returns the Watchtower depends-on label entries that name
// no container. They are left out of the graph.
//
// Returns:
//   - []UnresolvedLink: The entries, in the order the containers were given.
func (g *DependencyGraph) UnresolvedDependsOn() []UnresolvedLink {
	return g.unresolved
}

// Sort sorts containers in place by the dependencies between them, with
// Watchtower containers last.
//
// Only dependencies between the given containers affect the order. Ties are
// broken as SortByDependencies breaks them, so sorting every container of the
// graph gives the same order as SortByDependencies.
//
// Parameters:
//   - log: Process logger.
//   - containers: Slice to sort in place. Every container other than a
//     Watchtower container must be in the graph.
//
// Returns:
//   - error: A CircularReferenceError when the containers depend on each other
//     in a cycle, or ErrNotInGraph for a container outside the graph.
func (g *DependencyGraph) Sort(log *zerolog.Logger, containers []types.Container) error {
	ordered := make([]types.Container, 0, len(containers))
	watchtowers := make([]types.Container, 0, 1)

	for _, c := range containers {
		if c.IsWatchtower() {
			watchtowers = append(watchtowers, c)
		} else {
			ordered = append(ordered, c)
		}
	}

	containerMap, indegree, adjacency, normalizedMap, err := g.subgraph(ordered)
	if err != nil {
		return err
	}

	sorted, err := kahnSort(log, ordered, containerMap, indegree, adjacency, normalizedMap)
	if err != nil {
		return err
	}

	copy(containers, sorted)
	copy(containers[len(sorted):], watchtowers)

	return nil
}

// subgraph returns the part of the graph between containers, in the form
// kahnSort takes. Edges are added in the order of containers and of their
// links, so ties break as they do when the graph of containers alone is built.
//
// Parameters:
//   - containers: Containers of the graph.
//
// Returns:
//   - map[string]types.Container: Container for each identifier.
//   - map[string]int: Number of dependencies of each identifier.
//   - map[string][]string: Dependents of each identifier.
//   - map[types.Container]string: Identifier of each container.
//   - error: ErrNotInGraph for a container outside the graph, or an
//     IdentifierCollisionError for a container given twice.
func (g *DependencyGraph) subgraph(containers []types.Container) (
	map[string]types.Container, map[string]int, map[string][]string, map[types.Container]string, error,
) {
	containerMap := make(map[string]types.Container, len(containers))
	indegree := make(map[string]int, len(containers))
	adjacency := make(map[string][]string)
	normalizedMap := make(map[types.Container]string, len(containers))

	for _, c := range containers {
		identifier, ok := g.identifiers[c]
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("%w: %s", ErrNotInGraph, c.Name())
		}

		if existing, duplicate := containerMap[identifier]; duplicate {
			return nil, nil, nil, nil, IdentifierCollisionError{
				DuplicateIdentifier: identifier,
				AffectedContainers:  []types.Container{existing, c},
			}
		}

		containerMap[identifier] = c
		indegree[identifier] = 0
		normalizedMap[c] = identifier
	}

	for _, c := range containers {
		identifier := normalizedMap[c]

		for _, key := range g.dependencies[c] {
			if _, isNode := containerMap[key]; !isNode {
				// The link names a container outside the ones being sorted.
				continue
			}

			indegree[identifier]++
			adjacency[key] = append(adjacency[key], identifier)
		}
	}

	return containerMap, indegree, adjacency, normalizedMap, nil
}

// addDependent records that dependent depends on target.
//
// Parameters:
//   - target: The container depended on.
//   - dependent: The container whose link names target.
func (g *DependencyGraph) addDependent(target, dependent types.Container) {
	// A container's links are resolved together, so a container already
	// listed as a dependent of target is the last one listed.
	if listed := g.dependents[target]; len(listed) == 0 || listed[len(listed)-1] != dependent {
		g.dependents[target] = append(listed, dependent)
	}
}
