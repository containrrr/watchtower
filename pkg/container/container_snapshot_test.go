package container

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerNetwork "github.com/moby/moby/api/types/network"
)

// TestContainer_ChangesLeaveEarlierSnapshotsUnchanged verifies that changing a
// container replaces its inspect data rather than editing it, so inspect data
// obtained earlier from ContainerInfo stays exactly as it was.
func TestContainer_ChangesLeaveEarlierSnapshotsUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// change modifies the container.
		change func(c *Container)
	}{
		{name: "set label", change: func(c *Container) { c.SetLabel("added", "value") }},
		{name: "delete label", change: func(c *Container) { c.DeleteLabel("kept") }},
		{name: "set image name", change: func(c *Container) { c.SetImageName("org/app:2") }},
		{name: "verify configuration", change: func(c *Container) { _ = c.VerifyConfiguration() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := MockContainer(
				WithLabels(map[string]string{"kept": "value"}),
				WithImageName("org/app:1"),
				// An empty port binding is removed by VerifyConfiguration.
				func(cnt *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
					cnt.HostConfig.PortBindings = dockerNetwork.PortMap{
						dockerNetwork.MustParsePort("80/tcp"): nil,
						dockerNetwork.Port{}:                  nil,
					}
				},
			)

			before := c.ContainerInfo()
			labels := len(before.Config.Labels)
			image := before.Config.Image
			bindings := len(before.HostConfig.PortBindings)
			exposed := before.Config.ExposedPorts

			tt.change(c)

			assert.Len(t, before.Config.Labels, labels)
			assert.Equal(t, "value", before.Config.Labels["kept"])
			assert.NotContains(t, before.Config.Labels, "added")
			assert.Equal(t, image, before.Config.Image)
			assert.Len(t, before.HostConfig.PortBindings, bindings)
			assert.Equal(t, exposed, before.Config.ExposedPorts)
		})
	}
}

// TestContainer_ChangesAreVisible verifies that the changes are visible
// through the container afterwards.
func TestContainer_ChangesAreVisible(t *testing.T) {
	t.Parallel()

	c := MockContainer(
		WithLabels(map[string]string{"removed": "value"}),
		func(cnt *dockerContainer.InspectResponse, _ *dockerImage.InspectResponse) {
			cnt.HostConfig.PortBindings = dockerNetwork.PortMap{
				dockerNetwork.MustParsePort("80/tcp"): nil,
				dockerNetwork.Port{}:                  nil,
			}
		},
	)

	c.SetLabel("added", "value")
	c.DeleteLabel("removed")
	c.SetImageName("org/app:2")
	require.NoError(t, c.VerifyConfiguration())

	info := c.ContainerInfo()
	assert.Equal(t, map[string]string{"added": "value"}, info.Config.Labels)
	assert.Equal(t, "org/app:2", info.Config.Image)
	assert.Equal(t, "org/app:2", c.ImageName())
	assert.Len(t, info.HostConfig.PortBindings, 1)
	assert.NotNil(t, info.Config.ExposedPorts)
}

// TestContainer_GetCreateConfigLeavesInspectDataUnchanged verifies that
// building the create config, which drops values the image already provides,
// does not change the container's own health check or exposed ports.
func TestContainer_GetCreateConfigLeavesInspectDataUnchanged(t *testing.T) {
	t.Parallel()

	health := dockerContainer.HealthConfig{Test: []string{"CMD", "true"}, Retries: 3, Interval: time.Second}
	c := MockContainer(
		WithHealthcheck(health),
		WithImageHealthcheck(health),
		WithPortBindings("8080/tcp"),
		func(cnt *dockerContainer.InspectResponse, img *dockerImage.InspectResponse) {
			cnt.Config.ExposedPorts = dockerNetwork.PortSet{dockerNetwork.MustParsePort("80/tcp"): {}}
			img.Config.ExposedPorts = map[string]struct{}{"80/tcp": {}}
		},
	)

	config := c.GetCreateConfig()
	require.NotNil(t, config)
	assert.Nil(t, config.Healthcheck.Test, "the create config drops the image's health check")
	assert.NotContains(t, config.ExposedPorts, dockerNetwork.MustParsePort("80/tcp"))
	assert.Contains(t, config.ExposedPorts, dockerNetwork.MustParsePort("8080/tcp"))

	info := c.ContainerInfo()
	assert.Equal(t, health, *info.Config.Healthcheck, "the container's health check is unchanged")
	assert.Equal(t, dockerNetwork.PortSet{dockerNetwork.MustParsePort("80/tcp"): {}}, info.Config.ExposedPorts,
		"the container's exposed ports are unchanged")
}

// TestContainer_ConcurrentAccess runs changes and reads on one container at
// the same time. It finds data races when run with -race.
func TestContainer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	health := dockerContainer.HealthConfig{Test: []string{"CMD", "true"}}
	c := MockContainer(
		WithLabels(map[string]string{"com.docker.compose.project": "stack", "com.docker.compose.service": "app"}),
		WithHealthcheck(health),
		WithImageHealthcheck(health),
		WithPortBindings("8080/tcp"),
	)

	const rounds = 200

	var wg sync.WaitGroup

	for _, work := range []func(int){
		func(i int) { c.SetLabel(ContainerChainLabel, string(rune('a'+i%26))) },
		func(int) { c.DeleteLabel("missing") },
		func(int) { c.SetImageName("org/app:2") },
		func(int) { _ = c.VerifyConfiguration() },
		func(int) { _ = ResolveContainerIdentifier(c) },
		func(int) { _ = c.ContainerInfo().Config.Labels[ContainerChainLabel] },
		func(int) { _ = c.GetCreateConfig() },
		func(int) { _ = c.GetCreateHostConfig() },
		func(int) { _, _ = c.GetContainerChain() },
	} {
		wg.Go(func() {
			for i := range rounds {
				work(i)
			}
		})
	}

	wg.Wait()
}
