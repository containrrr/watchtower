package actions

import (
	"fmt"
	"io"
	"testing"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// benchmarkStackSize is the number of containers in each benchmark Compose stack.
const benchmarkStackSize = 7

// benchmarkComposeHost returns a host running stacks Compose stacks, each with
// a web service that depends on api, an api that depends on db and cache, and
// a worker that depends on db, with two replicas of api and worker.
func benchmarkComposeHost(stacks int) []types.Container {
	containers := make([]types.Container, 0, stacks*benchmarkStackSize)

	for s := range stacks {
		project := fmt.Sprintf("stack%d", s)

		for _, spec := range []struct{ service, number, dependsOn string }{
			{"web", "1", "api"},
			{"api", "1", "db,cache"},
			{"api", "2", "db,cache"},
			{"worker", "1", "db"},
			{"worker", "2", "db"},
			{"db", "1", ""},
			{"cache", "1", ""},
		} {
			containers = append(containers, restartSpec{
				name:      project + "-" + spec.service + "-" + spec.number,
				project:   project,
				service:   spec.service,
				number:    spec.number,
				dependsOn: spec.dependsOn,
			}.build())
		}
	}

	return containers
}

// BenchmarkImplicitRestarts measures the two implicit restart passes of an
// update scan on Compose hosts, with every tenth container restarting. The
// scan's dependency graph is built beforehand, as the update builds it once
// for ordering.
func BenchmarkImplicitRestarts(b *testing.B) {
	log := zerolog.New(io.Discard).Level(zerolog.InfoLevel)
	params := types.UpdateParams{}

	for _, stacks := range []int{2, 10, 70} {
		all := benchmarkComposeHost(stacks)

		b.Run(fmt.Sprintf("Containers_%d", len(all)), func(b *testing.B) {
			graph, err := sorter.NewDependencyGraph(&log, all, nil, false)
			if err != nil {
				b.Fatalf("NewDependencyGraph: %v", err)
			}

			b.ReportAllocs()

			for b.Loop() {
				for i, c := range all {
					c.SetStale(i%10 == 5)
				}

				_ = reconcileImplicitRestartsExcluding(&log, graph, all, all, params, nil)
				_ = reconcileImplicitRestartsExcluding(&log, graph, all, all, params, nil)
			}
		})
	}
}
