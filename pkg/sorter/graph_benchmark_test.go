package sorter

import (
	"fmt"
	"testing"

	mockSorter "github.com/nicholas-fedor/watchtower/pkg/sorter/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// cycleBenchmarkCase is a dependency graph that the cycle and sort benchmarks
// run against.
type cycleBenchmarkCase struct {
	// name identifies the graph in benchmark output.
	name string
	// containers builds the graph.
	containers func() []types.Container
}

// cycleBenchmarkCases returns graphs from acyclic to fully cyclic, at the sizes
// the dependency sort benchmarks use.
func cycleBenchmarkCases() []cycleBenchmarkCase {
	return []cycleBenchmarkCase{
		{name: "NoCycle_50", containers: func() []types.Container { return generateBenchmarkContainers(50, 0.2) }},
		{name: "NoCycle_500", containers: func() []types.Container { return generateBenchmarkContainers(500, 0.1) }},
		{name: "Chain_500", containers: func() []types.Container { return generateChainDependencies(500) }},
		{name: "Diamond_10", containers: func() []types.Container { return generateDiamondDependencies(10) }},
		{name: "Ring_500", containers: func() []types.Container { return generateRingDependencies(500) }},
	}
}

// generateRingDependencies creates containers that each depend on the next,
// with the last depending on the first, so every container is in one cycle.
func generateRingDependencies(count int) []types.Container {
	containers := make([]types.Container, count)

	for i := range count {
		containers[i] = &mockSorter.SimpleContainer{
			ContainerName:  fmt.Sprintf("ring-%d", i),
			ContainerID:    types.ContainerID(fmt.Sprintf("ring-id-%d", i)),
			ContainerLinks: []string{fmt.Sprintf("ring-%d", (i+1)%count)},
		}
	}

	return containers
}

// BenchmarkNewDependencyGraph measures building a dependency graph, which
// resolves every container's links once.
func BenchmarkNewDependencyGraph(b *testing.B) {
	for _, tc := range cycleBenchmarkCases() {
		b.Run(tc.name, func(b *testing.B) {
			containers := tc.containers()
			benchLog := newNopLogger()

			b.ReportAllocs()

			for b.Loop() {
				_, err := NewDependencyGraph(benchLog, containers, nil, true)
				if err != nil {
					b.Fatalf("NewDependencyGraph: %v", err)
				}
			}
		})
	}
}

// BenchmarkDependencyGraph_CycleMembers measures finding every container in a
// cycle of a built graph.
func BenchmarkDependencyGraph_CycleMembers(b *testing.B) {
	for _, tc := range cycleBenchmarkCases() {
		b.Run(tc.name, func(b *testing.B) {
			graph, err := NewDependencyGraph(newNopLogger(), tc.containers(), nil, true)
			if err != nil {
				b.Fatalf("NewDependencyGraph: %v", err)
			}

			b.ReportAllocs()

			for b.Loop() {
				_ = graph.CycleMembers()
			}
		})
	}
}

// BenchmarkDependencyGraph_Sort compares sorting a subset of containers with a
// graph of the full set, built beforehand, to sorting the subset on its own
// with SortByDependencies, which resolves the subset's links every time. The
// subset is every other container of an acyclic graph, as when only some
// containers restart.
func BenchmarkDependencyGraph_Sort(b *testing.B) {
	for _, tc := range cycleBenchmarkCases() {
		if tc.name == "Ring_500" {
			continue // A ring has no acyclic subset worth sorting.
		}

		all := tc.containers()

		subset := make([]types.Container, 0, len(all)/2+1)
		for i := 0; i < len(all); i += 2 {
			subset = append(subset, all[i])
		}

		b.Run(tc.name+"/Graph", func(b *testing.B) {
			benchLog := newNopLogger()
			sorted := make([]types.Container, len(subset))

			graph, err := NewDependencyGraph(benchLog, all, nil, true)
			if err != nil {
				b.Fatalf("NewDependencyGraph: %v", err)
			}

			b.ReportAllocs()

			for b.Loop() {
				copy(sorted, subset)

				err := graph.Sort(benchLog, sorted)
				if err != nil {
					b.Fatalf("Sort: %v", err)
				}
			}
		})

		b.Run(tc.name+"/Alone", func(b *testing.B) {
			benchLog := newNopLogger()
			sorted := make([]types.Container, len(subset))

			b.ReportAllocs()

			for b.Loop() {
				copy(sorted, subset)

				err := SortByDependencies(benchLog, sorted, true)
				if err != nil {
					b.Fatalf("SortByDependencies: %v", err)
				}
			}
		})
	}
}

// generateComposeHost creates a host running stacks Compose stacks. Each stack
// has a web service that depends on api, an api that depends on db and cache,
// and a worker that depends on db, with two replicas of api and worker. Every
// tenth stack also has a container routed through a VPN container by network
// mode and a backup container mounting the db volumes. A Watchtower container
// runs beside the stacks. Links come from real container labels and host
// config, as Container.Links reads them.
func generateComposeHost(stacks int) []types.Container {
	specs := []deployed{{name: "watchtower", watchtower: true}}

	for s := range stacks {
		project := fmt.Sprintf("stack%d", s)
		service := func(name, number, dependsOn string) deployed {
			return deployed{
				name:             project + "-" + name + "-" + number,
				project:          project,
				service:          name,
				number:           number,
				composeDependsOn: dependsOn,
			}
		}

		specs = append(specs,
			service("web", "1", "api:service_started:false"),
			service("api", "1", "db:service_healthy:false,cache:service_started:false"),
			service("api", "2", "db:service_healthy:false,cache:service_started:false"),
			service("worker", "1", "db:service_healthy:false"),
			service("worker", "2", "db:service_healthy:false"),
			service("db", "1", ""),
			service("cache", "1", ""),
		)

		if s%10 == 0 {
			vpn := project + "-vpn"
			specs = append(specs,
				deployed{name: vpn},
				deployed{name: project + "-torrent", networkMode: "container:" + dockerID(vpn)},
				deployed{name: project + "-backup", volumesFrom: []string{project + "-db-1:ro"}},
			)
		}
	}

	return deploy(specs...)
}

// BenchmarkScanOrdering measures the dependency ordering of one update scan on
// Compose hosts, as the update runs it: building the host's dependency graph,
// finding cycle members, sorting the containers outside them, and sorting the
// containers that restart (every tenth container) twice.
func BenchmarkScanOrdering(b *testing.B) {
	for _, stacks := range []int{2, 10, 70} {
		all := generateComposeHost(stacks)

		restart := make([]types.Container, 0, len(all)/10+1)
		for i := 0; i < len(all); i += 10 {
			restart = append(restart, all[i])
		}

		b.Run(fmt.Sprintf("Containers_%d", len(all)), func(b *testing.B) {
			benchLog := newNopLogger()
			sorted := make([]types.Container, len(all))
			restartSorted := make([]types.Container, len(restart))

			b.ReportAllocs()

			for b.Loop() {
				graph, err := NewDependencyGraph(benchLog, all, nil, true)
				if err != nil {
					b.Fatalf("NewDependencyGraph: %v", err)
				}

				members := graph.CycleMembers()
				if len(members) != 0 {
					b.Fatalf("%d cycle members", len(members))
				}

				copy(sorted, all)

				err = graph.Sort(benchLog, sorted)
				if err != nil {
					b.Fatalf("sorting the host: %v", err)
				}

				for range 2 {
					copy(restartSorted, restart)

					err = graph.Sort(benchLog, restartSorted)
					if err != nil {
						b.Fatalf("sorting the restarts: %v", err)
					}
				}
			}
		})
	}
}
