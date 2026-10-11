package actions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/container"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// restartSpec describes a container for the implicit restart tests.
type restartSpec struct {
	// name is the container name, without the leading slash.
	name string
	// project, service, and number are the Compose labels. Empty means unset.
	project, service, number string
	// dependsOn is the Watchtower depends-on label.
	dependsOn string
	// networkMode is the network mode, such as container:vpn.
	networkMode string
	// watchtower marks the container as a Watchtower instance.
	watchtower bool
	// stale marks the container as updating.
	stale bool
	// unmonitored leaves the container out of the monitored containers.
	unmonitored bool
	// excluded keeps the container from being marked or marking others.
	excluded bool
}

// restartSpecID returns a stable 64-character hex container ID for a name.
func restartSpecID(name string) string {
	sum := sha256.Sum256([]byte(name))

	return hex.EncodeToString(sum[:])
}

// build builds the container the description stands for.
func (s restartSpec) build() types.Container {
	labels := map[string]string{}

	for key, value := range map[string]string{
		"com.docker.compose.project":                s.project,
		"com.docker.compose.service":                s.service,
		"com.docker.compose.container-number":       s.number,
		"com.centurylinklabs.watchtower.depends-on": s.dependsOn,
	} {
		if value != "" {
			labels[key] = value
		}
	}

	if s.watchtower {
		labels["com.centurylinklabs.watchtower"] = "true"
	}

	c := container.NewContainer(nil, &dockerContainer.InspectResponse{
		ID:         restartSpecID(s.name),
		Name:       "/" + s.name,
		HostConfig: &dockerContainer.HostConfig{NetworkMode: dockerContainer.NetworkMode(s.networkMode)},
		Config:     &dockerContainer.Config{Labels: labels},
	}, nil)
	c.SetStale(s.stale)

	return c
}

// TestMarkImplicitRestarts verifies which containers are marked as linked to a
// restarting container: those whose links name it, directly or through other
// marked containers, by container name, Compose service, replica, or Docker
// ID. A Watchtower container is marked through its own links and marks the
// containers that name it exactly, or as the only replica of its Compose
// service. A link that names an unmonitored container, or a service more than
// one service could be, marks nothing.
func TestMarkImplicitRestarts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		specs []restartSpec
		want  []string
	}{
		{
			name:  "service name of a single container",
			specs: []restartSpec{{name: "app", dependsOn: "db"}, {name: "project1-db", stale: true}},
			want:  []string{"app"},
		},
		{
			name: "exact name before a service name",
			specs: []restartSpec{
				{name: "app", dependsOn: "db"},
				{name: "db"},
				{name: "project1-db", stale: true},
			},
			want: nil,
		},
		{
			name:  "project-qualified service",
			specs: []restartSpec{{name: "app", dependsOn: "myproject-db"}, {name: "myproject-db", stale: true}},
			want:  []string{"app"},
		},
		{
			name: "multi-segment container name",
			specs: []restartSpec{
				{name: "app", dependsOn: "production-api-gateway"},
				{name: "production-api-gateway", stale: true},
			},
			want: []string{"app"},
		},
		{
			name: "replica of a project-qualified service",
			specs: []restartSpec{
				{name: "app", dependsOn: "myapp-db"},
				{name: "myapp-db-1", project: "myapp", service: "db", number: "1", stale: true},
			},
			want: []string{"app"},
		},
		{
			name: "hyphenated service in another project",
			specs: []restartSpec{
				{name: "app1-foo-1", project: "app1", service: "foo", number: "1", dependsOn: "watchtower-test-database"},
				{
					name: "database1-watchtower-test-database-1", project: "database1",
					service: "watchtower-test-database", number: "1", stale: true,
				},
			},
			want: []string{"app1-foo-1"},
		},
		{
			name: "one replica of a service with two",
			specs: []restartSpec{
				{name: "app", dependsOn: "db"},
				{name: "stack-db-1", project: "stack", service: "db", number: "1"},
				{name: "stack-db-2", project: "stack", service: "db", number: "2", stale: true},
			},
			want: []string{"app"},
		},
		{
			name: "same service in two projects",
			specs: []restartSpec{
				{name: "app", dependsOn: "db"},
				{name: "project1-db-1", project: "project1", service: "db", number: "1", stale: true},
				{name: "project2-db-1", project: "project2", service: "db", number: "1"},
			},
			want: []string{"app"},
		},
		{
			name: "hyphenated service in two other projects, one by container name",
			specs: []restartSpec{
				{
					name: "app-web-1", project: "app", service: "web", number: "1",
					dependsOn: "watchtower-test-database",
				},
				{
					name: "app-web-2", project: "app", service: "web", number: "2",
					dependsOn: "watchtower-test-database",
				},
				{
					name: "database1-watchtower-test-database-1", project: "database1",
					service: "watchtower-test-database", number: "1",
				},
				{
					name: "watchtower-test-database2", project: "database2",
					service: "watchtower-test-database", number: "1", stale: true,
				},
			},
			want: []string{"app-web-1", "app-web-2"},
		},
		{
			name: "service of the dependent's project unmonitored",
			specs: []restartSpec{
				{name: "a-web-1", project: "a", service: "web", number: "1", dependsOn: "db"},
				{name: "a-db-1", project: "a", service: "db", number: "1", unmonitored: true},
				{name: "b-db-1", project: "b", service: "db", number: "1", stale: true},
			},
			want: nil,
		},
		{
			name: "service in two projects, one of them the dependent's",
			specs: []restartSpec{
				{name: "a-web-1", project: "a", service: "web", number: "1", dependsOn: "db"},
				{name: "a-db-1", project: "a", service: "db", number: "1"},
				{name: "b-db-1", project: "b", service: "db", number: "1", stale: true},
			},
			want: nil,
		},
		{
			name: "through a marked container",
			specs: []restartSpec{
				{name: "web", dependsOn: "api"},
				{name: "api", dependsOn: "db"},
				{name: "db", stale: true},
			},
			want: []string{"api", "web"},
		},
		{
			name: "through an excluded container",
			specs: []restartSpec{
				{name: "web", dependsOn: "api"},
				{name: "api", dependsOn: "db", excluded: true},
				{name: "db", stale: true},
			},
			want: nil,
		},
		{
			name: "Watchtower routed through a VPN container",
			specs: []restartSpec{
				{name: "watchtower", watchtower: true, networkMode: "container:" + restartSpecID("vpn")},
				{name: "vpn", stale: true},
			},
			want: []string{"watchtower"},
		},
		{
			name: "container naming a restarting Watchtower container",
			specs: []restartSpec{
				{name: "watchtower", watchtower: true, stale: true},
				{name: "app", dependsOn: "watchtower"},
			},
			want: []string{"app"},
		},
		{
			name: "container naming a restarting Watchtower container's Compose service",
			specs: []restartSpec{
				{name: "infra-watchtower-1", project: "infra", service: "watchtower", number: "1", watchtower: true, stale: true},
				{name: "app", dependsOn: "infra-watchtower"},
			},
			want: []string{"app"},
		},
		{
			name: "unmonitored container named exactly",
			specs: []restartSpec{
				{name: "app", dependsOn: "db"},
				{name: "db", unmonitored: true},
				{name: "db-1", stale: true},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var all, monitored, unmonitored []types.Container

			excluded := map[types.ContainerID]struct{}{}

			for _, spec := range tt.specs {
				c := spec.build()
				all = append(all, c)

				switch {
				case spec.unmonitored:
					unmonitored = append(unmonitored, c)
				case spec.excluded:
					monitored = append(monitored, c)
					excluded[c.ID()] = struct{}{}
				default:
					monitored = append(monitored, c)
				}
			}

			graph, err := sorter.NewDependencyGraph(testLogger(), monitored, unmonitored, false)
			require.NoError(t, err)

			markImplicitRestarts(testLogger(), graph, all, monitored, excluded)

			var marked []string

			for _, c := range all {
				if c.(*container.Container).IsLinkedToRestarting() {
					marked = append(marked, c.Name())
				}
			}

			sort.Strings(marked)
			assert.Equal(t, tt.want, marked)
		})
	}
}

// TestUpdate_WarnsOnUnresolvedDependsOn verifies that a scan logs a warning for
// each depends-on entry that names no container, and not for one that names a
// service shared by several projects.
func TestUpdate_WarnsOnUnresolvedDependsOn(t *testing.T) {
	t.Parallel()

	specs := []restartSpec{
		{name: "app", dependsOn: "db,missing"},
		{name: "a-db-1", project: "a", service: "db", number: "1"},
		{name: "b-db-1", project: "b", service: "db", number: "1"},
	}

	containers := make([]types.Container, 0, len(specs))
	for _, spec := range specs {
		containers = append(containers, spec.build())
	}

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return(containers, nil).Once()
	client.EXPECT().IsContainerStale(mock.Anything, mock.Anything, mock.Anything).
		Return(false, types.ImageID(""), "", nil)

	// The update logs from several goroutines, so the buffer is synchronized.
	var output bytes.Buffer

	log := zerolog.New(zerolog.SyncWriter(&output))

	_, _, err := Update(&log, t.Context(), client, types.UpdateParams{})
	require.NoError(t, err)

	assert.Contains(t, output.String(),
		`{"level":"warn","container":"app","depends_on":"missing","message":"Ignoring depends-on entry that names no container"}`)
	assert.NotContains(t, output.String(), `"depends_on":"db","message":"Ignoring depends-on entry`)
}
