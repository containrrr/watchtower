package sorter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// cycleContainer returns a container with the given name and labels.
func cycleContainer(name string, labels map[string]string) types.Container {
	return container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:         name + "-id",
		Name:       "/" + name,
		HostConfig: &dockerContainer.HostConfig{},
		Config:     &dockerContainer.Config{Labels: labels},
	}, nil)
}

// dependsOn returns a container whose Watchtower depends-on label lists links.
func dependsOn(name, links string) types.Container {
	labels := map[string]string{}
	if links != "" {
		labels["com.centurylinklabs.watchtower.depends-on"] = links
	}

	return cycleContainer(name, labels)
}

// composeService returns a Compose container of the stack project named
// <service>-server, whose depends_on label lists dependsOnServices.
func composeService(service, dependsOnServices string) types.Container {
	labels := map[string]string{
		"com.docker.compose.project":          "stack",
		"com.docker.compose.service":          service,
		"com.docker.compose.container-number": "1",
	}
	if dependsOnServices != "" {
		labels["com.docker.compose.depends_on"] = dependsOnServices
	}

	return cycleContainer(service+"-server", labels)
}

// TestDependencyGraph_CycleMembers verifies that every container in a circular dependency is
// reported, in the order given, and that containers outside a cycle are not,
// including those that only depend on a cycle member and Watchtower containers.
func TestDependencyGraph_CycleMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		containers []types.Container
		// useComposeDependsOn includes Compose depends_on labels as links.
		useComposeDependsOn bool
		want                []string
	}{
		{
			name:       "no containers",
			containers: nil,
			want:       []string{},
		},
		{
			name:       "acyclic",
			containers: []types.Container{dependsOn("c1", ""), dependsOn("c2", "c1")},
			want:       []string{},
		},
		{
			name:       "two containers",
			containers: []types.Container{dependsOn("c1", "c2"), dependsOn("c2", "c1")},
			want:       []string{"c1", "c2"},
		},
		{
			name: "three containers",
			containers: []types.Container{
				dependsOn("c1", "c2"),
				dependsOn("c2", "c3"),
				dependsOn("c3", "c1"),
			},
			want: []string{"c1", "c2", "c3"},
		},
		{
			// A container listing itself is a self-dependency, which the
			// update skips separately, not a cycle between containers.
			name:       "self-reference",
			containers: []types.Container{dependsOn("c1", "c1")},
			want:       []string{},
		},
		{
			name: "disconnected components",
			containers: []types.Container{
				dependsOn("c1", ""),
				dependsOn("c2", "c3"),
				dependsOn("c3", "c2"),
			},
			want: []string{"c2", "c3"},
		},
		{
			name: "dependents of a cycle member",
			containers: []types.Container{
				dependsOn("worker", "c1"),
				dependsOn("c1", "c2"),
				dependsOn("c2", "c1"),
			},
			want: []string{"c1", "c2"},
		},
		{
			name:       "unknown dependencies",
			containers: []types.Container{dependsOn("c1", "c2,unknown"), dependsOn("c2", "c1")},
			want:       []string{"c1", "c2"},
		},
		{
			// The sort orders Watchtower containers last without reading
			// their links, so they cannot be part of a cycle.
			name: "Watchtower containers set aside",
			containers: []types.Container{
				cycleContainer("watchtower", map[string]string{
					"com.centurylinklabs.watchtower":            "true",
					"com.centurylinklabs.watchtower.depends-on": "c1",
				}),
				dependsOn("c1", "watchtower"),
			},
			want: []string{},
		},
		{
			name: "Compose services with other container names",
			containers: []types.Container{
				composeService("web", "api:service_started:false"),
				composeService("api", "web:service_started:false"),
				composeService("app", ""),
			},
			useComposeDependsOn: true,
			want:                []string{"web-server", "api-server"},
		},
		{
			name: "Compose depends_on ignored",
			containers: []types.Container{
				composeService("web", "api:service_started:false"),
				composeService("api", "web:service_started:false"),
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph, err := NewDependencyGraph(testLog(), tt.containers, nil, tt.useComposeDependsOn)
			require.NoError(t, err)
			assert.Equal(t, tt.want, containerNames(graph.CycleMembers()))
		})
	}
}

// TestNewDependencyGraph_IdentifierCollision verifies that containers sharing
// a dependency identifier are reported as an error, as the sort reports them.
func TestNewDependencyGraph_IdentifierCollision(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"com.docker.compose.project": "stack",
		"com.docker.compose.service": "web",
	}

	_, err := NewDependencyGraph(testLog(), []types.Container{
		cycleContainer("first", labels),
		cycleContainer("second", labels),
	}, nil, false)
	require.ErrorIs(t, err, ErrIdentifierCollision)
}

// TestDependencyGraph_CycleMembersLinkSources verifies that a cycle is found whichever source
// of dependency links closes it, and however the link names its target: by
// container name, Compose service, Docker ID, or with a volumes-from mode.
func TestDependencyGraph_CycleMembersLinkSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		containers []types.Container
		// useComposeDependsOn includes Compose depends_on labels as links.
		useComposeDependsOn bool
		want                []string
	}{
		{
			name: "network mode by name",
			containers: deploy(
				deployed{name: "app", networkMode: "container:vpn"},
				deployed{name: "vpn", dependsOn: "app"},
			),
			want: []string{"app", "vpn"},
		},
		{
			name: "network mode by Docker ID",
			containers: deploy(
				deployed{name: "app", networkMode: "container:" + dockerID("vpn")},
				deployed{name: "vpn", dependsOn: "app"},
			),
			want: []string{"app", "vpn"},
		},
		{
			name: "volumes-from by name with a mode",
			containers: deploy(
				deployed{name: "app", volumesFrom: []string{"data:ro"}},
				deployed{name: "data", dependsOn: "app"},
			),
			want: []string{"app", "data"},
		},
		{
			name: "volumes-from by Docker ID",
			containers: deploy(
				deployed{name: "app", volumesFrom: []string{dockerID("data") + ":rw"}},
				deployed{name: "data", dependsOn: "app"},
			),
			want: []string{"app", "data"},
		},
		{
			// A depends-on label replaces the other link sources, except
			// volumes-from, which is always a dependency.
			name: "volumes-from beside a depends-on label",
			containers: deploy(
				deployed{name: "app", dependsOn: "db", volumesFrom: []string{"data"}},
				deployed{name: "data", dependsOn: "app"},
				deployed{name: "db"},
			),
			want: []string{"app", "data"},
		},
		{
			name: "legacy links",
			containers: deploy(
				deployed{name: "app", links: []string{"db:database"}},
				deployed{name: "db", links: []string{"/app:web"}},
			),
			want: []string{"app", "db"},
		},
		{
			name: "legacy links in a Compose project",
			containers: deploy(
				deployed{name: "stack-app-1", project: "stack", service: "app", number: "1", links: []string{"db:db"}},
				deployed{name: "stack-db-1", project: "stack", service: "db", number: "1", links: []string{"app:app"}},
			),
			want: []string{"stack-app-1", "stack-db-1"},
		},
		{
			name: "Compose depends_on across replicas",
			containers: deploy(
				deployed{name: "stack-web-1", project: "stack", service: "web", number: "1", composeDependsOn: "api:service_started:false"},
				deployed{name: "stack-api-1", project: "stack", service: "api", number: "1", composeDependsOn: "web:service_started:false"},
				deployed{name: "stack-api-2", project: "stack", service: "api", number: "2", composeDependsOn: "web:service_started:false"},
			),
			useComposeDependsOn: true,
			want:                []string{"stack-web-1", "stack-api-1", "stack-api-2"},
		},
		{
			name: "Compose depends_on in JSON form",
			containers: deploy(
				deployed{name: "stack-web-1", project: "stack", service: "web", number: "1", composeDependsOn: `{"api":{"condition":"service_started"}}`},
				deployed{name: "stack-api-1", project: "stack", service: "api", number: "1", composeDependsOn: `{"web":{"condition":"service_started"}}`},
			),
			useComposeDependsOn: true,
			want:                []string{"stack-web-1", "stack-api-1"},
		},
		{
			name: "Compose v1 container names",
			containers: deploy(
				deployed{name: "stack_web_1", project: "stack", service: "web", number: "1", composeDependsOn: "api"},
				deployed{name: "stack_api_1", project: "stack", service: "api", number: "1", composeDependsOn: "web"},
			),
			useComposeDependsOn: true,
			want:                []string{"stack_web_1", "stack_api_1"},
		},
		{
			name: "same services in two projects",
			containers: deploy(
				deployed{name: "a-web-1", project: "a", service: "web", number: "1", composeDependsOn: "db"},
				deployed{name: "b-db-1", project: "b", service: "db", number: "1", composeDependsOn: "web"},
			),
			useComposeDependsOn: true,
			want:                []string{},
		},
		{
			name: "network mode on a Watchtower container",
			containers: deploy(
				deployed{name: "watchtower", watchtower: true, dependsOn: "app"},
				deployed{name: "app", networkMode: "container:" + dockerID("watchtower")},
			),
			want: []string{},
		},
		{
			name: "network-mode dependent of a cycle",
			containers: deploy(
				deployed{name: "app", networkMode: "container:vpn"},
				deployed{name: "vpn", dependsOn: "proxy"},
				deployed{name: "proxy", dependsOn: "vpn"},
			),
			want: []string{"vpn", "proxy"},
		},
		{
			name: "three link sources in one cycle",
			containers: deploy(
				deployed{name: "app", networkMode: "container:" + dockerID("vpn")},
				deployed{name: "vpn", volumesFrom: []string{"data:ro"}},
				deployed{name: "data", links: []string{"app:app"}},
			),
			want: []string{"app", "vpn", "data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph, err := NewDependencyGraph(testLog(), tt.containers, nil, tt.useComposeDependsOn)
			require.NoError(t, err)
			assert.Equal(t, tt.want, containerNames(graph.CycleMembers()))
		})
	}
}

// TestDependencyGraph_Subgraph verifies that a subset's graph keeps exactly
// the links among its containers that the full set's graph has. A link whose
// target is outside the subset is dropped rather than matched to another
// container in the subset, whatever source and reference form the link uses.
// The alone column records the edges that resolving links within the subset
// alone would give instead.
func TestDependencyGraph_Subgraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		all  []types.Container
		// subset names the containers that become graph nodes.
		subset []string
		// useComposeDependsOn includes Compose depends_on labels as links.
		useComposeDependsOn bool
		// want lists the edges as dependent->dependency.
		want []string
		// alone lists the edges when links resolve within the subset alone.
		alone []string
	}{
		{
			name: "exact target outside the subset",
			all: deploy(
				deployed{name: "db"},
				deployed{name: "db-1", dependsOn: "c"},
				deployed{name: "c", dependsOn: "db"},
			),
			subset: []string{"c", "db-1"},
			want:   []string{"db-1->c"},
			alone:  []string{"c->db-1", "db-1->c"},
		},
		{
			name: "service name shared by two projects",
			all: deploy(
				deployed{name: "app", dependsOn: "db"},
				deployed{name: "xdb", project: "x", service: "db"},
				deployed{name: "ydb", project: "y", service: "db"},
			),
			subset: []string{"app", "xdb"},
			want:   []string{"app->xdb"},
			alone:  []string{"app->xdb"},
		},
		{
			name: "network mode by Docker ID inside the subset",
			all: deploy(
				deployed{name: "app", networkMode: "container:" + dockerID("vpn")},
				deployed{name: "vpn"},
				deployed{name: "other"},
			),
			subset: []string{"app", "vpn"},
			want:   []string{"app->vpn"},
			alone:  []string{"app->vpn"},
		},
		{
			name: "volumes-from by name outside the subset",
			all: deploy(
				deployed{name: "app", volumesFrom: []string{"data:ro"}},
				deployed{name: "data"},
				deployed{name: "data-1"},
			),
			subset: []string{"app", "data-1"},
			want:   []string{},
			alone:  []string{"app->data-1"},
		},
		{
			name: "volumes-from by Docker ID outside the subset",
			all: deploy(
				deployed{name: "app", volumesFrom: []string{dockerID("data") + ":ro"}},
				deployed{name: "data"},
				deployed{name: "data-1"},
			),
			subset: []string{"app", "data-1"},
			want:   []string{},
			alone:  []string{},
		},
		{
			name: "Compose replicas in and out of the subset",
			all: deploy(
				deployed{name: "stack-web-1", project: "stack", service: "web", number: "1", composeDependsOn: "api"},
				deployed{name: "stack-api-1", project: "stack", service: "api", number: "1"},
				deployed{name: "stack-api-2", project: "stack", service: "api", number: "2"},
			),
			subset:              []string{"stack-web-1", "stack-api-2"},
			useComposeDependsOn: true,
			want:                []string{"stack-web-1->stack-api-2"},
			alone:               []string{"stack-web-1->stack-api-2"},
		},
		{
			name: "legacy link outside the subset in a Compose project",
			all: deploy(
				deployed{name: "stack-app-1", project: "stack", service: "app", number: "1", links: []string{"db:db"}},
				deployed{name: "stack-db", project: "stack", service: "db"},
				deployed{name: "stack-db-1", project: "stack", service: "db", number: "1"},
			),
			subset: []string{"stack-app-1", "stack-db-1"},
			want:   []string{},
			alone:  []string{"stack-app-1->stack-db-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			subset := pick(tt.all, tt.subset...)
			require.Len(t, subset, len(tt.subset))

			assert.ElementsMatch(t, tt.want, graphEdges(t, subset, tt.all, tt.useComposeDependsOn),
				"links resolved against all")
			assert.ElementsMatch(t, tt.alone, graphEdges(t, subset, nil, tt.useComposeDependsOn),
				"links resolved within the subset")
		})
	}
}

// graphEdges returns the edges between containers, as dependent->dependency
// container names, in the dependency graph of all. With all nil, the graph is
// built from containers alone.
func graphEdges(t *testing.T, containers, all []types.Container, useComposeDependsOn bool) []string {
	t.Helper()

	if all == nil {
		all = containers
	}

	graph, err := NewDependencyGraph(testLog(), all, nil, useComposeDependsOn)
	require.NoError(t, err)

	containerMap, _, adjacency, _, err := graph.subgraph(containers)
	require.NoError(t, err)

	edges := []string{}

	for dependency, dependents := range adjacency {
		for _, dependent := range dependents {
			edges = append(edges, containerMap[dependent].Name()+"->"+containerMap[dependency].Name())
		}
	}

	return edges
}

// TestDependencyGraph_Sort verifies that sorting orders dependencies first
// across link sources, puts Watchtower containers last, sorts a subset by the
// links between its containers only, and rejects a container that is not in
// the graph.
func TestDependencyGraph_Sort(t *testing.T) {
	t.Parallel()

	all := deploy(
		deployed{name: "app", networkMode: "container:" + dockerID("vpn"), volumesFrom: []string{"data:ro"}},
		deployed{name: "vpn"},
		deployed{name: "data", dependsOn: "vpn"},
		deployed{name: "watchtower", watchtower: true, dependsOn: "app"},
		deployed{name: "db"},
		deployed{name: "db-1", dependsOn: "c"},
		deployed{name: "c", dependsOn: "db"},
	)

	graph, err := NewDependencyGraph(testLog(), all, nil, false)
	require.NoError(t, err)

	tests := []struct {
		name string
		// sort names the containers to sort, in their order before sorting.
		sort []string
		want []string
	}{
		{name: "link sources", sort: []string{"app", "data", "vpn"}, want: []string{"vpn", "data", "app"}},
		{name: "Watchtower last", sort: []string{"watchtower", "app", "vpn"}, want: []string{"vpn", "app", "watchtower"}},
		{name: "link target outside the subset", sort: []string{"db-1", "c"}, want: []string{"c", "db-1"}},
		{name: "nothing to sort", sort: []string{}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			containers := pick(all, tt.sort...)
			require.NoError(t, graph.Sort(testLog(), containers))
			assert.Equal(t, tt.want, containerNames(containers))
		})
	}

	t.Run("container not in the graph", func(t *testing.T) {
		t.Parallel()

		containers := append(pick(all, "app"), deployed{name: "other"}.container())
		require.ErrorIs(t, graph.Sort(testLog(), containers), ErrNotInGraph)
	})
}

// TestDependencyGraph_ServiceNames verifies that a link naming a Compose
// service resolves to every replica of the service, whether the name is bare
// or hyphenated. A name shared by several projects resolves to the service of
// the linking container's own project when it has one, and to every project's
// service otherwise, so a shared name never drops a dependency. A name never
// fans out to a different service that only ends in it.
func TestDependencyGraph_ServiceNames(t *testing.T) {
	t.Parallel()

	replica := func(project, service, number, dependsOn string) deployed {
		return deployed{
			name:      project + "-" + service + "-" + number,
			project:   project,
			service:   service,
			number:    number,
			dependsOn: dependsOn,
		}
	}

	tests := []struct {
		name       string
		containers []types.Container
		// want lists the edges as dependent->dependency.
		want []string
	}{
		{
			name: "hyphenated service in another project",
			containers: deploy(
				replica("app1", "foo", "1", "watchtower-test-database"),
				replica("database1", "watchtower-test-database", "1", ""),
			),
			want: []string{"app1-foo-1->database1-watchtower-test-database-1"},
		},
		{
			name: "bare service with two replicas",
			containers: deploy(
				deployed{name: "app", dependsOn: "db"},
				replica("stack", "db", "1", ""),
				replica("stack", "db", "2", ""),
			),
			want: []string{"app->stack-db-1", "app->stack-db-2"},
		},
		{
			name: "hyphenated service with two replicas",
			containers: deploy(
				deployed{name: "app", dependsOn: "api-gateway"},
				replica("edge", "api-gateway", "1", ""),
				replica("edge", "api-gateway", "2", ""),
			),
			want: []string{"app->edge-api-gateway-1", "app->edge-api-gateway-2"},
		},
		{
			name: "project-qualified service with two replicas",
			containers: deploy(
				deployed{name: "app", dependsOn: "stack-db"},
				replica("stack", "db", "1", ""),
				replica("stack", "db", "2", ""),
			),
			want: []string{"app->stack-db-1", "app->stack-db-2"},
		},
		{
			name: "same service in two projects",
			containers: deploy(
				deployed{name: "app", dependsOn: "db"},
				replica("a", "db", "1", ""),
				replica("b", "db", "1", ""),
			),
			want: []string{"app->a-db-1", "app->b-db-1"},
		},
		{
			name: "hyphenated service in two projects",
			containers: deploy(
				deployed{name: "app", dependsOn: "api-gateway"},
				replica("a", "api-gateway", "1", ""),
				replica("b", "api-gateway", "1", ""),
			),
			want: []string{"app->a-api-gateway-1", "app->b-api-gateway-1"},
		},
		{
			name: "service in two projects beside services ending in its name",
			containers: deploy(
				deployed{name: "app", dependsOn: "db"},
				replica("a", "db", "1", ""),
				replica("b", "db", "1", ""),
				replica("c", "mongo-db", "1", ""),
				deployed{name: "legacy-db"},
			),
			want: []string{"app->a-db-1", "app->b-db-1"},
		},
		{
			name: "hyphenated service in two projects beside a longer service",
			containers: deploy(
				deployed{name: "app", dependsOn: "api-gateway"},
				replica("a", "api-gateway", "1", ""),
				replica("b", "api-gateway", "1", ""),
				replica("x", "edge-api-gateway", "1", ""),
			),
			want: []string{"app->a-api-gateway-1", "app->b-api-gateway-1"},
		},
		{
			name: "service beside another service ending in its name",
			containers: deploy(
				deployed{name: "app", dependsOn: "db"},
				replica("a", "db", "1", ""),
				replica("b", "mongo-db", "1", ""),
			),
			want: []string{"app->a-db-1"},
		},
		{
			name: "service in two projects, one of them the linking container's",
			containers: deploy(
				replica("a", "web", "1", "db"),
				replica("a", "db", "1", ""),
				replica("b", "db", "1", ""),
			),
			want: []string{"a-web-1->a-db-1"},
		},
		{
			name: "service in two other projects, one by container name",
			containers: deploy(
				replica("app", "web", "1", "watchtower-test-database"),
				replica("app", "web", "2", "watchtower-test-database"),
				replica("database1", "watchtower-test-database", "1", ""),
				deployed{
					name: "watchtower-test-database2", project: "database2",
					service: "watchtower-test-database", number: "1",
				},
			),
			want: []string{
				"app-web-1->database1-watchtower-test-database-1",
				"app-web-1->watchtower-test-database2",
				"app-web-2->database1-watchtower-test-database-1",
				"app-web-2->watchtower-test-database2",
			},
		},
		{
			name: "own service and another project's",
			containers: deploy(
				replica("a", "api", "1", "api"),
				replica("b", "api", "1", ""),
			),
			want: []string{},
		},
		{
			name: "multi-segment name of another service",
			containers: deploy(
				deployed{name: "app", dependsOn: "net-proxy"},
				replica("edge", "other-proxy", "1", ""),
			),
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.ElementsMatch(t, tt.want, graphEdges(t, tt.containers, nil, false))
		})
	}
}

// TestDependencyGraph_Dependents verifies the containers recorded as depending
// on each container: Watchtower containers depend on what their links name,
// containers depend on a Watchtower container only when they name it exactly
// or as the only replica of its Compose service, and a link that names an
// unmonitored container exactly names nothing else, not even a Watchtower
// container known by that replica-free name. A service name whose service in
// the linking container's own project is unmonitored names nothing either,
// rather than the service of another project.
func TestDependencyGraph_Dependents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		monitored   []types.Container
		unmonitored []types.Container
		// useComposeDependsOn includes Compose depends_on labels as links.
		useComposeDependsOn bool
		// of names the container whose dependents are checked.
		of   string
		want []string
	}{
		{
			name:      "link to a container",
			monitored: deploy(deployed{name: "app", dependsOn: "db"}, deployed{name: "db"}),
			of:        "db",
			want:      []string{"app"},
		},
		{
			name: "Watchtower routed through a VPN container",
			monitored: deploy(
				deployed{name: "watchtower", watchtower: true, networkMode: "container:vpn"},
				deployed{name: "vpn"},
			),
			of:   "vpn",
			want: []string{"watchtower"},
		},
		{
			name: "Watchtower named by name and by ID",
			monitored: deploy(
				deployed{name: "watchtower", watchtower: true},
				deployed{name: "app", dependsOn: "watchtower"},
				deployed{name: "sidecar", networkMode: "container:" + dockerID("watchtower")},
			),
			of:   "watchtower",
			want: []string{"app", "sidecar"},
		},
		{
			name: "Watchtower named without its replica number",
			monitored: deploy(
				deployed{name: "watchtower-1", watchtower: true},
				deployed{name: "app", dependsOn: "watchtower"},
			),
			of:   "watchtower-1",
			want: []string{"app"},
		},
		{
			name: "Watchtower not named exactly",
			monitored: deploy(
				deployed{name: "watchtower-main", watchtower: true},
				deployed{name: "app", dependsOn: "watchtower"},
			),
			of:   "watchtower-main",
			want: nil,
		},
		{
			name: "Watchtower run by Compose",
			monitored: deploy(
				deployed{name: "infra-watchtower-1", project: "infra", service: "watchtower", number: "1", watchtower: true},
				deployed{name: "infra-app-1", project: "infra", service: "app", number: "1", composeDependsOn: "watchtower"},
				deployed{name: "tool", dependsOn: "infra-watchtower"},
				deployed{name: "bare", dependsOn: "watchtower"},
			),
			useComposeDependsOn: true,
			of:                  "infra-watchtower-1",
			want:                []string{"infra-app-1", "tool"},
		},
		{
			name: "Watchtower replica name taken by an unmonitored container",
			monitored: deploy(
				deployed{name: "infra-watchtower-1", project: "infra", service: "watchtower", number: "1", watchtower: true},
				deployed{name: "app", dependsOn: "infra-watchtower"},
				deployed{name: "tool", dependsOn: "infra-watchtower-1"},
			),
			unmonitored: deploy(deployed{name: "infra-watchtower"}),
			of:          "infra-watchtower-1",
			want:        []string{"tool"},
		},
		{
			name: "Watchtower run by Compose with two replicas",
			monitored: deploy(
				deployed{name: "infra-watchtower-1", project: "infra", service: "watchtower", number: "1", watchtower: true},
				deployed{name: "infra-watchtower-2", project: "infra", service: "watchtower", number: "2", watchtower: true},
				deployed{name: "tool", dependsOn: "infra-watchtower"},
			),
			of:   "infra-watchtower-1",
			want: nil,
		},
		{
			name: "unmonitored container named exactly",
			monitored: deploy(
				deployed{name: "app", dependsOn: "db"},
				deployed{name: "db-1"},
			),
			unmonitored: deploy(deployed{name: "db"}),
			of:          "db-1",
			want:        nil,
		},
		{
			name: "service of its own project unmonitored",
			monitored: deploy(
				deployed{name: "a-web-1", project: "a", service: "web", number: "1", dependsOn: "db"},
				deployed{name: "b-db-1", project: "b", service: "db", number: "1"},
				deployed{name: "c-db-1", project: "c", service: "db", number: "1"},
			),
			unmonitored: deploy(deployed{name: "a-db-1", project: "a", service: "db", number: "1"}),
			of:          "b-db-1",
			want:        nil,
		},
		{
			name: "service unmonitored in another project",
			monitored: deploy(
				deployed{name: "a-web-1", project: "a", service: "web", number: "1", dependsOn: "db"},
				deployed{name: "b-db-1", project: "b", service: "db", number: "1"},
			),
			unmonitored: deploy(deployed{name: "c-db-1", project: "c", service: "db", number: "1"}),
			of:          "b-db-1",
			want:        []string{"a-web-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph, err := NewDependencyGraph(testLog(), tt.monitored, tt.unmonitored, tt.useComposeDependsOn)
			require.NoError(t, err)

			of := pick(tt.monitored, tt.of)
			require.Len(t, of, 1)

			dependents := graph.Dependents(of[0])
			if tt.want == nil {
				assert.Empty(t, dependents)
			} else {
				assert.Equal(t, tt.want, containerNames(dependents))
			}

			// Watchtower and unmonitored containers never become ordering edges.
			containerMap, _, adjacency, _, err := graph.subgraph(graph.containers)
			require.NoError(t, err)

			for dependency, dependents := range adjacency {
				for _, dependent := range dependents {
					assert.False(t, containerMap[dependency].IsWatchtower() || containerMap[dependent].IsWatchtower())
				}
			}
		})
	}
}

// TestDependencyGraph_UnresolvedDependsOn verifies that a Watchtower
// depends-on entry is reported when it names no container, and is not
// reported when it names a container, an unmonitored container by name or
// service, a Watchtower container, a service, a service shared by several
// projects, or the labeled container itself.
func TestDependencyGraph_UnresolvedDependsOn(t *testing.T) {
	t.Parallel()

	monitored := deploy(
		deployed{name: "app", dependsOn: "missing,db,api,cache,watchtower,app,legacy,store"},
		deployed{name: "watchtower", watchtower: true, dependsOn: "proxy,gone"},
		deployed{name: "proxy"},
		deployed{name: "a-db-1", project: "a", service: "db", number: "1"},
		deployed{name: "b-db-1", project: "b", service: "db", number: "1"},
		deployed{name: "stack-api-1", project: "stack", service: "api", number: "1"},
		deployed{name: "stack-api-2", project: "stack", service: "api", number: "2"},
		deployed{name: "cache"},
	)
	unmonitored := deploy(
		deployed{name: "legacy"},
		deployed{name: "x-store-1", project: "x", service: "store", number: "1"},
	)

	graph, err := NewDependencyGraph(testLog(), monitored, unmonitored, false)
	require.NoError(t, err)

	unresolved := graph.UnresolvedDependsOn()

	got := make([]string, 0, len(unresolved))
	for _, link := range unresolved {
		got = append(got, link.Container.Name()+":"+link.Link)
	}

	assert.ElementsMatch(t, []string{"app:missing", "watchtower:gone"}, got)
}
