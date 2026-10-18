package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// Container IDs used by the chain tests.
const (
	// chainCurrentID is the running Watchtower container.
	chainCurrentID = "c000000000000000000000000000000000000000000000000000000000000000"
	// chainOldAID is a Watchtower container the current one replaced.
	chainOldAID = "a000000000000000000000000000000000000000000000000000000000000000"
	// chainOldBID is another Watchtower container the current one replaced.
	chainOldBID = "b000000000000000000000000000000000000000000000000000000000000000"
	// chainAppID is a container that is not a Watchtower container.
	chainAppID = "d000000000000000000000000000000000000000000000000000000000000000"
)

// chainTestContainer returns a running container with the given labels.
func chainTestContainer(id, name string, labels map[string]string) types.Container {
	return mockActions.CreateMockContainerWithConfig(
		id,
		"/"+name,
		"org/"+name+":1",
		true,
		false,
		time.Now(),
		&dockerContainer.Config{Image: "org/" + name + ":1", Labels: labels},
	)
}

// chainTestContainers returns the current Watchtower container with the given
// chain label, two Watchtower containers it may have replaced, and an
// ordinary container.
func chainTestContainers(chain string) (types.Container, []types.Container) {
	current := chainTestContainer(chainCurrentID, "watchtower", map[string]string{
		"com.centurylinklabs.watchtower":                 "true",
		"com.centurylinklabs.watchtower.container-chain": chain,
	})
	oldA := chainTestContainer(chainOldAID, "watchtower-a", map[string]string{"com.centurylinklabs.watchtower": "true"})
	oldB := chainTestContainer(chainOldBID, "watchtower-b", map[string]string{"com.centurylinklabs.watchtower": "true"})
	app := chainTestContainer(chainAppID, "app", map[string]string{})

	return current, []types.Container{current, oldA, oldB, app}
}

// containerIDs returns the ID of each container, in order.
func containerIDs(containers []types.Container) []types.ContainerID {
	ids := make([]types.ContainerID, 0, len(containers))
	for _, c := range containers {
		ids = append(ids, c.ID())
	}

	return ids
}

// TestGetChainedContainers verifies which containers named in the current
// Watchtower container's chain label are returned for removal.
func TestGetChainedContainers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chain string
		want  []types.ContainerID
	}{
		{
			name:  "Watchtower containers in the chain",
			chain: chainOldAID + "," + chainOldBID,
			want:  []types.ContainerID{chainOldAID, chainOldBID},
		},
		{
			name:  "a container that is not Watchtower is never returned",
			chain: chainOldAID + "," + chainAppID,
			want:  []types.ContainerID{chainOldAID},
		},
		{
			name:  "spaces around IDs",
			chain: " " + chainOldAID + " , " + chainOldBID + " ",
			want:  []types.ContainerID{chainOldAID, chainOldBID},
		},
		{
			name:  "empty entries",
			chain: "," + chainOldAID + ",, ," + chainOldBID + ",",
			want:  []types.ContainerID{chainOldAID, chainOldBID},
		},
		{
			name:  "the current container is never returned",
			chain: chainOldAID + "," + chainCurrentID,
			want:  []types.ContainerID{chainOldAID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			current, all := chainTestContainers(tt.chain)

			got := getChainedContainers(newTestLogger(t), all, current)

			assert.Equal(t, tt.want, containerIDs(got))
		})
	}
}

// TestRemoveExcessWatchtowerInstances_ChainKeepsOtherContainers verifies that
// startup cleanup leaves a container that is not Watchtower running when the
// chain label names it, and still removes the Watchtower container it names.
func TestRemoveExcessWatchtowerInstances_ChainKeepsOtherContainers(t *testing.T) {
	t.Parallel()

	current := chainTestContainer(chainCurrentID, "watchtower", map[string]string{
		"com.centurylinklabs.watchtower":                 "true",
		"com.centurylinklabs.watchtower.container-chain": chainOldAID + "," + chainAppID,
	})
	oldA := chainTestContainer(chainOldAID, "watchtower-a", map[string]string{"com.centurylinklabs.watchtower": "true"})
	app := chainTestContainer(chainAppID, "app", map[string]string{})

	// The mock fails the test on any other stop.
	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).Return([]types.Container{current, oldA, app}, nil)
	client.EXPECT().StopAndRemoveContainer(mock.Anything, oldA, mock.Anything).Return(nil).Once()

	removed, err := RemoveExcessWatchtowerInstances(newTestLogger(t), t.Context(), client, false, "", nil, current)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
}
