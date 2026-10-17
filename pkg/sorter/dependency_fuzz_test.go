package sorter

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/internal/util"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockSorter "github.com/nicholas-fedor/watchtower/pkg/sorter/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// FuzzDependencySort fuzz tests the dependency sorting algorithm with random inputs.
func FuzzDependencySort(f *testing.F) {
	// Add some seed corpus
	f.Add(int32(5), float32(0.5))  // Small graph with medium dependencies
	f.Add(int32(20), float32(0.2)) // Medium graph with low dependencies
	f.Add(int32(50), float32(0.8)) // Large graph with high dependencies

	f.Fuzz(func(t *testing.T, count int32, depFactor float32) {
		// Sanitize inputs
		if count < 1 {
			count = 1
		}

		if count > 200 { // Limit for fuzz testing
			count = 200
		}

		if depFactor < 0 {
			depFactor = 0
		}

		if depFactor > 1 {
			depFactor = 1
		}

		containers := generateBenchmarkContainers(int(count), float64(depFactor))

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// If there's an error, it should be a circularReferenceError or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
			}

			// Verify all containers are still present
			if len(testContainers) != len(containers) {
				t.Fatalf("Container count changed: %d -> %d", len(containers), len(testContainers))
			}

			// Verify all original containers are present in result
			originalNames := make(map[string]bool)
			for _, c := range containers {
				originalNames[c.Name()] = true
			}

			for _, c := range testContainers {
				if !originalNames[c.Name()] {
					t.Fatalf("Unknown container in result: %s", c.Name())
				}
			}
		}
	})
}

// FuzzCycleDetection fuzz tests cycle detection specifically.
func FuzzCycleDetection(f *testing.F) {
	// Add seed corpus with known cycle patterns
	f.Add([]byte("A->B,B->A"))      // Simple cycle
	f.Add([]byte("A->B,B->C,C->A")) // Triangle cycle
	f.Add([]byte("A->B,B->C,C->D")) // No cycle
	f.Add([]byte("A->B,B->A,C->D")) // Cycle with separate component

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse the fuzz data into a dependency graph
		containers := parseFuzzDependencyData(data)

		if len(containers) == 0 {
			return // Skip empty inputs
		}

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// Error should either be nil, circularReferenceError, or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
			}
		}
	})
}

// parseFuzzDependencyData parses fuzz input into containers with dependencies
// Format: "A->B,B->C" becomes containers A,B,C with A depending on B, B depending on C.
func parseFuzzDependencyData(data []byte) []types.Container {
	if len(data) == 0 {
		return nil
	}

	// Simple parsing: split by comma, then by ->
	dependencies := make(map[string][]string)
	allNodes := make(map[string]bool)

	parts := bytes.SplitSeq(data, []byte(","))
	for part := range parts {
		part = bytes.TrimSpace(part)
		if len(part) == 0 {
			continue
		}

		before, after, ok := bytes.Cut(part, []byte("->"))
		if !ok {
			// Single node
			node := string(bytes.TrimSpace(part))
			if node != "" {
				allNodes[node] = true
			}

			continue
		}

		from := string(bytes.TrimSpace(before))
		to := string(bytes.TrimSpace(after))

		if from != "" && to != "" {
			dependencies[from] = append(dependencies[from], to)
			allNodes[from] = true
			allNodes[to] = true
		}
	}

	// Create containers
	containers := make([]types.Container, 0, len(allNodes))

	i := 0

	for node := range allNodes {
		links := dependencies[node]
		if links == nil {
			links = []string{}
		}

		container := &mockSorter.SimpleContainer{
			ContainerName:  node,
			ContainerID:    types.ContainerID(fmt.Sprintf("fuzz-id-%d", i)),
			ContainerLinks: links,
		}
		containers = append(containers, container)
		i++
	}

	return containers
}

// FuzzSortByDependencies fuzz tests the sortByDependencies function directly with random container configurations.
func FuzzSortByDependencies(f *testing.F) {
	// Add some seed corpus
	f.Add(int32(5), float32(0.5))  // Small graph with medium dependencies
	f.Add(int32(20), float32(0.2)) // Medium graph with low dependencies
	f.Add(int32(50), float32(0.8)) // Large graph with high dependencies
	f.Add(int32(1), float32(0.0))  // Single container
	f.Add(int32(0), float32(0.0))  // Empty list

	f.Fuzz(func(t *testing.T, count int32, depFactor float32) {
		// Sanitize inputs
		if count < 0 {
			count = 0
		}

		if count > 200 { // Limit for fuzz testing
			count = 200
		}

		if depFactor < 0 {
			depFactor = 0
		}

		if depFactor > 1 {
			depFactor = 1
		}

		containers := generateBenchmarkContainers(int(count), float64(depFactor))

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			// This should not panic
			sorted, err := sortByDependencies(testLog(), containers, useComposeDependsOn)
			// If there's an error, it should be a circularReferenceError or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
				// For cycles or collisions, sorted should be nil
				continue
			}

			// Verify all containers are present in result
			if len(sorted) != len(containers) {
				t.Fatalf("Sorted length mismatch: expected %d, got %d", len(containers), len(sorted))
			}

			// Verify all original containers are present in result
			originalNames := make(map[string]bool)
			for _, c := range containers {
				originalNames[c.Name()] = true
			}

			for _, c := range sorted {
				if !originalNames[c.Name()] {
					t.Fatalf("Unknown container in result: %s", c.Name())
				}
			}

			// Verify no duplicates in result
			resultNames := make(map[string]bool)

			for _, c := range sorted {
				name := c.Name()
				if resultNames[name] {
					t.Fatalf("Duplicate container in result: %s", name)
				}

				resultNames[name] = true
			}
		}
	})
}

// generateCollisionContainers creates containers that could have identifier collisions.
func generateCollisionContainers(data []byte) []types.Container {
	containers := make([]types.Container, 0, 10)

	// Parse data to determine collision scenario

	switch {
	case bytes.Contains(data, []byte("same-service-name")):
		// Create multiple containers with the same service name
		for i := range 5 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			// Override ContainerInfo to include service label
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "shared-service",
					},
				},
			}
			containers = append(containers, container)
		}

	case bytes.Contains(data, []byte("empty-service-names")):
		// Create containers with empty service names
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "",
					},
				},
			}
			containers = append(containers, container)
		}

	case bytes.Contains(data, []byte("leading-slash-service")):
		// Create containers with service names that have leading slashes
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": fmt.Sprintf("/service%d", i),
					},
				},
			}
			containers = append(containers, container)
		}

	default:
		// Create containers with service names derived from fuzz data
		parts := bytes.Split(data, []byte(","))
		for i, part := range parts {
			if len(part) == 0 {
				continue
			}

			serviceName := string(bytes.TrimSpace(part))
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": serviceName,
					},
				},
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// isRealCycle verifies that a detected cycle is actually real and not a false positive.
func isRealCycle(containers []types.Container, circularErr circularReferenceError, useComposeDependsOn bool) bool {
	cyclePath := circularErr.CyclePath

	// Edge case: cycle path must have at least 3 elements for a valid cycle (A->B->A)
	if len(cyclePath) < 3 {
		return false
	}

	// Create a map from normalized identifier to container for efficient lookup
	containerMap := make(map[string]types.Container)

	for _, c := range containers {
		normalizedIdentifier := util.NormalizeContainerName(
			container.ResolveContainerIdentifier(c),
		)
		containerMap[normalizedIdentifier] = c
	}

	// Check each consecutive pair in the cycle path (excluding the closing duplicate)
	for i := range len(cyclePath) - 1 {
		fromName := cyclePath[i]
		toName := cyclePath[i+1]

		fromContainer, exists := containerMap[fromName]
		if !exists {
			// Container not found in the list
			return false
		}

		links := fromContainer.Links(useComposeDependsOn)
		found := slices.Contains(links, toName)

		if !found {
			// Dependency link not found
			return false
		}
	}

	// All dependencies in the cycle are verified
	return true
}

// generateMalformedLabelContainers creates containers with malformed labels.
func generateMalformedLabelContainers(data []byte) []types.Container {
	containers := make([]types.Container, 0, 5)

	// Parse data to determine malformed label scenario

	switch {
	case bytes.Contains(data, []byte("empty-labels")):
		// Create containers with empty label maps
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, container)
		}

	case bytes.Contains(data, []byte("nil-labels")):
		// Create containers with nil labels
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: nil,
				},
			}
			containers = append(containers, container)
		}

	default:
		// Create containers with service names containing the fuzz data
		parts := bytes.Split(data, []byte(","))
		for i, part := range parts {
			if len(part) == 0 {
				continue
			}

			serviceName := string(part) // Include any special characters
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": serviceName,
					},
				},
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// generateLinkNormalizationContainers creates containers with various link normalization scenarios.
func generateLinkNormalizationContainers(data []byte) []types.Container {
	containers := make([]types.Container, 0, 5)

	// Parse data to determine link scenario
	switch {
	case bytes.Contains(data, []byte("leading-slash-links")):
		// Create containers with links that have leading slashes
		container := &mockSorter.SimpleContainer{
			ContainerName:  "container1",
			ContainerID:    types.ContainerID("id-1"),
			ContainerLinks: []string{"/dependency1", "/dependency2"},
		}
		container.ContainerInfoField = &dockerContainer.InspectResponse{
			Name: "/container1",
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		}
		containers = append(containers, container)

		// Add dependency containers
		for i := 1; i <= 2; i++ {
			dep := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("dependency%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("dep-id-%d", i)),
				ContainerLinks: []string{},
			}
			dep.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/dependency%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, dep)
		}

	case bytes.Contains(data, []byte("empty-links")):
		// Create containers with empty link entries
		container := &mockSorter.SimpleContainer{
			ContainerName:  "container1",
			ContainerID:    types.ContainerID("id-1"),
			ContainerLinks: []string{"", "dependency1", ""},
		}
		container.ContainerInfoField = &dockerContainer.InspectResponse{
			Name: "/container1",
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		}
		containers = append(containers, container)

		dep := &mockSorter.SimpleContainer{
			ContainerName:  "dependency1",
			ContainerID:    types.ContainerID("dep-id-1"),
			ContainerLinks: []string{},
		}
		dep.ContainerInfoField = &dockerContainer.InspectResponse{
			Name: "/dependency1",
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		}
		containers = append(containers, dep)

	case bytes.Contains(data, []byte("malformed-host-links")):
		// Create containers with malformed host config links
		container := &mockSorter.SimpleContainer{
			ContainerName:  "container1",
			ContainerID:    types.ContainerID("id-1"),
			ContainerLinks: []string{}, // Will be set via ContainerInfoField
		}
		container.ContainerInfoField = &dockerContainer.InspectResponse{
			Name: "/container1",
			HostConfig: &dockerContainer.HostConfig{
				Links: []string{
					"dependency1:alias1",
					"malformed-link",
					":empty-name",
					"dependency2:",
				},
			},
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		}
		containers = append(containers, container)

		// Add dependency containers
		for i := 1; i <= 2; i++ {
			dep := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("dependency%d", i),
				ContainerID:    types.ContainerID(fmt.Sprintf("dep-id-%d", i)),
				ContainerLinks: []string{},
			}
			dep.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/dependency%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, dep)
		}

	default:
		// Create containers with links derived from fuzz data
		parts := bytes.Split(data, []byte(","))
		if len(parts) > 0 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  "container1",
				ContainerID:    types.ContainerID("id-1"),
				ContainerLinks: make([]string, 0, len(parts)),
			}

			// Add links from fuzz data
			for _, part := range parts {
				if len(part) > 0 {
					container.ContainerLinks = append(container.ContainerLinks, string(part))
				}
			}

			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: "/container1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// generateEmptyIdentifierContainers creates containers with empty or missing identifiers.
func generateEmptyIdentifierContainers(data []byte) []types.Container {
	containers := make([]types.Container, 0, 5)

	// Parse data to determine empty identifier scenario

	switch {
	case bytes.Contains(data, []byte("empty-names")):
		// Create containers with empty names
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  "", // Empty name
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: "", // Empty name
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, container)
		}

	case bytes.Contains(data, []byte("nil-container-info")):
		// Create containers with nil container info
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:      fmt.Sprintf("container%d", i),
				ContainerID:        types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks:     []string{},
				ContainerInfoField: nil, // Nil container info
			}
			containers = append(containers, container)
		}

	case bytes.Contains(data, []byte("empty-ids")):
		// Create containers with empty IDs
		for i := range 3 {
			container := &mockSorter.SimpleContainer{
				ContainerName:  fmt.Sprintf("container%d", i),
				ContainerID:    "", // Empty ID
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: fmt.Sprintf("/container%d", i),
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, container)
		}

	default:
		// Create containers with identifiers derived from fuzz data
		parts := bytes.Split(data, []byte(","))
		for i, part := range parts {
			if len(part) == 0 {
				continue
			}

			name := string(bytes.TrimSpace(part))
			container := &mockSorter.SimpleContainer{
				ContainerName:  name,
				ContainerID:    types.ContainerID(fmt.Sprintf("id-%d", i)),
				ContainerLinks: []string{},
			}
			container.ContainerInfoField = &dockerContainer.InspectResponse{
				Name: "/" + name,
				Config: &dockerContainer.Config{
					Labels: map[string]string{},
				},
			}
			containers = append(containers, container)
		}
	}

	return containers
}

// FuzzIdentifierCollisions fuzz tests scenarios where container identifiers could collide,
// causing false positive circular reference detection.
func FuzzIdentifierCollisions(f *testing.F) {
	// Add seed corpus with collision scenarios
	f.Add([]byte("same-service-name"))     // Multiple containers with same service name
	f.Add([]byte("empty-service-names"))   // Containers with empty service names
	f.Add([]byte("leading-slash-service")) // Service names with leading slashes
	f.Add([]byte("special-chars-!@#$%"))   // Service names with special characters

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Create containers that could have identifier collisions
		containers := generateCollisionContainers(data)

		if len(containers) == 0 {
			return
		}

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic and should not detect false cycles
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// Error should either be nil, circularReferenceError, or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if errors.As(err, &circularErr) {
					// Verify the cycle is real, not a false positive due to identifier collision
					if !isRealCycle(containers, circularErr, useComposeDependsOn) {
						t.Fatalf("False positive cycle detected: %v", circularErr)
					}
				} else if !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
				// identifierCollisionError is expected and acceptable
			}
		}
	})
}

// FuzzMalformedLabels fuzz tests containers with malformed or edge-case labels
// that could cause identifier resolution issues.
func FuzzMalformedLabels(f *testing.F) {
	// Add seed corpus with malformed label scenarios
	f.Add([]byte("empty-labels"))           // Containers with empty label maps
	f.Add([]byte("nil-labels"))             // Containers with nil labels
	f.Add([]byte("malformed-service-name")) // Service names with newlines, tabs, etc.
	f.Add([]byte("unicode-service-\u00e9")) // Unicode characters in service names

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Create containers with malformed labels
		containers := generateMalformedLabelContainers(data)

		if len(containers) == 0 {
			return
		}

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// Error should either be nil, circularReferenceError, or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
			}
		}
	})
}

// FuzzLinkNormalization fuzz tests link parsing and normalization edge cases.
func FuzzLinkNormalization(f *testing.F) {
	// Add seed corpus with link normalization scenarios
	f.Add([]byte("leading-slash-links"))    // Links with leading slashes
	f.Add([]byte("empty-links"))            // Empty link entries
	f.Add([]byte("malformed-host-links"))   // Malformed host config links
	f.Add([]byte("special-char-links-!@#")) // Links with special characters

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Create containers with various link normalization scenarios
		containers := generateLinkNormalizationContainers(data)

		if len(containers) == 0 {
			return
		}

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// Error should either be nil, circularReferenceError, or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
			}
		}
	})
}

// FuzzEmptyIdentifiers fuzz tests containers with empty or missing identifiers.
func FuzzEmptyIdentifiers(f *testing.F) {
	// Add seed corpus with empty identifier scenarios
	f.Add([]byte("empty-names"))        // Containers with empty names
	f.Add([]byte("nil-container-info")) // Containers with nil container info
	f.Add([]byte("empty-ids"))          // Containers with empty IDs

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Create containers with empty/missing identifiers
		containers := generateEmptyIdentifierContainers(data)

		if len(containers) == 0 {
			return
		}

		// Test with both useComposeDependsOn values
		for _, useComposeDependsOn := range []bool{true, false} {
			ds := dependencySorter{}
			testContainers := make([]types.Container, len(containers))
			copy(testContainers, containers)

			// This should not panic
			err := ds.Sort(testLog(), testContainers, useComposeDependsOn)
			// Error should either be nil, circularReferenceError, or identifierCollisionError
			if err != nil {
				var (
					circularErr  circularReferenceError
					collisionErr identifierCollisionError
				)

				if !errors.As(err, &circularErr) && !errors.As(err, &collisionErr) {
					t.Fatalf("Unexpected error type: %T, error: %v", err, err)
				}
			}
		}
	})
}

// referenceFindMatchingIdentifiers is the link matcher as it scanned every
// identifier for each link. The indexed matcher must return the same matches,
// in the same order.
func referenceFindMatchingIdentifiers(link string, idSet map[string]bool) []string {
	if link == "" || len(idSet) == 0 {
		return nil
	}

	var matches []string

	if idSet[link] {
		matches = append(matches, link)

		return matches
	}

	var replicaMatches []string

	for identifier := range idSet {
		if strings.HasPrefix(identifier, link+"-") {
			suffix := identifier[len(link)+1:]
			if isPositiveInteger(suffix) {
				replicaMatches = append(replicaMatches, identifier)
			}
		}
	}

	if len(replicaMatches) > 0 {
		sort.Strings(replicaMatches)
		matches = append(matches, replicaMatches...)

		return matches
	}

	var serviceMatches []string

	services := make(map[string]bool)
	serviceSuffix := "-" + link
	linkHasHyphen := strings.Contains(link, "-")

	for identifier := range idSet {
		service := replicaService(identifier)

		if strings.HasSuffix(identifier, serviceSuffix) || strings.HasSuffix(service, serviceSuffix) ||
			(!linkHasHyphen && referenceExtractServiceName(identifier) == link) {
			serviceMatches = append(serviceMatches, identifier)
			services[service] = true
		}
	}

	if len(services) == 1 {
		sort.Strings(serviceMatches)
		matches = append(matches, serviceMatches...)
	}

	return matches
}

// referenceExtractServiceName is extractServiceName as it split the whole
// identifier on hyphens.
func referenceExtractServiceName(identifier string) string {
	if identifier == "" {
		return ""
	}

	if !strings.Contains(identifier, "-") {
		return identifier
	}

	parts := strings.Split(identifier, "-")
	if isPositiveInteger(parts[len(parts)-1]) {
		return parts[len(parts)-2]
	}

	return parts[len(parts)-1]
}

// FuzzMatchSetMatch verifies that the indexed link matcher returns
// exactly what scanning every identifier returns, matches and order alike,
// and that extractServiceName is unchanged. Identifiers are given as a
// comma-separated list. The seeds cover exact names, replicas, Compose
// services in one and several projects, hyphenated services, multi-segment
// links, and replica-like suffixes that are not positive integers.
func FuzzMatchSetMatch(f *testing.F) {
	f.Add("db", "app,db")
	f.Add("db", "stack-db-1,stack-db-2,stack-web-1")
	f.Add("db", "a-db-1,b-db-1")
	f.Add("db", "a-db-1,a-mongo-db-1,legacy-db")
	f.Add("api-gateway", "edge-api-gateway-1,edge-api-gateway-2")
	f.Add("api-gateway", "a-api-gateway-1,x-edge-api-gateway-1")
	f.Add("net-proxy", "edge-other-proxy-1")
	f.Add("stack-db", "stack-db-1,stack-db-2,stack-db-backup")
	f.Add("web", "web-0,web-01,web--1,web-x,web-1")
	f.Add("a", "a-1-1,a-1,a--1,-a,a-")
	f.Add("", "app")
	f.Add("db", "")

	f.Fuzz(func(t *testing.T, link, list string) {
		idSet := make(map[string]bool)

		for identifier := range strings.SplitSeq(list, ",") {
			if identifier != "" {
				idSet[identifier] = true

				if got, want := extractServiceName(identifier), referenceExtractServiceName(identifier); got != want {
					t.Fatalf("extractServiceName(%q) = %q, want %q", identifier, got, want)
				}
			}
		}

		got := newMatchSet(idSet).match(link)
		want := referenceFindMatchingIdentifiers(link, idSet)

		if !slices.Equal(got, want) {
			t.Fatalf("match(%q) in %v = %q, want %q", link, slices.Sorted(maps.Keys(idSet)), got, want)
		}
	})
}
