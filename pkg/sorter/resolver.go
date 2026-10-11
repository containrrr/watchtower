package sorter

import (
	"slices"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// linkResolver resolves dependency links to the containers they name.
type linkResolver struct {
	// matchIDSet holds the identifiers and aliases of the graph's members.
	matchIDSet map[string]bool
	// aliasToCanonical maps each alias of a member to its identifier.
	aliasToCanonical map[string]string
	// projects maps the identifier of each member with a Compose project label
	// to its project.
	projects map[string]string
	// services maps each Compose service name to the identifiers of the
	// members of that service, in every project.
	services map[string][]string
	// watchtowers maps the exact names of monitored Watchtower containers to them.
	watchtowers map[string]types.Container
	// unmonitored maps the exact names of unmonitored containers to them.
	unmonitored map[string]types.Container
	// unmonitoredNames maps each Compose project, or empty for containers
	// without one, to the identifiers, names, and service names of its
	// unmonitored containers.
	unmonitoredNames map[string]map[string]bool
	// watchtowerAliases maps the identifier without its replica number of each
	// Watchtower container that is the only one with that identifier to it.
	watchtowerAliases map[string]types.Container
}

// resolve returns the containers a link of c names.
//
// A link that names a member exactly resolves to that member. Otherwise, a
// link that names a Watchtower container exactly, or an unmonitored container
// exactly, names only that container. Otherwise a link that names a Watchtower
// container without its replica number, when it is the only such Watchtower
// container, names it. Otherwise the link is matched by replica or service
// name, as findMatchingIdentifiersInSet matches it. When that finds more than
// one service, the link matches the members whose Compose service is the link,
// in every project, so a service name shared by several projects is never
// dropped. byProject then narrows the matches.
//
// Parameters:
//   - c: The container whose link this is.
//   - link: A normalized link from Container.Links.
//
// Returns:
//   - []string: Identifiers of the members the link names.
//   - types.Container: The Watchtower container the link names, or nil.
//   - bool: Whether the link names any container, monitored or not.
func (r *linkResolver) resolve(c types.Container, link string) ([]string, types.Container, bool) {
	exact := r.matchIDSet[link]
	if !exact {
		if watchtower, ok := r.watchtowers[link]; ok {
			return nil, watchtower, true
		}

		if _, ok := r.unmonitored[link]; ok {
			return nil, nil, true
		}

		if watchtower, ok := r.watchtowerAliases[link]; ok {
			return nil, watchtower, true
		}
	}

	keys := resolveLinkToCanonicalKeys(link, r.matchIDSet, r.aliasToCanonical)
	if exact {
		return keys, nil, len(keys) > 0
	}

	if len(keys) == 0 {
		keys = r.services[link]
	}

	keys, named := r.byProject(c, link, keys)

	return keys, nil, named
}

// byProject returns the members a link of c names by replica or service name.
//
// A service name means the service of the linking container's own Compose
// project when that project has one. When some keys are in the project of c,
// the link names only those. When an unmonitored container of the project of
// c has the name, the link names only that container, which is left alone.
// Otherwise the link names every key, so a dependency is never dropped because
// its name is shared by several projects.
//
// Parameters:
//   - c: The container whose link this is.
//   - link: A normalized link from Container.Links.
//   - keys: Identifiers of the members the link matches by replica or service
//     name.
//
// Returns:
//   - []string: Identifiers of the members the link names.
//   - bool: Whether the link names any container, monitored or not.
func (r *linkResolver) byProject(c types.Container, link string, keys []string) ([]string, bool) {
	if project := projectOf(c); project != "" {
		local := make([]string, 0, len(keys))

		for _, key := range keys {
			if r.projects[key] == project {
				local = append(local, key)
			}
		}

		if len(local) > 0 {
			return local, true
		}

		if len(findMatchingIdentifiersInSet(link, r.unmonitoredNames[project])) > 0 {
			return nil, true
		}
	}

	if len(keys) > 0 {
		return keys, true
	}

	for _, names := range r.unmonitoredNames {
		if len(findMatchingIdentifiersInSet(link, names)) > 0 {
			return nil, true
		}
	}

	return nil, false
}

// unresolvedDependsOn returns the Watchtower depends-on label entries of c that
// name no container. An entry naming c itself is a self-dependency, which the
// update reports on its own.
//
// Parameters:
//   - log: Process logger.
//   - c: A monitored container.
//   - self: The identifier of c in the graph, or empty when c is not a member.
//
// Returns:
//   - []UnresolvedLink: The entries that name no container.
func (r *linkResolver) unresolvedDependsOn(log *zerolog.Logger, c types.Container, self string) []UnresolvedLink {
	concrete, ok := c.(*container.Container)
	if !ok {
		return nil
	}

	var unresolved []UnresolvedLink

	for _, link := range container.GetLinksFromWatchtowerLabel(concrete, log) {
		if link == c.Name() || (self != "" && link == self) {
			continue
		}

		if _, _, named := r.resolve(c, link); !named {
			unresolved = append(unresolved, UnresolvedLink{Container: c, Link: link})
		}
	}

	return unresolved
}

// projectOf returns the Compose project of c, or empty when it has none.
//
// Parameters:
//   - c: A container.
//
// Returns:
//   - string: The value of the Compose project label.
func projectOf(c types.Container) string {
	info := c.ContainerInfo()
	if info == nil || info.Config == nil {
		return ""
	}

	return compose.GetProjectName(info.Config.Labels)
}

// serviceOf returns the normalized Compose service of c, or empty when it
// has none.
//
// Parameters:
//   - c: A container.
//
// Returns:
//   - string: The value of the Compose service label, normalized as a link.
func serviceOf(c types.Container) string {
	info := c.ContainerInfo()
	if info == nil || info.Config == nil {
		return ""
	}

	return util.NormalizeContainerName(compose.GetServiceName(info.Config.Labels))
}

// memberProjects maps the identifier of each member with a Compose project
// label to its project.
//
// Parameters:
//   - members: Members by identifier.
//
// Returns:
//   - map[string]string: Project for each identifier.
func memberProjects(members map[string]types.Container) map[string]string {
	projects := make(map[string]string, len(members))

	for identifier, c := range members {
		if project := projectOf(c); project != "" {
			projects[identifier] = project
		}
	}

	return projects
}

// memberServices maps each Compose service name to the identifiers of the
// members of that service, in every project, sorted.
//
// Parameters:
//   - members: Members by identifier.
//
// Returns:
//   - map[string][]string: Identifiers for each service name.
func memberServices(members map[string]types.Container) map[string][]string {
	services := make(map[string][]string)

	for identifier, c := range members {
		if service := serviceOf(c); service != "" {
			services[service] = append(services[service], identifier)
		}
	}

	for _, identifiers := range services {
		slices.Sort(identifiers)
	}

	return services
}

// projectNames maps each Compose project, or empty for containers without
// one, to the normalized identifiers, names, and service names of its
// containers, as a link names them.
//
// Parameters:
//   - containers: Containers to index.
//
// Returns:
//   - map[string]map[string]bool: Identifiers and names for each project.
func projectNames(containers []types.Container) map[string]map[string]bool {
	projects := make(map[string]map[string]bool)

	for _, c := range containers {
		project := projectOf(c)
		if projects[project] == nil {
			projects[project] = make(map[string]bool)
		}

		for _, name := range []string{
			util.NormalizeContainerName(container.ResolveContainerIdentifier(c)),
			util.NormalizeContainerName(c.Name()),
			serviceOf(c),
		} {
			if name != "" {
				projects[project][name] = true
			}
		}
	}

	return projects
}

// namesPerContainer is the number of exact names of a container: its
// identifier, its name, and its ID.
const namesPerContainer = 3

// exactNames maps the identifier, name, and ID of each container to it, as a
// link names a container exactly.
//
// Parameters:
//   - containers: Containers to index.
//
// Returns:
//   - map[string]types.Container: Container for each exact name.
func exactNames(containers []types.Container) map[string]types.Container {
	names := make(map[string]types.Container, len(containers)*namesPerContainer)

	for _, c := range containers {
		for _, name := range []string{
			util.NormalizeContainerName(container.ResolveContainerIdentifier(c)),
			util.NormalizeContainerName(c.Name()),
			string(c.ID()),
		} {
			if _, taken := names[name]; name != "" && !taken {
				names[name] = c
			}
		}
	}

	return names
}

// watchtowerAliases maps each Watchtower container's identifier without its
// replica number to it, when only one Watchtower container has that identifier
// and no Watchtower container has that exact name. A Compose depends_on entry
// links to a service that way, and the name identifies one container, as a
// replica match does for members.
//
// Parameters:
//   - watchtowers: Watchtower containers to index.
//   - names: The exact names of the Watchtower containers.
//
// Returns:
//   - map[string]types.Container: Container for each alias.
func watchtowerAliases(watchtowers []types.Container, names map[string]types.Container) map[string]types.Container {
	aliases := make(map[string]types.Container, len(watchtowers))
	replicas := make(map[string]int, len(watchtowers))

	for _, c := range watchtowers {
		identifier := util.NormalizeContainerName(container.ResolveContainerIdentifier(c))
		if service := replicaService(identifier); service != identifier {
			replicas[service]++
		}
	}

	for _, c := range watchtowers {
		identifier := util.NormalizeContainerName(container.ResolveContainerIdentifier(c))

		service := replicaService(identifier)
		if _, taken := names[service]; service != identifier && replicas[service] == 1 && !taken {
			aliases[service] = c
		}
	}

	return aliases
}
