package actions_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_Dependencies verifies that an update stops and starts
// containers in dependency order, restarts the dependents of updated
// containers through links, network modes, and Compose depends_on, and collects
// the right images for cleanup.
func TestUpdateAction_Dependencies(t *testing.T) {
	t.Parallel()

	t.Run("updating containers with chained dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should process containers in dependency order", func(t *testing.T) {
			t.Parallel()

			// Create a dependency chain: A depends on B, B depends on C
			containerC := mockActions.CreateMockContainerWithConfig(
				"test-container-c",
				"/test-container-c",
				"fake-image-c:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"test-container-b",
				"/test-container-b",
				"fake-image-b:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "test-container-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"test-container-a",
				"/test-container-a",
				"fake-image-a:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "test-container-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerA,
						containerB,
						containerC,
					},
					Staleness: map[string]bool{
						"test-container-a": true,
						"test-container-b": true,
						"test-container-c": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 3)

			// Verify that all containers were updated (dependencies were handled correctly)
			require.Len(t, cleanupImageInfos, 3)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image-a:latest") }))
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image-b:latest") }))
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("fake-image-c:latest") }))
		})
	})

	t.Run("container is linked to restarting containers", func(t *testing.T) {
		t.Parallel()

		t.Run("should be marked for restart and collect image IDs", func(t *testing.T) {
			t.Parallel()

			provider := mockActions.CreateMockContainerWithConfig(
				"test-container-provider",
				"/test-container-provider",
				"fake-image2:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			provider.SetStale(true)

			consumer := mockActions.CreateMockContainerWithConfig(
				"test-container-consumer",
				"/test-container-consumer",
				"fake-image3:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "test-container-provider",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{
				provider,
				consumer,
			}

			assert.True(t, provider.ToRestart())
			assert.False(t, consumer.ToRestart())

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart())
		})

		t.Run("should propagate restart in Docker Compose with service name mismatch", func(t *testing.T) {
			t.Parallel()

			testData := getComposeTestData()
			containers := testData.Containers

			// db container is stale
			containers[0].SetStale(true)

			assert.True(t, containers[0].ToRestart())  // db
			assert.False(t, containers[1].ToRestart()) // web

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// web should be marked for restart because it depends on db
			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart())
		})

		t.Run("should propagate restart through multi-hop chains in Docker Compose", func(t *testing.T) {
			t.Parallel()

			testData := getComposeMultiHopTestData()
			containers := testData.Containers

			// cache container is stale
			containers[0].SetStale(true)

			assert.True(t, containers[0].ToRestart())  // cache
			assert.False(t, containers[1].ToRestart()) // db
			assert.False(t, containers[2].ToRestart()) // app

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// All should be marked for restart: cache -> db -> app
			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart())
			assert.True(t, containers[2].ToRestart())
		})

		t.Run("should propagate restart with project-prefixed service names in Docker Compose", func(t *testing.T) {
			t.Parallel()

			testData := getComposeProjectPrefixedTestData()
			containers := testData.Containers

			// db container is stale
			containers[0].SetStale(true)

			assert.True(t, containers[0].ToRestart())  // db
			assert.False(t, containers[1].ToRestart()) // web

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// web should be marked for restart because it depends on db
			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart())
		})

		t.Run("should propagate restart for hyphenated Compose project names with explicit container_name using real emitted labels", func(t *testing.T) {
			t.Parallel()

			testData := getComposeHyphenatedProjectTestData()
			containers := testData.Containers

			// base is stale
			containers[0].SetStale(true)

			assert.True(t, containers[0].ToRestart())  // base
			assert.False(t, containers[1].ToRestart()) // dependent

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart())
		})

		t.Run("should mark dependents for implicit restart with multi-hyphen Compose project names and explicit container_name", func(t *testing.T) {
			t.Parallel()

			testData := getHyphenatedProjectWithContainerNameTestData()
			containers := testData.Containers

			// base is stale
			containers[0].SetStale(true)

			assert.True(t, containers[0].ToRestart())  // base
			assert.False(t, containers[1].ToRestart()) // dependent-simple
			assert.False(t, containers[2].ToRestart()) // dependent-network

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			assert.True(t, containers[0].ToRestart())
			assert.True(t, containers[1].ToRestart(), "dependent-simple should be marked via depends_on")
			assert.True(t, containers[2].ToRestart(), "dependent-network should be marked via depends_on + network_mode")
		})

		t.Run("should NOT propagate restart via compose depends_on when UseComposeDependsOn is false", func(t *testing.T) {
			t.Parallel()

			// Create containers with compose depends_on label
			dbContainer := mockActions.CreateMockContainerWithConfig(
				"myproject_db_1",
				"/myproject_db_1",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Image: "postgres:latest",
					Labels: map[string]string{
						"com.docker.compose.service": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			webContainer := mockActions.CreateMockContainerWithConfig(
				"myproject_web_1",
				"/myproject_web_1",
				"web:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "web:latest",
					Labels: map[string]string{
						"com.docker.compose.service":    "web",
						"com.docker.compose.depends_on": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{dbContainer, webContainer}

			// db container is stale
			dbContainer.SetStale(true)

			assert.True(t, dbContainer.ToRestart())
			assert.False(t, webContainer.ToRestart())

			// Call UpdateImplicitRestart with UseComposeDependsOn = false
			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, false)

			// web should NOT be marked for restart because compose depends_on is disabled
			assert.True(t, dbContainer.ToRestart())
			assert.False(t, webContainer.ToRestart())
		})

		t.Run("should still propagate restart via watchtower depends-on when UseComposeDependsOn is false", func(t *testing.T) {
			t.Parallel()

			// Create containers with watchtower depends-on label (should still work)
			dbContainer := mockActions.CreateMockContainerWithConfig(
				"myproject_db_1",
				"/myproject_db_1",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Image:        "postgres:latest",
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			webContainer := mockActions.CreateMockContainerWithConfig(
				"myproject_web_1",
				"/myproject_web_1",
				"web:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "web:latest",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "myproject_db_1",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{dbContainer, webContainer}

			// db container is stale
			dbContainer.SetStale(true)

			assert.True(t, dbContainer.ToRestart())
			assert.False(t, webContainer.ToRestart())

			// Call UpdateImplicitRestart with UseComposeDependsOn = false
			// But watchtower depends-on should still work
			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, false)

			// web should be marked for restart because it depends on db via watchtower label
			assert.True(t, dbContainer.ToRestart())
			assert.True(t, webContainer.ToRestart())
		})
		t.Run("should propagate restart through chained dependencies", func(t *testing.T) {
			t.Parallel()

			// Create a transitive dependency chain: A depends on B, B depends on C
			containerC := mockActions.CreateMockContainerWithConfig(
				"test-container-c",
				"/test-container-c",
				"fake-image-c:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"test-container-b",
				"/test-container-b",
				"fake-image-b:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "test-container-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"test-container-a",
				"/test-container-a",
				"fake-image-a:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "test-container-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{
				containerC,
				containerB,
				containerA,
			}

			// Initially, only C should be marked for restart
			containerC.SetStale(true)
			assert.True(t, containerC.ToRestart())
			assert.False(t, containerB.ToRestart())
			assert.False(t, containerA.ToRestart())

			// Run UpdateImplicitRestart to propagate restart through the chain
			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// Verify that restart propagates: A and B should now be marked for restart
			assert.True(t, containers[0].ToRestart()) // C
			assert.True(t, containers[1].ToRestart()) // B
			assert.True(t, containers[2].ToRestart()) // A
		})
	})

	t.Run("handling edge cases in dependency resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle malformed labels and missing containers gracefully", func(t *testing.T) {
			t.Parallel()

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithConfig(
							"container-with-malformed-depends",
							"/container-with-malformed-depends",
							"image:latest",
							true,
							false,
							time.Now().AddDate(0, 0, -1),
							&dockerContainer.Config{
								Labels: map[string]string{
									"com.centurylinklabs.watchtower.depends-on": "non-existent, malformed name",
								},
								ExposedPorts: dockerNetwork.PortSet{},
							},
						),
					},
					Staleness: map[string]bool{
						"container-with-malformed-depends": true,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
		})

		t.Run("should skip containers with no usable identifier without breaking restart propagation", func(t *testing.T) {
			t.Parallel()

			// Container that will cause ResolveContainerIdentifier to return ""
			// (no name, no ID, no useful labels).
			badContainer := mockActions.CreateMockContainerWithConfig(
				"", "",
				"bad:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			base := mockActions.CreateMockContainerWithConfig(
				"base",
				"/base",
				"base:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "test-project",
						"com.docker.compose.service": "base",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent",
				"/dependent",
				"dep:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":    "test-project",
						"com.docker.compose.service":    "dependent",
						"com.docker.compose.depends_on": "base:service_started:false",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{badContainer, base, dependent}
			base.SetStale(true)

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// The valid dependent should still be marked despite the bad container
			assert.True(t, dependent.ToRestart())
		})

		t.Run("should resolve a qualified link to a replica even without project label on dependent", func(t *testing.T) {
			t.Parallel()

			// Dependent has no project label and a hyphenated link from compose depends_on.
			// The restarting candidate is a replica, so the link resolves through the
			// replica prefix match.
			base := mockActions.CreateMockContainerWithConfig(
				"project-base-1",
				"/project-base-1",
				"base:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":          "project",
						"com.docker.compose.service":          "base",
						"com.docker.compose.container-number": "1",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent with no project label
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent",
				"/dependent",
				"dep:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service":    "dependent",
						"com.docker.compose.depends_on": "project-base:service_started:false",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{base, dependent}
			base.SetStale(true)

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// The qualified link resolves to the restarting replica.
			assert.True(t, dependent.ToRestart())
		})

		t.Run("should handle the replica special case in ResolveContainerIdentifier for restart decisions", func(t *testing.T) {
			t.Parallel()

			// This tests the early return in ResolveContainerIdentifier:
			// if the container's runtime name matches the pattern "project-service-N",
			// it returns the runtime name directly instead of the constructed form.
			// This can happen with certain container_name configurations.

			// Base container whose Name() will trigger the special case
			base := mockActions.CreateMockContainerWithConfig(
				"myproject-db-1", // runtime name matches project-service-N pattern
				"/myproject-db-1",
				"db:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "myproject",
						"com.docker.compose.service": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			dependent := mockActions.CreateMockContainerWithConfig(
				"myproject-app-1",
				"/myproject-app-1",
				"app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":    "myproject",
						"com.docker.compose.service":    "app",
						"com.docker.compose.depends_on": "myproject-db:service_started:false",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{base, dependent}
			base.SetStale(true)

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// The dependent should still be marked, even though the base resolved
			// to its runtime name "myproject-db-1" due to the special case.
			assert.True(t, dependent.ToRestart())
		})

		t.Run("should resolve dependencies that come only from network_mode via host config", func(t *testing.T) {
			t.Parallel()

			// This covers the case where a container has no com.docker.compose.depends_on
			// label and no watchtower depends-on label, so the link must come from
			// getLinksFromHostConfig (network_mode: container:xxx).
			base := mockActions.CreateMockContainerWithConfig(
				"base",
				"/base",
				"base:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "test",
						"com.docker.compose.service": "base",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent that declares the relationship only via HostConfig.NetworkMode
			// (no compose depends_on label, no watchtower depends-on label).
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent",
				"/dependent",
				"dep:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "test",
						"com.docker.compose.service": "dependent",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			dependent.ContainerInfo().HostConfig.NetworkMode = "container:base"

			containers := []types.Container{base, dependent}
			base.SetStale(true)

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			assert.True(t, dependent.ToRestart())
		})

		t.Run("should ensure dependencies are stopped and started in correct order", func(t *testing.T) {
			t.Parallel()

			// Create dependency chain: A depends on B, B depends on C
			containers := createDependencyChain(
				[]string{"container-a", "container-b", "container-c"},
			)
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: containers,
					Staleness: map[string]bool{
						"container-a": true,
						"container-b": true,
						"container-c": true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 3)
			require.Len(t, cleanupImageInfos, 3)
			// Verify stop order: dependents first (reverse dependency order)
			assert.Equal(t, []string{"container-a", "container-b", "container-c"}, client.TestData.StopOrder)
			// Verify create order: dependencies first
			assert.Equal(t, []string{"container-c", "container-b", "container-a"}, client.TestData.CreateOrder)
		})
	})
	t.Run("only necessary containers restarted", func(t *testing.T) {
		t.Parallel()

		t.Run("should verify that only dependent containers are restarted when dependencies update", func(t *testing.T) {
			t.Parallel()

			// Create base container (stale)
			base := mockActions.CreateMockContainerWithConfig(
				"base",
				"/base",
				"base:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			// Dependent
			dep1 := mockActions.CreateMockContainerWithConfig(
				"dep1",
				"/dep1",
				"dep1:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "base",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			// Independent
			indep := mockActions.CreateMockContainerWithConfig(
				"indep",
				"/indep",
				"indep:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{base, dep1, indep},
					Staleness: map[string]bool{
						"base":  true,
						"dep1":  false,
						"indep": false,
					},
				},
				false,
				false,
			)
			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "base", cleanupImageInfos[0].ContainerName)
			assert.True(t, dep1.ToRestart())
			assert.False(t, indep.ToRestart())
		})
	})

	t.Run("handling circular dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should detect circular dependencies and skip affected containers", func(t *testing.T) {
			t.Parallel()

			// Create containers with circular dependencies: A depends on B, B depends on A
			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a",
				"/container-a",
				"image-a:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b",
				"/container-b",
				"image-b:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-a",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerA,
						containerB,
					},
					Staleness: map[string]bool{
						"container-a": false,
						"container-b": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err, "Circular dependencies should not cause an error")
			require.Len(t, report.Skipped(), 2, "Both containers should be skipped due to circular dependency")
			assert.Empty(t, cleanupImageInfos, "No cleanup should occur for skipped containers")
		})
	})

	t.Run("handling depends-on with non-existent containers", func(t *testing.T) {
		t.Parallel()

		t.Run("should gracefully handle depends-on referencing containers that don't exist", func(t *testing.T) {
			t.Parallel()

			// Create a container that depends on a non-existent container
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent-container",
				"/dependent-container",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "non-existent-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						dependent,
					},
					Staleness: map[string]bool{
						"dependent-container": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err, "Non-existent dependencies should not cause errors")
			require.Len(t, report.Updated(), 1, "Dependent container should still be updated")

			// Verify that the container was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("dep-image:latest") }))
		})
	})

	t.Run("handling diamond dependency patterns", func(t *testing.T) {
		t.Parallel()

		t.Run("should propagate restart through multiple dependency paths", func(t *testing.T) {
			t.Parallel()

			// Create diamond pattern: A depends on B and C, both B and C depend on D
			containerD := mockActions.CreateMockContainerWithConfig(
				"container-d",
				"/container-d",
				"image-d:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b",
				"/container-b",
				"image-b:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-d",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerC := mockActions.CreateMockContainerWithConfig(
				"container-c",
				"/container-c",
				"image-c:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-d",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a",
				"/container-a",
				"image-a:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b,container-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerD,
						containerB,
						containerC,
						containerA,
					},
					Staleness: map[string]bool{
						"container-d": true,
						"container-b": false,
						"container-c": false,
						"container-a": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1) // Only base container D is stale and updated

			// Verify that base container was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("image-d:latest") }))
		})
	})

	t.Run("handling self-dependency in depends-on", func(t *testing.T) {
		t.Parallel()

		t.Run("should gracefully handle containers that depend on themselves", func(t *testing.T) {
			t.Parallel()

			// Create a container that depends on itself
			selfDependent := mockActions.CreateMockContainerWithConfig(
				"self-container",
				"/self-container",
				"self-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "self-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						selfDependent,
					},
					Staleness: map[string]bool{
						"self-container": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err, "Self-dependency should not cause an error")
			require.Len(t, report.Skipped(), 1, "Self-dependent container should be skipped")
			assert.Empty(t, cleanupImageInfos, "No cleanup should occur for skipped container")
		})
	})

	t.Run("handling malformed container names in depends-on", func(t *testing.T) {
		t.Parallel()

		t.Run("should gracefully handle depends-on with invalid container names", func(t *testing.T) {
			t.Parallel()

			// Create a container with malformed depends-on names
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent-container",
				"/dependent-container",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "invalid name,another-invalid",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						dependent,
					},
					Staleness: map[string]bool{
						"dependent-container": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err, "Malformed names should not cause errors")
			require.Len(t, report.Updated(), 1, "Dependent container should still be updated")

			// Verify that the container was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("dep-image:latest") }))
		})
	})

	t.Run("handling deep dependency chains", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle dependency chains with more than 3 levels", func(t *testing.T) {
			t.Parallel()

			// Create a deep dependency chain: A depends on B, B depends on C, C depends on D, D depends on E
			containerE := mockActions.CreateMockContainerWithConfig(
				"container-e",
				"/container-e",
				"image-e:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerD := mockActions.CreateMockContainerWithConfig(
				"container-d",
				"/container-d",
				"image-d:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-e",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerC := mockActions.CreateMockContainerWithConfig(
				"container-c",
				"/container-c",
				"image-c:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-d",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b",
				"/container-b",
				"image-b:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a",
				"/container-a",
				"image-a:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerE,
						containerD,
						containerC,
						containerB,
						containerA,
					},
					Staleness: map[string]bool{
						"container-e": true,
						"container-d": false,
						"container-c": false,
						"container-b": false,
						"container-a": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1) // Only base container E is stale and updated

			// Verify that base container was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("image-e:latest") }))
		})
	})

	t.Run("handling mixed valid and invalid dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should handle containers with some valid and some invalid dependencies", func(t *testing.T) {
			t.Parallel()

			// Create containers where A depends on both a valid container B and an invalid container
			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b",
				"/container-b",
				"image-b:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a",
				"/container-a",
				"image-a:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b,non-existent-container",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerB,
						containerA,
					},
					Staleness: map[string]bool{
						"container-b": true,
						"container-a": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err, "Mixed valid/invalid dependencies should not cause errors")
			require.Len(t, report.Updated(), 1, "Only stale container should be updated")

			// Verify that base container was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("image-b:latest") }))
		})
	})

	t.Run("handling missing dependency propagation when labels are on dependents", func(t *testing.T) {
		t.Parallel()

		t.Run("should propagate restart when dependent has depends-on label but dependency has no labels", func(t *testing.T) {
			t.Parallel()

			// Dependency container with no labels
			dependency := mockActions.CreateMockContainerWithConfig(
				"dependency-no-labels",
				"/dependency-no-labels",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent container with depends-on label
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent-with-label",
				"/dependent-with-label",
				"dep-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "dependency-no-labels",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{dependency, dependent}

			// Initially, only dependency should be marked for restart
			dependency.SetStale(true)
			assert.True(t, dependency.ToRestart())
			assert.False(t, dependent.ToRestart())

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// Verify that restart propagates to dependent
			assert.True(t, containers[0].ToRestart()) // dependency
			assert.True(t, containers[1].ToRestart()) // dependent
		})
	})

	t.Run("handling correct rolling restart order in dependency chains", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop and start containers in correct order during rolling restart", func(t *testing.T) {
			t.Parallel()

			// Create dependency chain: A depends on B, B depends on C
			containerC := mockActions.CreateMockContainerWithConfig(
				"container-c-rolling",
				"/container-c-rolling",
				"image-c:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"container-b-rolling",
				"/container-b-rolling",
				"image-b:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-c-rolling",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"container-a-rolling",
				"/container-a-rolling",
				"image-a:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "container-b-rolling",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{containerC, containerB, containerA},
					Staleness: map[string]bool{
						"container-c-rolling": true,
						"container-b-rolling": true,
						"container-a-rolling": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{
					Cleanup:        true,
					RollingRestart: true,
				},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 3)

			// Verify rolling restart was used (WaitForContainerHealthy called)
			assert.Equal(t, int32(3), client.TestData.WaitForContainerHealthyCount.Load())

			// Verify cleanup occurred for all updated containers
			require.Len(t, cleanupImageInfos, 3)
		})
	})

	t.Run("handling independent staleness of dependents", func(t *testing.T) {
		t.Parallel()

		t.Run("should update dependent when it is stale even if dependency is not", func(t *testing.T) {
			t.Parallel()

			// Dependency that is not stale
			dependency := mockActions.CreateMockContainerWithConfig(
				"fresh-dependency",
				"/fresh-dependency",
				"dep-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent that is stale
			dependent := mockActions.CreateMockContainerWithConfig(
				"stale-dependent",
				"/stale-dependent",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "fresh-dependency",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{dependency, dependent},
					Staleness: map[string]bool{
						"fresh-dependency": false,
						"stale-dependent":  true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, report.Scanned(), 2)

			// Only the stale dependent should be updated
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "stale-dependent", cleanupImageInfos[0].ContainerName)
		})

		t.Run("should restart dependent when dependency becomes stale", func(t *testing.T) {
			t.Parallel()

			// Dependency that becomes stale
			dependency := mockActions.CreateMockContainerWithConfig(
				"stale-dependency",
				"/stale-dependency",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent that is not stale
			dependent := mockActions.CreateMockContainerWithConfig(
				"fresh-dependent",
				"/fresh-dependent",
				"dep-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "stale-dependency",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{dependency, dependent},
					Staleness: map[string]bool{
						"stale-dependency": true,
						"fresh-dependent":  false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, report.Scanned(), 2)

			// Only the stale dependency should be updated, dependent should be restarted implicitly
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "stale-dependency", cleanupImageInfos[0].ContainerName)

			// Verify that dependent was marked for restart
			assert.True(t, dependent.ToRestart())
		})
	})

	t.Run("handling dependency sorting with multiple replicas", func(t *testing.T) {
		t.Parallel()

		t.Run("should sort containers with multiple replicas without collisions", func(t *testing.T) {
			t.Parallel()

			// Create multiple replicas of a service: app-1, app-2, app-3, all depending on db
			dbContainer := mockActions.CreateMockContainerWithConfig(
				"db",
				"/db",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			app1 := mockActions.CreateMockContainerWithConfig(
				"app-1",
				"/app-1",
				"app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			app2 := mockActions.CreateMockContainerWithConfig(
				"app-2",
				"/app-2",
				"app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			app3 := mockActions.CreateMockContainerWithConfig(
				"app-3",
				"/app-3",
				"app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{app1, app2, app3, dbContainer},
					Staleness: map[string]bool{
						"db":    true,
						"app-1": false,
						"app-2": false,
						"app-3": false,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "db", cleanupImageInfos[0].ContainerName)

			// Verify that all dependent containers are marked for restart
			assert.True(t, app1.ToRestart())
			assert.True(t, app2.ToRestart())
			assert.True(t, app3.ToRestart())

			// Verify stop order: dependents first (in any order since they are equivalent), then db
			stopOrder := client.TestData.StopOrder
			require.Len(t, stopOrder, 4)
			assert.Contains(t, stopOrder, "db")
			assert.Contains(t, stopOrder, "app-1")
			assert.Contains(t, stopOrder, "app-2")
			assert.Contains(t, stopOrder, "app-3")
			// db should be last in stop order
			assert.Equal(t, "db", stopOrder[len(stopOrder)-1])

			// Verify create order: db first, then dependents
			createOrder := client.TestData.CreateOrder
			require.Len(t, createOrder, 4)
			assert.Equal(t, "db", createOrder[0])
			assert.Contains(t, createOrder, "app-1")
			assert.Contains(t, createOrder, "app-2")
			assert.Contains(t, createOrder, "app-3")
		})
	})

	t.Run("handling mixed Compose and Watchtower dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should prioritize Watchtower depends-on over Compose depends-on in mixed scenarios", func(t *testing.T) {
			t.Parallel()

			// Container with both Compose and Watchtower labels, where Watchtower points to different dependency
			cacheContainer := mockActions.CreateMockContainerWithConfig(
				"cache",
				"/cache",
				"redis:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			databaseContainer := mockActions.CreateMockContainerWithConfig(
				"database",
				"/database",
				"postgres:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "myapp",
						"com.docker.compose.service": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			webService := mockActions.CreateMockContainerWithConfig(
				"web-service",
				"/web-service",
				"nginx:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":                "myapp",
						"com.docker.compose.service":                "web",
						"com.docker.compose.depends_on":             `{"database":{"condition":"service_started"}}`,
						"com.centurylinklabs.watchtower.depends-on": "cache", // Watchtower takes precedence
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						cacheContainer,
						databaseContainer,
						webService,
					},
					Staleness: map[string]bool{
						"cache":       true,
						"database":    false,
						"web-service": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "cache", cleanupImageInfos[0].ContainerName)

			// web-service should be marked for restart due to Watchtower depends-on cache, not Compose depends-on database
			assert.True(t, webService.ToRestart())
			assert.False(t, databaseContainer.ToRestart())
		})
	})

	t.Run("handling the direction of implicit restart propagation", func(t *testing.T) {
		t.Parallel()

		t.Run("should propagate restart from dependency to dependent but not from dependent to dependency", func(t *testing.T) {
			t.Parallel()

			// Dependency container
			dependency := mockActions.CreateMockContainerWithConfig(
				"dependency-bidirectional",
				"/dependency-bidirectional",
				"dep-image:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Dependent container
			dependent := mockActions.CreateMockContainerWithConfig(
				"dependent-bidirectional",
				"/dependent-bidirectional",
				"dep-image:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "dependency-bidirectional",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containers := []types.Container{dependency, dependent}

			// Test propagation from dependency to dependent
			dependency.SetStale(true)
			assert.True(t, dependency.ToRestart())
			assert.False(t, dependent.ToRestart())

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			assert.True(t, containers[0].ToRestart()) // dependency
			assert.True(t, containers[1].ToRestart()) // dependent

			// Reset and test propagation from dependent to dependency
			dependency.SetStale(false)
			dependent.SetStale(true)
			dependency.SetLinkedToRestarting(false)
			dependent.SetLinkedToRestarting(false)

			assert.False(t, dependency.ToRestart())
			assert.True(t, dependent.ToRestart())

			actions.UpdateImplicitRestart(newTestLogger(t), containers, containers, true)

			// Restart does not propagate from the dependent to its dependency,
			// so the dependency is not marked as linked.
			assert.False(t, containers[0].ToRestart()) // dependency
			assert.True(t, containers[1].ToRestart())  // dependent
		})
	})

	t.Run("handling docker-compose scenario with multiple dependents on single dependency", func(t *testing.T) {
		t.Parallel()

		t.Run("should restart all dependents when single dependency updates", func(t *testing.T) {
			t.Parallel()

			// Single dependency (like a database)
			database := mockActions.CreateMockContainerWithConfig(
				"database",
				"/database",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Multiple dependents (like web apps)
			webApp1 := mockActions.CreateMockContainerWithConfig(
				"web-app-1",
				"/web-app-1",
				"web-app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			webApp2 := mockActions.CreateMockContainerWithConfig(
				"web-app-2",
				"/web-app-2",
				"web-app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			apiService := mockActions.CreateMockContainerWithConfig(
				"api-service",
				"/api-service",
				"api:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{database, webApp1, webApp2, apiService},
					Staleness: map[string]bool{
						"database":    true,
						"web-app-1":   false,
						"web-app-2":   false,
						"api-service": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1) // Only database is stale
			require.Len(t, report.Scanned(), 4)

			// Only database should be in cleanup (updated)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "database", cleanupImageInfos[0].ContainerName)

			// All dependents should be marked for restart
			assert.True(t, webApp1.ToRestart())
			assert.True(t, webApp2.ToRestart())
			assert.True(t, apiService.ToRestart())
		})
	})

	t.Run("handling complex dependency chains with rolling restart", func(t *testing.T) {
		t.Parallel()

		t.Run("should maintain correct stop/start order in complex chains", func(t *testing.T) {
			t.Parallel()

			// Create complex chain: A depends on B, B depends on C, D depends on C
			containerC := mockActions.CreateMockContainerWithConfig(
				"service-c",
				"/service-c",
				"service-c:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerB := mockActions.CreateMockContainerWithConfig(
				"service-b",
				"/service-b",
				"service-b:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "service-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerA := mockActions.CreateMockContainerWithConfig(
				"service-a",
				"/service-a",
				"service-a:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "service-b",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			containerD := mockActions.CreateMockContainerWithConfig(
				"service-d",
				"/service-d",
				"service-d:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "service-c",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						containerC,
						containerB,
						containerA,
						containerD,
					},
					Staleness: map[string]bool{
						"service-c": true,
						"service-b": true,
						"service-a": true,
						"service-d": true,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true, RollingRestart: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 4)

			// Verify rolling restart was used for all containers
			assert.Equal(t, int32(4), client.TestData.WaitForContainerHealthyCount.Load())

			// Verify cleanup occurred for all updated containers
			require.Len(t, cleanupImageInfos, 4)
		})
	})

	t.Run("handling diamond dependency scenario with Docker Compose labels", func(t *testing.T) {
		t.Parallel()

		t.Run("should stop and start containers in correct dependency order", func(t *testing.T) {
			t.Parallel()

			// Create diamond dependency: a-database (base), b-service1 and d-service3 depend on a-database, c-service2 depends on b-service1
			aDatabase := mockActions.CreateMockContainerWithConfig(
				"a-database",
				"/a-database",
				"postgres:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			bService1 := mockActions.CreateMockContainerWithConfig(
				"b-service1",
				"/b-service1",
				"service1:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "a-database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			cService2 := mockActions.CreateMockContainerWithConfig(
				"c-service2",
				"/c-service2",
				"service2:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "b-service1",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			dService3 := mockActions.CreateMockContainerWithConfig(
				"d-service3",
				"/d-service3",
				"service3:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "a-database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{aDatabase, bService1, dService3, cService2},
					Staleness: map[string]bool{
						"a-database": false,
						"b-service1": true,
						"c-service2": true,
						"d-service3": true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 3)
			// b, c, d are stale and updated

			// Verify stop order: reverse dependency order
			assert.Equal(t, []string{"c-service2", "b-service1", "d-service3"}, client.TestData.StopOrder)

			// Verify create order: dependency order
			assert.Equal(t, []string{"d-service3", "b-service1", "c-service2"}, client.TestData.CreateOrder)

			// Verify cleanup for updated containers
			require.Len(t, cleanupImageInfos, 3)
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ContainerName == "b-service1" }))
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ContainerName == "c-service2" }))
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ContainerName == "d-service3" }))
		})
	})

	t.Run("handling network mode dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should restart containers that depend on updated containers via network mode", func(t *testing.T) {
			t.Parallel()

			testData := getNetworkModeTestData()
			client := mockActions.CreateMockClient(testData, false, false)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			// Only the stale network-dependency container is updated
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "network-dependency", cleanupImageInfos[0].ContainerName)

			// The dependent container should be marked for restart
			containers := testData.Containers
			assert.Equal(t, "network-dependency", containers[0].Name())
			// stale container
			assert.Equal(t, "network-dependent", containers[1].Name())
			// dependent container
			assert.True(t, containers[1].ToRestart())
		})

		t.Run("should restart containers with both compose depends_on and network_mode dependency", func(t *testing.T) {
			t.Parallel()

			testData := getComposeDependsOnWithNetworkModeTestData()
			client := mockActions.CreateMockClient(testData, false, false)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "download-stack-vpn-1", cleanupImageInfos[0].ContainerName)

			containers := testData.Containers
			assert.Equal(t, "download-stack-vpn-1", containers[0].Name())
			assert.Equal(t, "download-stack-web-1", containers[1].Name())

			// The dependent container should be marked for restart via compose depends_on
			assert.True(t, containers[1].ToRestart())

			// Verify stop order: dependent first, then dependency (reverse sorted order)
			assert.Equal(t, []string{"download-stack-web-1", "download-stack-vpn-1"}, client.TestData.StopOrder)

			// Verify create order: dependency first, then dependent
			assert.Equal(t, []string{"download-stack-vpn-1", "download-stack-web-1"}, client.TestData.CreateOrder)

			// Verify the dependent container preserves the network mode referencing the dependency
			newDependent := containers[1]
			networkMode := newDependent.ContainerInfo().HostConfig.NetworkMode
			assert.Equal(t, "container:download-stack-vpn-1", string(networkMode))
		})

		// Explicit container_name with Compose project/service labels and
		// depends-on / network_mode targeting the peer by container name.
		// Sort must put the network provider first so restart creates it before
		// the dependent joins its namespace.
		t.Run("should stop and start container_name network_mode peers in dependency order", func(t *testing.T) {
			t.Parallel()

			vpn := mockActions.CreateMockContainerWithConfig(
				"net-proxy",
				"/net-proxy",
				"vpn:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "myproject",
						"com.docker.compose.service":            "net-proxy",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			web := mockActions.CreateMockContainerWithConfig(
				"web-app",
				"/web-app",
				"nginx:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":                "myproject",
						"com.docker.compose.service":                "web",
						"com.centurylinklabs.watchtower.enable":     "true",
						"com.centurylinklabs.watchtower.depends-on": "net-proxy",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			web.ContainerInfo().HostConfig.NetworkMode = "container:net-proxy"

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{web, vpn},
					Staleness: map[string]bool{
						"web-app":   false,
						"net-proxy": true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, _, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.True(t, web.ToRestart())

			// Dependent first on stop; network provider first on create/start.
			assert.Equal(t, []string{"web-app", "net-proxy"}, client.TestData.StopOrder)
			assert.Equal(t, []string{"net-proxy", "web-app"}, client.TestData.CreateOrder)
		})

		// Cross-project container_name: network_mode holds the provider's
		// real name, which does not belong to the dependent's Compose project.
		t.Run("should stop and start cross-project network_mode peers in dependency order", func(t *testing.T) {
			t.Parallel()

			gluetun := mockActions.CreateMockContainerWithConfig(
				"gluetun",
				"/gluetun",
				"gluetun:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "gluetun",
						"com.docker.compose.service":            "vpn",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			qbittorrent := mockActions.CreateMockContainerWithConfig(
				"qbittorrent",
				"/qbittorrent",
				"qbittorrent:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "qbittorrent",
						"com.docker.compose.service":            "app",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			qbittorrent.ContainerInfo().HostConfig.NetworkMode = "container:gluetun"

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{qbittorrent, gluetun},
					Staleness: map[string]bool{
						"qbittorrent": false,
						"gluetun":     true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, _, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.True(t, gluetun.ToRestart())
			assert.True(t, qbittorrent.ToRestart())

			assert.Equal(t, []string{"qbittorrent", "gluetun"}, client.TestData.StopOrder)
			assert.Equal(t, []string{"gluetun", "qbittorrent"}, client.TestData.CreateOrder)
		})

		// Compose-default provider name: network_mode points at
		// project-service-N rather than an explicit container_name.
		t.Run("should stop and start cross-project compose-default network_mode peers in dependency order", func(t *testing.T) {
			t.Parallel()

			gluetun := mockActions.CreateMockContainerWithConfig(
				"gluetun-vpn-1",
				"/gluetun-vpn-1",
				"gluetun:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "gluetun",
						"com.docker.compose.service":            "vpn",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			qbittorrent := mockActions.CreateMockContainerWithConfig(
				"qbittorrent-app-1",
				"/qbittorrent-app-1",
				"qbittorrent:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "qbittorrent",
						"com.docker.compose.service":            "app",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			qbittorrent.ContainerInfo().HostConfig.NetworkMode = "container:gluetun-vpn-1"

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{qbittorrent, gluetun},
					Staleness: map[string]bool{
						"qbittorrent-app-1": false,
						"gluetun-vpn-1":     true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, _, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.True(t, gluetun.ToRestart())
			assert.True(t, qbittorrent.ToRestart())

			assert.Equal(t, []string{"qbittorrent-app-1", "gluetun-vpn-1"}, client.TestData.StopOrder)
			assert.Equal(t, []string{"gluetun-vpn-1", "qbittorrent-app-1"}, client.TestData.CreateOrder)
		})

		// Inspect stores container:<id> until list rewrite. Matching and
		// sort order must still work on that form.
		t.Run("should stop and start cross-project network_mode peers referenced by container ID", func(t *testing.T) {
			t.Parallel()

			const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

			gluetun := mockActions.CreateMockContainerWithConfig(
				providerID,
				"/gluetun",
				"gluetun:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "gluetun",
						"com.docker.compose.service":            "vpn",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			qbittorrent := mockActions.CreateMockContainerWithConfig(
				"qbittorrent",
				"/qbittorrent",
				"qbittorrent:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "qbittorrent",
						"com.docker.compose.service":            "app",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			qbittorrent.ContainerInfo().HostConfig.NetworkMode = dockerContainer.NetworkMode(
				"container:" + providerID,
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{qbittorrent, gluetun},
					Staleness: map[string]bool{
						"qbittorrent": false,
						"gluetun":     true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, _, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.True(t, gluetun.ToRestart())
			assert.True(t, qbittorrent.ToRestart())

			assert.Equal(t, []string{"qbittorrent", "gluetun"}, client.TestData.StopOrder)
			assert.Equal(t, []string{"gluetun", "qbittorrent"}, client.TestData.CreateOrder)
		})

		// Provider image is stale, dependents are not. Implicit restart must
		// still pull them into the update and stop them before the provider.
		t.Run("should restart multiple cross-project network_mode dependents when the provider is stale", func(t *testing.T) {
			t.Parallel()

			gluetun := mockActions.CreateMockContainerWithConfig(
				"gluetun",
				"/gluetun",
				"gluetun:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "gluetun",
						"com.docker.compose.service":            "vpn",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			qbittorrent := mockActions.CreateMockContainerWithConfig(
				"qbittorrent",
				"/qbittorrent",
				"qbittorrent:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "qbittorrent",
						"com.docker.compose.service":            "app",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			qbittorrent.ContainerInfo().HostConfig.NetworkMode = "container:gluetun"

			prowlarr := mockActions.CreateMockContainerWithConfig(
				"prowlarr",
				"/prowlarr",
				"prowlarr:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":            "prowlarr",
						"com.docker.compose.service":            "app",
						"com.centurylinklabs.watchtower.enable": "true",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)
			prowlarr.ContainerInfo().HostConfig.NetworkMode = "container:/gluetun"

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{qbittorrent, prowlarr, gluetun},
					Staleness: map[string]bool{
						"qbittorrent": false,
						"prowlarr":    false,
						"gluetun":     true,
					},
					StopOrder:   []string{},
					CreateOrder: []string{},
					StartOrder:  []string{},
				},
				false,
				false,
			)

			report, _, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)
			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			assert.True(t, gluetun.ToRestart())
			assert.True(t, qbittorrent.ToRestart())
			assert.True(t, prowlarr.ToRestart())

			require.Len(t, client.TestData.StopOrder, 3)
			assert.Equal(t, "gluetun", client.TestData.StopOrder[2])
			assert.Subset(t, client.TestData.StopOrder, []string{"qbittorrent", "prowlarr"})
			require.Len(t, client.TestData.CreateOrder, 3)
			assert.Equal(t, "gluetun", client.TestData.CreateOrder[0])
			assert.Subset(t, client.TestData.CreateOrder, []string{"qbittorrent", "prowlarr"})
		})
	})

	t.Run("handling cross-project dependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve Watchtower depends-on labels referencing containers in different projects", func(t *testing.T) {
			t.Parallel()

			// Create containers from different projects
			app1Db := mockActions.CreateMockContainerWithConfig(
				"app1-database",
				"/app1-database",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "app1",
						"com.docker.compose.service": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			app2Web := mockActions.CreateMockContainerWithConfig(
				"app2-web",
				"/app2-web",
				"nginx:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "app2",
						"com.docker.compose.service": "web",
						// Watchtower depends-on referencing container from different project
						"com.centurylinklabs.watchtower.depends-on": "app1-database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{app1Db, app2Web},
					Staleness: map[string]bool{
						"app1-database": true,
						"app2-web":      false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "app1-database", cleanupImageInfos[0].ContainerName)

			// app2-web should be marked for restart due to cross-project dependency
			assert.True(t, app2Web.ToRestart())
		})

		t.Run("should prioritize Watchtower depends-on over Compose depends-on labels", func(t *testing.T) {
			t.Parallel()

			// Container with both Watchtower and Compose depends-on labels
			webService := mockActions.CreateMockContainerWithConfig(
				"web-service",
				"/web-service",
				"nginx:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":    "myproject",
						"com.docker.compose.service":    "web",
						"com.docker.compose.depends_on": `{"database":{"condition":"service_started"}}`,
						// Watchtower depends-on should take precedence
						"com.centurylinklabs.watchtower.depends-on": "external-cache",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			databaseService := mockActions.CreateMockContainerWithConfig(
				"myproject-database",
				"/myproject-database",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "myproject",
						"com.docker.compose.service": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			externalCache := mockActions.CreateMockContainerWithConfig(
				"external-cache",
				"/external-cache",
				"redis:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels:       map[string]string{},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{databaseService, externalCache, webService},
					Staleness: map[string]bool{
						"myproject-database": true,
						"external-cache":     true,
						"web-service":        false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 2)
			require.Len(t, cleanupImageInfos, 2)

			// web-service should be marked for restart due to Watchtower depends-on external-cache,
			// not due to Compose depends-on database (even though database is also stale)
			assert.True(t, webService.ToRestart())
		})

		t.Run("should distinguish same service names in different projects using full identifiers", func(t *testing.T) {
			t.Parallel()

			// Two services with same name but different projects
			project1Db := mockActions.CreateMockContainerWithConfig(
				"project1-database",
				"/project1-database",
				"postgres:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "project1",
						"com.docker.compose.service": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			project2Db := mockActions.CreateMockContainerWithConfig(
				"project2-database",
				"/project2-database",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "project2",
						"com.docker.compose.service": "database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Service that depends on database from project1 only
			appService := mockActions.CreateMockContainerWithConfig(
				"app-service",
				"/app-service",
				"app:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":                "project1",
						"com.docker.compose.service":                "app",
						"com.centurylinklabs.watchtower.depends-on": "project1-database",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{project1Db, project2Db, appService},
					Staleness: map[string]bool{
						"project1-database": false,
						"project2-database": true,
						"app-service":       false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "project2-database", cleanupImageInfos[0].ContainerName)

			// app-service should NOT be marked for restart because it depends on project1-database,
			// which is not stale, and project2-database being stale doesn't affect it
			assert.False(t, appService.ToRestart())
		})

		t.Run("should resolve conflicts using full project-service-number identifiers", func(t *testing.T) {
			t.Parallel()

			// Containers with similar names requiring full identifier resolution
			db1 := mockActions.CreateMockContainerWithConfig(
				"myapp-database-1",
				"/myapp-database-1",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":          "myapp",
						"com.docker.compose.service":          "database",
						"com.docker.compose.container-number": "1",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			db2 := mockActions.CreateMockContainerWithConfig(
				"myapp-database-2",
				"/myapp-database-2",
				"postgres:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":          "myapp",
						"com.docker.compose.service":          "database",
						"com.docker.compose.container-number": "2",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			// Worker that depends on specific database instance
			worker := mockActions.CreateMockContainerWithConfig(
				"worker",
				"/worker",
				"worker:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "myapp-database-1",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{db1, db2, worker},
					Staleness: map[string]bool{
						"myapp-database-1": true,
						"myapp-database-2": false,
						"worker":           false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "myapp-database-1", cleanupImageInfos[0].ContainerName)

			// Worker should be marked for restart due to dependency on specific database instance
			assert.True(t, worker.ToRestart())
		})

		t.Run("should handle mixed project and non-project containers with Watchtower dependencies", func(t *testing.T) {
			t.Parallel()

			// Mix of containers: some with project labels, some without
			projectDB := mockActions.CreateMockContainerWithConfig(
				"project-db",
				"/project-db",
				"postgres:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project": "myproject",
						"com.docker.compose.service": "db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			standaloneCache := mockActions.CreateMockContainerWithConfig(
				"standalone-cache",
				"/standalone-cache",
				"redis:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "project-db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			projectWeb := mockActions.CreateMockContainerWithConfig(
				"myproject-web",
				"/myproject-web",
				"nginx:latest",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.project":                "myproject",
						"com.docker.compose.service":                "web",
						"com.centurylinklabs.watchtower.depends-on": "standalone-cache",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{projectDB, standaloneCache, projectWeb},
					Staleness: map[string]bool{
						"project-db":       true,
						"standalone-cache": false,
						"myproject-web":    false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "project-db", cleanupImageInfos[0].ContainerName)

			// Both dependent containers should be marked for restart
			assert.True(t, standaloneCache.ToRestart())
			assert.True(t, projectWeb.ToRestart())
		})
	})

	t.Run("handling transitive dependencies with replica container names", func(t *testing.T) {
		t.Parallel()

		t.Run("should restart all containers in db → app → web chain when db is updated", func(t *testing.T) {
			t.Parallel()

			// Create containers with replica-style names (e.g., myproject-db-1)
			// and depends-on labels referencing service names without replica numbers
			// This reproduces the bug where exact matching fails for transitive dependencies
			dbContainer := mockActions.CreateMockContainerWithConfig(
				"myproject-db-1",
				"/myproject-db-1",
				"mariadb:latest",
				true,
				false,
				time.Now().AddDate(0, 0, -1), // Make it stale
				&dockerContainer.Config{
					Image:  "mariadb:latest",
					Labels: map[string]string{
						// No depends-on for db
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			appContainer := mockActions.CreateMockContainerWithConfig(
				"myproject-app-1",
				"/myproject-app-1",
				"php:fpm",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "php:fpm",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "myproject-db",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			webContainer := mockActions.CreateMockContainerWithConfig(
				"myproject-web-1",
				"/myproject-web-1",
				"nginx:alpine",
				true,
				false,
				time.Now(),
				&dockerContainer.Config{
					Image: "nginx:alpine",
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "myproject-app",
					},
					ExposedPorts: dockerNetwork.PortSet{},
				},
			)

			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{dbContainer, appContainer, webContainer},
					Staleness: map[string]bool{
						"myproject-db-1":  true,
						"myproject-app-1": false,
						"myproject-web-1": false,
					},
				},
				false,
				false,
			)

			report, cleanupImageInfos, err := actions.Update(newTestLogger(t),
				t.Context(),
				client,
				types.UpdateParams{Cleanup: true},
			)

			require.NoError(t, err)
			require.Len(t, report.Updated(), 1)
			// Only db is stale and updated

			// Verify db was updated
			require.Len(t, cleanupImageInfos, 1)
			assert.Equal(t, "myproject-db-1", cleanupImageInfos[0].ContainerName)

			// Verify transitive dependencies were restarted
			assert.True(t, appContainer.ToRestart(), "app should be restarted due to db update")
			assert.True(t, webContainer.ToRestart(), "web should be restarted due to transitive dependency on app")
		})
	})
}
