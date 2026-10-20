package sorter

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// newTestLogger returns a logger that writes each event to the test's log, so a
// failing test shows what it logged.
//
// Parameters:
//   - t: Test that receives the log lines.
//
// Returns:
//   - *zerolog.Logger: Logger writing to t.
func newTestLogger(t *testing.T) *zerolog.Logger {
	t.Helper()

	// Frame 4 skips zerolog's write path, so each line names the log call.
	w := zerolog.TestWriter{T: t, Frame: 4}

	return new(zerolog.New(w))
}

// newNopLogger returns a logger that discards every event, for benchmarks and
// fuzz targets.
//
// Returns:
//   - *zerolog.Logger: Logger that discards all events.
func newNopLogger() *zerolog.Logger {
	return new(zerolog.Nop())
}

// Test_dependencySorter verifies that containers are sorted after the containers
// they depend on, through volumes-from, network_mode, Compose depends_on, and
// Watchtower depends-on links, with Watchtower containers last, and that a
// circular dependency is an error.
func Test_dependencySorter(t *testing.T) {
	t.Parallel()

	t.Run("Sort", func(t *testing.T) {
		t.Parallel()

		t.Run("should sort a volumes-from dependent after a source referenced by Docker ID", func(t *testing.T) {
			t.Parallel()

			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			app := concreteLinkedContainer(appID, "/nextcloud-app", "nextcloud", "app", "")
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app}
			err := dependencySorter{}.Sort(newTestLogger(t), containers, false)
			require.NoError(t, err)
			assert.Equal(t, "nextcloud-app", containers[0].Name())
			assert.Equal(t, "nextcloud-nginx", containers[1].Name())
		})

		t.Run("should sort a volumes-from dependent after a source in another Compose project", func(t *testing.T) {
			t.Parallel()

			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			app := concreteLinkedContainer(appID, "/shared-app", "data", "app", "")
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app}
			err := dependencySorter{}.Sort(newTestLogger(t), containers, false)
			require.NoError(t, err)
			assert.Equal(t, "shared-app", containers[0].Name())
			assert.Equal(t, "nextcloud-nginx", containers[1].Name())
		})

		t.Run("should sort a volumes-from chain after compose depends_on", func(t *testing.T) {
			t.Parallel()

			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			postgres := concreteLinkedContainer(
				"postgres-id",
				"/nextcloud-db",
				"nextcloud",
				"db",
				"",
			)
			app := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:         appID,
				Name:       "/nextcloud-app",
				HostConfig: &dockerContainer.HostConfig{},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project":    "nextcloud",
					"com.docker.compose.service":    "app",
					"com.docker.compose.depends_on": "db",
				}},
			}, nil)
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app, postgres}
			err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			assert.Equal(t, "nextcloud-db", containers[0].Name())
			assert.Equal(t, "nextcloud-app", containers[1].Name())
			assert.Equal(t, "nextcloud-nginx", containers[2].Name())
		})

		t.Run("should sort a volumes-from chain after watchtower depends-on", func(t *testing.T) {
			t.Parallel()

			const appID = "9b738d6a78250d731816cc5020d8cfaf55e605cd510f4d4662688bd9a6c4e0c7"

			postgres := concreteLinkedContainer(
				"postgres-id",
				"/nextcloud-db",
				"nextcloud",
				"db",
				"",
			)
			app := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:         appID,
				Name:       "/nextcloud-app",
				HostConfig: &dockerContainer.HostConfig{},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project":                "nextcloud",
					"com.docker.compose.service":                "app",
					"com.centurylinklabs.watchtower.depends-on": "nextcloud-db",
				}},
			}, nil)
			nginx := container.NewContainer(nil, &dockerContainer.InspectResponse{
				ID:   "nginx-id",
				Name: "/nextcloud-nginx",
				HostConfig: &dockerContainer.HostConfig{
					VolumesFrom: []string{appID + ":rw"},
				},
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.project": "nextcloud",
					"com.docker.compose.service": "web",
				}},
			}, nil)

			containers := []types.Container{nginx, app, postgres}
			err := dependencySorter{}.Sort(newTestLogger(t), containers, false)
			require.NoError(t, err)
			assert.Equal(t, "nextcloud-db", containers[0].Name())
			assert.Equal(t, "nextcloud-app", containers[1].Name())
			assert.Equal(t, "nextcloud-nginx", containers[2].Name())
		})

		t.Run("should sort a network_mode dependent after a provider referenced by Docker ID", func(t *testing.T) {
			t.Parallel()

			const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

			gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
			qbittorrent := concreteLinkedContainer(
				"qbittorrent-id",
				"/qbittorrent",
				"qbittorrent",
				"app",
				"container:"+providerID,
			)

			containers := []types.Container{qbittorrent, gluetun}
			err := dependencySorter{}.Sort(newTestLogger(t), containers, false)
			require.NoError(t, err)
			assert.Equal(t, "gluetun", containers[0].Name())
			assert.Equal(t, "qbittorrent", containers[1].Name())
		})

		t.Run("should resolve container ID network_mode to the ID owner when a peer reuses that ID as its name", func(t *testing.T) {
			t.Parallel()

			const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

			gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
			decoy := concreteLinkedContainer("decoy-id", "/"+providerID, "other", "svc", "")
			qbittorrent := concreteLinkedContainer(
				"qbittorrent-id",
				"/qbittorrent",
				"qbittorrent",
				"app",
				"container:"+providerID,
			)

			_, indegree, adjacency, _, err := buildDependencyGraph(
				newTestLogger(t),
				[]types.Container{qbittorrent, decoy, gluetun},
				false,
			)
			require.NoError(t, err)
			assert.Equal(t, 1, indegree["qbittorrent-app"])
			assert.Contains(t, adjacency["gluetun-vpn"], "qbittorrent-app")
			assert.NotContains(t, adjacency["other-svc"], "qbittorrent-app")
		})

		t.Run("should sort containers with no dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c3 := mockTypes.NewMockContainer(t)
			c3.EXPECT().Name().Return("c3")
			c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().IsWatchtower().Return(false)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c3", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2, c3}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containers, 3)
			// Order may vary since no dependencies
		})

		t.Run("should sort containers with simple dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containers, 2)
			assert.Equal(t, "c2", containers[0].Name())
			assert.Equal(t, "c1", containers[1].Name())
		})

		t.Run("should sort containers with complex dependency chains", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2", "c3"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return([]string{"c3"})
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c3 := mockTypes.NewMockContainer(t)
			c3.EXPECT().Name().Return("c3")
			c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().IsWatchtower().Return(false)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c3", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2, c3}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containers, 3)
			assert.Equal(t, "c3", containers[0].Name())
			assert.Equal(t, "c2", containers[1].Name())
			assert.Equal(t, "c1", containers[2].Name())
		})

		t.Run("should detect circular dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "circular reference detected")
			assert.Contains(t, err.Error(), "c1 -> c2 -> c1")
		})

		t.Run("should ignore Compose depends_on but honor Watchtower labels when useComposeDependsOn is false", func(t *testing.T) {
			t.Parallel()

			// c1 has a Watchtower explicit depends-on label referencing c2.
			// Links(false) still returns c2 because Watchtower labels are checked
			// before the Compose depends_on label in the Links() method.
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(false).Return([]string{"c2"}) // Watchtower label honored
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// c2 has no dependencies.
			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(false).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, false)
			require.NoError(t, err)
			require.Len(t, containers, 2)
			// c1 depends on c2, so c2 must come before c1
			assert.Equal(t, "c2", containers[0].Name())
			assert.Equal(t, "c1", containers[1].Name())
		})

		t.Run("should place Watchtower containers last", func(t *testing.T) {
			t.Parallel()

			watchtower := mockTypes.NewMockContainer(t)
			watchtower.EXPECT().Name().Return("watchtower")
			watchtower.EXPECT().IsWatchtower().Return(true)

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().IsWatchtower().Return(false)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{watchtower, c1, c2}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containers, 3)
			assert.Equal(t, "c2", containers[0].Name())
			assert.Equal(t, "c1", containers[1].Name())
			assert.Equal(t, "watchtower", containers[2].Name())
		})

		t.Run("should handle empty container list", func(t *testing.T) {
			t.Parallel()

			containers := []types.Container{}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			assert.Empty(t, containers)
		})

		t.Run("should handle single container", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().IsWatchtower().Return(false)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1}
			ds := dependencySorter{}
			err := ds.Sort(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containers, 1)
			assert.Equal(t, "c1", containers[0].Name())
		})
	})

	t.Run("sortByDependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("should sort containers with no dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
		})

		t.Run("should sort containers with simple dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, "c2", result[0].Name())
			assert.Equal(t, "c1", result[1].Name())
		})

		t.Run("should sort containers with complex chains", func(t *testing.T) {
			t.Parallel()

			a := mockTypes.NewMockContainer(t)
			a.EXPECT().Name().Return("a")
			a.EXPECT().ID().Return(types.ContainerID("id-a"))
			a.EXPECT().Links(true).Return([]string{"b"})
			a.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/a", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			b := mockTypes.NewMockContainer(t)
			b.EXPECT().Name().Return("b")
			b.EXPECT().ID().Return(types.ContainerID("id-b"))
			b.EXPECT().Links(true).Return([]string{"c"})
			b.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/b", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c := mockTypes.NewMockContainer(t)
			c.EXPECT().Name().Return("c")
			c.EXPECT().ID().Return(types.ContainerID("id-c"))
			c.EXPECT().Links(true).Return(nil)
			c.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{a, b, c}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 3)
			assert.Equal(t, "c", result[0].Name())
			assert.Equal(t, "b", result[1].Name())
			assert.Equal(t, "a", result[2].Name())
		})

		t.Run("should detect circular dependencies", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c2", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1, c2}
			_, err := sortByDependencies(newTestLogger(t), containers, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "circular reference detected")
			assert.Contains(t, err.Error(), "c1 -> c2 -> c1")
		})

		t.Run("should handle empty list", func(t *testing.T) {
			t.Parallel()

			containers := []types.Container{}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			assert.Empty(t, result)
		})

		t.Run("should handle single container", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})
			containers := []types.Container{c1}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 1)
			assert.Equal(t, "c1", result[0].Name())
		})

		t.Run("should handle disconnected components", func(t *testing.T) {
			t.Parallel()

			// Component 1: A -> B
			a := mockTypes.NewMockContainer(t)
			a.EXPECT().Name().Return("a")
			a.EXPECT().ID().Return(types.ContainerID("id-a"))
			a.EXPECT().Links(true).Return([]string{"b"})
			a.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/a", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			b := mockTypes.NewMockContainer(t)
			b.EXPECT().Name().Return("b")
			b.EXPECT().ID().Return(types.ContainerID("id-b"))
			b.EXPECT().Links(true).Return(nil)
			b.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/b", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// Component 2: C -> D
			c := mockTypes.NewMockContainer(t)
			c.EXPECT().Name().Return("c")
			c.EXPECT().ID().Return(types.ContainerID("id-c"))
			c.EXPECT().Links(true).Return([]string{"d"})
			c.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			d := mockTypes.NewMockContainer(t)
			d.EXPECT().Name().Return("d")
			d.EXPECT().ID().Return(types.ContainerID("id-d"))
			d.EXPECT().Links(true).Return(nil)
			d.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/d", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{a, b, c, d}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 4)

			resultNames := make([]string, len(result))
			for i, c := range result {
				resultNames[i] = c.Name()
			}

			assertOrderBefore(t, resultNames, "b", "a")
			assertOrderBefore(t, resultNames, "d", "c")
		})

		t.Run("should skip self-referencing containers without creating cycles", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("c1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c1"}) // Self-reference
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/c1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			// Self-reference is filtered, not a cycle
			require.Len(t, result, 1)
			assert.Equal(t, "c1", result[0].Name())
		})

		t.Run("should skip self-reference via prefix matching for replica-named containers", func(t *testing.T) {
			t.Parallel()

			// Container named "myapp-1" linking to "myapp" should NOT create a self-dependency
			// The prefix matching would match "myapp-1" when looking for "myapp" replicas,
			// but the self-reference guard should skip it
			myapp := mockTypes.NewMockContainer(t)
			myapp.EXPECT().Name().Return("myapp-1").Maybe()
			myapp.EXPECT().ID().Return(types.ContainerID("id-myapp")).Maybe()
			myapp.EXPECT().Links(true).Return([]string{"myapp"})
			myapp.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/myapp-1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{myapp}
			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containerMap, 1)

			// Self-reference via prefix matching should be skipped
			// indegree["myapp-1"] should be 0 (not incremented)
			assert.Equal(t, 0, indegree["myapp-1"])
			// adjacency["myapp-1"] should NOT contain "myapp-1"
			assert.NotContains(t, adjacency["myapp-1"], "myapp-1")
		})

		t.Run("should skip self-reference via prefix matching for db replica-named containers", func(t *testing.T) {
			t.Parallel()

			// Container named "db-1" linking to "db" should NOT create a self-dependency
			// Similar to the myapp case, this tests another common naming pattern
			db := mockTypes.NewMockContainer(t)
			db.EXPECT().Name().Return("db-1").Maybe()
			db.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
			db.EXPECT().Links(true).Return([]string{"db"})
			db.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/db-1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{db}
			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containerMap, 1)

			// Self-reference via prefix matching should be skipped
			// indegree["db-1"] should be 0 (not incremented)
			assert.Equal(t, 0, indegree["db-1"])
			// adjacency["db-1"] should NOT contain "db-1"
			assert.NotContains(t, adjacency["db-1"], "db-1")
		})

		t.Run("should handle containers with project-prefixed names and service links", func(t *testing.T) {
			t.Parallel()

			// test-db-1 (no dependencies)
			db := mockTypes.NewMockContainer(t)
			db.EXPECT().Name().Return("test-db-1")
			db.EXPECT().ID().Return(types.ContainerID("id-db"))
			db.EXPECT().Links(true).Return(nil)
			db.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/test-db-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// app-1 depends on "db"
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().Name().Return("app-1")
			app.EXPECT().ID().Return(types.ContainerID("id-app"))
			app.EXPECT().Links(true).Return([]string{"db"})
			app.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/app-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// test-web-1 depends on "app"
			web := mockTypes.NewMockContainer(t)
			web.EXPECT().Name().Return("test-web-1")
			web.EXPECT().ID().Return(types.ContainerID("id-web"))
			web.EXPECT().Links(true).Return([]string{"app"})
			web.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/test-web-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{web, app, db} // Unsorted order
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 3)
			// Dependencies first: db, then app (depends on db), then web (depends on app)
			assert.Equal(t, "test-db-1", result[0].Name())
			assert.Equal(t, "app-1", result[1].Name())
			assert.Equal(t, "test-web-1", result[2].Name())
		})

		t.Run("should handle containers with service names", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return([]string{"web"}) // Link to service name
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("container2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web"}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, "container2", result[0].Name()) // web service
			assert.Equal(t, "container1", result[1].Name()) // depends on web
		})

		t.Run("should handle containers with replicas without collision", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("myproject-web-1")
			c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/myproject-web-1", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web", "com.docker.compose.project": "myproject"}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("myproject-web-2")
			c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/myproject-web-2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "web", "com.docker.compose.project": "myproject"}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			// Order may vary since no dependencies
		})

		t.Run("should handle prefix matching linking service name to multiple replicas deterministically", func(t *testing.T) {
			t.Parallel()

			// App container that depends on "db" service
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})
			app.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/app", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			// Multiple db replicas
			db1 := mockTypes.NewMockContainer(t)
			db1.EXPECT().Name().Return("db-1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)
			db1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/db-1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			db2 := mockTypes.NewMockContainer(t)
			db2.EXPECT().Name().Return("db-2").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)
			db2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/db-2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			db3 := mockTypes.NewMockContainer(t)
			db3.EXPECT().Name().Return("db-3").Maybe()
			db3.EXPECT().ID().Return(types.ContainerID("id-db3")).Maybe()
			db3.EXPECT().Links(true).Return(nil)
			db3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/db-3", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{app, db1, db2, db3}
			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containerMap, 4)

			// Verify app depends on all db replicas
			assert.Equal(t, 3, indegree["app"])
			assert.Contains(t, adjacency["db-1"], "app")
			assert.Contains(t, adjacency["db-2"], "app")
			assert.Contains(t, adjacency["db-3"], "app")

			// Verify db replicas have no dependencies
			assert.Equal(t, 0, indegree["db-1"])
			assert.Equal(t, 0, indegree["db-2"])
			assert.Equal(t, 0, indegree["db-3"])
		})

		t.Run("should handle containers with service names containing leading slashes", func(t *testing.T) {
			t.Parallel()

			// Test that containers with service names containing leading slashes are handled correctly
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"web-service"}) // Link to normalized service name
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("container2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{"com.docker.compose.service": "/web-service"}}})

				// Malformed service name with leading slash

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, "container2", result[0].Name())
			// web-service container first
			assert.Equal(t, "container1", result[1].Name()) // depends on web-service
		})

		t.Run("should handle containers with container names containing leading slashes", func(t *testing.T) {
			t.Parallel()

			// Test containers with container names containing leading slashes
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"/web"}) // Link with leading slash
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("web").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/web", Config: &dockerContainer.Config{Labels: map[string]string{}}})

				// Container name with leading slash

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, "web", result[0].Name())
			// web container first
			assert.Equal(t, "container1", result[1].Name()) // depends on web
		})

		t.Run("should handle containers with empty names falling back to IDs", func(t *testing.T) {
			t.Parallel()

			// Test containers with empty names that fall back to container IDs
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("").Maybe() // Empty name
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"id-c2"}) // Link to other container's ID
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("").Maybe() // Empty name
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, types.ContainerID("id-c2"), result[0].ID())
			// dependency first
			assert.Equal(t, types.ContainerID("id-c1"), result[1].ID())
			// dependent second
		})

		t.Run("should handle containers with malformed link targets", func(t *testing.T) {
			t.Parallel()

			// Test containers with links to non-existent containers
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().
				Links(true).
				Return([]string{"non-existent", "/also-non-existent"})
				// Links to non-existent containers
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("container2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container2", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			// Order may vary since no valid dependencies
		})

		t.Run("should handle containers with network mode dependencies", func(t *testing.T) {
			t.Parallel()

			// Test containers with network mode dependencies
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"web"}) // Mock includes network dependency
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/container1",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("web").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/web",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 2)
			assert.Equal(t, "web", result[0].Name())        // web container first
			assert.Equal(t, "container1", result[1].Name()) // depends on web
		})

		t.Run("should handle containers with mixed dependency sources", func(t *testing.T) {
			t.Parallel()

			// Test containers with dependencies from labels, links, and network mode
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("app").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			c1.EXPECT().
				Links(true).
				Return([]string{"db", "cache", "redis", "proxy"})
				// Mock all dependencies
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/app",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("db").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/db",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c3 := mockTypes.NewMockContainer(t)
			c3.EXPECT().Name().Return("cache").Maybe()
			c3.EXPECT().ID().Return(types.ContainerID("id-cache")).Maybe()
			c3.EXPECT().Links(true).Return(nil)
			c3.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/cache",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c4 := mockTypes.NewMockContainer(t)
			c4.EXPECT().Name().Return("redis").Maybe()
			c4.EXPECT().ID().Return(types.ContainerID("id-redis")).Maybe()
			c4.EXPECT().Links(true).Return(nil)
			c4.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/redis",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			c5 := mockTypes.NewMockContainer(t)
			c5.EXPECT().Name().Return("proxy").Maybe()
			c5.EXPECT().ID().Return(types.ContainerID("id-proxy")).Maybe()
			c5.EXPECT().Links(true).Return(nil)
			c5.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{
					Name:   "/proxy",
					Config: &dockerContainer.Config{Labels: map[string]string{}},
				})

			containers := []types.Container{c1, c2, c3, c4, c5}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 5)
			// Dependencies should come before dependents
			resultNames := make([]string, len(result))
			for i, c := range result {
				resultNames[i] = c.Name()
			}
			// app should be last
			assert.Equal(t, "app", resultNames[len(resultNames)-1])
		})

		t.Run("should skip containers with self-referencing dependencies without creating cycles", func(t *testing.T) {
			t.Parallel()

			// Test containers that reference themselves (self-reference is filtered as safety net)
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"container1"}) // Self-reference
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "/container1", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			// Self-reference is filtered, not a cycle
			require.Len(t, result, 1)
			assert.Equal(t, "container1", result[0].Name())
		})

		t.Run("should handle large number of containers with no dependencies", func(t *testing.T) {
			t.Parallel()

			// Test performance and correctness with many containers
			containers := make([]types.Container, 100)

			for i := range 100 {
				c := mockTypes.NewMockContainer(t)
				c.EXPECT().Name().Return(fmt.Sprintf("container%d", i)).Maybe()
				c.EXPECT().ID().Return(types.ContainerID(fmt.Sprintf("id-c%d", i))).Maybe()
				c.EXPECT().Links(true).Return(nil)
				c.EXPECT().
					ContainerInfo().
					Return(&dockerContainer.InspectResponse{Name: fmt.Sprintf("/container%d", i), Config: &dockerContainer.Config{Labels: map[string]string{}}})
				containers[i] = c
			}

			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, result, 100)
		})

		t.Run("should handle containers with empty identifiers", func(t *testing.T) {
			t.Parallel()

			// This tests that containers with empty names fall back to using container ID as identifier
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("") // Empty name
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return(nil)
			c1.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("") // Empty name
			c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)
			c2.EXPECT().
				ContainerInfo().
				Return(&dockerContainer.InspectResponse{Name: "", Config: &dockerContainer.Config{Labels: map[string]string{}}})

			containers := []types.Container{c1, c2}
			result, err := sortByDependencies(newTestLogger(t), containers, true)
			require.NoError(t, err) // Should not detect false cycle
			require.Len(t, result, 2)
		})

		t.Run("should resolve a service name shared by two projects to both", func(t *testing.T) {
			t.Parallel()

			// app depending on "db"
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "db"},
				},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// db1 from project1
			db1 := mockTypes.NewMockContainer(t)
			db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_db_1",
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.service": "db",
					"com.docker.compose.project": "project1",
				}},
			})
			db1.EXPECT().Name().Return("project1_db_1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)

			// db2 from project2
			db2 := mockTypes.NewMockContainer(t)
			db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project2_db_1",
				Config: &dockerContainer.Config{Labels: map[string]string{
					"com.docker.compose.service": "db",
					"com.docker.compose.project": "project2",
				}},
			})
			db2.EXPECT().Name().Return("project2_db_1").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db1, db2}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// "db" names the db service of both projects, so app depends on each.
			require.Len(t, containerMap, 3)
			assert.Equal(t, 2, indegree["app"])
			assert.Equal(t, map[string][]string{
				"project1-db": {"app"},
				"project2-db": {"app"},
			}, adjacency)
		})

		t.Run("should detect circular dependencies via watchtower labels", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/c1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "c2"},
				},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
			c1.EXPECT().Links(true).Return([]string{"c2"})

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/c2",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "c1"},
				},
			})
			c2.EXPECT().Name().Return("c2")
			c2.EXPECT().Links(true).Return([]string{"c1"})

			containers := []types.Container{c1, c2}

			_, err := sortByDependencies(newTestLogger(t), containers, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "circular reference detected")
			assert.Contains(t, err.Error(), "c1 -> c2 -> c1")
		})

		t.Run("should handle dependencies to filtered containers", func(t *testing.T) {
			t.Parallel()

			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app",
				Config: &dockerContainer.Config{
					Labels: map[string]string{"com.centurylinklabs.watchtower.depends-on": "db"},
				},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// Only app in containers list, db not included
			containers := []types.Container{app}

			containerMap, indegree, _, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)
			require.Len(t, containerMap, 1)
			assert.Equal(t, 0, indegree["app"])
		})
	})
}

// TestResolveContainerIdentifier verifies that a container's identifier is its
// Compose service name when it has one, and its container name otherwise.
func TestResolveContainerIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("should return service name when present", func(t *testing.T) {
		t.Parallel()

		mockContainer := mockTypes.NewMockContainer(t)
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{"com.docker.compose.service": "web"},
			},
		})
		result := container.ResolveContainerIdentifier(mockContainer)
		assert.Equal(t, "web", result)
	})

	t.Run("should return container name when no service label", func(t *testing.T) {
		t.Parallel()

		mockContainer := mockTypes.NewMockContainer(t)
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{},
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		assert.Equal(t, "container1", result)
	})

	t.Run("should return container name when labels are nil", func(t *testing.T) {
		t.Parallel()

		mockContainer := mockTypes.NewMockContainer(t)
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: nil,
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		assert.Equal(t, "container1", result)
	})

	t.Run("should return container name when service label is empty", func(t *testing.T) {
		t.Parallel()

		mockContainer := mockTypes.NewMockContainer(t)
		mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Config: &dockerContainer.Config{
				Labels: map[string]string{"com.docker.compose.service": ""},
			},
		})
		mockContainer.EXPECT().Name().Return("container1")
		result := container.ResolveContainerIdentifier(mockContainer)
		assert.Equal(t, "container1", result)
	})
}

// TestPrefixMatchingIssues verifies that a link matches Compose replicas by their
// numeric suffix, and not containers whose names merely share a prefix.
func TestPrefixMatchingIssues(t *testing.T) {
	t.Parallel()

	t.Run("should not match containers with similar names that are not Docker Compose replicas (issue-1161)", func(t *testing.T) {
		t.Parallel()

		// App container that depends on "watchtower-test-database"
		app := mockTypes.NewMockContainer(t)
		app.EXPECT().Name().Return("watchtower-test-app1").Maybe()
		app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
		app.EXPECT().Links(true).Return([]string{"watchtower-test-database"})
		app.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/watchtower-test-app1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		// First database container (exact match)
		db1 := mockTypes.NewMockContainer(t)
		db1.EXPECT().Name().Return("watchtower-test-database").Maybe()
		db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
		db1.EXPECT().Links(true).Return(nil)
		db1.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/watchtower-test-database",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		// Second database container with similar name (should NOT be matched)
		db2 := mockTypes.NewMockContainer(t)
		db2.EXPECT().Name().Return("watchtower-test-database2").Maybe()
		db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
		db2.EXPECT().Links(true).Return(nil)
		db2.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/watchtower-test-database2",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		containers := []types.Container{app, db1, db2}
		containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
		require.NoError(t, err)
		require.Len(t, containerMap, 3)

		// Verify app depends ONLY on watchtower-test-database, not on watchtower-test-database2
		assert.Equal(t, 1, indegree["watchtower-test-app1"])
		assert.Contains(t, adjacency["watchtower-test-database"], "watchtower-test-app1")
		assert.Empty(t, adjacency["watchtower-test-database2"])

		// Verify db1 and db2 have no dependencies
		assert.Equal(t, 0, indegree["watchtower-test-database"])
		assert.Equal(t, 0, indegree["watchtower-test-database2"])
	})

	t.Run("should match Docker Compose-style numeric replica suffixes", func(t *testing.T) {
		t.Parallel()

		// App container that depends on "myapp-db"
		app := mockTypes.NewMockContainer(t)
		app.EXPECT().Name().Return("myapp-app1").Maybe()
		app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
		app.EXPECT().Links(true).Return([]string{"myapp-db"})
		app.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-app1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		// Database replicas with numeric suffixes (should be matched)
		db1 := mockTypes.NewMockContainer(t)
		db1.EXPECT().Name().Return("myapp-db-1").Maybe()
		db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
		db1.EXPECT().Links(true).Return(nil)
		db1.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-db-1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		db2 := mockTypes.NewMockContainer(t)
		db2.EXPECT().Name().Return("myapp-db-2").Maybe()
		db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
		db2.EXPECT().Links(true).Return(nil)
		db2.EXPECT().
			ContainerInfo().
			Return(&dockerContainer.InspectResponse{
				Name:   "/myapp-db-2",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})

		containers := []types.Container{app, db1, db2}
		containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
		require.NoError(t, err)
		require.Len(t, containerMap, 3)

		// Verify app depends on BOTH db replicas (since they have numeric suffixes)
		assert.Equal(t, 2, indegree["myapp-app1"])
		assert.Contains(t, adjacency["myapp-db-1"], "myapp-app1")
		assert.Contains(t, adjacency["myapp-db-2"], "myapp-app1")
	})
}

// TestIdentifierCollisionIssues verifies that containers resolving to the same
// identifier are reported as an identifier collision when the graph is built
// and when containers are sorted, and that distinct identifiers sort normally.
func TestIdentifierCollisionIssues(t *testing.T) {
	t.Parallel()

	t.Run("ResolveContainerIdentifier", func(t *testing.T) {
		t.Parallel()

		t.Run("should return different identifiers for containers from different projects with same service name", func(t *testing.T) {
			t.Parallel()

			// Container from project "app1" with service "web"
			mockContainer1 := mockTypes.NewMockContainer(t)
			mockContainer1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service":     "web",
						"com.docker.compose.project":     "app1",
						"com.docker.compose.version":     "3.8",
						"com.docker.compose.config-hash": "abc123",
					},
				},
			})
			mockContainer1.EXPECT().Name().Return("app1_web_1")

			// Container from project "app2" with same service "web"
			mockContainer2 := mockTypes.NewMockContainer(t)
			mockContainer2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service":     "web",
						"com.docker.compose.project":     "app2",
						"com.docker.compose.version":     "3.8",
						"com.docker.compose.config-hash": "def456",
					},
				},
			})
			mockContainer2.EXPECT().Name().Return("app2_web_1")

			result1 := container.ResolveContainerIdentifier(mockContainer1)
			result2 := container.ResolveContainerIdentifier(mockContainer2)

			// Containers from different projects should return different identifiers
			assert.Equal(t, "app1-web", result1)
			assert.Equal(t, "app2-web", result2)
			assert.NotEqual(t, result2, result1)
		})
	})

	t.Run("buildDependencyGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("should treat containers from different projects as separate entities in dependency graph", func(t *testing.T) {
			t.Parallel()

			// Create two containers from different projects with same service name
			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app1_web_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "web",
						"com.docker.compose.project": "app1",
					},
				},
			})
			c1.EXPECT().Name().Return("app1_web_1").Maybe()
			c1.EXPECT().Links(true).Return(nil)

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/app2_web_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "web",
						"com.docker.compose.project": "app2",
					},
				},
			})
			c2.EXPECT().Name().Return("app2_web_1").Maybe()
			c2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{c1, c2}

			containerMap, indegree, _, normalizedMap, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Containers from different projects should have different identifiers
			ident1 := container.ResolveContainerIdentifier(c1)
			ident2 := container.ResolveContainerIdentifier(c2)

			assert.Equal(t, "app1-web", ident1)
			assert.Equal(t, "app2-web", ident2)

			// Both containers should be in the map as separate entities
			require.Len(t, containerMap, 2)
			assert.Equal(t, c1, containerMap["app1-web"])
			assert.Equal(t, c2, containerMap["app2-web"])

			// Verify indegree reflects separate entities
			require.Len(t, indegree, 2)
			assert.Equal(t, 0, indegree["app1-web"])
			assert.Equal(t, 0, indegree["app2-web"])

			// The normalizedMap should show containers map to their respective identifiers
			require.Len(t, normalizedMap, 2)
			assert.Equal(t, "app1-web", normalizedMap[c1])
			assert.Equal(t, "app2-web", normalizedMap[c2])
		})

		t.Run("should maintain backward compatibility for containers without project labels", func(t *testing.T) {
			t.Parallel()

			// Container with service label but no project label
			mockContainer := mockTypes.NewMockContainer(t)
			mockContainer.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "web",
						// No project label
					},
				},
			})

			result := container.ResolveContainerIdentifier(mockContainer)

			// Should return just the service name, same as before
			assert.Equal(t, "web", result)
		})

		t.Run("should support exact matching for cross-project dependencies", func(t *testing.T) {
			t.Parallel()

			// Container that links to exact container name
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/app",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"project1-db"})

			// DB container from project1
			db1 := mockTypes.NewMockContainer(t)
			db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project1",
					},
				},
			})
			db1.EXPECT().Name().Return("project1_db_1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db1}

			containerMap, indegree, adjacency, normalizedMap, err := buildDependencyGraph(newTestLogger(t), containers,
				true,
			)
			require.NoError(t, err)

			// Verify identifiers
			assert.Equal(t, "app", container.ResolveContainerIdentifier(app))
			assert.Equal(t, "project1-db", container.ResolveContainerIdentifier(db1))

			// All containers should be in the map
			require.Len(t, containerMap, 2)

			// app should have indegree 1 (depends on db1)
			assert.Equal(t, 1, indegree["app"])

			// db1 should have app as dependent
			assert.Contains(t, adjacency["project1-db"], "app")

			// Verify normalizedMap
			require.Len(t, normalizedMap, 2)
		})
	})

	t.Run("identifierCollisionError", func(t *testing.T) {
		t.Parallel()

		t.Run("should format error message correctly", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().Name().Return("container1")
			c1.EXPECT().ID().Return(types.ContainerID("id1"))

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().Name().Return("container2")
			c2.EXPECT().ID().Return(types.ContainerID("id2"))

			containers := []types.Container{c1, c2}

			err := identifierCollisionError{
				DuplicateIdentifier: "test-id",
				AffectedContainers:  containers,
			}

			expected := "identifier collision detected: 'test-id' used by containers: container1 (id1), container2 (id2)"
			assert.Equal(t, expected, err.Error())
		})
	})

	t.Run("buildDependencyGraph with collisions", func(t *testing.T) {
		t.Parallel()

		t.Run("should return identifierCollisionError when containers have same normalized identifier", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1", // Same name
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c2.EXPECT().Name().Return("c1").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()

			containers := []types.Container{c1, c2}

			_, _, _, _, err := buildDependencyGraph(newTestLogger(t), containers, true)

			var collisionErr identifierCollisionError
			require.ErrorAs(t, err, &collisionErr)
			assert.Equal(t, "c1", collisionErr.DuplicateIdentifier)
			assert.Len(t, collisionErr.AffectedContainers, 2)
		})

		t.Run("should succeed when containers have different identifiers", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()
			c1.EXPECT().Links(true).Return(nil)

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c2",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c2.EXPECT().Name().Return("c2").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()
			c2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{c1, c2}

			containerMap, indegree, adjacency, normalizedMap, err := buildDependencyGraph(newTestLogger(t), containers,
				true,
			)

			require.NoError(t, err)
			require.Len(t, containerMap, 2)
			require.Len(t, indegree, 2)
			assert.Empty(t, adjacency) // No links set up
			require.Len(t, normalizedMap, 2)
		})
	})

	t.Run("sortByDependencies with collisions", func(t *testing.T) {
		t.Parallel()

		t.Run("should propagate identifierCollisionError from buildDependencyGraph", func(t *testing.T) {
			t.Parallel()

			c1 := mockTypes.NewMockContainer(t)
			c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c1.EXPECT().Name().Return("c1").Maybe()
			c1.EXPECT().ID().Return(types.ContainerID("id1")).Maybe()

			c2 := mockTypes.NewMockContainer(t)
			c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/c1", // Same name
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			c2.EXPECT().Name().Return("c1").Maybe()
			c2.EXPECT().ID().Return(types.ContainerID("id2")).Maybe()

			containers := []types.Container{c1, c2}

			_, err := sortByDependencies(newTestLogger(t), containers, true)

			var collisionErr identifierCollisionError
			require.ErrorAs(t, err, &collisionErr)
		})
	})
}

// Test_isPositiveInteger verifies which replica suffixes count as positive
// integers.
func Test_isPositiveInteger(t *testing.T) {
	t.Parallel()

	t.Run("validates positive integers correctly", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{name: "should return true for single digit positive integer", input: "1", expected: true},
			{name: "should return true for multi-digit positive integer", input: "123", expected: true},
			{name: "should return true for large positive integer", input: "999", expected: true},
			{name: "should return true for zero-prefixed positive integer", input: "01", expected: true},
			{name: "should return false for zero", input: "0", expected: false},
			{name: "should return false for negative number", input: "-1", expected: false},
			{name: "should return false for alphabetic string", input: "abc", expected: false},
			{name: "should return false for alphanumeric string", input: "1a", expected: false},
			{name: "should return false for empty string", input: "", expected: false},
			{name: "should return false for decimal number", input: "1.5", expected: false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				input, expected := tt.input, tt.expected

				result := isPositiveInteger(input)
				assert.Equal(t, expected, result)
			})
		}
	})
}

// Test_extractServiceName verifies the service name taken from a container
// identifier.
func Test_extractServiceName(t *testing.T) {
	t.Parallel()

	t.Run("extracts service name from container identifier", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{name: "should return simple identifier without project prefix as-is", input: "postgres", expected: "postgres"},
			{name: "should extract service name from project-service identifier", input: "postgresql-postgres", expected: "postgres"},
			{name: "should extract service name from project-service-replica identifier", input: "project-service-1", expected: "service"},
			{name: "should extract service name from complex project-service-replica identifier", input: "myapp-database-2", expected: "database"},
			{name: "should return empty string for empty input", input: "", expected: ""},
			{name: "should handle single character identifier", input: "a", expected: "a"},
			{name: "should handle identifier with multiple dashes", input: "my-complex-app-name-service-3", expected: "service"},
			{name: "should handle two-part identifier without replica", input: "myapp-web", expected: "web"},
			{name: "should handle two-part identifier with replica", input: "web-1", expected: "web"},
			{name: "should return last part when no replica suffix", input: "project-web", expected: "web"},
			{name: "should handle replica number 0 (not positive, so treat as name)", input: "service-0", expected: "0"},
			{name: "should handle large replica number", input: "project-service-999", expected: "service"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				input, expected := tt.input, tt.expected

				result := extractServiceName(input)
				assert.Equal(t, expected, result)
			})
		}
	})
}

// TestServiceOnlyMatching verifies that a bare service name links to the
// replicas of exactly one Compose service, and to nothing when several services
// share the name.
func TestServiceOnlyMatching(t *testing.T) {
	t.Parallel()

	t.Run("buildDependencyGraph", func(t *testing.T) {
		t.Parallel()

		t.Run("should match single unambiguous cross-project service by service name", func(t *testing.T) {
			t.Parallel()

			// App container from project1 that depends on "db" service
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_app_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "app",
						"com.docker.compose.project": "project1",
					},
				},
			})
			app.EXPECT().Name().Return("project1_app_1").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"}) // Link to just "db" service name

			// DB container from project2 (different project, but only one "db" service)
			db := mockTypes.NewMockContainer(t)
			db.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project2_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project2",
					},
				},
			})
			db.EXPECT().Name().Return("project2_db_1").Maybe()
			db.EXPECT().ID().Return(types.ContainerID("id-db")).Maybe()
			db.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Verify containers are in the map
			require.Len(t, containerMap, 2)

			// App should have indegree 1 (depends on db via service-only match)
			assert.Equal(t, 1, indegree["project1-app"])

			// DB should have app as dependent
			assert.Contains(t, adjacency["project2-db"], "project1-app")
		})

		t.Run("should match watchtower label referencing service name without project prefix", func(t *testing.T) {
			t.Parallel()

			// App container with watchtower depends-on label referencing just "postgres"
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/myapp",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.centurylinklabs.watchtower.depends-on": "postgres",
					},
				},
			})
			app.EXPECT().Name().Return("myapp").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"postgres"})

			// Postgres container with project prefix in identifier
			postgres := mockTypes.NewMockContainer(t)
			postgres.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/postgresql_postgres_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "postgres",
						"com.docker.compose.project": "postgresql",
					},
				},
			})
			postgres.EXPECT().Name().Return("postgresql_postgres_1").Maybe()
			postgres.EXPECT().ID().Return(types.ContainerID("id-postgres")).Maybe()
			postgres.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, postgres}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Verify containers are in the map
			require.Len(t, containerMap, 2)

			// App should have indegree 1 (depends on postgres via service-only match)
			assert.Equal(t, 1, indegree["myapp"])

			// Postgres should have app as dependent
			assert.Contains(t, adjacency["postgresql-postgres"], "myapp")
		})

		t.Run("should match every project's service when several share the service name", func(t *testing.T) {
			t.Parallel()

			// App container that depends on "db" service
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/app",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// DB container from project1
			db1 := mockTypes.NewMockContainer(t)
			db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project1",
					},
				},
			})
			db1.EXPECT().Name().Return("project1_db_1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)

			// DB container from project2 (same service name, different project)
			db2 := mockTypes.NewMockContainer(t)
			db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project2_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project2",
					},
				},
			})
			db2.EXPECT().Name().Return("project2_db_1").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db1, db2}

			containerMap, indegree, _, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Verify all containers are in the map
			require.Len(t, containerMap, 3)

			// App depends on the db service of both projects
			assert.Equal(t, 2, indegree["app"])

			// Both db containers should have indegree 0
			assert.Equal(t, 0, indegree["project1-db"])
			assert.Equal(t, 0, indegree["project2-db"])
		})

		t.Run("should prefer exact match over service-only match", func(t *testing.T) {
			t.Parallel()

			// App container that links to exact identifier "project1-db"
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/app",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"project1-db"})

			// DB container from project1
			db1 := mockTypes.NewMockContainer(t)
			db1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project1_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project1",
					},
				},
			})
			db1.EXPECT().Name().Return("project1_db_1").Maybe()
			db1.EXPECT().ID().Return(types.ContainerID("id-db1")).Maybe()
			db1.EXPECT().Links(true).Return(nil)

			// DB container from project2 (should NOT be matched)
			db2 := mockTypes.NewMockContainer(t)
			db2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project2_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project2",
					},
				},
			})
			db2.EXPECT().Name().Return("project2_db_1").Maybe()
			db2.EXPECT().ID().Return(types.ContainerID("id-db2")).Maybe()
			db2.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, db1, db2}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Verify all containers are in the map
			require.Len(t, containerMap, 3)

			// App should have indegree 1 (exact match to project1-db only)
			assert.Equal(t, 1, indegree["app"])

			// Only project1-db should have app as dependent
			assert.Contains(t, adjacency["project1-db"], "app")
			assert.NotContains(t, adjacency["project2-db"], "app")
		})

		t.Run("should prefer replica match over service-only match", func(t *testing.T) {
			t.Parallel()

			// App container that links to "db" (should match db-1 replica, not project1-db)
			app := mockTypes.NewMockContainer(t)
			app.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/app",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			app.EXPECT().Name().Return("app").Maybe()
			app.EXPECT().ID().Return(types.ContainerID("id-app")).Maybe()
			app.EXPECT().Links(true).Return([]string{"db"})

			// DB replica container (db-1 pattern)
			dbReplica := mockTypes.NewMockContainer(t)
			dbReplica.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name:   "/db-1",
				Config: &dockerContainer.Config{Labels: map[string]string{}},
			})
			dbReplica.EXPECT().Name().Return("db-1").Maybe()
			dbReplica.EXPECT().ID().Return(types.ContainerID("id-dbreplica")).Maybe()
			dbReplica.EXPECT().Links(true).Return(nil)

			// DB container with project prefix (service name also "db")
			dbProject := mockTypes.NewMockContainer(t)
			dbProject.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
				Name: "/project_db_1",
				Config: &dockerContainer.Config{
					Labels: map[string]string{
						"com.docker.compose.service": "db",
						"com.docker.compose.project": "project",
					},
				},
			})
			dbProject.EXPECT().Name().Return("project_db_1").Maybe()
			dbProject.EXPECT().ID().Return(types.ContainerID("id-dbproject")).Maybe()
			dbProject.EXPECT().Links(true).Return(nil)

			containers := []types.Container{app, dbReplica, dbProject}

			containerMap, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), containers, true)
			require.NoError(t, err)

			// Verify all containers are in the map
			require.Len(t, containerMap, 3)

			// App should have indegree 1 (replica match to db-1)
			assert.Equal(t, 1, indegree["app"])

			// db-1 should have app as dependent (replica match)
			assert.Contains(t, adjacency["db-1"], "app")
		})
	})
}

// assertOrderBefore checks that first comes before second in names.
//
// Parameters:
//   - t: Test that receives the failure.
//   - names: Container names in sorted order.
//   - first: Name that must come first.
//   - second: Name that must come after first.
func assertOrderBefore(t *testing.T, names []string, first, second string) {
	t.Helper()

	assert.Less(t, indexOf(names, first), indexOf(names, second), "%s must sort before %s", first, second)
}

// indexOf returns the position of target in names.
//
// Parameters:
//   - names: Names to search.
//   - target: Name to find.
//
// Returns:
//   - int: The position of target, or -1 when names lacks it.
func indexOf(names []string, target string) int {
	for i, name := range names {
		if name == target {
			return i
		}
	}

	return -1
}

// mockLinkedContainer builds a mock with Compose labels and fixed Links() output.
//
// Parameters:
//   - t: Test that owns the mock.
//   - name: Container name.
//   - id: Container ID.
//   - project: Compose project label, or empty for none.
//   - service: Compose service label, or empty for none.
//   - links: Links the container reports.
//   - useComposeLinks: Whether Links is called with Compose depends_on enabled.
//
// Returns:
//   - *mockTypes.MockContainer: Mock answering the reads the sorter makes.
func mockLinkedContainer(
	t *testing.T,
	name, id, project, service string,
	links []string,
	useComposeLinks bool,
) *mockTypes.MockContainer {
	t.Helper()

	c := mockTypes.NewMockContainer(t)
	c.EXPECT().Name().Return(name).Maybe()
	c.EXPECT().ID().Return(types.ContainerID(id)).Maybe()
	c.EXPECT().IsWatchtower().Return(false).Maybe()

	if useComposeLinks {
		c.EXPECT().Links(true).Return(links).Maybe()
	} else {
		c.EXPECT().Links(false).Return(links).Maybe()
	}

	labels := map[string]string{}
	if project != "" {
		labels["com.docker.compose.project"] = project
	}

	if service != "" {
		labels["com.docker.compose.service"] = service
	}

	c.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
		Name:   "/" + name,
		Config: &dockerContainer.Config{Labels: labels},
	}).Maybe()

	return c
}

// concreteLinkedContainer builds a *container.Container so Docker ID aliases
// in buildLinkMatchIndexes are exercised.
//
// Parameters:
//   - id: Container ID.
//   - name: Container name, with its leading slash.
//   - project: Compose project label, or empty for none.
//   - service: Compose service label, or empty for none.
//   - networkMode: HostConfig NetworkMode value, such as container:<id>, or empty.
//
// Returns:
//   - types.Container: Container built from the inspect data.
func concreteLinkedContainer(
	id, name, project, service, networkMode string,
) types.Container {
	labels := map[string]string{}
	if project != "" {
		labels["com.docker.compose.project"] = project
	}

	if service != "" {
		labels["com.docker.compose.service"] = service
	}

	hostConfig := &dockerContainer.HostConfig{}
	if networkMode != "" {
		hostConfig.NetworkMode = dockerContainer.NetworkMode(networkMode)
	}

	return container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:         id,
		Name:       name,
		HostConfig: hostConfig,
		Config:     &dockerContainer.Config{Labels: labels},
	}, nil)
}

// Test_matchSet_match verifies which identifiers each link form matches: exact
// keys, Compose replicas, unique service names, and multi-segment services,
// with ambiguous and partial matches left out.
func Test_matchSet_match(t *testing.T) {
	t.Parallel()

	// Exhaustive link x identifier permutations across Compose stacks, multi-segment
	// services, replicas, bare container names, and ambiguous cross-project cases.
	t.Run("matches identifiers for each link form", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			link        string
			identifiers []string
			expected    []string
		}{
			{name: "exact project-service key", link: "project-db", identifiers: []string{"project-db", "other-db"}, expected: []string{"project-db"}},
			{name: "exact bare container name", link: "net-proxy", identifiers: []string{"net-proxy", "web-app"}, expected: []string{"net-proxy"}},
			{
				name:        "replica suffixes from project-service link",
				link:        "project-db",
				identifiers: []string{"project-db-1", "project-db-2", "other"},
				expected:    []string{"project-db-1", "project-db-2"},
			},
			{name: "non-numeric hyphen suffix is not a replica", link: "project-db", identifiers: []string{"project-db-backup"}, expected: nil},
			{name: "numeric-looking middle segment is not a replica of shorter prefix", link: "project", identifiers: []string{"project-db-1"}, expected: nil},
			{name: "unhyphenated service against single project-service key", link: "db", identifiers: []string{"project1-db"}, expected: []string{"project1-db"}},
			{name: "unhyphenated service against multi-segment project-service key", link: "proxy", identifiers: []string{"myproject-net-proxy"}, expected: []string{"myproject-net-proxy"}},
			{name: "unhyphenated service ambiguous across two projects", link: "db", identifiers: []string{"project1-db", "project2-db"}, expected: nil},
			{name: "unhyphenated service with three projects still ambiguous", link: "cache", identifiers: []string{"a-cache", "b-cache", "c-cache"}, expected: nil},
			{name: "multi-segment link against project-service key", link: "net-proxy", identifiers: []string{"myproject-net-proxy", "myproject-web"}, expected: []string{"myproject-net-proxy"}},
			{name: "multi-segment link sole candidate", link: "net-proxy", identifiers: []string{"myproject-net-proxy"}, expected: []string{"myproject-net-proxy"}},
			{name: "multi-segment link must not match trailing-token-only peer", link: "net-proxy", identifiers: []string{"myproject-other-proxy"}, expected: nil},
			{name: "multi-segment link must not match substring service", link: "net-proxy", identifiers: []string{"myproject-net"}, expected: nil},
			{name: "multi-segment link must not match longer multi-segment sibling", link: "net-proxy", identifiers: []string{"myproject-net-proxy-extra"}, expected: nil},
			{name: "multi-segment link ambiguous across two projects", link: "net-proxy", identifiers: []string{"stack-a-net-proxy", "stack-b-net-proxy"}, expected: nil},
			{name: "multi-segment link prefers exact over suffix when both present", link: "net-proxy", identifiers: []string{"net-proxy", "myproject-net-proxy"}, expected: []string{"net-proxy"}},
			{name: "unhyphenated db among multi-stack unrelated services", link: "db", identifiers: []string{"frontend-web", "backend-api", "data-db"}, expected: []string{"data-db"}},
			{
				name:        "hyphenated link among multi-stack noise",
				link:        "net-proxy",
				identifiers: []string{"frontend-web", "backend-api", "infra-net-proxy", "infra-other-proxy"},
				expected:    []string{"infra-net-proxy"},
			},
			{
				name:        "hyphenated link with same trailing token in another stack",
				link:        "auth-gateway",
				identifiers: []string{"stack-a-auth-gateway", "stack-b-edge-gateway"},
				expected:    []string{"stack-a-auth-gateway"},
			},
			{name: "empty link", link: "", identifiers: []string{"db"}, expected: nil},
			{name: "empty identifiers", link: "db", identifiers: []string{}, expected: nil},
			{name: "no candidates at all", link: "missing", identifiers: []string{"a", "b"}, expected: nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				link, identifiers, expected := tt.link, tt.identifiers, tt.expected

				matches := newMatchSet(identifierSet(identifiers...)).match(link)
				if expected == nil {
					assert.Empty(t, matches)

					return
				}

				assert.Equal(t, expected, matches)
			})
		}
	})
}

// identifierSet builds a bool set from identifier strings for resolveLinkToCanonicalKeys tests.
func identifierSet(ids ...string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}

	return set
}

// Test_buildLinkMatchIndexes verifies the aliases a link can use for each
// container: its canonical key, a bare name no other container shares, and its
// Docker ID, without one container's alias replacing another's key.
func Test_buildLinkMatchIndexes(t *testing.T) {
	t.Parallel()

	t.Run("should map unique bare names to canonical identifiers", func(t *testing.T) {
		t.Parallel()

		dep := mockLinkedContainer(t, "net-proxy", "id-proxy", "myproject", "net-proxy", nil, true)
		app := mockLinkedContainer(t, "web-app", "id-web", "myproject", "web", nil, true)

		containerMap := map[string]types.Container{
			"myproject-net-proxy": dep,
			"myproject-web":       app,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, "myproject-net-proxy", aliasToCanonical["myproject-net-proxy"])
		assert.Equal(t, "myproject-net-proxy", aliasToCanonical["net-proxy"])
		assert.Equal(t, "myproject-web", aliasToCanonical["web-app"])
		assert.Contains(t, idSet, "myproject-net-proxy")
		assert.Contains(t, idSet, "myproject-web")
		assert.Contains(t, idSet, "net-proxy")
		assert.Contains(t, idSet, "web-app")
	})

	t.Run("should omit bare name alias when it collides across containers", func(t *testing.T) {
		t.Parallel()

		c1 := mockTypes.NewMockContainer(t)
		c1.EXPECT().Name().Return("shared").Maybe()

		c2 := mockTypes.NewMockContainer(t)
		c2.EXPECT().Name().Return("shared").Maybe()

		containerMap := map[string]types.Container{
			"project1-service": c1,
			"project2-service": c2,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		_, hasBare := aliasToCanonical["shared"]
		assert.False(t, hasBare)
		assert.Equal(t, "project1-service", aliasToCanonical["project1-service"])
		assert.Equal(t, "project2-service", aliasToCanonical["project2-service"])
	})

	t.Run("should preserve canonical self-mapping when bare name equals another canonical key", func(t *testing.T) {
		t.Parallel()

		// Container A is keyed by "shared".
		// Container B's bare name is also "shared".
		// B must not overwrite or delete A's canonical self-mapping.
		canonicalOwner := mockTypes.NewMockContainer(t)
		canonicalOwner.EXPECT().Name().Return("shared").Maybe()

		bareClaimant := mockTypes.NewMockContainer(t)
		bareClaimant.EXPECT().Name().Return("shared").Maybe()

		containerMap := map[string]types.Container{
			"shared":          canonicalOwner,
			"project-service": bareClaimant,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, "shared", aliasToCanonical["shared"])
		assert.Equal(t, "project-service", aliasToCanonical["project-service"])
		assert.Contains(t, idSet, "shared")
		assert.Contains(t, idSet, "project-service")
		require.Len(t, idSet, 2)
	})

	t.Run("should map a Docker ID alias for concrete containers", func(t *testing.T) {
		t.Parallel()

		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, "gluetun-vpn", aliasToCanonical[providerID])
		assert.Contains(t, idSet, providerID)
	})

	t.Run("should skip an empty Docker ID", func(t *testing.T) {
		t.Parallel()

		gluetun := concreteLinkedContainer("", "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
		}

		idSet, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, "gluetun-vpn", aliasToCanonical["gluetun-vpn"])
		assert.Equal(t, "gluetun-vpn", aliasToCanonical["gluetun"])
		require.Len(t, idSet, 2)
	})

	t.Run("should skip a Docker ID that equals the canonical identifier", func(t *testing.T) {
		t.Parallel()

		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/"+providerID, "", "", "")

		containerMap := map[string]types.Container{
			providerID: gluetun,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, providerID, aliasToCanonical[providerID])
	})

	t.Run("should not overwrite an existing canonical key with a Docker ID", func(t *testing.T) {
		t.Parallel()

		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		owner := concreteLinkedContainer("owner-id", "/owner", "owner", "svc", "")
		claimant := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")

		containerMap := map[string]types.Container{
			providerID:    owner,
			"gluetun-vpn": claimant,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, providerID, aliasToCanonical[providerID])
	})

	t.Run("should retain a Docker ID alias when another container uses that ID as its name", func(t *testing.T) {
		t.Parallel()

		const providerID = "25e75393800b5c450a6841212a3b92ed28fa35414a586dec9f2c8a520d4910c2"

		gluetun := concreteLinkedContainer(providerID, "/gluetun", "gluetun", "vpn", "")
		decoy := concreteLinkedContainer("decoy-id", "/"+providerID, "other", "svc", "")

		containerMap := map[string]types.Container{
			"gluetun-vpn": gluetun,
			"other-svc":   decoy,
		}

		_, aliasToCanonical := buildLinkMatchIndexes(newTestLogger(t), containerMap)
		assert.Equal(t, "gluetun-vpn", aliasToCanonical[providerID])
	})
}

// Test_resolveLinkToCanonicalKeys verifies that a link resolves to the canonical
// keys of the containers it names, without duplicates, and to nothing when it
// is empty or names no container.
func Test_resolveLinkToCanonicalKeys(t *testing.T) {
	t.Parallel()

	t.Run("should resolve bare multi-segment container name to project-service key", func(t *testing.T) {
		t.Parallel()

		idSet := identifierSet("myproject-net-proxy", "myproject-web", "net-proxy", "web-app")
		alias := map[string]string{
			"myproject-net-proxy": "myproject-net-proxy",
			"myproject-web":       "myproject-web",
			"net-proxy":           "myproject-net-proxy",
			"web-app":             "myproject-web",
		}

		keys := resolveLinkToCanonicalKeys("net-proxy", newMatchSet(idSet), alias)
		assert.Equal(t, []string{"myproject-net-proxy"}, keys)
	})

	t.Run("should resolve project-service suffix match without bare-name alias", func(t *testing.T) {
		t.Parallel()

		idSet := identifierSet("myproject-net-proxy")
		alias := map[string]string{"myproject-net-proxy": "myproject-net-proxy"}

		keys := resolveLinkToCanonicalKeys("net-proxy", newMatchSet(idSet), alias)
		assert.Equal(t, []string{"myproject-net-proxy"}, keys)
	})

	t.Run("should return nil for empty or unmatched links", func(t *testing.T) {
		t.Parallel()

		idSet := identifierSet("myproject-db")
		alias := map[string]string{"myproject-db": "myproject-db"}

		assert.Nil(t, resolveLinkToCanonicalKeys("", newMatchSet(idSet), alias))
		assert.Nil(t, resolveLinkToCanonicalKeys("missing", newMatchSet(idSet), alias))
	})

	t.Run("should dedupe when bare alias and canonical both match", func(t *testing.T) {
		t.Parallel()

		idSet := identifierSet("myproject-db", "db")
		alias := map[string]string{
			"myproject-db": "myproject-db",
			"db":           "myproject-db",
		}

		keys := resolveLinkToCanonicalKeys("db", newMatchSet(idSet), alias)
		assert.Equal(t, []string{"myproject-db"}, keys)
	})
}

// TestDependencyLinkFormPermutations verifies that each form a dependency link
// can take sorts the dependency before its dependent, across Compose stacks,
// replicas, and shared or ambiguous names.
func TestDependencyLinkFormPermutations(t *testing.T) {
	t.Parallel()

	// Links() values from Watchtower depends-on, Compose depends_on, Docker links,
	// and network_mode must resolve against ResolveContainerIdentifier graph keys.
	t.Run("sorts dependency before dependent for each link form", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name             string
			depName          string
			depProject       string
			depService       string
			dependentName    string
			dependentProject string
			dependentService string
			links            []string
			useCompose       bool
		}{
			{
				name:             "Watchtower depends-on bare container_name against project-service key",
				depName:          "net-proxy",
				depProject:       "myproject",
				depService:       "net-proxy",
				dependentName:    "web-app",
				dependentProject: "myproject",
				dependentService: "web",
				links:            []string{"net-proxy"},
				useCompose:       true,
			},
			{
				name:             "Watchtower depends-on bare name without Compose labels",
				depName:          "database",
				depProject:       "",
				depService:       "",
				dependentName:    "web",
				dependentProject: "",
				dependentService: "",
				links:            []string{"database"},
				useCompose:       true,
			},
			{
				name:             "Watchtower depends-on with useComposeDependsOn false",
				depName:          "postgres",
				depProject:       "stack",
				depService:       "postgres",
				dependentName:    "api",
				dependentProject: "stack",
				dependentService: "api",
				links:            []string{"postgres"},
				useCompose:       false,
			},
			{
				name:             "Compose depends_on project-qualified service link",
				depName:          "myproject-database",
				depProject:       "myproject",
				depService:       "database",
				dependentName:    "myproject-web",
				dependentProject: "myproject",
				dependentService: "web",
				links:            []string{"myproject-database"},
				useCompose:       true,
			},
			{
				name:             "Compose depends_on bare service name against project-service key",
				depName:          "myproject-database",
				depProject:       "myproject",
				depService:       "database",
				dependentName:    "myproject-web",
				dependentProject: "myproject",
				dependentService: "web",
				links:            []string{"database"},
				useCompose:       true,
			},
			{
				name:             "Compose depends_on multi-segment service under project prefix",
				depName:          "myproject-net-proxy",
				depProject:       "myproject",
				depService:       "net-proxy",
				dependentName:    "myproject-web",
				dependentProject: "myproject",
				dependentService: "web",
				links:            []string{"net-proxy"},
				useCompose:       true,
			},
			{
				name:             "network_mode HostConfig bare container name",
				depName:          "vpn",
				depProject:       "stack",
				depService:       "vpn",
				dependentName:    "client",
				dependentProject: "stack",
				dependentService: "client",
				links:            []string{"vpn"},
				useCompose:       true,
			},
			{
				name:             "network_mode with useComposeDependsOn false",
				depName:          "vpn",
				depProject:       "stack",
				depService:       "vpn",
				dependentName:    "client",
				dependentProject: "stack",
				dependentService: "client",
				links:            []string{"vpn"},
				useCompose:       false,
			},
			{
				name:             "legacy Docker link bare container name",
				depName:          "db",
				depProject:       "",
				depService:       "",
				dependentName:    "app",
				dependentProject: "",
				dependentService: "",
				links:            []string{"db"},
				useCompose:       true,
			},
			{
				name:             "cross-project Watchtower depends-on by container name",
				depName:          "app1-database",
				depProject:       "app1",
				depService:       "database",
				dependentName:    "app2-web",
				dependentProject: "app2",
				dependentService: "web",
				links:            []string{"app1-database"},
				useCompose:       true,
			},
			{
				name:             "Compose service name without container_name override",
				depName:          "stack-cache",
				depProject:       "stack",
				depService:       "cache",
				dependentName:    "stack-worker",
				dependentProject: "stack",
				dependentService: "worker",
				links:            []string{"cache"},
				useCompose:       true,
			},
			{
				name:             "normalized bare service name after Links processing",
				depName:          "redis",
				depProject:       "cache",
				depService:       "redis",
				dependentName:    "app",
				dependentProject: "cache",
				dependentService: "app",
				links:            []string{"redis"},
				useCompose:       true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				depName, depProject, depService, dependentName, dependentProject, dependentService, links, useCompose := tt.depName, tt.depProject, tt.depService, tt.dependentName, tt.dependentProject, tt.dependentService, tt.links, tt.useCompose

				dep := mockLinkedContainer(t, depName, "id-dep", depProject, depService, nil, useCompose)
				dependent := mockLinkedContainer(
					t, dependentName,
					"id-dependent",
					dependentProject,
					dependentService,
					links,
					useCompose,
				)

				containers := []types.Container{dependent, dep}
				err := dependencySorter{}.Sort(newTestLogger(t), containers, useCompose)
				require.NoError(t, err)
				assert.Equal(t, depName, containers[0].Name(), "dependency must sort before dependent")
				assert.Equal(t, dependentName, containers[1].Name())
			})
		}
	})

	t.Run("should order multiple dependents after a shared network provider", func(t *testing.T) {
		t.Parallel()

		vpn := mockLinkedContainer(t, "vpn", "id-vpn", "stack", "vpn", nil, true)
		clientA := mockLinkedContainer(t, "client-a", "id-a", "stack", "client-a", []string{"vpn"}, true)
		clientB := mockLinkedContainer(t, "client-b", "id-b", "stack", "client-b", []string{"vpn"}, true)

		containers := []types.Container{clientA, clientB, vpn}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		assert.Equal(t, "vpn", containers[0].Name())
		names := []string{containers[1].Name(), containers[2].Name()}
		assert.ElementsMatch(t, []string{"client-a", "client-b"}, names)
	})

	t.Run("should order when dependent has multiple dependencies", func(t *testing.T) {
		t.Parallel()

		db := mockLinkedContainer(t, "db", "id-db", "app", "db", nil, true)
		cache := mockLinkedContainer(t, "cache", "id-cache", "app", "cache", nil, true)
		web := mockLinkedContainer(t, "web", "id-web", "app", "web", []string{"db", "cache"}, true)

		containers := []types.Container{web, db, cache}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		assert.Equal(t, "web", containers[2].Name())

		ordered := []string{containers[0].Name(), containers[1].Name(), containers[2].Name()}
		assertOrderBefore(t, ordered, "db", "web")
		assertOrderBefore(t, ordered, "cache", "web")
	})

	t.Run("should match Compose replica identifiers from a service link", func(t *testing.T) {
		t.Parallel()

		db1 := mockLinkedContainer(t, "myproject-db-1", "id-db1", "myproject", "db", nil, true)
		db2 := mockLinkedContainer(t, "myproject-db-2", "id-db2", "myproject", "db", nil, true)
		app := mockLinkedContainer(t, "myproject-app", "id-app", "myproject", "app", []string{"myproject-db"}, true)

		containers := []types.Container{app, db1, db2}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		assert.Equal(t, "myproject-app", containers[2].Name())
		assert.ElementsMatch(t, []string{"myproject-db-1", "myproject-db-2"}, []string{containers[0].Name(), containers[1].Name()})
	})

	t.Run("should link a bare service name to the service in every project", func(t *testing.T) {
		t.Parallel()

		db1 := mockLinkedContainer(t, "project1-db", "id-1", "project1", "db", nil, true)
		db2 := mockLinkedContainer(t, "project2-db", "id-2", "project2", "db", nil, true)
		app := mockLinkedContainer(t, "project3-app", "id-3", "project3", "app", []string{"db"}, true)

		_, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), []types.Container{app, db1, db2}, true)
		require.NoError(t, err)
		assert.Equal(t, 2, indegree["project3-app"])
		assert.Contains(t, adjacency["project1-db"], "project3-app")
		assert.Contains(t, adjacency["project2-db"], "project3-app")
	})

	t.Run("should ignore missing dependency targets without failing sort", func(t *testing.T) {
		t.Parallel()

		app := mockLinkedContainer(t, "app", "id-app", "myproject", "app", []string{"missing-dep"}, true)
		standalone := mockLinkedContainer(t, "other", "id-other", "myproject", "other", nil, true)

		containers := []types.Container{app, standalone}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"app", "other"}, []string{containers[0].Name(), containers[1].Name()})
	})

	t.Run("should keep Watchtower last when linked non-Watchtower containers sort first", func(t *testing.T) {
		t.Parallel()

		db := mockLinkedContainer(t, "db", "id-db", "myproject", "db", nil, true)
		web := mockLinkedContainer(t, "web", "id-web", "myproject", "web", []string{"db"}, true)

		wt := mockTypes.NewMockContainer(t)
		wt.EXPECT().Name().Return("watchtower").Maybe()
		wt.EXPECT().ID().Return(types.ContainerID("id-wt")).Maybe()
		wt.EXPECT().IsWatchtower().Return(true).Maybe()
		wt.EXPECT().Links(true).Return(nil).Maybe()
		wt.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
			Name:   "/watchtower",
			Config: &dockerContainer.Config{Labels: map[string]string{}},
		}).Maybe()

		containers := []types.Container{wt, web, db}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		assert.Equal(t, "db", containers[0].Name())
		assert.Equal(t, "web", containers[1].Name())
		assert.Equal(t, "watchtower", containers[2].Name())
	})

	t.Run("should order independent stacks without cross-linking same service names", func(t *testing.T) {
		t.Parallel()

		// Two Compose projects each have db→web.
		// Bare link "db" is ambiguous and must not couple stacks.
		// Each web only orders relative to its own db when links use project-qualified or unique names.
		dbA := mockLinkedContainer(t, "stack-a-db", "id-da", "stack-a", "db", nil, true)
		webA := mockLinkedContainer(
			t, "stack-a-web", "id-wa", "stack-a", "web", []string{"stack-a-db"}, true,
		)
		dbB := mockLinkedContainer(t, "stack-b-db", "id-db", "stack-b", "db", nil, true)
		webB := mockLinkedContainer(
			t, "stack-b-web", "id-wb", "stack-b", "web", []string{"stack-b-db"}, true,
		)

		containers := []types.Container{webA, webB, dbA, dbB}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(t, ordered, "stack-a-db", "stack-a-web")
		assertOrderBefore(t, ordered, "stack-b-db", "stack-b-web")
	})

	t.Run("should order cross-stack Watchtower depends-on by foreign container_name", func(t *testing.T) {
		t.Parallel()

		// Provider stack owns the named network peer.
		// Consumer stack depends on it via Watchtower depends-on / network_mode container name.
		provider := mockLinkedContainer(
			t, "shared-net-proxy", "id-prov", "infra", "net-proxy", nil, true,
		)
		consumer := mockLinkedContainer(
			t, "app-web", "id-cons", "app", "web", []string{"shared-net-proxy"}, true,
		)
		unrelated := mockLinkedContainer(
			t, "other-db", "id-other", "other", "db", nil, true,
		)

		containers := []types.Container{consumer, unrelated, provider}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(t, ordered, "shared-net-proxy", "app-web")
		assert.ElementsMatch(t, []string{"shared-net-proxy", "app-web", "other-db"}, ordered)
	})

	t.Run("should not couple stacks when multi-segment services share a trailing token", func(t *testing.T) {
		t.Parallel()

		// stack-a has service net-proxy
		// stack-b has service other-proxy.
		// A dependent linking to net-proxy must only wait on stack-a's peer, not other-proxy.
		proxyA := mockLinkedContainer(
			t, "stack-a-net-proxy", "id-pa", "stack-a", "net-proxy", nil, true,
		)
		proxyB := mockLinkedContainer(
			t, "stack-b-other-proxy", "id-pb", "stack-b", "other-proxy", nil, true,
		)
		client := mockLinkedContainer(
			t, "stack-a-client", "id-cl", "stack-a", "client", []string{"net-proxy"}, true,
		)

		_, indegree, adjacency, _, err := buildDependencyGraph(newTestLogger(t), []types.Container{client, proxyB, proxyA}, true)
		require.NoError(t, err)
		assert.Equal(t, 1, indegree["stack-a-client"])
		assert.Contains(t, adjacency["stack-a-net-proxy"], "stack-a-client")
		assert.NotContains(t, adjacency["stack-b-other-proxy"], "stack-a-client")

		containers := []types.Container{client, proxyB, proxyA}
		err = dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(t, ordered, "stack-a-net-proxy", "stack-a-client")
	})

	t.Run("should order multi-segment container_name dependency among multi-stack peers", func(t *testing.T) {
		t.Parallel()

		// Explicit container_name equals multi-segment service and Compose keys differ.
		provider := mockLinkedContainer(
			t, "net-proxy", "id-np", "media", "net-proxy", nil, true,
		)
		dependent := mockLinkedContainer(
			t, "web-app", "id-wa", "media", "web", []string{"net-proxy"}, true,
		)
		foreign := mockLinkedContainer(
			t, "net-proxy-other", "id-fo", "other", "net-proxy-other", nil, true,
		)
		// Same trailing token in another stack must not steal the edge.
		decoy := mockLinkedContainer(
			t, "edge-proxy", "id-de", "edge", "edge-proxy", nil, true,
		)

		containers := []types.Container{dependent, foreign, decoy, provider}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(t, ordered, "net-proxy", "web-app")
	})

	t.Run("should order chain across three stacks with mixed link forms", func(t *testing.T) {
		t.Parallel()

		// data.db ← (compose service) app.worker ← (watchtower container_name) edge.client
		db := mockLinkedContainer(t, "data-db", "id-db", "data", "db", nil, true)
		worker := mockLinkedContainer(
			t, "app-worker", "id-wk", "app", "worker", []string{"db"}, true,
		)
		// Watchtower depends-on uses worker's container_name.
		client := mockLinkedContainer(
			t, "edge-client", "id-cl", "edge", "client", []string{"app-worker"}, true,
		)

		containers := []types.Container{client, worker, db}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)

		ordered := make([]string, len(containers))
		for i, c := range containers {
			ordered[i] = c.Name()
		}

		assertOrderBefore(t, ordered, "data-db", "app-worker")
		assertOrderBefore(t, ordered, "app-worker", "edge-client")
		assertOrderBefore(t, ordered, "data-db", "edge-client")
	})

	t.Run("should not create edge for hyphenated link against trailing-token-only peer", func(t *testing.T) {
		t.Parallel()

		// Regression: extractServiceName("net-proxy") and extractServiceName("other-proxy")
		// both yield "proxy".
		// Matching must not treat them as the same dependency.
		decoy := mockLinkedContainer(
			t, "other-proxy", "id-decoy", "other", "other-proxy", nil, true,
		)
		dependent := mockLinkedContainer(
			t, "web", "id-web", "app", "web", []string{"net-proxy"}, true,
		)

		containers := []types.Container{dependent, decoy}
		err := dependencySorter{}.Sort(newTestLogger(t), containers, true)
		require.NoError(t, err)
		// No dependency edge: both indegree 0. Order is reverse-alpha among keys.
		assert.ElementsMatch(t, []string{"web", "other-proxy"}, []string{containers[0].Name(), containers[1].Name()})
	})
}
