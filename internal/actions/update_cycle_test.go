package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"
	dockerNetwork "github.com/moby/moby/api/types/network"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_StaleContainerBesideCycle verifies that a dependency cycle
// among other containers does not prevent updating a stale container.
func TestUpdateAction_StaleContainerBesideCycle(t *testing.T) {
	t.Parallel()

	t.Run("updates the stale container and does not fail the session", func(t *testing.T) {
		t.Parallel()

		containerA := mockActions.CreateMockContainerWithConfig(
			"cycle-a",
			"/cycle-a",
			"fake-image-a:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "cycle-b,cycle-c",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		containerB := mockActions.CreateMockContainerWithConfig(
			"cycle-b",
			"/cycle-b",
			"fake-image-b:latest",
			true,
			false,
			time.Now(),
			&dockerContainer.Config{
				Labels: map[string]string{
					"com.centurylinklabs.watchtower.depends-on": "cycle-a",
				},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)
		containerC := mockActions.CreateMockContainerWithConfig(
			"cycle-c",
			"/cycle-c",
			"fake-image-c:latest",
			true,
			false,
			time.Now().AddDate(0, 0, -1),
			&dockerContainer.Config{
				Labels:       map[string]string{},
				ExposedPorts: dockerNetwork.PortSet{},
			},
		)

		client := mockActions.CreateMockClient(
			&mockActions.TestData{
				Containers: []types.Container{containerA, containerB, containerC},
				Staleness:  map[string]bool{"cycle-c": true},
			},
			false,
			false,
		)

		report, _, err := update(newTestLogger(t), t.Context(), client, types.UpdateParams{Cleanup: true}, ratelimit.New())

		require.NoError(t, err)
		require.Len(t, report.Updated(), 1)
		assert.Equal(t, "cycle-c", report.Updated()[0].Name())
	})
}
