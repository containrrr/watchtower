package sorter

import (
	"fmt"
	"math/rand"
	"testing"

	mockSorter "github.com/nicholas-fedor/watchtower/pkg/sorter/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// generateBenchmarkContainers creates containers with realistic dependency patterns.
func generateBenchmarkContainers(count int, dependencyFactor float64) []types.Container {
	containers := make([]types.Container, count)

	// Create containers
	for i := range count {
		name := fmt.Sprintf("container-%d", i)
		container := &mockSorter.SimpleContainer{
			ContainerName:  name,
			ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
			ContainerLinks: []string{}, // Will be set below
		}
		containers[i] = container
	}

	// Add dependencies based on dependency factor

	for i := range count {
		container := containers[i].(*mockSorter.SimpleContainer)
		// Each container depends on up to 'dependencyFactor' percentage of previous containers
		maxDeps := min(int(float64(i)*dependencyFactor),
			// Cap at 5 dependencies per container for realism
			5)

		var links []string

		for range maxDeps {
			depIndex := rand.Intn(i + 1)
			if depIndex != i { // Don't depend on self
				depName := fmt.Sprintf("container-%d", depIndex)
				links = append(links, depName)
			}
		}

		container.ContainerLinks = links
	}

	return containers
}

// generateChainDependencies creates a linear dependency chain.
func generateChainDependencies(count int) []types.Container {
	containers := make([]types.Container, count)

	for i := range count {
		name := fmt.Sprintf("chain-%d", i)

		var links []string
		if i > 0 {
			links = []string{fmt.Sprintf("chain-%d", i-1)}
		}

		container := &mockSorter.SimpleContainer{
			ContainerName:  name,
			ContainerID:    types.ContainerID(fmt.Sprintf("chain-id-%d", i)),
			ContainerLinks: links,
		}

		containers[i] = container
	}

	return containers
}

// generateDiamondDependencies creates a diamond-shaped dependency graph.
func generateDiamondDependencies(levels int) []types.Container {
	var containers []types.Container

	// Create root
	root := &mockSorter.SimpleContainer{
		ContainerName:  "diamond-root",
		ContainerID:    "diamond-id-root",
		ContainerLinks: []string{},
	}
	containers = append(containers, root)

	// Create levels
	for level := 1; level <= levels; level++ {
		nodesInLevel := level * 2 // Linear growth per level

		for i := range nodesInLevel {
			name := fmt.Sprintf("diamond-l%d-n%d", level, i)

			var links []string

			// Connect to all nodes in previous level
			if level == 1 {
				links = []string{"diamond-root"}
			} else {
				for j := range (level - 1) * 2 {
					links = append(links, fmt.Sprintf("diamond-l%d-n%d", level-1, j))
				}
			}

			container := &mockSorter.SimpleContainer{
				ContainerName:  name,
				ContainerID:    types.ContainerID(fmt.Sprintf("diamond-id-l%d-n%d", level, i)),
				ContainerLinks: links,
			}

			containers = append(containers, container)
		}
	}

	return containers
}

// Benchmark Kahn's algorithm sorting performance.
func BenchmarkDependencySorter(b *testing.B) {
	testCases := []struct {
		name        string
		containers  func() []types.Container
		description string
	}{
		{
			name:        "Small_NoDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(10, 0.0) },
			description: "10 containers with no dependencies",
		},
		{
			name:        "Small_LowDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(10, 0.3) },
			description: "10 containers with low dependency factor",
		},
		{
			name:        "Medium_NoDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(100, 0.0) },
			description: "100 containers with no dependencies",
		},
		{
			name:        "Medium_LowDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(100, 0.3) },
			description: "100 containers with low dependency factor",
		},
		{
			name:        "Medium_HighDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(100, 0.8) },
			description: "100 containers with high dependency factor",
		},
		{
			name:        "Large_NoDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(1000, 0.0) },
			description: "1000 containers with no dependencies",
		},
		{
			name:        "Large_LowDeps",
			containers:  func() []types.Container { return generateBenchmarkContainers(1000, 0.3) },
			description: "1000 containers with low dependency factor",
		},
		{
			name:        "Chain_100",
			containers:  func() []types.Container { return generateChainDependencies(100) },
			description: "Linear dependency chain of 100 containers",
		},
		{
			name:        "Chain_500",
			containers:  func() []types.Container { return generateChainDependencies(500) },
			description: "Linear dependency chain of 500 containers",
		},
		{
			name:        "Diamond_3Levels",
			containers:  func() []types.Container { return generateDiamondDependencies(3) },
			description: "Diamond dependency graph with 3 levels",
		},
		{
			name:        "Diamond_4Levels",
			containers:  func() []types.Container { return generateDiamondDependencies(4) },
			description: "Diamond dependency graph with 4 levels",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			containers := tc.containers()
			benchLog := testLog()

			b.ResetTimer()

			for b.Loop() {
				ds := dependencySorter{}
				// Create a copy for each iteration to avoid modifying the original
				testContainers := make([]types.Container, len(containers))
				copy(testContainers, containers)

				err := ds.Sort(benchLog, testContainers, true)
				if err != nil {
					b.Fatalf("Unexpected error in benchmark %s: %v", tc.name, err)
				}
			}
		})
	}
}

// Benchmark cycle detection specifically.
func BenchmarkCycleDetection(b *testing.B) {
	testCases := []struct {
		name        string
		containers  func() []types.Container
		description string
		hasCycle    bool
	}{
		{
			name:        "NoCycle_Small",
			containers:  func() []types.Container { return generateBenchmarkContainers(50, 0.2) },
			description: "50 containers, no cycles",
			hasCycle:    false,
		},
		{
			name:        "NoCycle_Large",
			containers:  func() []types.Container { return generateBenchmarkContainers(500, 0.1) },
			description: "500 containers, no cycles",
			hasCycle:    false,
		},
		{
			name: "Cycle_Simple",
			containers: func() []types.Container {
				c1 := &mockSorter.SimpleContainer{
					ContainerName:  "c1",
					ContainerID:    "id1",
					ContainerLinks: []string{"c2"},
				}
				c2 := &mockSorter.SimpleContainer{
					ContainerName:  "c2",
					ContainerID:    "id2",
					ContainerLinks: []string{"c1"},
				}

				return []types.Container{c1, c2}
			},
			description: "Simple 2-node cycle",
			hasCycle:    true,
		},
		{
			name: "Cycle_Complex",
			containers: func() []types.Container {
				c1 := &mockSorter.SimpleContainer{
					ContainerName:  "c1",
					ContainerID:    "id1",
					ContainerLinks: []string{"c2"},
				}
				c2 := &mockSorter.SimpleContainer{
					ContainerName:  "c2",
					ContainerID:    "id2",
					ContainerLinks: []string{"c3"},
				}
				c3 := &mockSorter.SimpleContainer{
					ContainerName:  "c3",
					ContainerID:    "id3",
					ContainerLinks: []string{"c1"},
				}
				c4 := &mockSorter.SimpleContainer{
					ContainerName:  "c4",
					ContainerID:    "id4",
					ContainerLinks: []string{},
				}

				return []types.Container{c1, c2, c3, c4}
			},
			description: "3-node cycle with additional node",
			hasCycle:    true,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			containers := tc.containers()
			benchLog := testLog()

			b.ResetTimer()

			for b.Loop() {
				ds := dependencySorter{}
				testContainers := make([]types.Container, len(containers))
				copy(testContainers, containers)

				err := ds.Sort(benchLog, testContainers, true)
				if tc.hasCycle && err == nil {
					b.Fatalf("Expected cycle detection error in benchmark %s", tc.name)
				}

				if !tc.hasCycle && err != nil {
					b.Fatalf("Unexpected error in benchmark %s: %v", tc.name, err)
				}
			}
		})
	}
}

// Benchmark memory usage patterns.
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("MemoryEfficiency", func(b *testing.B) {
		containers := generateBenchmarkContainers(1000, 0.5)
		benchLog := testLog()

		b.ResetTimer()

		b.ReportAllocs()

		for b.Loop() {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			err := ds.Sort(benchLog, testContainers, true)
			if err != nil {
				b.Fatalf("Unexpected error: %v", err)
			}
		}
	})
}

// Benchmark sort stability - ensure consistent results for equivalent inputs.
func BenchmarkSortStability(b *testing.B) {
	var firstResult []string

	b.Run("StabilityCheck", func(b *testing.B) {
		containers := generateBenchmarkContainers(100, 0.3)
		benchLog := testLog()

		for b.Loop() {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			err := ds.Sort(benchLog, testContainers, true)
			if err != nil {
				b.Fatalf("Unexpected error: %v", err)
			}

			names := make([]string, len(testContainers))
			for j, c := range testContainers {
				names[j] = c.Name()
			}

			// Verify against first result
			if firstResult == nil {
				firstResult = names
			} else {
				for j := range firstResult {
					if firstResult[j] != names[j] {
						b.Fatalf("Sort not stable at position %d", j)
					}
				}
			}
		}
	})
}
