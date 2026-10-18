package actions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdate_ComposeDependencyCycle verifies that every container in a
// circular dependency is skipped while the rest of the session updates. The
// cycle runs through Compose depends_on labels, which name services rather
// than containers, so it is only visible on the dependency graph the
// containers are ordered by.
func TestUpdate_ComposeDependencyCycle(t *testing.T) {
	t.Parallel()

	// composeContainer returns a running, stale-able Compose container of the
	// stack project whose name differs from its service name.
	composeContainer := func(id, service, dependsOn string) types.Container {
		labels := map[string]string{
			"com.docker.compose.project":          "stack",
			"com.docker.compose.service":          service,
			"com.docker.compose.container-number": "1",
		}
		if dependsOn != "" {
			labels["com.docker.compose.depends_on"] = dependsOn
		}

		return mockActions.CreateMockContainerWithConfig(
			id,
			"/"+service+"-server",
			"org/"+service+":latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{Image: "org/" + service + ":latest", Labels: labels},
		)
	}

	tests := []struct {
		name string
		// webDependsOn is the depends_on label of the web service, which
		// closes the cycle with the api service.
		webDependsOn string
	}{
		{
			name:         "cycle next to an unrelated container",
			webDependsOn: "api:service_started:false",
		},
		{
			name:         "cycle member that depends on an updated container",
			webDependsOn: "api:service_started:false,app:service_started:false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const newID = types.ContainerID("e000000000000000000000000000000000000000000000000000000000000000")

			web := composeContainer("a000000000000000000000000000000000000000000000000000000000000000", "web", tt.webDependsOn)
			api := composeContainer("b000000000000000000000000000000000000000000000000000000000000000", "api", "web:service_started:false")
			app := composeContainer("c000000000000000000000000000000000000000000000000000000000000000", "app", "")

			client := mockContainer.NewMockClient(t)
			client.EXPECT().ListContainers(mock.Anything, mock.Anything).
				Return([]types.Container{web, api, app}, nil).Once()
			client.EXPECT().IsContainerStale(mock.Anything, mock.Anything, mock.Anything).
				Return(true, types.ImageID("sha256:new"), "", nil)
			client.EXPECT().GetImageAnnotations(mock.Anything, mock.Anything).Return(oci.Annotations{}).Maybe()

			// Record the containers that are stopped for an update.
			var (
				mu      sync.Mutex
				stopped []string
			)

			client.EXPECT().StopAndRemoveContainer(mock.Anything, mock.Anything, mock.Anything).
				Run(func(_ context.Context, c types.Container, _ time.Duration) {
					mu.Lock()
					defer mu.Unlock()

					stopped = append(stopped, c.Name())
				}).
				Return(nil).Maybe()
			client.EXPECT().CreateContainer(mock.Anything, mock.Anything).Return(newID, nil).Maybe()
			client.EXPECT().StartContainerByID(mock.Anything, newID).Return(nil).Maybe()
			client.EXPECT().GetContainer(mock.Anything, newID).Return(app, nil).Maybe()

			report, _, err := update(newTestLogger(t), t.Context(), client,
				types.UpdateParams{UseComposeDependsOn: true}, ratelimit.New())
			require.NoError(t, err)

			assert.ElementsMatch(t, []string{"web-server", "api-server"}, reportNames(report.Skipped()),
				"both cycle members are skipped")
			assert.Equal(t, []string{"app-server"}, reportNames(report.Updated()),
				"the container outside the cycle is updated")
			assert.Equal(t, []string{"app-server"}, stopped, "only the updated container is restarted")
		})
	}
}

// TestUpdate_LinkToCycleMember verifies that a link to a skipped cycle member
// still names that container while the rest is sorted. The c container
// depends on db, which is in a cycle with e. With db set aside, the link would
// otherwise fall back to the db-1 replica name, and since db-1 depends on c,
// that phantom edge would form a cycle and abort the session.
func TestUpdate_LinkToCycleMember(t *testing.T) {
	t.Parallel()

	// dependent returns a running container whose depends-on label lists links.
	dependent := func(id, name, links string) types.Container {
		labels := map[string]string{}
		if links != "" {
			labels["com.centurylinklabs.watchtower.depends-on"] = links
		}

		return mockActions.CreateMockContainerWithConfig(
			id,
			"/"+name,
			"org/"+name+":latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{Image: "org/" + name + ":latest", Labels: labels},
		)
	}

	db := dependent("d000000000000000000000000000000000000000000000000000000000000000", "db", "e")
	e := dependent("e000000000000000000000000000000000000000000000000000000000000000", "e", "db")
	c := dependent("c000000000000000000000000000000000000000000000000000000000000000", "c", "db")
	replica := dependent("d100000000000000000000000000000000000000000000000000000000000000", "db-1", "c")

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).
		Return([]types.Container{db, e, c, replica}, nil).Once()
	client.EXPECT().IsContainerStale(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ct types.Container, _ types.UpdateParams) (bool, types.ImageID, string, error) {
			return ct.Name() == "c", types.ImageID("sha256:new"), "", nil
		})
	client.EXPECT().GetImageAnnotations(mock.Anything, mock.Anything).Return(oci.Annotations{}).Maybe()
	client.EXPECT().StopAndRemoveContainer(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Record the order in which containers are recreated.
	var created []string

	client.EXPECT().CreateContainer(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ct types.Container) (types.ContainerID, error) {
			created = append(created, ct.Name())

			return ct.ID(), nil
		}).Maybe()
	client.EXPECT().StartContainerByID(mock.Anything, mock.Anything).Return(nil).Maybe()
	client.EXPECT().GetContainer(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, id types.ContainerID) (types.Container, error) {
			for _, ct := range []types.Container{c, replica} {
				if ct.ID() == id {
					return ct, nil
				}
			}

			return nil, errNoContainer
		}).Maybe()

	report, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{}, ratelimit.New())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"db", "e"}, reportNames(report.Skipped()), "both cycle members are skipped")
	assert.Equal(t, []string{"c"}, reportNames(report.Updated()), "the stale container is updated")
	assert.Equal(t, []string{"c", "db-1"}, created, "the dependent restarts after the container it depends on")
}

// TestUpdate_HostConfigDependencyCycle verifies that a cycle closed by host
// config links that name containers by Docker ID is skipped. The app container
// shares the network of vpn through network_mode container:<id>, and vpn
// mounts the volumes of app through volumes-from <id>:ro.
func TestUpdate_HostConfigDependencyCycle(t *testing.T) {
	t.Parallel()

	const (
		appID = "a000000000000000000000000000000000000000000000000000000000000000"
		vpnID = "b000000000000000000000000000000000000000000000000000000000000000"
		webID = "c000000000000000000000000000000000000000000000000000000000000000"
	)

	// hostContainer returns a running container with the given host config.
	hostContainer := func(id, name string, hostConfig *dockerContainer.HostConfig) types.Container {
		image := "org/" + name + ":latest"

		return container.NewContainer(nil, &dockerContainer.InspectResponse{
			ID:         id,
			Name:       "/" + name,
			Image:      image,
			State:      &dockerContainer.State{Running: true},
			Created:    time.Now().Format(time.RFC3339Nano),
			HostConfig: hostConfig,
			Config:     &dockerContainer.Config{Image: image, Labels: map[string]string{}},
		}, mockActions.CreateMockImageInfo(image))
	}

	app := hostContainer(appID, "app", &dockerContainer.HostConfig{NetworkMode: "container:" + vpnID})
	vpn := hostContainer(vpnID, "vpn", &dockerContainer.HostConfig{VolumesFrom: []string{appID + ":ro"}})
	web := hostContainer(webID, "web", &dockerContainer.HostConfig{})

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).
		Return([]types.Container{app, vpn, web}, nil).Once()
	client.EXPECT().IsContainerStale(mock.Anything, mock.Anything, mock.Anything).
		Return(true, types.ImageID("sha256:new"), "", nil)
	client.EXPECT().GetImageAnnotations(mock.Anything, mock.Anything).Return(oci.Annotations{}).Maybe()

	var stopped []string

	client.EXPECT().StopAndRemoveContainer(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, c types.Container, _ time.Duration) { stopped = append(stopped, c.Name()) }).
		Return(nil).Maybe()
	client.EXPECT().CreateContainer(mock.Anything, mock.Anything).Return(types.ContainerID(webID), nil).Maybe()
	client.EXPECT().StartContainerByID(mock.Anything, mock.Anything).Return(nil).Maybe()
	client.EXPECT().GetContainer(mock.Anything, mock.Anything).Return(web, nil).Maybe()

	report, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{}, ratelimit.New())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"app", "vpn"}, reportNames(report.Skipped()), "both cycle members are skipped")
	assert.Equal(t, []string{"web"}, reportNames(report.Updated()), "the container outside the cycle is updated")
	assert.Equal(t, []string{"web"}, stopped, "only the updated container is restarted")
}

// errNoContainer is returned by the fake client for an unknown container ID.
var errNoContainer = errors.New("no such container")

// reportNames returns the container names of the given reports.
func reportNames(reports []types.ContainerReport) []string {
	names := make([]string, 0, len(reports))
	for _, r := range reports {
		names = append(names, r.Name())
	}

	return names
}
