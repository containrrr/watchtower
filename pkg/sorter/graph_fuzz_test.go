package sorter

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// FuzzCycleMembers fuzz tests cycle membership on dependency graphs parsed
// from "A->B,B->C" input (see parseFuzzDependencyData).
//
// The seed corpus covers a two-node cycle, a three-node cycle, an acyclic
// chain, a cycle beside a separate component, a dependent of a cycle member,
// and a link that resolves to a replica once its exact target is set aside.
//
// It checks that building the graph does not panic and only fails on
// identifier collisions, that CycleMembers reports each member once and in
// input order, and that a container is reported exactly when it can reach
// itself through its dependencies. It also checks what the update relies on:
// the graph sorts the containers it does not report, and any subset of them,
// without a circular reference.
func FuzzCycleMembers(f *testing.F) {
	f.Add([]byte("A->B,B->A"))
	f.Add([]byte("A->B,B->C,C->A"))
	f.Add([]byte("A->B,B->C,C->D"))
	f.Add([]byte("A->B,B->A,C->D"))
	f.Add([]byte("W->A,A->B,B->A"))
	f.Add([]byte("db->e,e->db,c->db,db-1->c"))

	f.Fuzz(func(t *testing.T, data []byte) {
		containers := parseFuzzDependencyData(data)

		for _, useComposeDependsOn := range []bool{true, false} {
			graph, err := NewDependencyGraph(newNopLogger(), containers, nil, useComposeDependsOn)
			if err != nil {
				if !errors.Is(err, errIdentifierCollision) {
					t.Fatalf("unexpected error: %v", err)
				}

				continue
			}

			members := graph.CycleMembers()
			assertInputOrder(t, containers, members)

			want := selfReachable(t, containers, useComposeDependsOn)

			inCycle := make(map[types.ContainerID]bool, len(members))
			for _, c := range members {
				inCycle[c.ID()] = true
			}

			for _, c := range containers {
				if inCycle[c.ID()] != want[c.ID()] {
					t.Fatalf("container %q: reported %t, reaches itself %t", c.Name(), inCycle[c.ID()], want[c.ID()])
				}
			}

			rest := make([]types.Container, 0, len(containers)-len(members))
			for _, c := range containers {
				if !inCycle[c.ID()] {
					rest = append(rest, c)
				}
			}

			// The update sorts the containers outside the cycles, then the
			// subset of them that restarts. Pick a subset from the input.
			subset := make([]types.Container, 0, len(rest))
			for i, c := range rest {
				if data[i%len(data)]&1 == 1 {
					subset = append(subset, c)
				}
			}

			for _, sorted := range [][]types.Container{rest, subset} {
				err = graph.Sort(newNopLogger(), sorted)
				if errors.Is(err, errCircularReference) {
					t.Fatalf("containers outside the reported cycles do not sort: %v", err)
				}
			}
		}
	})
}

// assertInputOrder fails the test unless members holds distinct containers
// that appear in containers in the same relative order.
func assertInputOrder(t *testing.T, containers, members []types.Container) {
	t.Helper()

	next := 0

	for _, member := range members {
		for next < len(containers) && containers[next] != member {
			next++
		}

		if next == len(containers) {
			t.Fatalf("member %q is repeated, out of order, or not an input", member.Name())
		}

		next++
	}
}

// selfReachable reports, for each container, whether it can reach itself by
// following the edges of the sorter's dependency graph. It checks every
// container with its own search, independently of the cycle finder.
func selfReachable(
	t *testing.T,
	containers []types.Container,
	useComposeDependsOn bool,
) map[types.ContainerID]bool {
	t.Helper()

	containerMap, _, adjacency, _, err := buildDependencyGraph(newNopLogger(), containers, useComposeDependsOn)
	if err != nil {
		t.Fatalf("building the graph: %v", err)
	}

	return reachesItself(containerMap, adjacency)
}

// reachesItself reports, for each container of a graph, whether it can reach
// itself by following the graph's edges, with a separate search per container.
func reachesItself(containerMap map[string]types.Container, adjacency map[string][]string) map[types.ContainerID]bool {
	reaches := make(map[types.ContainerID]bool, len(containerMap))

	for start, c := range containerMap {
		visited := map[string]bool{}
		queue := slices.Clone(adjacency[start])

		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]

			if node == start {
				reaches[c.ID()] = true

				break
			}

			if visited[node] {
				continue
			}

			visited[node] = true
			queue = append(queue, adjacency[node]...)
		}
	}

	return reaches
}

// fuzzBytes reads small choices from fuzz input, returning zero once the input
// runs out.
type fuzzBytes struct {
	// data is the fuzz input.
	data []byte
	// pos is the index of the next byte to read.
	pos int
}

// next returns a value in [0, n) from the next input byte.
func (f *fuzzBytes) next(n int) int {
	if f.pos >= len(f.data) {
		return 0
	}

	b := f.data[f.pos]
	f.pos++

	return int(b) % n
}

// generateDeployment builds a deployment of 2 to 8 containers from fuzz input.
//
// Containers are plain containers (some named like a Compose service, replica,
// or project-qualified service), Compose v2 replicas, Compose containers with a
// container_name, Compose v1 names, Compose services without a container
// number, and Watchtower containers. Each has up to three links, each from one
// of the sources Container.Links reads: the Watchtower depends-on label, the
// Compose depends_on label, legacy links, network mode, or volumes-from. A
// link names its target by container name, by name with a leading slash, by
// Docker ID, by Compose service, or by project-qualified service.
func generateDeployment(data []byte) []types.Container {
	in := &fuzzBytes{data: data}
	projects := []string{"a", "b"}
	services := []string{"web", "db", "api"}

	count := 2 + in.next(7)
	specs := make([]deployed, count)
	taken := make(map[string]bool, count)

	for i := range specs {
		number := strconv.Itoa(i + 1)
		project, service := projects[in.next(len(projects))], services[in.next(len(services))]

		var spec deployed

		switch in.next(6) {
		case 0:
			plain := []string{service, service + "-" + number, project + "-" + service, "c" + number}
			spec = deployed{name: plain[in.next(len(plain))]}
		case 1:
			spec = deployed{name: project + "-" + service + "-" + number, project: project, service: service, number: number}
		case 2:
			spec = deployed{name: "x" + number, project: project, service: service, number: number}
		case 3:
			spec = deployed{name: project + "_" + service + "_" + number, project: project, service: service, number: number}
		case 4:
			spec = deployed{name: "y" + number, project: project, service: service}
		default:
			spec = deployed{name: "watchtower" + number, watchtower: true}
		}

		// Docker container names are unique.
		if taken[spec.name] {
			spec.name = "c" + number
		}

		taken[spec.name] = true
		specs[i] = spec
	}

	for i := range specs {
		for range in.next(4) {
			target := specs[in.next(count)]

			references := []string{target.name, "/" + target.name, dockerID(target.name), target.name, target.name}
			if target.service != "" {
				references[3] = target.service
				references[4] = target.project + "-" + target.service
			}

			reference := references[in.next(len(references))]

			switch in.next(5) {
			case 0:
				specs[i].dependsOn = strings.TrimPrefix(specs[i].dependsOn+","+reference, ",")
			case 1:
				entry := reference + ":service_started:false"
				specs[i].composeDependsOn = strings.TrimPrefix(specs[i].composeDependsOn+","+entry, ",")
			case 2:
				specs[i].links = append(specs[i].links, reference+":alias")
			case 3:
				specs[i].networkMode = "container:" + reference
			default:
				modes := []string{"", ":ro", ":rw"}
				specs[i].volumesFrom = append(specs[i].volumesFrom, reference+modes[in.next(len(modes))])
			}
		}
	}

	return deploy(specs...)
}

// FuzzDeploymentGraphs fuzz tests cycle membership and dependency sorting on
// realistic deployments built by generateDeployment.
//
// The seed corpus is a few short inputs that build mixed deployments. The
// fuzzer explores identities, link sources, and reference forms from there.
//
// Some containers are left unmonitored, chosen from the input. For both
// settings of Compose depends_on, it checks that building the graph fails only
// on an identifier collision, and that CycleMembers reports monitored
// non-Watchtower containers once each, in input order, exactly when they can
// reach themselves. Every edge of the graph must be recorded among the
// dependents, and no unmonitored container may be a dependent or have
// dependents. Sorting the remaining containers, and a subset of them chosen
// from the input, with the graph must succeed, keep the same containers, put
// Watchtower containers last, and order every dependency of the graph before
// its dependents. With every container monitored and no cycle, sorting the
// whole deployment with the graph must give the same order as
// SortByDependencies.
func FuzzDeploymentGraphs(f *testing.F) {
	f.Add([]byte{3, 0, 1, 1, 2, 2, 0, 3, 1, 4, 2, 2, 1, 0, 3, 3})
	f.Add([]byte{5, 1, 0, 1, 1, 1, 1, 2, 3, 1, 0, 2, 4, 2, 1, 3, 0})
	f.Add([]byte{7, 0, 2, 0, 1, 2, 3, 1, 1, 4, 0, 0, 5, 2, 2, 2, 2, 1, 3, 2, 0, 4, 1, 1, 2, 3, 3})
	f.Add([]byte{2, 1, 1, 0, 0, 1, 0, 2, 1, 1, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		containers := generateDeployment(data)

		// Leave some containers unmonitored, chosen from the input.
		monitored := make([]types.Container, 0, len(containers))
		unmonitored := make([]types.Container, 0, len(containers))

		for i, c := range containers {
			if len(data) > 0 && data[(i*3+1)%len(data)]&8 == 8 {
				unmonitored = append(unmonitored, c)
			} else {
				monitored = append(monitored, c)
			}
		}

		ordered := make([]types.Container, 0, len(monitored))
		for _, c := range monitored {
			if !c.IsWatchtower() {
				ordered = append(ordered, c)
			}
		}

		for _, useComposeDependsOn := range []bool{true, false} {
			graph, err := NewDependencyGraph(newNopLogger(), monitored, unmonitored, useComposeDependsOn)
			if err != nil {
				if !errors.Is(err, errIdentifierCollision) {
					t.Fatalf("unexpected error: %v", err)
				}

				continue
			}

			containerMap, _, adjacency, _, err := graph.subgraph(ordered)
			if err != nil {
				t.Fatalf("subgraph of the whole graph: %v", err)
			}

			members := graph.CycleMembers()
			assertInputOrder(t, ordered, members)

			want := reachesItself(containerMap, adjacency)
			inCycle := make(map[types.ContainerID]bool, len(members))

			for _, c := range members {
				inCycle[c.ID()] = true
			}

			for _, c := range ordered {
				if inCycle[c.ID()] != want[c.ID()] {
					t.Fatalf("container %q: reported %t, reaches itself %t", c.Name(), inCycle[c.ID()], want[c.ID()])
				}
			}

			assertDependents(t, graph, monitored, unmonitored, containerMap, adjacency)

			rest := make([]types.Container, 0, len(monitored))
			for _, c := range monitored {
				if !inCycle[c.ID()] {
					rest = append(rest, c)
				}
			}

			subset := make([]types.Container, 0, len(rest))
			for i, c := range rest {
				if len(data) > 0 && data[(i*7)%len(data)]&2 == 2 {
					subset = append(subset, c)
				}
			}

			for _, toSort := range [][]types.Container{rest, subset} {
				sorted := slices.Clone(toSort)

				err := graph.Sort(newNopLogger(), sorted)
				if err != nil {
					t.Fatalf("sorting %v with the graph: %v", containerNames(toSort), err)
				}

				assertDependencyOrder(t, toSort, sorted, containerMap, adjacency)
			}

			// Unmonitored containers change how links resolve, so the order
			// matches SortByDependencies only when every container is monitored.
			if len(members) == 0 && len(unmonitored) == 0 {
				legacy := slices.Clone(containers)
				fromGraph := slices.Clone(containers)

				legacyErr := SortByDependencies(newNopLogger(), legacy, useComposeDependsOn)
				graphErr := graph.Sort(newNopLogger(), fromGraph)

				if (legacyErr == nil) != (graphErr == nil) {
					t.Fatalf("SortByDependencies error %v, graph sort error %v", legacyErr, graphErr)
				}

				if legacyErr == nil && !slices.Equal(containerNames(legacy), containerNames(fromGraph)) {
					t.Fatalf("orders differ: %v and %v", containerNames(legacy), containerNames(fromGraph))
				}
			}
		}
	})
}

// assertDependents fails the test unless every edge of the graph between its
// members is recorded among the dependents, and no unmonitored container is a
// dependent or has dependents.
func assertDependents(
	t *testing.T,
	graph *DependencyGraph,
	monitored, unmonitored []types.Container,
	containerMap map[string]types.Container,
	adjacency map[string][]string,
) {
	t.Helper()

	for dependencyKey, dependentKeys := range adjacency {
		dependents := graph.Dependents(containerMap[dependencyKey])

		for _, dependentKey := range dependentKeys {
			if !slices.Contains(dependents, containerMap[dependentKey]) {
				t.Fatalf("%q depends on %q but is not among its dependents",
					containerMap[dependentKey].Name(), containerMap[dependencyKey].Name())
			}
		}
	}

	for _, c := range unmonitored {
		if len(graph.Dependents(c)) > 0 {
			t.Fatalf("unmonitored container %q has dependents", c.Name())
		}
	}

	for _, c := range monitored {
		for _, dependent := range graph.Dependents(c) {
			if slices.Contains(unmonitored, dependent) {
				t.Fatalf("unmonitored container %q is a dependent of %q", dependent.Name(), c.Name())
			}
		}
	}
}

// assertDependencyOrder fails the test unless sorted holds the same containers
// as unsorted, with Watchtower containers last and every edge of the full
// graph between two sorted containers ordered dependency first.
func assertDependencyOrder(
	t *testing.T,
	unsorted, sorted []types.Container,
	containerMap map[string]types.Container,
	adjacency map[string][]string,
) {
	t.Helper()

	if !slices.Equal(slices.Sorted(slices.Values(containerNames(unsorted))),
		slices.Sorted(slices.Values(containerNames(sorted)))) {
		t.Fatalf("sorting changed the containers: %v to %v", containerNames(unsorted), containerNames(sorted))
	}

	position := make(map[types.Container]int, len(sorted))
	seenWatchtower := false

	for i, c := range sorted {
		position[c] = i

		if c.IsWatchtower() {
			seenWatchtower = true
		} else if seenWatchtower {
			t.Fatalf("Watchtower container sorted before %q: %v", c.Name(), containerNames(sorted))
		}
	}

	for dependencyKey, dependentKeys := range adjacency {
		dependency, ok := position[containerMap[dependencyKey]]
		if !ok {
			continue
		}

		for _, dependentKey := range dependentKeys {
			dependent, ok := position[containerMap[dependentKey]]
			if ok && dependent < dependency {
				t.Fatalf("%q sorted before its dependency %q: %v",
					containerMap[dependentKey].Name(), containerMap[dependencyKey].Name(), containerNames(sorted))
			}
		}
	}
}
