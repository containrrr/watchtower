package sorter_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	"github.com/nicholas-fedor/watchtower/pkg/sorter"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// TestContainerSorting verifies through the exported API that containers are
// sorted after their dependencies, including chained, multiple, and diamond
// dependencies given as slash-prefixed links, with Watchtower containers last.
func TestContainerSorting(t *testing.T) {
	t.Parallel()

	t.Run("SortByDependencies", func(t *testing.T) {
		t.Parallel()

		t.Run("sorting by dependencies", func(t *testing.T) {
			t.Parallel()

			t.Run("sorts containers with no links first", func(t *testing.T) {
				t.Parallel()

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(t)
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				require.Len(t, containers, 2)
				assert.Equal(t, "c2", containers[0].Name()) // No links
				assert.Equal(t, "c1", containers[1].Name()) // Depends on c2
			})

			t.Run("handles multiple dependencies", func(t *testing.T) {
				t.Parallel()

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2", "c3"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(t)
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string{"c3"})
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c3 := mockTypes.NewMockContainer(t)
				c3.EXPECT().Name().Return("c3")
				c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
				c3.EXPECT().Links(true).Return([]string(nil))
				c3.EXPECT().IsWatchtower().Return(false)
				c3.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2, c3}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				require.Len(t, containers, 3)
				assert.Equal(t, "c3", containers[0].Name()) // No links
				assert.Equal(t, "c2", containers[1].Name()) // Links to c3
				assert.Equal(t, "c1", containers[2].Name()) // Links to c2, c3
			})

			t.Run("detects circular references", func(t *testing.T) {
				t.Parallel()

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1")).Maybe()
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(t)
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2")).Maybe()
				c2.EXPECT().Links(true).Return([]string{"c1"})
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c2}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "circular reference detected")
				assert.Contains(t, err.Error(), "c1 -> c2 -> c1")
			})

			t.Run("handles missing dependencies gracefully", func(t *testing.T) {
				t.Parallel()

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c3 := mockTypes.NewMockContainer(t)
				c3.EXPECT().Name().Return("c3")
				c3.EXPECT().ID().Return(types.ContainerID("id-c3"))
				c3.EXPECT().Links(true).Return([]string(nil))
				c3.EXPECT().IsWatchtower().Return(false)
				c3.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{c1, c3}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				require.Len(t, containers, 2)
				assert.Equal(t, "c3", containers[0].Name()) // No links
				assert.Equal(t, "c1", containers[1].Name()) // Links to missing c2
			})

			t.Run("handles empty list", func(t *testing.T) {
				t.Parallel()

				containers := []types.Container{}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				assert.Empty(t, containers)
			})

			t.Run("places Watchtower containers last", func(t *testing.T) {
				t.Parallel()

				watchtower := mockTypes.NewMockContainer(t)
				watchtower.EXPECT().Name().Return("watchtower")
				watchtower.EXPECT().IsWatchtower().Return(true)

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				c2 := mockTypes.NewMockContainer(t)
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{watchtower, c1, c2}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				require.Len(t, containers, 3)
				assert.Equal(t, "c2", containers[0].Name()) // No links
				assert.Equal(t, "c1", containers[1].Name()) // Depends on c2
				assert.Equal(t, "watchtower", containers[2].Name())
				// Watchtower last
			})

			t.Run("places multiple Watchtower containers last", func(t *testing.T) {
				t.Parallel()

				watchtower1 := mockTypes.NewMockContainer(t)
				watchtower1.EXPECT().Name().Return("watchtower1")
				watchtower1.EXPECT().IsWatchtower().Return(true)

				c1 := mockTypes.NewMockContainer(t)
				c1.EXPECT().Name().Return("c1")
				c1.EXPECT().ID().Return(types.ContainerID("id-c1"))
				c1.EXPECT().Links(true).Return([]string{"c2"})
				c1.EXPECT().IsWatchtower().Return(false)
				c1.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})

				watchtower2 := mockTypes.NewMockContainer(t)
				watchtower2.EXPECT().Name().Return("watchtower2")
				watchtower2.EXPECT().IsWatchtower().Return(true)

				c2 := mockTypes.NewMockContainer(t)
				c2.EXPECT().Name().Return("c2")
				c2.EXPECT().ID().Return(types.ContainerID("id-c2"))
				c2.EXPECT().Links(true).Return([]string(nil))
				c2.EXPECT().IsWatchtower().Return(false)
				c2.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
					Config: &dockerContainer.Config{
						Labels: map[string]string{},
					},
				})
				containers := []types.Container{watchtower1, c1, watchtower2, c2}
				err := sorter.SortByDependencies(newTestLogger(t), containers, true)
				require.NoError(t, err)
				require.Len(t, containers, 4)
				assert.Equal(t, "c2", containers[0].Name()) // No links
				assert.Equal(t, "c1", containers[1].Name()) // Depends on c2
				// Watchtower containers at the end (order may vary)
				watchtowerNames := []string{containers[2].Name(), containers[3].Name()}
				assert.Contains(t, watchtowerNames, "watchtower1")
				assert.Contains(t, watchtowerNames, "watchtower2")
			})

			t.Run("handles chained dependencies with slash-prefixed links", func(t *testing.T) {
				t.Parallel()

				newMockContainer := func(t *testing.T, name, id string, links []string, labels map[string]string) types.Container {
					t.Helper()

					c := mockTypes.NewMockContainer(t)
					c.EXPECT().Name().Return(name)
					c.EXPECT().ID().Return(types.ContainerID(id))
					c.EXPECT().Links(true).Return(links)
					c.EXPECT().IsWatchtower().Return(false)
					c.EXPECT().ContainerInfo().Return(&dockerContainer.InspectResponse{
						Config: &dockerContainer.Config{
							Labels: labels,
						},
					})

					return c
				}

				makeSimpleChainContainers := func(t *testing.T) []types.Container {
					t.Helper()

					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/c"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b"}, map[string]string{})

					return []types.Container{c, b, a}
				}

				makeMultipleDepsContainers := func(t *testing.T) []types.Container {
					t.Helper()

					d := newMockContainer(t, "d", "id-d", nil, map[string]string{})
					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/d"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b", "/c"}, map[string]string{})

					return []types.Container{d, c, b, a}
				}

				makeDiamondContainers := func(t *testing.T) []types.Container {
					t.Helper()

					d := newMockContainer(t, "d", "id-d", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", []string{"/d"}, map[string]string{})
					c := newMockContainer(t, "c", "id-c", []string{"/d"}, map[string]string{})
					a := newMockContainer(t, "a", "id-a", []string{"/b", "/c"}, map[string]string{})

					return []types.Container{d, b, c, a}
				}

				makeNoDepsContainers := func(t *testing.T) []types.Container {
					t.Helper()

					c := newMockContainer(t, "c", "id-c", nil, map[string]string{})
					b := newMockContainer(t, "b", "id-b", nil, map[string]string{})
					a := newMockContainer(t, "a", "id-a", nil, map[string]string{})

					return []types.Container{c, b, a}
				}

				testCases := []struct {
					name          string
					containers    func(*testing.T) []types.Container
					expectedOrder []string
				}{
					{
						name:          "simple chain with slashes",
						containers:    makeSimpleChainContainers,
						expectedOrder: []string{"c", "b", "a"},
					},
					{
						name:          "multiple dependencies with slashes",
						containers:    makeMultipleDepsContainers,
						expectedOrder: []string{"d", "c", "b", "a"},
					},
					{
						name:       "diamond dependency graph",
						containers: makeDiamondContainers,
						expectedOrder: []string{
							"d",
							"b",
							"c",
							"a",
						}, // D first, then B and C (order may vary), then A
					},
					{
						name:          "no dependencies",
						containers:    makeNoDepsContainers,
						expectedOrder: []string{"c", "b", "a"},
					},
				}
				for _, tc := range testCases {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()

						containers := tc.containers(t)
						err := sorter.SortByDependencies(newTestLogger(t), containers, true)
						require.NoError(t, err)
						require.Len(t, containers, len(tc.expectedOrder))
						// For diamond, check that D is first, A is last, and B/C are in middle
						if tc.name == "diamond dependency graph" {
							assert.Equal(t, "d", containers[0].Name())
							assert.Equal(t, "a", containers[3].Name())
							middleNames := []string{containers[1].Name(), containers[2].Name()}
							assert.Contains(t, middleNames, "b")
							assert.Contains(t, middleNames, "c")
						} else {
							for i, name := range tc.expectedOrder {
								assert.Equal(t, name, containers[i].Name())
							}
						}
					})
				}
			})
		})
	})
}

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
